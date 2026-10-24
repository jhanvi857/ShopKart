package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync/atomic"
	"testing"

	"go.opentelemetry.io/otel/trace/noop"
)

func TestServicePool_HealthCheckedRoundRobin(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var reqCount1, reqCount2 atomic.Int32
	var s1Healthy atomic.Bool
	s1Healthy.Store(true)

	// Backend 1
	s1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/healthz" {
			if s1Healthy.Load() {
				w.WriteHeader(http.StatusOK)
			} else {
				w.WriteHeader(http.StatusServiceUnavailable)
			}
			return
		}
		reqCount1.Add(1)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("s1"))
	}))
	defer s1.Close()

	// Backend 2
	s2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/healthz" {
			w.WriteHeader(http.StatusOK)
			return
		}
		reqCount2.Add(1)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("s2"))
	}))
	defer s2.Close()

	u1, _ := url.Parse(s1.URL)
	u2, _ := url.Parse(s2.URL)

	tracer := noop.NewTracerProvider().Tracer("test")
	pool := NewServicePool(ctx, "testsvc", "/api/test", []string{u1.Host, u2.Host}, tracer)

	// Both are healthy: round robin
	for i := 0; i < 4; i++ {
		req := httptest.NewRequest(http.MethodGet, "/api/test/foo", nil)
		rr := httptest.NewRecorder()
		pool.ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("Expected 200, got %d", rr.Code)
		}
	}

	if reqCount1.Load() != 2 || reqCount2.Load() != 2 {
		t.Errorf("Expected balanced requests (2 and 2), got %d and %d", reqCount1.Load(), reqCount2.Load())
	}

	// Now fail backend 1
	s1Healthy.Store(false)
	pool.checkHealth()

	reqCount1.Store(0)
	reqCount2.Store(0)

	// Requests should only go to backend 2
	for i := 0; i < 3; i++ {
		req := httptest.NewRequest(http.MethodGet, "/api/test/foo", nil)
		rr := httptest.NewRecorder()
		pool.ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("Expected 200, got %d", rr.Code)
		}
	}

	if reqCount1.Load() != 0 {
		t.Errorf("Expected 0 requests to unhealthy s1, got %d", reqCount1.Load())
	}
	if reqCount2.Load() != 3 {
		t.Errorf("Expected 3 requests to healthy s2, got %d", reqCount2.Load())
	}
}

func TestParseUpstreamAddrs(t *testing.T) {
	addrs := parseUpstreamAddrs("localhost:9003, localhost:9013", "http://localhost:9003")
	if len(addrs) != 2 || addrs[0] != "localhost:9003" || addrs[1] != "localhost:9013" {
		t.Errorf("Unexpected parsed addrs: %v", addrs)
	}

	fallback := parseUpstreamAddrs("", "http://localhost:9003,http://localhost:9013")
	if len(fallback) != 2 {
		t.Errorf("Unexpected fallback addrs: %v", fallback)
	}
}
