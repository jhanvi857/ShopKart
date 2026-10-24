package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
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

type InventoryApp struct {
	db        *sql.DB
	configMgr *admin.ConfigManager
	tracer    trace.Tracer
	metrics   *metrics.Metrics
}

type StockResponse struct {
	ProductID string    `json:"product_id"`
	Stock     int       `json:"stock"`
	Reserved  int       `json:"reserved"`
	Available int       `json:"available"`
	UpdatedAt time.Time `json:"updated_at"`
}

type ReserveRequest struct {
	ProductID     string `json:"product_id"`
	Quantity      int    `json:"quantity"`
	ReservationID string `json:"reservation_id"`
}

type ActionResponse struct {
	Status    string `json:"status"`
	ProductID string `json:"product_id"`
	Quantity  int    `json:"quantity"`
	Available int    `json:"available"`
}

func main() {
	cfg := config.LoadCommon("inventory", "9004")
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
	m.MonitorDB(db, "inventory_db", 2*time.Second, stopDBMon)

	app := &InventoryApp{
		db:        db,
		configMgr: admin.NewConfigManager(cfg.AdminAPIKey, cfg.FaultEndpoints),
		tracer:    tracer,
		metrics:   m,
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

	r.Get("/inventory/{productId}", app.handleGetStock)
	r.Post("/reserve", app.handleReserve)
	r.Post("/release", app.handleRelease)
	r.Post("/commit", app.handleCommit)
	r.Post("/inventory/reserve", app.handleReserve)
	r.Post("/inventory/release", app.handleRelease)
	r.Post("/inventory/commit", app.handleCommit)

	r.Get("/admin/status", app.configMgr.HandleStatus)
	r.Post("/admin/config", app.handleUpdateConfig)

	server := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      r,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	go func() {
		logger.Info("Starting inventory service", slog.String("port", cfg.Port))
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
	logger.Info("Inventory service shutdown gracefully")
}

func (app *InventoryApp) handleGetStock(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	productID := chi.URLParam(r, "productId")

	var stock, reserved int
	var updatedAt time.Time
	err := app.db.QueryRowContext(ctx,
		`SELECT stock, reserved, updated_at FROM inventory WHERE product_id = $1`, productID,
	).Scan(&stock, &reserved, &updatedAt)

	if err == sql.ErrNoRows {
		http.Error(w, `{"error":"product not found in inventory"}`, http.StatusNotFound)
		return
	} else if err != nil {
		http.Error(w, `{"error":"database error"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(StockResponse{
		ProductID: productID,
		Stock:     stock,
		Reserved:  reserved,
		Available: stock - reserved,
		UpdatedAt: updatedAt,
	})
}

func (app *InventoryApp) handleReserve(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	ctx, span := app.tracer.Start(ctx, "InventoryApp.Reserve")
	defer span.End()

	var req ReserveRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.ProductID == "" || req.Quantity <= 0 {
		http.Error(w, `{"error":"invalid reserve request"}`, http.StatusBadRequest)
		return
	}

	tx, err := app.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		http.Error(w, `{"error":"failed to begin transaction"}`, http.StatusInternalServerError)
		return
	}
	defer tx.Rollback()

	// Use SELECT ... FOR UPDATE to establish a real row lock
	var stock, reserved int
	err = tx.QueryRowContext(ctx,
		`SELECT stock, reserved FROM inventory WHERE product_id = $1 FOR UPDATE`, req.ProductID,
	).Scan(&stock, &reserved)

	if err == sql.ErrNoRows {
		http.Error(w, `{"error":"product not found"}`, http.StatusNotFound)
		return
	} else if err != nil {
		slog.ErrorContext(ctx, "failed row lock", slog.String("error", err.Error()))
		http.Error(w, `{"error":"lock acquisition failed"}`, http.StatusInternalServerError)
		return
	}

	available := stock - reserved
	if available < req.Quantity {
		http.Error(w, fmt.Sprintf(`{"error":"insufficient stock: requested %d, available %d"}`, req.Quantity, available), http.StatusConflict)
		return
	}

	_, err = tx.ExecContext(ctx,
		`UPDATE inventory SET reserved = reserved + $1, updated_at = NOW() WHERE product_id = $2`,
		req.Quantity, req.ProductID,
	)
	if err != nil {
		http.Error(w, `{"error":"failed to reserve stock"}`, http.StatusInternalServerError)
		return
	}

	if err := tx.Commit(); err != nil {
		http.Error(w, `{"error":"failed to commit reservation"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(ActionResponse{
		Status:    "reserved",
		ProductID: req.ProductID,
		Quantity:  req.Quantity,
		Available: available - req.Quantity,
	})
}

func (app *InventoryApp) handleRelease(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var req ReserveRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.ProductID == "" || req.Quantity <= 0 {
		http.Error(w, `{"error":"invalid release request"}`, http.StatusBadRequest)
		return
	}

	tx, err := app.db.BeginTx(ctx, nil)
	if err != nil {
		http.Error(w, `{"error":"database error"}`, http.StatusInternalServerError)
		return
	}
	defer tx.Rollback()

	_, err = tx.ExecContext(ctx,
		`UPDATE inventory SET reserved = GREATEST(0, reserved - $1), updated_at = NOW() WHERE product_id = $2`,
		req.Quantity, req.ProductID,
	)
	if err != nil {
		http.Error(w, `{"error":"failed to release reservation"}`, http.StatusInternalServerError)
		return
	}

	_ = tx.Commit()
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(`{"status":"released"}`))
}

func (app *InventoryApp) handleCommit(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var req ReserveRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.ProductID == "" || req.Quantity <= 0 {
		http.Error(w, `{"error":"invalid commit request"}`, http.StatusBadRequest)
		return
	}

	tx, err := app.db.BeginTx(ctx, nil)
	if err != nil {
		http.Error(w, `{"error":"database error"}`, http.StatusInternalServerError)
		return
	}
	defer tx.Rollback()

	_, err = tx.ExecContext(ctx,
		`UPDATE inventory SET stock = GREATEST(0, stock - $1), reserved = GREATEST(0, reserved - $1), updated_at = NOW() WHERE product_id = $2`,
		req.Quantity, req.ProductID,
	)
	if err != nil {
		http.Error(w, `{"error":"failed to commit inventory deduction"}`, http.StatusInternalServerError)
		return
	}

	_ = tx.Commit()
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(`{"status":"committed"}`))
}

func (app *InventoryApp) handleUpdateConfig(w http.ResponseWriter, r *http.Request) {
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
