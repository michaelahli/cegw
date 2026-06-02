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

func TestTradingService_CreateMarketOrder(t *testing.T) {
	cfg := &config.Config{
		LogLevel:    "error",
		Timezone:    time.UTC,
		SandboxMode: true,
	}

	svc := NewTradingService(cfg)
	ctx := context.Background()

	tests := []struct {
		name     string
		req      *cegwv1.CreateMarketOrderRequest
		wantErr  bool
		wantCode codes.Code
	}{
		{
			name: "invalid exchange",
			req: &cegwv1.CreateMarketOrderRequest{
				Exchange: cegwv1.Exchange_EXCHANGE_UNSPECIFIED,
				Symbol:   "BTC/USDT",
				Side:     cegwv1.OrderSide_ORDER_SIDE_BUY,
				Quantity: 0.001,
				Credentials: &cegwv1.Credentials{
					ApiKey:    "test_key",
					ApiSecret: "test_secret",
					Sandbox:   true,
				},
			},
			wantErr:  true,
			wantCode: codes.InvalidArgument,
		},
		{
			name: "empty symbol",
			req: &cegwv1.CreateMarketOrderRequest{
				Exchange: cegwv1.Exchange_EXCHANGE_TOKOCRYPTO,
				Symbol:   "",
				Side:     cegwv1.OrderSide_ORDER_SIDE_BUY,
				Quantity: 0.001,
				Credentials: &cegwv1.Credentials{
					ApiKey:    "test_key",
					ApiSecret: "test_secret",
					Sandbox:   true,
				},
			},
			wantErr:  true,
			wantCode: codes.InvalidArgument,
		},
		{
			name: "invalid side",
			req: &cegwv1.CreateMarketOrderRequest{
				Exchange: cegwv1.Exchange_EXCHANGE_TOKOCRYPTO,
				Symbol:   "BTC/USDT",
				Side:     cegwv1.OrderSide_ORDER_SIDE_UNSPECIFIED,
				Quantity: 0.001,
				Credentials: &cegwv1.Credentials{
					ApiKey:    "test_key",
					ApiSecret: "test_secret",
					Sandbox:   true,
				},
			},
			wantErr:  true,
			wantCode: codes.InvalidArgument,
		},
		{
			name: "zero quantity",
			req: &cegwv1.CreateMarketOrderRequest{
				Exchange: cegwv1.Exchange_EXCHANGE_TOKOCRYPTO,
				Symbol:   "BTC/USDT",
				Side:     cegwv1.OrderSide_ORDER_SIDE_BUY,
				Quantity: 0,
				Credentials: &cegwv1.Credentials{
					ApiKey:    "test_key",
					ApiSecret: "test_secret",
					Sandbox:   true,
				},
			},
			wantErr:  true,
			wantCode: codes.InvalidArgument,
		},
		{
			name: "negative quantity",
			req: &cegwv1.CreateMarketOrderRequest{
				Exchange: cegwv1.Exchange_EXCHANGE_TOKOCRYPTO,
				Symbol:   "BTC/USDT",
				Side:     cegwv1.OrderSide_ORDER_SIDE_BUY,
				Quantity: -0.001,
				Credentials: &cegwv1.Credentials{
					ApiKey:    "test_key",
					ApiSecret: "test_secret",
					Sandbox:   true,
				},
			},
			wantErr:  true,
			wantCode: codes.InvalidArgument,
		},
		{
			name: "missing credentials",
			req: &cegwv1.CreateMarketOrderRequest{
				Exchange:    cegwv1.Exchange_EXCHANGE_TOKOCRYPTO,
				Symbol:      "BTC/USDT",
				Side:        cegwv1.OrderSide_ORDER_SIDE_BUY,
				Quantity:    0.001,
				Credentials: nil,
			},
			wantErr:  true,
			wantCode: codes.InvalidArgument,
		},
		{
			name: "empty api key",
			req: &cegwv1.CreateMarketOrderRequest{
				Exchange: cegwv1.Exchange_EXCHANGE_TOKOCRYPTO,
				Symbol:   "BTC/USDT",
				Side:     cegwv1.OrderSide_ORDER_SIDE_BUY,
				Quantity: 0.001,
				Credentials: &cegwv1.Credentials{
					ApiKey:    "",
					ApiSecret: "test_secret",
					Sandbox:   true,
				},
			},
			wantErr:  true,
			wantCode: codes.InvalidArgument,
		},
		{
			name: "empty api secret",
			req: &cegwv1.CreateMarketOrderRequest{
				Exchange: cegwv1.Exchange_EXCHANGE_TOKOCRYPTO,
				Symbol:   "BTC/USDT",
				Side:     cegwv1.OrderSide_ORDER_SIDE_BUY,
				Quantity: 0.001,
				Credentials: &cegwv1.Credentials{
					ApiKey:    "test_key",
					ApiSecret: "",
					Sandbox:   true,
				},
			},
			wantErr:  true,
			wantCode: codes.InvalidArgument,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := svc.CreateMarketOrder(ctx, tt.req)

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

func TestTradingService_TestCredentials(t *testing.T) {
	cfg := &config.Config{
		LogLevel:    "error",
		Timezone:    time.UTC,
		SandboxMode: true,
	}

	svc := NewTradingService(cfg)
	ctx := context.Background()

	tests := []struct {
		name     string
		req      *cegwv1.TestCredentialsRequest
		wantErr  bool
		wantCode codes.Code
	}{
		{
			name: "invalid exchange",
			req: &cegwv1.TestCredentialsRequest{
				Exchange: cegwv1.Exchange_EXCHANGE_UNSPECIFIED,
				Credentials: &cegwv1.Credentials{
					ApiKey:    "test_key",
					ApiSecret: "test_secret",
					Sandbox:   true,
				},
			},
			wantErr:  true,
			wantCode: codes.InvalidArgument,
		},
		{
			name: "missing credentials",
			req: &cegwv1.TestCredentialsRequest{
				Exchange:    cegwv1.Exchange_EXCHANGE_TOKOCRYPTO,
				Credentials: nil,
			},
			wantErr:  true,
			wantCode: codes.InvalidArgument,
		},
		{
			name: "empty api key",
			req: &cegwv1.TestCredentialsRequest{
				Exchange: cegwv1.Exchange_EXCHANGE_TOKOCRYPTO,
				Credentials: &cegwv1.Credentials{
					ApiKey:    "",
					ApiSecret: "test_secret",
					Sandbox:   true,
				},
			},
			wantErr:  true,
			wantCode: codes.InvalidArgument,
		},
		{
			name: "empty api secret",
			req: &cegwv1.TestCredentialsRequest{
				Exchange: cegwv1.Exchange_EXCHANGE_TOKOCRYPTO,
				Credentials: &cegwv1.Credentials{
					ApiKey:    "test_key",
					ApiSecret: "",
					Sandbox:   true,
				},
			},
			wantErr:  true,
			wantCode: codes.InvalidArgument,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := svc.TestCredentials(ctx, tt.req)

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
