package http_test

import (
	"fmt"
	"io"
	"math/rand"
	"net"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/hovanhoa/llmgateway/pkg/core/errors"
	"github.com/hovanhoa/llmgateway/pkg/core/http"
	"github.com/stretchr/testify/assert"
)

var (
	mu    sync.Mutex
	ports = map[int]bool{}
)

func TestService_WithHealthCheck(t *testing.T) {
	port := allocatePort()
	t.Cleanup(func() {
		deallocatePort(port)
	})

	s := http.NewService(
		http.WithHealthCheckFn(func() error {
			return errors.New("unhealthy")
		}),
		http.WithBindAddress(fmt.Sprintf(":%d", port)),
		http.WithMetricsBindAddress(":0"),
	)

	assert.NoError(t, s.Start())
	for i := 0; i < 10; i++ {
		assertUnhealthy(t, port, "unhealthy")
		time.Sleep(100 * time.Millisecond)
	}

	assert.NoError(t, s.GracefulStop())
}

func TestService_StartsUnhealthy(t *testing.T) {
	port := allocatePort()
	t.Cleanup(func() {
		deallocatePort(port)
	})

	s := http.NewService(
		http.WithBindAddress(fmt.Sprintf(":%d", port)),
		http.WithMetricsBindAddress(":0"),
	)

	go func() {
		time.Sleep(100 * time.Millisecond)
		assertUnhealthy(t, port, "server uninitialized")
	}()
	assert.NoError(t, s.Start())
	assertHealthy(t, port)

	assert.NoError(t, s.GracefulStop())
}

func TestService_StartError(t *testing.T) {
	s := http.NewService(
		http.WithBindAddress(":xyz"),
		http.WithMetricsBindAddress(":0"),
	)
	assert.Error(t, s.Start())
}

func TestService_GracefulStop(t *testing.T) {
	port := allocatePort()
	t.Cleanup(func() {
		deallocatePort(port)
	})

	requestStart := make(chan bool)
	requestDone := make(chan bool)
	shutdownDone := make(chan bool)

	s := http.NewService(
		http.WithBindAddress(fmt.Sprintf(":%d", port)),
		http.WithMetricsBindAddress(":0"),
	)

	s.Router().GET("/", func(c *http.Context) {
		requestStart <- true
		// Simulate a 1 second processing time for this request
		time.Sleep(1 * time.Second)
		c.JSON(http.StatusOK, map[string]bool{"Status": true})
	})

	assert.NoError(t, s.Start())

	// This request should succeed even after graceful stop is called
	go func() {
		res, err := http.NewClient().Get(fmt.Sprintf("http://localhost:%d/", port))
		assert.NoError(t, err)
		assert.NotNil(t, res)
		assert.Equal(t, http.StatusOK, res.StatusCode)
		requestDone <- true
	}()

	// Wait for the request to start processing before graceful-stopping the server
	<-requestStart
	go func() {
		assert.NoError(t, s.GracefulStop())
		shutdownDone <- true
	}()

	_, err := http.NewClient().Get(fmt.Sprintf("http://localhost:%d/", port))
	assert.Error(t, err)

	<-requestDone
	<-shutdownDone
}

func assertUnhealthy(t *testing.T, port int, msg string) {
	res, err := http.NewClient().Get(fmt.Sprintf("http://localhost:%d/healthz", port))
	assert.NoError(t, err)
	assert.Equal(t, http.StatusServiceUnavailable, res.StatusCode)

	data, err := io.ReadAll(res.Body)
	assert.NoError(t, err)
	assert.NoError(t, res.Body.Close())
	assert.Equal(t, []byte(`{"Message":"`+msg+`","Status":false}`), data)
}

func assertHealthy(t *testing.T, port int) {
	res, err := http.NewClient().Get(fmt.Sprintf("http://localhost:%d/healthz", port))
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, res.StatusCode)

	data, err := io.ReadAll(res.Body)
	assert.NoError(t, err)
	assert.NoError(t, res.Body.Close())
	assert.Equal(t, []byte(`{"Message":"ok","Status":true}`), data)
}

// isPortInUse checks if the given port is available to listen on
func isPortInUse(port int) bool {
	ln, err := net.Listen("tcp", ":"+strconv.Itoa(port))

	defer func() {
		if ln != nil {
			_ = ln.Close()
		}
	}()

	return err != nil
}

// allocatePort returns an open port in the range [10000, 65536).
func allocatePort() int {
	mu.Lock()
	defer mu.Unlock()

	random := rand.New(rand.NewSource(time.Now().UnixNano()))
	port := 10000 + random.Intn(65536-10000)

	for ports[port] || isPortInUse(port) {
		port = 10000 + random.Intn(65536-10000)
	}

	return port
}

func deallocatePort(port int) {
	mu.Lock()
	defer mu.Unlock()
	delete(ports, port)
}
