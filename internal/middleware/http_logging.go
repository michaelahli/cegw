package middleware

import (
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/michaelahli/cegw/internal/logger"
)

// HTTPLoggingMiddleware logs HTTP requests and responses with structured data
func HTTPLoggingMiddleware(log *logger.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requestID := r.Header.Get("X-Request-ID")
			if requestID == "" {
				requestID = uuid.New().String()
			}

			traceID := r.Header.Get("X-Trace-ID")
			if traceID == "" {
				traceID = uuid.New().String()
			}

			ctx := logger.ContextWithRequestID(r.Context(), requestID)
			ctx = logger.ContextWithTraceID(ctx, traceID)

			reqLog := log.WithContext(ctx).
				WithFields(map[string]interface{}{
					"http_method": r.Method,
					"http_path":   r.RequestURI,
					"http_proto":  r.Proto,
					"remote_addr": r.RemoteAddr,
					"user_agent":  r.UserAgent(),
				})

			reqLog.Infof("HTTP request started")

			// Wrap response writer to capture status and size
			wrapped := &responseWriter{
				ResponseWriter: w,
				statusCode:     http.StatusOK,
			}

			start := time.Now()
			next.ServeHTTP(wrapped, r.WithContext(ctx))
			duration := time.Since(start)

			reqLog.
				WithField("http_status", wrapped.statusCode).
				WithField("response_bytes", wrapped.bytesWritten).
				WithDuration(duration).
				Infof("HTTP request completed")
		})
	}
}

// responseWriter wraps http.ResponseWriter to capture status code and bytes written
type responseWriter struct {
	http.ResponseWriter
	statusCode   int
	bytesWritten int
}

func (w *responseWriter) WriteHeader(statusCode int) {
	w.statusCode = statusCode
	w.ResponseWriter.WriteHeader(statusCode)
}

func (w *responseWriter) Write(b []byte) (int, error) {
	n, err := w.ResponseWriter.Write(b)
	w.bytesWritten += n
	return n, err
}
