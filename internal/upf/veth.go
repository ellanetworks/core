// Copyright 2026 Ella Networks

package upf

import (
	"fmt"
	"net"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/vishvananda/netlink"
	"go.uber.org/zap"
)

const (
	// VethSMFName is the name of the veth endpoint owned by the Go
	// control plane (SMF side). Packets are injected here.
	VethSMFName = "veth-smf"

	// VethXDPName is the name of the veth endpoint where the XDP
	// program is attached. Packets arrive here from veth-smf.
	VethXDPName = "veth-xdp"

	// vethMTU is the MTU configured on both ends of the veth pair.
	// 9000 gives headroom for GTP-U encapsulation overhead.
	vethMTU = 9000
)

// CreateVethPair creates the veth-smf <-> veth-xdp pair and brings both
// links up. If the pair already exists (e.g. from a previous unclean
// shutdown) it is torn down first so the state is deterministic.
func CreateVethPair() error {
	// Clean up stale links if they exist. Deleting one side of a veth
	// pair automatically removes the peer.
	if existing, _ := netlink.LinkByName(VethSMFName); existing != nil {
		logger.UpfLog.Info("Removing stale veth pair", zap.String("link", VethSMFName))

		if err := netlink.LinkDel(existing); err != nil {
			return fmt.Errorf("delete stale %s: %w", VethSMFName, err)
		}
	}

	veth := &netlink.Veth{
		LinkAttrs: netlink.LinkAttrs{
			Name: VethSMFName,
			MTU:  vethMTU,
		},
		PeerName: VethXDPName,
	}

	if err := netlink.LinkAdd(veth); err != nil {
		return fmt.Errorf("create veth pair: %w", err)
	}

	// Set both ends up.
	for _, name := range []string{VethSMFName, VethXDPName} {
		link, err := netlink.LinkByName(name)
		if err != nil {
			// Best-effort cleanup on failure.
			_ = DestroyVethPair()
			return fmt.Errorf("lookup %s after creation: %w", name, err)
		}

		if err := netlink.LinkSetUp(link); err != nil {
			_ = DestroyVethPair()
			return fmt.Errorf("set %s up: %w", name, err)
		}
	}

	logger.UpfLog.Info("Created veth pair",
		zap.String("smf", VethSMFName),
		zap.String("xdp", VethXDPName),
		zap.Int("mtu", vethMTU),
	)

	return nil
}

// DestroyVethPair removes the veth pair. It is safe to call even if the
// pair does not exist.
func DestroyVethPair() error {
	link, err := netlink.LinkByName(VethSMFName)
	if err != nil {
		// Link does not exist — nothing to do.
		if _, ok := err.(netlink.LinkNotFoundError); ok {
			return nil
		}

		return fmt.Errorf("lookup %s for deletion: %w", VethSMFName, err)
	}

	if err := netlink.LinkDel(link); err != nil {
		return fmt.Errorf("delete %s: %w", VethSMFName, err)
	}

	logger.UpfLog.Info("Destroyed veth pair", zap.String("link", VethSMFName))

	return nil
}

// VethXDPIndex returns the ifindex of the veth-xdp interface. The link
// must already exist.
func VethXDPIndex() (int, error) {
	iface, err := net.InterfaceByName(VethXDPName)
	if err != nil {
		return 0, fmt.Errorf("lookup %s: %w", VethXDPName, err)
	}

	return iface.Index, nil
}
