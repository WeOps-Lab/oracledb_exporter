package collector

import (
	"errors"
	"fmt"
	"path/filepath"

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

// DefaultMetrics is a somewhat hacky way to load the default metrics
func (e *Exporter) DefaultMetrics() Metrics {
	var metricsToScrape Metrics
	if e.config.DefaultMetricsFile != "" {
		if _, err := toml.DecodeFile(filepath.Clean(e.config.DefaultMetricsFile), &metricsToScrape); err != nil {
			level.Error(e.logger).Log(fmt.Sprintf("there was an issue while loading specified default metrics file at: "+e.config.DefaultMetricsFile+", proceeding to run with default metrics."), err)
		}
		return metricsToScrape
	}

	if _, err := toml.Decode(defaultMetricsConst, &metricsToScrape); err != nil {
		level.Error(e.logger).Log(err)
		panic(errors.New("Error while loading " + defaultMetricsConst))
	}

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
