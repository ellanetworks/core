// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

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

	// VethBufName is the Go-side end of the downlink-buffering injection
	// pair; the BufferResponder sends re-injected frames here.
	VethBufName = "veth-buf"

	// VethBufXDPName is the program end of the buffering pair, where
	// upf_downlink_func is attached.
	VethBufXDPName = "veth-buf-xdp"

	// vethMTU is the MTU configured on both ends of every veth pair.
	// 9000 gives headroom for GTP-U encapsulation overhead.
	vethMTU = 9000
)

// CreateVethPair creates the veth-smf <-> veth-xdp pair and brings both
// links up. If the pair already exists (e.g. from a previous unclean
// shutdown) it is torn down first so the state is deterministic.
func CreateVethPair() error {
	return createVethPair(VethSMFName, VethXDPName)
}

// DestroyVethPair removes the veth-smf <-> veth-xdp pair. It is safe to
// call even if the pair does not exist.
func DestroyVethPair() error {
	return destroyVethPair(VethSMFName)
}

// VethXDPIndex returns the ifindex of the veth-xdp interface. The link
// must already exist.
func VethXDPIndex() (int, error) {
	return vethIndex(VethXDPName)
}

// createVethPair creates a named veth pair and brings both ends up. If the
// pair already exists it is torn down first so the state is deterministic;
// deleting one side of a veth pair removes the peer.
func createVethPair(a, b string) error {
	if existing, _ := netlink.LinkByName(a); existing != nil {
		logger.UpfLog.Info("Removing stale veth pair", zap.String("link", a))

		if err := netlink.LinkDel(existing); err != nil {
			return fmt.Errorf("delete stale %s: %w", a, err)
		}
	}

	veth := &netlink.Veth{
		LinkAttrs: netlink.LinkAttrs{
			Name: a,
			MTU:  vethMTU,
		},
		PeerName: b,
	}

	if err := netlink.LinkAdd(veth); err != nil {
		return fmt.Errorf("create veth pair: %w", err)
	}

	for _, name := range []string{a, b} {
		link, err := netlink.LinkByName(name)
		if err != nil {
			// Best-effort cleanup on failure.
			_ = destroyVethPair(a)
			return fmt.Errorf("lookup %s after creation: %w", name, err)
		}

		if err := netlink.LinkSetUp(link); err != nil {
			_ = destroyVethPair(a)
			return fmt.Errorf("set %s up: %w", name, err)
		}
	}

	logger.UpfLog.Info("Created veth pair",
		zap.String("a", a),
		zap.String("b", b),
		zap.Int("mtu", vethMTU),
	)

	return nil
}

// destroyVethPair removes the pair anchored at name. Safe when absent.
func destroyVethPair(name string) error {
	link, err := netlink.LinkByName(name)
	if err != nil {
		if _, ok := err.(netlink.LinkNotFoundError); ok {
			return nil
		}

		return fmt.Errorf("lookup %s for deletion: %w", name, err)
	}

	if err := netlink.LinkDel(link); err != nil {
		return fmt.Errorf("delete %s: %w", name, err)
	}

	logger.UpfLog.Info("Destroyed veth pair", zap.String("link", name))

	return nil
}

func vethIndex(name string) (int, error) {
	iface, err := net.InterfaceByName(name)
	if err != nil {
		return 0, fmt.Errorf("lookup %s: %w", name, err)
	}

	return iface.Index, nil
}
