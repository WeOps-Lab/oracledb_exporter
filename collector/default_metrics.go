package collector

import (
	"errors"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/go-kit/log/level"
)

// needs the const if imported, cannot os.ReadFile in this case
const defaultMetricsConst = `
[[metric]]
context = "uptime"
labels = [ "inst_id", "node_name", "instance_name"]
metricsdesc = { seconds = "instance uptime" }
request = "SELECT instance_number AS inst_id, host_name AS node_name, instance_name, (SYSDATE - startup_time) * 86400 AS seconds FROM v$instance"

[[metric]]
context = "sessions"
labels = [ "status", "type" ]
metricsdesc = { value= "Gauge metric with count of sessions by status and type." }
request = "SELECT status, type, COUNT(*) as value FROM v$session GROUP BY status, type"

[[metric]]
context = "resource"
labels = [ "resource_name" ]
metricsdesc = { current_utilization= "Generic counter metric from v$resource_limit view in Oracle (current value).", limit_value="Generic counter metric from v$resource_limit view in Oracle (UNLIMITED: -1)." }
request="SELECT resource_name,current_utilization,CASE WHEN TRIM(limit_value) LIKE 'UNLIMITED' THEN '-1' ELSE TRIM(limit_value) END as limit_value FROM v$resource_limit"

[[metric]]
context = "asm_diskgroup"
labels = [ "diskgroup_name" ]
metricsdesc = { total = "Total size of ASM disk group.", free = "Free space available on ASM disk group.", usage = "Percentage of ASM disk group used."}
request = "SELECT name as diskgroup_name,total_mb*1024*1024 as total,free_mb*1024*1024 as free, (1 - free_mb / total_mb) * 100 as usage FROM v$asm_diskgroup_stat where exists (select 1 from v$datafile where name like '+%')"
ignorezeroresult = true

[[metric]]
context = "activity"
metricsdesc = { value="Generic counter metric from v$sysstat view in Oracle." }
fieldtoappend = "name"
request = "SELECT name, value FROM v$sysstat WHERE name IN ('parse count (total)', 'execute count', 'user commits', 'user rollbacks')"

[[metric]]
context = "process"
metricsdesc = { count="Gauge metric with count of processes." }
request = "SELECT COUNT(*) as count FROM v$process"

[[metric]]
context = "wait_time"
metricsdesc = { value="Generic counter metric from v$waitclassmetric view in Oracle." }
fieldtoappend= "wait_class"
request = '''
SELECT
  n.wait_class as WAIT_CLASS,
  round(m.time_waited*10/(m.INTSIZE_CSEC),3) as VALUE
FROM
  v$waitclassmetric  m, v$system_wait_class n
WHERE
  m.wait_class_id=n.wait_class_id AND n.wait_class != 'Idle'
'''

[[metric]]
context = "tablespace"
labels = [ "tablespace", "type" ]
metricsdesc = { bytes = "Generic counter metric of tablespaces bytes in Oracle.", max_bytes = "Generic counter metric of tablespaces max bytes in Oracle.", free = "Generic counter metric of tablespaces free bytes in Oracle.", used_percent = "Gauge metric showing as a percentage of how much of the tablespace has been used." }
request = '''
SELECT
    dt.tablespace_name as tablespace,
    dt.contents as type,
    dt.block_size * dtum.used_space as bytes,
    dt.block_size * dtum.tablespace_size as max_bytes,
    dt.block_size * (dtum.tablespace_size - dtum.used_space) as free,
    dtum.used_percent
FROM  dba_tablespace_usage_metrics dtum, dba_tablespaces dt
WHERE dtum.tablespace_name = dt.tablespace_name
ORDER by tablespace
'''

[[metric]]
context = "pga"
metricsdesc = { total = "Generic counter metric of aggregate PGA target parameter in Oracle.", used = "Generic counter metric of total PGA allocated in Oracle.", used_percent = "Gauge metric showing as a percentage of how much of the PGA has been used." }
request = '''
SELECT
    (SELECT value FROM v$pgastat WHERE name = 'aggregate PGA target parameter') AS total,
    (SELECT value FROM v$pgastat WHERE name = 'total PGA allocated') AS used,
    (SELECT value FROM v$pgastat WHERE name = 'total PGA allocated') / (SELECT value FROM v$pgastat WHERE name = 'aggregate PGA target parameter') * 100 AS used_percent
FROM
    dual
'''

[[metric]]
context = "sga"
metricsdesc = { total = "Generic counter metric of total SGA size in Oracle.", free = "Generic counter metric of free SGA memory in Oracle.", used_percent = "Gauge metric showing as a percentage of how much of the SGA has been used." }
request = '''
SELECT
    (SELECT SUM(value) FROM v$sga) AS total,
    (SELECT SUM(bytes) FROM v$sgastat WHERE name = 'free memory') AS free,
    ((SELECT SUM(value) FROM v$sga) - (SELECT SUM(bytes) FROM v$sgastat WHERE name = 'free memory')) / (SELECT SUM(value) FROM v$sga) * 100 AS used_percent
FROM
    dual
'''
`
const ASMMetricsConst = `
[[metric]]
context = "asm_disk_stat"
labels = [ "inst_id", "node_name", "instance_name", "diskgroup_name", "disk_number", "failgroup", "path" ]
metricsdesc = { reads = "Total number of I/O read requests for the DG.", writes = "Total number of I/O write requests for the DG.", read_time = "Total I/O time (in hundreths of a second) for read requests for the disk", write_time = "Total I/O time (in hundreths of a second) for write requests for the disk", bytes_read = "Total number of bytes read from the DG", bytes_written = "Total number of bytes written from the DG", iops = "Total number of I/O requests for the DG" }
metricstype = { reads = "counter", writes = "counter", bytes_read = "counter", read_time = "counter", write_time = "counter", bytes_written = "counter", iops = "counter" }
request = '''
  SELECT i.instance_number                         AS inst_id,
		 i.host_name                               AS node_name,
		 i.instance_name,
		 g.name                                    AS diskgroup_name,
		 ds.disk_number                            AS disk_number,
		 ds.failgroup                              AS failgroup,
		 ds.reads                                  AS reads,
		 ds.writes                                 AS writes,
		 ds.read_time * 1000                       AS read_time,
		 ds.write_time * 1000                      AS write_time,
		 ds.bytes_read                             AS bytes_read,
		 ds.bytes_written                          AS bytes_written,
		 REGEXP_REPLACE (ds.PATH, '.*/\', '\')     AS PATH,
		 ds.reads + ds.writes                      AS iops
	FROM v$asm_disk_stat ds, v$asm_diskgroup_stat g, v$instance i
   WHERE ds.mount_status = 'CACHED' AND ds.group_number = g.group_number
'''

[[metric]]
context = "asm_space_consumers"
labels = [ "inst_id", "diskgroup_name", "node_name", "instance_name", "sid", "file_type" ]
metricsdesc = { size_mb = "Total space usage by db by file_type" , files = "Number of files by db by type" }
request = '''
  SELECT i.instance_number                     AS inst_id,
		 i.host_name                           AS node_name,
		 i.instance_name,
		 gname                                 AS diskgroup_name,
		 dbname                                AS sid,
		 file_type,
		 ROUND (SUM (space) / 1024 / 1024)     size_mb,
		 COUNT (*)                             AS files
	FROM v$instance i,
		 (SELECT gname,
				 REGEXP_SUBSTR (full_alias_path,
								'[[:alnum:]_]*',
								1,
								4)    dbname,
				 file_type,
				 space,
				 aname,
				 system_created,
				 alias_directory
			FROM (    SELECT CONCAT ('+' || gname,
									 SYS_CONNECT_BY_PATH (aname, '/'))
								 full_alias_path,
							 system_created,
							 alias_directory,
							 file_type,
							 space,
							 LEVEL,
							 gname,
							 aname
						FROM (SELECT b.name                gname,
									 a.parent_index        pindex,
									 a.name                aname,
									 a.reference_index     rindex,
									 a.system_created,
									 a.alias_directory,
									 c.TYPE                file_type,
									 c.space
								FROM v$asm_alias a, v$asm_diskgroup b, v$asm_file c
							   WHERE     a.group_number = b.group_number
									 AND a.group_number = c.group_number(+)
									 AND a.file_number = c.file_number(+)
									 AND a.file_incarnation = c.incarnation(+))
				  START WITH     (MOD (pindex, POWER (2, 24))) = 0
							 AND rindex IN
									 (SELECT a.reference_index
										FROM v$asm_alias a, v$asm_diskgroup b
									   WHERE     a.group_number =
												 b.group_number
											 AND (MOD (a.parent_index,
													   POWER (2, 24))) =
												 0)
				  CONNECT BY PRIOR rindex = pindex)
		   WHERE NOT file_type IS NULL AND system_created = 'Y')
GROUP BY i.instance_number,
		 i.host_name,
		 i.instance_name,
		 gname,
		 dbname,
		 file_type
'''
`
const DGMetricsConst = `
[[metric]]
context = "dataguard_apply"
metricsdesc = { lag_delay = "Apply lag in seconds." }
request = '''
SELECT
    to_number(substr(v.value, 2, 2)) * 24 * 60 * 60 + to_number(substr(v.value, 5, 2)) * 60 * 60 + to_number(substr(v.VALUE, 8, 2)) * 60 + to_number(substr(v.VALUE, 11, 2)) AS lag_delay
FROM
    v$dataguard_stats v
WHERE
    v.name = 'apply lag'
'''

[[metric]]
context = "dataguard_transport"
metricsdesc = { lag_delay = "Transport lag in seconds." }
request = '''
SELECT
    to_number(substr(v.value, 2, 2)) * 24 * 60 * 60 + to_number(substr(v.value, 5, 2)) * 60 * 60 + to_number(substr(v.VALUE, 8, 2)) * 60 + to_number(substr(v.VALUE, 11, 2)) AS lag_delay
FROM
    v$dataguard_stats v
WHERE
    v.name = 'transport lag'
'''
`

const racMetricsConst = `
[[metric]]
context = "rac"
metricsdesc = { node = "Number of nodes in the RAC cluster." }
request = "select count(*) as node from gv$instance where database_type='RAC'"
`

// DefaultMetrics is a somewhat hacky way to load the default metrics
func (e *Exporter) DefaultMetrics() Metrics {
	var metricsToScrape Metrics
	var err error
	if e.config.DefaultMetricsFile != "" {
		if strings.HasSuffix(e.config.DefaultMetricsFile, "toml") {
			err = loadTomlMetricsConfig(e.config.DefaultMetricsFile, &metricsToScrape)
		} else {
			err = loadYamlMetricsConfig(e.config.DefaultMetricsFile, &metricsToScrape)
		}
		if err == nil {
			return metricsToScrape
		}
		level.Error(e.logger).Log("defaultMetricsFile", e.config.DefaultMetricsFile, "msg", err)
		level.Warn(e.logger).Log("msg", "proceeding to run with default metrics")
	}

	if _, err := toml.Decode(defaultMetricsConst, &metricsToScrape); err != nil {
		level.Error(e.logger).Log("msg", err.Error())
		panic(errors.New("Error while loading " + defaultMetricsConst))
	}

	// rac类指标
	var racMetricsToScrape Metrics
	if e.config.IsRAC {
		if _, err := toml.Decode(racMetricsConst, &racMetricsToScrape); err != nil {
			level.Error(e.logger).Log(err)
			panic(errors.New("Error while loading " + racMetricsConst))
		}
		metricsToScrape.Metric = append(metricsToScrape.Metric, racMetricsToScrape.Metric...)
	}

	// ASM磁盘组类指标
	var ASMMetricsToScrape Metrics
	if e.config.IsASM {
		if _, err := toml.Decode(ASMMetricsConst, &ASMMetricsToScrape); err != nil {
			level.Error(e.logger).Log(err)
			panic(errors.New("Error while loading " + ASMMetricsConst))
		}
		metricsToScrape.Metric = append(metricsToScrape.Metric, ASMMetricsToScrape.Metric...)
	}

	// dataGuard类指标
	var dgMetricsToScrape Metrics
	if e.config.IsDG {
		if _, err := toml.Decode(DGMetricsConst, &dgMetricsToScrape); err != nil {
			level.Error(e.logger).Log(err)
			panic(errors.New("Error while loading " + DGMetricsConst))
		}
		metricsToScrape.Metric = append(metricsToScrape.Metric, dgMetricsToScrape.Metric...)
	}

	return metricsToScrape
}
