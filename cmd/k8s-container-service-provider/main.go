package main

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/dcm-project/k8s-container-service-provider/internal/apiserver"
	"github.com/dcm-project/k8s-container-service-provider/internal/config"
	"github.com/dcm-project/k8s-container-service-provider/internal/handlers"
	"github.com/dcm-project/k8s-container-service-provider/internal/registration"
)

// version is the application version, set at build time via
// -ldflags "-X main.version=X.Y.Z".
var version = "0.0.1-dev"

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	if err := run(logger); err != nil {
		logger.Error("fatal error", "error", err)
		os.Exit(1)
	}
}

func run(logger *slog.Logger) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("initializing: %w", err)
	}

	ln, err := net.Listen("tcp", cfg.Server.Address)
	if err != nil {
		return fmt.Errorf("listening on %s: %w", cfg.Server.Address, err)
	}
	defer ln.Close()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()

	registrar, err := registration.NewRegistrar(cfg, logger)
	if err != nil {
		return fmt.Errorf("failed to create registrar: %w", err)
	}

	h := handlers.New(logger, time.Now(), version)
	srv := apiserver.New(cfg, logger, h).WithOnReady(registrar.Start)

	return srv.Run(ctx, ln)
}
