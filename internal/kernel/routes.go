package kernel

import (
	"fmt"
	"net"

	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"
)

// Kernel defines the interface for kernel route management.
type Kernel interface {
	CreateRoute(destination *net.IPNet, gateway net.IP, priority int, interfaceName string) error
	DeleteRoute(destination *net.IPNet, gateway net.IP, priority int, interfaceName string) error
	InterfaceExists(interfaceName string) (bool, error)
	RouteExists(destination *net.IPNet, gateway net.IP, priority int, interfaceName string) (bool, error)
}

// RealKernel is the production implementation of the Kernel interface
// that actually applies routes via the netlink library.
type RealKernel struct{}

// CreateRoute adds a route to the kernel.
func (rk RealKernel) CreateRoute(destination *net.IPNet, gateway net.IP, priority int, interfaceName string) error {
	link, err := netlink.LinkByName(interfaceName)
	if err != nil {
		return fmt.Errorf("failed to find network interface: %v", err)
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
	return nil
}

// DeleteRoute removes a route from the kernel.
func (rk RealKernel) DeleteRoute(destination *net.IPNet, gateway net.IP, priority int, interfaceName string) error {
	link, err := netlink.LinkByName(interfaceName)
	if err != nil {
		return fmt.Errorf("failed to find network interface: %v", err)
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

// InterfaceExists checks if a network interface exists.
func (rk RealKernel) InterfaceExists(interfaceName string) (bool, error) {
	_, err := netlink.LinkByName(interfaceName)
	if err != nil {
		if _, ok := err.(netlink.LinkNotFoundError); ok {
			return false, nil
		}
		return false, fmt.Errorf("failed to find network interface: %v", err)
	}
	return true, nil
}

// RouteExists checks if a route exists.
func (rk RealKernel) RouteExists(destination *net.IPNet, gateway net.IP, priority int, interfaceName string) (bool, error) {
	link, err := netlink.LinkByName(interfaceName)
	if err != nil {
		return false, fmt.Errorf("failed to find network interface: %v", err)
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
