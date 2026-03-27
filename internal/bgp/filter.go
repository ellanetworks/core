package bgp

import "net"

// RouteFilter holds hard-coded safety rejection prefixes.
// Any received route that overlaps with a reject prefix is dropped
// before per-peer prefix list evaluation.
type RouteFilter struct {
	RejectPrefixes []*net.IPNet
}

// ImportPrefix represents a single import prefix list entry for a peer.
type ImportPrefix struct {
	Prefix    *net.IPNet
	MaxLength int
}

// overlapsAny returns true if route is the same as, or more specific than,
// any entry in the reject list. A broad route (e.g. 0.0.0.0/0) is NOT
// rejected just because it contains a reject prefix — the kernel's
// longest-prefix-match ensures traffic for reject prefixes still uses
// more-specific local routes.
func (f *RouteFilter) overlapsAny(route *net.IPNet) bool {
	routeOnes, _ := route.Mask.Size()

	for _, reject := range f.RejectPrefixes {
		rejectOnes, _ := reject.Mask.Size()

		// Route falls within (or equals) a reject prefix:
		// the reject prefix contains the route's network address
		// and the route is at least as specific.
		if reject.Contains(route.IP) && routeOnes >= rejectOnes {
			return true
		}
	}

	return false
}

// matchesPrefixList returns true if route matches any entry in the import prefix list.
// A route matches an entry when:
//   - The route's network address is contained within the entry's prefix
//   - The route's mask length is >= the entry's prefix length (route is same or more specific)
//   - The route's mask length is <= the entry's maxLength
//
// An empty entries list means "accept nothing".
func matchesPrefixList(route *net.IPNet, entries []ImportPrefix) bool {
	routeOnes, _ := route.Mask.Size()

	for _, e := range entries {
		entryOnes, _ := e.Prefix.Mask.Size()

		if e.Prefix.Contains(route.IP) && routeOnes >= entryOnes && routeOnes <= e.MaxLength {
			return true
		}
	}

	return false
}

// BuildRejectPrefixes constructs the safety rejection list by prepending
// hard-coded prefixes (link-local, multicast, loopback) to the caller-supplied
// subnets (UE IP pools, interface addresses). These are always enforced
// regardless of per-peer prefix list configuration.
func BuildRejectPrefixes(subnets []*net.IPNet) []*net.IPNet {
	hardCoded := []string{
		"169.254.0.0/16", // link-local
		"224.0.0.0/4",    // multicast
		"127.0.0.0/8",    // loopback
	}

	var prefixes []*net.IPNet

	for _, cidr := range hardCoded {
		_, network, err := net.ParseCIDR(cidr)
		if err != nil {
			continue
		}

		prefixes = append(prefixes, network)
	}

	prefixes = append(prefixes, subnets...)

	return prefixes
}

// BuildRouteFilter constructs a RouteFilter from UE IP pool subnets and
// network interface configuration. This is the single source of truth for
// filter construction — used at startup and when data networks change.
//
// Parameters:
//   - uePools: UE IP pool CIDRs from all data networks
//   - n3Addr: N3 interface IP address (added as /32), may be nil
//   - n6IfName: N6 interface name (its IPv4 subnets are added)
func BuildRouteFilter(uePools []*net.IPNet, n3Addr net.IP, n6IfName string) *RouteFilter {
	var subnets []*net.IPNet

	subnets = append(subnets, uePools...)

	if n3Addr != nil {
		subnets = append(subnets, &net.IPNet{
			IP:   n3Addr.To4(),
			Mask: net.CIDRMask(32, 32),
		})
	}

	subnets = append(subnets, InterfaceIPv4Subnets(n6IfName)...)

	return &RouteFilter{RejectPrefixes: BuildRejectPrefixes(subnets)}
}

// InterfaceIPv4Subnets returns the IPv4 subnets configured on the named
// network interface.
func InterfaceIPv4Subnets(ifName string) []*net.IPNet {
	iface, err := net.InterfaceByName(ifName)
	if err != nil {
		return nil
	}

	addrs, err := iface.Addrs()
	if err != nil {
		return nil
	}

	var subnets []*net.IPNet

	for _, addr := range addrs {
		_, network, err := net.ParseCIDR(addr.String())
		if err != nil {
			continue
		}

		if network.IP.To4() != nil {
			subnets = append(subnets, network)
		}
	}

	return subnets
}
