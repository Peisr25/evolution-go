package metrics

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	instance_repository "github.com/EvolutionAPI/evolution-go/pkg/instance/repository"
)

// Registry holds all Prometheus metrics for the application.
type Registry struct {
	reg          *prometheus.Registry
	httpRequests *prometheus.CounterVec
	httpDuration *prometheus.HistogramVec
}

// New creates a Registry and registers all metrics.
// version is embedded as a label in the build_info gauge.
func New(version string, instanceRepo instance_repository.InstanceRepository) *Registry {
	reg := prometheus.NewRegistry()

	httpRequests := prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "evolution_http_requests_total",
		Help: "Total number of HTTP requests partitioned by method, path and status code.",
	}, []string{"method", "path", "status"})

	httpDuration := prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "evolution_http_request_duration_seconds",
		Help:    "HTTP request latency in seconds partitioned by method and path.",
		Buckets: prometheus.DefBuckets,
	}, []string{"method", "path"})

	buildInfo := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "evolution_build_info",
		Help: "Build information. Always 1; use the 'version' label to read the value.",
	}, []string{"version"})
	buildInfo.WithLabelValues(version).Set(1)

	startTime := time.Now()
	uptimeGauge := prometheus.NewGaugeFunc(prometheus.GaugeOpts{
		Name: "evolution_uptime_seconds",
		Help: "Number of seconds since the server started.",
	}, func() float64 {
		return time.Since(startTime).Seconds()
	})

	reg.MustRegister(
		httpRequests,
		httpDuration,
		buildInfo,
		uptimeGauge,
		newInstanceCollector(instanceRepo),
	)

	return &Registry{
		reg:          reg,
		httpRequests: httpRequests,
		httpDuration: httpDuration,
	}
}

// Handler returns an http.Handler that serves the Prometheus metrics page.
func (r *Registry) Handler() http.Handler {
	return promhttp.HandlerFor(r.reg, promhttp.HandlerOpts{})
}

// GinMiddleware returns a Gin middleware that records HTTP request counts and latencies.
// The /metrics path itself is excluded to avoid self-referential noise.
func (r *Registry) GinMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.URL.Path == "/metrics" {
			c.Next()
			return
		}

		start := time.Now()
		c.Next()

		// c.FullPath() returns the registered route pattern (e.g. /instance/:instanceId)
		// which keeps cardinality bounded regardless of how many distinct IDs are used.
		path := c.FullPath()
		if path == "" {
			path = "unmatched"
		}

		r.httpRequests.WithLabelValues(
			c.Request.Method,
			path,
			strconv.Itoa(c.Writer.Status()),
		).Inc()

		r.httpDuration.WithLabelValues(c.Request.Method, path).Observe(time.Since(start).Seconds())
	}
}

// instanceCollector is a custom prometheus.Collector that queries the database at
// scrape time so the gauge values are always current without requiring event hooks.
type instanceCollector struct {
	repo             instance_repository.InstanceRepository
	descTotal        *prometheus.Desc
	descConnected    *prometheus.Desc
	descDisconnected *prometheus.Desc
}

func newInstanceCollector(repo instance_repository.InstanceRepository) prometheus.Collector {
	return &instanceCollector{
		repo: repo,
		descTotal: prometheus.NewDesc(
			"evolution_instances_total",
			"Total number of registered instances.",
			nil, nil,
		),
		descConnected: prometheus.NewDesc(
			"evolution_instances_connected",
			"Number of instances currently connected to WhatsApp.",
			nil, nil,
		),
		descDisconnected: prometheus.NewDesc(
			"evolution_instances_disconnected",
			"Number of instances currently disconnected from WhatsApp.",
			nil, nil,
		),
	}
}

func (c *instanceCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.descTotal
	ch <- c.descConnected
	ch <- c.descDisconnected
}

func (c *instanceCollector) Collect(ch chan<- prometheus.Metric) {
	instances, err := c.repo.GetAllInstances()
	if err != nil {
		// Emit nothing on error rather than stale data.
		return
	}

	var connected float64
	for _, inst := range instances {
		if inst.Connected {
			connected++
		}
	}
	total := float64(len(instances))

	ch <- prometheus.MustNewConstMetric(c.descTotal, prometheus.GaugeValue, total)
	ch <- prometheus.MustNewConstMetric(c.descConnected, prometheus.GaugeValue, connected)
	ch <- prometheus.MustNewConstMetric(c.descDisconnected, prometheus.GaugeValue, total-connected)
}
