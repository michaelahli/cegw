// Package client provides a simple gRPC client for CEGW API.
package client

import (
	"context"
	"fmt"
	"time"

	cegwv1 "github.com/michaelahli/cegw/gen/cegw/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// Client wraps the CEGW gRPC clients
type Client struct {
	conn              *grpc.ClientConn
	MarketDataService cegwv1.MarketDataServiceClient
	TradingService    cegwv1.TradingServiceClient
	MonitoringService cegwv1.MonitoringServiceClient
}

// Config holds client configuration
type Config struct {
	Address string
	Timeout time.Duration
}

// New creates a new CEGW client
func New(ctx context.Context, cfg Config) (*Client, error) {
	if cfg.Address == "" {
		return nil, fmt.Errorf("address is required")
	}

	if cfg.Timeout == 0 {
		cfg.Timeout = 30 * time.Second
	}

	opts := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	}

	conn, err := grpc.NewClient(cfg.Address, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to connect: %w", err)
	}

	return &Client{
		conn:              conn,
		MarketDataService: cegwv1.NewMarketDataServiceClient(conn),
		TradingService:    cegwv1.NewTradingServiceClient(conn),
		MonitoringService: cegwv1.NewMonitoringServiceClient(conn),
	}, nil
}

// Close closes the client connection
func (c *Client) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}
