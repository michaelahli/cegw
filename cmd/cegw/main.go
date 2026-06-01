package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/michaelahli/cegw/internal/config"
	"github.com/michaelahli/cegw/internal/server"
	"github.com/sirupsen/logrus"
)

func main() {
	log := logrus.New()
	log.SetFormatter(&logrus.JSONFormatter{})

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	level, err := logrus.ParseLevel(cfg.LogLevel)
	if err != nil {
		level = logrus.InfoLevel
	}
	log.SetLevel(level)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	grpcServer := server.NewGRPCServer(cfg)
	httpServer := server.NewHTTPServer(cfg)

	errChan := make(chan error, 2)

	go func() {
		log.Infof("Starting gRPC server on port %s", cfg.GRPCPort)
		if err := grpcServer.Start(ctx); err != nil {
			errChan <- fmt.Errorf("grpc server error: %w", err)
		}
	}()

	go func() {
		log.Infof("Starting HTTP server on port %s", cfg.HTTPPort)
		if err := httpServer.Start(ctx); err != nil {
			errChan <- fmt.Errorf("http server error: %w", err)
		}
	}()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	select {
	case err := <-errChan:
		log.Errorf("Server error: %v", err)
		cancel()
	case sig := <-sigChan:
		log.Infof("Received signal %v, shutting down gracefully", sig)
		cancel()
	}

	log.Info("Server stopped")
}
