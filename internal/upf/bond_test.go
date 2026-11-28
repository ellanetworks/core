// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: Apache-2.0

package upf

import (
	"context"
	"net"
	"os/exec"
	"slices"
	"testing"

	"github.com/cilium/ebpf/link"
	"github.com/ellanetworks/core/internal/config"
)

const testBond = "ellbtst0"

func runIP(t *testing.T, args ...string) {
	t.Helper()

	if out, err := exec.CommandContext(context.Background(), "ip", args...).CombinedOutput(); err != nil {
		t.Fatalf("ip %v: %v: %s", args, err, out)
	}
}

func deleteLink(name string) {
	_ = exec.CommandContext(context.Background(), "ip", "link", "del", name).Run()
}

func addBondMember(t *testing.T, bond, member, peer string) {
	t.Helper()

	deleteLink(member)
	runIP(t, "link", "add", member, "type", "veth", "peer", "name", peer)
	t.Cleanup(func() { deleteLink(member) })
	runIP(t, "link", "set", member, "master", bond)

	if out, err := exec.CommandContext(context.Background(), "ethtool", "-K", member, "gro", "on").CombinedOutput(); err != nil {
		t.Fatalf("enable GRO on %s: %v: %s", member, err, out)
	}
}

func requireGROOff(t *testing.T, devices ...string) {
	t.Helper()

	for _, device := range devices {
		enabled, err := interfaceGROEnabled(device)
		if err != nil {
			t.Fatalf("read GRO on %s: %v", device, err)
		}

		if enabled {
			t.Fatalf("GRO is still on for %s", device)
		}
	}
}

func setupTestBond(t *testing.T, members ...string) string {
	t.Helper()

	requireRoot(t)

	deleteLink(testBond)
	runIP(t, "link", "add", testBond, "type", "bond", "mode", "active-backup", "miimon", "100")
	t.Cleanup(func() { deleteLink(testBond) })

	for i, member := range members {
		addBondMember(t, testBond, member, "ellbtp"+string(rune('0'+i)))
	}

	runIP(t, "link", "set", testBond, "up")

	vlan := testBond + ".7"
	runIP(t, "link", "add", "link", testBond, "name", vlan, "type", "vlan", "id", "7")

	return vlan
}

func TestBondMembersReceiveForTheBond(t *testing.T) {
	members := []string{"ellbtm0", "ellbtm1"}

	vlan := setupTestBond(t, members...)

	for _, ifname := range []string{testBond, vlan} {
		got := receivingDevices(ifname)
		slices.Sort(got)

		if !slices.Equal(got, members) {
			t.Fatalf("receivingDevices(%s) = %v, want %v", ifname, got, members)
		}
	}

	if got := receivingDevices(members[0]); !slices.Equal(got, []string{members[0]}) {
		t.Fatalf("receivingDevices(%s) = %v, want the device itself", members[0], got)
	}

	if !nativeXDPBlackholes(testBond) {
		t.Fatalf("nativeXDPBlackholes(%s) = false, want true for a bond of veth members", testBond)
	}

	if got := mergingDevices(config.DatapathXDPNative, vlan); len(got) != 0 {
		t.Fatalf("mergingDevices(xdp-native) = %v, want none", got)
	}

	if got := mergingDevices(config.DatapathXDPGeneric, vlan); !slices.Contains(got, members[0]) || slices.Contains(got, vlan) {
		t.Fatalf("mergingDevices(xdp-generic, %s) = %v, want the members without the attached interface", vlan, got)
	}

	disableMergedPackets(config.DatapathTCX, testBond)

	requireGROOff(t, members...)
}

func TestLinkWatchTurnsOffMergingOnNewMembers(t *testing.T) {
	for _, mode := range []string{config.DatapathTCX, config.DatapathXDPGeneric} {
		t.Run(mode, func(t *testing.T) {
			vlan := setupTestBond(t, "ellbtm0")

			datapath := ifByNameForTest(t, vlan)

			u := &UPF{
				attachedMode:  mode,
				n3Ifindex:     datapath,
				n6Ifindex:     datapath,
				instanceLinks: map[int]link.Link{},
			}

			disableMergedPackets(mode, vlan)
			u.seedMerging()

			addBondMember(t, testBond, "ellbtm1", "ellbtp1")

			if enabled, err := interfaceGROEnabled("ellbtm1"); err != nil || !enabled {
				t.Fatalf("GRO on the new member before refresh = %t, %v; want on", enabled, err)
			}

			u.refreshLinks()

			requireGROOff(t, "ellbtm1")

			if out, err := exec.CommandContext(context.Background(), "ethtool", "-K", "ellbtm1", "gro", "on").CombinedOutput(); err != nil {
				t.Fatalf("re-enable GRO: %v: %s", err, out)
			}

			u.refreshLinks()

			requireGROOff(t, "ellbtm1")
		})
	}
}

func ifByNameForTest(t *testing.T, name string) int {
	t.Helper()

	iface, err := net.InterfaceByName(name)
	if err != nil {
		t.Fatalf("lookup %s: %v", name, err)
	}

	return iface.Index
}
