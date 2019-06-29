# Oracle DB Exporter
merge：https://github.com/iamseth/oracledb_exporter && https://github.com/freenetdigital/prometheus_oracle_exporter
Binary Release
-
```
oracledb_exporter.exe已是最新执行文件
```
config
-
```
常见metric请使用oracle.conf
top_table 的metric获取比较耗时，需要与其他metric配置区分，请使用oracle_table.conf
```
build
-
```
go build 默认构建成exe
之前尝试过发布成Linux 64位，但是在运行时出现许多问题，暂时只能在windows下运行
```
run
-
```
oracledb_exporter -configfile=oracle.conf -web.listen-address ip:port
例如：192.168.1.64上启动
oracledb_exporter -configfile=oracle.conf -web.listen-address 192.168.1.64:9162
oracledb_exporter -configfile=oracle_table.conf -web.listen-address 192.168.1.64:9161
```