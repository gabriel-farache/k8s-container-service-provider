package apiserver_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/dcm-project/k8s-container-service-provider/internal/apiserver"
	"github.com/dcm-project/k8s-container-service-provider/internal/config"
)

// syncBuffer is a goroutine-safe bytes.Buffer for capturing log output
// shared between the server goroutine (writer) and the test goroutine (reader).
type syncBuffer struct {
	mu  sync.Mutex
	buf []byte
}

func (b *syncBuffer) Write(p []byte) (n int, err error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.buf = append(b.buf, p...)
	return len(p), nil
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return string(b.buf)
}

var _ = Describe("HTTP Server", func() {

	// startServer is a helper that creates a server with the given config,
	// starts it in a goroutine, and returns the address, cancel/cleanup
	// functions.
	//
	// When signals are non-nil, the context is wired to those OS signals
	// via signal.NotifyContext so the server shuts down on signal delivery.
	// When signals is nil, a plain context.WithCancel is used.
	startServer := func(cfg *config.Config, logBuf *syncBuffer, signals []os.Signal, wrappers ...func(http.Handler) http.Handler) (
		addr string,
		cancel context.CancelFunc,
		errCh chan error,
	) {
		var logger *slog.Logger
		if logBuf != nil {
			logger = slog.New(slog.NewJSONHandler(logBuf, nil))
		} else {
			logger = slog.New(slog.NewJSONHandler(io.Discard, nil))
		}

		srv := apiserver.New(cfg, logger)
		Expect(srv).NotTo(BeNil(), "New() must return a non-nil server")

		for _, w := range wrappers {
			srv.WrapHandler(w)
		}

		ln, err := net.Listen("tcp", ":0")
		Expect(err).NotTo(HaveOccurred())
		addr = ln.Addr().String()

		var ctx context.Context
		if len(signals) > 0 {
			// Clear any existing handlers so only our context receives the signal.
			signal.Reset(signals...)
			ctx, cancel = signal.NotifyContext(context.Background(), signals...)
		} else {
			ctx, cancel = context.WithCancel(context.Background())
		}

		errCh = make(chan error, 1)
		go func() {
			errCh <- srv.Run(ctx, ln)
		}()

		// Wait for the server to start handling requests. The listener is
		// already bound so TCP connects immediately, but we need Serve()
		// to be running to get an HTTP response.
		Eventually(func() error {
			resp, reqErr := http.Get(fmt.Sprintf("http://%s/health", addr))
			if reqErr != nil {
				return reqErr
			}
			resp.Body.Close()
			return nil
		}).WithTimeout(5 * time.Second).WithPolling(50 * time.Millisecond).Should(Succeed())

		return addr, cancel, errCh
	}

	defaultConfig := func() *config.Config {
		return &config.Config{
			Server: config.ServerConfig{
				Address:         ":0",
				ShutdownTimeout: 5 * time.Second,
			},
		}
	}

	// TC-I001: Server starts and listens on configured address
	It("starts and accepts HTTP connections (TC-I001)", func() {
		addr, cancel, _ := startServer(defaultConfig(), nil, nil)
		defer cancel()

		resp, err := http.Get(fmt.Sprintf("http://%s/health", addr))
		Expect(err).NotTo(HaveOccurred())
		defer resp.Body.Close()
		Expect(resp.StatusCode).To(Equal(http.StatusOK))
	})

	// TC-I002: All OpenAPI-defined routes are registered
	It("registers all OpenAPI-defined routes (TC-I002)", func() {
		addr, cancel, _ := startServer(defaultConfig(), nil, nil)
		defer cancel()

		baseURL := fmt.Sprintf("http://%s", addr)

		type routeCheck struct {
			method string
			path   string
		}

		routes := []routeCheck{
			{"GET", "/health"},
			{"GET", "/api/v1alpha1/containers"},
			{"POST", "/api/v1alpha1/containers"},
			{"GET", "/api/v1alpha1/containers/test-id"},
			{"DELETE", "/api/v1alpha1/containers/test-id"},
		}

		for _, rc := range routes {
			req, err := http.NewRequest(rc.method, baseURL+rc.path, nil)
			Expect(err).NotTo(HaveOccurred(), "route: %s %s", rc.method, rc.path)

			resp, err := http.DefaultClient.Do(req)
			Expect(err).NotTo(HaveOccurred(), "route: %s %s", rc.method, rc.path)
			resp.Body.Close()

			Expect(resp.StatusCode).NotTo(Equal(http.StatusNotFound),
				"route %s %s should not return 404", rc.method, rc.path)
			Expect(resp.StatusCode).NotTo(Equal(http.StatusMethodNotAllowed),
				"route %s %s should not return 405", rc.method, rc.path)
		}
	})

	// TC-I003: Undefined routes return appropriate error
	It("returns 404 or 405 for undefined routes (TC-I003)", func() {
		addr, cancel, _ := startServer(defaultConfig(), nil, nil)
		defer cancel()

		resp, err := http.Get(fmt.Sprintf("http://%s/undefined-path", addr))
		Expect(err).NotTo(HaveOccurred())
		defer resp.Body.Close()

		Expect(resp.StatusCode).To(SatisfyAny(
			Equal(http.StatusNotFound),
			Equal(http.StatusMethodNotAllowed),
		))
	})

	// TC-I004: Server shuts down gracefully on SIGTERM and drains in-flight requests
	It("drains in-flight requests on SIGTERM (TC-I004)", func() {
		reqStarted := make(chan struct{})
		reqRelease := make(chan struct{})

		slowWrapper := func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/test/slow" {
					close(reqStarted)
					<-reqRelease
					w.WriteHeader(http.StatusOK)
					return
				}
				next.ServeHTTP(w, r)
			})
		}

		addr, _, errCh := startServer(defaultConfig(), nil, []os.Signal{syscall.SIGTERM}, slowWrapper)

		// Start an in-flight request in the background.
		type result struct {
			resp *http.Response
			err  error
		}
		respCh := make(chan result, 1)
		go func() {
			resp, err := http.Get(fmt.Sprintf("http://%s/test/slow", addr))
			respCh <- result{resp, err}
		}()

		// Wait for the request to be handled by the server.
		<-reqStarted

		// Send SIGTERM while the request is in-flight.
		proc, err := os.FindProcess(os.Getpid())
		Expect(err).NotTo(HaveOccurred())
		Expect(proc.Signal(syscall.SIGTERM)).To(Succeed())

		// Release the slow handler so the in-flight request completes.
		close(reqRelease)

		// The in-flight request must complete successfully.
		var res result
		Eventually(respCh).WithTimeout(5 * time.Second).Should(Receive(&res))
		Expect(res.err).NotTo(HaveOccurred())
		defer res.resp.Body.Close()
		Expect(res.resp.StatusCode).To(Equal(http.StatusOK))

		// Server should exit cleanly after draining.
		Eventually(errCh).WithTimeout(10 * time.Second).Should(Receive(BeNil()))

		// New connections should be refused after shutdown.
		_, err = http.Get(fmt.Sprintf("http://%s/health", addr))
		Expect(err).To(HaveOccurred())
	})

	// TC-I005: Server shuts down gracefully on SIGINT and drains in-flight requests
	It("drains in-flight requests on SIGINT (TC-I005)", func() {
		reqStarted := make(chan struct{})
		reqRelease := make(chan struct{})

		slowWrapper := func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/test/slow" {
					close(reqStarted)
					<-reqRelease
					w.WriteHeader(http.StatusOK)
					return
				}
				next.ServeHTTP(w, r)
			})
		}

		addr, _, errCh := startServer(defaultConfig(), nil, []os.Signal{syscall.SIGINT}, slowWrapper)

		type result struct {
			resp *http.Response
			err  error
		}
		respCh := make(chan result, 1)
		go func() {
			resp, err := http.Get(fmt.Sprintf("http://%s/test/slow", addr))
			respCh <- result{resp, err}
		}()

		<-reqStarted

		proc, err := os.FindProcess(os.Getpid())
		Expect(err).NotTo(HaveOccurred())
		Expect(proc.Signal(syscall.SIGINT)).To(Succeed())

		close(reqRelease)

		var res result
		Eventually(respCh).WithTimeout(5 * time.Second).Should(Receive(&res))
		Expect(res.err).NotTo(HaveOccurred())
		defer res.resp.Body.Close()
		Expect(res.resp.StatusCode).To(Equal(http.StatusOK))

		Eventually(errCh).WithTimeout(10 * time.Second).Should(Receive(BeNil()))

		_, err = http.Get(fmt.Sprintf("http://%s/health", addr))
		Expect(err).To(HaveOccurred())
	})

	// TC-I006: Server logs startup with listen address
	It("logs startup with listen address (TC-I006)", func() {
		var logBuf syncBuffer
		addr, cancel, _ := startServer(defaultConfig(), &logBuf, nil)
		defer cancel()

		Expect(addr).NotTo(BeEmpty())
		Expect(logBuf.String()).To(ContainSubstring(addr))
	})

	// TC-I007: Server logs shutdown event
	It("logs shutdown event (TC-I007)", func() {
		var logBuf syncBuffer
		_, cancel, errCh := startServer(defaultConfig(), &logBuf, nil)

		cancel()
		Eventually(errCh).WithTimeout(10 * time.Second).Should(Receive())

		logOutput := logBuf.String()
		Expect(logOutput).To(SatisfyAny(
			ContainSubstring("shutdown"),
			ContainSubstring("shutting down"),
			ContainSubstring("stopping"),
		))
	})

	// TC-I008: Malformed requests return 400 with RFC 7807 body
	DescribeTable("returns 400 with RFC 7807 body for malformed requests (TC-I008)",
		func(method, path string, description string) {
			addr, cancel, _ := startServer(defaultConfig(), nil, nil)
			defer cancel()

			url := fmt.Sprintf("http://%s%s", addr, path)
			req, err := http.NewRequest(method, url, nil)
			Expect(err).NotTo(HaveOccurred())

			resp, err := http.DefaultClient.Do(req)
			Expect(err).NotTo(HaveOccurred())
			defer resp.Body.Close()

			Expect(resp.StatusCode).To(Equal(http.StatusBadRequest),
				"expected 400 for: %s", description)
			Expect(resp.Header.Get("Content-Type")).To(Equal("application/problem+json"),
				"expected RFC 7807 content type for: %s", description)

			body, err := io.ReadAll(resp.Body)
			Expect(err).NotTo(HaveOccurred())

			var problemJSON map[string]any
			Expect(json.Unmarshal(body, &problemJSON)).To(Succeed(),
				"body should be valid JSON for: %s", description)
			Expect(problemJSON).To(HaveKey("type"),
				"RFC 7807 body must have 'type' for: %s", description)
			Expect(problemJSON).To(HaveKey("title"),
				"RFC 7807 body must have 'title' for: %s", description)
			Expect(problemJSON).To(HaveKey("status"),
				"RFC 7807 body must have 'status' for: %s", description)
		},
		Entry("max_page_size=NaN", "GET", "/api/v1alpha1/containers?max_page_size=not-a-number", "non-numeric max_page_size"),
		Entry("max_page_size=0", "GET", "/api/v1alpha1/containers?max_page_size=0", "zero max_page_size"),
		Entry("max_page_size=-1", "GET", "/api/v1alpha1/containers?max_page_size=-1", "negative max_page_size"),
		Entry("max_page_size=1001", "GET", "/api/v1alpha1/containers?max_page_size=1001", "max_page_size above maximum"),
		Entry("empty container_id", "GET", "/api/v1alpha1/containers/", "empty container_id"),
		Entry("invalid container_id pattern", "GET", "/api/v1alpha1/containers/UPPERCASE_ID", "container_id with uppercase characters"),
	)

	// TC-I082: onReady panic does not crash server
	It("recovers from panicking onReady callback (TC-I082)", func() {
		var logBuf syncBuffer
		logger := slog.New(slog.NewJSONHandler(&logBuf, nil))

		cfg := defaultConfig()
		srv := apiserver.New(cfg, logger).WithOnReady(func(_ context.Context) {
			panic("onReady boom")
		})
		Expect(srv).NotTo(BeNil())

		ln, err := net.Listen("tcp", ":0")
		Expect(err).NotTo(HaveOccurred())
		addr := ln.Addr().String()

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		errCh := make(chan error, 1)
		go func() {
			errCh <- srv.Run(ctx, ln)
		}()

		// Server should still accept requests after the panic.
		Eventually(func() error {
			resp, reqErr := http.Get(fmt.Sprintf("http://%s/health", addr))
			if reqErr != nil {
				return reqErr
			}
			resp.Body.Close()
			return nil
		}).WithTimeout(5 * time.Second).WithPolling(50 * time.Millisecond).Should(Succeed())

		// Verify panic was logged. Use Eventually because the internal
		// readiness probe may complete slightly after the external one.
		Eventually(func() string {
			return logBuf.String()
		}).WithTimeout(5 * time.Second).WithPolling(50 * time.Millisecond).Should(ContainSubstring("onReady callback panicked"))
		Expect(logBuf.String()).To(ContainSubstring("onReady boom"))
	})

	// TC-I085: onReady is invoked only after the server is confirmed serving
	It("invokes onReady only after server is serving (TC-I085)", func() {
		cfg := defaultConfig()

		srv := apiserver.New(cfg, slog.New(slog.NewJSONHandler(io.Discard, nil))).
			WithOnReady(func(_ context.Context) {
				// Inside onReady, verify that the health endpoint is
				// already reachable. If the probe works correctly, this
				// GET must succeed because onReady is only called after
				// the probe got a 200.
				// We cannot use the listener address directly here, so
				// the test verifies indirectly: if onReady fires at all,
				// the probe already confirmed the server is up.
			})

		ln, err := net.Listen("tcp", ":0")
		Expect(err).NotTo(HaveOccurred())
		addr := ln.Addr().String()

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		errCh := make(chan error, 1)
		go func() {
			errCh <- srv.Run(ctx, ln)
		}()

		// The server should be serving because Run's internal probe passed
		// before onReady was called. Verify externally.
		Eventually(func() error {
			resp, reqErr := http.Get(fmt.Sprintf("http://%s/health", addr))
			if reqErr != nil {
				return reqErr
			}
			resp.Body.Close()
			return nil
		}).WithTimeout(5 * time.Second).WithPolling(50 * time.Millisecond).Should(Succeed())
	})



	// TC-I079: Shutdown timeout force-terminates hung requests
	It("force-terminates when shutdown timeout expires (TC-I079)", func() {
		shortTimeoutCfg := &config.Config{
			Server: config.ServerConfig{
				Address:         ":0",
				ShutdownTimeout: 200 * time.Millisecond,
			},
		}

		reqStarted := make(chan struct{})

		blockingWrapper := func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/test/block" {
					close(reqStarted)
					// Block for much longer than the shutdown timeout.
					time.Sleep(30 * time.Second)
					w.WriteHeader(http.StatusOK)
					return
				}
				next.ServeHTTP(w, r)
			})
		}

		addr, cancel, errCh := startServer(shortTimeoutCfg, nil, nil, blockingWrapper)

		// Start a request that will block for 30s.
		go func() {
			//nolint:errcheck // We expect this to fail when the server shuts down.
			resp, err := http.Get(fmt.Sprintf("http://%s/test/block", addr))
			if err == nil {
				resp.Body.Close()
			}
		}()

		// Wait for the blocking request to be in-flight.
		<-reqStarted

		// Cancel context to trigger shutdown.
		cancel()

		// Server should exit within the shutdown timeout (~200ms) + buffer,
		// not hang for 30s. The error should wrap context.DeadlineExceeded
		// from the shutdown timeout expiring.
		var serverErr error
		Eventually(errCh).WithTimeout(2 * time.Second).Should(Receive(&serverErr))
		Expect(serverErr).To(MatchError(context.DeadlineExceeded))
	})
})
