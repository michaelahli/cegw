# CEGW

CEGW is a lightweight gateway for cryptocurrency exchange data and trading.
It provides a simple HTTP/JSON API and gRPC endpoint so your apps can access market prices, symbols, and trading functionality from a unified service.

## What it does

- Fetch market data for supported exchanges
- Return current prices, market lists, and trading pairs
- Place market buy/sell orders
- Support HTTP/JSON, gRPC, and native WebSocket clients
- Run in Docker for easy local or cloud deployment

## Run with Docker

```bash
docker run --rm -p 8080:8080 -p 50051:50051 ghcr.io/michaelahli/cegw:latest
```

Then open:

- `http://localhost:8080` for the HTTP API gateway
- `http://localhost:8080/docs` for API docs

## Deploy with Helm

```bash
# Add Helm repository
helm repo add cegw https://michaelahli.github.io/cegw
helm repo update

# Install chart
helm install cegw cegw/cegw
```

See [charts/cegw/README.md](charts/cegw/README.md) for more options.

## API Documentation

View the interactive API documentation:

- **Local**: `http://localhost:8080/docs` (when running the application)
- **Online**: [https://michaelahli.github.io/cegw/docs/](https://michaelahli.github.io/cegw/docs/)

## Quick usage

### Get current price

```bash
curl http://localhost:8080/v1/market/price/1/BTC/USDT
```

### List markets

```bash
curl http://localhost:8080/v1/market/list?exchange=1
```

### Stream latest prices over WebSocket

Use the native WebSocket endpoint when you want JSON updates from a browser or a standard WebSocket client.

```javascript
const ws = new WebSocket(
  "ws://localhost:8080/v1/ws/market/price?exchange=2&symbol=BTC/USDT",
);

ws.onmessage = (event) => {
  console.log(JSON.parse(event.data));
};
```

Messages are sent as JSON:

```json
{
  "symbol": "BTC/USDT",
  "price": 68420.12,
  "timestamp": "2026-06-07T10:15:30Z"
}
```

### Stream latest prices over gRPC

Use `MarketDataService.StreamCurrentPrice` for server-streaming gRPC clients. It returns a stream of `GetCurrentPriceResponse` messages and uses exchange WebSocket feeds through CCXT.

```go
stream, err := c.MarketDataService.StreamCurrentPrice(ctx, &cegwv1.GetCurrentPriceRequest{
    Exchange: cegwv1.Exchange_EXCHANGE_BINANCE,
    Symbol:   "BTC/USDT",
})
if err != nil {
    log.Fatal(err)
}

for {
    price, err := stream.Recv()
    if err != nil {
        log.Fatal(err)
    }
    log.Printf("%s price: %.8f", price.Symbol, price.Price)
}
```

### Create a market order

```bash
curl -X POST http://localhost:8080/v1/trading/order \
  -H "Content-Type: application/json" \
  -d '{
    "exchange": 1,
    "symbol": "BTC/USDT",
    "side": 1,
    "quantity": 0.001,
    "credentials": {
      "api_key": "YOUR_API_KEY",
      "api_secret": "YOUR_API_SECRET",
      "sandbox": true
    }
  }'
```

## Streaming Market Data

CEGW supports two streaming paths for latest price updates:

- gRPC server streaming: `MarketDataService.StreamCurrentPrice`
- Native WebSocket JSON: `GET /v1/ws/market/price?exchange={id}&symbol={symbol}`

The WebSocket endpoint is included in `docs/openapi.json` through a manual OpenAPI fragment because it uses an HTTP upgrade instead of a protobuf HTTP annotation.

When authentication is enabled, WebSocket clients must send credentials during the upgrade request. Basic auth clients can use a URL or an `Authorization` header depending on the client library. OAuth2 clients should send `Authorization: Bearer <token>`.

## Ports

- `8080` - HTTP/JSON API
- `50051` - gRPC API

## Observability

- `/metrics` - Prometheus metrics endpoint (on HTTP port)

## Configuration

CEGW supports a few simple environment variables:

- `GRPC_PORT` (default `50051`)
- `HTTP_PORT` (default `8080`)
- `LOG_LEVEL` (default `info`)
- `TIMEZONE` (default `Asia/Jakarta`)
- `SANDBOX_MODE` (default `false`)
- `HTTPS_PROXY` (optional, supports `http://`, `https://`, `socks5://`)
- `HTTP_PROXY` (optional, supports `http://`, `https://`, `socks5://`)
- `NO_PROXY` (optional, comma-separated list of hosts to bypass proxy)
- `ALLOWED_WS_ORIGINS` (optional, comma-separated list of allowed browser WebSocket origins; empty allows all origins)
- `WS_PRICE_POLL_INTERVAL` (default `5s`) - Price stream polling interval when native WebSocket is unavailable
- `WS_ORDERBOOK_POLL_INTERVAL` (default `3s`) - Order book stream polling interval when native WebSocket is unavailable

For WebSocket origin restriction, supported values include exact origins like `https://app.example.com`, wildcard `*`, and subdomain patterns like `*.example.com`. Non-browser clients without an `Origin` header are still allowed.

### Helm values for WebSocket origins

Set browser WebSocket origins in `values.yaml` as an array:

```yaml
config:
  allowedWsOrigins:
    - https://app.example.com
    - https://admin.example.com
    - '*.trusted.com'
```

This is rendered into the `ALLOWED_WS_ORIGINS` environment variable as a comma-separated list.
### Authentication (optional)

- `AUTH_ENABLED` (default `false`) - Enable authentication
- `AUTH_TYPE` (default `basic`) - Auth type: `basic` or `oauth2`

**Basic Auth:**

- `AUTH_BASIC_USERNAME` - Username for basic auth
- `AUTH_BASIC_PASSWORD` - Password for basic auth

**OAuth2:**

- `AUTH_OAUTH2_ISSUER` - OAuth2 issuer URL
- `AUTH_OAUTH2_AUDIENCE` - OAuth2 audience

Note: Health check endpoints (`/healthz`, `/readyz`, `/metrics`) are always accessible without auth.

## Docker Compose

Use the included `docker-compose.yml` to start the service locally.

## Supported exchange today

- Tokocrypto
- Binance
- Coinbase
- CEX.IO
- Indodax
- OKX
- KuCoin
- Crypto.com
- Bybit
- Bitget
- Coinex
- Hashkey

## License

MIT
