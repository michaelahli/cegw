package server

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	cegwv1 "github.com/michaelahli/cegw/gen/cegw/v1"
	"github.com/michaelahli/cegw/internal/config"
	"github.com/sirupsen/logrus"
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
	// Create main HTTP mux for routing
	mainMux := http.NewServeMux()

	// Serve static docs
	fs := http.FileServer(http.Dir("docs"))
	mainMux.Handle("/docs/", http.StripPrefix("/docs/", fs))

	// gRPC gateway mux
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

	// Mount gRPC gateway under root path
	mainMux.Handle("/", mux)

	s.server = &http.Server{
		Addr:              fmt.Sprintf(":%s", s.cfg.HTTPPort),
		Handler:           mainMux,
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() { // nolint:gosec
		<-ctx.Done()
		if err := s.server.Shutdown(context.Background()); err != nil {
			logrus.WithError(err).Error("HTTP server shutdown error")
		}
	}()

	if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("failed to serve http: %w", err)
	}

	return nil
}
