package health

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"time"
)

type CheckFunc func(ctx context.Context) error

type Checker struct {
	mu     sync.RWMutex
	checks map[string]CheckFunc
}

func NewChecker() *Checker {
	return &Checker{
		checks: make(map[string]CheckFunc),
	}
}

func (c *Checker) Register(name string, check CheckFunc) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.checks[name] = check
}

func (c *Checker) HealthzHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"alive"}`))
	}
}

type ReadyResult struct {
	Status  string            `json:"status"`
	Details map[string]string `json:"details"`
}

func (c *Checker) ReadyzHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		c.mu.RLock()
		defer c.mu.RUnlock()

		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()

		results := make(map[string]string)
		allOk := true

		for name, check := range c.checks {
			if err := check(ctx); err != nil {
				allOk = false
				results[name] = "unhealthy: " + err.Error()
			} else {
				results[name] = "healthy"
			}
		}

		res := ReadyResult{
			Status:  "ready",
			Details: results,
		}

		w.Header().Set("Content-Type", "application/json")
		if !allOk {
			res.Status = "not ready"
			w.WriteHeader(http.StatusServiceUnavailable)
		} else {
			w.WriteHeader(http.StatusOK)
		}

		_ = json.NewEncoder(w).Encode(res)
	}
}
