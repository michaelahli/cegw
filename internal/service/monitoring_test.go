package service

import (
	"context"
	"testing"
	"time"

	cegwv1 "github.com/michaelahli/cegw/gen/cegw/v1"
	"github.com/michaelahli/cegw/internal/config"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestMonitoringService_CheckPriceAlerts(t *testing.T) {
	cfg := &config.Config{
		LogLevel:    "error",
		Timezone:    time.UTC,
		SandboxMode: true,
	}

	svc := NewMonitoringService(cfg)
	ctx := context.Background()

	tests := []struct {
		name     string
		req      *cegwv1.CheckPriceAlertsRequest
		wantErr  bool
		wantCode codes.Code
	}{
		{
			name: "invalid exchange",
			req: &cegwv1.CheckPriceAlertsRequest{
				Exchange: cegwv1.Exchange_EXCHANGE_UNSPECIFIED,
			Alerts: []*cegwv1.PriceAlert{
				{
					Symbol:      "BTC/USDT",
					TargetPrice: 50000,
					Operator:    cegwv1.ComparisonOperator_COMPARISON_OPERATOR_GREATER_THAN,
				},
			},
			},
			wantErr:  true,
			wantCode: codes.InvalidArgument,
		},
		{
			name: "empty alerts",
			req: &cegwv1.CheckPriceAlertsRequest{
				Exchange: cegwv1.Exchange_EXCHANGE_TOKOCRYPTO,
				Alerts:   []*cegwv1.PriceAlert{},
			},
			wantErr:  true,
			wantCode: codes.InvalidArgument,
		},
		{
			name: "nil alerts",
			req: &cegwv1.CheckPriceAlertsRequest{
				Exchange: cegwv1.Exchange_EXCHANGE_TOKOCRYPTO,
				Alerts:   nil,
			},
			wantErr:  true,
			wantCode: codes.InvalidArgument,
		},
		{
			name: "alert with empty symbol",
			req: &cegwv1.CheckPriceAlertsRequest{
				Exchange: cegwv1.Exchange_EXCHANGE_TOKOCRYPTO,
			Alerts: []*cegwv1.PriceAlert{
				{
					Symbol:      "",
					TargetPrice: 50000,
					Operator:    cegwv1.ComparisonOperator_COMPARISON_OPERATOR_GREATER_THAN,
				},
			},
			},
			wantErr:  true,
			wantCode: codes.InvalidArgument,
		},
		{
			name: "alert with zero target price",
			req: &cegwv1.CheckPriceAlertsRequest{
				Exchange: cegwv1.Exchange_EXCHANGE_TOKOCRYPTO,
			Alerts: []*cegwv1.PriceAlert{
				{
					Symbol:      "BTC/USDT",
					TargetPrice: 0,
					Operator:    cegwv1.ComparisonOperator_COMPARISON_OPERATOR_GREATER_THAN,
				},
			},
			},
			wantErr:  true,
			wantCode: codes.InvalidArgument,
		},
		{
			name: "alert with negative target price",
			req: &cegwv1.CheckPriceAlertsRequest{
				Exchange: cegwv1.Exchange_EXCHANGE_TOKOCRYPTO,
			Alerts: []*cegwv1.PriceAlert{
				{
					Symbol:      "BTC/USDT",
					TargetPrice: -1000,
					Operator:    cegwv1.ComparisonOperator_COMPARISON_OPERATOR_GREATER_THAN,
				},
			},
			},
			wantErr:  true,
			wantCode: codes.InvalidArgument,
		},
		{
			name: "alert with unspecified condition",
			req: &cegwv1.CheckPriceAlertsRequest{
				Exchange: cegwv1.Exchange_EXCHANGE_TOKOCRYPTO,
			Alerts: []*cegwv1.PriceAlert{
				{
					Symbol:      "BTC/USDT",
					TargetPrice: 50000,
					Operator:    cegwv1.ComparisonOperator_COMPARISON_OPERATOR_UNSPECIFIED,
				},
			},
			},
			wantErr:  true,
			wantCode: codes.InvalidArgument,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := svc.CheckPriceAlerts(ctx, tt.req)

			if tt.wantErr {
				if err == nil {
					t.Errorf("Expected error but got nil")
					return
				}
				st, ok := status.FromError(err)
				if !ok {
					t.Errorf("Error is not a status error")
					return
				}
				if st.Code() != tt.wantCode {
					t.Errorf("Expected code %v, got %v", tt.wantCode, st.Code())
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if resp == nil {
				t.Errorf("Response is nil")
			}
		})
	}
}
