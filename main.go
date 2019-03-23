package main

import (
	"database/sql"
	"flag"
	"net/http"
	"strings"
	"time"

	// _"github.com/mattn/go-oci8" // linux
	_ "github.com/wendal/go-oci8" // windows

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/log"
)

var (
	// Version will be set at build time.
	Version       = "0.0.0.dev"
	listenAddress = flag.String("web.listen-address", ":9161", "Address to listen on for web interface and telemetry.")
	metricPath    = flag.String("web.telemetry-path", "/metrics", "Path under which to expose metrics.")
	configFile    = flag.String("configfile", "oracle.conf", "ConfigurationFile in YAML format.")
	landingPage   = []byte("<html><head><title>Oracle DB Exporter " + Version + "</title></head><body><h1>Oracle DB Exporter " + Version + "</h1><p><a href='" + *metricPath + "'>Metrics</a></p></body></html>")
)

// Metric name parts.
const (
	namespace = "oracledb"
	exporter  = "exporter"
)

// Exporter collects Oracle DB metrics. It implements prometheus.Collector.
type Exporter struct {
	duration, error prometheus.Gauge
	totalScrapes    prometheus.Counter
	scrapeErrors    *prometheus.CounterVec
	up              *prometheus.GaugeVec
}

// NewExporter returns a new Oracle DB exporter for the provided DSN.
func NewExporter() *Exporter {
	return &Exporter{
		duration: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Subsystem: exporter,
			Name:      "last_scrape_duration_seconds",
			Help:      "Duration of the last scrape of metrics from Oracle DB.",
		}),
		totalScrapes: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: exporter,
			Name:      "scrapes_total",
			Help:      "Total number of times Oracle DB was scraped for metrics.",
		}),
		scrapeErrors: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: exporter,
			Name:      "scrape_errors_total",
			Help:      "Total number of times an error occured scraping a Oracle database.",
		}, []string{"collector"}),
		error: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Subsystem: exporter,
			Name:      "last_scrape_error",
			Help:      "Whether the last scrape of metrics from Oracle DB resulted in an error (1 for error, 0 for success).",
		}),
		up: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "up",
			Help:      "Whether the Oracle database server is up.",
		}, []string{"database", "dbinstance", "id"}),
	}
}

// Describe describes all the metrics exported by the MS SQL exporter.
func (e *Exporter) Describe(ch chan<- *prometheus.Desc) {
	// We cannot know in advance what metrics the exporter will generate
	// So we use the poor man's describe method: Run a collect
	// and send the descriptors of all the collected metrics. The problem
	// here is that we need to connect to the Oracle DB. If it is currently
	// unavailable, the descriptors will be incomplete. Since this is a
	// stand-alone exporter and not used as a library within other code
	// implementing additional metrics, the worst that can happen is that we
	// don't detect inconsistent metrics created by this exporter
	// itself. Also, a change in the monitored Oracle instance may change the
	// exported metrics during the runtime of the exporter.

	metricCh := make(chan prometheus.Metric)
	doneCh := make(chan struct{})

	go func() {
		for m := range metricCh {
			ch <- m.Desc()
		}
		close(doneCh)
	}()

	e.Collect(metricCh)
	close(metricCh)
	<-doneCh

}

// Collect implements prometheus.Collector.
func (e *Exporter) Collect(ch chan<- prometheus.Metric) {
	for _, conf := range config.Cfgs {
		e.scrape(ch, conf)
		ch <- e.duration
		ch <- e.totalScrapes
		ch <- e.error
		e.scrapeErrors.Collect(ch)
		e.up.Collect(ch)
	}
}

func (e *Exporter) scrape(ch chan<- prometheus.Metric, conf Config) {
	e.totalScrapes.Inc()
	var err error
	defer func(begun time.Time) {
		e.duration.Set(time.Since(begun).Seconds())
		if err == nil {
			e.error.Set(0)
		} else {
			e.error.Set(1)
		}
	}(time.Now())

	db, err := sql.Open("oci8", conf.Connection)
	if err != nil {
		log.Errorln("Error opening connection to database:", err)
		return
	}
	defer db.Close()

	isUpRows, err := db.Query("SELECT 1 FROM DUAL")
	if err != nil {
		log.Errorln("Error pinging oracle:", err)
		e.up.WithLabelValues(conf.Database, conf.Instance, conf.Id).Set(0)
		return
	}
	isUpRows.Close()
	e.up.WithLabelValues(conf.Database, conf.Instance, conf.Id).Set(1)

	if err = ScrapeActivity(db, conf, ch); err != nil {
		log.Errorln("Error scraping for activity:", err)
		e.scrapeErrors.WithLabelValues("activity").Inc()
	}

	if err = ScrapeTablespace(db, conf, ch); err != nil {
		log.Errorln("Error scraping for tablespace:", err)
		e.scrapeErrors.WithLabelValues("tablespace").Inc()
	}

	if err = ScrapeWaitTime(db, conf, ch); err != nil {
		log.Errorln("Error scraping for wait_time:", err)
		e.scrapeErrors.WithLabelValues("wait_time").Inc()
	}

	if err = ScrapeSessions(db, conf, ch); err != nil {
		log.Errorln("Error scraping for sessions:", err)
		e.scrapeErrors.WithLabelValues("sessions").Inc()
	}

	if err = ScrapeProcesses(db, conf, ch); err != nil {
		log.Errorln("Error scraping for process:", err)
		e.scrapeErrors.WithLabelValues("process").Inc()
	}

	if err = ScrapeUptime(db, conf, ch); err != nil {
		log.Errorln("Error scraping for uptime:", err)
		e.scrapeErrors.WithLabelValues("uptime").Inc()
	}

	if err = ScrapeTopSQL(db, conf, ch); err != nil {
		log.Errorln("Error scraping for topSQL:", err)
		e.scrapeErrors.WithLabelValues("topSQL").Inc()
	}

	if err = ScrapeAsmspace(db, conf, ch); err != nil {
		log.Errorln("Error scraping for asmspace:", err)
		e.scrapeErrors.WithLabelValues("asmspace").Inc()
	}

	if err = ScrapeIOPS(db, conf, ch); err != nil {
		log.Errorln("Error scraping for iops:", err)
		e.scrapeErrors.WithLabelValues("iops").Inc()
	}

	if err = ScrapeCache(db, conf, ch); err != nil {
		log.Errorln("Error scraping for cache:", err)
		e.scrapeErrors.WithLabelValues("cache").Inc()
	}

}

// ScrapeProcesses gets information about the currently active processes.

func ScrapeProcesses(db *sql.DB, conf Config, ch chan<- prometheus.Metric) error {

	var count float64
	err := db.QueryRow("SELECT COUNT(*) FROM v$process").Scan(&count)
	if err != nil {
		return err
	}

	ch <- prometheus.MustNewConstMetric(
		prometheus.NewDesc(prometheus.BuildFQName(namespace, "process", "count"),
			"Gauge metric with count of processes", []string{"database", "dbinstance", "id"}, nil),
		prometheus.GaugeValue,
		count,
		conf.Database,
		conf.Instance,
		conf.Id,
	)

	return nil

}

func ScrapeTopSQL(db *sql.DB, conf Config, ch chan<- prometheus.Metric) error {
	/*	var (
			rows *sql.Rows
			err  error
		)

		rows, err = db.Query(`
	SELECT * FROM (
	  SELECT SQL_TEXT,X.ELAPSED_TIME_DELTA,X.EXECUTIONS_DELTA,X.DISK_READS_DELTA,X.DIRECT_WRITES_DELTA,X.IOWAIT_DELTA,X.LOADS_DELTA
		FROM DBA_HIST_SQLTEXT DHST,
		(
	      SELECT DHSS.SQL_ID SQL_ID,
	    	SUM(LOADS_DELTA) LOADS_DELTA,
	    	SUM(DHSS.EXECUTIONS_DELTA) EXECUTIONS_DELTA,
	    	SUM(DHSS.IOWAIT_DELTA) IOWAIT_DELTA,
	    	SUM(DHSS.ELAPSED_TIME_DELTA) ELAPSED_TIME_DELTA,
	    	SUM(DHSS.DISK_READS_DELTA) DISK_READS_DELTA,
	    	SUM(DHSS.DIRECT_WRITES_DELTA) DIRECT_WRITES_DELTA
	      	  FROM DBA_HIST_SQLSTAT DHSS
	          	GROUP BY DHSS.SQL_ID
		)X
	  	  WHERE COMMAND_TYPE != 47
			ORDER BY X.ELAPSED_TIME_DELTA DESC
	)
	where rownum < 6
	`)

		if err != nil {
			return err
		}
		defer rows.Close()

		for rows.Next() {
			var (
				sql      string
				elapsedTime   float64
				executions    float64
				diskReads     float64
				directWrites  float64
				ioWait  	  float64
				loads   	  float64

			)
			if err := rows.Scan(&sql, &elapsedTime, &executions, &diskReads, &directWrites, &ioWait, &loads); err != nil {
				return err
			}
			ch <- prometheus.MustNewConstMetric(
				prometheus.NewDesc(prometheus.BuildFQName(namespace, "sql", "top"),
					"Gauge metric with count of sessions by status and type", []string{"database", "dbinstance", "id", "sql", "loads", "ioWait", "executions", "diskReads", "directWrites"}, nil),
				prometheus.GaugeValue,
				elapsedTime,
				conf.Database,
				conf.Instance,
				conf.Id,
				sql,
				strconv.FormatFloat(float64(loads), 'f', 6, 64),
				strconv.FormatFloat(float64(ioWait), 'f', 6, 64),
				strconv.FormatFloat(float64(executions), 'f', 6, 64),
				strconv.FormatFloat(float64(diskReads), 'f', 6, 64),
				strconv.FormatFloat(float64(directWrites), 'f', 6, 64),
			)
		}*/

	return nil

}

// ScrapeAsmspace collects ASM metrics
func ScrapeAsmspace(db *sql.DB, conf Config, ch chan<- prometheus.Metric) error {
	/*	var (
			rows *sql.Rows
			err  error
		)

		rows, err = db.Query(`SELECT g.name, sum(d.total_mb), sum(d.free_mb)
	                                  FROM v$asm_disk d, v$asm_diskgroup g
	                                 WHERE  d.group_number = g.group_number
	                                  AND  d.header_status = 'MEMBER'
	                                 GROUP by  g.name,  g.group_number`)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var name string
			var tsize float64
			var tfree float64
			if err := rows.Scan(&name, &tsize, &tfree); err != nil {
				return err
			}

			ch <- prometheus.MustNewConstMetric(
				prometheus.NewDesc(prometheus.BuildFQName(namespace, "asm", "total"),
					"Gauge metric with total/free size of the ASM Diskgroups.", []string{"database", "dbinstance", "id", "name"}, nil),
				prometheus.GaugeValue,
				tsize,
				conf.Database,
				conf.Instance,
				conf.Id,
				name,
			)

			ch <- prometheus.MustNewConstMetric(
				prometheus.NewDesc(prometheus.BuildFQName(namespace, "asm", "free"),
					"Gauge metric with total/free size of the ASM Diskgroups.", []string{"database", "dbinstance", "id", "name"}, nil),
				prometheus.GaugeValue,
				tfree,
				conf.Database,
				conf.Instance,
				conf.Id,
				name,
			)

			ch <- prometheus.MustNewConstMetric(
				prometheus.NewDesc(prometheus.BuildFQName(namespace, "asm", "used"),
					"Gauge metric with total/free size of the ASM Diskgroups.", []string{"database", "dbinstance", "id", "name"}, nil),
				prometheus.GaugeValue,
				tsize-tfree,
				conf.Database,
				conf.Instance,
				conf.Id,
				name,
			)
		}*/
	return nil
}

// ScrapeUptime Instance uptime
func ScrapeUptime(db *sql.DB, conf Config, ch chan<- prometheus.Metric) error {

	/*	var uptime float64
		err := db.QueryRow("select sysdate-startup_time from v$instance").Scan(&uptime)
		if err != nil {
			return err
		}

		ch <- prometheus.MustNewConstMetric(
			prometheus.NewDesc(prometheus.BuildFQName(namespace, "up", "time"),
				"Gauge metric with uptime in days of the Instance.", []string{"database", "dbinstance", "id"}, nil),
			prometheus.GaugeValue,
			uptime,
			conf.Database,
			conf.Instance,
			conf.Id,
		)*/

	return nil
}

// ScrapeSessions collects session metrics from the v$session view.
func ScrapeSessions(db *sql.DB, conf Config, ch chan<- prometheus.Metric) error {
	var (
		rows *sql.Rows
		err  error
	)
	// Retrieve status and type for all sessions.
	rows, err = db.Query("SELECT status, type, COUNT(*) FROM v$session GROUP BY status, type")
	if err != nil {
		return err
	}

	defer rows.Close()
	activeCount := 0.
	inactiveCount := 0.
	for rows.Next() {
		var (
			status      string
			sessionType string
			count       float64
		)
		if err := rows.Scan(&status, &sessionType, &count); err != nil {
			return err
		}
		ch <- prometheus.MustNewConstMetric(
			prometheus.NewDesc(prometheus.BuildFQName(namespace, "sessions", "activity"),
				"Gauge metric with count of sessions by status and type", []string{"database", "dbinstance", "status", "type", "id"}, nil),
			prometheus.GaugeValue,
			count,
			conf.Database,
			conf.Instance,
			status,
			sessionType,
			conf.Id,
		)

		// These metrics are deprecated though so as to not break existing monitoring straight away, are included for the next few releases.
		if status == "ACTIVE" {
			activeCount += count
		}

		if status == "INACTIVE" {
			inactiveCount += count
		}
	}

	ch <- prometheus.MustNewConstMetric(
		prometheus.NewDesc(prometheus.BuildFQName(namespace, "sessions", "active"),
			"Gauge metric with count of sessions marked ACTIVE. DEPRECATED: use sum(oracledb_sessions_activity{status='ACTIVE}) instead.", []string{"database", "dbinstance", "id"}, nil),
		prometheus.GaugeValue,
		activeCount,
		conf.Database,
		conf.Instance,
		conf.Id,
	)
	ch <- prometheus.MustNewConstMetric(
		prometheus.NewDesc(prometheus.BuildFQName(namespace, "sessions", "inactive"),
			"Gauge metric with count of sessions marked INACTIVE. DEPRECATED: use sum(oracledb_sessions_activity{status='INACTIVE'}) instead.", []string{"database", "dbinstance", "id"}, nil),
		prometheus.GaugeValue,
		inactiveCount,
		conf.Database,
		conf.Instance,
		conf.Id,
	)
	return nil
}

// ScrapeWaitTime collects wait time metrics from the v$waitclassmetric view.
func ScrapeWaitTime(db *sql.DB, conf Config, ch chan<- prometheus.Metric) error {
	var (
		rows *sql.Rows
		err  error
	)
	rows, err = db.Query("SELECT n.wait_class, round(m.time_waited/m.INTSIZE_CSEC,3) AAS from v$waitclassmetric  m, v$system_wait_class n where m.wait_class_id=n.wait_class_id and n.wait_class != 'Idle'")
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var name string
		var value float64
		if err := rows.Scan(&name, &value); err != nil {
			return err
		}
		name = cleanName(name)
		ch <- prometheus.MustNewConstMetric(
			prometheus.NewDesc(prometheus.BuildFQName(namespace, "wait_class", "time"),
				"Generic counter metric from v$waitclassmetric view in Oracle.", []string{"database", "dbinstance", "id", "type"}, nil),
			prometheus.CounterValue,
			value,
			conf.Database,
			conf.Instance,
			conf.Id,
			name,
		)
	}
	return nil
}

// ScrapeActivity collects activity metrics from the v$sysstat view.
func ScrapeActivity(db *sql.DB, conf Config, ch chan<- prometheus.Metric) error {
	var (
		rows *sql.Rows
		err  error
	)
	rows, err = db.Query(`
SELECT name, value 
  FROM v$sysstat 
  	WHERE name IN ('parse count (total)', 'parse time cpu', 'parse time elapsed', 'parse count (hard)', 'execute count', 'opened cursors current', 'session cursor cache count', 'user commits', 'user rollbacks', 'user calls', 'transaction rollbacks', 'redo size', 'logons current',
  	  'physical reads', 'physical writes', 'physical reads cache', 'db block gets', 'consistent gets', 'lob reads', 'lob writes',
	  'bytes received via SQL*Net from client', 'bytes sent via SQL*Net to client',
	  'index fast full scans (full)'
  	)
`)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var name string
		var value float64
		if err := rows.Scan(&name, &value); err != nil {
			return err
		}
		name = cleanName(name)

		var subsystem string
		var dbBlockField = "physical_reads,physical_writes,physical_reads_cache,db_block_gets,consistent_gets,lob_reads,lob_writes"
		var netTransfer = "bytes_received_via_sql*net_from_client,bytes_sent_via_sql*net_to_client"

		if name == "index_fast_full_scans_full" {
			subsystem = "index"
		} else if strings.Contains(dbBlockField, name) {
			subsystem = "block"
		} else if strings.Contains(netTransfer, name) {
			subsystem = "net"
			name = strings.Split(name, "_")[0] + strings.Split(name, "_")[1]
		} else {
			subsystem = "activity"
		}

		if subsystem == "activity" && strings.Contains(name, "user") {
			ch <- prometheus.MustNewConstMetric(
				prometheus.NewDesc(prometheus.BuildFQName(namespace, subsystem, "user"),
					"Generic counter metric from v$sysstat view in Oracle.", []string{"database", "dbinstance", "id", "type"}, nil),
				prometheus.CounterValue,
				value,
				conf.Database,
				conf.Instance,
				conf.Id,
				name,
			)
		} else {
			ch <- prometheus.MustNewConstMetric(
				prometheus.NewDesc(prometheus.BuildFQName(namespace, subsystem, name),
					"Generic counter metric from v$sysstat view in Oracle.", []string{"database", "dbinstance", "id"}, nil),
				prometheus.CounterValue,
				value,
				conf.Database,
				conf.Instance,
				conf.Id,
			)
		}
	}
	return nil
}

// ScrapeTablespace collects tablespace size.
func ScrapeTablespace(db *sql.DB, conf Config, ch chan<- prometheus.Metric) error {
	var (
		rows *sql.Rows
		err  error
	)
	rows, err = db.Query(`
SELECT
  Z.name,
  dt.status,
  dt.contents,
  dt.extent_management,
  Z.bytes,
  Z.max_bytes,
  Z.free_bytes
FROM
(
  SELECT
    X.name                   as name,
    SUM(nvl(X.free_bytes,0)) as free_bytes,
    SUM(X.bytes)             as bytes,
    SUM(X.max_bytes)         as max_bytes
  FROM
    (
      SELECT
        ddf.tablespace_name as name,
        ddf.status as status,
        ddf.bytes as bytes,
        sum(coalesce(dfs.bytes, 0)) as free_bytes,
        CASE
          WHEN ddf.maxbytes = 0 THEN ddf.bytes
          ELSE ddf.maxbytes
        END as max_bytes
      FROM
        sys.dba_data_files ddf,
        sys.dba_tablespaces dt,
        sys.dba_free_space dfs
      WHERE ddf.tablespace_name = dt.tablespace_name
      AND ddf.file_id = dfs.file_id(+)
      GROUP BY
        ddf.tablespace_name,
        ddf.file_name,
        ddf.status,
        ddf.bytes,
        ddf.maxbytes
    ) X
  GROUP BY X.name
  UNION ALL
  SELECT
    Y.name                   as name,
    MAX(nvl(Y.free_bytes,0)) as free_bytes,
    SUM(Y.bytes)             as bytes,
    SUM(Y.max_bytes)         as max_bytes
  FROM
    (
      SELECT
        dtf.tablespace_name as name,
        dtf.status as status,
        dtf.bytes as bytes,
        (
          SELECT
            ((f.total_blocks - s.tot_used_blocks)*vp.value)
          FROM
            (SELECT tablespace_name, sum(used_blocks) tot_used_blocks FROM gv$sort_segment WHERE  tablespace_name!='DUMMY' GROUP BY tablespace_name) s,
            (SELECT tablespace_name, sum(blocks) total_blocks FROM dba_temp_files where tablespace_name !='DUMMY' GROUP BY tablespace_name) f,
            (SELECT value FROM v$parameter WHERE name = 'db_block_size') vp
          WHERE f.tablespace_name=s.tablespace_name AND f.tablespace_name = dtf.tablespace_name
        ) as free_bytes,
        CASE
          WHEN dtf.maxbytes = 0 THEN dtf.bytes
          ELSE dtf.maxbytes
        END as max_bytes
      FROM
        sys.dba_temp_files dtf
    ) Y
  GROUP BY Y.name
) Z, sys.dba_tablespaces dt
WHERE
  Z.name = dt.tablespace_name
`)
	if err != nil {
		return err
	}
	defer rows.Close()
	tablespaceBytesDesc := prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "tablespace", "bytes"),
		"Generic counter metric of tablespaces bytes in Oracle.",
		[]string{"database", "dbinstance", "tablespace", "type", "id"}, nil,
	)
	tablespaceMaxBytesDesc := prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "tablespace", "max_bytes"),
		"Generic counter metric of tablespaces max bytes in Oracle.",
		[]string{"database", "dbinstance", "tablespace", "type", "id"}, nil,
	)
	tablespaceFreeBytesDesc := prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "tablespace", "free"),
		"Generic counter metric of tablespaces free bytes in Oracle.",
		[]string{"database", "dbinstance", "tablespace", "type", "id"}, nil,
	)

	for rows.Next() {
		var tablespace_name string
		var status string
		var contents string
		var extent_management string
		var bytes float64
		var max_bytes float64
		var bytes_free float64

		if err := rows.Scan(&tablespace_name, &status, &contents, &extent_management, &bytes, &max_bytes, &bytes_free); err != nil {
			return err
		}
		ch <- prometheus.MustNewConstMetric(tablespaceBytesDesc, prometheus.GaugeValue, float64(bytes), conf.Database, conf.Instance, tablespace_name, contents, conf.Id)
		ch <- prometheus.MustNewConstMetric(tablespaceMaxBytesDesc, prometheus.GaugeValue, float64(max_bytes), conf.Database, conf.Instance, tablespace_name, contents, conf.Id)
		ch <- prometheus.MustNewConstMetric(tablespaceFreeBytesDesc, prometheus.GaugeValue, float64(bytes_free), conf.Database, conf.Instance, tablespace_name, contents, conf.Id)
	}
	return nil
}

// ScrapeIOPS collects IOPS metrics from the v$sysmetrics view.
func ScrapeIOPS(db *sql.DB, conf Config, ch chan<- prometheus.Metric) error {
	var (
		rows *sql.Rows
		err  error
	)

	//metric_id  metric_name
	//2092    Physical Read Total IO Requests Per Sec
	//2093    Physical Read Total Bytes Per Sec
	//2100    Physical Write Total IO Requests Per Sec
	//2124    Physical Write Total Bytes Per Sec
	//2106    SQL Service Response Time
	rows, err = db.Query("select metric_name,value from v$sysmetric where metric_id in (2092,2093,2124,2100,2106)")
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var name string
		var value float64
		if err := rows.Scan(&name, &value); err != nil {
			break
		}
		name = cleanName(name)

		if name == "sql_service_response_time" {
			ch <- prometheus.MustNewConstMetric(
				prometheus.NewDesc(prometheus.BuildFQName(namespace, "response", name),
					"Generic counter metric from v$sysmetric view in Oracle.", []string{"database", "dbinstance", "id"}, nil),
				prometheus.CounterValue,
				value,
				conf.Database,
				conf.Instance,
				conf.Id,
			)
		} else {
			var n string
			if strings.Contains(name, "requests") {
				n = "iops"
			} else {
				n = "throughput"
			}

			ch <- prometheus.MustNewConstMetric(
				prometheus.NewDesc(prometheus.BuildFQName(namespace, "physical", n),
					"Generic counter metric from v$sysmetric view in Oracle.", []string{"database", "dbinstance", "id", "type"}, nil),
				prometheus.CounterValue,
				value,
				conf.Database,
				conf.Instance,
				conf.Id,
				name,
			)
		}
	}
	return nil
}

// ScrapeCache collects session metrics from the v$sysmetrics view.
func ScrapeCache(db *sql.DB, conf Config, ch chan<- prometheus.Metric) error {
	var (
		rows *sql.Rows
		err  error
	)
	//metric_id  metric_name
	//2000    Buffer Cache Hit Ratio
	//2050    Cursor Cache Hit Ratio
	//2112    Library Cache Hit Ratio
	//2110    Row Cache Hit Ratio
	rows, err = db.Query(`select metric_name,value from v$sysmetric where group_id=2 and metric_id in (2000,2050,2112,2110)`)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var name string
		var value float64
		if err := rows.Scan(&name, &value); err != nil {
			break
		}
		name = cleanName(name)
		ch <- prometheus.MustNewConstMetric(
			prometheus.NewDesc(prometheus.BuildFQName(namespace, "cache", "hitratio"),
				"Gauge metric witch Cache hit ratios (v$sysmetric).", []string{"database", "dbinstance", "id", "type"}, nil),
			prometheus.CounterValue,
			value,
			conf.Database,
			conf.Instance,
			conf.Id,
			name,
		)
	}
	return nil
}

// Oracle gives us some ugly names back. This function cleans things up for Prometheus.
func cleanName(s string) string {
	s = strings.Replace(s, " ", "_", -1) // Remove spaces
	s = strings.Replace(s, "(", "", -1)  // Remove open parenthesis
	s = strings.Replace(s, ")", "", -1)  // Remove close parenthesis
	s = strings.Replace(s, "/", "", -1)  // Remove forward slashes
	s = strings.ToLower(s)
	return s
}

func main() {
	flag.Parse()
	log.Infoln("Starting oracledb_exporter " + Version)
	// dsn := os.Getenv("DATA_SOURCE_NAME")
	// dsn := "infodba/infodba@192.168.1.64:1521/tc"
	if loadConfig() {
		exporter := NewExporter()
		prometheus.MustRegister(exporter)
		http.Handle(*metricPath, prometheus.Handler())
		http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			w.Write(landingPage)
		})
		log.Infoln("Listening on", *listenAddress)
		log.Fatal(http.ListenAndServe(*listenAddress, nil))
	}
}
