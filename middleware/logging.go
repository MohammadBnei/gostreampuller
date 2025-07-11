package middleware

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5/middleware" // Import chi's middleware package for RequestIDKey

	"gostreampuller/config"
)

// LoggingMiddleware logs details about each HTTP request.
func LoggingMiddleware(cfg *config.Config) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !cfg.DebugMode {
				// If debug mode is not enabled, just call the next handler.
				next.ServeHTTP(w, r)
				return
			}

			start := time.Now()
			recorder := &responseRecorder{ResponseWriter: w, statusCode: http.StatusOK} // Default to 200 OK

			// Log request information
			slog.Info("Request received",
				"method", r.Method,
				"path", r.URL.Path,
				"remote_addr", r.RemoteAddr,
				"user_agent", r.UserAgent(),
				"request_id", r.Context().Value(middleware.RequestIDKey), // Assuming chi's RequestID middleware is used
			)

			next.ServeHTTP(recorder, r)

			// Log response information
			slog.Info("Request completed",
				"method", r.Method,
				"path", r.URL.Path,
				"status", recorder.statusCode,
				"size", recorder.size,
				"duration", time.Since(start),
				"request_id", r.Context().Value(middleware.RequestIDKey),
			)
		})
	}
}

// responseRecorder is a wrapper around http.ResponseWriter that captures the status code and size.
// It also implements http.Flusher to pass through Flush calls.
type responseRecorder struct {
	http.ResponseWriter
	statusCode int
	size       int
}

// WriteHeader captures the status code before calling the underlying WriteHeader.
func (r *responseRecorder) WriteHeader(statusCode int) {
	r.statusCode = statusCode
	r.ResponseWriter.WriteHeader(statusCode)
}

// Write captures the size of the response body before calling the underlying Write.
func (r *responseRecorder) Write(b []byte) (int, error) {
	size, err := r.ResponseWriter.Write(b)
	r.size += size
	return size, err
}

// Flush implements the http.Flusher interface.
// It checks if the underlying ResponseWriter is an http.Flusher and calls its Flush method.
func (r *responseRecorder) Flush() {
	if flusher, ok := r.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}
