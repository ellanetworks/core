// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package integration_test

import (
	"context"
	"testing"
)

func runNATHAMatrix(ctx context.Context, t *testing.T, h *haMatrixEnv) {
	const writerIdx = 1

	writer := h.Clients[writerIdx]

	natBaseline, localSwitchBaseline, err := defaultDataNetworkForwarding(ctx, writer)
	if err != nil {
		t.Fatalf("writer baseline: %v", err)
	}

	t.Cleanup(func() {
		if err := updateDefaultDataNetworkForwarding(ctx, writer, &natBaseline, &localSwitchBaseline); err != nil {
			t.Logf("cleanup: restore NAT and local switching on writer: %v", err)
		}
	})

	targetNAT, targetLocalSwitch := !natBaseline, !localSwitchBaseline

	if err := updateDefaultDataNetworkForwarding(ctx, writer, &targetNAT, &targetLocalSwitch); err != nil {
		t.Fatalf("update on writer: %v", err)
	}

	gotNAT, gotLocalSwitch, err := defaultDataNetworkForwarding(ctx, writer)
	if err != nil {
		t.Fatalf("writer after update: %v", err)
	}

	if gotNAT != targetNAT || gotLocalSwitch != targetLocalSwitch {
		t.Fatalf("writer NAT %t, local switch %t; want %t, %t", gotNAT, gotLocalSwitch, targetNAT, targetLocalSwitch)
	}

	stabilizeLocal(t, ctx, h)

	for i, c := range h.Clients {
		if i == writerIdx {
			continue
		}

		nat, localSwitch, err := defaultDataNetworkForwarding(ctx, c)
		if err != nil {
			t.Fatalf("node %d after writer update: %v", i+1, err)
		}

		if nat != targetNAT || localSwitch != targetLocalSwitch {
			t.Fatalf("node %d NAT %t, local switch %t; want %t, %t (replicated from node %d)", i+1, nat, localSwitch, targetNAT, targetLocalSwitch, writerIdx+1)
		}
	}
}
