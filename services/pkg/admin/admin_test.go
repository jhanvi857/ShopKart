package admin

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAdminAuthAndConfig(t *testing.T) {
	mgr := NewConfigManager("secret123", true)
	mgr.Set("worker_limit", 10)

	// Unauthorized test
	req := httptest.NewRequest(http.MethodGet, "/admin/status", nil)
	rr := httptest.NewRecorder()
	mgr.HandleStatus(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 unauthorized, got %d", rr.Code)
	}

	// Authorized test
	reqAuth := httptest.NewRequest(http.MethodGet, "/admin/status", nil)
	reqAuth.Header.Set("X-Admin-Key", "secret123")
	rrAuth := httptest.NewRecorder()
	mgr.HandleStatus(rrAuth, reqAuth)
	if rrAuth.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", rrAuth.Code)
	}

	// Update test
	body := bytes.NewReader([]byte(`{"worker_limit":25}`))
	reqUpdate := httptest.NewRequest(http.MethodPost, "/admin/config", body)
	reqUpdate.Header.Set("X-Admin-Key", "secret123")
	rrUpdate := httptest.NewRecorder()
	mgr.HandleUpdateConfig(rrUpdate, reqUpdate)
	if rrUpdate.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on update, got %d", rrUpdate.Code)
	}

	val, ok := mgr.Get("worker_limit")
	if !ok || val.(float64) != 25 {
		t.Fatalf("expected updated worker_limit 25, got %v", val)
	}
}
