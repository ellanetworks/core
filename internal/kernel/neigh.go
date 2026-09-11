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

// ErrNoRouteToNeighbour is returned when the kernel has no route to the address,
// so there is no link on which a neighbour entry would be meaningful.
var ErrNoRouteToNeighbour = errors.New("no route to neighbour")

// AddNeighbour resolves the address against the kernel routing table and adds a
// neighbour entry for each nexthop the kernel would forward through: the address
// itself when it is directly connected, or the gateway when it is not.
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
		return fmt.Errorf("%w: %s", ErrNoRouteToNeighbour, neigh)
	}

	span.SetAttributes(attribute.Int("nexthops", len(hops)))

	for _, h := range hops {
		if err := setNeighbour(h.ifindex, h.ip); err != nil {
			return fmt.Errorf("could not add neighbour %s on link %d: %w", h.ip, h.ifindex, err)
		}
	}

	return nil
}

type nexthop struct {
	ifindex int
	ip      net.IP
}

type nexthopKey struct {
	ifindex int
	ip      string
}

func nexthopsFromRoutes(dst net.IP, routes []netlink.Route) []nexthop {
	var hops []nexthop

	seen := make(map[nexthopKey]struct{})

	add := func(ifindex int, ip net.IP) {
		if ifindex <= 0 || ip == nil {
			return
		}

		key := nexthopKey{ifindex: ifindex, ip: ip.String()}
		if _, dup := seen[key]; dup {
			return
		}

		seen[key] = struct{}{}

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

func neighbourFor(ifindex int, ip net.IP) netlink.Neigh {
	return netlink.Neigh{
		LinkIndex: ifindex,
		IP:        ip,
		FlagsExt:  netlink.NTF_EXT_MANAGED,
	}
}

func setNeighbour(ifindex int, ip net.IP) error {
	nlNeigh := neighbourFor(ifindex, ip)

	return netlink.NeighSet(&nlNeigh)
}
