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

func TestMarketDataService_GetCurrentPrice(t *testing.T) {
	cfg := &config.Config{
		LogLevel:    "error",
		Timezone:    time.UTC,
		SandboxMode: true,
	}

	svc := NewMarketDataService(cfg)
	ctx := context.Background()

	tests := []struct {
		name      string
		req       *cegwv1.GetCurrentPriceRequest
		wantErr   bool
		wantCode  codes.Code
		checkResp func(*cegwv1.GetCurrentPriceResponse) error
	}{
		{
			name: "invalid exchange",
			req: &cegwv1.GetCurrentPriceRequest{
				Exchange: cegwv1.Exchange_EXCHANGE_UNSPECIFIED,
				Symbol:   "BTC/USDT",
			},
			wantErr:  true,
			wantCode: codes.InvalidArgument,
		},
		{
			name: "empty symbol",
			req: &cegwv1.GetCurrentPriceRequest{
				Exchange: cegwv1.Exchange_EXCHANGE_TOKOCRYPTO,
				Symbol:   "",
			},
			wantErr:  true,
			wantCode: codes.InvalidArgument,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := svc.GetCurrentPrice(ctx, tt.req)

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

			if tt.checkResp != nil {
				if err := tt.checkResp(resp); err != nil {
					t.Errorf("Response check failed: %v", err)
				}
			}
		})
	}
}

func TestMarketDataService_ListMarkets(t *testing.T) {
	cfg := &config.Config{
		LogLevel:    "error",
		Timezone:    time.UTC,
		SandboxMode: true,
	}

	svc := NewMarketDataService(cfg)
	ctx := context.Background()

	tests := []struct {
		name     string
		req      *cegwv1.ListMarketsRequest
		wantErr  bool
		wantCode codes.Code
	}{
		{
			name: "invalid exchange",
			req: &cegwv1.ListMarketsRequest{
				Exchange: cegwv1.Exchange_EXCHANGE_UNSPECIFIED,
			},
			wantErr:  true,
			wantCode: codes.InvalidArgument,
		},
		{
			name: "valid exchange",
			req: &cegwv1.ListMarketsRequest{
				Exchange: cegwv1.Exchange_EXCHANGE_TOKOCRYPTO,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := svc.ListMarkets(ctx, tt.req)

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
				return
			}
		})
	}
}

func TestMarketDataService_SearchTicker(t *testing.T) {
	cfg := &config.Config{
		LogLevel:    "error",
		Timezone:    time.UTC,
		SandboxMode: true,
	}

	svc := NewMarketDataService(cfg)
	ctx := context.Background()

	tests := []struct {
		name     string
		req      *cegwv1.SearchTickerRequest
		wantErr  bool
		wantCode codes.Code
	}{
		{
			name: "invalid exchange",
			req: &cegwv1.SearchTickerRequest{
				Exchange: cegwv1.Exchange_EXCHANGE_UNSPECIFIED,
				Query:    "BTC",
			},
			wantErr:  true,
			wantCode: codes.InvalidArgument,
		},
		{
			name: "empty query",
			req: &cegwv1.SearchTickerRequest{
				Exchange: cegwv1.Exchange_EXCHANGE_TOKOCRYPTO,
				Query:    "",
			},
			wantErr:  true,
			wantCode: codes.InvalidArgument,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := svc.SearchTicker(ctx, tt.req)

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

func TestMarketDataService_GetQuotes(t *testing.T) {
	cfg := &config.Config{
		LogLevel:    "error",
		Timezone:    time.UTC,
		SandboxMode: true,
	}

	svc := NewMarketDataService(cfg)
	ctx := context.Background()

	tests := []struct {
		name     string
		req      *cegwv1.GetQuotesRequest
		wantErr  bool
		wantCode codes.Code
	}{
		{
			name: "invalid exchange",
			req: &cegwv1.GetQuotesRequest{
				Exchange: cegwv1.Exchange_EXCHANGE_UNSPECIFIED,
				Symbol:   "BTC/USDT",
				Interval: cegwv1.Interval_INTERVAL_1H,
			},
			wantErr:  true,
			wantCode: codes.InvalidArgument,
		},
		{
			name: "empty symbol",
			req: &cegwv1.GetQuotesRequest{
				Exchange: cegwv1.Exchange_EXCHANGE_TOKOCRYPTO,
				Symbol:   "",
				Interval: cegwv1.Interval_INTERVAL_1H,
			},
			wantErr:  true,
			wantCode: codes.InvalidArgument,
		},
		{
			name: "invalid interval",
			req: &cegwv1.GetQuotesRequest{
				Exchange: cegwv1.Exchange_EXCHANGE_TOKOCRYPTO,
				Symbol:   "BTC/USDT",
				Interval: cegwv1.Interval_INTERVAL_UNSPECIFIED,
			},
			wantErr:  true,
			wantCode: codes.InvalidArgument,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := svc.GetQuotes(ctx, tt.req)

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
