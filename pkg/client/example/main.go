package main

import (
	"context"
	"fmt"
	"log"

	"github.com/michaelahli/cegw/gen/cegw/v1"
	"github.com/michaelahli/cegw/pkg/client"
)

func main() {
	ctx := context.Background()

	// Create client
	c, err := client.New(ctx, client.Config{
		Address: "localhost:50051",
	})
	if err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}
	defer func() {
		if err := c.Close(); err != nil {
			log.Printf("Failed to close client: %v", err)
		}
	}()

	// Example 1: Get current price
	priceResp, err := c.MarketDataService.GetCurrentPrice(ctx, &cegwv1.GetCurrentPriceRequest{
		Exchange: cegwv1.Exchange_EXCHANGE_TOKOCRYPTO,
		Symbol:   "BTC/USDT",
	})
	if err != nil {
		log.Fatalf("GetCurrentPrice failed: %v", err)
	}
	fmt.Printf("BTC/USDT: $%.2f\n", priceResp.Price)

	// Example 2: List markets
	marketsResp, err := c.MarketDataService.ListMarkets(ctx, &cegwv1.ListMarketsRequest{
		Exchange: cegwv1.Exchange_EXCHANGE_TOKOCRYPTO,
	})
	if err != nil {
		log.Fatalf("ListMarkets failed: %v", err)
	}
	fmt.Printf("Found %d markets\n", marketsResp.Count)

	// Example 3: Create order (with credentials)
	orderResp, err := c.TradingService.CreateMarketOrder(ctx, &cegwv1.CreateMarketOrderRequest{
		Exchange: cegwv1.Exchange_EXCHANGE_TOKOCRYPTO,
		Symbol:   "BTC/USDT",
		Side:     cegwv1.OrderSide_ORDER_SIDE_BUY,
		Quantity: 0.001,
		Credentials: &cegwv1.Credentials{
			ApiKey:    "YOUR_API_KEY",
			ApiSecret: "YOUR_API_SECRET",
			Sandbox:   true,
		},
	})
	if err != nil {
		log.Fatalf("CreateMarketOrder failed: %v", err)
	}
	fmt.Printf("Order created: %s\n", orderResp.Order.OrderId)
}
