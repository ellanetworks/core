// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package integration_test

import (
	"context"
	"testing"

	"github.com/ellanetworks/core/integration/apimatrix"
	"github.com/ellanetworks/core/integration/suites"
)

func TestAPIMatrixHA(t *testing.T) {
	suites.Require(t, suites.APIMatrixHA)

	beginHATest(t)

	ctx := context.Background()
	h := setupHAMatrixEnv(ctx, t)

	for name, run := range apimatrix.HAResources {
		t.Run(name, func(t *testing.T) {
			run(ctx, t, h)
		})
	}
}

func setupHAMatrixEnv(ctx context.Context, t *testing.T) *apimatrix.HAEnv {
	t.Helper()

	dc, err := NewDockerClient()
	if err != nil {
		t.Fatalf("docker client: %v", err)
	}

	t.Cleanup(func() { _ = dc.Close() })

	clients, err := bringUpHACluster(t, ctx, dc)
	if err != nil {
		t.Fatalf("bring up HA cluster: %v", err)
	}

	t.Cleanup(func() {
		dumpClusterDiagnostics(t, ctx, dc, haComposeDir, haNodeServices, clients)
	})

	if err := waitForAllNodesReady(ctx, clients); err != nil {
		t.Fatalf("not all nodes ready: %v", err)
	}

	leaderIdx, leader, err := findLeader(ctx, clients)
	if err != nil {
		t.Fatalf("find leader: %v", err)
	}

	return &apimatrix.HAEnv{
		Clients:   clients,
		Leader:    leader,
		LeaderIdx: leaderIdx,
		IPv6:      DetectIPFamily() != IPv4Only,
	}
}
