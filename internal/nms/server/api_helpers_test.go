// Contains helper functions for testing the server
package server_test

import (
	"net/http/httptest"

	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/nms/server"
)

func setupServer(filepath string) (*httptest.Server, error) {
	testdb, err := db.NewDatabase(filepath)
	if err != nil {
		return nil, err
	}
	err = testdb.InitializeNetwork()
	if err != nil {
		return nil, err
	}
	jwtSecretStr := "testsecret"
	jwtSecret := []byte(jwtSecretStr)
	ts := httptest.NewTLSServer(server.NewHandler(testdb, jwtSecret))
	return ts, nil
}
