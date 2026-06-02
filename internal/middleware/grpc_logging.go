package middleware

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/michaelahli/cegw/internal/logger"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// GRPCUnaryLoggingInterceptor logs gRPC unary RPC calls with structured data
func GRPCUnaryLoggingInterceptor(log *logger.Logger) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		// Extract or generate request ID and trace ID
		md, _ := metadata.FromIncomingContext(ctx)
		requestID := ""
		if vals := md.Get("x-request-id"); len(vals) > 0 {
			requestID = vals[0]
		}
		if requestID == "" {
			requestID = uuid.New().String()
		}

		traceID := ""
		if vals := md.Get("x-trace-id"); len(vals) > 0 {
			traceID = vals[0]
		}
		if traceID == "" {
			traceID = uuid.New().String()
		}

		// Add IDs to context
		ctx = logger.ContextWithRequestID(ctx, requestID)
		ctx = logger.ContextWithTraceID(ctx, traceID)

		callLog := log.WithContext(ctx).
			WithFields(map[string]interface{}{
				"grpc_method": info.FullMethod,
				"grpc_type":   "unary",
			})

		callLog.Debugf("gRPC request started")

		start := time.Now()
		resp, err := handler(ctx, req)
		duration := time.Since(start)

		// Determine gRPC status code
		statusCode := codes.Unknown
		if err != nil {
			s, _ := status.FromError(err)
			statusCode = s.Code()
		} else {
			statusCode = codes.OK
		}

		logEntry := callLog.
			WithField("grpc_code", statusCode.String()).
			WithDuration(duration)

		if err != nil {
			logEntry.WithError(err).Warnf("gRPC request failed")
		} else {
			logEntry.Debugf("gRPC request completed")
		}

		return resp, err
	}
}

// GRPCStreamLoggingInterceptor logs gRPC streaming RPC calls with structured data
func GRPCStreamLoggingInterceptor(log *logger.Logger) grpc.StreamServerInterceptor {
	return func(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		ctx := ss.Context()

		// Extract or generate request ID and trace ID
		md, _ := metadata.FromIncomingContext(ctx)
		requestID := ""
		if vals := md.Get("x-request-id"); len(vals) > 0 {
			requestID = vals[0]
		}
		if requestID == "" {
			requestID = uuid.New().String()
		}

		traceID := ""
		if vals := md.Get("x-trace-id"); len(vals) > 0 {
			traceID = vals[0]
		}
		if traceID == "" {
			traceID = uuid.New().String()
		}

		// Add IDs to context
		ctx = logger.ContextWithRequestID(ctx, requestID)
		ctx = logger.ContextWithTraceID(ctx, traceID)

		callLog := log.WithContext(ctx).
			WithFields(map[string]interface{}{
				"grpc_method": info.FullMethod,
				"grpc_type":   "stream",
				"is_client":   info.IsClientStream,
				"is_server":   info.IsServerStream,
			})

		callLog.Debugf("gRPC stream started")

		start := time.Now()
		err := handler(srv, &wrappedServerStream{
			ServerStream: ss,
			ctx:          ctx,
		})
		duration := time.Since(start)

		statusCode := codes.Unknown
		if err != nil {
			s, _ := status.FromError(err)
			statusCode = s.Code()
		} else {
			statusCode = codes.OK
		}

		logEntry := callLog.
			WithField("grpc_code", statusCode.String()).
			WithDuration(duration)

		if err != nil {
			logEntry.WithError(err).Warnf("gRPC stream failed")
		} else {
			logEntry.Debugf("gRPC stream completed")
		}

		return err
	}
}

// wrappedServerStream wraps grpc.ServerStream to propagate context
type wrappedServerStream struct {
	grpc.ServerStream
	ctx context.Context
}

func (w *wrappedServerStream) Context() context.Context {
	return w.ctx
}
