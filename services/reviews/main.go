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
	"github.com/google/uuid"
	_ "github.com/lib/pq"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"

	"shopkart/pkg/config"
	"shopkart/pkg/health"
	"shopkart/pkg/logging"
	"shopkart/pkg/metrics"
	shopkartmw "shopkart/pkg/middleware"
	"shopkart/pkg/tracing"
)

type ReviewsApp struct {
	db      *sql.DB
	tracer  trace.Tracer
	metrics *metrics.Metrics
}

type Review struct {
	ID        string    `json:"id"`
	ProductID string    `json:"product_id"`
	Rating    int       `json:"rating"`
	Comment   string    `json:"comment"`
	Author    string    `json:"author"`
	CreatedAt time.Time `json:"created_at"`
}

func main() {
	cfg := config.LoadCommon("reviews", "9009")
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

	db.SetMaxOpenConns(config.GetEnvInt("DB_MAX_OPEN_CONNS", 10))
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)

	stopDBMon := make(chan struct{})
	defer close(stopDBMon)
	m.MonitorDB(db, "reviews_db", 2*time.Second, stopDBMon)

	app := &ReviewsApp{
		db:      db,
		tracer:  tracer,
		metrics: m,
	}

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

	r.Get("/reviews/product/{productId}", app.handleGetProductReviews)
	r.Post("/reviews", app.handleCreateReview)

	server := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      r,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	go func() {
		logger.Info("Starting reviews service", slog.String("port", cfg.Port))
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
	logger.Info("Reviews service shutdown gracefully")
}

func (app *ReviewsApp) handleGetProductReviews(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	productID := chi.URLParam(r, "productId")

	rows, err := app.db.QueryContext(ctx,
		`SELECT id, product_id, rating, comment, author, created_at FROM reviews WHERE product_id = $1 ORDER BY created_at DESC`,
		productID,
	)
	if err != nil {
		http.Error(w, `{"error":"database error"}`, http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	reviews := make([]Review, 0)
	for rows.Next() {
		var rev Review
		if err := rows.Scan(&rev.ID, &rev.ProductID, &rev.Rating, &rev.Comment, &rev.Author, &rev.CreatedAt); err == nil {
			reviews = append(reviews, rev)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(reviews)
}

func (app *ReviewsApp) handleCreateReview(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var req Review
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.ProductID == "" || req.Rating < 1 || req.Rating > 5 {
		http.Error(w, `{"error":"invalid review parameters"}`, http.StatusBadRequest)
		return
	}

	reviewID := fmt.Sprintf("rev_%s", uuid.New().String()[:10])
	now := time.Now().UTC()

	_, err := app.db.ExecContext(ctx,
		`INSERT INTO reviews (id, product_id, rating, comment, author, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6)`,
		reviewID, req.ProductID, req.Rating, req.Comment, req.Author, now,
	)
	if err != nil {
		http.Error(w, `{"error":"failed to save review"}`, http.StatusInternalServerError)
		return
	}

	req.ID = reviewID
	req.CreatedAt = now

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(req)
}
