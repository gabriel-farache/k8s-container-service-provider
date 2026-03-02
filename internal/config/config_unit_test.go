package config_test

import (
	"os"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/dcm-project/k8s-container-service-provider/internal/config"
)

var _ = Describe("Configuration", func() {

	// Helper to unset all config-related env vars between tests.
	clearEnv := func() {
		os.Unsetenv("SP_SERVER_ADDRESS")
		os.Unsetenv("SP_SERVER_SHUTDOWN_TIMEOUT")
		os.Unsetenv("SP_PROVIDER_NAME")
		os.Unsetenv("SP_PROVIDER_DISPLAY_NAME")
		os.Unsetenv("SP_PROVIDER_ENDPOINT")
		os.Unsetenv("SP_PROVIDER_REGION")
		os.Unsetenv("SP_PROVIDER_ZONE")
		os.Unsetenv("SP_DCM_REGISTRATION_URL")
	}

	BeforeEach(func() {
		clearEnv()
	})

	AfterEach(func() {
		clearEnv()
	})

	// setRequiredEnv sets the mandatory env vars so Load() succeeds.
	setRequiredEnv := func() {
		os.Setenv("SP_PROVIDER_NAME", "test-sp")
		os.Setenv("SP_PROVIDER_ENDPOINT", "https://test.example.com")
		os.Setenv("SP_DCM_REGISTRATION_URL", "https://dcm.example.com")
	}

	// TC-U002: Load configuration from environment variables
	It("loads configuration from environment variables (TC-U002)", func() {
		setRequiredEnv()
		os.Setenv("SP_SERVER_ADDRESS", ":9090")
		os.Setenv("SP_SERVER_SHUTDOWN_TIMEOUT", "30s")
		os.Setenv("SP_PROVIDER_DISPLAY_NAME", "Test Provider")
		os.Setenv("SP_PROVIDER_REGION", "us-east-1")
		os.Setenv("SP_PROVIDER_ZONE", "us-east-1a")

		cfg, err := config.Load()
		Expect(err).NotTo(HaveOccurred())
		Expect(cfg).NotTo(BeNil())
		Expect(cfg.Server.Address).To(Equal(":9090"))
		Expect(cfg.Server.ShutdownTimeout).To(Equal(30 * time.Second))
		Expect(cfg.Provider.Name).To(Equal("test-sp"))
		Expect(cfg.Provider.DisplayName).To(Equal("Test Provider"))
		Expect(cfg.Provider.Endpoint).To(Equal("https://test.example.com"))
		Expect(cfg.Provider.Region).To(Equal("us-east-1"))
		Expect(cfg.Provider.Zone).To(Equal("us-east-1a"))
		Expect(cfg.DCM.RegistrationURL).To(Equal("https://dcm.example.com"))
	})

	// TC-U004: Default values applied when no config specified
	It("applies default values when no config is specified (TC-U004)", func() {
		setRequiredEnv()

		cfg, err := config.Load()
		Expect(err).NotTo(HaveOccurred())
		Expect(cfg).NotTo(BeNil())
		Expect(cfg.Server.Address).To(Equal(":8080"))
		Expect(cfg.Server.ShutdownTimeout).To(Equal(15 * time.Second))
	})

	// TC-U063: Load returns error when required fields are missing
	It("returns error when required fields are missing (TC-U063)", func() {
		// All required env vars unset (clearEnv called in BeforeEach).
		cfg, err := config.Load()
		Expect(err).To(HaveOccurred())
		Expect(cfg).To(BeNil())

		errMsg := err.Error()
		Expect(errMsg).To(ContainSubstring("SP_PROVIDER_NAME"))
		Expect(errMsg).To(ContainSubstring("SP_PROVIDER_ENDPOINT"))
		Expect(errMsg).To(ContainSubstring("SP_DCM_REGISTRATION_URL"))
	})
})
