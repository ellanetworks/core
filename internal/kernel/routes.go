package kernel

import (
	"fmt"
	"net"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/vishvananda/netlink"
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
	CreateRoute(destination *net.IPNet, gateway net.IP, priority int, ifKey NetworkInterface) error
	DeleteRoute(destination *net.IPNet, gateway net.IP, priority int, ifKey NetworkInterface) error
	InterfaceExists(ifKey NetworkInterface) (bool, error)
	RouteExists(destination *net.IPNet, gateway net.IP, priority int, ifKey NetworkInterface) (bool, error)
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
	logger.EllaLog.Infof("Added route: %v", nlRoute)
	return nil
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
