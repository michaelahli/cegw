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

type TradingService struct {
	cegwv1.UnimplementedTradingServiceServer
	cfg     *config.Config
	log     *logger.Logger
	metrics *metrics.Metrics
}

func NewTradingService(cfg *config.Config, log *logger.Logger, m *metrics.Metrics) *TradingService {
	return &TradingService{
		cfg:     cfg,
		log:     log,
		metrics: m,
	}
}

func (s *TradingService) CreateMarketOrder(ctx context.Context, req *cegwv1.CreateMarketOrderRequest) (*cegwv1.CreateMarketOrderResponse, error) {
	log := s.log.WithContext(ctx).
		WithField("operation", "CreateMarketOrder").
		WithField("symbol", req.Symbol).
		WithField("exchange", req.Exchange.String()).
		WithField("side", req.Side.String()).
		WithField("quantity", req.Quantity)

	if req.Exchange == cegwv1.Exchange_EXCHANGE_UNSPECIFIED {
		log.Warnf("invalid request: exchange unspecified")
		return nil, status.Error(codes.InvalidArgument, "exchange is required")
	}

	if req.Symbol == "" {
		log.Warnf("invalid request: symbol empty")
		return nil, status.Error(codes.InvalidArgument, "symbol is required")
	}

	if req.Side == cegwv1.OrderSide_ORDER_SIDE_UNSPECIFIED {
		log.Warnf("invalid request: side unspecified")
		return nil, status.Error(codes.InvalidArgument, "order side is required")
	}

	if req.Quantity <= 0 {
		log.Warnf("invalid request: quantity not positive")
		return nil, status.Error(codes.InvalidArgument, "quantity must be positive")
	}

	if req.Credentials == nil {
		log.Warnf("invalid request: credentials missing")
		return nil, status.Error(codes.InvalidArgument, "credentials are required")
	}

	if req.Credentials.ApiKey == "" {
		log.Warnf("invalid request: api_key missing")
		return nil, status.Error(codes.InvalidArgument, "api_key is required")
	}

	if req.Credentials.ApiSecret == "" {
		log.Warnf("invalid request: api_secret missing")
		return nil, status.Error(codes.InvalidArgument, "api_secret is required")
	}

	log.Debugf("creating market order")

	client, err := ccxt.NewClientForExchange(ctx, req.Exchange, req.Credentials)
	if err != nil {
		log.WithError(err).Errorf("failed to create CCXT client")
		return nil, err
	}

	exchange := ccxt.AsExchange(client)
	if exchange == nil {
		log.Warnf("exchange not supported")
		return nil, status.Error(codes.Unimplemented, "exchange not supported")
	}

	var order ccxtlib.Order

	switch req.Side {
	case cegwv1.OrderSide_ORDER_SIDE_BUY:
		log.Debugf("fetching ticker for buy order")
		ticker, err := exchange.FetchTicker(req.Symbol)
		if err != nil {
			log.WithError(err).Errorf("failed to fetch ticker")
			return nil, ccxt.MapError(err)
		}

		ask := ccxt.Float64P(ticker.Ask)
		if ask == 0 {
			ask = ccxt.Float64P(ticker.Last)
		}
		if ask == 0 {
			log.Errorf("no ask/last price available for buy order")
			return nil, status.Error(codes.Internal, "no ask/last price available")
		}

		log.WithField("ask_price", ask).Debugf("creating buy market order")
		order, err = exchange.CreateMarketOrder(req.Symbol, "buy", req.Quantity,
			ccxtlib.WithCreateMarketOrderPrice(ask))
		if err != nil {
			log.WithError(err).Errorf("failed to create buy market order")
			return nil, ccxt.MapError(err)
		}

	case cegwv1.OrderSide_ORDER_SIDE_SELL:
		log.Debugf("creating sell market order")
		order, err = exchange.CreateMarketOrder(req.Symbol, "sell", req.Quantity)
		if err != nil {
			log.WithError(err).Errorf("failed to create sell market order")
			return nil, ccxt.MapError(err)
		}

	default:
		log.Warnf("invalid order side")
		return nil, status.Error(codes.InvalidArgument, "invalid order side")
	}

	orderStatus := cegwv1.OrderStatus_ORDER_STATUS_UNSPECIFIED
	if order.Status != nil {
		switch *order.Status {
		case "new":
			orderStatus = cegwv1.OrderStatus_ORDER_STATUS_NEW
		case "filled":
			orderStatus = cegwv1.OrderStatus_ORDER_STATUS_FILLED
		case "partially_filled":
			orderStatus = cegwv1.OrderStatus_ORDER_STATUS_PARTIALLY_FILLED
		case "canceled":
			orderStatus = cegwv1.OrderStatus_ORDER_STATUS_CANCELED
		case "rejected":
			orderStatus = cegwv1.OrderStatus_ORDER_STATUS_REJECTED
		}
	}

	var timestamp *timestamppb.Timestamp
	if order.Timestamp != nil {
		timestamp = timestamppb.New(time.UnixMilli(ccxt.Int64P(order.Timestamp)))
	} else {
		timestamp = timestamppb.Now()
	}

	log.WithField("order_id", order.Id).
		WithField("order_status", orderStatus.String()).
		Infof("market order created successfully")

	return &cegwv1.CreateMarketOrderResponse{
		Order: &cegwv1.Order{
			OrderId:   ccxt.StringP(order.Id),
			Symbol:    req.Symbol,
			Side:      req.Side,
			Quantity:  ccxt.Float64P(order.Amount),
			Price:     ccxt.Float64P(order.Price),
			Status:    orderStatus,
			Timestamp: timestamp,
		},
	}, nil
}

func (s *TradingService) TestCredentials(ctx context.Context, req *cegwv1.TestCredentialsRequest) (*cegwv1.TestCredentialsResponse, error) {
	log := s.log.WithContext(ctx).
		WithField("operation", "TestCredentials").
		WithField("exchange", req.Exchange.String())

	if req.Exchange == cegwv1.Exchange_EXCHANGE_UNSPECIFIED {
		log.Warnf("invalid request: exchange unspecified")
		return nil, status.Error(codes.InvalidArgument, "exchange is required")
	}

	if req.Credentials == nil {
		log.Warnf("invalid request: credentials missing")
		return nil, status.Error(codes.InvalidArgument, "credentials are required")
	}

	if req.Credentials.ApiKey == "" {
		log.Warnf("invalid request: api_key missing")
		return nil, status.Error(codes.InvalidArgument, "api_key is required")
	}

	if req.Credentials.ApiSecret == "" {
		log.Warnf("invalid request: api_secret missing")
		return nil, status.Error(codes.InvalidArgument, "api_secret is required")
	}

	log.Debugf("testing credentials")

	client, err := ccxt.NewClientForExchange(ctx, req.Exchange, req.Credentials)
	if err != nil {
		log.WithError(err).Warnf("failed to create CCXT client during credential test")
		return &cegwv1.TestCredentialsResponse{
			Valid:   false,
			Message: "invalid credentials",
		}, nil
	}

	exchange := ccxt.AsExchange(client)
	if exchange == nil {
		log.Warnf("exchange not supported")
		return nil, status.Error(codes.Unimplemented, "exchange not supported")
	}

	balances, err := exchange.FetchBalance()
	if err != nil {
		log.WithError(err).Warnf("failed to fetch balance during credential test")
		return &cegwv1.TestCredentialsResponse{
			Valid:   false,
			Message: "invalid credentials",
		}, nil
	}

	if len(balances.Balances) < 1 {
		log.Warnf("no balances found for credentials")
		return &cegwv1.TestCredentialsResponse{
			Valid:   false,
			Message: "invalid credentials",
		}, nil
	}

	log.WithField("balance_count", len(balances.Balances)).Infof("credentials validated successfully")
	return &cegwv1.TestCredentialsResponse{
		Valid:   true,
		Message: "credentials are valid",
	}, nil
}
