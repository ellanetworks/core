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

	natBaselines := make(map[int]bool, len(h.Clients))

	var localSwitchBaseline bool

	for i, c := range h.Clients {
		nat, localSwitch, err := defaultDataNetworkForwarding(ctx, c)
		if err != nil {
			t.Fatalf("node %d baseline: %v", i+1, err)
		}

		natBaselines[i] = nat
		localSwitchBaseline = localSwitch
	}

	t.Cleanup(func() {
		nat := natBaselines[writerIdx]
		if err := updateDefaultDataNetworkForwarding(ctx, writer, &nat, &localSwitchBaseline); err != nil {
			t.Logf("cleanup: restore NAT and local switching on writer: %v", err)
		}
	})

	targetNAT, targetLocalSwitch := !natBaselines[writerIdx], !localSwitchBaseline

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

		if nat != natBaselines[i] {
			t.Fatalf("node %d NAT drifted: got %t, want %t (set on node %d only)", i+1, nat, natBaselines[i], writerIdx+1)
		}

		if localSwitch != targetLocalSwitch {
			t.Fatalf("node %d local switch %t, want %t (replicated from node %d)", i+1, localSwitch, targetLocalSwitch, writerIdx+1)
		}
	}
}
