package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func TestSupervisor_AuthAndAllowlist(t *testing.T) {
	tmpAudit := "test_audit.log"
	defer os.Remove(tmpAudit)

	sup, err := NewSupervisor("test-secret-key", tmpAudit)
	if err != nil {
		t.Fatalf("Failed to init supervisor: %v", err)
	}

	// 1. Unauthorized request
	req := httptest.NewRequest(http.MethodGet, "/ops/instances", nil)
	rr := httptest.NewRecorder()
	handler := sup.AuthMiddleware(http.HandlerFunc(sup.ListInstances))
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("Expected 401 Unauthorized without key, got %d", rr.Code)
	}

	// 2. Authorized request
	reqAuth := httptest.NewRequest(http.MethodGet, "/ops/instances", nil)
	reqAuth.Header.Set("X-Admin-Key", "test-secret-key")
	rrAuth := httptest.NewRecorder()
	handler.ServeHTTP(rrAuth, reqAuth)

	if rrAuth.Code != http.StatusOK {
		t.Errorf("Expected 200 OK with valid key, got %d", rrAuth.Code)
	}

	// 3. Reject non-allowlisted service
	badStart := map[string]interface{}{
		"service": "malicious-crypto-miner",
		"port":    9999,
	}
	body, _ := json.Marshal(badStart)
	reqBad := httptest.NewRequest(http.MethodPost, "/ops/instances/start", bytes.NewReader(body))
	reqBad.Header.Set("X-Admin-Key", "test-secret-key")
	rrBad := httptest.NewRecorder()
	handlerStart := sup.AuthMiddleware(http.HandlerFunc(sup.StartInstance))
	handlerStart.ServeHTTP(rrBad, reqBad)

	if rrBad.Code != http.StatusForbidden {
		t.Errorf("Expected 403 Forbidden for non-allowlisted service, got %d", rrBad.Code)
	}

	// 4. Verify audit entries were logged
	if len(sup.auditLog) == 0 {
		t.Errorf("Expected audit entries to be recorded, got 0")
	}
}
