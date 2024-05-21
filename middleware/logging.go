package middleware

import (
	"log/slog"
	"net/http"
	"time"
)

func NewLoggingMiddleware(logger *slog.Logger) func(http.HandlerFunc) http.HandlerFunc {
	return func(next http.HandlerFunc) http.HandlerFunc {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			wrappedWriter := newResponseWriter(w)
			t1 := time.Now()
			next.ServeHTTP(wrappedWriter, r)
			t2 := time.Now()

			statusCode := wrappedWriter.statusCode
			path := r.URL.Path

			if statusCode < 500 {
				logger.InfoContext(r.Context(),
					"request",
					"status", statusCode,
					"path", path,
					"duration", t2.Sub(t1),
				)
			} else {
				logger.ErrorContext(r.Context(),
					"error",
					"status", statusCode,
					"path", path,
					"duration", t2.Sub(t1),
				)
			}
		})
	}
}

type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func newResponseWriter(w http.ResponseWriter) *responseWriter {
	return &responseWriter{w, http.StatusOK}
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}
