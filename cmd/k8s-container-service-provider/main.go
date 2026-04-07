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
	"github.com/dcm-project/k8s-container-service-provider/internal/monitoring"
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
	defer func() { _ = ln.Close() }()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()

	registrar, err := registration.NewRegistrar(cfg, logger)
	if err != nil {
		return fmt.Errorf("failed to create registrar: %w", err)
	}

	publisher, err := monitoring.NewNATSPublisher(cfg.NATS.URL, cfg.Provider.Name, logger)
	if err != nil {
		return fmt.Errorf("creating NATS publisher: %w", err)
	}
	defer func() { _ = publisher.Close() }()

	k8sClient, err := k8s.NewClient(cfg.Kubernetes.Kubeconfig)
	if err != nil {
		return fmt.Errorf("creating kubernetes client: %w", err)
	}
	k8sCfg := k8s.K8sConfig{
		Namespace:           cfg.Kubernetes.Namespace,
		ExternalServiceType: cfg.Kubernetes.ExternalServiceType,
	}
	store := k8s.NewK8sContainerStore(k8sClient, k8sCfg, logger)

	monitorCfg := monitoring.MonitorConfig{
		Namespace:    cfg.Kubernetes.Namespace,
		ProviderName: cfg.Provider.Name,
		DebounceMs:   cfg.Monitoring.DebounceMs,
		ResyncPeriod: cfg.Monitoring.ResyncPeriod,
	}
	monitor := monitoring.NewStatusMonitor(k8sClient, monitorCfg, publisher, logger)

	containerHandler := containerhandler.NewHandler(store, logger, time.Now(), version)
	strictAdapter := oapigen.NewStrictHandlerWithOptions(containerHandler, nil, oapigen.StrictHTTPServerOptions{
		RequestErrorHandlerFunc:  apiserver.NewRequestErrorHandler(logger),
		ResponseErrorHandlerFunc: apiserver.NewResponseErrorHandler(logger),
	})

	srv := apiserver.New(cfg, logger, strictAdapter).WithOnReady(func(ctx context.Context) {
		registrar.Start(ctx)
		go func() {
			if err := monitor.Start(ctx); err != nil {
				logger.Error("status monitor failed", "error", err)
			}
		}()
	})

	return srv.Run(ctx, ln)
}
