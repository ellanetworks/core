// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: Apache-2.0

//go:build linux

package upf

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"unsafe"

	cebpf "github.com/cilium/ebpf"
	"github.com/cilium/ebpf/link"
	"github.com/ellanetworks/core/internal/config"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/upf/ebpf"
	"go.uber.org/zap"
	"golang.org/x/sys/unix"
)

// bpfPinDir holds the datapath's link pins. A pinned link keeps the program
// attached across an unclean process exit, so a restart re-adopts the running
// datapath without a traffic gap. Pinning is best-effort: without a writable
// bpffs the links live only as long as the process, which is the behavior of
// an unpinned attach.
const bpfPinDir = "/sys/fs/bpf/ella-core"

// pinnedLink unpins on Close so a clean shutdown detaches the datapath, while
// a crash leaves the pin for the next process to adopt.
type pinnedLink struct {
	link.Link
	pinPath string
}

func (p *pinnedLink) Close() error {
	if p.pinPath != "" {
		if err := p.Unpin(); err != nil && !errors.Is(err, os.ErrNotExist) {
			_ = p.Link.Close()
			return fmt.Errorf("unpin datapath link %s: %w", p.pinPath, err)
		}
	}

	return p.Link.Close()
}

// datapathPinName is the pin for one interface at one hook. The XDP name
// carries the mode because bpf_xdp_link_update validates only the program type
// and expected attach type: updating a pinned generic link with a native-mode
// program succeeds and reinstalls in the pinned link's own mode. The mode in
// the name keeps each pin to the mode that created it.
func datapathPinName(hook, ifname string) string {
	return hook + "-" + ifname
}

// datapathPinNames lists every pin the datapath may hold for an interface,
// across hooks and XDP modes, including the unqualified xdp-<if> name.
func datapathPinNames(ifname string) []string {
	return []string{
		datapathPinName("xdp", ifname),
		datapathPinName("xdp-native", ifname),
		datapathPinName("xdp-generic", ifname),
		datapathPinName("tcx-ingress", ifname),
	}
}

// releaseForeignPins drops every pin for ifname other than keep. A pin holds a
// reference that keeps its program attached, so one left by a previous hook or
// mode would run alongside the new attach: XDP ahead of TCX, encapsulating,
// translating and metering each packet twice.
func releaseForeignPins(ifname, keep string) error {
	for _, name := range datapathPinNames(ifname) {
		if name == keep {
			continue
		}

		pinPath := filepath.Join(bpfPinDir, name)

		l, err := link.LoadPinnedLink(pinPath, nil)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}

			return fmt.Errorf("inspect pin %s: %w", pinPath, err)
		}

		err = l.Unpin()
		_ = l.Close()

		if err != nil {
			return fmt.Errorf("unpin datapath link %s: %w", pinPath, err)
		}

		logger.UpfLog.Info("released datapath link pinned at another hook",
			zap.String("pin", pinPath))
	}

	return nil
}

// adoptOrAttach re-points a pinned link from a previous process at prog, or
// attaches fresh via attach and pins the result. Pins belonging to another
// hook or XDP mode are released first, then a stale pin of this name that
// cannot be updated (incompatible program) is discarded before the fresh
// attach so hooks cannot stack.
func adoptOrAttach(ifname, pinName string, prog *cebpf.Program, attach func() (link.Link, error)) (link.Link, error) {
	pinPath := filepath.Join(bpfPinDir, pinName)

	if err := releaseForeignPins(ifname, pinName); err != nil {
		return nil, err
	}

	if l, err := link.LoadPinnedLink(pinPath, nil); err == nil {
		if err := l.Update(prog); err == nil {
			logger.UpfLog.Info("adopted pinned datapath link", zap.String("pin", pinPath))
			return &pinnedLink{Link: l, pinPath: pinPath}, nil
		}

		unpinErr := l.Unpin()
		_ = l.Close()

		if unpinErr != nil {
			return nil, fmt.Errorf("unpin stale datapath link %s: %w", pinPath, unpinErr)
		}
	}

	l, err := attach()
	if err != nil {
		return nil, err
	}

	if err := os.MkdirAll(bpfPinDir, 0o750); err != nil {
		logger.UpfLog.Warn("datapath link not pinned; it detaches with the process",
			zap.String("dir", bpfPinDir), zap.Error(err))

		return l, nil
	}

	if err := l.Pin(pinPath); err != nil {
		logger.UpfLog.Warn("datapath link not pinned; it detaches with the process",
			zap.String("pin", pinPath), zap.Error(err))

		return l, nil
	}

	return &pinnedLink{Link: l, pinPath: pinPath}, nil
}

// attachXDP attaches prog at the XDP hook of the interface in the given mode,
// adopting a pinned link when one exists.
func attachXDP(prog *cebpf.Program, ifindex int, ifname string, flags link.XDPAttachFlags) (link.Link, error) {
	hook := "xdp-native"
	if flags == link.XDPGenericMode {
		hook = "xdp-generic"
	}

	return adoptOrAttach(ifname, datapathPinName(hook, ifname), prog, func() (link.Link, error) {
		return link.AttachXDP(link.XDPOptions{
			Program:   prog,
			Interface: ifindex,
			Flags:     flags,
		})
	})
}

// attachTCX attaches prog at TCX ingress of the interface, adopting a pinned
// link when one exists. Reload goes through link.Update on the held link: a
// second AttachTCX would stack a program on the hook.
func attachTCX(prog *cebpf.Program, ifindex int, ifname string) (link.Link, error) {
	return adoptOrAttach(ifname, datapathPinName("tcx-ingress", ifname), prog, func() (link.Link, error) {
		return link.AttachTCX(link.TCXOptions{
			Program:   prog,
			Attach:    cebpf.AttachTCXIngress,
			Interface: ifindex,
		})
	})
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

	n3Link, n6Link, err := attachBothXDP(objs, n3, n6, link.XDPDriverMode)
	if err == nil {
		return config.DatapathXDPNative, n3Link, n6Link, nil
	}

	if !errors.Is(err, unix.EOPNOTSUPP) {
		return "", nil, nil, err
	}

	logger.UpfLog.Info("no driver-level XDP support on the datapath interfaces, attaching at TCX",
		zap.String("n3", n3.name), zap.String("n6", n6.name))

	// The XDP object cannot serve a TCX hook: reload as SCHED_CLS. Every map
	// and program handle taken from objs before this point refers to the
	// closed object, so map readers must be constructed after attachDatapath
	// returns.
	if err := objs.Close(); err != nil {
		logger.UpfLog.Warn("failed to close XDP objects before TCX fallback", zap.Error(err))
	}

	objs.UseTCX = true
	if err := objs.Load(); err != nil {
		return "", nil, nil, fmt.Errorf("load TCX datapath objects: %w", err)
	}

	n3Link, n6Link, err = attachBothTCX(objs, n3, n6)
	if err != nil {
		/* The reloaded objects are unreachable to the caller once the
		 * attach fails, so they are released here. */
		if closeErr := objs.Close(); closeErr != nil {
			logger.UpfLog.Warn("failed to close TCX objects after a failed attach",
				zap.Error(closeErr))
		}

		return "", nil, nil, err
	}

	return config.DatapathTCX, n3Link, n6Link, nil
}

func attachBothXDP(objs *ebpf.BpfObjects, n3, n6 datapathIface, flags link.XDPAttachFlags) (link.Link, *link.Link, error) {
	n3Link, err := attachXDP(objs.UpfEntryFunc, n3.index, n3.name, flags)
	if err != nil {
		return nil, nil, fmt.Errorf("attach datapath on n3 interface %q: %w", n3.name, err)
	}

	if n6.index == n3.index {
		return n3Link, nil, nil
	}

	n6Link, err := attachXDP(objs.UpfEntryFunc, n6.index, n6.name, flags)
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

// warnGROOnTCX logs the TCX GSO exposure when the N6 receive interface has
// GRO enabled: encapsulated super-frames are segmented with the super-frame's
// GTP message_length on every segment, and an IPv6 outer additionally carries
// a stale outer UDP checksum. GRO-off on this interface removes both.
func warnGROOnTCX(n6Ifname string) {
	enabled, err := interfaceGROEnabled(n6Ifname)
	if err != nil {
		logger.UpfLog.Debug("could not read GRO state", zap.String("iface", n6Ifname), zap.Error(err))
		return
	}

	if enabled {
		logger.UpfLog.Warn("GRO is enabled on the N6 interface while the datapath is attached at TCX: "+
			"large downlink flows are encapsulated as GSO super-frames whose segments carry the "+
			"super-frame's GTP message_length, and IPv6 GTP-U transport is unsupported; disable with "+
			"`ethtool -K <iface> gro off` if peers reject such traffic",
			zap.String("iface", n6Ifname))
	}
}
