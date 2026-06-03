package metrics

import (
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
)

// Tracer returns the global tracer for CEGW
func Tracer() trace.Tracer {
	return otel.Tracer("cegw")
}
