// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package kernel

import (
	"errors"
	"fmt"
	"net"
	"net/netip"
	"os"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/google/nftables"
	"github.com/vishvananda/netlink"
	"go.uber.org/zap"
	"golang.org/x/sys/unix"
)

// NetworkInterface is an enum for network interface keys.
type NetworkInterface int

const (
	N3 NetworkInterface = iota
	N6
)

// rtProtoElla tags every route Ella inserts into the kernel routing
// table. Listing and deletion paths filter on it so the reconciler can
// remove stale Ella routes without touching operator-installed ones.
// Value sits in the user-defined range (192-255) to avoid collisions
// with kernel-defined RTPROT_* constants.
const rtProtoElla netlink.RouteProtocol = 0xEC

// ManagedRoute describes one Ella-owned kernel route.
type ManagedRoute struct {
	Destination netip.Prefix
	Gateway     netip.Addr
	Priority    int
}

// Kernel defines the interface for kernel route management.
type Kernel interface {
	EnableIPForwarding() error
	IsIPForwardingEnabled() (bool, error)
	CreateRoute(destination netip.Prefix, gateway netip.Addr, priority int, ifKey NetworkInterface) error
	DeleteRoute(destination netip.Prefix, gateway netip.Addr, priority int, ifKey NetworkInterface) error
	ReplaceRoute(destination netip.Prefix, gateway netip.Addr, priority int, ifKey NetworkInterface) error
	ListManagedRoutes(ifKey NetworkInterface) ([]ManagedRoute, error)
	InterfaceExists(ifKey NetworkInterface) (bool, error)
	RouteExists(destination netip.Prefix, gateway netip.Addr, priority int, ifKey NetworkInterface) (bool, error)
	EnsureGatewaysOnInterfaceInNeighTable(ifKey NetworkInterface) error
}

// RealKernel is the production implementation of the Kernel interface.
type RealKernel struct {
	ifMapping map[NetworkInterface]string // maps N3 and N6 to their actual interface names.
}

// NewRealKernel creates a new RealKernel instance.
// The user must supply the interface names for the n3 and n6 interfaces.
func NewRealKernel(n3Interface, n6Interface string) *RealKernel {
	return &RealKernel{
		ifMapping: map[NetworkInterface]string{
			N3: n3Interface,
			N6: n6Interface,
		},
	}
}

func UnmapPrefix(p netip.Prefix) netip.Prefix {
	if !p.Addr().Is4In6() {
		return p
	}

	bits := p.Bits()
	if bits >= 96 {
		bits -= 96
	}

	if bits < 0 || bits > 32 {
		return p
	}

	return netip.PrefixFrom(p.Addr().Unmap(), bits)
}

func prefixFromIPNet(n *net.IPNet) (netip.Prefix, bool) {
	addr, ok := netip.AddrFromSlice(n.IP)
	if !ok {
		return netip.Prefix{}, false
	}

	ones, bits := n.Mask.Size()
	if bits == 0 {
		return netip.Prefix{}, false
	}

	p := UnmapPrefix(netip.PrefixFrom(addr, ones))
	if !p.IsValid() {
		return netip.Prefix{}, false
	}

	return p, true
}

func addrFromNetIP(ip net.IP) (netip.Addr, bool) {
	addr, ok := netip.AddrFromSlice(ip)
	if !ok {
		return netip.Addr{}, false
	}

	return addr.Unmap(), true
}

// prefixToIPNet converts a netip.Prefix to a *net.IPNet for netlink.
func prefixToIPNet(p netip.Prefix) *net.IPNet {
	p = UnmapPrefix(p)

	addr := p.Masked().Addr()

	var maskBits int
	if addr.Is4() {
		maskBits = 32
	} else {
		maskBits = 128
	}

	return &net.IPNet{
		IP:   addr.AsSlice(),
		Mask: net.CIDRMask(p.Bits(), maskBits),
	}
}

// addrToNetIP converts a netip.Addr to a net.IP for netlink.
func addrToNetIP(a netip.Addr) net.IP {
	if !a.IsValid() {
		return nil
	}

	return a.Unmap().AsSlice()
}

// gwOrVia builds the gateway/via fields for a route.
// When the destination and gateway families match, it uses Gw (RTA_GATEWAY).
// When they differ (e.g. IPv4 dst + IPv6 gw), it uses Via (RTA_VIA) since
// RTA_GATEWAY requires family matching in the kernel.
func gwOrVia(destination netip.Prefix, gateway netip.Addr) (net.IP, *netlink.Via) {
	if !gateway.IsValid() {
		return nil, nil
	}

	dst := destination.Addr().Unmap()
	gw := gateway.Unmap()

	if dst.Is4() == gw.Is4() {
		return addrToNetIP(gw), nil
	}

	family := netlink.FAMILY_V4
	if gw.Is6() {
		family = netlink.FAMILY_V6
	}

	return nil, &netlink.Via{
		AddrFamily: family,
		Addr:       net.IP(gw.AsSlice()),
	}
}

// CreateRoute adds a route to the kernel for the interface defined by ifKey.
func (rk *RealKernel) CreateRoute(destination netip.Prefix, gateway netip.Addr, priority int, ifKey NetworkInterface) error {
	interfaceName, ok := rk.ifMapping[ifKey]
	if !ok {
		return fmt.Errorf("invalid interface key: %v", ifKey)
	}

	link, err := netlink.LinkByName(interfaceName)
	if err != nil {
		return fmt.Errorf("failed to find network interface %q: %v", interfaceName, err)
	}

	gw, via := gwOrVia(destination, gateway)

	nlRoute := netlink.Route{
		Dst:       prefixToIPNet(destination),
		Gw:        gw,
		LinkIndex: link.Attrs().Index,
		Priority:  priority,
		Table:     unix.RT_TABLE_MAIN,
		Protocol:  rtProtoElla,
	}
	if via != nil {
		nlRoute.Via = via
	}

	if err := netlink.RouteAdd(&nlRoute); err != nil {
		logger.EllaLog.Debug("failed to add route", zap.Any("nlRoute", nlRoute))
		return fmt.Errorf("failed to add route: %v", err)
	}

	logger.EllaLog.Debug("Added route", zap.String("destination", destination.String()), zap.String("gateway", gateway.String()), zap.Int("priority", priority), zap.String("interface", interfaceName))

	// Tells the kernel that the gateway is in use, and ARP requests should be sent out
	if gw != nil {
		return addNeighbourForLink(gw, link)
	}

	if via != nil {
		return addNeighbourForLink(via.Addr, link)
	}

	return nil
}

// DeleteRoute removes a route from the kernel for the interface defined by ifKey.
// Matches only routes carrying Ella's protocol marker, so an operator-installed
// route with the same prefix/gateway/metric is left alone.
func (rk *RealKernel) DeleteRoute(destination netip.Prefix, gateway netip.Addr, priority int, ifKey NetworkInterface) error {
	interfaceName, ok := rk.ifMapping[ifKey]
	if !ok {
		return fmt.Errorf("invalid interface key: %v", ifKey)
	}

	link, err := netlink.LinkByName(interfaceName)
	if err != nil {
		return fmt.Errorf("failed to find network interface %q: %v", interfaceName, err)
	}

	gw, via := gwOrVia(destination, gateway)

	nlRoute := netlink.Route{
		Dst:       prefixToIPNet(destination),
		Gw:        gw,
		LinkIndex: link.Attrs().Index,
		Priority:  priority,
		Table:     unix.RT_TABLE_MAIN,
		Protocol:  rtProtoElla,
	}
	if via != nil {
		nlRoute.Via = via
	}

	if err := netlink.RouteDel(&nlRoute); err != nil {
		return fmt.Errorf("failed to delete route: %v", err)
	}

	return nil
}

// ReplaceRoute creates or updates a route in the kernel for the interface defined by ifKey.
// Unlike CreateRoute, this is idempotent — it will update an existing route with the same
// destination and priority rather than returning an error.
func (rk *RealKernel) ReplaceRoute(destination netip.Prefix, gateway netip.Addr, priority int, ifKey NetworkInterface) error {
	interfaceName, ok := rk.ifMapping[ifKey]
	if !ok {
		return fmt.Errorf("invalid interface key: %v", ifKey)
	}

	link, err := netlink.LinkByName(interfaceName)
	if err != nil {
		return fmt.Errorf("failed to find network interface %q: %v", interfaceName, err)
	}

	gw, via := gwOrVia(destination, gateway)

	nlRoute := netlink.Route{
		Dst:       prefixToIPNet(destination),
		Gw:        gw,
		LinkIndex: link.Attrs().Index,
		Priority:  priority,
		Table:     unix.RT_TABLE_MAIN,
		Protocol:  rtProtoElla,
	}
	if via != nil {
		nlRoute.Via = via
	}

	if err := netlink.RouteReplace(&nlRoute); err != nil {
		return fmt.Errorf("failed to replace route: %v", err)
	}

	logger.EllaLog.Debug("Replaced route", zap.String("destination", destination.String()), zap.String("gateway", gateway.String()), zap.Int("priority", priority), zap.String("interface", interfaceName))

	if gw != nil {
		return addNeighbourForLink(gw, link)
	}

	if via != nil {
		return addNeighbourForLink(via.Addr, link)
	}

	return nil
}

// ListManagedRoutes returns every Ella-owned route on the interface
// identified by ifKey, with full destination/gateway/priority info so the
// caller can diff against a desired set and call DeleteRoute for stale
// entries.
func (rk *RealKernel) ListManagedRoutes(ifKey NetworkInterface) ([]ManagedRoute, error) {
	interfaceName, ok := rk.ifMapping[ifKey]
	if !ok {
		return nil, fmt.Errorf("invalid interface key: %v", ifKey)
	}

	link, err := netlink.LinkByName(interfaceName)
	if err != nil {
		return nil, fmt.Errorf("failed to find network interface %q: %v", interfaceName, err)
	}

	nlRoute := netlink.Route{
		LinkIndex: link.Attrs().Index,
		Table:     unix.RT_TABLE_MAIN,
		Protocol:  rtProtoElla,
	}

	var allRoutes []netlink.Route

	for _, af := range []int{unix.AF_INET, unix.AF_INET6} {
		routes, err := netlink.RouteListFiltered(af, &nlRoute, netlink.RT_FILTER_OIF|netlink.RT_FILTER_TABLE|netlink.RT_FILTER_PROTOCOL)
		if err != nil {
			return nil, fmt.Errorf("failed to list routes: %v", err)
		}

		allRoutes = append(allRoutes, routes...)
	}

	result := make([]ManagedRoute, 0, len(allRoutes))

	for _, r := range allRoutes {
		if r.Dst == nil {
			continue
		}

		dst, ok := prefixFromIPNet(r.Dst)
		if !ok {
			continue
		}

		var gw netip.Addr
		if r.Gw != nil {
			gw, _ = addrFromNetIP(r.Gw)
		} else if r.Via != nil {
			if via, ok := r.Via.(*netlink.Via); ok {
				if viaAddr, ok := addrFromNetIP(via.Addr); ok {
					gw = viaAddr
				}
			}
		}

		result = append(result, ManagedRoute{
			Destination: dst,
			Gateway:     gw,
			Priority:    r.Priority,
		})
	}

	return result, nil
}

// InterfaceExists checks if the interface corresponding to ifKey exists.
func (rk *RealKernel) InterfaceExists(ifKey NetworkInterface) (bool, error) {
	interfaceName, ok := rk.ifMapping[ifKey]
	if !ok {
		return false, fmt.Errorf("invalid interface key: %v", ifKey)
	}

	_, err := netlink.LinkByName(interfaceName)
	if err != nil {
		if _, ok := err.(netlink.LinkNotFoundError); ok {
			return false, nil
		}

		return false, fmt.Errorf("failed to find network interface %q: %v", interfaceName, err)
	}

	return true, nil
}

// RouteExists checks if an Ella-owned route exists for the interface
// defined by ifKey. Operator-installed routes with the same prefix do not
// count.
func (rk *RealKernel) RouteExists(destination netip.Prefix, gateway netip.Addr, priority int, ifKey NetworkInterface) (bool, error) {
	interfaceName, ok := rk.ifMapping[ifKey]
	if !ok {
		return false, fmt.Errorf("invalid interface key: %v", ifKey)
	}

	link, err := netlink.LinkByName(interfaceName)
	if err != nil {
		return false, fmt.Errorf("failed to find network interface %q: %v", interfaceName, err)
	}

	nlRoute := netlink.Route{
		Dst:       prefixToIPNet(destination),
		LinkIndex: link.Attrs().Index,
		Priority:  priority,
		Table:     unix.RT_TABLE_MAIN,
		Protocol:  rtProtoElla,
	}

	af := unix.AF_INET
	if !destination.Addr().Unmap().Is4() {
		af = unix.AF_INET6
	}

	_, expectedVia := gwOrVia(destination, gateway)

	routes, err := netlink.RouteListFiltered(af, &nlRoute, netlink.RT_FILTER_DST|netlink.RT_FILTER_OIF|netlink.RT_FILTER_TABLE|netlink.RT_FILTER_PROTOCOL)
	if err != nil {
		return false, fmt.Errorf("failed to list routes: %v", err)
	}

	for _, r := range routes {
		if r.Gw != nil {
			if gw, ok := addrFromNetIP(r.Gw); ok && gw == gateway.Unmap() {
				return true, nil
			}
		}

		if expectedVia != nil && r.Via != nil && r.Via.Equal(expectedVia) {
			return true, nil
		}
	}

	return false, nil
}

// filterForwarding adds a firewall default rule to block forwarding with nftables
func (rk *RealKernel) filterForwarding() error {
	conn, err := nftables.New()
	if err != nil {
		return fmt.Errorf("failed to access nftables: %v", err)
	}

	t := nftables.Table{
		Name:   "filter",
		Family: nftables.TableFamilyINet,
	}

	conn.AddTable(&t)

	polDrop := nftables.ChainPolicyDrop

	conn.AddChain(&nftables.Chain{
		Name:     "forward",
		Priority: nftables.ChainPriorityFilter,
		Table:    &t,
		Hooknum:  nftables.ChainHookForward,
		Type:     nftables.ChainTypeFilter,
		Policy:   &polDrop,
	})

	err = conn.Flush()
	if err != nil {
		return fmt.Errorf("failed to install nftables rules: %v", err)
	}

	return nil
}

// isRunningInKubernetes checks if we are running inside Kubernetes
func isRunningInKubernetes() bool {
	ksh := os.Getenv("KUBERNETES_SERVICE_HOST")
	return len(ksh) != 0
}

// EnableIPForwarding enables IP forwarding on the host.
func (rk *RealKernel) EnableIPForwarding() error {
	// Before enabling IP forwarding, we add a firewall rule to
	// default drop any forwarding for security. Because we use XDP
	// for forwarding, we will bypass these rules for legitimate traffic.
	if !isRunningInKubernetes() {
		err := rk.filterForwarding()
		if err != nil {
			return fmt.Errorf("failed to add firewall rules: %v", err)
		}
	}

	err := os.WriteFile("/proc/sys/net/ipv4/ip_forward", []byte("1"), 0o600)
	if err != nil {
		return fmt.Errorf("failed to enable ip_forward: %v", err)
	}

	err = os.WriteFile("/proc/sys/net/ipv6/conf/all/forwarding", []byte("1"), 0o600)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("failed to enable ipv6 forwarding: %v", err)
	}

	logger.EllaLog.Debug("Enabled IP forwarding")

	return nil
}

// IsIPForwardingEnabled checks if IP forwarding is enabled on the host.
func (rk *RealKernel) IsIPForwardingEnabled() (bool, error) {
	data, err := os.ReadFile("/proc/sys/net/ipv4/ip_forward")
	if err != nil {
		return false, fmt.Errorf("failed to read ip_forward: %v", err)
	}

	ipv4Enabled := string(data) == "1"

	data, err = os.ReadFile("/proc/sys/net/ipv6/conf/all/forwarding")
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			// IPv6 is disabled at the kernel level; treat as not enabled.
			return ipv4Enabled, nil
		}

		return false, fmt.Errorf("failed to read ipv6 forwarding: %v", err)
	}

	ipv6Enabled := string(data) == "1"

	return ipv4Enabled && ipv6Enabled, nil
}

func (rk *RealKernel) EnsureGatewaysOnInterfaceInNeighTable(ifKey NetworkInterface) error {
	interfaceName, ok := rk.ifMapping[ifKey]
	if !ok {
		return fmt.Errorf("invalid interface key: %v", ifKey)
	}

	link, err := netlink.LinkByName(interfaceName)
	if err != nil {
		return fmt.Errorf("failed to find network interface %q: %v", interfaceName, err)
	}

	nlRoute := netlink.Route{LinkIndex: link.Attrs().Index}

	var routes []netlink.Route

	for _, af := range []int{unix.AF_INET, unix.AF_INET6} {
		r, err := netlink.RouteListFiltered(af, &nlRoute, netlink.RT_FILTER_OIF)
		if err != nil {
			return fmt.Errorf("failed to list routes: %v", err)
		}

		routes = append(routes, r...)
	}

	for _, route := range routes {
		if route.Gw != nil {
			err := addNeighbourForLink(route.Gw, link)
			if err != nil {
				logger.EllaLog.Warn("failed to add gateway to neighbour list, arp may need to be triggered manually", zap.String("gateway", route.Gw.String()), zap.Error(err))
			}
		}

		if route.Via != nil {
			if via, ok := route.Via.(*netlink.Via); ok {
				err := addNeighbourForLink(via.Addr, link)
				if err != nil {
					logger.EllaLog.Warn("failed to add gateway to neighbour list, arp may need to be triggered manually", zap.String("gateway", via.Addr.String()), zap.Error(err))
				}
			}
		}
	}

	return nil
}
