package handlers_test

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/dcm-project/k8s-container-service-provider/api/v1alpha1"
	"github.com/dcm-project/k8s-container-service-provider/internal/handlers"
)

var _ = Describe("Composite Handler", func() {

	// TC-U064: GetHealth delegates to health sub-handler
	It("delegates GetHealth to health sub-handler (TC-U064)", func() {
		logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
		h := handlers.New(logger, time.Now(), "1.0.0")

		req := httptest.NewRequest(http.MethodGet, "/health", nil)
		rec := httptest.NewRecorder()

		h.GetHealth(rec, req)

		Expect(rec.Code).To(Equal(http.StatusOK))
		Expect(rec.Header().Get("Content-Type")).To(Equal("application/json"))

		var body map[string]any
		err := json.Unmarshal(rec.Body.Bytes(), &body)
		Expect(err).NotTo(HaveOccurred(), "response body must be valid JSON")
		Expect(body).To(HaveKeyWithValue("status", "healthy"))
		Expect(body).To(HaveKeyWithValue("version", "1.0.0"))
	})

	// TC-U065: Unimplemented endpoints return 501
	It("returns 501 for unimplemented endpoints (TC-U065)", func() {
		logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
		h := handlers.New(logger, time.Now(), "1.0.0")

		req := httptest.NewRequest(http.MethodGet, "/api/v1alpha1/containers", nil)
		rec := httptest.NewRecorder()

		h.ListContainers(rec, req, v1alpha1.ListContainersParams{})

		Expect(rec.Code).To(Equal(http.StatusNotImplemented))
	})
})
