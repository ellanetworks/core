// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package kernel

import (
	"errors"
	"testing"

	"github.com/ellanetworks/core/internal/netutil"
	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"
)

func TestRouteTableForLink(t *testing.T) {
	upVRF := &netlink.Vrf{
		LinkAttrs: netlink.LinkAttrs{Name: "up-vrf", Index: 10},
		Table:     1001,
	}
	n3 := &netlink.Device{
		LinkAttrs: netlink.LinkAttrs{Name: "n3", Index: 2, MasterIndex: 10},
	}
	plain := &netlink.Device{
		LinkAttrs: netlink.LinkAttrs{Name: "eth0", Index: 4},
	}
	bridge := &netlink.Bridge{
		LinkAttrs: netlink.LinkAttrs{Name: "br0", Index: 11},
	}
	bridged := &netlink.Device{
		LinkAttrs: netlink.LinkAttrs{Name: "eth1", Index: 5, MasterIndex: 11},
	}
	orphan := &netlink.Device{
		LinkAttrs: netlink.LinkAttrs{Name: "orphan", Index: 6, MasterIndex: 99},
	}
	enslavedVlan := &netlink.Vlan{
		LinkAttrs: netlink.LinkAttrs{Name: "n3.100", Index: 7, MasterIndex: 10, ParentIndex: 2},
		VlanId:    100,
	}
	vlanOverEnslavedParent := &netlink.Vlan{
		LinkAttrs: netlink.LinkAttrs{Name: "n3.200", Index: 8, ParentIndex: 2},
		VlanId:    200,
	}

	byIndex := map[int]netlink.Link{
		10: upVRF,
		2:  n3,
		4:  plain,
		11: bridge,
		5:  bridged,
		6:  orphan,
		7:  enslavedVlan,
		8:  vlanOverEnslavedParent,
	}

	oldByIndex := netutil.LinkByIndex

	netutil.LinkByIndex = func(index int) (netlink.Link, error) {
		if l, ok := byIndex[index]; ok {
			return l, nil
		}

		return nil, errors.New("no such index")
	}

	t.Cleanup(func() { netutil.LinkByIndex = oldByIndex })

	for _, tc := range []struct {
		name string
		link netlink.Link
		want int
	}{
		{"enslaved", n3, 1001},
		{"plain", plain, unix.RT_TABLE_MAIN},
		{"vrf itself", upVRF, 1001},
		{"non-vrf master", bridged, unix.RT_TABLE_MAIN},
		{"enslaved vlan", enslavedVlan, 1001},
		{"vlan over enslaved parent", vlanOverEnslavedParent, unix.RT_TABLE_MAIN},
	} {
		got, err := routeTableForLink(tc.link)
		if err != nil {
			t.Errorf("%s: unexpected error: %v", tc.name, err)
			continue
		}

		if got != tc.want {
			t.Errorf("%s: got table %d, want %d", tc.name, got, tc.want)
		}
	}

	if _, err := routeTableForLink(orphan); err == nil {
		t.Error("missing master: expected error, got nil")
	}

	if _, err := routeTableForLink(nil); err == nil {
		t.Error("nil link: expected error, got nil")
	}
}
