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

    // Stream current price updates
    stream, err := c.MarketDataService.StreamCurrentPrice(ctx, &cegwv1.GetCurrentPriceRequest{
        Exchange: cegwv1.Exchange_EXCHANGE_TOKOCRYPTO,
        Symbol:   "BTC/USDT",
    })
    if err != nil {
        log.Fatal(err)
    }

    for {
        update, err := stream.Recv()
        if err != nil {
            log.Fatal(err)
        }
        log.Printf("Price update: $%.2f", update.Price)
    }
}
```

## Services Available

- **MarketDataService**: GetCurrentPrice, StreamCurrentPrice, GetQuotes, ListMarkets, SearchTicker
- **TradingService**: CreateMarketOrder, TestCredentials
- **MonitoringService**: CheckPriceAlerts

## WebSocket Clients

Standard WebSocket clients can consume latest price updates from the HTTP server without using this Go client package. A browser example is available at `pkg/client/example/websocket.html` and connects to Tokocrypto `BTC/USDT` by default.

```javascript
const ws = new WebSocket("ws://localhost:8080/v1/ws/market/price?exchange=1&symbol=BTC/USDT");
ws.onmessage = (event) => console.log(JSON.parse(event.data));
```

## Examples

See `pkg/client/example/main.go` for gRPC usage and `pkg/client/example/websocket.html` for a browser WebSocket example.
