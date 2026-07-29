// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package kernel

import (
	"context"
	"fmt"
	"net"
	"net/netip"

	"github.com/vishvananda/netlink"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

var tracer = otel.Tracer("ella-core/kernel")

// AddNeighbourOnLink adds the provided IP as a neighbour on one specific link.
func AddNeighbourOnLink(ctx context.Context, neigh netip.Addr, ifindex int) error {
	_, span := tracer.Start(
		ctx,
		"kernel/add_neighbour_on_link",
		trace.WithAttributes(
			attribute.String("IP", neigh.String()),
			attribute.Int("ifindex", ifindex),
		))
	defer span.End()

	nlNeigh := netlink.Neigh{
		LinkIndex: ifindex,
		IP:        neigh.AsSlice(),
		FlagsExt:  netlink.NTF_EXT_MANAGED,
	}

	return netlink.NeighSet(&nlNeigh)
}

// AddNeighbour adds the provided IP as a neighbour
// on all links that have an address in the same subnet.
func AddNeighbour(ctx context.Context, neigh netip.Addr) error {
	_, span := tracer.Start(
		ctx,
		"kernel/add_neighbour",
		trace.WithAttributes(
			attribute.String("IP", neigh.String()),
		))
	defer span.End()

	neighIP := neigh.AsSlice()

	links, err := netlink.LinkList()
	if err != nil {
		return fmt.Errorf("could not list network links: %v", err)
	}

	added := false

	for _, l := range links {
		addrs, err := netlink.AddrList(l, netlink.FAMILY_ALL)
		if err != nil {
			return fmt.Errorf("could not list addresses for link: %v", err)
		}

		for _, a := range addrs {
			if a.Contains(neighIP) {
				err = addNeighbourForLink(neighIP, l)
				if err != nil {
					return fmt.Errorf("could not add neighbour for link: %v", err)
				}

				added = true
			}
		}
	}

	if !added {
		return fmt.Errorf("could not add neighbour")
	}

	return nil
}

func addNeighbourForLink(neigh net.IP, link netlink.Link) error {
	nlNeigh := netlink.Neigh{
		LinkIndex: link.Attrs().Index,
		IP:        neigh,
		FlagsExt:  netlink.NTF_EXT_MANAGED,
	}

	return netlink.NeighSet(&nlNeigh)
}
