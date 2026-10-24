package main

import (
	"context"
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
	"github.com/redis/go-redis/v9"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"

	"shopkart/pkg/config"
	"shopkart/pkg/health"
	"shopkart/pkg/logging"
	"shopkart/pkg/metrics"
	shopkartmw "shopkart/pkg/middleware"
	"shopkart/pkg/tracing"
)

type CartApp struct {
	rdb     *redis.Client
	tracer  trace.Tracer
	metrics *metrics.Metrics
}

type CartItem struct {
	ProductID string  `json:"product_id"`
	Name      string  `json:"name"`
	Price     float64 `json:"price"`
	Quantity  int     `json:"quantity"`
}

type Cart struct {
	CartID    string     `json:"cart_id"`
	Items     []CartItem `json:"items"`
	Total     float64    `json:"total"`
	UpdatedAt time.Time  `json:"updated_at"`
}

func main() {
	cfg := config.LoadCommon("cart", "9006")
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

	redisAddr := config.GetEnv("REDIS_ADDR", "localhost:6380")
	rdb := redis.NewClient(&redis.Options{
		Addr: redisAddr,
	})
	defer rdb.Close()

	app := &CartApp{
		rdb:     rdb,
		tracer:  tracer,
		metrics: m,
	}

	checker := health.NewChecker()
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

	r.Get("/{cartId}", app.handleGetCart)
	r.Get("/cart/{cartId}", app.handleGetCart)
	r.Post("/{cartId}/items", app.handleAddItem)
	r.Post("/cart/{cartId}/items", app.handleAddItem)
	r.Delete("/{cartId}/items/{productId}", app.handleRemoveItem)
	r.Delete("/cart/{cartId}/items/{productId}", app.handleRemoveItem)
	r.Delete("/{cartId}", app.handleClearCart)
	r.Delete("/cart/{cartId}", app.handleClearCart)

	server := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      r,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	go func() {
		logger.Info("Starting cart service", slog.String("port", cfg.Port))
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
	logger.Info("Cart service shutdown gracefully")
}

func (app *CartApp) getCartKey(cartID string) string {
	return fmt.Sprintf("cart:%s", cartID)
}

func (app *CartApp) fetchCart(ctx context.Context, cartID string) (Cart, error) {
	cart := Cart{
		CartID:    cartID,
		Items:     []CartItem{},
		Total:     0,
		UpdatedAt: time.Now().UTC(),
	}

	val, err := app.rdb.Get(ctx, app.getCartKey(cartID)).Result()
	if err == redis.Nil {
		return cart, nil
	} else if err != nil {
		return cart, err
	}

	if err := json.Unmarshal([]byte(val), &cart); err != nil {
		return cart, err
	}

	return cart, nil
}

func (app *CartApp) saveCart(ctx context.Context, cart Cart) error {
	var total float64
	for _, item := range cart.Items {
		total += item.Price * float64(item.Quantity)
	}
	cart.Total = total
	cart.UpdatedAt = time.Now().UTC()

	data, err := json.Marshal(cart)
	if err != nil {
		return err
	}

	return app.rdb.Set(ctx, app.getCartKey(cart.CartID), data, 7*24*time.Hour).Err()
}

func (app *CartApp) handleGetCart(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	cartID := chi.URLParam(r, "cartId")

	cart, err := app.fetchCart(ctx, cartID)
	if err != nil {
		http.Error(w, `{"error":"failed to fetch cart"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(cart)
}

func (app *CartApp) handleAddItem(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	cartID := chi.URLParam(r, "cartId")

	var item CartItem
	if err := json.NewDecoder(r.Body).Decode(&item); err != nil || item.ProductID == "" || item.Quantity <= 0 {
		http.Error(w, `{"error":"invalid item"}`, http.StatusBadRequest)
		return
	}

	cart, err := app.fetchCart(ctx, cartID)
	if err != nil {
		http.Error(w, `{"error":"database error"}`, http.StatusInternalServerError)
		return
	}

	found := false
	for i, existing := range cart.Items {
		if existing.ProductID == item.ProductID {
			cart.Items[i].Quantity += item.Quantity
			if item.Price > 0 {
				cart.Items[i].Price = item.Price
			}
			found = true
			break
		}
	}
	if !found {
		cart.Items = append(cart.Items, item)
	}

	if err := app.saveCart(ctx, cart); err != nil {
		http.Error(w, `{"error":"failed to update cart"}`, http.StatusInternalServerError)
		return
	}

	cart, _ = app.fetchCart(ctx, cartID)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(cart)
}

func (app *CartApp) handleRemoveItem(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	cartID := chi.URLParam(r, "cartId")
	productID := chi.URLParam(r, "productId")

	cart, err := app.fetchCart(ctx, cartID)
	if err != nil {
		http.Error(w, `{"error":"database error"}`, http.StatusInternalServerError)
		return
	}

	newItems := make([]CartItem, 0, len(cart.Items))
	for _, item := range cart.Items {
		if item.ProductID != productID {
			newItems = append(newItems, item)
		}
	}
	cart.Items = newItems

	if err := app.saveCart(ctx, cart); err != nil {
		http.Error(w, `{"error":"failed to update cart"}`, http.StatusInternalServerError)
		return
	}

	cart, _ = app.fetchCart(ctx, cartID)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(cart)
}

func (app *CartApp) handleClearCart(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	cartID := chi.URLParam(r, "cartId")

	_ = app.rdb.Del(ctx, app.getCartKey(cartID)).Err()
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(`{"status":"cleared"}`))
}
