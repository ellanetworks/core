// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: Apache-2.0

package upf

import (
	"context"
	"os/exec"
	"slices"
	"testing"

	"github.com/ellanetworks/core/internal/config"
)

func runIP(t *testing.T, args ...string) {
	t.Helper()

	if out, err := exec.CommandContext(context.Background(), "ip", args...).CombinedOutput(); err != nil {
		t.Fatalf("ip %v: %v: %s", args, err, out)
	}
}

func TestBondMembersReceiveForTheBond(t *testing.T) {
	const bond = "ellbtst0"

	members := []string{"ellbtm0", "ellbtm1"}

	requireRoot(t)

	_ = exec.CommandContext(context.Background(), "ip", "link", "del", bond).Run()

	runIP(t, "link", "add", bond, "type", "bond", "mode", "active-backup", "miimon", "100")
	t.Cleanup(func() { _ = exec.CommandContext(context.Background(), "ip", "link", "del", bond).Run() })

	for i, member := range members {
		peer := "ellbtp" + string(rune('0'+i))

		_ = exec.CommandContext(context.Background(), "ip", "link", "del", member).Run()
		runIP(t, "link", "add", member, "type", "veth", "peer", "name", peer)
		t.Cleanup(func() { _ = exec.CommandContext(context.Background(), "ip", "link", "del", member).Run() })
		runIP(t, "link", "set", member, "master", bond)

		if out, err := exec.CommandContext(context.Background(), "ethtool", "-K", member, "gro", "on").CombinedOutput(); err != nil {
			t.Fatalf("enable GRO on %s: %v: %s", member, err, out)
		}
	}

	runIP(t, "link", "set", bond, "up")

	got := bondMembers(bond)
	slices.Sort(got)

	if !slices.Equal(got, members) {
		t.Fatalf("bondMembers(%s) = %v, want %v", bond, got, members)
	}

	if got := receivingDevices(members[0]); !slices.Equal(got, []string{members[0]}) {
		t.Fatalf("receivingDevices(%s) = %v, want the device itself", members[0], got)
	}

	if !nativeXDPBlackholes(bond) {
		t.Fatalf("nativeXDPBlackholes(%s) = false, want true for a bond of veth members", bond)
	}

	disableMergedPackets(config.DatapathTCX, bond)

	for _, member := range members {
		enabled, err := interfaceGROEnabled(member)
		if err != nil {
			t.Fatalf("read GRO on %s: %v", member, err)
		}

		if enabled {
			t.Fatalf("GRO is still on for bond member %s", member)
		}
	}
}
