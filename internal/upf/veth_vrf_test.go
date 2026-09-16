// SPDX-FileCopyrightText: Ella Networks Inc.

// SPDX-License-Identifier: BUSL-1.1

package upf

import (
	"errors"
	"testing"

	"github.com/vishvananda/netlink"
)

type vethStubTopo struct {
	byName   map[string]netlink.Link
	byIndex  map[int]netlink.Link
	mastered []string
}

func stubVethNetlink(t *testing.T, topo *vethStubTopo) {
	t.Helper()

	oldByName, oldByIndex, oldSetMaster := vethLinkByName, vethLinkByIndex, vethLinkSetMaster

	t.Cleanup(func() {
		vethLinkByName, vethLinkByIndex, vethLinkSetMaster = oldByName, oldByIndex, oldSetMaster
	})

	vethLinkByName = func(name string) (netlink.Link, error) {
		if l, ok := topo.byName[name]; ok {
			return l, nil
		}

		return nil, netlink.LinkNotFoundError{}
	}

	vethLinkByIndex = func(index int) (netlink.Link, error) {
		if l, ok := topo.byIndex[index]; ok {
			return l, nil
		}

		return nil, errors.New("no such index")
	}

	vethLinkSetMaster = func(l netlink.Link, master netlink.Link) error {
		topo.mastered = append(topo.mastered, l.Attrs().Name+"->"+master.Attrs().Name)
		return nil
	}
}

func vethDevice(name string, index, master int) *netlink.Device {
	return &netlink.Device{
		LinkAttrs: netlink.LinkAttrs{Name: name, Index: index, MasterIndex: master},
	}
}

func TestEnslaveVethPairsToReferenceVRF(t *testing.T) {
	upVRF := &netlink.Vrf{
		LinkAttrs: netlink.LinkAttrs{Name: "up-vrf", Index: 10},
		Table:     1001,
	}

	topo := &vethStubTopo{
		byName: map[string]netlink.Link{
			"up-vrf":       upVRF,
			"n3":           vethDevice("n3", 2, 10),
			"veth-smf":     vethDevice("veth-smf", 20, 0),
			"veth-xdp":     vethDevice("veth-xdp", 21, 0),
			"veth-buf":     vethDevice("veth-buf", 22, 0),
			"veth-buf-xdp": vethDevice("veth-buf-xdp", 23, 0),
		},
		byIndex: map[int]netlink.Link{},
	}

	for _, l := range topo.byName {
		topo.byIndex[l.Attrs().Index] = l
	}

	stubVethNetlink(t, topo)

	if err := enslaveVethPairsToReferenceVRF("n3"); err != nil {
		t.Fatalf("enslave with VRF reference: %v", err)
	}

	if len(topo.mastered) != 4 {
		t.Fatalf("expected 4 enslavements, got %d (%v)", len(topo.mastered), topo.mastered)
	}

	for _, m := range topo.mastered {
		want := map[string]bool{
			"veth-smf->up-vrf":     true,
			"veth-xdp->up-vrf":     true,
			"veth-buf->up-vrf":     true,
			"veth-buf-xdp->up-vrf": true,
		}

		if !want[m] {
			t.Errorf("unexpected enslavement %q", m)
		}
	}
}

func TestEnslaveVethPairsWithoutVRF(t *testing.T) {
	bridge := &netlink.Bridge{
		LinkAttrs: netlink.LinkAttrs{Name: "br0", Index: 11},
	}

	topo := &vethStubTopo{
		byName: map[string]netlink.Link{
			"br0":      bridge,
			"eth0":     vethDevice("eth0", 4, 0),
			"eth1":     vethDevice("eth1", 5, 11),
			"veth-smf": vethDevice("veth-smf", 20, 0),
		},
		byIndex: map[int]netlink.Link{},
	}

	for _, l := range topo.byName {
		topo.byIndex[l.Attrs().Index] = l
	}

	stubVethNetlink(t, topo)

	for _, ref := range []string{"eth0", "eth1"} {
		topo.mastered = nil

		if err := enslaveVethPairsToReferenceVRF(ref); err != nil {
			t.Fatalf("enslave with reference %s: %v", ref, err)
		}

		if len(topo.mastered) != 0 {
			t.Errorf("reference %s: expected no enslavements, got %v", ref, topo.mastered)
		}
	}
}

func TestEnslaveVethPairsSkipsMissing(t *testing.T) {
	upVRF := &netlink.Vrf{
		LinkAttrs: netlink.LinkAttrs{Name: "up-vrf", Index: 10},
		Table:     1001,
	}

	topo := &vethStubTopo{
		byName: map[string]netlink.Link{
			"up-vrf":   upVRF,
			"n3":       vethDevice("n3", 2, 10),
			"veth-smf": vethDevice("veth-smf", 20, 0),
		},
		byIndex: map[int]netlink.Link{},
	}

	for _, l := range topo.byName {
		topo.byIndex[l.Attrs().Index] = l
	}

	stubVethNetlink(t, topo)

	if err := enslaveVethPairsToReferenceVRF("n3"); err != nil {
		t.Fatalf("enslave with missing pairs: %v", err)
	}

	if len(topo.mastered) != 1 || topo.mastered[0] != "veth-smf->up-vrf" {
		t.Fatalf("expected only veth-smf enslaved, got %v", topo.mastered)
	}
}

func TestEnslaveVethPairsMissingReference(t *testing.T) {
	topo := &vethStubTopo{byName: map[string]netlink.Link{}, byIndex: map[int]netlink.Link{}}
	stubVethNetlink(t, topo)

	if err := enslaveVethPairsToReferenceVRF("n3"); err == nil {
		t.Error("missing reference: expected error, got nil")
	}
}
