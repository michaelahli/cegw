package ccxt

import ccxt "github.com/ccxt/ccxt/go/v4"

// Exchange is a common interface for supported exchanges.
type Exchange interface {
	FetchOHLCV(symbol string, opts ...ccxt.FetchOHLCVOptions) ([]ccxt.OHLCV, error)
	FetchTicker(symbol string, opts ...ccxt.FetchTickerOptions) (ccxt.Ticker, error)
	FetchOrderBook(symbol string, opts ...ccxt.FetchOrderBookOptions) (ccxt.OrderBook, error)
	LoadMarkets(params ...any) (map[string]ccxt.MarketInterface, error)
	CreateMarketOrder(symbol string, side string, amount float64, opts ...ccxt.CreateMarketOrderOptions) (ccxt.Order, error)
	FetchBalance(params ...any) (ccxt.Balances, error)
}

// StreamingExchange is implemented by exchanges that expose CCXT WebSocket ticker streams.
type StreamingExchange interface {
	WatchTicker(symbol string, opts ...ccxt.WatchTickerOptions) (ccxt.Ticker, error)
	WatchOrderBook(symbol string, opts ...ccxt.WatchOrderBookOptions) (ccxt.OrderBook, error)
}

// AsExchange converts CCXT client to common Exchange interface.
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
	case *ccxt.Bitget:
		return v
	case *ccxt.Coinex:
		return v
	case *ccxt.Hashkey:
		return v
	default:
		return nil
	}
}

// AsStreamingExchange converts CCXT client to a WebSocket-capable exchange interface.
func AsStreamingExchange(client interface{}) StreamingExchange {
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
	case *ccxt.Bitget:
		return v
	case *ccxt.Coinex:
		return v
	case *ccxt.Hashkey:
		return v
	default:
		return nil
	}
}
