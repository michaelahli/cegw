package server

import (
	"context"
	"fmt"
	"io"
	"net"
	"testing"
	"time"

	cegwv1 "github.com/michaelahli/cegw/gen/cegw/v1"
	"github.com/michaelahli/cegw/internal/config"
	"github.com/michaelahli/cegw/internal/logger"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
)

const bufSize = 1024 * 1024

type TestServer struct {
	GRPCServer *GRPCServer
	HTTPServer *HTTPServer
	Listener   *bufconn.Listener
	Conn       *grpc.ClientConn
	ctx        context.Context
	cancel     context.CancelFunc
	t          *testing.T
}

func NewTestServer(t *testing.T) *TestServer {
	t.Helper()

	cfg := &config.Config{
		GRPCPort:    "0",
		HTTPPort:    "0",
		LogLevel:    "error",
		Timezone:    time.UTC,
		SandboxMode: true,
	}

	ctx, cancel := context.WithCancel(context.Background())

	// Create logger that discards output for tests
	log := logger.New("error", io.Discard)

	grpcServer := NewGRPCServer(cfg, log)
	listener := bufconn.Listen(bufSize)

	go func() {
		if err := grpcServer.server.Serve(listener); err != nil {
			t.Logf("Test gRPC server error: %v", err)
		}
	}()

	//nolint:staticcheck // bufnet dialer requires DialContext pattern
	conn, err := grpc.DialContext(ctx, "bufnet",
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) {
			return listener.Dial()
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		cancel()
		t.Fatalf("Failed to dial bufnet: %v", err)
	}

	return &TestServer{
		GRPCServer: grpcServer,
		Listener:   listener,
		Conn:       conn,
		ctx:        ctx,
		cancel:     cancel,
		t:          t,
	}
}

func (ts *TestServer) Close() {
	ts.t.Helper()
	if ts.Conn != nil {
		_ = ts.Conn.Close()
	}
	if ts.GRPCServer != nil {
		ts.GRPCServer.server.GracefulStop()
	}
	if ts.Listener != nil {
		_ = ts.Listener.Close()
	}
	if ts.cancel != nil {
		ts.cancel()
	}
}

func (ts *TestServer) NewMarketDataClient() cegwv1.MarketDataServiceClient {
	return cegwv1.NewMarketDataServiceClient(ts.Conn)
}

func (ts *TestServer) NewTradingClient() cegwv1.TradingServiceClient {
	return cegwv1.NewTradingServiceClient(ts.Conn)
}

func (ts *TestServer) NewMonitoringClient() cegwv1.MonitoringServiceClient {
	return cegwv1.NewMonitoringServiceClient(ts.Conn)
}

func GetFreePort() (int, error) {
	addr, err := net.ResolveTCPAddr("tcp", "localhost:0")
	if err != nil {
		return 0, err
	}

	l, err := net.ListenTCP("tcp", addr)
	if err != nil {
		return 0, err
	}
	defer func() { _ = l.Close() }()
	return l.Addr().(*net.TCPAddr).Port, nil
}

func StartRealServer(t *testing.T) (string, string, context.CancelFunc) {
	t.Helper()

	grpcPort, err := GetFreePort()
	if err != nil {
		t.Fatalf("Failed to get free port for gRPC: %v", err)
	}

	httpPort, err := GetFreePort()
	if err != nil {
		t.Fatalf("Failed to get free port for HTTP: %v", err)
	}

	cfg := &config.Config{
		GRPCPort:    fmt.Sprintf("%d", grpcPort),
		HTTPPort:    fmt.Sprintf("%d", httpPort),
		LogLevel:    "error",
		Timezone:    time.UTC,
		SandboxMode: true,
	}

	ctx, cancel := context.WithCancel(context.Background())

	// Create logger that discards output for tests
	log := logger.New("error", io.Discard)

	grpcServer := NewGRPCServer(cfg, log)
	httpServer := NewHTTPServer(cfg, log)

	go func() {
		if err := grpcServer.Start(ctx); err != nil {
			t.Logf("gRPC server error: %v", err)
		}
	}()

	go func() {
		if err := httpServer.Start(ctx); err != nil {
			t.Logf("HTTP server error: %v", err)
		}
	}()

	return fmt.Sprintf("localhost:%d", grpcPort), fmt.Sprintf("localhost:%d", httpPort), cancel
}
