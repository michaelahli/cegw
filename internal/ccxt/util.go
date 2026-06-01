package ccxt

import (
	"context"

	"github.com/michaelahli/cegw/gen/cegw/v1"
	ccxt "github.com/ccxt/ccxt/go/v4"
)

func NewClientForExchange(ctx context.Context, exchange cegwv1.Exchange, creds *cegwv1.Credentials) (interface{}, error) {
	cfg := ClientConfig{
		Sandbox:  false,
		ProxyURL: ProxyFromEnv(),
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
		client := NewTokocryptoClient(cfg)
		return client.Client(ctx)
	default:
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
