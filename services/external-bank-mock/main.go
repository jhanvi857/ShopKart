package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"math/rand"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/google/uuid"

	"shopkart/pkg/config"
	"shopkart/pkg/health"
	"shopkart/pkg/logging"
	"shopkart/pkg/metrics"
	shopkartmw "shopkart/pkg/middleware"
	"shopkart/pkg/tracing"
)

type BankConfig struct {
	mu            sync.RWMutex
	BaseLatencyMs int     `json:"base_latency_ms"`
	JitterMs      int     `json:"jitter_ms"`
	FailureRate   float64 `json:"failure_rate"` // 0.0 to 1.0
}

var bankSettings = &BankConfig{
	BaseLatencyMs: 35,
	JitterMs:      15,
	FailureRate:   0.0,
}

type ChargeRequest struct {
	Amount         float64 `json:"amount"`
	Currency       string  `json:"currency"`
	IdempotencyKey string  `json:"idempotency_key"`
	CardToken      string  `json:"card_token"`
}

type ChargeResponse struct {
	TransactionID string    `json:"transaction_id"`
	Status        string    `json:"status"`
	Amount        float64   `json:"amount"`
	Currency      string    `json:"currency"`
	ProcessedAt   time.Time `json:"processed_at"`
	Reason        string    `json:"reason,omitempty"`
}

func main() {
	cfg := config.LoadCommon("external-bank-mock", "9008")
	logger := logging.InitLogger(cfg.ServiceName, cfg.InstanceID, cfg.Environment)
	m := metrics.New(cfg.ServiceName, cfg.InstanceID, cfg.Environment)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	tp, err := tracing.InitTracer(ctx, cfg.ServiceName, cfg.InstanceID, cfg.Environment, cfg.OTLPEndpoint)
	if err != nil {
		logger.Warn("Failed to initialize OpenTelemetry tracer", slog.String("error", err.Error()))
	} else {
		defer func() { _ = tp.Shutdown(ctx) }()
	}

	checker := health.NewChecker()

	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)
	r.Use(shopkartmw.ObservabilityMiddleware(cfg.ServiceName, m))

	r.Get("/healthz", checker.HealthzHandler())
	r.Get("/readyz", checker.ReadyzHandler())
	r.Handle("/metrics", m.Handler())

	r.Post("/charge", handleCharge)
	r.Get("/admin/config", handleGetBankConfig)
	r.Post("/admin/config", handleUpdateBankConfig)

	server := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      r,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	go func() {
		logger.Info("Starting external-bank-mock", slog.String("port", cfg.Port))
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("Server error", slog.String("error", err.Error()))
			os.Exit(1)
		}
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	<-sigCh

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer shutdownCancel()
	_ = server.Shutdown(shutdownCtx)
	logger.Info("external-bank-mock shutdown gracefully")
}

func handleCharge(w http.ResponseWriter, r *http.Request) {
	var req ChargeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request"}`, http.StatusBadRequest)
		return
	}

	bankSettings.mu.RLock()
	baseLat := bankSettings.BaseLatencyMs
	jitter := bankSettings.JitterMs
	failRate := bankSettings.FailureRate
	bankSettings.mu.RUnlock()

	// Simulate bank network latency
	sleepMs := baseLat
	if jitter > 0 {
		sleepMs += rand.Intn(jitter)
	}
	time.Sleep(time.Duration(sleepMs) * time.Millisecond)

	w.Header().Set("Content-Type", "application/json")

	// Simulate bank declination if failure rate configured
	if failRate > 0 && rand.Float64() < failRate {
		w.WriteHeader(http.StatusPaymentRequired)
		_ = json.NewEncoder(w).Encode(ChargeResponse{
			TransactionID: fmt.Sprintf("txn_failed_%s", uuid.New().String()[:8]),
			Status:        "declined",
			Amount:        req.Amount,
			Currency:      req.Currency,
			ProcessedAt:   time.Now().UTC(),
			Reason:        "insufficient_funds_or_bank_error",
		})
		return
	}

	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(ChargeResponse{
		TransactionID: fmt.Sprintf("txn_%s", uuid.New().String()),
		Status:        "approved",
		Amount:        req.Amount,
		Currency:      req.Currency,
		ProcessedAt:   time.Now().UTC(),
	})
}

func handleGetBankConfig(w http.ResponseWriter, r *http.Request) {
	bankSettings.mu.RLock()
	defer bankSettings.mu.RUnlock()
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(bankSettings)
}

func handleUpdateBankConfig(w http.ResponseWriter, r *http.Request) {
	var updates struct {
		BaseLatencyMs *int     `json:"base_latency_ms"`
		JitterMs      *int     `json:"jitter_ms"`
		FailureRate   *float64 `json:"failure_rate"`
	}

	if err := json.NewDecoder(r.Body).Decode(&updates); err != nil {
		http.Error(w, `{"error":"invalid body"}`, http.StatusBadRequest)
		return
	}

	bankSettings.mu.Lock()
	if updates.BaseLatencyMs != nil {
		bankSettings.BaseLatencyMs = *updates.BaseLatencyMs
	}
	if updates.JitterMs != nil {
		bankSettings.JitterMs = *updates.JitterMs
	}
	if updates.FailureRate != nil {
		bankSettings.FailureRate = *updates.FailureRate
	}
	bankSettings.mu.Unlock()

	handleGetBankConfig(w, r)
}
