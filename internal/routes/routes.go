package routes

import (
	"fmt"
	"net"

	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"
)

func CreateKernelRoute(destination *net.IPNet, gateway net.IP, priority int, interfaceName string) error {
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

func DeleteKernelRoute(destination *net.IPNet, gateway net.IP, priority int, interfaceName string) error {
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
