// Package metrics provides Prometheus metrics collection.
package metrics

import (
	"github.com/ilaziness/app-tpl/internal/config"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// Metrics wraps Prometheus metrics collectors.
type Metrics struct {
	registry *prometheus.Registry

	// HTTP metrics
	HTTPRequestsTotal   *prometheus.CounterVec
	HTTPRequestDuration *prometheus.HistogramVec
	HTTPInFlight        prometheus.Gauge

	// Database metrics
	DBConnectionsActive prometheus.Gauge
	DBConnectionsIdle   prometheus.Gauge
	DBQueryDuration     *prometheus.HistogramVec

	// Redis metrics
	RedisCacheHits         *prometheus.CounterVec
	RedisCacheMisses       *prometheus.CounterVec
	RedisOperationDuration *prometheus.HistogramVec

	// TCP metrics
	TCPConnections   prometheus.Gauge
	TCPBytesReceived *prometheus.CounterVec
	TCPBytesSent     *prometheus.CounterVec

	// UDP metrics
	UDPPacketsReceived *prometheus.CounterVec
	UDPPacketsSent     *prometheus.CounterVec
}

// NewMetrics creates a new metrics instance based on configuration.
// If metrics is disabled, returns an empty metrics instance.
func NewMetrics(cfg *config.Config) *Metrics {
	if !cfg.Metrics.Enabled {
		return &Metrics{}
	}

	registry := prometheus.NewRegistry()

	// Register default collectors (Go runtime, process)
	registry.MustRegister(collectors.NewGoCollector())
	registry.MustRegister(collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}))

	m := &Metrics{
		registry: registry,

		// HTTP metrics
		HTTPRequestsTotal: promauto.With(registry).NewCounterVec(
			prometheus.CounterOpts{
				Name: "http_requests_total",
				Help: "Total number of HTTP requests",
			},
			[]string{"method", "path", "status"},
		),
		HTTPRequestDuration: promauto.With(registry).NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "http_request_duration_seconds",
				Help:    "HTTP request duration in seconds",
				Buckets: prometheus.DefBuckets,
			},
			[]string{"method", "path"},
		),
		HTTPInFlight: promauto.With(registry).NewGauge(
			prometheus.GaugeOpts{
				Name: "http_in_flight_requests",
				Help: "Number of in-flight HTTP requests",
			},
		),

		// Database metrics
		DBConnectionsActive: promauto.With(registry).NewGauge(
			prometheus.GaugeOpts{
				Name: "db_connections_active",
				Help: "Number of active database connections",
			},
		),
		DBConnectionsIdle: promauto.With(registry).NewGauge(
			prometheus.GaugeOpts{
				Name: "db_connections_idle",
				Help: "Number of idle database connections",
			},
		),
		DBQueryDuration: promauto.With(registry).NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "db_query_duration_seconds",
				Help:    "Database query duration in seconds",
				Buckets: prometheus.DefBuckets,
			},
			[]string{"operation", "table"},
		),

		// Redis metrics
		RedisCacheHits: promauto.With(registry).NewCounterVec(
			prometheus.CounterOpts{
				Name: "redis_cache_hits_total",
				Help: "Total number of Redis cache hits",
			},
			[]string{"key_prefix"},
		),
		RedisCacheMisses: promauto.With(registry).NewCounterVec(
			prometheus.CounterOpts{
				Name: "redis_cache_misses_total",
				Help: "Total number of Redis cache misses",
			},
			[]string{"key_prefix"},
		),
		RedisOperationDuration: promauto.With(registry).NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "redis_operation_duration_seconds",
				Help:    "Redis operation duration in seconds",
				Buckets: prometheus.DefBuckets,
			},
			[]string{"operation"},
		),

		// TCP metrics
		TCPConnections: promauto.With(registry).NewGauge(
			prometheus.GaugeOpts{
				Name: "tcp_connections",
				Help: "Number of active TCP connections",
			},
		),
		TCPBytesReceived: promauto.With(registry).NewCounterVec(
			prometheus.CounterOpts{
				Name: "tcp_bytes_received_total",
				Help: "Total bytes received via TCP",
			},
			[]string{"remote_addr"},
		),
		TCPBytesSent: promauto.With(registry).NewCounterVec(
			prometheus.CounterOpts{
				Name: "tcp_bytes_sent_total",
				Help: "Total bytes sent via TCP",
			},
			[]string{"remote_addr"},
		),

		// UDP metrics
		UDPPacketsReceived: promauto.With(registry).NewCounterVec(
			prometheus.CounterOpts{
				Name: "udp_packets_received_total",
				Help: "Total UDP packets received",
			},
			[]string{"remote_addr"},
		),
		UDPPacketsSent: promauto.With(registry).NewCounterVec(
			prometheus.CounterOpts{
				Name: "udp_packets_sent_total",
				Help: "Total UDP packets sent",
			},
			[]string{"remote_addr"},
		),
	}

	return m
}

// Registry returns the Prometheus registry.
func (m *Metrics) Registry() *prometheus.Registry {
	return m.registry
}
