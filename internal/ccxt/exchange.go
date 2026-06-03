package ccxt

import ccxt "github.com/ccxt/ccxt/go/v4"

// Exchange is a common interface for supported exchanges
type Exchange interface {
	FetchOHLCV(symbol string, opts ...ccxt.FetchOHLCVOptions) ([]ccxt.OHLCV, error)
	FetchTicker(symbol string, opts ...ccxt.FetchTickerOptions) (ccxt.Ticker, error)
	LoadMarkets(params ...any) (map[string]ccxt.MarketInterface, error)
	CreateMarketOrder(symbol string, side string, amount float64, opts ...ccxt.CreateMarketOrderOptions) (ccxt.Order, error)
	FetchBalance(params ...any) (ccxt.Balances, error)
}

// AsExchange converts CCXT client to common Exchange interface
func AsExchange(client interface{}) Exchange {
	switch v := client.(type) {
	case *ccxt.Tokocrypto:
		return v
	case *ccxt.Binance:
		return v
	case *ccxt.Coinbase:
		return v
	case *ccxt.Cex:
		return v
	case *ccxt.Indodax:
		return v
	case *ccxt.Okx:
		return v
	case *ccxt.Kucoin:
		return v
	case *ccxt.Cryptocom:
		return v
	case *ccxt.Bybit:
		return v
	default:
		return nil
	}
}
