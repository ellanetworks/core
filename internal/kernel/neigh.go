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
	"github.com/vishvananda/netlink/nl"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"golang.org/x/sys/unix"
)

var tracer = otel.Tracer("ella-core/kernel")

var errNoRouteToNeighbour = errors.New("no route to neighbour")

var (
	neighRouteGetWithOptions = netlink.RouteGetWithOptions
	neighSet                 = netlink.NeighSet
	neighGet                 = getNeighbour
)

// AddNeighbourOnLink adds the provided IP as a neighbour on one specific link.
func AddNeighbourOnLink(ctx context.Context, neigh netip.Addr, ifindex int) error {
	_, span := tracer.Start(
		ctx,
		"kernel/add_neighbour_on_link",
		trace.WithAttributes(
			attribute.String("kernel.neighbour.address", neigh.String()),
			attribute.Int("kernel.link.index", ifindex),
		))
	defer span.End()

	return setNeighbour(ifindex, neigh.AsSlice())
}

func AddNeighbour(ctx context.Context, neigh netip.Addr, vrfDevice string) error {
	_, span := tracer.Start(
		ctx,
		"kernel/add_neighbour",
		trace.WithAttributes(
			attribute.String("kernel.neighbour.address", neigh.String()),
			attribute.String("kernel.vrf.device", vrfDevice),
		))
	defer span.End()

	dst := neigh.AsSlice()

	routes, err := routesToNeighbour(dst, vrfDevice)
	if err != nil {
		return fmt.Errorf("could not resolve route to %s: %w", neigh, err)
	}

	hops := nexthopsFromRoutes(dst, routes)
	if len(hops) == 0 {
		return fmt.Errorf("%w: %s", errNoRouteToNeighbour, neigh)
	}

	span.SetAttributes(attribute.Int("kernel.nexthop.count", len(hops)))

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

	span.SetAttributes(attribute.Int("kernel.nexthop.installed", installed))

	if installed == 0 {
		return firstErr
	}

	return nil
}

type nexthop struct {
	ifindex int
	ip      net.IP
}

func routesToNeighbour(dst net.IP, vrfDevice string) ([]netlink.Route, error) {
	opts := &netlink.RouteGetOptions{FIBMatch: true}
	if vrfDevice != "" {
		opts.VrfName = vrfDevice
	}

	routes, err := neighRouteGetWithOptions(dst, opts)
	if err != nil && !errors.Is(err, unix.EHOSTUNREACH) && !errors.Is(err, unix.ENETUNREACH) {
		return nil, err
	}

	return routes, nil
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
	static, err := hasStaticNeighbour(ifindex, ip)
	if err != nil {
		return err
	}

	if static {
		return nil
	}

	nlNeigh := netlink.Neigh{
		LinkIndex: ifindex,
		IP:        ip,
		FlagsExt:  netlink.NTF_EXT_MANAGED,
	}

	return neighSet(&nlNeigh)
}

func hasStaticNeighbour(ifindex int, ip net.IP) (bool, error) {
	n, err := neighGet(ifindex, ip)
	if err != nil {
		return false, fmt.Errorf("could not look up neighbour %s on link %d: %w", ip, ifindex, err)
	}

	if n == nil {
		return false, nil
	}

	return n.State&(netlink.NUD_PERMANENT|netlink.NUD_NOARP) != 0, nil
}

func neighFamilyAndAddr(ip net.IP) (uint8, []byte) {
	if v4 := ip.To4(); v4 != nil {
		return unix.AF_INET, v4
	}

	return unix.AF_INET6, ip.To16()
}

func getNeighbour(ifindex int, ip net.IP) (*netlink.Neigh, error) {
	family, addr := neighFamilyAndAddr(ip)

	req := nl.NewNetlinkRequest(unix.RTM_GETNEIGH, 0)
	req.AddData(&netlink.Ndmsg{Family: family, Index: uint32(ifindex)})
	req.AddData(nl.NewRtAttr(unix.NDA_DST, addr))

	msgs, err := req.Execute(unix.NETLINK_ROUTE, unix.RTM_NEWNEIGH)
	if errors.Is(err, unix.ENOENT) {
		return nil, nil
	}

	if err != nil {
		return nil, err
	}

	if len(msgs) == 0 {
		return nil, nil
	}

	return netlink.NeighDeserialize(msgs[0])
}
