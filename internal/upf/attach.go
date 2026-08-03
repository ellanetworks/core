// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: Apache-2.0

//go:build linux

package upf

import (
	"errors"
	"fmt"
	"unsafe"

	cebpf "github.com/cilium/ebpf"
	"github.com/cilium/ebpf/link"
	"github.com/ellanetworks/core/internal/config"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/upf/ebpf"
	"github.com/vishvananda/netlink"
	"go.uber.org/zap"
	"golang.org/x/sys/unix"
)

// attachXDP attaches prog at the XDP hook of the interface in the given mode.
// The returned link owns the attachment: closing it, or exiting, detaches.
func attachXDP(prog *cebpf.Program, ifindex int, flags link.XDPAttachFlags) (link.Link, error) {
	return link.AttachXDP(link.XDPOptions{
		Program:   prog,
		Interface: ifindex,
		Flags:     flags,
	})
}

// attachTCX attaches prog at TCX ingress of the interface.
//
// TCX is a multi-program hook: unlike XDP, which refuses a second attach with
// EBUSY, it accepts one and runs both. A second instance of this daemon would
// therefore double-process every frame — encapsulating, translating and
// metering it twice — rather than failing to start, so that case is refused
// here. Foreign programs are left alone: sharing the hook with a CNI or an
// observability agent is what TCX is for.
func attachTCX(prog *cebpf.Program, ifindex int, ifname string) (link.Link, error) {
	if err := datapathAlreadyAttached(prog, ifindex); err != nil {
		return nil, fmt.Errorf("%s: %w", ifname, err)
	}

	return link.AttachTCX(link.TCXOptions{
		Program:   prog,
		Attach:    cebpf.AttachTCXIngress,
		Interface: ifindex,
	})
}

// datapathAlreadyAttached reports an error when a program with the same name
// as prog is already at TCX ingress on ifindex, which means another instance
// of this daemon holds the hook. A kernel that cannot be queried is treated as
// empty: refusing to start on a probe failure is worse than the stacking this
// guards against.
func datapathAlreadyAttached(prog *cebpf.Program, ifindex int) error {
	info, err := prog.Info()
	if err != nil {
		return nil
	}

	name := info.Name

	attached, err := link.QueryPrograms(link.QueryOptions{
		Target: ifindex,
		Attach: cebpf.AttachTCXIngress,
	})
	if err != nil || attached == nil {
		return nil
	}

	for _, ap := range attached.Programs {
		other, err := cebpf.NewProgramFromID(ap.ID)
		if err != nil {
			continue
		}

		otherInfo, err := other.Info()
		_ = other.Close()

		if err == nil && otherInfo.Name == name {
			return fmt.Errorf("%q is already attached at TCX ingress: refusing to stack a second datapath", name)
		}
	}

	return nil
}

// loadDatapathObjects loads the object the requested mechanism can actually
// attach: a TCX hook takes SCHED_CLS programs, every XDP mode takes XDP
// programs. The default chain starts at native XDP and reloads in
// attachDatapath if it falls back to TCX.
func loadDatapathObjects(objs *ebpf.BpfObjects, mode string) error {
	objs.UseTCX = mode == config.DatapathTCX

	return objs.Load()
}

// datapathIface is one side of the datapath: the netdev the program attaches
// to, already resolved through any VLAN master.
type datapathIface struct {
	index int
	name  string
}

// attachDatapath loads the object matching the requested mechanism and
// attaches it to both interfaces, returning the mechanism actually used.
//
// config.DatapathChain tries driver-level XDP first and falls back to TCX
// when the NIC has no XDP support; generic is never reached that way. The
// fallback reloads the objects because the two hooks need different program
// types, which is a one-off cost at startup on non-native NICs.
func attachDatapath(objs *ebpf.BpfObjects, mode string, n3, n6 datapathIface) (string, link.Link, *link.Link, error) {
	switch mode {
	case config.DatapathXDPNative, config.DatapathXDPGeneric:
		flags := link.XDPDriverMode
		if mode == config.DatapathXDPGeneric {
			flags = link.XDPGenericMode
		}

		n3Link, n6Link, err := attachBothXDP(objs, n3, n6, flags)

		return mode, n3Link, n6Link, err

	case config.DatapathTCX:
		n3Link, n6Link, err := attachBothTCX(objs, n3, n6)

		return mode, n3Link, n6Link, err
	}

	// A veth accepts a native attach and then drops every redirected frame,
	// so the EOPNOTSUPP fallback below never fires for it. Skip straight to
	// TCX rather than blackhole the datapath.
	for _, iface := range []datapathIface{n3, n6} {
		if nativeXDPBlackholes(iface.name) {
			logger.UpfLog.Info("interface cannot forward redirected frames in native XDP, attaching at TCX",
				zap.String("iface", iface.name))

			return attachChainTCX(objs, n3, n6)
		}
	}

	n3Link, n6Link, err := attachBothXDP(objs, n3, n6, link.XDPDriverMode)
	if err == nil {
		return config.DatapathXDPNative, n3Link, n6Link, nil
	}

	if !errors.Is(err, unix.EOPNOTSUPP) {
		return "", nil, nil, err
	}

	logger.UpfLog.Info("no driver-level XDP support on the datapath interfaces, attaching at TCX",
		zap.String("n3", n3.name), zap.String("n6", n6.name))

	return attachChainTCX(objs, n3, n6)
}

// attachChainTCX is the chain's TCX leg. The XDP object cannot serve a TCX
// hook, so the objects are reloaded as SCHED_CLS: every map and program handle
// taken before this point refers to the closed object, and map readers must be
// constructed after attachDatapath returns.
func attachChainTCX(objs *ebpf.BpfObjects, n3, n6 datapathIface) (string, link.Link, *link.Link, error) {
	if err := objs.Close(); err != nil {
		logger.UpfLog.Warn("failed to close XDP objects before TCX fallback", zap.Error(err))
	}

	objs.UseTCX = true
	if err := objs.Load(); err != nil {
		return "", nil, nil, fmt.Errorf("load TCX datapath objects: %w", err)
	}

	n3Link, n6Link, err := attachBothTCX(objs, n3, n6)
	if err != nil {
		// The reloaded objects are unreachable to the caller once the attach
		// fails, so they are released here.
		if closeErr := objs.Close(); closeErr != nil {
			logger.UpfLog.Warn("failed to close TCX objects after a failed attach",
				zap.Error(closeErr))
		}

		return "", nil, nil, err
	}

	return config.DatapathTCX, n3Link, n6Link, nil
}

func attachBothXDP(objs *ebpf.BpfObjects, n3, n6 datapathIface, flags link.XDPAttachFlags) (link.Link, *link.Link, error) {
	n3Link, err := attachXDP(objs.UpfEntryFunc, n3.index, flags)
	if err != nil {
		return nil, nil, fmt.Errorf("attach datapath on n3 interface %q: %w", n3.name, err)
	}

	if n6.index == n3.index {
		return n3Link, nil, nil
	}

	n6Link, err := attachXDP(objs.UpfEntryFunc, n6.index, flags)
	if err != nil {
		_ = n3Link.Close()

		return nil, nil, fmt.Errorf("attach datapath on n6 interface %q: %w", n6.name, err)
	}

	return n3Link, &n6Link, nil
}

func attachBothTCX(objs *ebpf.BpfObjects, n3, n6 datapathIface) (link.Link, *link.Link, error) {
	n3Link, err := attachTCX(objs.UpfEntryFunc, n3.index, n3.name)
	if err != nil {
		if tcxUnavailable(err) {
			return nil, nil, fmt.Errorf("attach datapath at TCX on %q: TCX needs kernel 6.6 or newer: %w", n3.name, err)
		}

		return nil, nil, fmt.Errorf("attach datapath at TCX on n3 interface %q: %w", n3.name, err)
	}

	if n6.index == n3.index {
		return n3Link, nil, nil
	}

	n6Link, err := attachTCX(objs.UpfEntryFunc, n6.index, n6.name)
	if err != nil {
		_ = n3Link.Close()

		return nil, nil, fmt.Errorf("attach datapath at TCX on n6 interface %q: %w", n6.name, err)
	}

	return n3Link, &n6Link, nil
}

// tcxUnavailable reports the kernel lacking TCX support. EINVAL is not a
// signal here: the kernel also returns it for an attach the program cannot
// serve, such as an XDP program offered to a TCX hook.
func tcxUnavailable(err error) bool {
	return errors.Is(err, cebpf.ErrNotSupported)
}

const ethtoolGGRO = 0x2b

// nativeXDPBlackholes reports whether a redirect out of this interface would
// be dropped in native XDP mode. On a veth, xdp_features carries
// NETDEV_XDP_ACT_NDO_XMIT only while the peer has its own XDP program or GRO
// (drivers/net/veth.c), and veth_xdp_xmit refuses without the peer's NAPI —
// so the attach succeeds and the traffic disappears.
func nativeXDPBlackholes(ifname string) bool {
	l, err := netlink.LinkByName(ifname)
	if err != nil {
		logger.UpfLog.Debug("could not read interface type",
			zap.String("iface", ifname), zap.Error(err))

		return false
	}

	return l.Type() == "veth"
}

// interfaceGROEnabled reads the interface's generic-receive-offload state via
// the ETHTOOL_GGRO ioctl.
func interfaceGROEnabled(ifname string) (bool, error) {
	fd, err := unix.Socket(unix.AF_INET, unix.SOCK_DGRAM, 0)
	if err != nil {
		return false, err
	}

	defer func() { _ = unix.Close(fd) }()

	value := struct {
		cmd  uint32
		data uint32
	}{cmd: ethtoolGGRO}

	// Padded to sizeof(struct ifreq): the kernel copies the full structure
	// in, so a shorter one is read past its end.
	var ifr struct {
		name [unix.IFNAMSIZ]byte
		data unsafe.Pointer
		_    [16]byte
	}

	if len(ifname) >= unix.IFNAMSIZ {
		return false, fmt.Errorf("interface name %q too long", ifname)
	}

	copy(ifr.name[:], ifname)
	ifr.data = unsafe.Pointer(&value)

	if _, _, errno := unix.Syscall(unix.SYS_IOCTL, uintptr(fd),
		unix.SIOCETHTOOL, uintptr(unsafe.Pointer(&ifr))); errno != 0 {
		return false, fmt.Errorf("ETHTOOL_GGRO %s: %w", ifname, errno)
	}

	return value.data != 0, nil
}

// warnMergedPacketSources warns when an interface has GRO enabled under TCX.
// It covers only the receive-side merge: a veth or virtio peer that offloads
// segmentation delivers merged packets with GRO off, which no local feature
// reports. Both directions matter — the uplink drops merged packets too.
func warnMergedPacketSources(ifnames ...string) {
	for _, ifname := range ifnames {
		enabled, err := interfaceGROEnabled(ifname)
		if err != nil {
			logger.UpfLog.Debug("could not read GRO state", zap.String("iface", ifname), zap.Error(err))
			continue
		}

		if enabled {
			logger.UpfLog.Warn("GRO is enabled on a datapath interface while attached at TCX: "+
				"merged packets cannot be encapsulated or decapsulated into valid GTP-U and are "+
				"dropped, counted in app_upf_datapath_drop_total under encap_gso and decap_gso; "+
				"see the disable-merged-packets guide for the knob this deployment needs",
				zap.String("iface", ifname))
		}
	}
}
