package handlers

import (
	"log/slog"
	"net/http"
	"time"

	oapigen "github.com/dcm-project/k8s-container-service-provider/internal/api/server"
	"github.com/dcm-project/k8s-container-service-provider/internal/handlers/health"
)

// TC-U008: Compile-time assertion that Handler satisfies oapigen.ServerInterface.
var _ oapigen.ServerInterface = (*Handler)(nil)

// Handler is the composite API handler that implements oapigen.ServerInterface.
// It embeds Unimplemented so only implemented endpoints need explicit methods.
// Sub-handlers are composed here; the apiserver package is responsible only
// for HTTP transport concerns (router, middleware, lifecycle).
type Handler struct {
	oapigen.Unimplemented
	health *health.Handler
}

// New creates a composite Handler with all sub-handlers initialised.
func New(logger *slog.Logger, startTime time.Time, version string) *Handler {
	return &Handler{
		health: health.NewHandler(startTime, version, logger),
	}
}

// GetHealth delegates to the health sub-handler.
func (h *Handler) GetHealth(w http.ResponseWriter, r *http.Request) {
	h.health.GetHealth(w, r)
}
