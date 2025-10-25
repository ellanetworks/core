// start_integration_test.go
package api

import (
	"context"
	"io"
	"io/fs"
	"net"
	"net/http"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/kernel"
)

// freePort finds an available port on localhost.
func freePort(t *testing.T) int {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to get free port: %v", err)
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port
}

type dummyFS struct{}

func (dummyFS) Open(name string) (fs.File, error) {
	return nil, fs.ErrNotExist
}

func TestStartServerStandup(t *testing.T) {
	// Override routeReconciler to a no-op to avoid actual route reconciliation.
	origReconciler := routeReconciler
	routeReconciler = func(dbInstance *db.Database, kernelInt kernel.Kernel) error {
		return nil
	}
	defer func() { routeReconciler = origReconciler }()

	// Use HTTP scheme for testing.
	scheme := HTTP

	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "db.sqlite3")

	testdb, err := db.NewDatabase(dbPath)
	if err != nil {
		t.Fatalf("could not create new database: %v", err)
	}

	address := "127.0.0.1"
	port := freePort(t)
	// For HTTP, these cert/key files are unused.
	certFile := "dummy_cert.pem"
	keyFile := "dummy_key.pem"
	n3Interface := "eth0"
	n6Interface := "eth1"

	// Start the server in a separate goroutine.
	dummyFS := dummyFS{}
	if err := Start(testdb, nil, address, port, scheme, certFile, keyFile, n3Interface, n6Interface, false, dummyFS, nil); err != nil {
		t.Fatalf("Start returned error: %v", err)
	}

	// Poll the server until it responds or timeout occurs.
	baseURL := "http://127.0.0.1:" + strconv.Itoa(port)
	client := &http.Client{}
	var resp *http.Response
	var lastErr error
	timeout := time.Now().Add(5 * time.Second)
	for time.Now().Before(timeout) {
		req, reqErr := http.NewRequestWithContext(context.Background(), "GET", baseURL+"/", nil)
		if reqErr != nil {
			lastErr = reqErr
			time.Sleep(100 * time.Millisecond)
			continue
		}
		resp, err = client.Do(req)
		if err == nil {
			break
		}
		lastErr = err
		time.Sleep(100 * time.Millisecond)
	}
	if err != nil {
		t.Fatalf("failed to reach server: %v", lastErr)
	}
	defer resp.Body.Close()

	// Read and log the response.
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("failed to read response body: %v", err)
	}
	t.Logf("Server is up. Response status: %s, body: %s", resp.Status, string(body))
}
