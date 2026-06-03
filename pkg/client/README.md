# CEGW Go Client

Simple Go client library for integrating with CEGW API.

## Installation

```bash
go get github.com/michaelahli/cegw
```

## Quick Start

```go
package main

import (
    "context"
    "log"
    
    cegwv1 "github.com/michaelahli/cegw/gen/cegw/v1"
    "github.com/michaelahli/cegw/pkg/client"
)

func main() {
    ctx := context.Background()
    
    // Connect to CEGW
    c, err := client.New(ctx, client.Config{
        Address: "localhost:50051",
    })
    if err != nil {
        log.Fatal(err)
    }
    defer c.Close()
    
    // Get current price
    resp, err := c.MarketDataService.GetCurrentPrice(ctx, &cegwv1.GetCurrentPriceRequest{
        Exchange: cegwv1.Exchange_EXCHANGE_TOKOCRYPTO,
        Symbol:   "BTC/USDT",
    })
    if err != nil {
        log.Fatal(err)
    }
    
    log.Printf("Price: $%.2f", resp.Price)
}
```

## Services Available

- **MarketDataService**: GetCurrentPrice, GetQuotes, ListMarkets, SearchTicker
- **TradingService**: CreateMarketOrder, TestCredentials
- **MonitoringService**: CheckPriceAlerts

## Examples

See `pkg/client/example/main.go` for complete usage examples.
