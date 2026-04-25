package websocket

import (
	"net/http"
	"os"
	"strings"

	"github.com/gorilla/websocket"
)

var allowedWsOrigins = parseAllowedOrigins(os.Getenv("ALLOWED_ORIGINS"))

func parseAllowedOrigins(raw string) map[string]bool {
	set := make(map[string]bool)
	for _, o := range strings.Split(raw, ",") {
		o = strings.TrimSpace(o)
		if o != "" {
			set[o] = true
		}
	}
	return set
}

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		origin := r.Header.Get("Origin")
		if origin == "" {
			return true
		}
		return allowedWsOrigins[origin]
	},
}
