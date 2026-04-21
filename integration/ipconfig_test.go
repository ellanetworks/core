package integration_test

import (
	"fmt"
	"os"
	"strings"
)

// IPFamily represents the IP address family mode for tests
type IPFamily string

const (
	IPv4Only  IPFamily = "ipv4"
	IPv6Only  IPFamily = "ipv6"
	DualStack IPFamily = "dualstack"
)

// DetectIPFamily reads the IP_VERSION environment variable and returns the corresponding IPFamily
// Defaults to IPv4Only if not set
func DetectIPFamily() IPFamily {
	switch strings.ToLower(os.Getenv("IP_VERSION")) {
	case "v6", "ipv6":
		return IPv6Only
	case "dual", "dualstack", "both":
		return DualStack
	default:
		return IPv4Only
	}
}

// formatIPv6 wraps IPv6 addresses in brackets for use in URLs/addresses
func formatIPv6(addr string) string {
	return fmt.Sprintf("[%s]", addr)
}

// APIAddress returns the API endpoint URL in the format appropriate for the current IP family
func APIAddress() string {
	family := DetectIPFamily()
	port := 5002

	switch family {
	case IPv6Only, DualStack:
		return fmt.Sprintf("http://%s:%d", formatIPv6("2001:db8:1::10"), port)
	default: // IPv4Only
		return fmt.Sprintf("http://10.3.0.2:%d", port)
	}
}

// N2Address returns the N2 interface address for the given node
// node parameter is mainly used for HA tests
func N2Address(node int) string {
	family := DetectIPFamily()

	switch family {
	case IPv6Only, DualStack:
		return "2001:db8:1::10"
	default: // IPv4Only
		return "10.3.0.2"
	}
}

// N3Address returns the N3 interface address for the given node
// node parameter is mainly used for HA tests
func N3Address(node int) string {
	family := DetectIPFamily()

	switch family {
	case IPv6Only, DualStack:
		return "2001:db8:3::10"
	default: // IPv4Only
		return "10.3.0.2"
	}
}

// N6Address returns the N6 (external) address - always IPv4
func N6Address() string {
	return "10.6.0.3"
}

// CoreTesterN3Address returns the core-tester N3 interface address
func CoreTesterN3Address() string {
	family := DetectIPFamily()

	switch family {
	case IPv6Only, DualStack:
		return "2001:db8:3::11"
	default: // IPv4Only
		return "10.3.0.3"
	}
}

// CoreTesterN3AddressSecondary returns the core-tester secondary N3 interface address
func CoreTesterN3AddressSecondary() string {
	family := DetectIPFamily()

	switch family {
	case IPv6Only, DualStack:
		return "2001:db8:3::13"
	default: // IPv4Only
		return "10.3.0.4"
	}
}

// CoreTesterDefaultAddress returns the core-tester default network address
func CoreTesterDefaultAddress() string {
	family := DetectIPFamily()

	switch family {
	case IPv6Only, DualStack:
		return "2001:db8:1::11"
	default: // IPv4Only
		return "10.3.0.3"
	}
}

// ClusterAddress returns the cluster (Raft) address for the given HA node (1-indexed)
func ClusterAddress(node int) string {
	var base string

	family := DetectIPFamily()

	switch family {
	case IPv6Only, DualStack:
		base = "2001:db8:2::%d"
	default: // IPv4Only
		base = "10.100.0.%d"
	}

	return fmt.Sprintf(base, 10+node)
}

// ClusterAddressWithBrackets returns the cluster address formatted with brackets for IPv6
func ClusterAddressWithBrackets(node int) string {
	family := DetectIPFamily()

	switch family {
	case IPv6Only, DualStack:
		return formatIPv6(ClusterAddress(node))
	default: // IPv4Only
		return ClusterAddress(node)
	}
}

// ClusterAddressWithPort returns the cluster address with the given port
func ClusterAddressWithPort(node int, port int) string {
	family := DetectIPFamily()
	addr := ClusterAddress(node)

	switch family {
	case IPv6Only, DualStack:
		return fmt.Sprintf("%s:%d", formatIPv6(addr), port)
	default: // IPv4Only
		return fmt.Sprintf("%s:%d", addr, port)
	}
}

// APIAddressForCluster returns the API address URL for a given HA node (1-indexed)
func APIAddressForCluster(node int) string {
	family := DetectIPFamily()
	port := 5002

	switch family {
	case IPv6Only, DualStack:
		return fmt.Sprintf("http://%s:%d", formatIPv6(ClusterAddress(node)), port)
	default: // IPv4Only
		return fmt.Sprintf("http://%s:%d", ClusterAddress(node), port)
	}
}

// UERANSIMGNBAddress returns the UERANSIM gNB address for N2/N3
func UERANSIMGNBAddress() string {
	family := DetectIPFamily()

	switch family {
	case IPv6Only, DualStack:
		return "2001:db8:1::11"
	default: // IPv4Only
		return "10.3.0.11"
	}
}

// UERANSIMGNBN3Address returns the UERANSIM gNB address for N3
func UERANSIMGNBN3Address() string {
	family := DetectIPFamily()

	switch family {
	case IPv6Only, DualStack:
		return "2001:db8:3::11"
	default: // IPv4Only
		return "10.3.0.11"
	}
}

// ComposeFile returns the compose file name to use based on the current IP family
func ComposeFile() string {
	family := DetectIPFamily()

	switch family {
	case IPv6Only:
		return "compose-ipv6.yaml"
	case DualStack:
		return "compose-dualstack.yaml"
	default: // IPv4Only
		return "compose.yaml"
	}
}
