package health

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHealthzHandler(t *testing.T) {
	checker := NewChecker()
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rr := httptest.NewRecorder()

	checker.HealthzHandler()(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
}

func TestReadyzHandler_AllHealthy(t *testing.T) {
	checker := NewChecker()
	checker.Register("db", func(ctx context.Context) error {
		return nil
	})

	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	rr := httptest.NewRecorder()

	checker.ReadyzHandler()(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
}

func TestReadyzHandler_Unhealthy(t *testing.T) {
	checker := NewChecker()
	checker.Register("db", func(ctx context.Context) error {
		return errors.New("db connection timeout")
	})

	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	rr := httptest.NewRecorder()

	checker.ReadyzHandler()(rr, req)

	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("expected 503, got %d", rr.Code)
	}
}
