package health

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/dcm-project/k8s-container-service-provider/api/v1alpha1"
	"github.com/dcm-project/k8s-container-service-provider/internal/util"
)

// Handler serves the /health endpoint.
// It is a lightweight component with no external dependencies.
type Handler struct {
	startTime time.Time
	version   string
	logger    *slog.Logger
}

// NewHandler creates a new health Handler with the given start time, version, and logger.
func NewHandler(startTime time.Time, version string, logger *slog.Logger) *Handler {
	return &Handler{
		startTime: startTime,
		version:   version,
		logger:    logger,
	}
}

// GetHealth handles GET /health requests.
func (h *Handler) GetHealth(w http.ResponseWriter, r *http.Request) {
	uptime := max(0, int(time.Since(h.startTime).Seconds()))

	resp := v1alpha1.Health{
		Status:  "healthy",
		Type:    util.Ptr("k8s-container-service-provider.dcm.io/health"),
		Path:    util.Ptr("health"),
		Uptime:  &uptime,
		Version: util.Ptr(h.version),
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		h.logger.Error("failed to encode health response", "error", err)
	}
}
