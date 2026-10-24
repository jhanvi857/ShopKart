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
	"github.com/redis/go-redis/v9"
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

type CatalogApp struct {
	db        *sql.DB
	rdb       *redis.Client
	configMgr *admin.ConfigManager
	tracer    trace.Tracer
	metrics   *metrics.Metrics
}

type Product struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Brand       string    `json:"brand"`
	MRP         float64   `json:"mrp"`
	Price       float64   `json:"price"`
	Category    string    `json:"category"`
	Rating      float64   `json:"rating"`
	RatingCount int       `json:"rating_count"`
	Specs       string    `json:"specs"`
	ImageURL    string    `json:"image_url"`
	CreatedAt   time.Time `json:"created_at"`
}

func main() {
	cfg := config.LoadCommon("catalog", "9001")
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
	m.MonitorDB(db, "catalog_db", 2*time.Second, stopDBMon)

	redisAddr := config.GetEnv("REDIS_ADDR", "localhost:6380")
	rdb := redis.NewClient(&redis.Options{Addr: redisAddr})
	defer rdb.Close()

	app := &CatalogApp{
		db:        db,
		rdb:       rdb,
		configMgr: admin.NewConfigManager(cfg.AdminAPIKey, cfg.FaultEndpoints),
		tracer:    tracer,
		metrics:   m,
	}
	app.configMgr.Set("db_max_open_conns", initialMaxConns)

	checker := health.NewChecker()
	checker.Register("postgres", func(ctx context.Context) error {
		return db.PingContext(ctx)
	})
	checker.Register("redis", func(ctx context.Context) error {
		return rdb.Ping(ctx).Err()
	})

	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)
	r.Use(shopkartmw.ObservabilityMiddleware(cfg.ServiceName, m))

	r.Get("/healthz", checker.HealthzHandler())
	r.Get("/readyz", checker.ReadyzHandler())
	r.Handle("/metrics", m.Handler())

	r.Get("/products", app.handleListProducts)
	r.Get("/products/{id}", app.handleGetProduct)
	r.Get("/catalog/products", app.handleListProducts)
	r.Get("/catalog/products/{id}", app.handleGetProduct)

	r.Get("/admin/status", app.configMgr.HandleStatus)
	r.Post("/admin/config", app.handleUpdateConfig)
	r.Post("/admin/cache/flush", app.handleFlushCache)

	server := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      r,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	go func() {
		logger.Info("Starting catalog service", slog.String("port", cfg.Port))
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
	logger.Info("Catalog service shutdown gracefully")
}

func (app *CatalogApp) handleListProducts(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	ctx, span := app.tracer.Start(ctx, "CatalogApp.ListProducts")
	defer span.End()

	cacheKey := "catalog:products:all"

	// Try cache-aside via Redis
	if val, err := app.rdb.Get(ctx, cacheKey).Result(); err == nil {
		app.metrics.CacheHitsTotal.WithLabelValues("catalog_redis").Inc()
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Cache", "HIT")
		_, _ = w.Write([]byte(val))
		return
	}

	app.metrics.CacheMissesTotal.WithLabelValues("catalog_redis").Inc()

	// Query Postgres
	rows, err := app.db.QueryContext(ctx, `SELECT id, name, description, brand, mrp, price, category, rating, rating_count, specs, image_url, created_at FROM products ORDER BY name ASC`)
	if err != nil {
		slog.ErrorContext(ctx, "failed to query products", slog.String("error", err.Error()))
		http.Error(w, `{"error":"database error"}`, http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	products := make([]Product, 0)
	for rows.Next() {
		var p Product
		if err := rows.Scan(&p.ID, &p.Name, &p.Description, &p.Brand, &p.MRP, &p.Price, &p.Category, &p.Rating, &p.RatingCount, &p.Specs, &p.ImageURL, &p.CreatedAt); err != nil {
			http.Error(w, `{"error":"scan error"}`, http.StatusInternalServerError)
			return
		}
		products = append(products, p)
	}

	data, err := json.Marshal(products)
	if err == nil {
		_ = app.rdb.Set(ctx, cacheKey, data, 60*time.Second).Err()
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Cache", "MISS")
	_, _ = w.Write(data)
}

func (app *CatalogApp) handleGetProduct(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	ctx, span := app.tracer.Start(ctx, "CatalogApp.GetProduct")
	defer span.End()

	productID := chi.URLParam(r, "id")
	cacheKey := fmt.Sprintf("catalog:product:%s", productID)

	if val, err := app.rdb.Get(ctx, cacheKey).Result(); err == nil {
		app.metrics.CacheHitsTotal.WithLabelValues("catalog_redis").Inc()
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Cache", "HIT")
		_, _ = w.Write([]byte(val))
		return
	}

	app.metrics.CacheMissesTotal.WithLabelValues("catalog_redis").Inc()

	var p Product
	err := app.db.QueryRowContext(ctx,
		`SELECT id, name, description, brand, mrp, price, category, rating, rating_count, specs, image_url, created_at FROM products WHERE id = $1`, productID,
	).Scan(&p.ID, &p.Name, &p.Description, &p.Brand, &p.MRP, &p.Price, &p.Category, &p.Rating, &p.RatingCount, &p.Specs, &p.ImageURL, &p.CreatedAt)

	if err == sql.ErrNoRows {
		http.Error(w, `{"error":"product not found"}`, http.StatusNotFound)
		return
	} else if err != nil {
		http.Error(w, `{"error":"database query error"}`, http.StatusInternalServerError)
		return
	}

	data, err := json.Marshal(p)
	if err == nil {
		_ = app.rdb.Set(ctx, cacheKey, data, 60*time.Second).Err()
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Cache", "MISS")
	_, _ = w.Write(data)
}

func (app *CatalogApp) handleUpdateConfig(w http.ResponseWriter, r *http.Request) {
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

func (app *CatalogApp) handleFlushCache(w http.ResponseWriter, r *http.Request) {
	if !app.configMgr.Authenticate(r) {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}

	ctx := r.Context()
	keys, err := app.rdb.Keys(ctx, "catalog:*").Result()
	if err == nil && len(keys) > 0 {
		_ = app.rdb.Del(ctx, keys...).Err()
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"status":        "flushed",
		"keys_evicted": len(keys),
	})
}
