package middleware

import (
	"context"
	"time"

	"github.com/michaelahli/cegw/internal/metrics"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	"google.golang.org/grpc"
	"google.golang.org/grpc/status"
)

// GRPCMetricsInterceptor records metrics for gRPC calls
func GRPCMetricsInterceptor(m *metrics.Metrics) grpc.UnaryServerInterceptor {
	if m == nil {
		return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
			return handler(ctx, req)
		}
	}

	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		start := time.Now()

		tracer := metrics.Tracer()
		ctx, span := tracer.Start(ctx, info.FullMethod)
		defer span.End()

		resp, err := handler(ctx, req)

		duration := time.Since(start).Seconds()
		statusCode := status.Code(err)

		attrs := []attribute.KeyValue{
			attribute.String("method", info.FullMethod),
			attribute.String("status", statusCode.String()),
		}

		m.RequestCount.Add(ctx, 1, metric.WithAttributes(attrs...))
		m.RequestDuration.Record(ctx, duration, metric.WithAttributes(attrs...))

		if err != nil {
			m.ErrorCount.Add(ctx, 1, metric.WithAttributes(attrs...))
			span.SetStatus(codes.Error, err.Error())
		}

		return resp, err
	}
}
