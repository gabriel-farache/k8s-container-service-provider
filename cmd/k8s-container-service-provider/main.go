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

	oapigen "github.com/dcm-project/k8s-container-service-provider/internal/api/server"
	"github.com/dcm-project/k8s-container-service-provider/internal/apiserver"
	"github.com/dcm-project/k8s-container-service-provider/internal/config"
	containerhandler "github.com/dcm-project/k8s-container-service-provider/internal/handlers/container"
	k8s "github.com/dcm-project/k8s-container-service-provider/internal/kubernetes"
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

	k8sClient, err := k8s.NewClient(cfg.Kubernetes.Kubeconfig)
	if err != nil {
		return fmt.Errorf("creating kubernetes client: %w", err)
	}
	k8sCfg := k8s.K8sConfig{
		Namespace:          cfg.Kubernetes.Namespace,
		DefaultServiceType: cfg.Kubernetes.DefaultServiceType,
	}
	store := k8s.NewK8sContainerStore(k8sClient, k8sCfg, logger)

	containerHandler := containerhandler.NewHandler(store, logger, time.Now(), version)
	strictAdapter := oapigen.NewStrictHandlerWithOptions(containerHandler, nil, oapigen.StrictHTTPServerOptions{})

	srv := apiserver.New(cfg, logger, strictAdapter).WithOnReady(registrar.Start)

	return srv.Run(ctx, ln)
}
