package websocket

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"sync/atomic"
	"time"

	"github.com/dahhou-ilyas/host-container/service"
	"github.com/dahhou-ilyas/host-container/utils"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/gorilla/mux"
)

type logEntry struct {
	ID        string `json:"id"`
	Timestamp string `json:"timestamp"`
	Stream    string `json:"stream"`
	Message   string `json:"message"`
}

type LogsHandler struct {
	manager *service.ContainerManager
}

func NewLogsHandler(manager *service.ContainerManager) *LogsHandler {
	return &LogsHandler{manager: manager}
}

type wsLogWriter struct {
	conn   interface{ WriteJSON(v any) error }
	stream string
	seq    *atomic.Int64
}

func (w *wsLogWriter) Write(p []byte) (int, error) {
	for _, line := range strings.Split(strings.TrimRight(string(p), "\n"), "\n") {
		if line == "" {
			continue
		}
		entry := logEntry{
			ID:        fmt.Sprintf("%d", w.seq.Add(1)),
			Timestamp: time.Now().UTC().Format(time.RFC3339),
			Stream:    w.stream,
			Message:   line,
		}
		if err := w.conn.WriteJSON(entry); err != nil {
			return 0, err
		}
	}
	return len(p), nil
}

func (lh *LogsHandler) WsHandler(w http.ResponseWriter, r *http.Request) {
	projectID := mux.Vars(r)["project_id"]
	if projectID == "" {
		utils.RespondError(w, "project_id requis", http.StatusBadRequest)
		return
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		slog.Error("logs ws upgrade failed", "error", err)
		return
	}
	defer conn.Close()

	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	// Detect client disconnect
	go func() {
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				cancel()
				return
			}
		}
	}()

	reader, err := lh.manager.StreamLogs(ctx, projectID)
	if err != nil {
		slog.Error("failed to stream logs", "error", err, "project_id", projectID)
		return
	}
	defer reader.Close()

	var seq atomic.Int64
	stdoutWriter := &wsLogWriter{conn: conn, stream: "stdout", seq: &seq}
	stderrWriter := &wsLogWriter{conn: conn, stream: "stderr", seq: &seq}

	if _, err := stdcopy.StdCopy(stdoutWriter, stderrWriter, reader); err != nil {
		slog.Debug("log streaming ended", "error", err)
	}
}
