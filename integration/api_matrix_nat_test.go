// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package integration_test

import (
	"context"
	"testing"

	"github.com/ellanetworks/core/client"
)

func runNATMatrix(ctx context.Context, t *testing.T, c *client.Client) {
	origNAT, origLocalSwitch, err := defaultDataNetworkForwarding(ctx, c)
	if err != nil {
		t.Fatalf("baseline: %v", err)
	}

	t.Cleanup(func() {
		if err := updateDefaultDataNetworkForwarding(ctx, c, &origNAT, &origLocalSwitch); err != nil {
			t.Logf("cleanup: restore NAT and local switching: %v", err)
		}
	})

	targetNAT, targetLocalSwitch := !origNAT, !origLocalSwitch

	if err := updateDefaultDataNetworkForwarding(ctx, c, &targetNAT, &targetLocalSwitch); err != nil {
		t.Fatalf("update NAT and local switching: %v", err)
	}

	gotNAT, gotLocalSwitch, err := defaultDataNetworkForwarding(ctx, c)
	if err != nil {
		t.Fatalf("after update: %v", err)
	}

	if gotNAT != targetNAT || gotLocalSwitch != targetLocalSwitch {
		t.Fatalf("NAT %t, local switch %t; want %t, %t", gotNAT, gotLocalSwitch, targetNAT, targetLocalSwitch)
	}
}
