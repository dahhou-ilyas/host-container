package middlware

import (
	"net/http"
	"os"
	"strings"
)

// CORSMiddleware sets CORS headers based on the ALLOWED_ORIGINS env var.
// Origins must be comma-separated (e.g. "https://app.example.com,https://other.example.com").
// If ALLOWED_ORIGINS is empty, all cross-origin requests are rejected.
func CORSMiddleware(next http.Handler) http.Handler {
	allowedOrigins := parseAllowedOriginsHTTP(os.Getenv("ALLOWED_ORIGINS"))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")

		if origin != "" && allowedOrigins[origin] {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS, PATCH")
			w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
			w.Header().Set("Access-Control-Max-Age", "86400")
			w.Header().Set("Vary", "Origin")
		}

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func parseAllowedOriginsHTTP(raw string) map[string]bool {
	set := make(map[string]bool)
	for _, o := range strings.Split(raw, ",") {
		o = strings.TrimSpace(o)
		if o != "" {
			set[o] = true
		}
	}
	return set
}
