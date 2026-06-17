package hcloud

import (
	"context"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus/testutil"

	"github.com/armanfeyzi/hcloud-operator/internal/metrics"
)

func TestInstrumentedClientRecordsSuccess(t *testing.T) {
	metrics.Register()

	fake := NewFakeClient()
	_, err := fake.CreateServer(context.Background(), ServerCreateOpts{
		Name:       "test",
		ServerType: "cx23",
		Image:      "ubuntu-22.04",
		Location:   "fsn1",
	})
	if err != nil {
		t.Fatalf("setup fake server: %v", err)
	}

	wrapped := Instrument(fake)
	if _, err := wrapped.GetServer(context.Background(), 1); err != nil {
		t.Fatalf("GetServer: %v", err)
	}

	value := testutil.ToFloat64(metrics.APIRequestsCounter().WithLabelValues("GetServer", "success"))
	if value < 1.0 {
		t.Fatalf("expected GetServer success metric >= 1, got %f", value)
	}
}

func TestInstrumentedClientRecordsError(t *testing.T) {
	metrics.Register()

	fake := NewFakeClient()
	fake.GetErr = context.Canceled

	wrapped := Instrument(fake)
	_, err := wrapped.GetServer(context.Background(), 99)
	if err == nil {
		t.Fatal("expected error from fake client")
	}

	value := testutil.ToFloat64(metrics.APIRequestsCounter().WithLabelValues("GetServer", "error"))
	if value < 1.0 {
		t.Fatalf("expected GetServer error metric >= 1, got %f", value)
	}
}

func TestInstrumentNilAndIdempotent(t *testing.T) {
	if Instrument(nil) != nil {
		t.Fatal("Instrument(nil) should return nil")
	}
	fake := NewFakeClient()
	once := Instrument(fake)
	twice := Instrument(once)
	if once != twice {
		t.Fatal("double Instrument should be a no-op")
	}
}

func TestInstrumentedClientRecordsDuration(t *testing.T) {
	metrics.Register()

	fake := NewFakeClient()
	wrapped := Instrument(fake)

	start := time.Now()
	_, _ = wrapped.GetServerByName(context.Background(), "missing")
	if time.Since(start) < 0 {
		t.Fatal("unexpected negative duration")
	}

	histCount := testutil.CollectAndCount(metrics.APIRequestDurationHistogram())
	if histCount == 0 {
		t.Fatal("expected API duration histogram to collect samples")
	}
}
