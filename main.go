package main

import (
	"docker-wrapper/db_config"
	"docker-wrapper/middlware"
	"docker-wrapper/service"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/gorilla/mux"
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
	basePath := os.Getenv("PROJECTS_BASE_PATH")
	if basePath == "" {
		basePath = "/tmp/projects"
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

	router := mux.NewRouter()

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
		Addr:    ":8000",
		Handler: router,
	}

	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		<-sigChan
		log.Println("Shutting down server...")
		server.Close()
	}()

	log.Printf("Server starting on :8000")
	log.Printf("Projects base path: %s", basePath)

	if err := server.ListenAndServe(); err != http.ErrServerClosed {
		log.Fatalf("Server error: %v", err)
	}
}
