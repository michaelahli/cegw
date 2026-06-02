package server

import (
	"context"
	"fmt"
	"net"

	cegwv1 "github.com/michaelahli/cegw/gen/cegw/v1"
	"github.com/michaelahli/cegw/internal/config"
	"github.com/michaelahli/cegw/internal/logger"
	"github.com/michaelahli/cegw/internal/middleware"
	"github.com/michaelahli/cegw/internal/service"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"
)

type GRPCServer struct {
	server *grpc.Server
	cfg    *config.Config
	log    *logger.Logger
}

func NewGRPCServer(cfg *config.Config, log *logger.Logger) *GRPCServer {
	s := grpc.NewServer(
		grpc.UnaryInterceptor(middleware.GRPCUnaryLoggingInterceptor(log)),
		grpc.StreamInterceptor(middleware.GRPCStreamLoggingInterceptor(log)),
	)

	marketDataSvc := service.NewMarketDataService(cfg, log)
	tradingSvc := service.NewTradingService(cfg, log)
	monitoringSvc := service.NewMonitoringService(cfg, log)

	cegwv1.RegisterMarketDataServiceServer(s, marketDataSvc)
	cegwv1.RegisterTradingServiceServer(s, tradingSvc)
	cegwv1.RegisterMonitoringServiceServer(s, monitoringSvc)

	healthServer := health.NewServer()
	grpc_health_v1.RegisterHealthServer(s, healthServer)
	healthServer.SetServingStatus("", grpc_health_v1.HealthCheckResponse_SERVING)

	reflection.Register(s)

	return &GRPCServer{
		server: s,
		cfg:    cfg,
		log:    log,
	}
}

func (s *GRPCServer) Start(ctx context.Context) error {
	lis, err := net.Listen("tcp", fmt.Sprintf(":%s", s.cfg.GRPCPort))
	if err != nil {
		return fmt.Errorf("failed to listen: %w", err)
	}

	go func() {
		<-ctx.Done()
		s.server.GracefulStop()
	}()

	if err := s.server.Serve(lis); err != nil {
		return fmt.Errorf("failed to serve: %w", err)
	}

	return nil
}
