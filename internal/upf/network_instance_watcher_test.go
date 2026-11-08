// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package upf

import (
	"errors"
	"net"
	"slices"
	"testing"

	"github.com/cilium/ebpf/link"
	"github.com/ellanetworks/core/internal/config"
	"github.com/ellanetworks/core/internal/upf/ebpf"
	"github.com/ellanetworks/core/internal/upf/engine"
)

type fakeInstanceDatapath struct {
	ifaces   map[string]int
	attached []int
	closed   []string
	entries  map[uint32]ebpf.NetworkInstance
}

func stubInstanceDatapath(t *testing.T, fake *fakeInstanceDatapath) {
	t.Helper()

	oldByName, oldAttach, oldPut, oldDelete := interfaceByName, attachInstanceLink, putNetworkInstance, deleteNetworkInstance

	t.Cleanup(func() {
		interfaceByName, attachInstanceLink, putNetworkInstance, deleteNetworkInstance = oldByName, oldAttach, oldPut, oldDelete
	})

	interfaceByName = func(name string) (*net.Interface, error) {
		index, ok := fake.ifaces[name]
		if !ok {
			return nil, errors.New("no such interface")
		}

		return &net.Interface{Index: index, Name: name}, nil
	}

	attachInstanceLink = func(_ *ebpf.BpfObjects, _ string, iface datapathIface) (link.Link, error) {
		fake.attached = append(fake.attached, iface.index)

		return recordingLink{name: iface.name, closed: &fake.closed}, nil
	}

	putNetworkInstance = func(_ *ebpf.BpfObjects, ni ebpf.NetworkInstance) error {
		fake.entries[ni.ID] = ni

		return nil
	}

	deleteNetworkInstance = func(_ *ebpf.BpfObjects, ni ebpf.NetworkInstance) error {
		delete(fake.entries, ni.ID)

		return nil
	}
}

func TestNetworkInstanceFollowsItsInterface(t *testing.T) {
	fake := &fakeInstanceDatapath{
		ifaces:  map[string]int{"n6-ims": 7},
		entries: map[uint32]ebpf.NetworkInstance{1: {ID: 1, Ifindex: 7, RoutingIfindex: 7}},
	}
	stubInstanceDatapath(t, fake)

	se := &engine.SessionEngine{}
	se.SetNetworkInstances(map[string]uint8{"ims": 1})

	u := &UPF{
		se:              se,
		attachedMode:    config.DatapathTCX,
		instanceLinks:   map[int]link.Link{7: recordingLink{name: "n6-ims", closed: &fake.closed}},
		datapathIfindex: map[int]bool{2: true, 3: true},
		instances: newNetworkInstanceStates([]resolvedNetworkInstance{{
			name:          "ims",
			attachName:    "n6-ims",
			interfaceName: "n6-ims",
			datapath:      ebpf.NetworkInstance{ID: 1, Ifindex: 7, RoutingIfindex: 7},
		}}),
	}

	u.refreshNetworkInstances()

	if len(fake.attached) != 0 || len(fake.closed) != 0 {
		t.Fatalf("an unchanged interface was re-attached (%v) or detached (%v)", fake.attached, fake.closed)
	}

	delete(fake.ifaces, "n6-ims")
	u.refreshNetworkInstances()

	if _, ok := fake.entries[1]; ok || se.HasNetworkInstance("ims") {
		t.Fatal("a network instance without an interface is still available")
	}

	if !slices.Equal(fake.closed, []string{"n6-ims"}) || len(u.instanceLinks) != 0 {
		t.Fatalf("detached %v, links %v; want the removed interface detached", fake.closed, u.instanceLinks)
	}

	fake.ifaces["n6-ims"] = 9

	u.refreshNetworkInstances()

	if got := fake.entries[1]; got.Ifindex != 9 || got.RoutingIfindex != 9 {
		t.Fatalf("network instance entry = %+v, want the recreated interface 9", got)
	}

	if !se.HasNetworkInstance("ims") || !slices.Equal(fake.attached, []int{9}) {
		t.Fatalf("recreated interface: available %t, attached %v", se.HasNetworkInstance("ims"), fake.attached)
	}
}

func TestNetworkInstanceOnADatapathInterfaceIsNotReattached(t *testing.T) {
	fake := &fakeInstanceDatapath{
		ifaces:  map[string]int{"eth1": 3, "eth1.200": 11},
		entries: map[uint32]ebpf.NetworkInstance{},
	}
	stubInstanceDatapath(t, fake)

	u := &UPF{
		se:              &engine.SessionEngine{},
		attachedMode:    config.DatapathTCX,
		instanceLinks:   map[int]link.Link{},
		datapathIfindex: map[int]bool{2: true, 3: true},
		instances: []*networkInstanceState{{
			name: "corp", attachName: "eth1", routingName: "eth1.200", id: 1, vlan: 200,
		}},
	}

	u.refreshNetworkInstances()

	if len(fake.attached) != 0 {
		t.Fatalf("attached %v to an interface N6 already carries", fake.attached)
	}

	if got := fake.entries[1]; got.Ifindex != 3 || got.RoutingIfindex != 11 || got.Vlan != 200 {
		t.Fatalf("network instance entry = %+v", got)
	}
}
