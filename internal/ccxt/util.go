package ccxt

import (
	"context"

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

	cfg := ClientConfig{
		Sandbox:  false,
		ProxyURL: ProxyFromEnv(log),
		Options: map[string]any{
			"recvWindow": 5000,
		},
	}

	if creds != nil {
		cfg.APIKey = creds.ApiKey
		cfg.APISecret = creds.ApiSecret
		cfg.Sandbox = creds.Sandbox
		if creds.Sandbox {
			cfg.Options["sandbox"] = true
		}
	}

	switch exchange {
	case cegwv1.Exchange_EXCHANGE_TOKOCRYPTO:
		client := NewTokocryptoClient(cfg, log)
		return client.Client(ctx)
	case cegwv1.Exchange_EXCHANGE_BINANCE:
		client := NewBinanceClient(cfg, log)
		return client.Client(ctx)
	case cegwv1.Exchange_EXCHANGE_COINBASE:
		client := NewCoinbaseClient(cfg, log)
		return client.Client(ctx)
	case cegwv1.Exchange_EXCHANGE_CEXIO:
		client := NewCEXIOClient(cfg, log)
		return client.Client(ctx)
	case cegwv1.Exchange_EXCHANGE_INDODAX:
		client := NewIndodaxClient(cfg, log)
		return client.Client(ctx)
	case cegwv1.Exchange_EXCHANGE_OKX:
		client := NewOKXClient(cfg, log)
		return client.Client(ctx)
	case cegwv1.Exchange_EXCHANGE_KUCOIN:
		client := NewKuCoinClient(cfg, log)
		return client.Client(ctx)
	case cegwv1.Exchange_EXCHANGE_CRYPTOCOM:
		client := NewCryptocomClient(cfg, log)
		return client.Client(ctx)
	default:
		log.WithContext(ctx).
			WithField("exchange", exchange.String()).
			Warnf("unsupported exchange")
		return nil, nil
	}
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
	case cegwv1.Interval_INTERVAL_1H:
		return "1h"
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
