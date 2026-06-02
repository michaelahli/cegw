package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/michaelahli/cegw/internal/config"
	"github.com/michaelahli/cegw/internal/logger"
	"github.com/michaelahli/cegw/internal/metrics"
	"github.com/michaelahli/cegw/internal/server"
)

func main() {
	log := logger.New("info", os.Stdout)

	cfg, err := config.Load()
	if err != nil {
		log.WithError(err).Fatalf("failed to load config")
	}

	// Set log level from config
	logLevel := cfg.LogLevel
	if logLevel == "" {
		logLevel = "info"
	}
	log = logger.New(logLevel, os.Stdout)

	// Initialize Prometheus metrics
	_, err = metrics.InitPrometheus(context.Background())
	if err != nil {
		log.WithError(err).Warnf("failed to initialize metrics")
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	log.WithFields(map[string]interface{}{
		"grpc_port":   cfg.GRPCPort,
		"http_port":   cfg.HTTPPort,
		"log_level":   logLevel,
		"sandbox_mode": cfg.SandboxMode,
	}).Infof("CEGW starting up")

	grpcServer := server.NewGRPCServer(cfg, log)
	httpServer := server.NewHTTPServer(cfg, log)

	errChan := make(chan error, 2)

	go func() {
		log.WithField("component", "grpc").Infof("Starting gRPC server on port %s", cfg.GRPCPort)
		if err := grpcServer.Start(ctx); err != nil {
			errChan <- fmt.Errorf("grpc server error: %w", err)
		}
	}()

	go func() {
		log.WithField("component", "http").Infof("Starting HTTP server on port %s", cfg.HTTPPort)
		if err := httpServer.Start(ctx); err != nil {
			errChan <- fmt.Errorf("http server error: %w", err)
		}
	}()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	select {
	case err := <-errChan:
		log.WithError(err).WithField("component", "server").Errorf("Server error")
		cancel()
	case sig := <-sigChan:
		log.WithField("signal", sig.String()).Infof("Received signal, shutting down gracefully")
		cancel()
	}

	log.Infof("CEGW shutdown complete")
}
