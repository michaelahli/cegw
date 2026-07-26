package middleware

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"time"

	"github.com/michaelahli/cegw/internal/logger"
	"google.golang.org/grpc"
	"google.golang.org/grpc/status"
)

// bodyLogResponseWriter wraps http.ResponseWriter to capture the response body.
type bodyLogResponseWriter struct {
	http.ResponseWriter
	statusCode int
	body       bytes.Buffer
}

func (w *bodyLogResponseWriter) WriteHeader(statusCode int) {
	w.statusCode = statusCode
	w.ResponseWriter.WriteHeader(statusCode)
}

func (w *bodyLogResponseWriter) Write(b []byte) (int, error) {
	w.body.Write(b)
	return w.ResponseWriter.Write(b)
}

// HTTPBodyLoggingMiddleware logs HTTP request and response bodies at trace level.
// Only produces output when LOG_LEVEL=trace is set.
// Use this for debugging purposes only — it can expose sensitive data and
// adds overhead for large payloads.
func HTTPBodyLoggingMiddleware(log *logger.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Skip health check endpoints to reduce noise
			if r.URL.Path == "/healthz" || r.URL.Path == "/readyz" {
				next.ServeHTTP(w, r)
				return
			}

			// Read and log request body
			var reqBody []byte
			if r.Body != nil {
				reqBody, _ = io.ReadAll(r.Body)
				r.Body = io.NopCloser(bytes.NewBuffer(reqBody))
			}

			entry := log.WithContext(r.Context()).
				WithFields(map[string]interface{}{
					"http_method": r.Method,
					"http_path":   r.RequestURI,
				})

			if len(reqBody) > 0 {
				entry = entry.WithField("request_body", string(reqBody))
			} else {
				entry = entry.WithField("request_body", "(empty)")
			}

			entry.Tracef("HTTP request body")

			// Wrap response writer to capture response body
			wrapped := &bodyLogResponseWriter{
				ResponseWriter: w,
				statusCode:     http.StatusOK,
			}

			start := time.Now()
			next.ServeHTTP(wrapped, r)
			duration := time.Since(start)

			// Log response body
			bodyStr := "(empty)"
			if wrapped.body.Len() > 0 {
				bodyStr = wrapped.body.String()
				// Truncate very large responses to avoid excessive log output
				if len(bodyStr) > 4096 {
					bodyStr = bodyStr[:4096] + "... (truncated)"
				}
			}

			log.WithContext(r.Context()).
				WithFields(map[string]interface{}{
					"http_method":    r.Method,
					"http_path":      r.RequestURI,
					"http_status":    wrapped.statusCode,
					"response_body":  bodyStr,
					"response_bytes": wrapped.body.Len(),
					"duration_ms":    duration.Milliseconds(),
				}).
				Tracef("HTTP response body")
		})
	}
}

// GRPCBodyLoggingInterceptor logs gRPC request and response payloads at trace level.
// Only produces output when LOG_LEVEL=trace is set.
// Use this for debugging purposes only — it can expose sensitive data and
// adds overhead for large payloads.
func GRPCBodyLoggingInterceptor(log *logger.Logger) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		// Skip health check to reduce noise
		if info.FullMethod == "/grpc.health.v1.Health/Check" {
			return handler(ctx, req)
		}

		log.WithContext(ctx).
			WithFields(map[string]interface{}{
				"grpc_method": info.FullMethod,
				"grpc_type":   "unary",
				"request":     truncatePayload(req),
			}).
			Tracef("gRPC request body")

		start := time.Now()
		resp, err := handler(ctx, req)
		duration := time.Since(start)

		fields := map[string]interface{}{
			"grpc_method": info.FullMethod,
			"grpc_type":   "unary",
			"duration_ms": duration.Milliseconds(),
		}

		if err != nil {
			fields["error"] = err.Error()
			fields["grpc_code"] = status.Code(err).String()
		} else {
			fields["response"] = truncatePayload(resp)
		}

		log.WithContext(ctx).WithFields(fields).Tracef("gRPC response body")

		return resp, err
	}
}

// truncatePayload converts a proto message to a string and truncates it if too long.
func truncatePayload(msg interface{}) string {
	if msg == nil {
		return "(nil)"
	}
	s := msg.(interface{ String() string }).String()
	if len(s) > 2048 {
		s = s[:2048] + "... (truncated)"
	}
	return s
}