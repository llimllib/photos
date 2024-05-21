package middleware

import (
	"log/slog"
	"net/http"
)

// NewPanicMiddleware returns a middleware function that closes over a given
// logger. The returned middleware function recovers from a panic, logs it, and
// writes a 500 to the http stream
func NewPanicMiddleware(logger *slog.Logger) func(http.HandlerFunc) http.HandlerFunc {
	return func(next http.HandlerFunc) http.HandlerFunc {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if r := recover(); r != nil {
					logger.Error("Panic handled", "error:", r)
					http.Error(w, "Internal server error", http.StatusInternalServerError)
				}
			}()

			next.ServeHTTP(w, r)
		})
	}
}
