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
	"github.com/michaelahli/cegw/internal/metrics"
	"github.com/michaelahli/cegw/internal/middleware"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/health/grpc_health_v1"
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

	// Serve Prometheus metrics
	mainMux.Handle("/metrics", metrics.Handler())

	// Serve OpenAPI documentation
	mainMux.HandleFunc("/docs", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "docs/index.html")
	})
	mainMux.HandleFunc("/docs/openapi.json", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "docs/openapi.json")
	})

	// Health check endpoints for Kubernetes
	grpcEndpointHealth := fmt.Sprintf("localhost:%s", s.cfg.GRPCPort)
	mainMux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()

		conn, err := grpc.NewClient(grpcEndpointHealth, grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte("unhealthy"))
			return
		}
		defer func() { _ = conn.Close() }()

		healthClient := grpc_health_v1.NewHealthClient(conn)
		resp, err := healthClient.Check(ctx, &grpc_health_v1.HealthCheckRequest{})
		if err != nil || resp.Status != grpc_health_v1.HealthCheckResponse_SERVING {
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte("unhealthy"))
			return
		}

		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	mainMux.HandleFunc("/readyz", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()

		conn, err := grpc.NewClient(grpcEndpointHealth, grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte("not ready"))
			return
		}
		defer func() { _ = conn.Close() }()

		healthClient := grpc_health_v1.NewHealthClient(conn)
		resp, err := healthClient.Check(ctx, &grpc_health_v1.HealthCheckRequest{})
		if err != nil || resp.Status != grpc_health_v1.HealthCheckResponse_SERVING {
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte("not ready"))
			return
		}

		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ready"))
	})

	// WebSocket market data streams
	wsHandler := middleware.AuthMiddleware(s.cfg, s.log)(handlePriceWebsocket(s.log))
	wsHandler = middleware.HTTPLoggingMiddleware(s.log)(wsHandler)
	mainMux.Handle("/v1/ws/market/price", wsHandler)

	// Mount gRPC gateway under root path with auth and logging middleware
	handler := middleware.AuthMiddleware(s.cfg, s.log)(mux)
	handler = middleware.HTTPLoggingMiddleware(s.log)(handler)
	mainMux.Handle("/", handler)

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
