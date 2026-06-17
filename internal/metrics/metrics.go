package metrics

import (
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	ctrlmetrics "sigs.k8s.io/controller-runtime/pkg/metrics"
)

var (
	// reconcileTotal tracks total number of reconciliations by kind and result
	reconcileTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "hcloud_reconcile_total",
			Help: "Total number of reconciliations by kind and result",
		},
		[]string{"kind", "result"},
	)

	// reconcileDuration tracks reconciliation duration by kind
	reconcileDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "hcloud_reconcile_duration_seconds",
			Help:    "Reconciliation duration in seconds by kind",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"kind"},
	)

	// apiRequestsTotal tracks total number of HCloud API requests by operation and result
	apiRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "hcloud_api_requests_total",
			Help: "Total number of HCloud API requests by operation and result",
		},
		[]string{"operation", "result"},
	)

	// apiRequestDuration tracks HCloud API request duration by operation
	apiRequestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "hcloud_api_request_duration_seconds",
			Help:    "HCloud API request duration in seconds by operation",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"operation"},
	)

	registerOnce sync.Once
)

// Register registers all metrics with the controller-runtime metrics registry.
// It uses sync.Once to ensure metrics are only registered once, preventing
// duplicate registration panics.
func Register() {
	registerOnce.Do(func() {
		ctrlmetrics.Registry.MustRegister(
			reconcileTotal,
			reconcileDuration,
			apiRequestsTotal,
			apiRequestDuration,
		)
	})
}

// RecordReconcile records a reconciliation event with the given kind, result, and duration.
// The result should be either "success" or "error".
func RecordReconcile(kind, result string, duration time.Duration) {
	reconcileTotal.WithLabelValues(kind, result).Inc()
	reconcileDuration.WithLabelValues(kind).Observe(duration.Seconds())
}

// RecordAPI records an HCloud API request with the given operation, result, and duration.
// The result should be either "success" or "error".
func RecordAPI(operation, result string, duration time.Duration) {
	apiRequestsTotal.WithLabelValues(operation, result).Inc()
	apiRequestDuration.WithLabelValues(operation).Observe(duration.Seconds())
}

// APIRequestsCounter exposes the API requests counter (for tests).
func APIRequestsCounter() *prometheus.CounterVec {
	return apiRequestsTotal
}

// APIRequestDurationHistogram exposes the API request duration histogram (for tests).
func APIRequestDurationHistogram() *prometheus.HistogramVec {
	return apiRequestDuration
}
