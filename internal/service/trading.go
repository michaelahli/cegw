package service

import (
	"context"
	"fmt"
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
		log.Infof("invalid request: exchange unspecified")
		return nil, status.Error(codes.InvalidArgument, "exchange is required")
	}

	if req.Symbol == "" {
		log.Infof("invalid request: symbol empty")
		return nil, status.Error(codes.InvalidArgument, "symbol is required")
	}

	if req.Side == cegwv1.OrderSide_ORDER_SIDE_UNSPECIFIED {
		log.Infof("invalid request: side unspecified")
		return nil, status.Error(codes.InvalidArgument, "order side is required")
	}

	if req.Quantity <= 0 {
		log.Infof("invalid request: quantity not positive")
		return nil, status.Error(codes.InvalidArgument, "quantity must be positive")
	}

	if req.Credentials == nil {
		log.Infof("invalid request: credentials missing")
		return nil, status.Error(codes.InvalidArgument, "credentials are required")
	}

	if req.Credentials.ApiKey == "" {
		log.Infof("invalid request: api_key missing")
		return nil, status.Error(codes.InvalidArgument, "api_key is required")
	}

	if req.Credentials.ApiSecret == "" {
		log.Infof("invalid request: api_secret missing")
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
		log.Errorf("exchange not supported")
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
		log.Infof("invalid order side")
		return nil, status.Error(codes.InvalidArgument, "invalid order side")
	}

	log.WithField("order_id", order.Id).
		WithField("order_status", mapOrderStatus(order.Status).String()).
Debugf("market order created")

	return &cegwv1.CreateMarketOrderResponse{
		Order: buildOrder(order, ccxt.Float64P(order.Price)),
	}, nil
}

// mapOrderStatus converts a CCXT order status string to proto OrderStatus enum.
// CCXT uses many statuses: open, closed, canceled, filled, new, partially_filled,
// rejected, expired, etc. We map them to a simplified proto enum.
func mapOrderStatus(status *string) cegwv1.OrderStatus {
	if status == nil {
		return cegwv1.OrderStatus_ORDER_STATUS_UNSPECIFIED
	}
	switch *status {
	case "open":
		return cegwv1.OrderStatus_ORDER_STATUS_NEW
	case "new":
		return cegwv1.OrderStatus_ORDER_STATUS_NEW
	case "filled":
		return cegwv1.OrderStatus_ORDER_STATUS_FILLED
	case "partially_filled":
		return cegwv1.OrderStatus_ORDER_STATUS_PARTIALLY_FILLED
	case "canceled":
		return cegwv1.OrderStatus_ORDER_STATUS_CANCELED
	case "rejected":
		return cegwv1.OrderStatus_ORDER_STATUS_REJECTED
	case "closed":
		return cegwv1.OrderStatus_ORDER_STATUS_FILLED
	case "expired":
		return cegwv1.OrderStatus_ORDER_STATUS_REJECTED
	default:
		return cegwv1.OrderStatus_ORDER_STATUS_NEW
	}
}

// mapOrderSide converts a CCXT side string to proto OrderSide enum.
func mapOrderSide(side *string) cegwv1.OrderSide {
	if side == nil {
		return cegwv1.OrderSide_ORDER_SIDE_UNSPECIFIED
	}
	switch *side {
	case "buy":
		return cegwv1.OrderSide_ORDER_SIDE_BUY
	case "sell":
		return cegwv1.OrderSide_ORDER_SIDE_SELL
	default:
		return cegwv1.OrderSide_ORDER_SIDE_UNSPECIFIED
	}
}

// orderTimestamp converts a CCXT millisecond timestamp to protobuf Timestamp.
func orderTimestamp(ts *int64) *timestamppb.Timestamp {
	if ts != nil {
		return timestamppb.New(time.UnixMilli(*ts))
	}
	return timestamppb.Now()
}

// buildOrder converts a CCXT Order to the proto Order message.
func buildOrder(order ccxtlib.Order, overridePrice float64) *cegwv1.Order {
	price := ccxt.Float64P(order.Price)
	if overridePrice != 0 {
		price = overridePrice
	}
	return &cegwv1.Order{
		OrderId:   ccxt.StringP(order.Id),
		Symbol:    ccxt.StringP(order.Symbol),
		Side:      mapOrderSide(order.Side),
		Quantity:  ccxt.Float64P(order.Amount),
		Price:     price,
		Status:    mapOrderStatus(order.Status),
		Timestamp: orderTimestamp(order.Timestamp),
	}
}

// timeInForceToString converts the TimeInForce enum to CCXT string format.
func timeInForceToString(tif cegwv1.TimeInForce) string {
	switch tif {
	case cegwv1.TimeInForce_TIME_IN_FORCE_GTC:
		return "GTC"
	case cegwv1.TimeInForce_TIME_IN_FORCE_IOC:
		return "IOC"
	case cegwv1.TimeInForce_TIME_IN_FORCE_FOK:
		return "FOK"
	default:
		return "GTC"
	}
}

func (s *TradingService) CreateLimitOrder(ctx context.Context, req *cegwv1.CreateLimitOrderRequest) (*cegwv1.CreateLimitOrderResponse, error) {
	log := s.log.WithContext(ctx).
		WithField("operation", "CreateLimitOrder").
		WithField("symbol", req.Symbol).
		WithField("exchange", req.Exchange.String()).
		WithField("side", req.Side.String()).
		WithField("quantity", req.Quantity).
		WithField("price", req.Price)

	if req.Exchange == cegwv1.Exchange_EXCHANGE_UNSPECIFIED {
		log.Infof("invalid request: exchange unspecified")
		return nil, status.Error(codes.InvalidArgument, "exchange is required")
	}

	if req.Symbol == "" {
		log.Infof("invalid request: symbol empty")
		return nil, status.Error(codes.InvalidArgument, "symbol is required")
	}

	if req.Side == cegwv1.OrderSide_ORDER_SIDE_UNSPECIFIED {
		log.Infof("invalid request: side unspecified")
		return nil, status.Error(codes.InvalidArgument, "order side is required")
	}

	if req.Quantity <= 0 {
		log.Infof("invalid request: quantity not positive")
		return nil, status.Error(codes.InvalidArgument, "quantity must be positive")
	}

	if req.Price <= 0 {
		log.Infof("invalid request: price not positive")
		return nil, status.Error(codes.InvalidArgument, "price must be positive")
	}

	if req.Credentials == nil {
		log.Infof("invalid request: credentials missing")
		return nil, status.Error(codes.InvalidArgument, "credentials are required")
	}

	if req.Credentials.ApiKey == "" {
		log.Infof("invalid request: api_key missing")
		return nil, status.Error(codes.InvalidArgument, "api_key is required")
	}

	if req.Credentials.ApiSecret == "" {
		log.Infof("invalid request: api_secret missing")
		return nil, status.Error(codes.InvalidArgument, "api_secret is required")
	}

	log.Debugf("creating limit order")

	client, err := ccxt.NewClientForExchange(ctx, req.Exchange, req.Credentials)
	if err != nil {
		log.WithError(err).Errorf("failed to create CCXT client")
		return nil, err
	}

	exchange := ccxt.AsExchange(client)
	if exchange == nil {
		log.Errorf("exchange not supported")
		return nil, status.Error(codes.Unimplemented, "exchange not supported")
	}

	var side string
	switch req.Side {
	case cegwv1.OrderSide_ORDER_SIDE_BUY:
		side = "buy"
	case cegwv1.OrderSide_ORDER_SIDE_SELL:
		side = "sell"
	default:
		log.Infof("invalid order side")
		return nil, status.Error(codes.InvalidArgument, "invalid order side")
	}

	var opts []ccxtlib.CreateLimitOrderOptions
	if req.TimeInForce != cegwv1.TimeInForce_TIME_IN_FORCE_UNSPECIFIED {
		tif := timeInForceToString(req.TimeInForce)
		log.WithField("time_in_force", tif).Debugf("setting time-in-force")
		opts = append(opts, ccxtlib.WithCreateLimitOrderParams(map[string]any{
			"timeInForce": tif,
		}))
	}

	order, err := exchange.CreateLimitOrder(req.Symbol, side, req.Quantity, req.Price, opts...)
	if err != nil {
		log.WithError(err).Errorf("failed to create limit order")
		return nil, ccxt.MapError(err)
	}

	log.WithField("order_id", order.Id).
		WithField("order_status", mapOrderStatus(order.Status).String()).
Debugf("limit order created")

	return &cegwv1.CreateLimitOrderResponse{
		Order: buildOrder(order, req.Price),
	}, nil
}

func (s *TradingService) GetBalance(ctx context.Context, req *cegwv1.GetBalanceRequest) (*cegwv1.GetBalanceResponse, error) {
	log := s.log.WithContext(ctx).
		WithField("operation", "GetBalance").
		WithField("exchange", req.Exchange.String())

	if req.Exchange == cegwv1.Exchange_EXCHANGE_UNSPECIFIED {
		log.Infof("invalid request: exchange unspecified")
		return nil, status.Error(codes.InvalidArgument, "exchange is required")
	}

	if req.Credentials == nil {
		log.Infof("invalid request: credentials missing")
		return nil, status.Error(codes.InvalidArgument, "credentials are required")
	}

	if req.Credentials.ApiKey == "" {
		log.Infof("invalid request: api_key missing")
		return nil, status.Error(codes.InvalidArgument, "api_key is required")
	}

	if req.Credentials.ApiSecret == "" {
		log.Infof("invalid request: api_secret missing")
		return nil, status.Error(codes.InvalidArgument, "api_secret is required")
	}

	log.Debugf("fetching balance")

	client, err := ccxt.NewClientForExchange(ctx, req.Exchange, req.Credentials)
	if err != nil {
		log.WithError(err).Errorf("failed to create CCXT client")
		return nil, err
	}

	exchange := ccxt.AsExchange(client)
	if exchange == nil {
		log.Errorf("exchange not supported")
		return nil, status.Error(codes.Unimplemented, "exchange not supported")
	}

	balances, err := exchange.FetchBalance()
	if err != nil {
		log.WithError(err).Errorf("failed to fetch balance")
		return nil, ccxt.MapError(err)
	}

	var result []*cegwv1.Balance

	// Balances has Free, Used, Total as map[string]float64
	// Collect all unique assets
	assets := make(map[string]bool)
	for asset := range balances.Free {
		assets[asset] = true
	}
	for asset := range balances.Used {
		assets[asset] = true
	}
	for asset := range balances.Total {
		assets[asset] = true
	}

	for asset := range assets {
		b := &cegwv1.Balance{
			Asset: asset,
			Free:  ccxt.Float64P(balances.Free[asset]),
			Used:  ccxt.Float64P(balances.Used[asset]),
			Total: ccxt.Float64P(balances.Total[asset]),
		}
		// If total not provided, calculate from free + used
		if b.Total == 0 && (b.Free > 0 || b.Used > 0) {
			b.Total = b.Free + b.Used
		}
		result = append(result, b)
	}

	log.WithField("asset_count", len(result)).Infof("balance fetched successfully")

	return &cegwv1.GetBalanceResponse{
		Balances: result,
	}, nil
}

func (s *TradingService) GetOrder(ctx context.Context, req *cegwv1.GetOrderRequest) (*cegwv1.GetOrderResponse, error) {
	log := s.log.WithContext(ctx).
		WithField("operation", "GetOrder").
		WithField("exchange", req.Exchange.String()).
		WithField("order_id", req.OrderId)

	if req.Exchange == cegwv1.Exchange_EXCHANGE_UNSPECIFIED {
		log.Infof("invalid request: exchange unspecified")
		return nil, status.Error(codes.InvalidArgument, "exchange is required")
	}

	if req.OrderId == "" {
		log.Infof("invalid request: order_id empty")
		return nil, status.Error(codes.InvalidArgument, "order_id is required")
	}

	if req.Credentials == nil {
		log.Infof("invalid request: credentials missing")
		return nil, status.Error(codes.InvalidArgument, "credentials are required")
	}

	if req.Credentials.ApiKey == "" {
		log.Infof("invalid request: api_key missing")
		return nil, status.Error(codes.InvalidArgument, "api_key is required")
	}

	if req.Credentials.ApiSecret == "" {
		log.Infof("invalid request: api_secret missing")
		return nil, status.Error(codes.InvalidArgument, "api_secret is required")
	}

	log.Debugf("fetching order")

	client, err := ccxt.NewClientForExchange(ctx, req.Exchange, req.Credentials)
	if err != nil {
		log.WithError(err).Errorf("failed to create CCXT client")
		return nil, err
	}

	exchange := ccxt.AsExchange(client)
	if exchange == nil {
		log.Errorf("exchange not supported")
		return nil, status.Error(codes.Unimplemented, "exchange not supported")
	}

	var order ccxtlib.Order
	if req.Symbol != "" {
		order, err = exchange.FetchOrder(req.OrderId, ccxtlib.WithFetchOrderSymbol(req.Symbol))
	} else {
		order, err = exchange.FetchOrder(req.OrderId)
	}
	if err != nil {
		log.WithError(err).Errorf("failed to fetch order")
		return nil, ccxt.MapError(err)
	}

	log.WithField("order_id", order.Id).
		WithField("order_status", mapOrderStatus(order.Status).String()).
		Infof("order fetched successfully")

	return &cegwv1.GetOrderResponse{
		Order: buildOrder(order, 0),
	}, nil
}

func (s *TradingService) CancelOrder(ctx context.Context, req *cegwv1.CancelOrderRequest) (*cegwv1.CancelOrderResponse, error) {
	log := s.log.WithContext(ctx).
		WithField("operation", "CancelOrder").
		WithField("exchange", req.Exchange.String()).
		WithField("order_id", req.OrderId).
		WithField("symbol", req.Symbol)

	if req.Exchange == cegwv1.Exchange_EXCHANGE_UNSPECIFIED {
		log.Infof("invalid request: exchange unspecified")
		return nil, status.Error(codes.InvalidArgument, "exchange is required")
	}

	if req.OrderId == "" {
		log.Infof("invalid request: order_id empty")
		return nil, status.Error(codes.InvalidArgument, "order_id is required")
	}

	if req.Symbol == "" {
		log.Infof("invalid request: symbol empty")
		return nil, status.Error(codes.InvalidArgument, "symbol is required")
	}

	if req.Credentials == nil {
		log.Infof("invalid request: credentials missing")
		return nil, status.Error(codes.InvalidArgument, "credentials are required")
	}

	if req.Credentials.ApiKey == "" {
		log.Infof("invalid request: api_key missing")
		return nil, status.Error(codes.InvalidArgument, "api_key is required")
	}

	if req.Credentials.ApiSecret == "" {
		log.Infof("invalid request: api_secret missing")
		return nil, status.Error(codes.InvalidArgument, "api_secret is required")
	}

	log.Debugf("cancelling order")

	client, err := ccxt.NewClientForExchange(ctx, req.Exchange, req.Credentials)
	if err != nil {
		log.WithError(err).Errorf("failed to create CCXT client")
		return nil, err
	}

	exchange := ccxt.AsExchange(client)
	if exchange == nil {
		log.Errorf("exchange not supported")
		return nil, status.Error(codes.Unimplemented, "exchange not supported")
	}

	_, err = exchange.CancelOrder(req.OrderId, ccxtlib.WithCancelOrderSymbol(req.Symbol))
	if err != nil {
		log.WithError(err).Errorf("failed to cancel order")
		return nil, ccxt.MapError(err)
	}

	log.Infof("order cancelled successfully")

	return &cegwv1.CancelOrderResponse{
		Success: true,
		Message: "order cancelled successfully",
	}, nil
}

func (s *TradingService) CancelAllOrders(ctx context.Context, req *cegwv1.CancelAllOrdersRequest) (*cegwv1.CancelAllOrdersResponse, error) {
	log := s.log.WithContext(ctx).
		WithField("operation", "CancelAllOrders").
		WithField("exchange", req.Exchange.String()).
		WithField("symbol", req.Symbol)

	if req.Exchange == cegwv1.Exchange_EXCHANGE_UNSPECIFIED {
		log.Infof("invalid request: exchange unspecified")
		return nil, status.Error(codes.InvalidArgument, "exchange is required")
	}

	if req.Credentials == nil {
		log.Infof("invalid request: credentials missing")
		return nil, status.Error(codes.InvalidArgument, "credentials are required")
	}

	if req.Credentials.ApiKey == "" {
		log.Infof("invalid request: api_key missing")
		return nil, status.Error(codes.InvalidArgument, "api_key is required")
	}

	if req.Credentials.ApiSecret == "" {
		log.Infof("invalid request: api_secret missing")
		return nil, status.Error(codes.InvalidArgument, "api_secret is required")
	}

	log.Debugf("cancelling all orders")

	client, err := ccxt.NewClientForExchange(ctx, req.Exchange, req.Credentials)
	if err != nil {
		log.WithError(err).Errorf("failed to create CCXT client")
		return nil, err
	}

	exchange := ccxt.AsExchange(client)
	if exchange == nil {
		log.Errorf("exchange not supported")
		return nil, status.Error(codes.Unimplemented, "exchange not supported")
	}

	var cancelledOrders []ccxtlib.Order
	if req.Symbol != "" {
		cancelledOrders, err = exchange.CancelAllOrders(ccxtlib.WithCancelAllOrdersSymbol(req.Symbol))
	} else {
		cancelledOrders, err = exchange.CancelAllOrders()
	}
	if err != nil {
		log.WithError(err).Errorf("failed to cancel all orders")
		return nil, ccxt.MapError(err)
	}

	count := len(cancelledOrders)
	if count > 2147483647 {
		count = 2147483647
	}
	cancelledCount := int32(count)
	log.WithField("cancelled_count", count).Infof("all orders cancelled successfully")

	return &cegwv1.CancelAllOrdersResponse{
		Success:        true,
		Message:        fmt.Sprintf("cancelled %d orders", count),
		CancelledCount: cancelledCount,
	}, nil
}

func (s *TradingService) ListOpenOrders(ctx context.Context, req *cegwv1.ListOpenOrdersRequest) (*cegwv1.ListOpenOrdersResponse, error) {
	log := s.log.WithContext(ctx).
		WithField("operation", "ListOpenOrders").
		WithField("exchange", req.Exchange.String()).
		WithField("symbol", req.Symbol)

	if req.Exchange == cegwv1.Exchange_EXCHANGE_UNSPECIFIED {
		log.Infof("invalid request: exchange unspecified")
		return nil, status.Error(codes.InvalidArgument, "exchange is required")
	}

	if req.Credentials == nil {
		log.Infof("invalid request: credentials missing")
		return nil, status.Error(codes.InvalidArgument, "credentials are required")
	}

	if req.Credentials.ApiKey == "" {
		log.Infof("invalid request: api_key missing")
		return nil, status.Error(codes.InvalidArgument, "api_key is required")
	}

	if req.Credentials.ApiSecret == "" {
		log.Infof("invalid request: api_secret missing")
		return nil, status.Error(codes.InvalidArgument, "api_secret is required")
	}

	log.Debugf("fetching open orders")

	client, err := ccxt.NewClientForExchange(ctx, req.Exchange, req.Credentials)
	if err != nil {
		log.WithError(err).Errorf("failed to create CCXT client")
		return nil, err
	}

	exchange := ccxt.AsExchange(client)
	if exchange == nil {
		log.Errorf("exchange not supported")
		return nil, status.Error(codes.Unimplemented, "exchange not supported")
	}

	var orders []ccxtlib.Order
	if req.Symbol != "" {
		orders, err = exchange.FetchOpenOrders(ccxtlib.WithFetchOpenOrdersSymbol(req.Symbol))
	} else {
		orders, err = exchange.FetchOpenOrders()
	}
	if err != nil {
		log.WithError(err).Errorf("failed to fetch open orders")
		return nil, ccxt.MapError(err)
	}

	var result []*cegwv1.Order
	for _, order := range orders {
		result = append(result, buildOrder(order, 0))
	}

	log.WithField("order_count", len(result)).Infof("open orders fetched successfully")

	return &cegwv1.ListOpenOrdersResponse{
		Orders: result,
	}, nil
}

func (s *TradingService) ListClosedOrders(ctx context.Context, req *cegwv1.ListClosedOrdersRequest) (*cegwv1.ListClosedOrdersResponse, error) {
	log := s.log.WithContext(ctx).
		WithField("operation", "ListClosedOrders").
		WithField("exchange", req.Exchange.String()).
		WithField("symbol", req.Symbol)

	if req.Exchange == cegwv1.Exchange_EXCHANGE_UNSPECIFIED {
		log.Infof("invalid request: exchange unspecified")
		return nil, status.Error(codes.InvalidArgument, "exchange is required")
	}

	if req.Credentials == nil {
		log.Infof("invalid request: credentials missing")
		return nil, status.Error(codes.InvalidArgument, "credentials are required")
	}

	if req.Credentials.ApiKey == "" {
		log.Infof("invalid request: api_key missing")
		return nil, status.Error(codes.InvalidArgument, "api_key is required")
	}

	if req.Credentials.ApiSecret == "" {
		log.Infof("invalid request: api_secret missing")
		return nil, status.Error(codes.InvalidArgument, "api_secret is required")
	}

	log.Debugf("fetching closed orders")

	client, err := ccxt.NewClientForExchange(ctx, req.Exchange, req.Credentials)
	if err != nil {
		log.WithError(err).Errorf("failed to create CCXT client")
		return nil, err
	}

	exchange := ccxt.AsExchange(client)
	if exchange == nil {
		log.Errorf("exchange not supported")
		return nil, status.Error(codes.Unimplemented, "exchange not supported")
	}

	var orders []ccxtlib.Order
	if req.Symbol != "" {
		orders, err = exchange.FetchClosedOrders(ccxtlib.WithFetchClosedOrdersSymbol(req.Symbol))
	} else {
		orders, err = exchange.FetchClosedOrders()
	}
	if err != nil {
		log.WithError(err).Errorf("failed to fetch closed orders")
		return nil, ccxt.MapError(err)
	}

	var result []*cegwv1.Order
	for _, order := range orders {
		result = append(result, buildOrder(order, 0))
	}

	log.WithField("order_count", len(result)).Infof("closed orders fetched successfully")

	return &cegwv1.ListClosedOrdersResponse{
		Orders: result,
	}, nil
}

func (s *TradingService) TestCredentials(ctx context.Context, req *cegwv1.TestCredentialsRequest) (*cegwv1.TestCredentialsResponse, error) {
	log := s.log.WithContext(ctx).
		WithField("operation", "TestCredentials").
		WithField("exchange", req.Exchange.String())

	if req.Exchange == cegwv1.Exchange_EXCHANGE_UNSPECIFIED {
		log.Infof("invalid request: exchange unspecified")
		return nil, status.Error(codes.InvalidArgument, "exchange is required")
	}

	if req.Credentials == nil {
		log.Infof("invalid request: credentials missing")
		return nil, status.Error(codes.InvalidArgument, "credentials are required")
	}

	if req.Credentials.ApiKey == "" {
		log.Infof("invalid request: api_key missing")
		return nil, status.Error(codes.InvalidArgument, "api_key is required")
	}

	if req.Credentials.ApiSecret == "" {
		log.Infof("invalid request: api_secret missing")
		return nil, status.Error(codes.InvalidArgument, "api_secret is required")
	}

	log.Debugf("testing credentials")

	client, err := ccxt.NewClientForExchange(ctx, req.Exchange, req.Credentials)
	if err != nil {
		log.WithError(err).Warnf("failed to create CCXT client during credential test")
		// Client creation failure is typically a configuration/credential issue
		return &cegwv1.TestCredentialsResponse{
			Valid:   false,
			Message: "failed to initialize exchange client - check credentials format",
		}, nil
	}

	exchange := ccxt.AsExchange(client)
	if exchange == nil {
		log.Errorf("exchange not supported")
		return nil, status.Error(codes.Unimplemented, "exchange not supported")
	}

	balances, err := exchange.FetchBalance()
	if err != nil {
		log.WithError(err).Warnf("failed to fetch balance during credential test")
		
		// Use existing error mapper to classify the error
		mappedErr := ccxt.MapError(err)
		if mappedErr != nil {
			st, ok := status.FromError(mappedErr)
			if ok {
				switch st.Code() {
				case codes.Unauthenticated:
					return &cegwv1.TestCredentialsResponse{
						Valid:   false,
						Message: "invalid credentials - authentication failed",
					}, nil
				case codes.Unavailable:
					return &cegwv1.TestCredentialsResponse{
						Valid:   false,
						Message: "exchange temporarily unavailable - please retry later",
					}, nil
				case codes.ResourceExhausted:
					return &cegwv1.TestCredentialsResponse{
						Valid:   false,
						Message: "rate limit exceeded - please retry later",
					}, nil
				case codes.PermissionDenied:
					return &cegwv1.TestCredentialsResponse{
						Valid:   false,
						Message: "insufficient permissions - API key lacks required permissions",
					}, nil
				default:
					return &cegwv1.TestCredentialsResponse{
						Valid:   false,
						Message: st.Message(),
					}, nil
				}
			}
		}
		
		// Fallback for unmapped errors
		return &cegwv1.TestCredentialsResponse{
			Valid:   false,
			Message: "credential test failed - " + err.Error(),
		}, nil
	}

	log.WithField("balance_count", len(balances.Balances)).Infof("credentials validated successfully")
	return &cegwv1.TestCredentialsResponse{
		Valid:   true,
		Message: "credentials are valid",
	}, nil
}
