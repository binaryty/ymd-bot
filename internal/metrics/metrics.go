package metrics

import (
	"net/http"
	"sync"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

const namespace = "ym_bot"

// Metrics holds application counters and gauges.
type Metrics struct {
	// DownloadsTotal counts successful track downloads (sent to user).
	DownloadsTotal prometheus.Counter
	// DownloadsFailed counts failed download attempts.
	DownloadsFailed prometheus.Counter
	// SearchesTotal counts inline search requests.
	SearchesTotal prometheus.Counter
	// SearchesFailed counts failed search requests.
	SearchesFailed prometheus.Counter
	// InlineResultsTotal counts tracks returned in inline query results.
	InlineResultsTotal prometheus.Counter
	// StreamURLRequests counts StreamURL (get direct URL) calls.
	StreamURLRequests prometheus.Counter
	// StreamURLFailed counts StreamURL failures (e.g. no direct URL).
	StreamURLFailed prometheus.Counter

	// ChartsRequests counts inline feature usages (trending/new).
	ChartsRequests *prometheus.CounterVec

	reg *prometheus.Registry
}

var (
	instance *Metrics
	once     sync.Once
)

// New creates and registers application metrics. Safe to call multiple times; returns same instance.
func New() *Metrics {
	once.Do(func() {
		reg := prometheus.NewRegistry()
		instance = &Metrics{
			reg: reg,
			DownloadsTotal: prometheus.NewCounter(prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "downloads_total",
				Help:      "Total number of tracks successfully downloaded and sent to user",
			}),
			DownloadsFailed: prometheus.NewCounter(prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "downloads_failed_total",
				Help:      "Total number of failed download/send attempts",
			}),
			SearchesTotal: prometheus.NewCounter(prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "searches_total",
				Help:      "Total number of inline search requests",
			}),
			SearchesFailed: prometheus.NewCounter(prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "searches_failed_total",
				Help:      "Total number of failed search requests",
			}),
			InlineResultsTotal: prometheus.NewCounter(prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "inline_results_total",
				Help:      "Total number of track results returned in inline queries",
			}),
			StreamURLRequests: prometheus.NewCounter(prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "stream_url_requests_total",
				Help:      "Total number of StreamURL (direct URL) requests",
			}),
			StreamURLFailed: prometheus.NewCounter(prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "stream_url_failed_total",
				Help:      "Total number of StreamURL failures",
			}),
			ChartsRequests: prometheus.NewCounterVec(prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "inline_features_total",
				Help:      "Total number of inline feature requests by type",
			}),
		}
		reg.MustRegister(
			instance.DownloadsTotal,
			instance.DownloadsFailed,
			instance.SearchesTotal,
			instance.SearchesFailed,
			instance.InlineResultsTotal,
			instance.StreamURLRequests,
			instance.StreamURLFailed,
			instance.ChartsRequests,
		)
	})
	return instance
}

// Handler returns HTTP handler for Prometheus scrape (GET /metrics).
func (m *Metrics) Handler() http.Handler {
	return promhttp.HandlerFor(m.reg, promhttp.HandlerOpts{})
}
