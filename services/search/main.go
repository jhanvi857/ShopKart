package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"

	"shopkart/pkg/config"
	"shopkart/pkg/health"
	"shopkart/pkg/logging"
	"shopkart/pkg/metrics"
	shopkartmw "shopkart/pkg/middleware"
	"shopkart/pkg/tracing"
)

type SearchApp struct {
	catalogURL string
	httpClient *http.Client
	tracer     trace.Tracer
	metrics    *metrics.Metrics
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
	cfg := config.LoadCommon("search", "9002")
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

	catalogURL := config.GetEnv("CATALOG_URL", "http://localhost:9001")

	app := &SearchApp{
		catalogURL: catalogURL,
		httpClient: &http.Client{Timeout: 5 * time.Second},
		tracer:     tracer,
		metrics:    m,
	}

	checker := health.NewChecker()
	checker.Register("catalog", func(ctx context.Context) error {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, catalogURL+"/healthz", nil)
		if err != nil {
			return err
		}
		resp, err := app.httpClient.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("catalog healthz returned %d", resp.StatusCode)
		}
		return nil
	})

	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)
	r.Use(shopkartmw.ObservabilityMiddleware(cfg.ServiceName, m))

	r.Get("/healthz", checker.HealthzHandler())
	r.Get("/readyz", checker.ReadyzHandler())
	r.Handle("/metrics", m.Handler())

	r.Get("/search", app.handleSearch)

	server := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      r,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	go func() {
		logger.Info("Starting search service", slog.String("port", cfg.Port))
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
	logger.Info("Search service shutdown gracefully")
}

func (app *SearchApp) handleSearch(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	ctx, span := app.tracer.Start(ctx, "SearchApp.Search")
	defer span.End()

	q := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("q")))
	cat := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("category")))
	minPriceStr := r.URL.Query().Get("minPrice")
	maxPriceStr := r.URL.Query().Get("maxPrice")

	var minPrice, maxPrice float64
	if minPriceStr != "" {
		minPrice, _ = strconv.ParseFloat(minPriceStr, 64)
	}
	if maxPriceStr != "" {
		maxPrice, _ = strconv.ParseFloat(maxPriceStr, 64)
	}

	catReq, err := http.NewRequestWithContext(ctx, http.MethodGet, app.catalogURL+"/catalog/products", nil)
	if err != nil {
		http.Error(w, `{"error":"failed to build catalog request"}`, http.StatusInternalServerError)
		return
	}
	tracing.InjectHTTPHeaders(ctx, catReq)

	catResp, err := app.httpClient.Do(catReq)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"failed to reach catalog: %s"}`, err.Error()), http.StatusBadGateway)
		return
	}
	defer catResp.Body.Close()

	var allProducts []Product
	if err := json.NewDecoder(catResp.Body).Decode(&allProducts); err != nil {
		http.Error(w, `{"error":"failed to parse catalog response"}`, http.StatusBadGateway)
		return
	}

	matched := make([]Product, 0)
	for _, p := range allProducts {
		if q != "" {
			nameMatch := strings.Contains(strings.ToLower(p.Name), q)
			descMatch := strings.Contains(strings.ToLower(p.Description), q)
			if !nameMatch && !descMatch {
				continue
			}
		}

		if cat != "" && !strings.EqualFold(p.Category, cat) {
			continue
		}

		if minPrice > 0 && p.Price < minPrice {
			continue
		}

		if maxPrice > 0 && p.Price > maxPrice {
			continue
		}

		matched = append(matched, p)
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"query":    q,
		"category": cat,
		"count":    len(matched),
		"results":  matched,
	})
}
