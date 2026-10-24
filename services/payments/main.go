package main

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	_ "github.com/lib/pq"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"

	"shopkart/pkg/admin"
	"shopkart/pkg/config"
	"shopkart/pkg/health"
	"shopkart/pkg/logging"
	"shopkart/pkg/metrics"
	shopkartmw "shopkart/pkg/middleware"
	"shopkart/pkg/tracing"
)

type PaymentsApp struct {
	db              *sql.DB
	bankURL         string
	httpClient      *http.Client
	configMgr       *admin.ConfigManager
	tracer          trace.Tracer
	metrics         *metrics.Metrics
	leakMu          sync.Mutex
	leakedBuffers   [][]byte
	leakRateKB      int
	leakActive      bool
}

type ChargeRequest struct {
	OrderID        string  `json:"order_id"`
	Amount         float64 `json:"amount"`
	Currency       string  `json:"currency"`
	IdempotencyKey string  `json:"idempotency_key"`
	CardToken      string  `json:"card_token"`
}

type ChargeResult struct {
	PaymentID     string    `json:"payment_id"`
	OrderID       string    `json:"order_id"`
	TransactionID string    `json:"transaction_id"`
	Status        string    `json:"status"`
	Amount        float64   `json:"amount"`
	Currency      string    `json:"currency"`
	CreatedAt     time.Time `json:"created_at"`
}

func main() {
	cfg := config.LoadCommon("payments", "9005")
	logger := logging.InitLogger(cfg.ServiceName, cfg.InstanceID, cfg.Environment)
	m := metrics.New(cfg.ServiceName, cfg.InstanceID, cfg.Environment)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	tp, err := tracing.InitTracer(ctx, cfg.ServiceName, cfg.InstanceID, cfg.Environment, cfg.OTLPEndpoint)
	if err != nil {
		logger.Warn("Failed to initialize OTel tracer", slog.String("error", err.Error()))
	} else {
		defer func() { _ = tp.Shutdown(ctx) }()
	}
	tracer := otel.GetTracerProvider().Tracer(cfg.ServiceName)

	dbConnStr := config.GetEnv("DATABASE_URL", "postgres://shopkart_user:shopkart_pass_secret@localhost:5433/shopkart_db?sslmode=disable")
	db, err := sql.Open("postgres", dbConnStr)
	if err != nil {
		logger.Error("Failed to open DB", slog.String("error", err.Error()))
		os.Exit(1)
	}
	defer db.Close()

	initialMaxConns := config.GetEnvInt("DB_MAX_OPEN_CONNS", 10)
	db.SetMaxOpenConns(initialMaxConns)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)

	stopDBMon := make(chan struct{})
	defer close(stopDBMon)
	m.MonitorDB(db, "payments_db", 2*time.Second, stopDBMon)

	bankURL := config.GetEnv("BANK_MOCK_URL", "http://localhost:9008")

	app := &PaymentsApp{
		db:         db,
		bankURL:    bankURL,
		httpClient: &http.Client{Timeout: 5 * time.Second},
		configMgr:  admin.NewConfigManager(cfg.AdminAPIKey, cfg.FaultEndpoints),
		tracer:     tracer,
		metrics:    m,
		leakRateKB: 1024, // 1MB per allocation when leaky path activated
	}
	app.configMgr.Set("db_max_open_conns", initialMaxConns)

	checker := health.NewChecker()
	checker.Register("postgres", func(ctx context.Context) error {
		return db.PingContext(ctx)
	})

	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)
	r.Use(shopkartmw.ObservabilityMiddleware(cfg.ServiceName, m))

	r.Get("/healthz", checker.HealthzHandler())
	r.Get("/readyz", checker.ReadyzHandler())
	r.Handle("/metrics", m.Handler())

	r.Post("/charge", app.handleCharge)
	r.Post("/payments/charge", app.handleCharge)
	r.Get("/{orderId}", app.handleGetPayment)
	r.Get("/payments/{orderId}", app.handleGetPayment)

	// Admin endpoints
	r.Get("/admin/status", app.configMgr.HandleStatus)
	r.Post("/admin/config", app.handleUpdateConfig)

	// Controlled fault endpoints (enabled only if SHOPKART_FAULT_ENDPOINTS=true)
	if cfg.FaultEndpoints {
		r.Post("/admin/fault/leak-enable", app.handleEnableLeak)
		r.Post("/admin/fault/leak-disable", app.handleDisableLeak)
		r.Post("/admin/fault/leak-clear", app.handleClearLeak)
	}

	server := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      r,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	go func() {
		logger.Info("Starting payments service", slog.String("port", cfg.Port))
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
	logger.Info("Payments service shutdown gracefully")
}

func (app *PaymentsApp) handleCharge(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	ctx, span := app.tracer.Start(ctx, "PaymentsApp.Charge")
	defer span.End()

	var req ChargeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request format"}`, http.StatusBadRequest)
		return
	}

	if req.IdempotencyKey == "" || req.OrderID == "" || req.Amount <= 0 {
		http.Error(w, `{"error":"missing required payment fields"}`, http.StatusBadRequest)
		return
	}

	// Real memory pressure path if enabled by ops/fault script
	app.leakMu.Lock()
	if app.leakActive {
		chunk := make([]byte, app.leakRateKB*1024)
		for i := range chunk {
			chunk[i] = byte(i % 256)
		}
		app.leakedBuffers = append(app.leakedBuffers, chunk)
	}
	app.leakMu.Unlock()

	// Check idempotency in Postgres
	var existing ChargeResult
	err := app.db.QueryRowContext(ctx,
		`SELECT payment_id, order_id, transaction_id, status, amount, currency, created_at 
		 FROM payments WHERE idempotency_key = $1`, req.IdempotencyKey,
	).Scan(&existing.PaymentID, &existing.OrderID, &existing.TransactionID, &existing.Status, &existing.Amount, &existing.Currency, &existing.CreatedAt)

	if err == nil {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(existing)
		return
	}

	// Call external bank mock
	bankReqBody, _ := json.Marshal(map[string]interface{}{
		"amount":          req.Amount,
		"currency":        req.Currency,
		"idempotency_key": req.IdempotencyKey,
		"card_token":      req.CardToken,
	})

	bankReq, err := http.NewRequestWithContext(ctx, http.MethodPost, app.bankURL+"/charge", bytes.NewReader(bankReqBody))
	if err != nil {
		http.Error(w, `{"error":"failed to create bank request"}`, http.StatusInternalServerError)
		return
	}
	bankReq.Header.Set("Content-Type", "application/json")
	tracing.InjectHTTPHeaders(ctx, bankReq)

	bankResp, err := app.httpClient.Do(bankReq)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"bank service unavailable: %s"}`, err.Error()), http.StatusBadGateway)
		return
	}
	defer bankResp.Body.Close()

	var bankResult struct {
		TransactionID string `json:"transaction_id"`
		Status        string `json:"status"`
		Reason        string `json:"reason"`
	}
	if err := json.NewDecoder(bankResp.Body).Decode(&bankResult); err != nil {
		http.Error(w, `{"error":"failed to parse bank response"}`, http.StatusBadGateway)
		return
	}

	paymentID := fmt.Sprintf("pay_%d", time.Now().UnixNano())
	createdAt := time.Now().UTC()

	// Store result in Postgres
	_, err = app.db.ExecContext(ctx,
		`INSERT INTO payments (payment_id, order_id, idempotency_key, transaction_id, status, amount, currency, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		paymentID, req.OrderID, req.IdempotencyKey, bankResult.TransactionID, bankResult.Status, req.Amount, req.Currency, createdAt,
	)
	if err != nil {
		slog.ErrorContext(ctx, "failed to persist payment", slog.String("error", err.Error()))
		http.Error(w, `{"error":"database error recording payment"}`, http.StatusInternalServerError)
		return
	}

	res := ChargeResult{
		PaymentID:     paymentID,
		OrderID:       req.OrderID,
		TransactionID: bankResult.TransactionID,
		Status:        bankResult.Status,
		Amount:        req.Amount,
		Currency:      req.Currency,
		CreatedAt:     createdAt,
	}

	w.Header().Set("Content-Type", "application/json")
	if bankResult.Status != "approved" {
		w.WriteHeader(http.StatusPaymentRequired)
	} else {
		w.WriteHeader(http.StatusOK)
	}
	_ = json.NewEncoder(w).Encode(res)
}

func (app *PaymentsApp) handleGetPayment(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	orderID := chi.URLParam(r, "orderId")

	var res ChargeResult
	err := app.db.QueryRowContext(ctx,
		`SELECT payment_id, order_id, transaction_id, status, amount, currency, created_at 
		 FROM payments WHERE order_id = $1 ORDER BY created_at DESC LIMIT 1`, orderID,
	).Scan(&res.PaymentID, &res.OrderID, &res.TransactionID, &res.Status, &res.Amount, &res.Currency, &res.CreatedAt)

	if err == sql.ErrNoRows {
		http.Error(w, `{"error":"payment not found"}`, http.StatusNotFound)
		return
	} else if err != nil {
		http.Error(w, `{"error":"database error"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(res)
}

func (app *PaymentsApp) handleUpdateConfig(w http.ResponseWriter, r *http.Request) {
	if !app.configMgr.Authenticate(r) {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}

	var updates struct {
		DBMaxOpenConns *int `json:"db_max_open_conns"`
	}
	if err := json.NewDecoder(r.Body).Decode(&updates); err != nil {
		http.Error(w, `{"error":"invalid body"}`, http.StatusBadRequest)
		return
	}

	if updates.DBMaxOpenConns != nil && *updates.DBMaxOpenConns > 0 {
		app.db.SetMaxOpenConns(*updates.DBMaxOpenConns)
		app.configMgr.Set("db_max_open_conns", *updates.DBMaxOpenConns)
	}

	app.configMgr.HandleStatus(w, r)
}

func (app *PaymentsApp) handleEnableLeak(w http.ResponseWriter, r *http.Request) {
	if !app.configMgr.Authenticate(r) {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}
	app.leakMu.Lock()
	app.leakActive = true
	app.leakMu.Unlock()
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(`{"status":"leak_enabled"}`))
}

func (app *PaymentsApp) handleDisableLeak(w http.ResponseWriter, r *http.Request) {
	if !app.configMgr.Authenticate(r) {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}
	app.leakMu.Lock()
	app.leakActive = false
	app.leakMu.Unlock()
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(`{"status":"leak_disabled"}`))
}

func (app *PaymentsApp) handleClearLeak(w http.ResponseWriter, r *http.Request) {
	if !app.configMgr.Authenticate(r) {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}
	app.leakMu.Lock()
	app.leakedBuffers = nil
	app.leakActive = false
	app.leakMu.Unlock()
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(`{"status":"leak_cleared"}`))
}
