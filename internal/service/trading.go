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
)

type TradingService struct {
	cegwv1.UnimplementedTradingServiceServer
	cfg *config.Config
}

func NewTradingService(cfg *config.Config) *TradingService {
	return &TradingService{
		cfg: cfg,
	}
}

func (s *TradingService) CreateMarketOrder(ctx context.Context, req *cegwv1.CreateMarketOrderRequest) (*cegwv1.CreateMarketOrderResponse, error) {
	if req.Exchange == cegwv1.Exchange_EXCHANGE_UNSPECIFIED {
		return nil, status.Error(codes.InvalidArgument, "exchange is required")
	}

	if req.Symbol == "" {
		return nil, status.Error(codes.InvalidArgument, "symbol is required")
	}

	if req.Side == cegwv1.OrderSide_ORDER_SIDE_UNSPECIFIED {
		return nil, status.Error(codes.InvalidArgument, "order side is required")
	}

	if req.Quantity <= 0 {
		return nil, status.Error(codes.InvalidArgument, "quantity must be positive")
	}

	if req.Credentials == nil {
		return nil, status.Error(codes.InvalidArgument, "credentials are required")
	}

	client, err := ccxt.NewClientForExchange(ctx, req.Exchange, req.Credentials)
	if err != nil {
		return nil, err
	}

	tokocrypto, ok := client.(*ccxtlib.Tokocrypto)
	if !ok {
		return nil, status.Error(codes.Unimplemented, "exchange not supported")
	}

	var order ccxtlib.Order

	switch req.Side {
	case cegwv1.OrderSide_ORDER_SIDE_BUY:
		ticker, err := tokocrypto.FetchTicker(req.Symbol)
		if err != nil {
			return nil, ccxt.MapError(err)
		}

		ask := ccxt.Float64P(ticker.Ask)
		if ask == 0 {
			ask = ccxt.Float64P(ticker.Last)
		}
		if ask == 0 {
			return nil, status.Error(codes.Internal, "no ask/last price available")
		}

		order, err = tokocrypto.CreateMarketOrder(req.Symbol, "buy", req.Quantity,
			ccxtlib.WithCreateMarketOrderPrice(ask))
		if err != nil {
			return nil, ccxt.MapError(err)
		}

	case cegwv1.OrderSide_ORDER_SIDE_SELL:
		order, err = tokocrypto.CreateMarketOrder(req.Symbol, "sell", req.Quantity)
		if err != nil {
			return nil, ccxt.MapError(err)
		}

	default:
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
	if req.Exchange == cegwv1.Exchange_EXCHANGE_UNSPECIFIED {
		return nil, status.Error(codes.InvalidArgument, "exchange is required")
	}

	if req.Credentials == nil {
		return nil, status.Error(codes.InvalidArgument, "credentials are required")
	}

	client, err := ccxt.NewClientForExchange(ctx, req.Exchange, req.Credentials)
	if err != nil {
		return nil, err
	}

	tokocrypto, ok := client.(*ccxtlib.Tokocrypto)
	if !ok {
		return nil, status.Error(codes.Unimplemented, "exchange not supported")
	}

	balances, err := tokocrypto.FetchBalance()
	if err != nil {
		return &cegwv1.TestCredentialsResponse{
			Valid:   false,
			Message: "invalid credentials",
		}, nil
	}

	if len(balances.Balances) < 1 {
		return &cegwv1.TestCredentialsResponse{
			Valid:   false,
			Message: "invalid credentials",
		}, nil
	}

	return &cegwv1.TestCredentialsResponse{
		Valid:   true,
		Message: "credentials are valid",
	}, nil
}
