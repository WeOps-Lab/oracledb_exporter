## 嘉为蓝鲸oracledb插件使用说明

## 使用说明

### 插件功能
采集器连接oracle数据库，执行SQL查询语句，将结果解析到prometheus数据格式的监控指标。  
实际收集的指标取决于数据库的配置和版本。

### 版本支持

操作系统支持: linux, windows

是否支持arm: 支持

**组件支持版本：**

Oracle Database: `11g`, `12c`, `18c`, `19c`, `21c`

部署模式支持: `standalone(单点)`, `RAC(集群)`

**是否支持远程采集:**  

是

### 参数说明

| **参数名**              | **含义**                                                                                                                                 | **是否必填** | **使用举例**                                       |
|----------------------|----------------------------------------------------------------------------------------------------------------------------------------|----------|------------------------------------------------|
| DATA_SOURCE_NAME     | DSN参数，在连接Oracle数据库时，需要提供一个连接字符串，其中包括Oracle数据库实例的主机名、端口号和服务名称，例如： oracle://username:password@hostname:port/service_name **注意！该参数为环境变量** | 是        | oracle://weops:Weops123@127.0.0.1:1521/ORCLCDB |
| --custom.metrics     | 自定义指标查询文件路径   **注意！该参数在平台层面为文件参数，进程中该参数值为采集配置文件路径(上传文件即可，平台会补充文件路径)！**                                                                 |          |                                                |
| --query.timeout      | 查询超时秒数，默认使用5s                                                                                                                          | 否        | 5                                              |
| --log.level          | 日志级别                                                                                                                                   | 否        | info                                           |
| --web.listen-address | exporter监听id及端口地址                                                                                                                      | 否        | 127.0.0.1:9601                                 |


### 使用指引
1. 查看Oracle数据库服务名和域名  
   注意！**对于oracle数据库12版本，DSN中数据库名后必须加入域名，其他版本一般不需要**  
   ORCLCDB是Oracle数据库的一个服务名称（Service Name），它用于唯一标识数据库实例中的一个服务。  
   例: "oracle://system:Weops123!@db12c-oracle-db.oracle:1521/ORCLCDB.localdomain"  
   - 查看当前数据库实例的 `SERVICE_NAME` 参数的值。
      ```sql
     SELECT value FROM v$parameter WHERE name = 'service_names'; 
     ```  

   - 查看当前数据库实例的 `DB_DOMAIN` 参数的值。如果返回结果为空，表示未设置特定的域名。
      ```sql
      SELECT value FROM v$parameter WHERE name = 'db_domain';
      ```
  
2. 若出现unknown service error
   - 需检查监听器的当前状态，确保监听器正在运行并监听正确的端口，运行命令 `lsnrctl status`。
   - 确认监听器配置文件（`lsnrctl status`会输出监听器配置状态等信息，寻找配置文件，通常是 listener.ora）中是否正确定义了服务名称，并与您尝试连接的服务名称匹配。
   - `lsnrctl` 在oracle数据库12版本中，此命令一般存放于 `/u01/app/oracle/product/12.2.0/dbhome_1/` ； 在oracle数据库19版本中，一般存放于 `/opt/oracle/product/19c/dbhome_1/bin`

3. 连接Oracle数据库  
   使用操作系统的身份认证（通常是超级用户或管理员），直接以 sysdba 角色登录到数据库
    ```shell
    sqlplus / as sysdba
    ```  
      
    使用指定账户登录
    ```shell
    sqlplus username/password@host:port/service_name
    ```  

4. 创建账户及授权  
    注意！创建账户时必须使用管理员账户
    ```sql
    # 新建用户
    CREATE USER [user] IDENTIFIED BY [password];

    # 修改用户的密码，密码若含特殊字符需使用双引号将密码括起来
    ALTER USER [user] IDENTIFIED BY [password];

    # 允许用户建立数据库会话
    GRANT CREATE SESSION TO [user];

    # uptime指标授权
    GRANT SELECT ON V_$instance to [user];

    # rac指标授权
    GRANT SELECT ON GV_$instance to [user];

    # sessions类指标授权
    GRANT SELECT ON V_$session to [user];

    # resource类指标授权
    GRANT SELECT ON V_$resource_limit to [user];

    # asm_diskgroup类指标授权
    GRANT SELECT ON V_$datafile to [user];
    GRANT SELECT ON V_$asm_diskgroup_stat to [user];

    # activity类指标授权
    GRANT SELECT ON V_$sysstat to [user];

    # process类指标授权
    GRANT SELECT ON V_$process to [user];

    # wait_time类指标授权
    GRANT SELECT ON V_$waitclassmetric to [user];
    GRANT SELECT ON V_$system_wait_class to [user];

    # tablespace类指标授权
    GRANT SELECT ON dba_tablespace_usage_metrics to [user];
    GRANT SELECT ON dba_tablespaces to [user];

    # asm_disk_stat类指标授权
    GRANT SELECT ON V_$asm_disk_stat to [user];
    GRANT SELECT ON V_$asm_diskgroup_stat to [user];
    GRANT SELECT ON V_$instance to [user];

    # asm_space_consumers类指标授权
    GRANT SELECT ON V_$asm_alias to [user];
    GRANT SELECT ON V_$asm_diskgroup to [user];
    GRANT SELECT ON V_$asm_file to [user];
   
    # sga类指标授权
    GRANT SELECT ON V_$sga TO weops;
    GRANT SELECT ON V_$sgastat TO weops;
    
    # pga类指标授权
    GRANT SELECT ON V_$pgastat TO weops;
    ```

5. 自定义指标查询文件
   - 文件内容规范
     - 每一类自定义查询指标必须含有`[[metric]]`开头
     - 对于每个指标部分，需要提供上下文（context）、请求（request）和请求字段与注释之间的映射。
     - `context` 指标前缀
     - `labels` 指标维度数据信息，[维度1], [维度2], [维度3]...
     - `metricsdesc`  [指标后缀] = [指标的描述信息]
     - `metricstype` [指标后缀] = [指标类型]
     - `request` sql查询语句，注意sql中字段与 `labels` 和 `metricsdesc` 的映射  

   - 使用自定义指标查询 (通过命令行参数 `--custom.metrics` 设置)，下方是默认的自定义指标文件配置内容
    ```toml
    [[metric]]
    context = "rac"
    metricsdesc = { node = "Number of nodes in the RAC cluster." }
    request = "select count(*) as node from gv$instance where database_type='RAC'"

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
    ```


### 指标简介
| **指标ID**                                       | **指标中文名**             | **维度ID**                                                                        | **维度含义**                                   | **单位**  |
|------------------------------------------------|-----------------------|---------------------------------------------------------------------------------|--------------------------------------------|---------|
| oracledb_up                                    | Oracle数据库运行状态         | -                                                                               | -                                          | -       |
| oracledb_uptime_seconds                        | Oracle数据库实例已运行时间      | inst_id, instance_name, node_name                                               | 实例ID, 实例名称, 节点名称                           | s       |
| oracledb_activity_execute_count                | Oracle数据库执行次数         | -                                                                               | -                                          | -       |
| oracledb_activity_parse_count_total            | Oracle数据库解析次数         | -                                                                               | -                                          | -       |
| oracledb_activity_user_commits                 | Oracle数据库用户提交次数       | -                                                                               | -                                          | -       |
| oracledb_activity_user_rollbacks               | Oracle数据库用户回滚次数       | -                                                                               | -                                          | -       |
| oracledb_wait_time_application                 | Oracle数据库应用类等待时间      | -                                                                               | -                                          | ms      |
| oracledb_wait_time_commit                      | Oracle数据库提交等待时间       | -                                                                               | -                                          | ms      |
| oracledb_wait_time_concurrency                 | Oracle数据库并发等待时间       | -                                                                               | -                                          | ms      |
| oracledb_wait_time_configuration               | Oracle数据库配置等待时间       | -                                                                               | -                                          | ms      |
| oracledb_wait_time_network                     | Oracle数据库网络等待时间       | -                                                                               | -                                          | ms      |
| oracledb_wait_time_other                       | Oracle数据库其他等待时间       | -                                                                               | -                                          | ms      |
| oracledb_wait_time_scheduler                   | Oracle数据库调度程序等待时间     | -                                                                               | -                                          | ms      |
| oracledb_wait_time_system_io                   | Oracle数据库系统I/O等待时间    | -                                                                               | -                                          | ms      |
| oracledb_wait_time_user_io                     | Oracle数据库用户I/O等待时间    | -                                                                               | -                                          | ms      |
| oracledb_resource_current_utilization          | Oracle数据库当前资源使用量      | resource_name                                                                   | 资源类型                                       | -       |
| oracledb_resource_limit_value                  | Oracle数据库资源限定值        | resource_name                                                                   | 资源类型                                       | -       |
| oracledb_process_count                         | Oracle数据库进程数          | -                                                                               | -                                          | -       |
| oracledb_sessions_value                        | Oracle数据库会话数          | status, type                                                                    | 会话状态，会话类型                                  | -       |
| oracledb_sga_total                             | Oracle数据库SGA总大小       | -                                                                               | -                                          | bytes   |
| oracledb_sga_free                              | Oracle数据库SGA可用大小      | -                                                                               | -                                          | bytes   |
| oracledb_sga_used_percent                      | Oracle数据库SGA使用率       | -                                                                               | -                                          | percent |
| oracledb_pga_total                             | Oracle数据库PGA总大小       | -                                                                               | -                                          | bytes   |
| oracledb_pga_used                              | Oracle数据库PGA已使用大小     | -                                                                               | -                                          | bytes   |
| oracledb_pga_used_percent                      | Oracle数据库PGA使用率       | -                                                                               | -                                          | percent |
| oracledb_tablespace_bytes                      | Oracle数据库表已使用容量大小     | tablespace, type                                                                | 表空间名称，表空间类型                                | bytes   |
| oracledb_tablespace_max_bytes                  | Oracle数据库表最大容量限制      | tablespace, type                                                                | 表空间名称，表空间类型                                | bytes   |
| oracledb_tablespace_free                       | Oracle数据库表可用容量大小      | tablespace, type                                                                | 表空间名称，表空间类型                                | bytes   |
| oracledb_tablespace_used_percent               | Oracle数据库表空间使用率       | tablespace, type                                                                | 表空间名称，表空间类型                                | percent |
| oracledb_rac_node                              | Oracle数据库RAC节点数量      | -                                                                               | -                                          | -       |
| oracledb_asm_diskgroup_free                    | Oracle数据库ASM磁盘组可用空间   | diskgroup_name                                                                  | 磁盘组名称                                      | bytes   |
| oracledb_asm_diskgroup_total                   | Oracle数据库ASM磁盘组总容量    | diskgroup_name                                                                  | 磁盘组名称                                      | bytes   |
| oracledb_asm_diskgroup_usage                   | Oracle数据库ASM磁盘组空间使用率  | diskgroup_name                                                                  | 磁盘组名称                                      | percent |
| oracledb_asm_disk_stat_reads                   | Oracle数据库ASM磁盘的读操作总数  | inst_id, node_name, instance_name, diskgroup_name, disk_number, failgroup, path | 实例ID, 节点名称, 实例名称, 磁盘组名称, 磁盘编号, 故障组名称, 磁盘路径 | -       |
| oracledb_asm_disk_stat_writes                  | Oracle数据库ASM磁盘的写操作总数  | inst_id, node_name, instance_name, diskgroup_name, disk_number, failgroup, path | 实例ID, 节点名称, 实例名称, 磁盘组名称, 磁盘编号, 故障组名称, 磁盘路径 | -       |
| oracledb_asm_disk_stat_bytes_read              | Oracle数据库ASM磁盘的总读取字节数 | inst_id, node_name, instance_name, diskgroup_name, disk_number, failgroup, path | 实例ID, 节点名称, 实例名称, 磁盘组名称, 磁盘编号, 故障组名称, 磁盘路径 | bytes   |
| oracledb_asm_disk_stat_read_time               | Oracle数据库ASM磁盘的读取时间总和 | inst_id, node_name, instance_name, diskgroup_name, disk_number, failgroup, path | 实例ID, 节点名称, 实例名称, 磁盘组名称, 磁盘编号, 故障组名称, 磁盘路径 | ms      |
| oracledb_asm_disk_stat_write_time              | Oracle数据库ASM磁盘的写入时间总和 | inst_id, node_name, instance_name, diskgroup_name, disk_number, failgroup, path | 实例ID, 节点名称, 实例名称, 磁盘组名称, 磁盘编号, 故障组名称, 磁盘路径 | ms      |
| oracledb_asm_disk_stat_bytes_written           | Oracle数据库ASM磁盘的总写入字节数 | inst_id, node_name, instance_name, diskgroup_name, disk_number, failgroup, path | 实例ID, 节点名称, 实例名称, 磁盘组名称, 磁盘编号, 故障组名称, 磁盘路径 | bytes   |
| oracledb_asm_disk_stat_iops                    | Oracle数据库ASM磁盘每秒IO    | inst_id, node_name, instance_name, diskgroup_name, disk_number, failgroup, path | 实例ID, 节点名称, 实例名称, 磁盘组名称, 磁盘编号, 故障组名称, 磁盘路径 | -       |
| oracledb_asm_space_consumers_files             | Oracle数据库ASM磁盘组上文件数量  | diskgroup_name, file_type, inst_id, instance_name, node_name                    | 磁盘组名称, 文件类型, 实例ID, 实例名称, 节点名称              | -       |
| oracledb_asm_space_consumers_size_mb           | Oracle数据库ASM磁盘组上文件大小  | diskgroup_name, file_type, inst_id, instance_name, node_name                    | 磁盘组名称, 文件类型, 实例ID, 实例名称, 节点名称              | MB      |
| process_cpu_seconds_total                      | Oracle数据库进程CPU秒数总计    | -                                                                               | -                                          | s       |
| process_max_fds                                | Oracle数据库进程最大文件描述符数   | -                                                                               | -                                          | -       |
| process_open_fds                               | Oracle数据库进程打开文件描述符数   | -                                                                               | -                                          | -       |
| process_resident_memory_bytes                  | Oracle数据库进程常驻内存大小     | -                                                                               | -                                          | bytes   |
| process_virtual_memory_bytes                   | Oracle数据库进程虚拟内存大小     | -                                                                               | -                                          | bytes   |
| oracledb_exporter_last_scrape_duration_seconds | Oracle数据库监控探针最近一次抓取时长 | -                                                                               | -                                          | s       |
| oracledb_exporter_last_scrape_error            | Oracle数据库监控探针最近一次抓取状态 | -                                                                               | -                                          | -       |
| oracledb_exporter_scrapes_total                | Oracle数据库监控探针抓取指标总数   | -                                                                               | -                                          | -       |

### 版本日志

#### weops_oracledb_exporter 2.2.0

- weops调整

添加“小嘉”微信即可获取oracle数据库监控指标最佳实践礼包，其他更多问题欢迎咨询

<img src="https://wedoc.canway.net/imgs/img/小嘉.jpg" width="50%" height="50%">
