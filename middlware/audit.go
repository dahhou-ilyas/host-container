package middlware

import (
	"log/slog"
	"net/http"

	"github.com/dahhou-ilyas/host-container/repository"
	"github.com/dahhou-ilyas/host-container/utils"
)

// AuditMiddleware logs authenticated actions to the audit_log table.
// It wraps a handler and records action + resource derived from the route.
func AuditMiddleware(repo *repository.AuditRepository, action, resource string) func(http.HandlerFunc) http.HandlerFunc {
	return func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			next(w, r)

			userID, ok := r.Context().Value(utils.UserIDKey).(string)
			if !ok || userID == "" {
				return
			}

			entry := repository.AuditEntry{
				UserID:    userID,
				Action:    action,
				Resource:  resource,
				IP:        clientIP(r),
				UserAgent: r.Header.Get("User-Agent"),
			}
			if err := repo.Log(r.Context(), entry); err != nil {
				slog.Warn("audit log failed", "error", err, "action", action)
			}
		}
	}
}
