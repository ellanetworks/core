package kernel

import (
	"fmt"
	"net"
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

// Kernel defines the interface for kernel route management.
type Kernel interface {
	EnableIPForwarding() error
	IsIPForwardingEnabled() (bool, error)
	CreateRoute(destination *net.IPNet, gateway net.IP, priority int, ifKey NetworkInterface) error
	DeleteRoute(destination *net.IPNet, gateway net.IP, priority int, ifKey NetworkInterface) error
	InterfaceExists(ifKey NetworkInterface) (bool, error)
	RouteExists(destination *net.IPNet, gateway net.IP, priority int, ifKey NetworkInterface) (bool, error)
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

// CreateRoute adds a route to the kernel for the interface defined by ifKey.
func (rk *RealKernel) CreateRoute(destination *net.IPNet, gateway net.IP, priority int, ifKey NetworkInterface) error {
	interfaceName, ok := rk.ifMapping[ifKey]
	if !ok {
		return fmt.Errorf("invalid interface key: %v", ifKey)
	}

	link, err := netlink.LinkByName(interfaceName)
	if err != nil {
		return fmt.Errorf("failed to find network interface %q: %v", interfaceName, err)
	}

	nlRoute := netlink.Route{
		Dst:       destination,
		Gw:        gateway,
		LinkIndex: link.Attrs().Index,
		Priority:  priority,
		Table:     unix.RT_TABLE_MAIN,
	}

	if err := netlink.RouteAdd(&nlRoute); err != nil {
		return fmt.Errorf("failed to add route: %v", err)
	}
	logger.EllaLog.Debug("Added route", zap.String("destination", destination.String()), zap.String("gateway", gateway.String()), zap.Int("priority", priority), zap.String("interface", interfaceName))

	// Tells the kernel that the gateway is in use, and ARP requests should be sent out
	return addNeighbourForLink(gateway, link)
}

// DeleteRoute removes a route from the kernel for the interface defined by ifKey.
func (rk *RealKernel) DeleteRoute(destination *net.IPNet, gateway net.IP, priority int, ifKey NetworkInterface) error {
	interfaceName, ok := rk.ifMapping[ifKey]
	if !ok {
		return fmt.Errorf("invalid interface key: %v", ifKey)
	}

	link, err := netlink.LinkByName(interfaceName)
	if err != nil {
		return fmt.Errorf("failed to find network interface %q: %v", interfaceName, err)
	}

	nlRoute := netlink.Route{
		Dst:       destination,
		Gw:        gateway,
		LinkIndex: link.Attrs().Index,
		Priority:  priority,
		Table:     unix.RT_TABLE_MAIN,
	}

	if err := netlink.RouteDel(&nlRoute); err != nil {
		return fmt.Errorf("failed to delete route: %v", err)
	}
	return nil
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

// RouteExists checks if a route exists for the interface defined by ifKey.
func (rk *RealKernel) RouteExists(destination *net.IPNet, gateway net.IP, priority int, ifKey NetworkInterface) (bool, error) {
	interfaceName, ok := rk.ifMapping[ifKey]
	if !ok {
		return false, fmt.Errorf("invalid interface key: %v", ifKey)
	}

	link, err := netlink.LinkByName(interfaceName)
	if err != nil {
		return false, fmt.Errorf("failed to find network interface %q: %v", interfaceName, err)
	}

	nlRoute := netlink.Route{
		Dst:       destination,
		Gw:        gateway,
		LinkIndex: link.Attrs().Index,
		Priority:  priority,
		Table:     unix.RT_TABLE_MAIN,
	}

	routes, err := netlink.RouteListFiltered(unix.AF_INET, &nlRoute, netlink.RT_FILTER_DST|netlink.RT_FILTER_GW|netlink.RT_FILTER_OIF|netlink.RT_FILTER_TABLE)
	if err != nil {
		return false, fmt.Errorf("failed to list routes: %v", err)
	}

	return len(routes) > 0, nil
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
	c := nftables.Chain{
		Name:     "forward",
		Priority: nftables.ChainPriorityFilter,
		Table:    &t,
		Hooknum:  nftables.ChainHookForward,
		Type:     nftables.ChainTypeFilter,
		Policy:   &polDrop,
	}
	conn.AddChain(&c)
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
	logger.EllaLog.Debug("Enabled IP forwarding")
	return nil
}

// IsIPForwardingEnabled checks if IP forwarding is enabled on the host.
func (rk *RealKernel) IsIPForwardingEnabled() (bool, error) {
	data, err := os.ReadFile("/proc/sys/net/ipv4/ip_forward")
	if err != nil {
		return false, fmt.Errorf("failed to read ip_forward: %v", err)
	}
	return string(data) == "1", nil
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
	routes, err := netlink.RouteListFiltered(unix.AF_INET, &nlRoute, netlink.RT_FILTER_OIF)
	if err != nil {
		return fmt.Errorf("failed to list routes: %v", err)
	}

	for _, route := range routes {
		if route.Gw != nil {
			err := addNeighbourForLink(route.Gw, link)
			if err != nil {
				logger.EllaLog.Warn("failed to add gateway to neighbour list, arp may need to be triggered manually", zap.String("gateway", route.Gw.String()), zap.Error(err))
			}
		}
	}

	return nil
}
