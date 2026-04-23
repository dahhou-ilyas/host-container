package main

import (
	"context"
	"errors"
	"log"
	"log/slog"
	"net/http"
	"net/http/pprof"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/dahhou-ilyas/host-container/db_config"
	"github.com/dahhou-ilyas/host-container/middlware"
	"github.com/dahhou-ilyas/host-container/service"
	"github.com/dahhou-ilyas/host-container/websocket"
	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

func main() {
	// Structured JSON logging for production
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})))

	appAddr := os.Getenv("APP_ADDR")

	adminAddr := os.Getenv("ADMIN_ADDR")

	serviceName := os.Getenv("OTEL_SERVICE_NAME")

	if serviceName == "" {
		serviceName = "my-go-service"
	}
	if appAddr == "" {
		appAddr = ":8080"
	}
	if  adminAddr == "" {
		adminAddr = ":2112"
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	// ---- OpenTelemetry (traces)
	shutdownOtel, err := initOpenTelemetry(ctx, serviceName)
	if err != nil {
		log.Fatal("otel init failed", "err", err)
		return
	}
	defer func() {
		_ = shutdownOtel(context.Background())
	}()

	httpMetrics := newHTTPMetrics()
	prometheus.MustRegister(httpMetrics.inFlight, httpMetrics.reqTotal, httpMetrics.reqDuration)


	basePath := os.Getenv("PROJECTS_BASE_PATH")
	if basePath == "" {
		basePath = "/tmp/projects"
	}

	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		log.Fatal("DATABASE_URL environment variable must be set")
	}

	autoStopTimeoutStr := os.Getenv("CONTAINER_AUTO_STOP_TIMEOUT")
	if autoStopTimeoutStr == "" {
		autoStopTimeoutStr = "30m"
	}
	autoStopTimeout, err := time.ParseDuration(autoStopTimeoutStr)
	if err != nil {
		log.Fatalf("Invalid CONTAINER_AUTO_STOP_TIMEOUT value %q: %v", autoStopTimeoutStr, err)
	}

	if err := db_config.Init(dsn); err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}

	if err := runMigrations(dsn); err != nil {
		log.Fatalf("Failed to run database migrations: %v", err)
	}

	handler, err := service.NewHandler(basePath, db_config.Pool(), autoStopTimeout)
	if err != nil {
		log.Fatalf("Failed to create handler: %v", err)
	}

	userHandler := service.NewUserHandler(db_config.Pool())

	handler.StartAutoStopWatcher()

	defer func() {
		handler.StopAutoStopWatcher()
		handler.Close()
		db_config.Close()
	}()

	//################################" Admin App ############################################"
		// ---- Admin mux (monitoring endpoints)
	adminMux := http.NewServeMux()

	// /metrics (Prometheus)
	// promhttp.Handler() expose les métriques enregistrées :contentReference[oaicite:2]{index=2}
	adminMux.Handle("/metrics", promhttp.Handler())

	// Health endpoints (k8s / load balancer)
	adminMux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		// Liveness: process is alive
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	})
	adminMux.HandleFunc("/readyz", func(w http.ResponseWriter, r *http.Request) {
		// Readiness: can we serve traffic? check DB connectivity
		w.Header().Set("Content-Type", "application/json")
		if err := db_config.Pool().Ping(r.Context()); err != nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			w.Write([]byte(`{"status":"unavailable","reason":"database unreachable"}`))
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ready"}`))
	})

	// pprof only enabled when ENABLE_PPROF=true (never expose in production without network restriction)
	if os.Getenv("ENABLE_PPROF") == "true" {
		adminMux.HandleFunc("/debug/pprof/", pprof.Index)
		adminMux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
		adminMux.HandleFunc("/debug/pprof/profile", pprof.Profile)
		adminMux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
		adminMux.HandleFunc("/debug/pprof/trace", pprof.Trace)
		log.Printf("pprof profiling enabled on %s/debug/pprof/", adminAddr)
	}


	//################################" Main App ############################################"
	router := mux.NewRouter()

	// Global middleware stack (outermost first)
	router.Use(middlware.Recovery)
	router.Use(middlware.NewIPRateLimiter(10, 30).Middleware)
	router.Use(middlware.RequestLogger)
	router.Use(prometheusMiddleware(httpMetrics))

	router.HandleFunc("/ping", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("pong"))
	})

	router.HandleFunc("/auth/login", userHandler.Login)
	router.HandleFunc("/auth/register", userHandler.Register)
	router.HandleFunc("/auth/refresh", userHandler.RefreshToken)
	router.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	})

	router.HandleFunc("/user/me", middlware.AuthMiddleware(userHandler.GetUser))
	router.HandleFunc("/user/containers", middlware.AuthMiddleware(userHandler.GetUserWithContainers))
	router.HandleFunc("/user", middlware.AuthMiddleware(userHandler.DeleteUser))

	router.HandleFunc("/containers", middlware.AuthMiddleware(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			//handler.ListContainers(w, r)
		case http.MethodPost:
			handler.CreateContainer(w, r)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	}))
	router.HandleFunc("/containers/get", middlware.AuthMiddleware(handler.GetContainer))
	router.HandleFunc("/containers/start", middlware.AuthMiddleware(handler.StartContainer))
	router.HandleFunc("/containers/stop", middlware.AuthMiddleware(handler.StopContainer))
	router.HandleFunc("/containers/remove", middlware.AuthMiddleware(handler.RemoveContainer))
	router.HandleFunc("/containers/exec", middlware.AuthMiddleware(handler.ExecCommand))

	router.HandleFunc("/containers/showTreeFolder",middlware.AuthMiddleware(handler.TreeFolder))
	router.HandleFunc("/containers/file/read", middlware.AuthMiddleware(handler.ReadFile))
	router.HandleFunc("/containers/file/write", middlware.AuthMiddleware(handler.WriteFile))

	metricHandler, err := websocket.NewMetricHandler(handler.Manager())
	if err != nil {
		log.Fatalf("Failed to create metric handler: %v", err)
	}
	router.HandleFunc("/ws", middlware.AuthMiddleware(metricHandler.WsHandler))

	server := &http.Server{
		Addr:    appAddr,
		Handler: middlware.CORSMiddleware(router),
	}

	adminSrv := &http.Server{
		Addr:              adminAddr,
		Handler:           adminMux,
		ReadHeaderTimeout: 5 * time.Second,
	}	

	go func() {
		log.Printf("Server starting on %s", adminAddr)
		if err := adminSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatal("admin server error", "err", err)
			stop()
		}
	}()


	go func() {
		log.Printf("Server starting on %s", appAddr)

		log.Printf("Projects base path: %s", basePath)
		if err := server.ListenAndServe(); err != http.ErrServerClosed {
			log.Fatalf("Server error: %v", err)
			stop()
		}
	}()

	<-ctx.Done()
	log.Printf("Shutting down servers...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = server.Shutdown(shutdownCtx)
	_ = adminSrv.Shutdown(shutdownCtx)
	
}



// ---------- Observability helpers ----------

func initOpenTelemetry(ctx context.Context, serviceName string) (func(context.Context) error, error) {
	// Export OTLP/HTTP (configurable via env vars)
	// OTEL_EXPORTER_OTLP_ENDPOINT est supporté (et /v1/traces est append en OTLP/HTTP) :contentReference[oaicite:4]{index=4}
	exp, err := otlptracehttp.New(ctx)
	if err != nil {
		return nil, err
	}

	res, err := resource.New(ctx,
		resource.WithAttributes(
			attribute.String("service.name", serviceName),
		),
	)
	if err != nil {
		return nil, err
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exp),
		sdktrace.WithResource(res),
	)
	otel.SetTracerProvider(tp)

	// Propagation W3C (TraceContext + Baggage)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	return tp.Shutdown, nil
}

type httpMetrics struct {
	inFlight    prometheus.Gauge
	reqTotal    *prometheus.CounterVec
	reqDuration *prometheus.HistogramVec
}



func newHTTPMetrics() *httpMetrics {
	return &httpMetrics{
		inFlight: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "http_in_flight_requests",
			Help: "Current number of in-flight HTTP requests.",
		}),
		reqTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total number of HTTP requests.",
		}, []string{"handler", "code", "method"}),
		reqDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "Duration of HTTP requests in seconds.",
			Buckets: prometheus.DefBuckets,
		}, []string{"handler", "code", "method"}),
	}
}

// prometheusMiddleware instruments ALL routes automatically using Gorilla Mux route templates as labels.
func prometheusMiddleware(m *httpMetrics) mux.MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			routeName := r.URL.Path
			if route := mux.CurrentRoute(r); route != nil {
				if tpl, err := route.GetPathTemplate(); err == nil {
					routeName = tpl
				}
			}
			h := otelhttp.NewHandler(next, routeName)
			h = promhttp.InstrumentHandlerInFlight(m.inFlight, h)
			h = promhttp.InstrumentHandlerCounter(
				m.reqTotal.MustCurryWith(prometheus.Labels{"handler": routeName}), h,
			)
			h = promhttp.InstrumentHandlerDuration(
				m.reqDuration.MustCurryWith(prometheus.Labels{"handler": routeName}), h,
			)
			h.ServeHTTP(w, r)
		})
	}
}




func parseTree(tree string){


}