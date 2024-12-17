// Contains helper functions for testing the server
package server_test

import (
	"net/http/httptest"

	"github.com/yeastengine/ella/internal/db"
	"github.com/yeastengine/ella/internal/nms/server"
)

func setupServer(filepath string) (*httptest.Server, *db.Database, error) {
	testdb, err := db.NewDatabase(filepath)
	if err != nil {
		return nil, nil, err
	}
	ts := httptest.NewTLSServer(server.NewHandler(testdb))
	return ts, testdb, nil
}
