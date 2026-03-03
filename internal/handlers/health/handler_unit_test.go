package health_test

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/dcm-project/k8s-container-service-provider/internal/handlers/health"
)

var _ = Describe("Health Handler", func() {

	// TC-U005: Returns 200 OK with correct body and content type
	// Validates: REQ-HLT-010, REQ-HLT-020, REQ-HLT-030
	// Transitively covers: TC-U007 (REQ-HLT-040 — handler has no external
	// dependencies; it is constructed with only a start time and version string)
	It("returns 200 OK with correct body and content type (TC-U005)", func() {
		logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
		handler := health.NewHandler(time.Now(), "1.0.0", logger)

		req := httptest.NewRequest(http.MethodGet, "/health", nil)
		rec := httptest.NewRecorder()

		handler.GetHealth(rec, req)

		// REQ-HLT-010: HTTP 200 OK
		Expect(rec.Code).To(Equal(http.StatusOK))

		// REQ-HLT-030: Content-Type: application/json
		Expect(rec.Header().Get("Content-Type")).To(Equal("application/json"))

		// REQ-HLT-020: JSON body with required fields
		var body map[string]any
		err := json.Unmarshal(rec.Body.Bytes(), &body)
		Expect(err).NotTo(HaveOccurred(), "response body must be valid JSON")

		Expect(body).To(HaveKeyWithValue("status", "healthy"))
		Expect(body).To(HaveKeyWithValue("type", "k8s-container-service-provider.dcm.io/health"))
		Expect(body).To(HaveKeyWithValue("path", "health"))
		Expect(body).To(HaveKeyWithValue("version", "1.0.0"))

		// uptime must be an integer >= 0
		Expect(body).To(HaveKey("uptime"))
		uptime, ok := body["uptime"].(float64) // JSON numbers decode as float64
		Expect(ok).To(BeTrue(), "uptime must be a number")
		Expect(int(uptime)).To(BeNumerically(">=", 0))
	})

	// TC-U006: Uptime increases over time
	// Validates: REQ-HLT-020
	It("reports uptime increasing over time (TC-U006)", func() {
		logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
		startTime := time.Now().Add(-60 * time.Second)
		handler := health.NewHandler(startTime, "1.0.0", logger)

		req := httptest.NewRequest(http.MethodGet, "/health", nil)
		rec := httptest.NewRecorder()

		handler.GetHealth(rec, req)

		var body map[string]any
		err := json.Unmarshal(rec.Body.Bytes(), &body)
		Expect(err).NotTo(HaveOccurred(), "response body must be valid JSON")

		Expect(body).To(HaveKey("uptime"))
		uptime, ok := body["uptime"].(float64)
		Expect(ok).To(BeTrue(), "uptime must be a number")
		Expect(int(uptime)).To(BeNumerically(">=", 60))
	})
})
