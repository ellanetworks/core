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

// overlapsAny returns true if route overlaps with any entry in the reject list.
// Two prefixes overlap if either contains the other's network address.
func (f *RouteFilter) overlapsAny(route *net.IPNet) bool {
	for _, reject := range f.RejectPrefixes {
		if reject.Contains(route.IP) || route.Contains(reject.IP) {
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

// BuildRejectPrefixes constructs the hard-coded safety rejection list from
// the UE IP pool and interface subnets. These are always enforced regardless
// of per-peer prefix list configuration.
func BuildRejectPrefixes(uePool *net.IPNet, extraSubnets []*net.IPNet) []*net.IPNet {
	// Hard-coded rejections: link-local, multicast, loopback
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

	if uePool != nil {
		prefixes = append(prefixes, uePool)
	}

	prefixes = append(prefixes, extraSubnets...)

	return prefixes
}
