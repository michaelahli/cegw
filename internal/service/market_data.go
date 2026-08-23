package service

import (
	"context"
	"sort"
	"strings"
	"sync"
	"time"

	ccxtlib "github.com/ccxt/ccxt/go/v4"
	"golang.org/x/sync/singleflight"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	cegwv1 "github.com/michaelahli/cegw/gen/cegw/v1"
	"github.com/michaelahli/cegw/internal/ccxt"
	"github.com/michaelahli/cegw/internal/config"
	"github.com/michaelahli/cegw/internal/logger"
	"github.com/michaelahli/cegw/internal/metrics"
)

// cacheTTL bounds how long a per-exchange market cache is considered fresh.
// After this elapses, the next SearchTicker/ListMarkets call will refresh
// the cache in the background.
const cacheTTL = 15 * time.Minute

// defaultSearchLimit is applied when a client does not pass a limit.
const defaultSearchLimit = 50

// maxSearchLimit caps the requested limit to avoid pathological responses.
const maxSearchLimit = 500

// cachedTicker is the internal representation of a market with pre-indexed
// lowercase fields for fast case-insensitive search and base/quote matching.
type cachedTicker struct {
	proto         *cegwv1.Ticker
	symbolLower   string
	baseLower     string
	quoteLower    string
	active        bool
}

// marketCache holds the market list for a single exchange along with metadata.
type marketCache struct {
	tickers    []cachedTicker
	loadedAt   time.Time
	loadInProg bool
}

// MarketDataService implements the gRPC MarketDataService.
type MarketDataService struct {
	cegwv1.UnimplementedMarketDataServiceServer
	cfg     *config.Config
	log     *logger.Logger
	metrics *metrics.Metrics

	caches    map[cegwv1.Exchange]*marketCache
	cacheMux  sync.RWMutex
	loadGroup singleflight.Group
}

// NewMarketDataService creates a service and kicks off background cache
// priming for every supported exchange. Each exchange's cache is loaded
// exactly once via singleflight, so duplicate concurrent loads collapse.
func NewMarketDataService(cfg *config.Config, log *logger.Logger, m *metrics.Metrics) *MarketDataService {
	svc := &MarketDataService{
		cfg:     cfg,
		log:     log,
		metrics: m,
		caches:  make(map[cegwv1.Exchange]*marketCache),
	}

	for _, exch := range supportedExchanges() {
		go svc.cacheMarkets(exch)
	}
	return svc
}

// supportedExchanges returns the list of exchanges whose market cache should
// be pre-warmed at startup.
func supportedExchanges() []cegwv1.Exchange {
	return []cegwv1.Exchange{
		cegwv1.Exchange_EXCHANGE_TOKOCRYPTO,
		cegwv1.Exchange_EXCHANGE_BINANCE,
		cegwv1.Exchange_EXCHANGE_COINBASE,
		cegwv1.Exchange_EXCHANGE_CEXIO,
		cegwv1.Exchange_EXCHANGE_INDODAX,
		cegwv1.Exchange_EXCHANGE_OKX,
		cegwv1.Exchange_EXCHANGE_KUCOIN,
		cegwv1.Exchange_EXCHANGE_CRYPTOCOM,
		cegwv1.Exchange_EXCHANGE_BYBIT,
		cegwv1.Exchange_EXCHANGE_BITGET,
		cegwv1.Exchange_EXCHANGE_COINEX,
		cegwv1.Exchange_EXCHANGE_HASHKEY,
	}
}

// cacheMarkets loads (or refreshes) the market cache for a single exchange.
// Concurrent calls for the same exchange are deduplicated via singleflight.
func (s *MarketDataService) cacheMarkets(exchangeID cegwv1.Exchange) {
	ctx := context.Background()
	_, _, _ = s.loadGroup.Do(exchangeID.String(), func() (any, error) {
		s.loadMarketsInto(ctx, exchangeID)
		return nil, nil
	})
}

// loadMarketsInto performs the actual LoadMarkets + cache write.
func (s *MarketDataService) loadMarketsInto(ctx context.Context, exchangeID cegwv1.Exchange) {
	log := s.log.WithContext(ctx).WithField("operation", "cacheMarkets").
		WithField("exchange", exchangeID.String())

	s.markLoading(exchangeID, true)

	client, err := ccxt.NewClientForExchange(ctx, exchangeID, nil)
	if err != nil || client == nil {
		log.WithError(err).Errorf("failed to initialize CCXT client for market cache")
		s.markLoading(exchangeID, false)
		return
	}

	exchange := ccxt.AsExchange(client)
	if exchange == nil {
		log.Errorf("failed to cast client to exchange interface")
		s.markLoading(exchangeID, false)
		return
	}

	markets, err := exchange.LoadMarkets()
	if err != nil {
		log.WithError(err).Errorf("failed to load markets")
		s.markLoading(exchangeID, false)
		return
	}

cached := make([]cachedTicker, 0, len(markets))
	for _, m := range markets {
		symbol := ccxt.StringP(m.Symbol)
		base := ccxt.StringP(m.BaseCurrency)
		quote := ccxt.StringP(m.QuoteCurrency)
		cached = append(cached, cachedTicker{
			proto: &cegwv1.Ticker{
				Symbol: symbol,
				Base:   base,
				Quote:  quote,
			},
			symbolLower: strings.ToLower(symbol),
			baseLower:   strings.ToLower(base),
			quoteLower:  strings.ToLower(quote),
			active:      m.Active != nil && *m.Active,
		})
	}

	s.cacheMux.Lock()
	s.caches[exchangeID] = &marketCache{
		tickers:  cached,
		loadedAt: time.Now(),
	}
	s.cacheMux.Unlock()

	log.WithField("ticker_count", len(cached)).Infof("market cache loaded")
}

// markLoading toggles the in-progress flag on a cache entry without
// overwriting any existing ticker list.
func (s *MarketDataService) markLoading(exchangeID cegwv1.Exchange, loading bool) {
	s.cacheMux.Lock()
	defer s.cacheMux.Unlock()

	c, ok := s.caches[exchangeID]
	if !ok {
		if loading {
			s.caches[exchangeID] = &marketCache{loadInProg: true}
		}
		return
	}
	c.loadInProg = loading
}

// snapshotCache returns a copy of the cached tickers and the cache load time.
func (s *MarketDataService) snapshotCache(exchangeID cegwv1.Exchange) ([]cachedTicker, time.Time, bool) {
	s.cacheMux.RLock()
	defer s.cacheMux.RUnlock()

	c, ok := s.caches[exchangeID]
	if !ok || c == nil {
		return nil, time.Time{}, false
	}
	out := make([]cachedTicker, len(c.tickers))
	copy(out, c.tickers)
	return out, c.loadedAt, c.loadInProg
}

// ensureCache returns tickers for the given exchange, triggering a load
// if the cache is empty or stale. It blocks until at least one load attempt
// completes (either from a fresh kickoff or by joining an in-progress one).
func (s *MarketDataService) ensureCache(ctx context.Context, exchangeID cegwv1.Exchange) ([]cachedTicker, error) {
	tickers, loadedAt, _ := s.snapshotCache(exchangeID)
	fresh := !loadedAt.IsZero() && time.Since(loadedAt) < cacheTTL

	if fresh {
		return tickers, nil
	}

	// Use singleflight to coalesce concurrent loads for the same exchange.
	_, _, _ = s.loadGroup.Do(exchangeID.String(), func() (any, error) {
		// Re-check inside the singleflight callback to avoid races where
		// another goroutine refreshed while we waited to enter.
		s.cacheMux.RLock()
		c := s.caches[exchangeID]
		s.cacheMux.RUnlock()
		if c != nil && !c.loadedAt.IsZero() && time.Since(c.loadedAt) < cacheTTL && !c.loadInProg {
			return nil, nil
		}
		s.loadMarketsInto(ctx, exchangeID)
		return nil, nil
	})

	var inProg bool
	tickers, _, inProg = s.snapshotCache(exchangeID)
	if inProg || len(tickers) == 0 {
		return nil, status.Error(codes.Unavailable, "market cache is still loading, please retry")
	}
	return tickers, nil
}

// GetQuotes fetches historical OHLCV data with batching support.
func (s *MarketDataService) GetQuotes(ctx context.Context, req *cegwv1.GetQuotesRequest) (*cegwv1.GetQuotesResponse, error) {
	log := s.log.WithContext(ctx).
		WithField("operation", "GetQuotes").
		WithField("symbol", req.Symbol).
		WithField("exchange", req.Exchange.String())

	if req.Exchange == cegwv1.Exchange_EXCHANGE_UNSPECIFIED {
		log.Infof("invalid request: exchange unspecified")
		return nil, status.Error(codes.InvalidArgument, "exchange is required")
	}

	if req.Symbol == "" {
		log.Infof("invalid request: symbol empty")
		return nil, status.Error(codes.InvalidArgument, "symbol is required")
	}

	if !ccxt.IsIntervalSupported(req.Exchange, req.Interval) {
		log.WithField("interval", req.Interval.String()).Infof("interval not supported by exchange")
		return nil, status.Errorf(codes.InvalidArgument, "interval %s is not supported by %s", req.Interval.String(), req.Exchange.String())
	}

	interval := ccxt.MapInterval(req.Interval)
	if interval == "" {
		log.WithField("interval", req.Interval).Infof("invalid interval")
		return nil, status.Error(codes.InvalidArgument, "invalid interval")
	}

	log = log.WithField("interval", interval)
	log.Debugf("fetching quotes")

	client, err := ccxt.NewClientForExchange(ctx, req.Exchange, nil)
	if err != nil {
		log.WithError(err).Errorf("failed to create CCXT client")
		return nil, err
	}

	exchange := ccxt.AsExchange(client)
	if exchange == nil {
		log.Errorf("exchange not supported")
		return nil, status.Error(codes.Unimplemented, "exchange not supported")
	}

	start := time.Time{}
	if req.Start != nil {
		start = req.Start.AsTime()
	}
	end := time.Time{}
	if req.End != nil && !req.End.AsTime().IsZero() {
		end = req.End.AsTime()
	}

	userLimit := int64(0)
	if req.Limit > 0 {
		userLimit = int64(req.Limit)
		log = log.WithField("user_limit", userLimit)
	}

	batchLimit := int64(1000)

	var mergedKlines []ccxtlib.OHLCV
	batchCount := 0

	if start.IsZero() {
		fetchLimit := batchLimit
		if userLimit > 0 && userLimit < fetchLimit {
			fetchLimit = userLimit
		}

		opts := []ccxtlib.FetchOHLCVOptions{
			ccxtlib.WithFetchOHLCVTimeframe(interval),
			ccxtlib.WithFetchOHLCVLimit(fetchLimit),
		}

		klines, err := exchange.FetchOHLCV(req.Symbol, opts...)
		if err != nil {
			log.WithError(err).Errorf("failed to fetch latest OHLCV data")
			return nil, ccxt.MapError(err)
		}
		batchCount++
		log.WithField("batch_size", len(klines)).Debugf("fetched latest OHLCV batch")

		mergedKlines = klines
	} else {
		shiftedStart := start

		for {
			limit := batchLimit
			if !end.IsZero() {
				candleDur := ccxt.IntervalDuration(req.Interval)
				if candleDur > 0 {
					remaining := end.Sub(shiftedStart)
					if remaining > 0 {
						if calc := remaining.Milliseconds() / candleDur; calc < limit {
							limit = calc
						}
						if limit < 1 {
							limit = 1
						}
					}
				}
			}

			if userLimit > 0 && int64(len(mergedKlines)) >= userLimit {
				log.WithField("merged_count", len(mergedKlines)).Debugf("user limit reached, stopping fetch")
				break
			}

			opts := []ccxtlib.FetchOHLCVOptions{
				ccxtlib.WithFetchOHLCVTimeframe(interval),
				ccxtlib.WithFetchOHLCVSince(shiftedStart.UnixMilli()),
				ccxtlib.WithFetchOHLCVLimit(limit),
			}

			klines, err := exchange.FetchOHLCV(req.Symbol, opts...)
			if err != nil {
				log.WithError(err).WithField("batch_number", batchCount+1).Errorf("failed to fetch OHLCV data")
				return nil, ccxt.MapError(err)
			}
			batchCount++
			log.WithField("batch_number", batchCount).WithField("batch_size", len(klines)).Debugf("fetched OHLCV batch")

			mergedKlines = append(mergedKlines, klines...)

			if userLimit > 0 && int64(len(mergedKlines)) >= userLimit {
				log.WithField("merged_count", len(mergedKlines)).Debugf("user limit reached after batch append")
				break
			}

			if len(klines) < 1000 {
				break
			}

			last := klines[len(klines)-1]
			shiftedStart = time.UnixMilli(last.Timestamp).Add(time.Millisecond)
			if !end.IsZero() && shiftedStart.After(end) {
				break
			}
		}

		if userLimit > 0 && int64(len(mergedKlines)) > userLimit {
			mergedKlines = mergedKlines[:userLimit]
			log.WithField("truncated_to", userLimit).Debugf("truncated quotes to user limit")
		}
	}

	quotes := make([]*cegwv1.Quote, 0, len(mergedKlines))
	for _, kline := range mergedKlines {
		quotes = append(quotes, &cegwv1.Quote{
			Timestamp: timestamppb.New(time.UnixMilli(kline.Timestamp)),
			Ohlcv:     ccxt.OHLCVToProto(kline),
		})
	}

	quotesCount := len(quotes)
	if quotesCount > 2147483647 {
		quotesCount = 2147483647
	}

	log.WithField("quote_count", quotesCount).WithField("batch_count", batchCount).Debugf("quotes fetched")
	return &cegwv1.GetQuotesResponse{
		Quotes: quotes,
		Count:  int32(quotesCount), // #nosec G115
	}, nil
}

// GetCurrentPrice fetches the latest ticker price for a single symbol.
func (s *MarketDataService) GetCurrentPrice(ctx context.Context, req *cegwv1.GetCurrentPriceRequest) (*cegwv1.GetCurrentPriceResponse, error) {
	log := s.log.WithContext(ctx).
		WithField("operation", "GetCurrentPrice").
		WithField("symbol", req.Symbol).
		WithField("exchange", req.Exchange.String())

	if req.Exchange == cegwv1.Exchange_EXCHANGE_UNSPECIFIED {
		log.Infof("invalid request: exchange unspecified")
		return nil, status.Error(codes.InvalidArgument, "exchange is required")
	}

	if req.Symbol == "" {
		log.Infof("invalid request: symbol empty")
		return nil, status.Error(codes.InvalidArgument, "symbol is required")
	}

	log.Debugf("fetching current price")

	client, err := ccxt.NewClientForExchange(ctx, req.Exchange, nil)
	if err != nil {
		log.WithError(err).Errorf("failed to create CCXT client")
		return nil, err
	}

	exchange := ccxt.AsExchange(client)
	if exchange == nil {
		log.Errorf("exchange not supported")
		return nil, status.Error(codes.Unimplemented, "exchange not supported")
	}

	ticker, err := exchange.FetchTicker(req.Symbol)
	if err != nil {
		log.WithError(err).Errorf("failed to fetch ticker")
		return nil, ccxt.MapError(err)
	}

	price := ccxt.Float64P(ticker.Close)
	log.WithField("price", price).Debugf("current price fetched")

	return &cegwv1.GetCurrentPriceResponse{
		Symbol:    req.Symbol,
		Price:     price,
		Timestamp: timestamppb.Now(),
	}, nil
}

// GetOrderBook fetches the latest order book depth for a symbol.
func (s *MarketDataService) GetOrderBook(ctx context.Context, req *cegwv1.GetOrderBookRequest) (*cegwv1.GetOrderBookResponse, error) {
	log := s.log.WithContext(ctx).
		WithField("operation", "GetOrderBook").
		WithField("symbol", req.Symbol).
		WithField("exchange", req.Exchange.String()).
		WithField("limit", req.Limit)

	if req.Exchange == cegwv1.Exchange_EXCHANGE_UNSPECIFIED {
		log.Infof("invalid request: exchange unspecified")
		return nil, status.Error(codes.InvalidArgument, "exchange is required")
	}

	if req.Symbol == "" {
		log.Infof("invalid request: symbol empty")
		return nil, status.Error(codes.InvalidArgument, "symbol is required")
	}

	if req.Limit < 0 {
		log.Infof("invalid request: negative limit")
		return nil, status.Error(codes.InvalidArgument, "limit must be greater than or equal to 0")
	}

	log.Debugf("fetching order book")

	client, err := ccxt.NewClientForExchange(ctx, req.Exchange, nil)
	if err != nil {
		log.WithError(err).Errorf("failed to create CCXT client")
		return nil, err
	}

	exchange := ccxt.AsExchange(client)
	if exchange == nil {
		log.Errorf("exchange not supported")
		return nil, status.Error(codes.Unimplemented, "exchange not supported")
	}

	opts := []ccxtlib.FetchOrderBookOptions{}
	if req.Limit > 0 {
		opts = append(opts, ccxtlib.WithFetchOrderBookLimit(int64(req.Limit)))
	}

	orderBook, err := exchange.FetchOrderBook(req.Symbol, opts...)
	if err != nil {
		log.WithError(err).Errorf("failed to fetch order book")
		return nil, ccxt.MapError(err)
	}

	bids := orderBookLevelsToProto(orderBook.Bids, req.Limit)
	asks := orderBookLevelsToProto(orderBook.Asks, req.Limit)

	log.WithField("bid_count", len(bids)).WithField("ask_count", len(asks)).Debugf("order book fetched")
	return &cegwv1.GetOrderBookResponse{
		Symbol:    req.Symbol,
		Bids:      bids,
		Asks:      asks,
		Timestamp: timestamppb.Now(),
	}, nil
}

// StreamCurrentPrice streams ticker updates via WebSocket when supported,
// falling back to REST polling otherwise.
func (s *MarketDataService) StreamCurrentPrice(req *cegwv1.GetCurrentPriceRequest, stream cegwv1.MarketDataService_StreamCurrentPriceServer) error {
	ctx := stream.Context()
	log := s.log.WithContext(ctx).
		WithField("operation", "StreamCurrentPrice").
		WithField("symbol", req.Symbol).
		WithField("exchange", req.Exchange.String())

	if req.Exchange == cegwv1.Exchange_EXCHANGE_UNSPECIFIED {
		log.Infof("invalid request: exchange unspecified")
		return status.Error(codes.InvalidArgument, "exchange is required")
	}

	if req.Symbol == "" {
		log.Infof("invalid request: symbol empty")
		return status.Error(codes.InvalidArgument, "symbol is required")
	}

	log.Debugf("starting current price stream")

	client, err := ccxt.GetClientPool(s.log).Acquire(ctx, req.Exchange, nil)
	if err != nil {
		log.WithError(err).Errorf("failed to create CCXT client")
		return err
	}
	defer ccxt.ReleaseClientForExchange(ctx, req.Exchange)

	exchange := ccxt.AsStreamingExchange(client)
	if exchange == nil {
		log.Infof("exchange streaming not supported, falling back to ticker polling")
		return s.pollCurrentPrice(ctx, client, req.Symbol, func(resp *cegwv1.GetCurrentPriceResponse) error {
			return stream.Send(resp)
		})
	}

	for {
		select {
		case <-ctx.Done():
			log.Debugf("current price stream closed by client")
			return nil
		default:
		}

		ticker, err := exchange.WatchTicker(req.Symbol)
		if err != nil {
			if ctx.Err() != nil {
				log.Debugf("current price stream closed during ticker watch")
				return nil
			}
			if ccxt.IsWatchTickerUnsupported(err) {
				log.WithError(err).Warnf("watch ticker unsupported, falling back to ticker polling")
				return s.pollCurrentPrice(ctx, client, req.Symbol, func(resp *cegwv1.GetCurrentPriceResponse) error {
					return stream.Send(resp)
				})
			}
			log.WithError(err).Errorf("failed to watch ticker")
			return ccxt.MapError(err)
		}

		resp := tickerToCurrentPriceResponse(req.Symbol, ticker)

		if err := stream.Send(resp); err != nil {
			log.WithError(err).Debugf("failed to send current price update")
			return err
		}

		log.WithField("price", resp.Price).Debugf("current price update streamed")
	}
}

// pollCurrentPrice is the REST polling fallback used when WebSocket streams
// are unavailable for the requested exchange.
func (s *MarketDataService) pollCurrentPrice(ctx context.Context, client interface{}, symbol string, send func(*cegwv1.GetCurrentPriceResponse) error) error {
	exchange := ccxt.AsExchange(client)
	if exchange == nil {
		return status.Error(codes.Unimplemented, "exchange not supported")
	}

	ticker := time.NewTicker(s.cfg.WSPricePollInterval)
	defer ticker.Stop()

	for {
		latest, err := exchange.FetchTicker(symbol)
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return ccxt.MapError(err)
		}

		if err := send(tickerToCurrentPriceResponse(symbol, latest)); err != nil {
			return err
		}

		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}
	}
}

// tickerToCurrentPriceResponse converts a CCXT ticker into the proto response.
func tickerToCurrentPriceResponse(symbol string, ticker ccxtlib.Ticker) *cegwv1.GetCurrentPriceResponse {
	price := ccxt.Float64P(ticker.Close)
	if price == 0 {
		price = ccxt.Float64P(ticker.Last)
	}

	return &cegwv1.GetCurrentPriceResponse{
		Symbol:    symbol,
		Price:     price,
		Timestamp: timestamppb.Now(),
	}
}

// orderBookLevelsToProto converts a CCXT order book side into the proto form.
func orderBookLevelsToProto(levels [][]float64, limit int32) []*cegwv1.OrderBookLevel {
	if limit > 0 && len(levels) > int(limit) {
		levels = levels[:limit]
	}

	result := make([]*cegwv1.OrderBookLevel, 0, len(levels))
	for _, level := range levels {
		if len(level) < 2 {
			continue
		}
		result = append(result, &cegwv1.OrderBookLevel{
			Price:  level[0],
			Amount: level[1],
		})
	}
	return result
}

// SearchTicker returns tickers whose symbol, base, or quote currency
// contains the query (case-insensitive). Results are ranked with prefix
// matches on the symbol first, then matches on base/quote. The response
// is capped at req.Limit (default 50, max 500).
func (s *MarketDataService) SearchTicker(ctx context.Context, req *cegwv1.SearchTickerRequest) (*cegwv1.SearchTickerResponse, error) {
	log := s.log.WithContext(ctx).
		WithField("operation", "SearchTicker").
		WithField("exchange", req.Exchange.String())

	if req.Exchange == cegwv1.Exchange_EXCHANGE_UNSPECIFIED {
		log.Infof("invalid request: exchange unspecified")
		return nil, status.Error(codes.InvalidArgument, "exchange is required")
	}

	query := strings.TrimSpace(req.Query)
	if query == "" {
		log.Infof("invalid request: query empty")
		return nil, status.Error(codes.InvalidArgument, "query is required")
	}
	log = log.WithField("query", query)

	if req.Limit < 0 {
		log.Infof("invalid request: negative limit")
		return nil, status.Error(codes.InvalidArgument, "limit must be greater than or equal to 0")
	}
	limit := int(req.Limit)
	if limit == 0 {
		limit = defaultSearchLimit
	}
	if limit > maxSearchLimit {
		limit = maxSearchLimit
	}
	log = log.WithField("limit", limit)

	log.Debugf("searching tickers")

	tickers, err := s.ensureCache(ctx, req.Exchange)
	if err != nil {
		log.WithError(err).Warnf("cache unavailable")
		return nil, err
	}

	q := strings.ToLower(query)
	results := rankTickerMatches(tickers, q, limit)

	log.WithField("result_count", len(results)).Debugf("search completed")
	return &cegwv1.SearchTickerResponse{Tickers: results}, nil
}

// matchRank describes how a ticker matches a query:
//  - rankSymbolPrefix: best — symbol starts with the query
//  - rankSymbolContains: symbol contains the query
//  - rankBaseOrQuote: query matches the base or quote currency
type matchRank int

const (
	rankNoMatch matchRank = iota
	rankBaseOrQuote
	rankSymbolContains
	rankSymbolPrefix
)

// rankedTicker holds a ticker and its match rank for sorting.
type rankedTicker struct {
	ticker *cegwv1.Ticker
	rank   matchRank
}

// rankTickerMatches scans the cache for tickers matching the lowercased
// query, sorts them by match quality (prefix > contains > base/quote),
// breaks ties alphabetically by symbol, and returns up to `limit` results.
func rankTickerMatches(cached []cachedTicker, qLower string, limit int) []*cegwv1.Ticker {
	if limit <= 0 || len(cached) == 0 {
		return nil
	}

	matches := make([]rankedTicker, 0, 16)
	for i := range cached {
		t := &cached[i]
		rank := classifyMatch(t, qLower)
		if rank == rankNoMatch {
			continue
		}
		matches = append(matches, rankedTicker{ticker: t.proto, rank: rank})
	}

	sort.SliceStable(matches, func(i, j int) bool {
		if matches[i].rank != matches[j].rank {
			return matches[i].rank > matches[j].rank
		}
		return matches[i].ticker.Symbol < matches[j].ticker.Symbol
	})

	if len(matches) > limit {
		matches = matches[:limit]
	}

	out := make([]*cegwv1.Ticker, len(matches))
	for i, m := range matches {
		out[i] = m.ticker
	}
	return out
}

// classifyMatch returns the best match rank for a ticker against qLower.
func classifyMatch(t *cachedTicker, qLower string) matchRank {
	switch {
	case strings.HasPrefix(t.symbolLower, qLower):
		return rankSymbolPrefix
	case strings.Contains(t.symbolLower, qLower):
		return rankSymbolContains
	case t.baseLower != "" && strings.Contains(t.baseLower, qLower):
		return rankBaseOrQuote
	case t.quoteLower != "" && strings.Contains(t.quoteLower, qLower):
		return rankBaseOrQuote
	default:
		return rankNoMatch
	}
}

// ListMarkets returns the full market list for an exchange, loading it on
// demand if the cache is empty or stale.
func (s *MarketDataService) ListMarkets(ctx context.Context, req *cegwv1.ListMarketsRequest) (*cegwv1.ListMarketsResponse, error) {
	log := s.log.WithContext(ctx).
		WithField("operation", "ListMarkets").
		WithField("exchange", req.Exchange.String())

	if req.Exchange == cegwv1.Exchange_EXCHANGE_UNSPECIFIED {
		log.Infof("invalid request: exchange unspecified")
		return nil, status.Error(codes.InvalidArgument, "exchange is required")
	}

	log.Debugf("loading markets")

	cached, err := s.ensureCache(ctx, req.Exchange)
	if err != nil {
		log.WithError(err).Warnf("cache unavailable")
		return nil, err
	}

markets := make([]*cegwv1.Market, 0, len(cached))
	for i := range cached {
		t := &cached[i]
		markets = append(markets, &cegwv1.Market{
			Symbol: t.proto.Symbol,
			Base:   t.proto.Base,
			Quote:  t.proto.Quote,
			Active: t.active,
		})
	}

	marketsCount := len(markets)
	if marketsCount > 2147483647 {
		marketsCount = 2147483647
	}

	log.WithField("market_count", marketsCount).Debugf("markets loaded")
	return &cegwv1.ListMarketsResponse{
		Markets: markets,
		Count:   int32(marketsCount), // #nosec G115
	}, nil
}