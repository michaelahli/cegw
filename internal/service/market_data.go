package service

import (
	"context"
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
	cfg     *config.Config
	log     *logger.Logger
	avail   []*cegwv1.Ticker
	metrics *metrics.Metrics
}

func NewMarketDataService(cfg *config.Config, log *logger.Logger, m *metrics.Metrics) *MarketDataService {
	svc := &MarketDataService{
		cfg:     cfg,
		log:     log,
		metrics: m,
	}
	go svc.cacheMarkets()
	return svc
}

func (s *MarketDataService) cacheMarkets() {
	ctx := context.Background()
	log := s.log.WithContext(ctx).WithField("operation", "cacheMarkets")

	log.Debugf("initializing market cache")

	client, err := ccxt.NewClientForExchange(ctx, cegwv1.Exchange_EXCHANGE_TOKOCRYPTO, nil)
	if err != nil || client == nil {
		log.WithError(err).WithField("exchange", "TOKOCRYPTO").Warnf("failed to initialize CCXT client for market cache")
		return
	}

	exchange := ccxt.AsExchange(client)
	if exchange == nil {
		log.Warnf("failed to cast client to exchange interface")
		return
	}

	log.Debugf("loading markets from Tokocrypto")
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
	s.avail = tickers
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

	batchCount := 0
	for {
		limit := int64(1000)
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

		if len(klines) < 1000 {
			break
		}

		last := klines[len(klines)-1]
		shiftedStart = time.UnixMilli(last.Timestamp).Add(time.Millisecond)
		if !end.IsZero() && shiftedStart.After(end) {
			break
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

	client, err := ccxt.NewClientForExchange(ctx, req.Exchange, nil)
	if err != nil {
		log.WithError(err).Errorf("failed to create CCXT client")
		return err
	}

	exchange := ccxt.AsStreamingExchange(client)
	if exchange == nil {
		log.Warnf("exchange streaming not supported")
		return status.Error(codes.Unimplemented, "exchange streaming not supported")
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
			log.WithError(err).Errorf("failed to watch ticker")
			return ccxt.MapError(err)
		}

		price := ccxt.Float64P(ticker.Close)
		if price == 0 {
			price = ccxt.Float64P(ticker.Last)
		}

		resp := &cegwv1.GetCurrentPriceResponse{
			Symbol:    req.Symbol,
			Price:     price,
			Timestamp: timestamppb.Now(),
		}

		if err := stream.Send(resp); err != nil {
			log.WithError(err).Debugf("failed to send current price update")
			return err
		}

		log.WithField("price", price).Debugf("current price update streamed")
	}
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

	var filtered []*cegwv1.Ticker
	for _, ticker := range s.avail {
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
