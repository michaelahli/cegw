package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	ccxtlib "github.com/ccxt/ccxt/go/v4"
	"github.com/gorilla/websocket"
	cegwv1 "github.com/michaelahli/cegw/gen/cegw/v1"
	"github.com/michaelahli/cegw/internal/ccxt"
	"github.com/michaelahli/cegw/internal/logger"
)

type priceStreamMessage struct {
	Symbol    string    `json:"symbol"`
	Price     float64   `json:"price"`
	Timestamp time.Time `json:"timestamp"`
}

type websocketErrorMessage struct {
	Error string `json:"error"`
}

var priceWebsocketUpgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

func handlePriceWebsocket(log *logger.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		exchange, symbol, ok := parsePriceWebsocketRequest(w, r)
		if !ok {
			return
		}

		conn, err := priceWebsocketUpgrader.Upgrade(w, r, nil)
		if err != nil {
			log.WithError(err).Warnf("failed to upgrade websocket connection")
			return
		}
		defer func() { _ = conn.Close() }()

		ctx, cancel := context.WithCancel(r.Context())
		defer cancel()

		go func() {
			for {
				if _, _, err := conn.ReadMessage(); err != nil {
					cancel()
					return
				}
			}
		}()

		streamPriceToWebsocket(ctx, conn, log, exchange, symbol)
	}
}

func parsePriceWebsocketRequest(w http.ResponseWriter, r *http.Request) (cegwv1.Exchange, string, bool) {
	query := r.URL.Query()
	exchangeRaw := query.Get("exchange")
	symbol := query.Get("symbol")

	if exchangeRaw == "" {
		http.Error(w, "exchange is required", http.StatusBadRequest)
		return cegwv1.Exchange_EXCHANGE_UNSPECIFIED, "", false
	}

	exchange, err := parseExchangeQuery(exchangeRaw)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return cegwv1.Exchange_EXCHANGE_UNSPECIFIED, "", false
	}

	if symbol == "" {
		http.Error(w, "symbol is required", http.StatusBadRequest)
		return cegwv1.Exchange_EXCHANGE_UNSPECIFIED, "", false
	}

	return exchange, symbol, true
}

func parseExchangeQuery(exchangeRaw string) (cegwv1.Exchange, error) {
	switch exchangeRaw {
	case "1":
		return cegwv1.Exchange_EXCHANGE_TOKOCRYPTO, nil
	case "2":
		return cegwv1.Exchange_EXCHANGE_BINANCE, nil
	case "3":
		return cegwv1.Exchange_EXCHANGE_COINBASE, nil
	case "4":
		return cegwv1.Exchange_EXCHANGE_CEXIO, nil
	case "5":
		return cegwv1.Exchange_EXCHANGE_INDODAX, nil
	case "6":
		return cegwv1.Exchange_EXCHANGE_OKX, nil
	case "7":
		return cegwv1.Exchange_EXCHANGE_KUCOIN, nil
	case "8":
		return cegwv1.Exchange_EXCHANGE_CRYPTOCOM, nil
	case "9":
		return cegwv1.Exchange_EXCHANGE_BYBIT, nil
	case "10":
		return cegwv1.Exchange_EXCHANGE_BITGET, nil
	case "11":
		return cegwv1.Exchange_EXCHANGE_COINEX, nil
	case "12":
		return cegwv1.Exchange_EXCHANGE_HASHKEY, nil
	default:
		return cegwv1.Exchange_EXCHANGE_UNSPECIFIED, fmt.Errorf("exchange is not supported")
	}
}

func streamPriceToWebsocket(ctx context.Context, conn *websocket.Conn, log *logger.Logger, exchangeID cegwv1.Exchange, symbol string) {
	log = log.WithContext(ctx).
		WithField("operation", "PriceWebSocket").
		WithField("exchange", exchangeID.String()).
		WithField("symbol", symbol)

	client, err := ccxt.NewClientForExchange(ctx, exchangeID, nil)
	if err != nil {
		log.WithError(err).Errorf("failed to create CCXT client")
		writeWebsocketError(conn, "failed to create exchange client")
		return
	}

	exchange := ccxt.AsStreamingExchange(client)
	if exchange == nil {
		log.Warnf("exchange streaming not supported, falling back to ticker polling")
		pollPriceToWebsocket(ctx, conn, client, log, symbol)
		return
	}

	for {
		select {
		case <-ctx.Done():
			log.Debugf("websocket price stream closed")
			return
		default:
		}

		ticker, err := exchange.WatchTicker(symbol)
		if err != nil {
			if ctx.Err() != nil {
				log.Debugf("websocket price stream closed during ticker watch")
				return
			}
			if ccxt.IsWatchTickerUnsupported(err) {
				log.WithError(err).Warnf("watch ticker unsupported, falling back to ticker polling")
				pollPriceToWebsocket(ctx, conn, client, log, symbol)
				return
			}
			log.WithError(err).Errorf("failed to watch ticker")
			writeWebsocketError(conn, "failed to watch ticker")
			return
		}

		if !writeTickerToWebsocket(conn, symbol, ticker) {
			log.Debugf("failed to write websocket price update")
			return
		}
	}
}

func pollPriceToWebsocket(ctx context.Context, conn *websocket.Conn, client interface{}, log *logger.Logger, symbol string) {
	exchange := ccxt.AsExchange(client)
	if exchange == nil {
		log.Warnf("exchange polling not supported")
		writeWebsocketError(conn, "exchange not supported")
		return
	}

	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()

	for {
		latest, err := exchange.FetchTicker(symbol)
		if err != nil {
			if ctx.Err() != nil {
				log.Debugf("websocket polling stream closed during ticker fetch")
				return
			}
			log.WithError(err).Errorf("failed to fetch ticker")
			writeWebsocketError(conn, "failed to fetch ticker")
			return
		}

		if !writeTickerToWebsocket(conn, symbol, latest) {
			log.Debugf("failed to write websocket polling update")
			return
		}

		select {
		case <-ctx.Done():
			log.Debugf("websocket polling stream closed")
			return
		case <-ticker.C:
		}
	}
}

func writeTickerToWebsocket(conn *websocket.Conn, symbol string, ticker ccxtlib.Ticker) bool {
	price := ccxt.Float64P(ticker.Close)
	if price == 0 {
		price = ccxt.Float64P(ticker.Last)
	}

	message := priceStreamMessage{
		Symbol:    symbol,
		Price:     price,
		Timestamp: time.Now().UTC(),
	}
	return conn.WriteJSON(message) == nil
}

func writeWebsocketError(conn *websocket.Conn, message string) {
	payload, err := json.Marshal(websocketErrorMessage{Error: message})
	if err != nil {
		return
	}
	_ = conn.WriteMessage(websocket.TextMessage, payload)
}
