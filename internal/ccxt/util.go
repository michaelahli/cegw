package ccxt

import (
	"context"
	"strings"

	ccxt "github.com/ccxt/ccxt/go/v4"
	cegwv1 "github.com/michaelahli/cegw/gen/cegw/v1"
	"github.com/michaelahli/cegw/internal/logger"
)

func NewClientForExchange(ctx context.Context, exchange cegwv1.Exchange, creds *cegwv1.Credentials) (interface{}, error) {
	// Get logger from context or create a new one
	var log *logger.Logger
	if logVal := ctx.Value("logger"); logVal != nil {
		if l, ok := logVal.(*logger.Logger); ok {
			log = l
		}
	}

	// Fallback: create a noop logger if not in context
	if log == nil {
		log = logger.New("error", nil) // This will log only errors
	}

	// Use shared pool for stateless (no-credentials) clients.
	// Credential-based clients (trading) are NOT pooled because each user
	// has unique API keys.
	//
	// Short-lived REST calls use Borrow (no ref counting).
	// Long-lived WebSocket/gRPC stream consumers should call
	// AcquireClientForExchange directly.
	if creds == nil {
		pool := GetClientPool(log)
		return pool.Borrow(ctx, exchange, nil)
	}

	return newClientForExchange(ctx, exchange, creds)
}

// ReleaseClientForExchange releases a shared client back to the pool.
// This should be called when a long-lived client (e.g., WebSocket connection)
// is no longer needed. Short-lived clients (REST API calls) do not need to
// call this because the pool is designed for reuse.
func ReleaseClientForExchange(ctx context.Context, exchange cegwv1.Exchange) {
	var log *logger.Logger
	if logVal := ctx.Value("logger"); logVal != nil {
		if l, ok := logVal.(*logger.Logger); ok {
			log = l
		}
	}
	if log == nil {
		log = logger.New("error", nil)
	}

	pool := GetClientPool(log)
	pool.Release(ctx, exchange)
}

func IsWatchTickerUnsupported(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "watchticker") && strings.Contains(message, "not supported")
}

func Float64P(v *float64) float64 {
	if v == nil {
		return 0
	}
	return *v
}

func Int64P(v *int64) int64 {
	if v == nil {
		return 0
	}
	return *v
}

func StringP(v *string) string {
	if v == nil {
		return ""
	}
	return *v
}

func MapInterval(interval cegwv1.Interval) string {
	switch interval {
	case cegwv1.Interval_INTERVAL_1M:
		return "1m"
	case cegwv1.Interval_INTERVAL_5M:
		return "5m"
	case cegwv1.Interval_INTERVAL_30M:
		return "30m"
	case cegwv1.Interval_INTERVAL_45M:
		return "45m"
	case cegwv1.Interval_INTERVAL_1H:
		return "1h"
	case cegwv1.Interval_INTERVAL_2H:
		return "2h"
	case cegwv1.Interval_INTERVAL_4H:
		return "4h"
	case cegwv1.Interval_INTERVAL_1D:
		return "1d"
	case cegwv1.Interval_INTERVAL_1W:
		return "1w"
	case cegwv1.Interval_INTERVAL_1M_MONTH:
		return "1M"
	default:
		return ""
	}
}

func IntervalDuration(interval cegwv1.Interval) int64 {
	switch interval {
	case cegwv1.Interval_INTERVAL_1M:
		return 60000
	case cegwv1.Interval_INTERVAL_5M:
		return 300000
	case cegwv1.Interval_INTERVAL_30M:
		return 1800000
	case cegwv1.Interval_INTERVAL_1H:
		return 3600000
	case cegwv1.Interval_INTERVAL_1D:
		return 86400000
	case cegwv1.Interval_INTERVAL_1W:
		return 604800000
	case cegwv1.Interval_INTERVAL_1M_MONTH:
		return 2592000000
	default:
		return 0
	}
}

func OHLCVToProto(ohlcv ccxt.OHLCV) *cegwv1.OHLCV {
	return &cegwv1.OHLCV{
		High:   ohlcv.High,
		Low:    ohlcv.Low,
		Open:   ohlcv.Open,
		Close:  ohlcv.Close,
		Volume: ohlcv.Volume,
	}
}

// IsIntervalSupported checks if an interval is supported by a specific exchange
// Based on CCXT timeframe compatibility testing (June 2026)
func IsIntervalSupported(exchange cegwv1.Exchange, interval cegwv1.Interval) bool {
	// 45m is not supported by any exchange
	if interval == cegwv1.Interval_INTERVAL_45M {
		return false
	}

	// 2h is missing on Indodax and Crypto.com
	if interval == cegwv1.Interval_INTERVAL_2H {
		if exchange == cegwv1.Exchange_EXCHANGE_INDODAX || exchange == cegwv1.Exchange_EXCHANGE_CRYPTOCOM {
			return false
		}
	}

	// 4h is missing on Coinbase
	if interval == cegwv1.Interval_INTERVAL_4H {
		if exchange == cegwv1.Exchange_EXCHANGE_COINBASE {
			return false
		}
	}

	// All other intervals are universally supported
	return true
}
