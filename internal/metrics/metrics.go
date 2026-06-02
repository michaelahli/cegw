package metrics

import (
	"context"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/metric"
)

type Metrics struct {
	RequestCount    metric.Int64Counter
	RequestDuration metric.Float64Histogram
	ErrorCount      metric.Int64Counter
}

func New(ctx context.Context) (*Metrics, error) {
	meter := otel.Meter("cegw")

	requestCount, err := meter.Int64Counter(
		"cegw.requests.total",
		metric.WithDescription("Total number of requests"),
	)
	if err != nil {
		return nil, err
	}

	requestDuration, err := meter.Float64Histogram(
		"cegw.request.duration",
		metric.WithDescription("Request duration in seconds"),
		metric.WithUnit("s"),
	)
	if err != nil {
		return nil, err
	}

	errorCount, err := meter.Int64Counter(
		"cegw.errors.total",
		metric.WithDescription("Total number of errors"),
	)
	if err != nil {
		return nil, err
	}

	return &Metrics{
		RequestCount:    requestCount,
		RequestDuration: requestDuration,
		ErrorCount:      errorCount,
	}, nil
}
