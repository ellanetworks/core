// Copyright 2026 Ella Networks

package server_test

import (
	"testing"

	"github.com/ellanetworks/core/internal/api/server"
)

func TestMain(m *testing.M) {
	server.RegisterMetrics()

	m.Run()
}
