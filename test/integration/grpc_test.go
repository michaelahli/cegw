package integration

import (
	"context"
	"testing"
	"time"

	cegwv1 "github.com/michaelahli/cegw/gen/cegw/v1"
	"github.com/michaelahli/cegw/internal/server"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestGRPC_MarketDataService(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ts := server.NewTestServer(t)
	defer ts.Close()

	client := ts.NewMarketDataClient()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	t.Run("ListMarkets", func(t *testing.T) {
		t.Skip("Skipping test that requires real exchange API")
		req := &cegwv1.ListMarketsRequest{
			Exchange: cegwv1.Exchange_EXCHANGE_TOKOCRYPTO,
		}

		resp, err := client.ListMarkets(ctx, req)
		if err != nil {
			t.Fatalf("ListMarkets failed: %v", err)
		}

		if resp == nil {
			t.Fatal("Response is nil")
		}

		if len(resp.Markets) == 0 {
			t.Error("Expected markets, got empty list")
		}

		for _, market := range resp.Markets {
			if market.Symbol == "" {
				t.Error("Market symbol is empty")
			}
		}
	})

	t.Run("GetCurrentPrice_InvalidExchange", func(t *testing.T) {
		req := &cegwv1.GetCurrentPriceRequest{
			Exchange: cegwv1.Exchange_EXCHANGE_UNSPECIFIED,
			Symbol:   "BTC/USDT",
		}

		_, err := client.GetCurrentPrice(ctx, req)
		if err == nil {
			t.Fatal("Expected error for invalid exchange")
		}

		st, ok := status.FromError(err)
		if !ok {
			t.Fatal("Error is not a status error")
		}

		if st.Code() != codes.InvalidArgument {
			t.Errorf("Expected InvalidArgument, got %v", st.Code())
		}
	})

	t.Run("GetCurrentPrice_EmptySymbol", func(t *testing.T) {
		req := &cegwv1.GetCurrentPriceRequest{
			Exchange: cegwv1.Exchange_EXCHANGE_TOKOCRYPTO,
			Symbol:   "",
		}

		_, err := client.GetCurrentPrice(ctx, req)
		if err == nil {
			t.Fatal("Expected error for empty symbol")
		}

		st, ok := status.FromError(err)
		if !ok {
			t.Fatal("Error is not a status error")
		}

		if st.Code() != codes.InvalidArgument {
			t.Errorf("Expected InvalidArgument, got %v", st.Code())
		}
	})

	t.Run("SearchTicker_InvalidExchange", func(t *testing.T) {
		req := &cegwv1.SearchTickerRequest{
			Exchange: cegwv1.Exchange_EXCHANGE_UNSPECIFIED,
			Query:    "BTC",
		}

		_, err := client.SearchTicker(ctx, req)
		if err == nil {
			t.Fatal("Expected error for invalid exchange")
		}

		st, ok := status.FromError(err)
		if !ok {
			t.Fatal("Error is not a status error")
		}

		if st.Code() != codes.InvalidArgument {
			t.Errorf("Expected InvalidArgument, got %v", st.Code())
		}
	})

	t.Run("GetQuotes_InvalidExchange", func(t *testing.T) {
		req := &cegwv1.GetQuotesRequest{
			Exchange: cegwv1.Exchange_EXCHANGE_UNSPECIFIED,
			Symbol:   "BTC/USDT",
			Interval: cegwv1.Interval_INTERVAL_1H,
		}

		_, err := client.GetQuotes(ctx, req)
		if err == nil {
			t.Fatal("Expected error for invalid exchange")
		}

		st, ok := status.FromError(err)
		if !ok {
			t.Fatal("Error is not a status error")
		}

		if st.Code() != codes.InvalidArgument {
			t.Errorf("Expected InvalidArgument, got %v", st.Code())
		}
	})

	t.Run("GetQuotes_InvalidInterval", func(t *testing.T) {
		req := &cegwv1.GetQuotesRequest{
			Exchange: cegwv1.Exchange_EXCHANGE_TOKOCRYPTO,
			Symbol:   "BTC/USDT",
			Interval: cegwv1.Interval_INTERVAL_UNSPECIFIED,
		}

		_, err := client.GetQuotes(ctx, req)
		if err == nil {
			t.Fatal("Expected error for invalid interval")
		}

		st, ok := status.FromError(err)
		if !ok {
			t.Fatal("Error is not a status error")
		}

		if st.Code() != codes.InvalidArgument {
			t.Errorf("Expected InvalidArgument, got %v", st.Code())
		}
	})
}

func TestGRPC_TradingService(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ts := server.NewTestServer(t)
	defer ts.Close()

	client := ts.NewTradingClient()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	t.Run("CreateMarketOrder_InvalidExchange", func(t *testing.T) {
		req := &cegwv1.CreateMarketOrderRequest{
			Exchange: cegwv1.Exchange_EXCHANGE_UNSPECIFIED,
			Symbol:   "BTC/USDT",
			Side:     cegwv1.OrderSide_ORDER_SIDE_BUY,
			Quantity: 0.001,
			Credentials: &cegwv1.Credentials{
				ApiKey:    "test_key",
				ApiSecret: "test_secret",
				Sandbox:   true,
			},
		}

		_, err := client.CreateMarketOrder(ctx, req)
		if err == nil {
			t.Fatal("Expected error for invalid exchange")
		}

		st, ok := status.FromError(err)
		if !ok {
			t.Fatal("Error is not a status error")
		}

		if st.Code() != codes.InvalidArgument {
			t.Errorf("Expected InvalidArgument, got %v", st.Code())
		}
	})

	t.Run("CreateMarketOrder_MissingCredentials", func(t *testing.T) {
		req := &cegwv1.CreateMarketOrderRequest{
			Exchange:    cegwv1.Exchange_EXCHANGE_TOKOCRYPTO,
			Symbol:      "BTC/USDT",
			Side:        cegwv1.OrderSide_ORDER_SIDE_BUY,
			Quantity:    0.001,
			Credentials: nil,
		}

		_, err := client.CreateMarketOrder(ctx, req)
		if err == nil {
			t.Fatal("Expected error for missing credentials")
		}

		st, ok := status.FromError(err)
		if !ok {
			t.Fatal("Error is not a status error")
		}

		if st.Code() != codes.InvalidArgument {
			t.Errorf("Expected InvalidArgument, got %v", st.Code())
		}
	})

	t.Run("CreateMarketOrder_ZeroQuantity", func(t *testing.T) {
		req := &cegwv1.CreateMarketOrderRequest{
			Exchange: cegwv1.Exchange_EXCHANGE_TOKOCRYPTO,
			Symbol:   "BTC/USDT",
			Side:     cegwv1.OrderSide_ORDER_SIDE_BUY,
			Quantity: 0,
			Credentials: &cegwv1.Credentials{
				ApiKey:    "test_key",
				ApiSecret: "test_secret",
				Sandbox:   true,
			},
		}

		_, err := client.CreateMarketOrder(ctx, req)
		if err == nil {
			t.Fatal("Expected error for zero quantity")
		}

		st, ok := status.FromError(err)
		if !ok {
			t.Fatal("Error is not a status error")
		}

		if st.Code() != codes.InvalidArgument {
			t.Errorf("Expected InvalidArgument, got %v", st.Code())
		}
	})

	t.Run("TestCredentials_InvalidExchange", func(t *testing.T) {
		req := &cegwv1.TestCredentialsRequest{
			Exchange: cegwv1.Exchange_EXCHANGE_UNSPECIFIED,
			Credentials: &cegwv1.Credentials{
				ApiKey:    "test_key",
				ApiSecret: "test_secret",
				Sandbox:   true,
			},
		}

		_, err := client.TestCredentials(ctx, req)
		if err == nil {
			t.Fatal("Expected error for invalid exchange")
		}

		st, ok := status.FromError(err)
		if !ok {
			t.Fatal("Error is not a status error")
		}

		if st.Code() != codes.InvalidArgument {
			t.Errorf("Expected InvalidArgument, got %v", st.Code())
		}
	})

	t.Run("TestCredentials_MissingCredentials", func(t *testing.T) {
		req := &cegwv1.TestCredentialsRequest{
			Exchange:    cegwv1.Exchange_EXCHANGE_TOKOCRYPTO,
			Credentials: nil,
		}

		_, err := client.TestCredentials(ctx, req)
		if err == nil {
			t.Fatal("Expected error for missing credentials")
		}

		st, ok := status.FromError(err)
		if !ok {
			t.Fatal("Error is not a status error")
		}

		if st.Code() != codes.InvalidArgument {
			t.Errorf("Expected InvalidArgument, got %v", st.Code())
		}
	})
}

func TestGRPC_MonitoringService(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ts := server.NewTestServer(t)
	defer ts.Close()

	client := ts.NewMonitoringClient()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	t.Run("CheckPriceAlerts_InvalidExchange", func(t *testing.T) {
		req := &cegwv1.CheckPriceAlertsRequest{
			Exchange: cegwv1.Exchange_EXCHANGE_UNSPECIFIED,
			Alerts: []*cegwv1.PriceAlert{
				{
					Symbol:      "BTC/USDT",
					TargetPrice: 50000,
					Operator:    cegwv1.ComparisonOperator_COMPARISON_OPERATOR_GREATER_THAN,
				},
			},
		}

		_, err := client.CheckPriceAlerts(ctx, req)
		if err == nil {
			t.Fatal("Expected error for invalid exchange")
		}

		st, ok := status.FromError(err)
		if !ok {
			t.Fatal("Error is not a status error")
		}

		if st.Code() != codes.InvalidArgument {
			t.Errorf("Expected InvalidArgument, got %v", st.Code())
		}
	})

	t.Run("CheckPriceAlerts_EmptyAlerts", func(t *testing.T) {
		req := &cegwv1.CheckPriceAlertsRequest{
			Exchange: cegwv1.Exchange_EXCHANGE_TOKOCRYPTO,
			Alerts:   []*cegwv1.PriceAlert{},
		}

		_, err := client.CheckPriceAlerts(ctx, req)
		if err == nil {
			t.Fatal("Expected error for empty alerts")
		}

		st, ok := status.FromError(err)
		if !ok {
			t.Fatal("Error is not a status error")
		}

		if st.Code() != codes.InvalidArgument {
			t.Errorf("Expected InvalidArgument, got %v", st.Code())
		}
	})

	t.Run("CheckPriceAlerts_InvalidTargetPrice", func(t *testing.T) {
		req := &cegwv1.CheckPriceAlertsRequest{
			Exchange: cegwv1.Exchange_EXCHANGE_TOKOCRYPTO,
			Alerts: []*cegwv1.PriceAlert{
				{
					Symbol:      "BTC/USDT",
					TargetPrice: 0,
					Operator:    cegwv1.ComparisonOperator_COMPARISON_OPERATOR_GREATER_THAN,
				},
			},
		}

		_, err := client.CheckPriceAlerts(ctx, req)
		if err == nil {
			t.Fatal("Expected error for invalid target price")
		}

		st, ok := status.FromError(err)
		if !ok {
			t.Fatal("Error is not a status error")
		}

		if st.Code() != codes.InvalidArgument {
			t.Errorf("Expected InvalidArgument, got %v", st.Code())
		}
	})
}

func TestGRPC_ConcurrentRequests(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	t.Skip("Skipping test that requires real exchange API")

	ts := server.NewTestServer(t)
	defer ts.Close()

	client := ts.NewMarketDataClient()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	const numRequests = 10

	errChan := make(chan error, numRequests)

	for i := 0; i < numRequests; i++ {
		go func() {
			req := &cegwv1.ListMarketsRequest{
				Exchange: cegwv1.Exchange_EXCHANGE_TOKOCRYPTO,
			}
			_, err := client.ListMarkets(ctx, req)
			errChan <- err
		}()
	}

	for i := 0; i < numRequests; i++ {
		if err := <-errChan; err != nil {
			t.Errorf("Concurrent request %d failed: %v", i, err)
		}
	}
}
