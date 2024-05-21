package middleware

import (
	"log/slog"
	"net/http"

	"github.com/go-llsqlite/crawshaw/sqlitex"
	"github.com/llimllib/photos/data"
)

func NewAuthRequiredMiddleware(logger *slog.Logger, db *sqlitex.Pool) func(http.HandlerFunc) http.HandlerFunc {
	return func(next http.HandlerFunc) http.HandlerFunc {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			cookie, err := r.Cookie("photos-login")
			if err != nil {
				logger.Debug("cookie error", "error", err)
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
			}
			session, err := data.LookupSession(db, cookie.Value, r.Context())
			if err != nil || session == nil {
				logger.Debug("Failed to find session", "id", cookie.Value, "err", err)
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
			}

			// In this app, we're assuming that a login == authentication for
			// now; otherwise, here we might check that the user linked to the
			// session object has the permissions to visit whatever page this
			// is

			next.ServeHTTP(w, r)
		})
	}
}
