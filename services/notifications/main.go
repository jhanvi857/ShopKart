package main

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/segmentio/kafka-go"
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

type NotificationMessage struct {
	OrderID         string  `json:"order_id"`
	CustomerName    string  `json:"customer_name"`
	CustomerEmail   string  `json:"customer_email"`
	TotalAmount     float64 `json:"total_amount"`
	Currency        string  `json:"currency"`
	Status          string  `json:"status"`
}

type NotificationApp struct {
	kafkaReader    *kafka.Reader
	configMgr      *admin.ConfigManager
	tracer         trace.Tracer
	metrics        *metrics.Metrics
	activeWorkers  int64
	workerCapacity int64
	processDelayMs int64
}

func main() {
	cfg := config.LoadCommon("notifications", "9007")
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

	kafkaBroker := config.GetEnv("KAFKA_BROKER", "localhost:9094")
	topic := "shopkart-orders"
	groupID := "shopkart-notifications-group"

	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:        []string{kafkaBroker},
		GroupID:        groupID,
		Topic:          topic,
		MinBytes:       10e3, // 10KB
		MaxBytes:       10e6, // 10MB
		CommitInterval: 1 * time.Second,
	})
	defer reader.Close()

	workerCount := int64(config.GetEnvInt("NOTIFICATION_WORKERS", 2))
	processDelay := int64(config.GetEnvInt("NOTIFICATION_DELAY_MS", 35))

	app := &NotificationApp{
		kafkaReader:    reader,
		configMgr:      admin.NewConfigManager(cfg.AdminAPIKey, cfg.FaultEndpoints),
		tracer:         tracer,
		metrics:        m,
		workerCapacity: workerCount,
		processDelayMs: processDelay,
	}
	app.configMgr.Set("notification_workers", workerCount)
	app.configMgr.Set("process_delay_ms", processDelay)

	checker := health.NewChecker()
	checker.Register("kafka_reader", func(ctx context.Context) error {
		// Verify reader is active
		stats := reader.Stats()
		if stats.Topic == "" {
			return nil
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

	r.Get("/admin/status", app.configMgr.HandleStatus)
	r.Post("/admin/config", app.handleUpdateConfig)

	server := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      r,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	go func() {
		logger.Info("Starting notifications service", slog.String("port", cfg.Port))
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("Server error", slog.String("error", err.Error()))
			os.Exit(1)
		}
	}()

	// Start Kafka consumer worker loop
	msgQueue := make(chan kafka.Message, 100)
	go app.startReaderLoop(ctx, msgQueue)
	go app.startWorkerPool(ctx, msgQueue)

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	<-sigCh

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer shutdownCancel()
	_ = server.Shutdown(shutdownCtx)
	logger.Info("Notifications service shutdown gracefully")
}

func (app *NotificationApp) startReaderLoop(ctx context.Context, msgQueue chan<- kafka.Message) {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				stats := app.kafkaReader.Stats()
				app.metrics.ConsumerLag.WithLabelValues("shopkart-orders", "shopkart-notifications-group").Set(float64(stats.Lag))
				app.metrics.QueueDepth.WithLabelValues("notification_queue").Set(float64(len(msgQueue)))
			}
		}
	}()

	for {
		msg, err := app.kafkaReader.FetchMessage(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			time.Sleep(500 * time.Millisecond)
			continue
		}

		select {
		case msgQueue <- msg:
		case <-ctx.Done():
			return
		}
	}
}

func (app *NotificationApp) startWorkerPool(ctx context.Context, msgQueue <-chan kafka.Message) {
	for {
		select {
		case <-ctx.Done():
			return
		case msg := <-msgQueue:
			// Throttle according to workerCapacity
			capacity := atomic.LoadInt64(&app.workerCapacity)
			for atomic.LoadInt64(&app.activeWorkers) >= capacity {
				time.Sleep(10 * time.Millisecond)
			}

			atomic.AddInt64(&app.activeWorkers, 1)
			app.updateWorkerMetric()

			go func(m kafka.Message) {
				defer func() {
					atomic.AddInt64(&app.activeWorkers, -1)
					app.updateWorkerMetric()
				}()

				app.processMessage(ctx, m)
				_ = app.kafkaReader.CommitMessages(ctx, m)
			}(msg)
		}
	}
}

func (app *NotificationApp) updateWorkerMetric() {
	active := atomic.LoadInt64(&app.activeWorkers)
	capacity := atomic.LoadInt64(&app.workerCapacity)
	if capacity > 0 {
		ratio := float64(active) / float64(capacity)
		app.metrics.WorkerPoolUtilization.WithLabelValues("notification_workers").Set(ratio)
	}
}

func (app *NotificationApp) processMessage(ctx context.Context, msg kafka.Message) {
	// Extract OpenTelemetry trace from Kafka headers
	msgCtx := tracing.ExtractKafkaHeaders(ctx, msg.Headers)
	msgCtx, span := app.tracer.Start(msgCtx, "NotificationApp.ProcessNotification")
	defer span.End()

	var order NotificationMessage
	if err := json.Unmarshal(msg.Value, &order); err != nil {
		slog.ErrorContext(msgCtx, "failed to parse order message", slog.String("error", err.Error()))
		return
	}

	delay := time.Duration(atomic.LoadInt64(&app.processDelayMs)) * time.Millisecond
	if delay > 0 {
		time.Sleep(delay)
	}

	slog.InfoContext(msgCtx, "dispatched order confirmation notification",
		slog.String("order_id", order.OrderID),
		slog.String("email", order.CustomerEmail),
		slog.Float64("total", order.TotalAmount),
	)
}

func (app *NotificationApp) handleUpdateConfig(w http.ResponseWriter, r *http.Request) {
	if !app.configMgr.Authenticate(r) {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}

	var updates struct {
		NotificationWorkers *int64 `json:"notification_workers"`
		ProcessDelayMs      *int64 `json:"process_delay_ms"`
	}
	if err := json.NewDecoder(r.Body).Decode(&updates); err != nil {
		http.Error(w, `{"error":"invalid body"}`, http.StatusBadRequest)
		return
	}

	if updates.NotificationWorkers != nil && *updates.NotificationWorkers > 0 {
		atomic.StoreInt64(&app.workerCapacity, *updates.NotificationWorkers)
		app.configMgr.Set("notification_workers", *updates.NotificationWorkers)
	}
	if updates.ProcessDelayMs != nil && *updates.ProcessDelayMs >= 0 {
		atomic.StoreInt64(&app.processDelayMs, *updates.ProcessDelayMs)
		app.configMgr.Set("process_delay_ms", *updates.ProcessDelayMs)
	}

	app.configMgr.HandleStatus(w, r)
}
