package metrics

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestMetricsRegistrationAndTracking(t *testing.T) {
	m := New("test-service", "inst-1", "test")

	m.TrackRequest("GET", "/api/test", 200, 15*time.Millisecond)
	m.RequestsInFlight.WithLabelValues().Inc()
	m.WorkerPoolUtilization.WithLabelValues("test_pool").Set(0.75)
	m.CacheHitsTotal.WithLabelValues("redis").Inc()

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rr := httptest.NewRecorder()

	m.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}

	body := rr.Body.String()
	if !strings.Contains(body, "http_requests_total") {
		t.Errorf("metrics output missing http_requests_total")
	}
	if !strings.Contains(body, `service="test-service"`) {
		t.Errorf("metrics output missing service label")
	}
	if !strings.Contains(body, "worker_pool_utilization_ratio") {
		t.Errorf("metrics output missing worker_pool_utilization_ratio")
	}
}
