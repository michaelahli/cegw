package ccxt

import (
	"context"
	"time"

	"github.com/michaelahli/cegw/internal/metrics"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// RecordCCXTCall records metrics for a CCXT API call
func RecordCCXTCall(ctx context.Context, m *metrics.Metrics, exchange, method string, start time.Time, err error) {
	if m == nil {
		return
	}

	duration := time.Since(start).Seconds()
	status := "success"
	if err != nil {
		status = "error"
	}

	attrs := []attribute.KeyValue{
		attribute.String("exchange", exchange),
		attribute.String("method", method),
		attribute.String("status", status),
	}

	m.CCXTCallCount.Add(ctx, 1, metric.WithAttributes(attrs...))
	m.CCXTCallDuration.Record(ctx, duration, metric.WithAttributes(attrs...))
}
