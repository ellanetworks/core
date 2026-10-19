// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: Apache-2.0

//go:build linux

package upf

import (
	"errors"
	"testing"

	"github.com/vishvananda/netlink"
)

func addTestVeth(t *testing.T, name string, txQueues int) netlink.Link {
	t.Helper()

	veth := &netlink.Veth{
		LinkAttrs: netlink.LinkAttrs{Name: name, NumTxQueues: txQueues, NumRxQueues: txQueues},
		PeerName:  name + "p",
	}
	if err := netlink.LinkAdd(veth); err != nil {
		t.Fatalf("add veth %s: %v", name, err)
	}

	t.Cleanup(func() { _ = netlink.LinkDel(veth) })

	link, err := netlink.LinkByName(name)
	if err != nil {
		t.Fatalf("lookup %s: %v", name, err)
	}

	return link
}

func departureQdiscs(t *testing.T, link netlink.Link) (root netlink.Qdisc, leaves []*netlink.Fq) {
	t.Helper()

	qdiscs, err := netlink.QdiscList(link)
	if err != nil {
		t.Fatalf("list qdiscs: %v", err)
	}

	root = rootQdisc(qdiscs)
	if root == nil {
		t.Fatal("no root qdisc")
	}

	for _, q := range qdiscs {
		if fq, ok := q.(*netlink.Fq); ok {
			leaves = append(leaves, fq)
		}
	}

	return root, leaves
}

func requireDepartureFq(t *testing.T, fqs []*netlink.Fq, want int) {
	t.Helper()

	if len(fqs) != want {
		t.Fatalf("%d fq qdiscs, want %d", len(fqs), want)
	}

	for _, fq := range fqs {
		if fq.FlowPacketLimit != departureQueuePackets {
			t.Errorf("fq flow_limit %d, want %d: the default 100 packets drops a single flow's departure queue long before the horizon",
				fq.FlowPacketLimit, departureQueuePackets)
		}
	}
}

func TestInstallDepartureQdiscMultiqueue(t *testing.T) {
	requireRoot(t)

	const txQueues = 4

	link := addTestVeth(t, "ellfq0", txQueues)

	for range 2 {
		if err := installDepartureQdisc(link.Attrs().Name); err != nil {
			t.Fatalf("install: %v", err)
		}
	}

	root, fqs := departureQdiscs(t, link)
	if root.Type() != "mq" {
		t.Fatalf("root qdisc %s, want mq so each TX queue keeps its own lock", root.Type())
	}

	requireDepartureFq(t, fqs, txQueues)
}

func TestInstallDepartureQdiscSingleQueue(t *testing.T) {
	requireRoot(t)

	link := addTestVeth(t, "ellfq1", 1)

	if err := installDepartureQdisc(link.Attrs().Name); err != nil {
		t.Fatalf("install: %v", err)
	}

	root, fqs := departureQdiscs(t, link)
	if root.Type() != "fq" {
		t.Fatalf("root qdisc %s, want fq", root.Type())
	}

	requireDepartureFq(t, fqs, 1)
}

func TestInstallDepartureQdiscLeavesOperatorQdisc(t *testing.T) {
	requireRoot(t)

	link := addTestVeth(t, "ellfq2", 1)

	tbf := &netlink.Tbf{
		QdiscAttrs: netlink.QdiscAttrs{LinkIndex: link.Attrs().Index, Parent: netlink.HANDLE_ROOT},
		Rate:       1 << 20,
		Limit:      1 << 16,
		Buffer:     1 << 14,
	}
	if err := netlink.QdiscAdd(tbf); err != nil {
		t.Fatalf("add tbf: %v", err)
	}

	if err := installDepartureQdisc(link.Attrs().Name); !errors.Is(err, errCustomQdisc) {
		t.Fatalf("install over tbf returned %v, want errCustomQdisc", err)
	}

	if root, _ := departureQdiscs(t, link); root.Type() != "tbf" {
		t.Fatalf("root qdisc %s after install, want the operator's tbf left in place", root.Type())
	}
}
