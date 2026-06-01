# CEGW - Crypto Exchange Gateway

Cloud-native HTTP and gRPC gateway for cryptocurrency exchanges powered by CCXT. Provides a unified API for market data, account management, and trading operations.

## Features

- **Dual Protocol Support**: Both gRPC (port 50051) and HTTP/JSON (port 8080) APIs
- **Multiple Exchanges**: Tokocrypto (with plans for Binance, CEX.IO, Coinbase, Indodax, OKX)
- **Market Data**: Historical OHLCV quotes, current prices, ticker search
- **Trading**: Market orders (buy/sell), credential validation
- **Monitoring**: Price alert evaluation
- **Cloud Native**: Docker images, Helm charts, Kubernetes-ready
- **Observability**: Structured logging, health checks
- **Proxy Support**: HTTP/HTTPS/SOCKS5 proxy configuration

## Quick Start

### Prerequisites

- Go 1.23+
- Protocol Buffers compiler (protoc)
- Docker (optional)
- Kubernetes + Helm (optional)

### Installation

```bash
# Clone repository
git clone https://github.com/michaelahli/cegw.git
cd cegw

# Install dependencies
make deps

# Generate protobuf files
make proto

# Build binary
make build

# Run
./bin/cegw
```

### Using Docker

```bash
# Build image
docker build -t cegw:latest -f Dockerfile.alpine .

# Run container
docker run -p 50051:50051 -p 8080:8080 cegw:latest
```

### Using Helm

```bash
# Add Helm repository
helm repo add cegw https://michaelahli.github.io/cegw

# Install
helm install cegw cegw/cegw

# With custom values
helm install cegw cegw/cegw -f values.yaml
```

## Configuration

Environment variables:

| Variable | Default | Description |
|----------|---------|-------------|
| `GRPC_PORT` | `50051` | gRPC server port |
| `HTTP_PORT` | `8080` | HTTP server port |
| `LOG_LEVEL` | `info` | Log level (debug, info, warn, error) |
| `TIMEZONE` | `Asia/Jakarta` | Timezone for date operations |
| `SANDBOX_MODE` | `false` | Enable sandbox mode for testing |
| `HTTPS_PROXY` | - | HTTPS proxy URL |
| `HTTP_PROXY` | - | HTTP proxy URL |

## API Documentation

API documentation is available via Redoc at `docs/index.html`.

### Example Requests

#### Get Current Price (HTTP)

```bash
curl http://localhost:8080/v1/market/price/1/BTC/USDT
```

#### List Markets (HTTP)

```bash
curl http://localhost:8080/v1/market/list?exchange=1
```

#### Create Market Order (HTTP)

```bash
curl -X POST http://localhost:8080/v1/trading/order \
  -H "Content-Type: application/json" \
  -d '{
    "exchange": 1,
    "symbol": "BTC/USDT",
    "side": 1,
    "quantity": 0.001,
    "credentials": {
      "api_key": "your-api-key",
      "api_secret": "your-api-secret",
      "sandbox": true
    }
  }'
```

#### Get Quotes (gRPC)

```bash
grpcurl -plaintext \
  -d '{
    "exchange": 1,
    "symbol": "BTC/USDT",
    "interval": 4,
    "limit": 100
  }' \
  localhost:50051 cegw.v1.MarketDataService/GetQuotes
```

## Development

### Project Structure

```
.
├── cmd/cegw/              # Main application entry point
├── internal/
│   ├── ccxt/              # CCXT client wrapper
│   ├── config/            # Configuration management
│   ├── server/            # gRPC and HTTP servers
│   └── service/           # Business logic services
├── proto/cegw/v1/         # Protocol buffer definitions
├── gen/                   # Generated code (gitignored)
├── charts/cegw/           # Helm chart
├── docs/                  # API documentation
└── scripts/               # Build scripts
```

### Building

```bash
# Generate protobuf files
make proto

# Build binary
make build

# Run tests
make test

# Run linters
make lint

# Clean generated files
make clean
```

### Testing

```bash
# Run all tests
make test

# Run specific package tests
go test -v ./internal/ccxt/

# Run with coverage
go test -v -race -coverprofile=coverage.out ./...
```

## Supported Exchanges

### Phase 1 (Current)
- ✅ Tokocrypto

### Phase 2 (Planned)
- ⏳ Binance
- ⏳ CEX.IO
- ⏳ Coinbase

### Phase 3 (Future)
- ⏳ Indodax
- ⏳ OKX

## Deployment

### Kubernetes

```bash
# Deploy with Helm
helm install cegw charts/cegw \
  --set image.tag=1.0.0 \
  --set replicaCount=3 \
  --set config.logLevel=info

# Check status
kubectl get pods -l app.kubernetes.io/name=cegw

# View logs
kubectl logs -l app.kubernetes.io/name=cegw -f
```

### Docker Compose

```yaml
version: '3.8'
services:
  cegw:
    image: ghcr.io/michaelahli/cegw:latest
    ports:
      - "50051:50051"
      - "8080:8080"
    environment:
      - LOG_LEVEL=info
      - TIMEZONE=Asia/Jakarta
    restart: unless-stopped
```

## Architecture

CEGW uses a dual-protocol architecture:

1. **gRPC Server** (port 50051): Binary protocol for high-performance communication
2. **HTTP Gateway** (port 8080): RESTful JSON API via grpc-gateway
3. **CCXT Integration**: Unified interface to cryptocurrency exchanges
4. **Stateless Design**: No credential storage, all auth per-request

## License

MIT License - see LICENSE file for details
