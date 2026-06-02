package server

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	cegwv1 "github.com/michaelahli/cegw/gen/cegw/v1"
	"github.com/michaelahli/cegw/internal/config"
	"github.com/michaelahli/cegw/internal/logger"
	"github.com/michaelahli/cegw/internal/middleware"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type HTTPServer struct {
	server *http.Server
	cfg    *config.Config
	log    *logger.Logger
}

func NewHTTPServer(cfg *config.Config, log *logger.Logger) *HTTPServer {
	return &HTTPServer{
		cfg: cfg,
		log: log,
	}
}

func (s *HTTPServer) Start(ctx context.Context) error {
	// gRPC gateway mux
	mux := runtime.NewServeMux()

	opts := []grpc.DialOption{grpc.WithTransportCredentials(insecure.NewCredentials())}
	grpcEndpoint := fmt.Sprintf("localhost:%s", s.cfg.GRPCPort)

	if err := cegwv1.RegisterMarketDataServiceHandlerFromEndpoint(ctx, mux, grpcEndpoint, opts); err != nil {
		s.log.WithError(err).Errorf("failed to register market data handler")
		return fmt.Errorf("failed to register market data handler: %w", err)
	}

	if err := cegwv1.RegisterTradingServiceHandlerFromEndpoint(ctx, mux, grpcEndpoint, opts); err != nil {
		s.log.WithError(err).Errorf("failed to register trading handler")
		return fmt.Errorf("failed to register trading handler: %w", err)
	}

	if err := cegwv1.RegisterMonitoringServiceHandlerFromEndpoint(ctx, mux, grpcEndpoint, opts); err != nil {
		s.log.WithError(err).Errorf("failed to register monitoring handler")
		return fmt.Errorf("failed to register monitoring handler: %w", err)
	}

	// Create main HTTP mux for routing
	mainMux := http.NewServeMux()

	// Serve static docs
	fs := http.FileServer(http.Dir("docs"))
	mainMux.Handle("/docs/", http.StripPrefix("/docs/", fs))

	// Mount gRPC gateway under root path with logging middleware
	mainMux.Handle("/", middleware.HTTPLoggingMiddleware(s.log)(mux))

	s.server = &http.Server{
		Addr:              fmt.Sprintf(":%s", s.cfg.HTTPPort),
		Handler:           mainMux,
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
		defer cancel()
		if err := s.server.Shutdown(shutdownCtx); err != nil {
			s.log.WithError(err).WithField("component", "http").Errorf("HTTP server shutdown error")
		}
	}()

	s.log.WithField("component", "http").Debugf("HTTP server listening on %s", s.server.Addr)
	if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		s.log.WithError(err).WithField("component", "http").Errorf("failed to serve http")
		return fmt.Errorf("failed to serve http: %w", err)
	}

	return nil
}
