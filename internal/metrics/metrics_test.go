package metrics

import (
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus/testutil"
)

func TestReconcileMetrics(t *testing.T) {
	Register()

	duration := 100 * time.Millisecond
	RecordReconcile("HCloudServer", "success", duration)

	count := testutil.CollectAndCount(reconcileTotal)
	if count != 1 {
		t.Errorf("expected 1 metric for reconcileTotal, got %d", count)
	}

	value := testutil.ToFloat64(reconcileTotal.WithLabelValues("HCloudServer", "success"))
	if value != 1.0 {
		t.Errorf("expected reconcileTotal value to be 1.0, got %f", value)
	}

	histCount := testutil.CollectAndCount(reconcileDuration)
	if histCount == 0 {
		t.Error("expected histogram to have collected metrics")
	}
}

func TestReconcileMetricsError(t *testing.T) {
	Register()

	duration := 50 * time.Millisecond
	RecordReconcile("HCloudMachine", "error", duration)

	value := testutil.ToFloat64(reconcileTotal.WithLabelValues("HCloudMachine", "error"))
	if value < 1.0 {
		t.Errorf("expected reconcileTotal error count to be at least 1.0, got %f", value)
	}
}

func TestAPIMetrics(t *testing.T) {
	Register()

	duration := 200 * time.Millisecond
	RecordAPI("CreateServer", "success", duration)

	count := testutil.CollectAndCount(apiRequestsTotal)
	if count < 1 {
		t.Errorf("expected at least 1 metric for apiRequestsTotal, got %d", count)
	}

	value := testutil.ToFloat64(apiRequestsTotal.WithLabelValues("CreateServer", "success"))
	if value < 1.0 {
		t.Errorf("expected apiRequestsTotal value to be at least 1.0, got %f", value)
	}

	histCount := testutil.CollectAndCount(apiRequestDuration)
	if histCount == 0 {
		t.Error("expected API histogram to have collected metrics")
	}
}

func TestAPIMetricsError(t *testing.T) {
	Register()

	duration := 75 * time.Millisecond
	RecordAPI("DeleteServer", "error", duration)

	value := testutil.ToFloat64(apiRequestsTotal.WithLabelValues("DeleteServer", "error"))
	if value < 1.0 {
		t.Errorf("expected apiRequestsTotal error count to be at least 1.0, got %f", value)
	}
}

func TestMultipleReconciles(t *testing.T) {
	Register()

	for i := 0; i < 5; i++ {
		RecordReconcile("HCloudVolume", "success", 10*time.Millisecond)
	}

	value := testutil.ToFloat64(reconcileTotal.WithLabelValues("HCloudVolume", "success"))
	if value < 5.0 {
		t.Errorf("expected reconcileTotal value to be at least 5.0, got %f", value)
	}
}

func TestMultipleAPIRequests(t *testing.T) {
	Register()

	for i := 0; i < 3; i++ {
		RecordAPI("ListServers", "success", 15*time.Millisecond)
	}

	value := testutil.ToFloat64(apiRequestsTotal.WithLabelValues("ListServers", "success"))
	if value < 3.0 {
		t.Errorf("expected apiRequestsTotal value to be at least 3.0, got %f", value)
	}
}
