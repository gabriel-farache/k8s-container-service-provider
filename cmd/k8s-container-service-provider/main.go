package main

import (
	"context"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/dcm-project/k8s-container-service-provider/internal/apiserver"
	"github.com/dcm-project/k8s-container-service-provider/internal/config"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	cfg, err := config.Load()
	if err != nil {
		logger.Error("failed to load configuration", "error", err)
		os.Exit(1)
	}

	ln, err := net.Listen("tcp", cfg.Server.Address)
	if err != nil {
		logger.Error("failed to listen", "address", cfg.Server.Address, "error", err)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()

	srv := apiserver.New(cfg, logger)
	if err := srv.Run(ctx, ln); err != nil {
		logger.Error("server error", "error", err)
		os.Exit(1)
	}
}
