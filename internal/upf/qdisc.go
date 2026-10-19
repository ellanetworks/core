// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: Apache-2.0

//go:build linux

package upf

import (
	"errors"
	"fmt"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/vishvananda/netlink"
	"go.uber.org/zap"
)

const departureQueuePackets = 10000

var errCustomQdisc = errors.New("interface carries an operator-configured qdisc")

var replaceableQdiscs = map[string]bool{
	"noqueue":    true,
	"pfifo_fast": true,
	"pfifo":      true,
	"fq_codel":   true,
	"fq":         true,
}

func installDepartureQdiscs(ifnames ...string) {
	for _, ifname := range ifnames {
		err := installDepartureQdisc(ifname)
		if err == nil {
			continue
		}

		logger.UpfLog.Warn("could not install fq on a datapath interface: "+
			"traffic above a QER bitrate is dropped instead of queued, "+
			"which TCP recovers from far below the configured rate",
			zap.String("iface", ifname), zap.Error(err))
	}
}

func installDepartureQdisc(ifname string) error {
	link, err := netlink.LinkByName(ifname)
	if err != nil {
		return fmt.Errorf("lookup %s: %w", ifname, err)
	}

	qdiscs, err := netlink.QdiscList(link)
	if err != nil {
		return fmt.Errorf("list qdiscs on %s: %w", ifname, err)
	}

	root := rootQdisc(qdiscs)

	switch {
	case root == nil || replaceableQdiscs[root.Type()]:
	case root.Type() == "mq":
		for _, leaf := range mqLeaves(qdiscs, root) {
			if !replaceableQdiscs[leaf.Type()] {
				return fmt.Errorf("%w: %s under mq", errCustomQdisc, leaf.Type())
			}
		}
	default:
		return fmt.Errorf("%w: %s", errCustomQdisc, root.Type())
	}

	if link.Attrs().NumTxQueues <= 1 {
		return replaceWithFq(link.Attrs().Index, netlink.HANDLE_ROOT)
	}

	if root == nil || root.Type() != "mq" {
		mq := &netlink.GenericQdisc{
			QdiscAttrs: netlink.QdiscAttrs{LinkIndex: link.Attrs().Index, Parent: netlink.HANDLE_ROOT},
			QdiscType:  "mq",
		}
		if err := netlink.QdiscReplace(mq); err != nil {
			return replaceWithFq(link.Attrs().Index, netlink.HANDLE_ROOT)
		}

		if qdiscs, err = netlink.QdiscList(link); err != nil {
			return fmt.Errorf("list qdiscs on %s: %w", ifname, err)
		}

		root = rootQdisc(qdiscs)
		if root == nil {
			return fmt.Errorf("mq on %s left no root qdisc", ifname)
		}
	}

	for _, leaf := range mqLeaves(qdiscs, root) {
		if err := replaceWithFq(link.Attrs().Index, leaf.Attrs().Parent); err != nil {
			return err
		}
	}

	return nil
}

func rootQdisc(qdiscs []netlink.Qdisc) netlink.Qdisc {
	for _, q := range qdiscs {
		if q.Attrs().Parent == netlink.HANDLE_ROOT {
			return q
		}
	}

	return nil
}

func mqLeaves(qdiscs []netlink.Qdisc, root netlink.Qdisc) []netlink.Qdisc {
	major, _ := netlink.MajorMinor(root.Attrs().Handle)

	var leaves []netlink.Qdisc

	for _, q := range qdiscs {
		parent := q.Attrs().Parent
		if parent == netlink.HANDLE_ROOT || parent == netlink.HANDLE_INGRESS || parent == netlink.HANDLE_CLSACT {
			continue
		}

		if m, _ := netlink.MajorMinor(parent); m == major {
			leaves = append(leaves, q)
		}
	}

	return leaves
}

func replaceWithFq(ifindex int, parent uint32) error {
	fq := netlink.NewFq(netlink.QdiscAttrs{LinkIndex: ifindex, Parent: parent})
	fq.PacketLimit = departureQueuePackets
	fq.FlowPacketLimit = departureQueuePackets

	if err := netlink.QdiscReplace(fq); err != nil {
		return fmt.Errorf("replace qdisc %s with fq: %w", netlink.HandleStr(parent), err)
	}

	return nil
}
