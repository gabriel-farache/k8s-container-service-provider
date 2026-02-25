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
	}

	BeforeEach(func() {
		clearEnv()
	})

	AfterEach(func() {
		clearEnv()
	})

	// TC-U002: Load configuration from environment variables
	It("loads configuration from environment variables (TC-U002)", func() {
		os.Setenv("SP_SERVER_ADDRESS", ":9090")
		os.Setenv("SP_SERVER_SHUTDOWN_TIMEOUT", "30s")

		cfg, err := config.Load()
		Expect(err).NotTo(HaveOccurred())
		Expect(cfg).NotTo(BeNil())
		Expect(cfg.Server.Address).To(Equal(":9090"))
		Expect(cfg.Server.ShutdownTimeout).To(Equal(30 * time.Second))
	})

	// TC-U004: Default values applied when no config specified
	It("applies default values when no config is specified (TC-U004)", func() {
		cfg, err := config.Load()
		Expect(err).NotTo(HaveOccurred())
		Expect(cfg).NotTo(BeNil())
		Expect(cfg.Server.Address).To(Equal(":8080"))
		Expect(cfg.Server.ShutdownTimeout).To(Equal(15 * time.Second))
	})
})
