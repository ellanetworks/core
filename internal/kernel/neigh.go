// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package kernel

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/netip"

	"github.com/vishvananda/netlink"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"golang.org/x/sys/unix"
)

var tracer = otel.Tracer("ella-core/kernel")

var errNoRouteToNeighbour = errors.New("no route to neighbour")

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

	return setNeighbour(ifindex, neigh.AsSlice())
}

func AddNeighbour(ctx context.Context, neigh netip.Addr) error {
	_, span := tracer.Start(
		ctx,
		"kernel/add_neighbour",
		trace.WithAttributes(
			attribute.String("IP", neigh.String()),
		))
	defer span.End()

	dst := neigh.AsSlice()

	routes, err := netlink.RouteGetWithOptions(dst, &netlink.RouteGetOptions{FIBMatch: true})
	if err != nil && !errors.Is(err, unix.EHOSTUNREACH) && !errors.Is(err, unix.ENETUNREACH) {
		return fmt.Errorf("could not resolve route to %s: %w", neigh, err)
	}

	hops := nexthopsFromRoutes(dst, routes)
	if len(hops) == 0 {
		return fmt.Errorf("%w: %s", errNoRouteToNeighbour, neigh)
	}

	span.SetAttributes(attribute.Int("nexthops", len(hops)))

	var firstErr error

	installed := 0

	for _, h := range hops {
		if err := setNeighbour(h.ifindex, h.ip); err != nil {
			if firstErr == nil {
				firstErr = fmt.Errorf("could not add neighbour %s on link %d: %w", h.ip, h.ifindex, err)
			}

			continue
		}

		installed++
	}

	span.SetAttributes(attribute.Int("nexthops.installed", installed))

	if installed == 0 {
		return firstErr
	}

	return nil
}

type nexthop struct {
	ifindex int
	ip      net.IP
}

func nexthopsFromRoutes(dst net.IP, routes []netlink.Route) []nexthop {
	var hops []nexthop

	add := func(ifindex int, ip net.IP) {
		if ifindex <= 0 {
			return
		}

		hops = append(hops, nexthop{ifindex: ifindex, ip: ip})
	}

	for _, r := range routes {
		if len(r.MultiPath) > 0 {
			for _, mp := range r.MultiPath {
				add(mp.LinkIndex, nexthopAddr(dst, mp.Gw, mp.Via))
			}

			continue
		}

		add(r.LinkIndex, nexthopAddr(dst, r.Gw, r.Via))
	}

	return hops
}

func nexthopAddr(dst net.IP, gw net.IP, via netlink.Destination) net.IP {
	if gw != nil {
		return gw
	}

	if v, ok := via.(*netlink.Via); ok && v != nil && v.Addr != nil {
		return v.Addr
	}

	return dst
}

func addNeighbourForLink(neigh net.IP, link netlink.Link) error {
	return setNeighbour(link.Attrs().Index, neigh)
}

func setNeighbour(ifindex int, ip net.IP) error {
	nlNeigh := netlink.Neigh{
		LinkIndex: ifindex,
		IP:        ip,
		FlagsExt:  netlink.NTF_EXT_MANAGED,
	}

	return netlink.NeighSet(&nlNeigh)
}
