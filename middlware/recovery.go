package middlware

import (
	"log/slog"
	"net/http"
	"runtime/debug"
)

// Recovery catches panics in HTTP handlers, logs the stack trace, and returns a 500.
func Recovery(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				slog.Error("panic recovered",
					"error", err,
					"method", r.Method,
					"path", r.URL.Path,
					"stack", string(debug.Stack()),
				)
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusInternalServerError)
				w.Write([]byte(`{"success":false,"error":"internal server error"}`))
			}
		}()
		next.ServeHTTP(w, r)
	})
}
