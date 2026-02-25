package apiserver

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"net/http"

	v1alpha1 "github.com/dcm-project/k8s-container-service-provider/api/v1alpha1"
	oapigen "github.com/dcm-project/k8s-container-service-provider/internal/api/server"
	"github.com/dcm-project/k8s-container-service-provider/internal/config"
	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/getkin/kin-openapi/routers"
	legacyrouter "github.com/getkin/kin-openapi/routers/legacy"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

// apiHandler implements oapigen.ServerInterface.
// It embeds Unimplemented so only GetHealth needs an override for now.
type apiHandler struct {
	oapigen.Unimplemented
	logger *slog.Logger
}

func (h *apiHandler) GetHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(map[string]string{"status": "healthy"}); err != nil {
		h.logger.Error("failed to encode health response", "error", err)
	}
}

// Server is the HTTP server for the container service provider API.
type Server struct {
	cfg    *config.Config
	logger *slog.Logger
	srv    *http.Server
}

// newBadRequestHandler returns a handler that writes a 400 Bad Request
// response with an RFC 7807 application/problem+json body. It is used
// by the parameter binding layer (generated chi wrapper), OpenAPI
// validation middleware, and the empty-container_id guard.
func newBadRequestHandler(logger *slog.Logger) func(http.ResponseWriter, *http.Request, error) {
	return func(w http.ResponseWriter, _ *http.Request, err error) {
		w.Header().Set("Content-Type", "application/problem+json")
		w.WriteHeader(http.StatusBadRequest)
		if encErr := json.NewEncoder(w).Encode(map[string]any{
			"type":   "INVALID_ARGUMENT",
			"title":  "Bad Request",
			"status": http.StatusBadRequest,
			"detail": err.Error(),
		}); encErr != nil {
			logger.Error("failed to encode error response", "error", encErr)
		}
	}
}

// New creates a new Server with the given config and logger.
func New(cfg *config.Config, logger *slog.Logger) *Server {
	h := &apiHandler{logger: logger}
	badReq := newBadRequestHandler(logger)

	r := chi.NewRouter()
	r.Use(middleware.Recoverer)

	// Load OpenAPI spec for request validation middleware.
	spec, err := v1alpha1.GetSwagger()
	if err != nil {
		logger.Warn("failed to load OpenAPI spec, request validation disabled", "error", err)
	} else {
		spec.Servers = nil // Avoid URL prefix matching issues.
		specRouter, routerErr := legacyrouter.NewRouter(spec)
		if routerErr != nil {
			logger.Warn("failed to create OpenAPI router, request validation disabled", "error", routerErr)
		} else {
			r.Use(openAPIValidationMiddleware(specRouter, badReq))
		}
	}

	// Reject trailing-slash requests with empty container_id before the
	// generated router sees them. Chi treats /containers/ as a distinct
	// path from /containers/{container_id}, so without this route it would 404.
	emptyIDHandler := func(w http.ResponseWriter, r *http.Request) {
		badReq(w, r, fmt.Errorf("container_id is required and cannot be empty"))
	}
	r.Get("/api/v1alpha1/containers/", emptyIDHandler)
	r.Delete("/api/v1alpha1/containers/", emptyIDHandler)

	handler := oapigen.HandlerWithOptions(h, oapigen.ChiServerOptions{
		BaseRouter:       r,
		ErrorHandlerFunc: badReq,
	})

	return &Server{
		cfg:    cfg,
		logger: logger,
		srv:    &http.Server{Handler: handler},
	}
}

// openAPIValidationMiddleware validates incoming requests against the OpenAPI spec.
// It checks path/query parameter constraints and request body validation.
// Routes not found in the spec are passed through to the chi router.
func openAPIValidationMiddleware(specRouter routers.Router, badReq func(http.ResponseWriter, *http.Request, error)) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			route, pathParams, err := specRouter.FindRoute(r)
			if err != nil {
				next.ServeHTTP(w, r)
				return
			}

			input := &openapi3filter.RequestValidationInput{
				Request:    r,
				PathParams: pathParams,
				Route:      route,
			}

			if err := openapi3filter.ValidateRequest(r.Context(), input); err != nil {
				badReq(w, r, err)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// Run starts the HTTP server on the provided listener and blocks until
// the context is cancelled. Signal handling is the caller's responsibility;
// pass a context that is cancelled on SIGTERM/SIGINT (e.g., via
// signal.NotifyContext).
func (s *Server) Run(ctx context.Context, ln net.Listener) error {
	s.logger.Info("server starting", "address", ln.Addr().String())

	serveCh := make(chan error, 1)
	go func() {
		if err := s.srv.Serve(ln); err != nil && err != http.ErrServerClosed {
			serveCh <- err
		}
		close(serveCh)
	}()

	select {
	case <-ctx.Done():
	case err := <-serveCh:
		if err != nil {
			return fmt.Errorf("serving on %s: %w", ln.Addr(), err)
		}
	}

	s.logger.Info("shutting down server")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), s.cfg.Server.ShutdownTimeout)
	defer shutdownCancel()

	if err := s.srv.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("shutting down server: %w", err)
	}
	return nil
}
