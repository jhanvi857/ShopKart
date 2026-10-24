package metrics

import (
	"database/sql"
	"net/http"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type Metrics struct {
	Registry              *prometheus.Registry
	serviceName           string
	instanceID            string
	environment           string
	RequestsTotal         *prometheus.CounterVec
	RequestDuration       *prometheus.HistogramVec
	RequestsInFlight      *prometheus.GaugeVec
	WorkerPoolUtilization *prometheus.GaugeVec
	DBPoolInUse           *prometheus.GaugeVec
	DBPoolWaitCount       *prometheus.GaugeVec
	DBPoolMaxOpen         *prometheus.GaugeVec
	CacheHitsTotal        *prometheus.CounterVec
	CacheMissesTotal      *prometheus.CounterVec
	QueueDepth            *prometheus.GaugeVec
	ConsumerLag           *prometheus.GaugeVec
}

func New(serviceName, instanceID, environment string) *Metrics {
	reg := prometheus.NewRegistry()

	// Register standard Go runtime and process metrics (CPU, memory, goroutines)
	reg.MustRegister(
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
	)

	constLabels := prometheus.Labels{
		"service":  serviceName,
		"instance": instanceID,
		"env":      environment,
	}

	requestsTotal := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name:        "http_requests_total",
			Help:        "Total number of HTTP requests processed.",
			ConstLabels: constLabels,
		},
		[]string{"method", "path", "status"},
	)

	requestDuration := prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:        "http_request_duration_seconds",
			Help:        "Latency histogram of HTTP requests.",
			ConstLabels: constLabels,
			Buckets:     []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10},
		},
		[]string{"method", "path", "status"},
	)

	requestsInFlight := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name:        "http_requests_in_flight",
			Help:        "Current number of in-flight HTTP requests.",
			ConstLabels: constLabels,
		},
		[]string{},
	)

	workerPoolUtilization := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name:        "worker_pool_utilization_ratio",
			Help:        "Ratio of active workers to total worker capacity (0.0 to 1.0).",
			ConstLabels: constLabels,
		},
		[]string{"pool_name"},
	)

	dbPoolInUse := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name:        "db_pool_connections_in_use",
			Help:        "Number of currently open database connections in use.",
			ConstLabels: constLabels,
		},
		[]string{"database"},
	)

	dbPoolWaitCount := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name:        "db_pool_connections_wait_count",
			Help:        "Total number of connections waited for due to pool saturation.",
			ConstLabels: constLabels,
		},
		[]string{"database"},
	)

	dbPoolMaxOpen := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name:        "db_pool_max_open_connections",
			Help:        "Maximum configured open connections in the pool.",
			ConstLabels: constLabels,
		},
		[]string{"database"},
	)

	cacheHitsTotal := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name:        "cache_hits_total",
			Help:        "Total number of cache hits.",
			ConstLabels: constLabels,
		},
		[]string{"cache"},
	)

	cacheMissesTotal := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name:        "cache_misses_total",
			Help:        "Total number of cache misses.",
			ConstLabels: constLabels,
		},
		[]string{"cache"},
	)

	queueDepth := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name:        "queue_depth",
			Help:        "Current queue depth or pending messages.",
			ConstLabels: constLabels,
		},
		[]string{"queue"},
	)

	consumerLag := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name:        "consumer_lag",
			Help:        "Current consumer lag in messages.",
			ConstLabels: constLabels,
		},
		[]string{"topic", "group"},
	)

	reg.MustRegister(
		requestsTotal,
		requestDuration,
		requestsInFlight,
		workerPoolUtilization,
		dbPoolInUse,
		dbPoolWaitCount,
		dbPoolMaxOpen,
		cacheHitsTotal,
		cacheMissesTotal,
		queueDepth,
		consumerLag,
	)

	return &Metrics{
		Registry:              reg,
		serviceName:           serviceName,
		instanceID:            instanceID,
		environment:           environment,
		RequestsTotal:         requestsTotal,
		RequestDuration:       requestDuration,
		RequestsInFlight:      requestsInFlight,
		WorkerPoolUtilization: workerPoolUtilization,
		DBPoolInUse:           dbPoolInUse,
		DBPoolWaitCount:       dbPoolWaitCount,
		DBPoolMaxOpen:         dbPoolMaxOpen,
		CacheHitsTotal:        cacheHitsTotal,
		CacheMissesTotal:      cacheMissesTotal,
		QueueDepth:            queueDepth,
		ConsumerLag:           consumerLag,
	}
}

func (m *Metrics) Handler() http.Handler {
	return promhttp.HandlerFor(m.Registry, promhttp.HandlerOpts{})
}

func (m *Metrics) TrackRequest(method, path string, statusCode int, duration time.Duration) {
	statusStr := strconv.Itoa(statusCode)
	m.RequestsTotal.WithLabelValues(method, path, statusStr).Inc()
	m.RequestDuration.WithLabelValues(method, path, statusStr).Observe(duration.Seconds())
}

func (m *Metrics) MonitorDB(db *sql.DB, dbName string, interval time.Duration, stopCh <-chan struct{}) {
	ticker := time.NewTicker(interval)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-stopCh:
				return
			case <-ticker.C:
				stats := db.Stats()
				m.DBPoolInUse.WithLabelValues(dbName).Set(float64(stats.InUse))
				m.DBPoolWaitCount.WithLabelValues(dbName).Set(float64(stats.WaitCount))
				m.DBPoolMaxOpen.WithLabelValues(dbName).Set(float64(stats.MaxOpenConnections))
			}
		}
	}()
}
