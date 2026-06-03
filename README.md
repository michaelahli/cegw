# CEGW

CEGW is a lightweight gateway for cryptocurrency exchange data and trading.
It provides a simple HTTP/JSON API and gRPC endpoint so your apps can access market prices, symbols, and trading functionality from a unified service.

## What it does

- Fetch market data for supported exchanges
- Return current prices, market lists, and trading pairs
- Place market buy/sell orders
- Support both HTTP/JSON and gRPC clients
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

## Quick usage

### Get current price

```bash
curl http://localhost:8080/v1/market/price/1/BTC/USDT
```

### List markets

```bash
curl http://localhost:8080/v1/market/list?exchange=1
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

## License

MIT
