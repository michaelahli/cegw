package service

import (
	"context"

	ccxtlib "github.com/ccxt/ccxt/go/v4"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	cegwv1 "github.com/michaelahli/cegw/gen/cegw/v1"
	"github.com/michaelahli/cegw/internal/ccxt"
	"github.com/michaelahli/cegw/internal/config"
)

type MonitoringService struct {
	cegwv1.UnimplementedMonitoringServiceServer
	cfg *config.Config
}

func NewMonitoringService(cfg *config.Config) *MonitoringService {
	return &MonitoringService{
		cfg: cfg,
	}
}

func (s *MonitoringService) CheckPriceAlerts(ctx context.Context, req *cegwv1.CheckPriceAlertsRequest) (*cegwv1.CheckPriceAlertsResponse, error) {
	if req.Exchange == cegwv1.Exchange_EXCHANGE_UNSPECIFIED {
		return nil, status.Error(codes.InvalidArgument, "exchange is required")
	}

	if len(req.Alerts) == 0 {
		return nil, status.Error(codes.InvalidArgument, "alerts cannot be empty")
	}

	for _, alert := range req.Alerts {
		if alert.TargetPrice <= 0 {
			return nil, status.Error(codes.InvalidArgument, "target_price must be greater than 0")
		}
	}

	client, err := ccxt.NewClientForExchange(ctx, req.Exchange, nil)
	if err != nil {
		return nil, err
	}

	tokocrypto, ok := client.(*ccxtlib.Tokocrypto)
	if !ok {
		return nil, status.Error(codes.Unimplemented, "exchange not supported")
	}

	priceCache := make(map[string]float64)
	updatedAlerts := make([]*cegwv1.PriceAlert, 0, len(req.Alerts))

	for _, alert := range req.Alerts {
		price, exists := priceCache[alert.Symbol]
		if !exists {
			ticker, err := tokocrypto.FetchTicker(alert.Symbol)
			if err != nil {
				updatedAlerts = append(updatedAlerts, alert)
				continue
			}
			price = ccxt.Float64P(ticker.Close)
			priceCache[alert.Symbol] = price
		}

		updatedAlert := &cegwv1.PriceAlert{
			Id:          alert.Id,
			Symbol:      alert.Symbol,
			TargetPrice: alert.TargetPrice,
			Operator:    alert.Operator,
			Status:      cegwv1.AlertStatus_ALERT_STATUS_PENDING,
		}

		switch alert.Operator {
		case cegwv1.ComparisonOperator_COMPARISON_OPERATOR_GREATER_THAN:
			if price >= alert.TargetPrice {
				updatedAlert.Status = cegwv1.AlertStatus_ALERT_STATUS_TRIGGERED
			}
		case cegwv1.ComparisonOperator_COMPARISON_OPERATOR_LESS_THAN:
			if price <= alert.TargetPrice {
				updatedAlert.Status = cegwv1.AlertStatus_ALERT_STATUS_TRIGGERED
			}
		}

		updatedAlerts = append(updatedAlerts, updatedAlert)
	}

	return &cegwv1.CheckPriceAlertsResponse{
		Alerts:    updatedAlerts,
		CheckedAt: timestamppb.Now(),
	}, nil
}
