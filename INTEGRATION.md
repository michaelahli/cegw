# Integration Guide

## How to Integrate CEGW Client in Your Project

### Step 1: Add Dependency

In your project's `go.mod`:

```bash
go get github.com/michaelahli/cegw@latest
```

### Step 2: Import and Use

```go
import (
    cegwv1 "github.com/michaelahli/cegw/gen/cegw/v1"
    "github.com/michaelahli/cegw/pkg/client"
)

// Connect
c, err := client.New(ctx, client.Config{
    Address: "cegw-service:50051",
})
defer c.Close()

// Use
price, err := c.MarketDataService.GetCurrentPrice(ctx, req)
```

### Step 3: Deploy CEGW

Using Docker:
```bash
docker run -p 50051:50051 -p 8080:8080 ghcr.io/michaelahli/cegw:latest
```

Using Kubernetes:
```bash
helm repo add cegw https://michaelahli.github.io/cegw
helm install cegw cegw/cegw
```

### Configuration

Client connects to `Address` (default port 50051).

Service endpoints:
- gRPC: `localhost:50051`
- HTTP: `localhost:8080`
- Metrics: `localhost:8080/metrics`
