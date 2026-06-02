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
	"github.com/michaelahli/cegw/internal/logger"
)

type MonitoringService struct {
	cegwv1.UnimplementedMonitoringServiceServer
	cfg *config.Config
	log *logger.Logger
}

func NewMonitoringService(cfg *config.Config, log *logger.Logger) *MonitoringService {
	return &MonitoringService{
		cfg: cfg,
		log: log,
	}
}

func (s *MonitoringService) CheckPriceAlerts(ctx context.Context, req *cegwv1.CheckPriceAlertsRequest) (*cegwv1.CheckPriceAlertsResponse, error) {
	log := s.log.WithContext(ctx).
		WithField("operation", "CheckPriceAlerts").
		WithField("exchange", req.Exchange.String())

	if req.Exchange == cegwv1.Exchange_EXCHANGE_UNSPECIFIED {
		log.Warnf("invalid request: exchange unspecified")
		return nil, status.Error(codes.InvalidArgument, "exchange is required")
	}

	if len(req.Alerts) == 0 {
		log.Warnf("invalid request: alerts empty")
		return nil, status.Error(codes.InvalidArgument, "alerts cannot be empty")
	}

	log = log.WithField("alert_count", len(req.Alerts))
	log.Debugf("validating alerts")

	for _, alert := range req.Alerts {
		if alert.Symbol == "" {
			log.WithField("alert_id", alert.Id).Warnf("invalid alert: symbol empty")
			return nil, status.Error(codes.InvalidArgument, "alert symbol is required")
		}
		if alert.TargetPrice <= 0 {
			log.WithField("alert_id", alert.Id).
				WithField("target_price", alert.TargetPrice).
				Warnf("invalid alert: target_price not positive")
			return nil, status.Error(codes.InvalidArgument, "target_price must be greater than 0")
		}
		if alert.Operator == cegwv1.ComparisonOperator_COMPARISON_OPERATOR_UNSPECIFIED {
			log.WithField("alert_id", alert.Id).Warnf("invalid alert: operator unspecified")
			return nil, status.Error(codes.InvalidArgument, "alert operator is required")
		}
	}

	log.Debugf("creating CCXT client for price checks")
	client, err := ccxt.NewClientForExchange(ctx, req.Exchange, nil)
	if err != nil {
		log.WithError(err).Errorf("failed to create CCXT client")
		return nil, err
	}

	tokocrypto, ok := client.(*ccxtlib.Tokocrypto)
	if !ok {
		log.Warnf("exchange not supported")
		return nil, status.Error(codes.Unimplemented, "exchange not supported")
	}

	priceCache := make(map[string]float64)
	updatedAlerts := make([]*cegwv1.PriceAlert, 0, len(req.Alerts))
	triggeredCount := 0

	for i, alert := range req.Alerts {
		alertLog := log.WithField("alert_id", alert.Id).
			WithField("alert_index", i+1).
			WithField("symbol", alert.Symbol).
			WithField("target_price", alert.TargetPrice).
			WithField("operator", alert.Operator.String())

		price, exists := priceCache[alert.Symbol]
		if !exists {
			alertLog.Debugf("fetching ticker price")
			ticker, err := tokocrypto.FetchTicker(alert.Symbol)
			if err != nil {
				alertLog.WithError(err).Warnf("failed to fetch ticker, keeping alert pending")
				updatedAlerts = append(updatedAlerts, alert)
				continue
			}
			price = ccxt.Float64P(ticker.Close)
			priceCache[alert.Symbol] = price
			alertLog = alertLog.WithField("current_price", price)
		} else {
			alertLog = alertLog.WithField("current_price", price)
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
				triggeredCount++
				alertLog.Infof("alert triggered: price >= target")
			} else {
				alertLog.Debugf("alert pending: price < target")
			}
		case cegwv1.ComparisonOperator_COMPARISON_OPERATOR_LESS_THAN:
			if price <= alert.TargetPrice {
				updatedAlert.Status = cegwv1.AlertStatus_ALERT_STATUS_TRIGGERED
				triggeredCount++
				alertLog.Infof("alert triggered: price <= target")
			} else {
				alertLog.Debugf("alert pending: price > target")
			}
		}

		updatedAlerts = append(updatedAlerts, updatedAlert)
	}

	log.WithField("triggered_count", triggeredCount).
		WithField("pending_count", len(req.Alerts)-triggeredCount).
		Infof("price alerts checked successfully")

	return &cegwv1.CheckPriceAlertsResponse{
		Alerts:    updatedAlerts,
		CheckedAt: timestamppb.Now(),
	}, nil
}
