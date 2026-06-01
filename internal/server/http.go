package server

import (
	"context"
	"fmt"
	"net/http"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"github.com/michaelahli/cegw/gen/cegw/v1"
	"github.com/michaelahli/cegw/internal/config"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type HTTPServer struct {
	server *http.Server
	cfg    *config.Config
}

func NewHTTPServer(cfg *config.Config) *HTTPServer {
	return &HTTPServer{
		cfg: cfg,
	}
}

func (s *HTTPServer) Start(ctx context.Context) error {
	mux := runtime.NewServeMux()

	opts := []grpc.DialOption{grpc.WithTransportCredentials(insecure.NewCredentials())}
	grpcEndpoint := fmt.Sprintf("localhost:%s", s.cfg.GRPCPort)

	if err := cegwv1.RegisterMarketDataServiceHandlerFromEndpoint(ctx, mux, grpcEndpoint, opts); err != nil {
		return fmt.Errorf("failed to register market data handler: %w", err)
	}

	if err := cegwv1.RegisterTradingServiceHandlerFromEndpoint(ctx, mux, grpcEndpoint, opts); err != nil {
		return fmt.Errorf("failed to register trading handler: %w", err)
	}

	if err := cegwv1.RegisterMonitoringServiceHandlerFromEndpoint(ctx, mux, grpcEndpoint, opts); err != nil {
		return fmt.Errorf("failed to register monitoring handler: %w", err)
	}

	s.server = &http.Server{
		Addr:    fmt.Sprintf(":%s", s.cfg.HTTPPort),
		Handler: mux,
	}

	go func() {
		<-ctx.Done()
		s.server.Shutdown(context.Background())
	}()

	if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("failed to serve http: %w", err)
	}

	return nil
}
