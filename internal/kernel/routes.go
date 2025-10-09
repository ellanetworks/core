package kernel

import (
	"fmt"
	"net"
	"os"
	"time"

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

const routeTable = 42
const vrfName = "ella"

// Kernel defines the interface for kernel route management.
type Kernel interface {
	EnableIPForwarding() error
	IsIPForwardingEnabled() (bool, error)
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

func (rk *RealKernel) createVRF() error {
	ella_vrf := &netlink.Vrf{
		LinkAttrs: netlink.LinkAttrs{
			Name:      vrfName,
			OperState: netlink.OperUp,
		},
		Table: routeTable,
	}
	err := netlink.LinkAdd(ella_vrf)
	if err != nil {
		return fmt.Errorf("failed to create vrf: %v", err)
	}
	return nil
}

// SetupVRF creates the ella core VRF and assigns the UPF interfaces
// to that VRF
func (rk *RealKernel) SetupVRF() error {
	ella_link, err := netlink.LinkByName(vrfName)
	if err != nil {
		err := rk.createVRF()
		if err != nil {
			return err
		}
		logger.EllaLog.Debug("Created VRF")
	} else {
		ella_vrf, ok := ella_link.(*netlink.Vrf)
		if !ok || ella_vrf.Table != routeTable {
			err = netlink.LinkDel(ella_vrf)
			if err != nil {
				return fmt.Errorf("failed to delete link: %v", err)
			}
			err = rk.createVRF()
			if err != nil {
				return err
			}
			logger.EllaLog.Debug("Recreated VRF")
		}
	}
	ella_link, err = netlink.LinkByName(vrfName)
	if err != nil {
		return fmt.Errorf("failed to get vrf link: %v", err)
	}

	for key, name := range rk.ifMapping {
		link, err := netlink.LinkByName(name)
		if err != nil {
			return fmt.Errorf("failed to find network interface %q: %v", name, err)
		}
		addrs, err := netlink.AddrList(link, unix.AF_INET)
		if err != nil {
			return fmt.Errorf("failed to list network addresses for interface %q: %v", name, err)
		}

		err = netlink.LinkSetMaster(link, ella_link)
		if err != nil {
			return fmt.Errorf("failed to set %q interface in vrf: %v", key, err)
		}
		logger.EllaLog.Debug("Assigned interface to VRF", zap.String("interface", name), zap.String("vrf", vrfName))

		// In some cases, moving the interface to the VRF can result in the interface losing its IP address.
		// We try to reassign the original IP address here, ignoring errors.
		for _, addr := range addrs {
			err = netlink.AddrAdd(link, &addr)
			if err != nil {
				logger.EllaLog.Debug("Could not reassign address to interface", zap.String("address", addr.String()), zap.String("interface", name))
			}
		}
	}

	// Many things need to happen in the kernel after the interfaces are moved to the VRF.
	// This creates a race condition with the route reconciliation if this is not completed.
	// The following sleep ensures that this does not happen.
	time.Sleep(500 * time.Millisecond)
	return nil
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
		Protocol:  unix.RTPROT_STATIC,
		Table:     routeTable,
	}

	if err := netlink.RouteAdd(&nlRoute); err != nil {
		return fmt.Errorf("failed to add route: %v", err)
	}
	logger.EllaLog.Debug("Added route", zap.String("destination", destination.String()), zap.String("gateway", gateway.String()), zap.Int("priority", priority), zap.String("interface", interfaceName))
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
		Protocol:  unix.RTPROT_STATIC,
		Table:     routeTable,
	}

	if err := netlink.RouteDel(&nlRoute); err != nil {
		return fmt.Errorf("failed to delete route: %v", err)
	}
	logger.EllaLog.Debug("Deleted route", zap.String("destination", destination.String()), zap.String("gateway", gateway.String()), zap.Int("priority", priority), zap.String("interface", interfaceName))
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
		Protocol:  unix.RTPROT_STATIC,
		Table:     routeTable,
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
