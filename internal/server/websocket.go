package server

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"time"

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

	exchangeID, err := strconv.Atoi(exchangeRaw)
	if err != nil {
		http.Error(w, "exchange must be numeric", http.StatusBadRequest)
		return cegwv1.Exchange_EXCHANGE_UNSPECIFIED, "", false
	}

	exchange := cegwv1.Exchange(exchangeID)
	if exchange == cegwv1.Exchange_EXCHANGE_UNSPECIFIED {
		http.Error(w, "exchange is required", http.StatusBadRequest)
		return cegwv1.Exchange_EXCHANGE_UNSPECIFIED, "", false
	}

	if symbol == "" {
		http.Error(w, "symbol is required", http.StatusBadRequest)
		return cegwv1.Exchange_EXCHANGE_UNSPECIFIED, "", false
	}

	return exchange, symbol, true
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
		log.Warnf("exchange streaming not supported")
		writeWebsocketError(conn, "exchange streaming not supported")
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
			log.WithError(err).Errorf("failed to watch ticker")
			writeWebsocketError(conn, "failed to watch ticker")
			return
		}

		price := ccxt.Float64P(ticker.Close)
		if price == 0 {
			price = ccxt.Float64P(ticker.Last)
		}

		message := priceStreamMessage{
			Symbol:    symbol,
			Price:     price,
			Timestamp: time.Now().UTC(),
		}
		if err := conn.WriteJSON(message); err != nil {
			log.WithError(err).Debugf("failed to write websocket price update")
			return
		}
	}
}

func writeWebsocketError(conn *websocket.Conn, message string) {
	payload, err := json.Marshal(websocketErrorMessage{Error: message})
	if err != nil {
		return
	}
	_ = conn.WriteMessage(websocket.TextMessage, payload)
}
