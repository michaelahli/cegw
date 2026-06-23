package service

import (
	"context"
	"sync"
	"time"

	ccxtlib "github.com/ccxt/ccxt/go/v4"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	cegwv1 "github.com/michaelahli/cegw/gen/cegw/v1"
	"github.com/michaelahli/cegw/internal/ccxt"
	"github.com/michaelahli/cegw/internal/config"
	"github.com/michaelahli/cegw/internal/logger"
	"github.com/michaelahli/cegw/internal/metrics"
)

type MarketDataService struct {
	cegwv1.UnimplementedMarketDataServiceServer
	cfg         *config.Config
	log         *logger.Logger
	availByExch map[cegwv1.Exchange][]*cegwv1.Ticker
	availMutex  sync.RWMutex
	cacheReady  map[cegwv1.Exchange]bool
	metrics     *metrics.Metrics
}

func NewMarketDataService(cfg *config.Config, log *logger.Logger, m *metrics.Metrics) *MarketDataService {
	svc := &MarketDataService{
		cfg:         cfg,
		log:         log,
		availByExch: make(map[cegwv1.Exchange][]*cegwv1.Ticker),
		cacheReady:  make(map[cegwv1.Exchange]bool),
		metrics:     m,
	}
	go svc.cacheMarkets(cegwv1.Exchange_EXCHANGE_TOKOCRYPTO)
	return svc
}

func (s *MarketDataService) cacheMarkets(exchangeID cegwv1.Exchange) {
	ctx := context.Background()
	log := s.log.WithContext(ctx).WithField("operation", "cacheMarkets")

	log.Debugf("initializing market cache")

	client, err := ccxt.NewClientForExchange(ctx, exchangeID, nil)
	if err != nil || client == nil {
		log.WithError(err).WithField("exchange", exchangeID.String()).Warnf("failed to initialize CCXT client for market cache")
		return
	}

	exchange := ccxt.AsExchange(client)
	if exchange == nil {
		log.Warnf("failed to cast client to exchange interface")
		return
	}

	log.Debugf("loading markets from exchange")
	markets, err := exchange.LoadMarkets()
	if err != nil {
		log.WithError(err).Errorf("failed to load markets from Tokocrypto")
		return
	}

	tickers := make([]*cegwv1.Ticker, 0, len(markets))
	for _, market := range markets {
		tickers = append(tickers, &cegwv1.Ticker{
			Symbol: ccxt.StringP(market.Symbol),
		})
	}

	s.availMutex.Lock()
	s.availByExch[exchangeID] = tickers
	s.cacheReady[exchangeID] = true
	s.availMutex.Unlock()

	log.WithField("ticker_count", len(tickers)).Infof("market cache initialized successfully")
}

func (s *MarketDataService) GetQuotes(ctx context.Context, req *cegwv1.GetQuotesRequest) (*cegwv1.GetQuotesResponse, error) {
	log := s.log.WithContext(ctx).
		WithField("operation", "GetQuotes").
		WithField("symbol", req.Symbol).
		WithField("exchange", req.Exchange.String())

	if req.Exchange == cegwv1.Exchange_EXCHANGE_UNSPECIFIED {
		log.Warnf("invalid request: exchange unspecified")
		return nil, status.Error(codes.InvalidArgument, "exchange is required")
	}

	if req.Symbol == "" {
		log.Warnf("invalid request: symbol empty")
		return nil, status.Error(codes.InvalidArgument, "symbol is required")
	}

	// Validate interval support for the exchange
	if !ccxt.IsIntervalSupported(req.Exchange, req.Interval) {
		log.WithField("interval", req.Interval.String()).Warnf("interval not supported by exchange")
		return nil, status.Errorf(codes.InvalidArgument, "interval %s is not supported by %s", req.Interval.String(), req.Exchange.String())
	}

	interval := ccxt.MapInterval(req.Interval)
	if interval == "" {
		log.WithField("interval", req.Interval).Warnf("invalid interval")
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
		log.Warnf("exchange not supported")
		return nil, status.Error(codes.Unimplemented, "exchange not supported")
	}

	var mergedKlines []ccxtlib.OHLCV
	start := req.Start.AsTime()
	if start.IsZero() {
		start = time.Now().In(s.cfg.Timezone).Add(-24 * time.Hour)
	}

	shiftedStart := start
	end := time.Time{}
	if req.End != nil && !req.End.AsTime().IsZero() {
		end = req.End.AsTime()
	}

	// Default batch limit for CCXT API
	batchLimit := int64(1000)

	// Apply user-supplied limit if specified
	userLimit := int64(0)
	if req.Limit > 0 {
		userLimit = int64(req.Limit)
		log = log.WithField("user_limit", userLimit)
	}

	batchCount := 0
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

		// Stop batching if user limit is reached
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

		// Stop if user limit is reached after appending
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

	// Truncate to user limit if specified
	if userLimit > 0 && int64(len(mergedKlines)) > userLimit {
		mergedKlines = mergedKlines[:userLimit]
		log.WithField("truncated_to", userLimit).Debugf("truncated quotes to user limit")
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

	log.WithField("quote_count", quotesCount).WithField("batch_count", batchCount).Infof("quotes fetched successfully")
	return &cegwv1.GetQuotesResponse{
		Quotes: quotes,
		Count:  int32(quotesCount), // #nosec G115
	}, nil
}

func (s *MarketDataService) GetCurrentPrice(ctx context.Context, req *cegwv1.GetCurrentPriceRequest) (*cegwv1.GetCurrentPriceResponse, error) {
	log := s.log.WithContext(ctx).
		WithField("operation", "GetCurrentPrice").
		WithField("symbol", req.Symbol).
		WithField("exchange", req.Exchange.String())

	if req.Exchange == cegwv1.Exchange_EXCHANGE_UNSPECIFIED {
		log.Warnf("invalid request: exchange unspecified")
		return nil, status.Error(codes.InvalidArgument, "exchange is required")
	}

	if req.Symbol == "" {
		log.Warnf("invalid request: symbol empty")
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
		log.Warnf("exchange not supported")
		return nil, status.Error(codes.Unimplemented, "exchange not supported")
	}

	ticker, err := exchange.FetchTicker(req.Symbol)
	if err != nil {
		log.WithError(err).Errorf("failed to fetch ticker")
		return nil, ccxt.MapError(err)
	}

	price := ccxt.Float64P(ticker.Close)
	log.WithField("price", price).Infof("current price fetched successfully")

	return &cegwv1.GetCurrentPriceResponse{
		Symbol:    req.Symbol,
		Price:     price,
		Timestamp: timestamppb.Now(),
	}, nil
}

func (s *MarketDataService) GetOrderBook(ctx context.Context, req *cegwv1.GetOrderBookRequest) (*cegwv1.GetOrderBookResponse, error) {
	log := s.log.WithContext(ctx).
		WithField("operation", "GetOrderBook").
		WithField("symbol", req.Symbol).
		WithField("exchange", req.Exchange.String()).
		WithField("limit", req.Limit)

	if req.Exchange == cegwv1.Exchange_EXCHANGE_UNSPECIFIED {
		log.Warnf("invalid request: exchange unspecified")
		return nil, status.Error(codes.InvalidArgument, "exchange is required")
	}

	if req.Symbol == "" {
		log.Warnf("invalid request: symbol empty")
		return nil, status.Error(codes.InvalidArgument, "symbol is required")
	}

	if req.Limit < 0 {
		log.Warnf("invalid request: negative limit")
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
		log.Warnf("exchange not supported")
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

	log.WithField("bid_count", len(bids)).WithField("ask_count", len(asks)).Infof("order book fetched successfully")
	return &cegwv1.GetOrderBookResponse{
		Symbol:    req.Symbol,
		Bids:      bids,
		Asks:      asks,
		Timestamp: timestamppb.Now(),
	}, nil
}

func (s *MarketDataService) StreamCurrentPrice(req *cegwv1.GetCurrentPriceRequest, stream cegwv1.MarketDataService_StreamCurrentPriceServer) error {
	ctx := stream.Context()
	log := s.log.WithContext(ctx).
		WithField("operation", "StreamCurrentPrice").
		WithField("symbol", req.Symbol).
		WithField("exchange", req.Exchange.String())

	if req.Exchange == cegwv1.Exchange_EXCHANGE_UNSPECIFIED {
		log.Warnf("invalid request: exchange unspecified")
		return status.Error(codes.InvalidArgument, "exchange is required")
	}

	if req.Symbol == "" {
		log.Warnf("invalid request: symbol empty")
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
		log.Warnf("exchange streaming not supported, falling back to ticker polling")
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

func (s *MarketDataService) SearchTicker(ctx context.Context, req *cegwv1.SearchTickerRequest) (*cegwv1.SearchTickerResponse, error) {
	log := s.log.WithContext(ctx).
		WithField("operation", "SearchTicker").
		WithField("query", req.Query).
		WithField("exchange", req.Exchange.String())

	if req.Exchange == cegwv1.Exchange_EXCHANGE_UNSPECIFIED {
		log.Warnf("invalid request: exchange unspecified")
		return nil, status.Error(codes.InvalidArgument, "exchange is required")
	}

	if req.Query == "" {
		log.Warnf("invalid request: query empty")
		return nil, status.Error(codes.InvalidArgument, "query is required")
	}

	log.Debugf("searching tickers")

	s.availMutex.RLock()
	ready := s.cacheReady[req.Exchange]
	_, exists := s.availByExch[req.Exchange]
	s.availMutex.RUnlock()

	if !ready || !exists {
		log.WithField("exchange", req.Exchange.String()).Debugf("market cache missing for exchange, loading on demand")
		s.cacheMarkets(req.Exchange)
	}

	s.availMutex.RLock()
	if !s.cacheReady[req.Exchange] {
		s.availMutex.RUnlock()
		log.WithField("exchange", req.Exchange.String()).Warnf("market cache not ready yet")
		return nil, status.Error(codes.Unavailable, "market cache is still loading, please retry")
	}
	availForExchange := s.availByExch[req.Exchange]
	availCopy := make([]*cegwv1.Ticker, len(availForExchange))
	copy(availCopy, availForExchange)
	s.availMutex.RUnlock()

	var filtered []*cegwv1.Ticker
	for _, ticker := range availCopy {
		if contains(ticker.Symbol, req.Query) {
			filtered = append(filtered, ticker)
		}
	}

	log.WithField("result_count", len(filtered)).Infof("search completed")
	return &cegwv1.SearchTickerResponse{Tickers: filtered}, nil
}

func (s *MarketDataService) ListMarkets(ctx context.Context, req *cegwv1.ListMarketsRequest) (*cegwv1.ListMarketsResponse, error) {
	log := s.log.WithContext(ctx).
		WithField("operation", "ListMarkets").
		WithField("exchange", req.Exchange.String())

	if req.Exchange == cegwv1.Exchange_EXCHANGE_UNSPECIFIED {
		log.Warnf("invalid request: exchange unspecified")
		return nil, status.Error(codes.InvalidArgument, "exchange is required")
	}

	log.Debugf("loading markets")

	client, err := ccxt.NewClientForExchange(ctx, req.Exchange, nil)
	if err != nil {
		log.WithError(err).Errorf("failed to create CCXT client")
		return nil, err
	}

	exchange := ccxt.AsExchange(client)
	if exchange == nil {
		log.Warnf("exchange not supported")
		return nil, status.Error(codes.Unimplemented, "exchange not supported")
	}

	marketData, err := exchange.LoadMarkets()
	if err != nil {
		log.WithError(err).Errorf("failed to load markets")
		return nil, ccxt.MapError(err)
	}

	markets := make([]*cegwv1.Market, 0, len(marketData))
	for _, m := range marketData {
		base := ""
		quote := ""
		if m.BaseId != nil {
			base = *m.BaseId
		}
		if m.QuoteId != nil {
			quote = *m.QuoteId
		}
		markets = append(markets, &cegwv1.Market{
			Symbol: ccxt.StringP(m.Symbol),
			Base:   base,
			Quote:  quote,
			Active: m.Active != nil && *m.Active,
		})
	}

	marketsCount := len(markets)
	if marketsCount > 2147483647 {
		marketsCount = 2147483647
	}

	log.WithField("market_count", marketsCount).Infof("markets loaded successfully")
	return &cegwv1.ListMarketsResponse{
		Markets: markets,
		Count:   int32(marketsCount), // #nosec G115
	}, nil
}

func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
