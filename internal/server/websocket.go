package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	ccxtlib "github.com/ccxt/ccxt/go/v4"
	"github.com/gorilla/websocket"
	cegwv1 "github.com/michaelahli/cegw/gen/cegw/v1"
	"github.com/michaelahli/cegw/internal/ccxt"
	"github.com/michaelahli/cegw/internal/config"
	"github.com/michaelahli/cegw/internal/logger"
)

type priceStreamMessage struct {
	Symbol    string    `json:"symbol"`
	Price     float64   `json:"price"`
	Timestamp time.Time `json:"timestamp"`
}

type orderBookStreamMessage struct {
	Symbol    string          `json:"symbol"`
	Bids      [][]float64     `json:"bids"`
	Asks      [][]float64     `json:"asks"`
	Timestamp time.Time       `json:"timestamp"`
}

type websocketErrorMessage struct {
	Error string `json:"error"`
}

// checkWebSocketOrigin validates the Origin header against allowed origins.
func checkWebSocketOrigin(cfg *config.Config) func(r *http.Request) bool {
	return func(r *http.Request) bool {
		// If no origins configured, allow all (backward compatible)
		if len(cfg.AllowedWSOrigins) == 0 {
			return true
		}

		origin := r.Header.Get("Origin")
		if origin == "" {
			// No origin header - allow (e.g., non-browser clients)
			return true
		}

		// Check if origin is in allowed list
		for _, allowed := range cfg.AllowedWSOrigins {
			// Support wildcard matching
			if allowed == "*" {
				return true
			}
			// Exact match
			if origin == allowed {
				return true
			}
			// Wildcard subdomain matching (e.g., "*.example.com")
			if strings.HasPrefix(allowed, "*.") {
				domain := allowed[2:]
				if strings.HasSuffix(origin, domain) || origin == "https://"+domain || origin == "http://"+domain {
					return true
				}
			}
		}

		return false
	}
}

func newPriceWebsocketUpgrader(cfg *config.Config) websocket.Upgrader {
	return websocket.Upgrader{
		CheckOrigin: checkWebSocketOrigin(cfg),
	}
}

func newOrderBookWebsocketUpgrader(cfg *config.Config) websocket.Upgrader {
	return websocket.Upgrader{
		CheckOrigin: checkWebSocketOrigin(cfg),
	}
}

func handlePriceWebsocket(cfg *config.Config, log *logger.Logger) http.HandlerFunc {
	upgrader := newPriceWebsocketUpgrader(cfg)
	return func(w http.ResponseWriter, r *http.Request) {
		exchange, symbol, ok := parsePriceWebsocketRequest(w, r)
		if !ok {
			return
		}

		conn, err := upgrader.Upgrade(w, r, nil)
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

func handleOrderBookWebsocket(cfg *config.Config, log *logger.Logger) http.HandlerFunc {
	upgrader := newOrderBookWebsocketUpgrader(cfg)
	return func(w http.ResponseWriter, r *http.Request) {
		exchange, symbol, limit, ok := parseOrderBookWebsocketRequest(w, r)
		if !ok {
			return
		}

		conn, err := upgrader.Upgrade(w, r, nil)
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

		streamOrderBookToWebsocket(ctx, conn, log, exchange, symbol, limit)
	}
}

func parseOrderBookWebsocketRequest(w http.ResponseWriter, r *http.Request) (cegwv1.Exchange, string, int, bool) {
	query := r.URL.Query()
	exchangeRaw := query.Get("exchange")
	symbol := query.Get("symbol")
	limitRaw := query.Get("limit")

	if exchangeRaw == "" {
		http.Error(w, "exchange is required", http.StatusBadRequest)
		return cegwv1.Exchange_EXCHANGE_UNSPECIFIED, "", 0, false
	}

	exchange, err := parseExchangeQuery(exchangeRaw)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return cegwv1.Exchange_EXCHANGE_UNSPECIFIED, "", 0, false
	}

	if symbol == "" {
		http.Error(w, "symbol is required", http.StatusBadRequest)
		return cegwv1.Exchange_EXCHANGE_UNSPECIFIED, "", 0, false
	}

	limit := 20
	if limitRaw != "" {
		parsedLimit, err := strconv.Atoi(limitRaw)
		if err != nil {
			http.Error(w, "limit must be a valid integer", http.StatusBadRequest)
			return cegwv1.Exchange_EXCHANGE_UNSPECIFIED, "", 0, false
		}
		if parsedLimit <= 0 || parsedLimit > 100 {
			http.Error(w, "limit must be between 1 and 100", http.StatusBadRequest)
			return cegwv1.Exchange_EXCHANGE_UNSPECIFIED, "", 0, false
		}
		limit = parsedLimit
	}

	return exchange, symbol, limit, true
}

func streamOrderBookToWebsocket(ctx context.Context, conn *websocket.Conn, log *logger.Logger, exchangeID cegwv1.Exchange, symbol string, limit int) {
	log = log.WithContext(ctx).
		WithField("operation", "OrderBookWebSocket").
		WithField("exchange", exchangeID.String()).
		WithField("symbol", symbol).
		WithField("limit", limit)

	client, err := ccxt.NewClientForExchange(ctx, exchangeID, nil)
	if err != nil {
		log.WithError(err).Errorf("failed to create CCXT client")
		writeWebsocketError(conn, "failed to create exchange client")
		return
	}

	exchange := ccxt.AsStreamingExchange(client)
	if exchange == nil {
		log.Warnf("exchange streaming not supported, falling back to order book polling")
		pollOrderBookToWebsocket(ctx, conn, client, log, symbol, limit)
		return
	}

	for {
		select {
		case <-ctx.Done():
			log.Debugf("websocket order book stream closed")
			return
		default:
		}

		orderBook, err := exchange.WatchOrderBook(symbol)
		if err != nil {
			if ctx.Err() != nil {
				log.Debugf("websocket order book stream closed during watch")
				return
			}
			if ccxt.IsWatchOrderBookUnsupported(err) {
				log.WithError(err).Warnf("watch order book unsupported, falling back to polling")
				pollOrderBookToWebsocket(ctx, conn, client, log, symbol, limit)
				return
			}
			log.WithError(err).Errorf("failed to watch order book")
			writeWebsocketError(conn, "failed to watch order book")
			return
		}

		if !writeOrderBookToWebsocket(conn, symbol, orderBook, limit) {
			log.Debugf("failed to write websocket order book update")
			return
		}
	}
}

func pollOrderBookToWebsocket(ctx context.Context, conn *websocket.Conn, client interface{}, log *logger.Logger, symbol string, limit int) {
	exchange := ccxt.AsExchange(client)
	if exchange == nil {
		log.Warnf("exchange polling not supported")
		writeWebsocketError(conn, "exchange not supported")
		return
	}

	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		orderBook, err := exchange.FetchOrderBook(symbol)
		if err != nil {
			if ctx.Err() != nil {
				log.Debugf("websocket order book polling stream closed during fetch")
				return
			}
			log.WithError(err).Errorf("failed to fetch order book")
			writeWebsocketError(conn, "failed to fetch order book")
			return
		}

		if !writeOrderBookToWebsocket(conn, symbol, orderBook, limit) {
			log.Debugf("failed to write websocket order book polling update")
			return
		}

		select {
		case <-ctx.Done():
			log.Debugf("websocket order book polling stream closed")
			return
		case <-ticker.C:
		}
	}
}

func writeOrderBookToWebsocket(conn *websocket.Conn, symbol string, orderBook ccxtlib.OrderBook, limit int) bool {
	bids := convertOrderBookSide(orderBook.Bids, limit)
	asks := convertOrderBookSide(orderBook.Asks, limit)

	message := orderBookStreamMessage{
		Symbol:    symbol,
		Bids:      bids,
		Asks:      asks,
		Timestamp: time.Now().UTC(),
	}
	return conn.WriteJSON(message) == nil
}

func convertOrderBookSide(side [][]float64, limit int) [][]float64 {
	if len(side) == 0 {
		return [][]float64{}
	}

	if limit > 0 && len(side) > limit {
		side = side[:limit]
	}

	result := make([][]float64, len(side))
	for i, entry := range side {
		if len(entry) >= 2 {
			result[i] = []float64{entry[0], entry[1]}
		}
	}
	return result
}
