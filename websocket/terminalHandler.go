package websocket

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/dahhou-ilyas/host-container/service"
	"github.com/dahhou-ilyas/host-container/utils"
	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
)

type resizeMsg struct {
	Type string `json:"type"`
	Cols uint   `json:"cols"`
	Rows uint   `json:"rows"`
}

type TerminalHandler struct {
	manager *service.ContainerManager
}

func NewTerminalHandler(manager *service.ContainerManager) *TerminalHandler {
	return &TerminalHandler{manager: manager}
}

func (th *TerminalHandler) WsHandler(w http.ResponseWriter, r *http.Request) {
	projectID := mux.Vars(r)["project_id"]
	if projectID == "" {
		utils.RespondError(w, "project_id requis", http.StatusBadRequest)
		return
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		slog.Error("terminal ws upgrade failed", "error", err)
		return
	}
	defer conn.Close()

	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	execID, dockerConn, dockerReader, err := th.manager.ExecTerminal(ctx, projectID)
	if err != nil {
		slog.Error("failed to start terminal exec", "error", err, "project_id", projectID)
		_ = conn.WriteMessage(websocket.TextMessage, []byte("\r\n\x1b[31mFailed to start terminal: "+err.Error()+"\x1b[0m\r\n"))
		return
	}
	defer dockerConn.Close()

	// Docker stdout/stderr → WebSocket
	go func() {
		defer cancel()
		buf := make([]byte, 4096)
		for {
			n, readErr := dockerReader.Read(buf)
			if n > 0 {
				if writeErr := conn.WriteMessage(websocket.BinaryMessage, buf[:n]); writeErr != nil {
					return
				}
			}
			if readErr != nil {
				return
			}
		}
	}()

	// WebSocket input → Docker stdin (or terminal resize)
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		_, data, err := conn.ReadMessage()
		if err != nil {
			return
		}

		var resize resizeMsg
		if json.Unmarshal(data, &resize) == nil && resize.Type == "resize" && resize.Cols > 0 && resize.Rows > 0 {
			_ = th.manager.ResizeTerminal(ctx, execID, resize.Cols, resize.Rows)
			continue
		}

		if _, err := dockerConn.Write(data); err != nil {
			return
		}
	}
}
