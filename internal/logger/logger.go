package logger

import (
	"context"
	"io"
	"time"

	"github.com/sirupsen/logrus"
)

type Logger struct {
	*logrus.Entry
}

type ContextKey string

const (
	RequestIDKey  ContextKey = "request_id"
	TraceIDKey    ContextKey = "trace_id"
	SpanIDKey     ContextKey = "span_id"
	UserIDKey     ContextKey = "user_id"
	ExchangeKey   ContextKey = "exchange"
	SymbolKey     ContextKey = "symbol"
	OperationKey  ContextKey = "operation"
)

// New creates a new logger with JSON formatting for production
func New(level string, output io.Writer) *Logger {
	log := logrus.New()
	log.SetOutput(output)
	log.SetFormatter(&logrus.JSONFormatter{
		TimestampFormat: time.RFC3339Nano,
		FieldMap: logrus.FieldMap{
			logrus.FieldKeyTime:  "timestamp",
			logrus.FieldKeyLevel: "level",
			logrus.FieldKeyMsg:   "message",
			logrus.FieldKeyFunc:  "func",
		},
	})

	lvl, err := logrus.ParseLevel(level)
	if err != nil {
		lvl = logrus.InfoLevel
	}
	log.SetLevel(lvl)

	return &Logger{
		Entry: logrus.NewEntry(log),
	}
}

// WithContext extracts logging fields from context and returns a new logger
func (l *Logger) WithContext(ctx context.Context) *Logger {
	entry := l.Entry

	// Extract standard context values
	if requestID, ok := ctx.Value(RequestIDKey).(string); ok && requestID != "" {
		entry = entry.WithField("request_id", requestID)
	}
	if traceID, ok := ctx.Value(TraceIDKey).(string); ok && traceID != "" {
		entry = entry.WithField("trace_id", traceID)
	}
	if spanID, ok := ctx.Value(SpanIDKey).(string); ok && spanID != "" {
		entry = entry.WithField("span_id", spanID)
	}
	if userID, ok := ctx.Value(UserIDKey).(string); ok && userID != "" {
		entry = entry.WithField("user_id", userID)
	}
	if exchange, ok := ctx.Value(ExchangeKey).(string); ok && exchange != "" {
		entry = entry.WithField("exchange", exchange)
	}
	if symbol, ok := ctx.Value(SymbolKey).(string); ok && symbol != "" {
		entry = entry.WithField("symbol", symbol)
	}
	if operation, ok := ctx.Value(OperationKey).(string); ok && operation != "" {
		entry = entry.WithField("operation", operation)
	}

	return &Logger{Entry: entry}
}

// WithFields adds multiple fields and returns a new logger
func (l *Logger) WithFields(fields map[string]interface{}) *Logger {
	return &Logger{Entry: l.Entry.WithFields(logrus.Fields(fields))}
}

// WithField adds a single field and returns a new logger
func (l *Logger) WithField(key string, value interface{}) *Logger {
	return &Logger{Entry: l.Entry.WithField(key, value)}
}

// WithError adds an error field and returns a new logger
func (l *Logger) WithError(err error) *Logger {
	return &Logger{Entry: l.Entry.WithError(err)}
}

// WithDuration adds duration field for operation timing
func (l *Logger) WithDuration(d time.Duration) *Logger {
	return &Logger{Entry: l.Entry.WithField("duration_ms", d.Milliseconds())}
}

// ContextWithRequestID returns a new context with request ID
func ContextWithRequestID(ctx context.Context, requestID string) context.Context {
	return context.WithValue(ctx, RequestIDKey, requestID)
}

// ContextWithTraceID returns a new context with trace ID
func ContextWithTraceID(ctx context.Context, traceID string) context.Context {
	return context.WithValue(ctx, TraceIDKey, traceID)
}

// ContextWithSpanID returns a new context with span ID
func ContextWithSpanID(ctx context.Context, spanID string) context.Context {
	return context.WithValue(ctx, SpanIDKey, spanID)
}

// ContextWithOperation returns a new context with operation name
func ContextWithOperation(ctx context.Context, operation string) context.Context {
	return context.WithValue(ctx, OperationKey, operation)
}

// ContextWithExchange returns a new context with exchange name
func ContextWithExchange(ctx context.Context, exchange string) context.Context {
	return context.WithValue(ctx, ExchangeKey, exchange)
}

// ContextWithSymbol returns a new context with symbol
func ContextWithSymbol(ctx context.Context, symbol string) context.Context {
	return context.WithValue(ctx, SymbolKey, symbol)
}
