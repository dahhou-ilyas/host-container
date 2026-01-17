package main

import (
	"context"
	"docker-wrapper/db_config"
	"docker-wrapper/middlware"
	"docker-wrapper/service"
	"errors"
	"log"
	"net/http"
	"net/http/pprof"
	"os"
	"os/signal"
	"syscall"
	"time"

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

//import (
//	"encoding/base64"
//	"encoding/json"
//	"fmt"
//	"log"
//	"net/http"
//	"os"
//	"os/exec"
//
//	"github.com/google/uuid"
//	"github.com/gorilla/mux"
//)
//
//type Code struct {
//	ID         string `json:"id,omitempty"`
//	CodeSource string `json:"codeSource,omitempty"`
//	Language   string `json:"language,omitempty"`
//}
//
//type CodeResult struct {
//	ID      string `json:"id,omitempty"`
//	Content []byte `json:"content"`
//}
//
//func decodeCodeSource(body *Code) string {
//	data, err := base64.StdEncoding.DecodeString(body.CodeSource)
//	if err != nil {
//		log.Fatal("error:", err)
//	}
//	return string(data)
//}
//
//func getExtensionAndContainerName(language string) (string, string, string) {
//	switch language {
//	case "golang":
//		return ".go", "go_box", "go run"
//	case "javascript":
//		return ".js", "node_box", "node"
//	case "python":
//		return ".py", "python_box", "python"
//	default:
//		return "unknow", "unknow", "unknow"
//	}
//}
//
//func saveFileWhithContent(filename string, codeSource string) bool {
//
//	data := []byte(codeSource)
//	err := os.WriteFile(filename, data, 0644)
//	if err != nil {
//		log.Fatal(err)
//		return false
//	}
//	return true
//}
//
//func removeFile(fileName string) {
//	e := os.Remove(fileName)
//	if e != nil {
//		log.Fatal(e)
//	}
//}
//
//func excuteDocker(fileName string, containerName string, excuter string) []byte {
//
//	cmd := exec.Command("docker", "exec", containerName, excuter, fileName)
//
//	out, err := cmd.CombinedOutput() // stdout + stderr combinés
//	if err != nil {
//		fmt.Println("Erreur:", err)
//	}
//
//	return out
//}
//
//func main() {
//	router := mux.NewRouter()
//
//	router.HandleFunc("/code", func(w http.ResponseWriter, r *http.Request) {
//		var code Code
//		err := json.NewDecoder(r.Body).Decode(&code)
//		if err != nil {
//			http.Error(w, err.Error(), http.StatusBadRequest)
//		}
//
//		// genere un ID de file name de file
//		fileName := uuid.New().String()
//		extension, containerName, excuter := getExtensionAndContainerName(code.Language)
//		if extension == "unknow" {
//			log.Fatal("Language not suportable")
//		}
//		fileName = fileName + extension
//
//		// decode le base64 code
//		sourceCodeDecoded := decodeCodeSource(&code)
//
//		// suavgarder dans le workspace
//		saveFileWhithContent(fileName, sourceCodeDecoded)
//
//		//excuté le file avec le nom random de file et retourenr la réponse
//		outpuut := excuteDocker(fileName, containerName, excuter)
//
//		// supprimé le file dans le worksapce
//		removeFile(fileName)
//
//		response := CodeResult{
//			ID:      code.ID,
//			Content: outpuut,
//		}
//
//		w.Header().Set("Content-Type", "application/json")
//		json.NewEncoder(w).Encode(response)
//	})
//
//	http.ListenAndServe(":8000", router)
//}

//import (
//	"context"
//	"log"
//	"time"
//
//	"github.com/docker/docker/api/types/container" // <-- ICI (docker/docker)
//	"github.com/moby/moby/client"                  // <-- client moby
//)
//
//func main() {
//	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
//	if err != nil {
//		log.Fatal(err)
//	}
//	defer cli.Close()
//
//	containerConfig := &container.Config{
//		Image:      "node:18-alpine",
//		Tty:        true,
//		WorkingDir: "/workspace",
//		Cmd:        []string{"/bin/sh"},
//	}
//
//	hostConfig := &container.HostConfig{
//		Resources: container.Resources{
//			Memory:   512 * 1024 * 1024,
//			NanoCPUs: 1_000_000_000,
//		},
//	}
//
//	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
//	defer cancel()
//
//	resp, err := cli.ContainerCreate(ctx, containerConfig, hostConfig, nil, nil, "")
//	if err != nil {
//		log.Fatal(err)
//	}
//
//	log.Println("created:", resp.ID)
//}

//func main() {
//	dsn := os.Getenv("DATABASE_URL") // ex: postgres://app:secret@localhost:5432/appdb?sslmode=disable
//	if err := db_config.Init(dsn); err != nil {
//		log.Fatal(err)
//	}
//	defer db_config.Close()
//
//	// Arrêt propre
//	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
//	defer stop()
//
//	// Exemples d’usage
//	_, err := db_config.Pool().Exec(ctx, `create table if not exists items(id serial primary key, label text)`)
//	if err != nil {
//		log.Fatal(err)
//	}
//}

func main() {

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
		basePath = "/Users/ilyasdahhou/Downloads/docker-wrapper/projectExemple"
	}

	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		dsn = "postgres://postgres:postgres@localhost:5432/docker_wrapper?sslmode=disable"
	}

	if err := db_config.Init(dsn); err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	
	handler, err := service.NewHandler(basePath, db_config.Pool())
	if err != nil {
		log.Fatalf("Failed to create handler: %v", err)
	}

	userHandler := service.NewUserHandler(db_config.Pool())

	defer func() {
		handler.Close()
		db_config.Close()
	}()

	//################################" Admin App ############################################"
		// ---- Admin mux (monitoring endpoints)
	adminMux := http.NewServeMux()

	// /metrics (Prometheus)
	// promhttp.Handler() expose les métriques enregistrées :contentReference[oaicite:2]{index=2}
	adminMux.Handle("/metrics", promhttp.Handler())

	// Health endpoints (k8s/load balancer)
	adminMux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})
	adminMux.HandleFunc("/readyz", func(w http.ResponseWriter, r *http.Request) {
		// Ici tu mets tes checks (DB, redis, dépendances…)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ready"))
	})

	// pprof (profiling). À sécuriser (réseau interne / auth) :contentReference[oaicite:3]{index=3}
	// On enregistre pprof sur NOTRE adminMux (pas le DefaultServeMux)
	adminMux.HandleFunc("/debug/pprof/", pprof.Index)
	adminMux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	adminMux.HandleFunc("/debug/pprof/profile", pprof.Profile)
	adminMux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	adminMux.HandleFunc("/debug/pprof/trace", pprof.Trace)


	//################################" Main App ############################################"
	router := mux.NewRouter()

	router.Handle("/ping", instrumentRoute("ping", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("pong"))
	}), httpMetrics))

	router.HandleFunc("/auth/login", userHandler.Login)
	router.HandleFunc("/auth/register", userHandler.Register)
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

	server := &http.Server{
		Addr:    appAddr,
		Handler: router,
	}

	adminSrv := &http.Server{
		Addr:              adminAddr,
		Handler:           adminMux,
		ReadHeaderTimeout: 5 * time.Second,
	}	

	go func() {
		log.Printf("Server starting on "+adminAddr)
		if err := adminSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatal("admin server error", "err", err)
			stop()
		}
	}()


	go func() {
		log.Printf("Server starting on :"+appAddr)
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

func instrumentRoute(routeName string, next http.Handler, m *httpMetrics) http.Handler {
	// 1) Tracing auto (otelhttp) :contentReference[oaicite:5]{index=5}
	h := otelhttp.NewHandler(next, routeName)

	// 2) Prometheus middleware (counter/duration/in-flight) via promhttp :contentReference[oaicite:6]{index=6}
	h = promhttp.InstrumentHandlerInFlight(m.inFlight, h)
	h = promhttp.InstrumentHandlerCounter(m.reqTotal.MustCurryWith(prometheus.Labels{"handler": routeName}), h)
	h = promhttp.InstrumentHandlerDuration(m.reqDuration.MustCurryWith(prometheus.Labels{"handler": routeName}), h)

	return h
}