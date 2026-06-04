package ccxt

import (
	"testing"

	cegwv1 "github.com/michaelahli/cegw/gen/cegw/v1"
)

func TestMapInterval(t *testing.T) {
	tests := []struct {
		name     string
		interval cegwv1.Interval
		want     string
	}{
		{"1m", cegwv1.Interval_INTERVAL_1M, "1m"},
		{"5m", cegwv1.Interval_INTERVAL_5M, "5m"},
		{"30m", cegwv1.Interval_INTERVAL_30M, "30m"},
		{"1h", cegwv1.Interval_INTERVAL_1H, "1h"},
		{"1d", cegwv1.Interval_INTERVAL_1D, "1d"},
		{"1w", cegwv1.Interval_INTERVAL_1W, "1w"},
		{"1M", cegwv1.Interval_INTERVAL_1M_MONTH, "1M"},
		{"unspecified", cegwv1.Interval_INTERVAL_UNSPECIFIED, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := MapInterval(tt.interval); got != tt.want {
				t.Errorf("MapInterval() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIntervalDuration(t *testing.T) {
	tests := []struct {
		name     string
		interval cegwv1.Interval
		want     int64
	}{
		{"1m", cegwv1.Interval_INTERVAL_1M, 60000},
		{"5m", cegwv1.Interval_INTERVAL_5M, 300000},
		{"30m", cegwv1.Interval_INTERVAL_30M, 1800000},
		{"1h", cegwv1.Interval_INTERVAL_1H, 3600000},
		{"1d", cegwv1.Interval_INTERVAL_1D, 86400000},
		{"1w", cegwv1.Interval_INTERVAL_1W, 604800000},
		{"1M", cegwv1.Interval_INTERVAL_1M_MONTH, 2592000000},
		{"unspecified", cegwv1.Interval_INTERVAL_UNSPECIFIED, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IntervalDuration(tt.interval); got != tt.want {
				t.Errorf("IntervalDuration() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFloat64P(t *testing.T) {
	val := 123.45
	if got := Float64P(&val); got != val {
		t.Errorf("Float64P() = %v, want %v", got, val)
	}

	if got := Float64P(nil); got != 0 {
		t.Errorf("Float64P(nil) = %v, want 0", got)
	}
}

func TestInt64P(t *testing.T) {
	val := int64(12345)
	if got := Int64P(&val); got != val {
		t.Errorf("Int64P() = %v, want %v", got, val)
	}

	if got := Int64P(nil); got != 0 {
		t.Errorf("Int64P(nil) = %v, want 0", got)
	}
}

func TestStringP(t *testing.T) {
	val := "test"
	if got := StringP(&val); got != val {
		t.Errorf("StringP() = %v, want %v", got, val)
	}

	if got := StringP(nil); got != "" {
		t.Errorf("StringP(nil) = %v, want empty string", got)
	}
}

func TestIsIntervalSupported(t *testing.T) {
	tests := []struct {
		name     string
		exchange cegwv1.Exchange
		interval cegwv1.Interval
		want     bool
	}{
		// 45m is not supported by any exchange
		{"45m on Binance", cegwv1.Exchange_EXCHANGE_BINANCE, cegwv1.Interval_INTERVAL_45M, false},
		{"45m on Tokocrypto", cegwv1.Exchange_EXCHANGE_TOKOCRYPTO, cegwv1.Interval_INTERVAL_45M, false},
		{"45m on OKX", cegwv1.Exchange_EXCHANGE_OKX, cegwv1.Interval_INTERVAL_45M, false},
		
		// 2h is missing on Indodax and Crypto.com
		{"2h on Binance", cegwv1.Exchange_EXCHANGE_BINANCE, cegwv1.Interval_INTERVAL_2H, true},
		{"2h on Indodax", cegwv1.Exchange_EXCHANGE_INDODAX, cegwv1.Interval_INTERVAL_2H, false},
		{"2h on Crypto.com", cegwv1.Exchange_EXCHANGE_CRYPTOCOM, cegwv1.Interval_INTERVAL_2H, false},
		{"2h on OKX", cegwv1.Exchange_EXCHANGE_OKX, cegwv1.Interval_INTERVAL_2H, true},
		
		// 4h is missing on Coinbase
		{"4h on Binance", cegwv1.Exchange_EXCHANGE_BINANCE, cegwv1.Interval_INTERVAL_4H, true},
		{"4h on Coinbase", cegwv1.Exchange_EXCHANGE_COINBASE, cegwv1.Interval_INTERVAL_4H, false},
		{"4h on Indodax", cegwv1.Exchange_EXCHANGE_INDODAX, cegwv1.Interval_INTERVAL_4H, true},
		
		// Common intervals should be supported everywhere
		{"1m on Binance", cegwv1.Exchange_EXCHANGE_BINANCE, cegwv1.Interval_INTERVAL_1M, true},
		{"5m on Coinbase", cegwv1.Exchange_EXCHANGE_COINBASE, cegwv1.Interval_INTERVAL_5M, true},
		{"1h on Indodax", cegwv1.Exchange_EXCHANGE_INDODAX, cegwv1.Interval_INTERVAL_1H, true},
		{"1d on OKX", cegwv1.Exchange_EXCHANGE_OKX, cegwv1.Interval_INTERVAL_1D, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsIntervalSupported(tt.exchange, tt.interval); got != tt.want {
				t.Errorf("IsIntervalSupported(%s, %s) = %v, want %v", 
					tt.exchange.String(), tt.interval.String(), got, tt.want)
			}
		})
	}
}
