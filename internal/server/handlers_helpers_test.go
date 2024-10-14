// Contains helper functions for testing the server
package server_test

import (
	"net/http/httptest"

	"github.com/yeastengine/ella/internal/db/sql"
	"github.com/yeastengine/ella/internal/server"
)

func setupServer() (*httptest.Server, *server.HandlerConfig, error) {
	dbQueries, err := sql.Initialize(":memory:")
	if err != nil {
		return nil, nil, err
	}
	config := &server.HandlerConfig{
		DBQueries: dbQueries,
	}
	ts := httptest.NewTLSServer(server.NewEllaRouter(config))
	return ts, config, nil
}
