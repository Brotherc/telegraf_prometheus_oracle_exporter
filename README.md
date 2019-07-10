# Telegraf Prometheus Oracle Exporter

A oracle Exporter for [telegraf prometheus input plugins](https://github.com/influxdata/telegraf/tree/master/plugins/inputs/prometheus)

The following metrics are exposed currently. Support for RAC (databasename and instancename added via lables)

- oracledb_exporter_last_scrape_duration_seconds
- oracledb_exporter_last_scrape_error
- oracledb_exporter_scrapes_total
- oracledb_up_info(dbtime, is_rac, uptime, version)
- oracledb_process_count
- oracledb_sql_top
- oracledb_sessions_activity
- oracledb_sessions_active
- oracledb_sessions_inactive
- oracledb_wait_class_time (view v$waitclass)
- oracledb_activity_user
- oracledb_activity_parse
- oracledb_block_num
- oracledb_index_index_fast_full_scans_full
- oracledb_net_bytessent
- oracledb_net_bytesreceived
- oracledb_tablespace_size (tablespace total/free)
- oracledb_parse_ratio
- oracledb_physical_iops
- oracledb_physical_throughput
- oracledb_workload_overview
- oracledb_cache_hitratio (Cache hit ratios (v$sysmetric)

*took very long or Infrequent, be careful (expose the Metrics below by another exporter with Scrape-Config):
- oracledb_table_top

# StartUp
```
go run main.go misc.go
```

# Installation

Ensure that the configfile (oracle.conf or oracle_table.conf) is set correctly before starting. You can add multiple instances.

# telegraf Configuration
scrape oracle-exporter with oracle.conf
```
[global_tags]

[agent]
  interval = "60s"                       #change interval
  round_interval = true
  metric_batch_size = 1000
  metric_buffer_limit = 10000
  collection_jitter = "0s"
  flush_interval = "10s"
  flush_jitter = "0s"
  precision = ""
  debug = false
  quiet = false
  logfile = ""
  hostname = ""
  omit_hostname = false

###############################################################################
#                            OUTPUT PLUGINS                                   #
###############################################################################
[[outputs.influxdb]]
  urls = ["http://ip:8086"]       #change influxdb url
  database = "oracledb_monitor"

###############################################################################
#                            PROCESSOR PLUGINS                                #
###############################################################################
[[processors.converter]]
  [processors.converter.tags]
    string = ["info_version", "sql_text", "sql_username"]
    boolean = ["info_is_rac"]
    float = ["info_uptime", "info_dbtime", "sql_exec"]

###############################################################################
#                            INPUT PLUGINS                                    #
###############################################################################
[[inputs.prometheus]]
  urls = ["http://ip:port/metrics"]    #change oracle exporter web.listen-address
  response_timeout = "150s"
```

# telegraf Configuration
scrape oracle-exporter with oracle_table.conf
```
[global_tags]

[agent]
  interval = "24h"                         #change interval (Don't be less than 30s)
  round_interval = true
  metric_batch_size = 1000
  metric_buffer_limit = 10000
  collection_jitter = "0s"
  flush_interval = "10s"
  flush_jitter = "0s"
  precision = ""
  debug = false
  quiet = false
  logfile = ""
  hostname = ""
  omit_hostname = false

###############################################################################
#                            OUTPUT PLUGINS                                   #
###############################################################################
[[outputs.influxdb]]
  urls = ["http://ip:8086"]       #change influxdb url
  database = "oracledb_monitor"

###############################################################################
#                            PROCESSOR PLUGINS                                #
###############################################################################
[[processors.converter]]
  [processors.converter.tags]
    integer = ["index_bytes", "lob_bytes", "num_rows"]
    string = ["info_version", "sql_text", "sql_username"]
    boolean = ["info_is_rac"]
    float = ["info_uptime", "info_dbtime", "sql_exec"]

###############################################################################
#                            INPUT PLUGINS                                    #
###############################################################################
[[inputs.prometheus]]
  urls = ["http://ip:port/metrics"]    #change oracle exporter web.listen-address
  response_timeout = "150s"
```

```bash
/path/to/binary -configfile=oracle.conf -web.listen-address ip:port or
/path/to/binary -configfile=oracle_table.conf -web.listen-address ip:port
```

## Usage

```bash
Usage of ./telegraf_prometheus_oracle_exporter:
  -configfile string
    	ConfigurationFile in YAML format. (default "oracle.conf")
  -web.listen-address string
    	Address to listen on for web interface and telemetry. (default ":9161")
  -web.telemetry-path string
    	Path under which to expose metrics. (default "/metrics")
```