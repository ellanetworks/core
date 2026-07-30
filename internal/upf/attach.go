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
		if err := p.Unpin(); err != nil {
			logger.UpfLog.Warn("failed to unpin datapath link",
				zap.String("pin", p.pinPath), zap.Error(err))
		}
	}

	return p.Link.Close()
}

// adoptOrAttach re-points a pinned link from a previous process at prog, or
// attaches fresh via attach and pins the result. A stale pin that cannot be
// updated (changed attach mode, incompatible program) is discarded before the
// fresh attach so TCX hooks cannot stack.
func adoptOrAttach(pinName string, prog *cebpf.Program, attach func() (link.Link, error)) (link.Link, error) {
	pinPath := filepath.Join(bpfPinDir, pinName)

	if l, err := link.LoadPinnedLink(pinPath, nil); err == nil {
		if err := l.Update(prog); err == nil {
			logger.UpfLog.Info("adopted pinned datapath link", zap.String("pin", pinPath))
			return &pinnedLink{Link: l, pinPath: pinPath}, nil
		}

		if err := l.Unpin(); err != nil {
			logger.UpfLog.Warn("failed to unpin stale datapath link",
				zap.String("pin", pinPath), zap.Error(err))
		}

		_ = l.Close()
	}

	l, err := attach()
	if err != nil {
		return nil, err
	}

	if err := os.MkdirAll(bpfPinDir, 0o755); err != nil {
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
	return adoptOrAttach("xdp-"+ifname, prog, func() (link.Link, error) {
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
	return adoptOrAttach("tcx-ingress-"+ifname, prog, func() (link.Link, error) {
		return link.AttachTCX(link.TCXOptions{
			Program:   prog,
			Attach:    cebpf.AttachTCXIngress,
			Interface: ifindex,
		})
	})
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

	// The XDP object cannot serve a TCX hook: reload as SCHED_CLS.
	if err := objs.Close(); err != nil {
		logger.UpfLog.Warn("failed to close XDP objects before TCX fallback", zap.Error(err))
	}

	objs.UseTCX = true
	if err := objs.Load(); err != nil {
		return "", nil, nil, fmt.Errorf("load TCX datapath objects: %w", err)
	}

	n3Link, n6Link, err = attachBothTCX(objs, n3, n6)

	return config.DatapathTCX, n3Link, n6Link, err
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

// tcxUnavailable reports the kernel lacking TCX entirely (< 6.6); a per-NIC
// native-XDP refusal is unix.EOPNOTSUPP from the attach instead.
func tcxUnavailable(err error) bool {
	return errors.Is(err, cebpf.ErrNotSupported) || errors.Is(err, unix.EINVAL)
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

	var ifr struct {
		name [unix.IFNAMSIZ]byte
		data unsafe.Pointer
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
