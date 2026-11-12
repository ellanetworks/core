// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: Apache-2.0

//go:build linux

package upf

import (
	"encoding/binary"
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

// The returned link owns the attachment: closing it, or exiting, detaches.
func attachXDP(prog *cebpf.Program, ifindex int, flags link.XDPAttachFlags) (link.Link, error) {
	return link.AttachXDP(link.XDPOptions{
		Program:   prog,
		Interface: ifindex,
		Flags:     flags,
	})
}

// TCX is a multi-program hook: unlike XDP, which refuses a second attach with
// EBUSY, it accepts one and runs both, so a second instance of this daemon
// would double-process every frame rather than fail to start. Foreign programs
// are left alone: sharing the hook with a CNI or an observability agent is what
// TCX is for.
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

// A program with the same name at TCX ingress means another instance of this
// daemon holds the hook. A kernel that cannot be queried is treated as empty:
// refusing to start on a probe failure is worse than the stacking this guards
// against.
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

// A TCX hook takes SCHED_CLS programs and every XDP mode takes XDP programs.
// The default chain starts at native XDP and reloads in attachDatapath if it
// falls back to TCX.
func loadDatapathObjects(objs *ebpf.BpfObjects, mode string) error {
	objs.UseTCX = mode == config.DatapathTCX

	return objs.Load()
}

// The netdev the program attaches to, already resolved through any VLAN
// master.
type datapathIface struct {
	index  int
	name   string
	tagged bool
}

// attachDatapath attaches the datapath to both interfaces and returns the
// mechanism actually used. config.DatapathChain tries driver-level XDP first
// and falls back to TCX, reloading the objects because the two hooks need
// different program types.
func attachDatapath(objs *ebpf.BpfObjects, mode string, n3, n6 datapathIface, extra []datapathIface) (string, link.Link, *link.Link, []link.Link, error) {
	attachedMode, n3Link, n6Link, err := attachN3N6(objs, mode, n3, n6, extra)
	if err != nil {
		return "", nil, nil, nil, err
	}

	extraLinks, err := attachExtra(objs, attachedMode, extra)
	if err == nil {
		return attachedMode, n3Link, n6Link, extraLinks, nil
	}

	closeN3N6(n3Link, n6Link)

	if mode != config.DatapathChain || attachedMode != config.DatapathXDPNative || !driverRefusedXDP(err) {
		return "", nil, nil, nil, err
	}

	logger.UpfLog.Info("no driver-level XDP support on a network instance interface, attaching at TCX",
		zap.Error(err))

	attachedMode, n3Link, n6Link, err = attachChainTCX(objs, n3, n6)
	if err != nil {
		return "", nil, nil, nil, err
	}

	extraLinks, err = attachExtra(objs, attachedMode, extra)
	if err != nil {
		closeN3N6(n3Link, n6Link)

		return "", nil, nil, nil, err
	}

	return attachedMode, n3Link, n6Link, extraLinks, nil
}

func closeN3N6(n3Link link.Link, n6Link *link.Link) {
	if n6Link != nil {
		_ = (*n6Link).Close()
	}

	_ = n3Link.Close()
}

func attachExtra(objs *ebpf.BpfObjects, attachedMode string, extra []datapathIface) ([]link.Link, error) {
	links := make([]link.Link, 0, len(extra))

	for _, iface := range extra {
		l, err := attachInterface(objs, attachedMode, iface)
		if err != nil {
			for _, attached := range links {
				_ = attached.Close()
			}

			return nil, fmt.Errorf("attach datapath on network instance interface %q: %w", iface.name, err)
		}

		links = append(links, l)
	}

	return links, nil
}

func attachInterface(objs *ebpf.BpfObjects, attachedMode string, iface datapathIface) (link.Link, error) {
	switch attachedMode {
	case config.DatapathTCX:
		disableMergedPackets(attachedMode, iface.name)

		return attachTCX(objs.UpfEntryFunc, iface.index, iface.name)
	case config.DatapathXDPGeneric:
		disableMergedPackets(attachedMode, iface.name)

		return attachXDP(objs.UpfEntryFunc, iface.index, link.XDPGenericMode)
	default:
		return attachXDP(objs.UpfEntryFunc, iface.index, link.XDPDriverMode)
	}
}

func attachN3N6(objs *ebpf.BpfObjects, mode string, n3, n6 datapathIface, extra []datapathIface) (string, link.Link, *link.Link, error) {
	all := append([]datapathIface{n3, n6}, extra...)

	switch mode {
	case config.DatapathXDPNative, config.DatapathXDPGeneric:
		flags := link.XDPDriverMode
		if mode == config.DatapathXDPGeneric {
			flags = link.XDPGenericMode
		}

		if mode == config.DatapathXDPNative {
			for _, iface := range all {
				if nativeXDPBlackholes(iface.name) {
					logger.UpfLog.Warn("configured attach mode cannot forward redirected frames on this interface: "+
						"the attach succeeds and every redirected frame is dropped, use tcx instead",
						zap.String("iface", iface.name), zap.String("mode", mode))
				}

				warnVLANStrippedBeforeXDP(iface, mode)
			}
		} else {
			disableMergedPackets(mode, datapathNames(n3, n6)...)
		}

		n3Link, n6Link, err := attachBothXDP(objs, n3, n6, flags)

		return mode, n3Link, n6Link, err

	case config.DatapathTCX:
		n3Link, n6Link, err := attachBothTCX(objs, n3, n6)

		return mode, n3Link, n6Link, err
	}

	// A veth accepts a native attach and then drops every redirected frame,
	// so the driver-refusal fallback below never fires for it.
	for _, iface := range all {
		if nativeXDPBlackholes(iface.name) {
			logger.UpfLog.Info("interface cannot forward redirected frames in native XDP, attaching at TCX",
				zap.String("iface", iface.name))

			return attachChainTCX(objs, n3, n6)
		}

		if iface.tagged && vlanStrippedBeforeXDP(iface.name) {
			logger.UpfLog.Info("interface strips VLAN tags before native XDP, attaching at TCX",
				zap.String("iface", iface.name))

			return attachChainTCX(objs, n3, n6)
		}
	}

	n3Link, n6Link, err := attachBothXDP(objs, n3, n6, link.XDPDriverMode)
	if err == nil {
		return config.DatapathXDPNative, n3Link, n6Link, nil
	}

	if !driverRefusedXDP(err) {
		return "", nil, nil, err
	}

	logger.UpfLog.Info("no driver-level XDP support on the datapath interfaces, attaching at TCX",
		zap.String("n3", n3.name), zap.String("n6", n6.name), zap.Error(err))

	return attachChainTCX(objs, n3, n6)
}

func driverRefusedXDP(err error) bool {
	return errors.Is(err, unix.EOPNOTSUPP) || errors.Is(err, unix.EINVAL)
}

// The chain's TCX leg reloads the objects as SCHED_CLS, so every map and
// program handle taken before this point refers to the closed object: map
// readers must be constructed after attachDatapath returns.
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
		// The caller cannot reach the reloaded objects once the attach fails.
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

func datapathNames(n3, n6 datapathIface) []string {
	if n6.index == n3.index {
		return []string{n3.name}
	}

	return []string{n3.name, n6.name}
}

func attachBothTCX(objs *ebpf.BpfObjects, n3, n6 datapathIface) (link.Link, *link.Link, error) {
	disableMergedPackets(config.DatapathTCX, datapathNames(n3, n6)...)

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

// EINVAL is not a signal of a missing TCX: the kernel also returns it for an
// attach the program cannot serve, such as an XDP program offered to a TCX
// hook.
func tcxUnavailable(err error) bool {
	return errors.Is(err, cebpf.ErrNotSupported)
}

const (
	ethtoolGStrings  = 0x1b
	ethtoolGFlags    = 0x25
	ethtoolGGRO      = 0x2b
	ethtoolGSsetInfo = 0x37
	ethtoolGFeatures = 0x3a
	ethtoolSFeatures = 0x3b
	ethFlagRXVLAN    = 1 << 8
	ethSSFeatures    = 4
	ethGStringLen    = 32
)

var mergingFeatures = []string{"rx-gro", "rx-gro-hw", "rx-lro"}

// Whether a redirect out of this interface would be dropped in native XDP
// mode. On a veth, xdp_features carries NETDEV_XDP_ACT_NDO_XMIT only while the
// peer has its own XDP program or GRO (drivers/net/veth.c), and veth_xdp_xmit
// refuses without the peer's NAPI, so the attach succeeds and the traffic
// disappears. A probe failure returns false and lets the attach proceed.
func nativeXDPBlackholes(ifname string) bool {
	l, err := netlink.LinkByName(ifname)
	if err != nil {
		logger.UpfLog.Warn("could not read interface type, assuming native XDP can forward redirected frames",
			zap.String("iface", ifname), zap.Error(err))

		return false
	}

	return l.Type() == "veth"
}

func interfaceGROEnabled(ifname string) (bool, error) {
	value, err := ethtoolValue(ifname, ethtoolGGRO)
	if err != nil {
		return false, fmt.Errorf("ETHTOOL_GGRO %s: %w", ifname, err)
	}

	return value != 0, nil
}

func vlanStrippedBeforeXDP(ifname string) bool {
	flags, err := ethtoolValue(ifname, ethtoolGFlags)
	if err != nil {
		logger.UpfLog.Warn("could not read VLAN offload, assuming native XDP sees VLAN tags",
			zap.String("iface", ifname), zap.Error(err))

		return false
	}

	return flags&ethFlagRXVLAN != 0
}

func warnVLANStrippedBeforeXDP(iface datapathIface, mode string) {
	if mode != config.DatapathXDPNative || !iface.tagged || !vlanStrippedBeforeXDP(iface.name) {
		return
	}

	logger.UpfLog.Warn("interface strips VLAN tags before native XDP, so the datapath cannot classify its VLANs: "+
		"turn off rx-vlan-offload on it or use tcx",
		zap.String("iface", iface.name), zap.String("mode", mode))
}

func ethtoolValue(ifname string, cmd uint32) (uint32, error) {
	value := struct {
		cmd  uint32
		data uint32
	}{cmd: cmd}

	if err := ethtoolCall(ifname, unsafe.Pointer(&value)); err != nil {
		return 0, err
	}

	return value.data, nil
}

func ethtoolCall(ifname string, data unsafe.Pointer) error {
	fd, err := unix.Socket(unix.AF_INET, unix.SOCK_DGRAM, 0)
	if err != nil {
		return err
	}

	defer func() { _ = unix.Close(fd) }()

	// Padded to sizeof(struct ifreq): the kernel copies the full structure
	// in, so a shorter one is read past its end.
	var ifr struct {
		name [unix.IFNAMSIZ]byte
		data unsafe.Pointer
		_    [16]byte
	}

	if len(ifname) >= unix.IFNAMSIZ {
		return fmt.Errorf("interface name %q too long", ifname)
	}

	copy(ifr.name[:], ifname)
	ifr.data = data

	if _, _, errno := unix.Syscall(unix.SYS_IOCTL, uintptr(fd),
		unix.SIOCETHTOOL, uintptr(unsafe.Pointer(&ifr))); errno != 0 {
		return errno
	}

	return nil
}

func featureNames(ifname string) ([]string, error) {
	info := make([]byte, 20)
	binary.NativeEndian.PutUint32(info[0:], ethtoolGSsetInfo)
	binary.NativeEndian.PutUint64(info[8:], 1<<ethSSFeatures)

	if err := ethtoolCall(ifname, unsafe.Pointer(&info[0])); err != nil {
		return nil, fmt.Errorf("ETHTOOL_GSSET_INFO %s: %w", ifname, err)
	}

	if binary.NativeEndian.Uint64(info[8:]) == 0 {
		return nil, fmt.Errorf("ETHTOOL_GSSET_INFO %s: no feature names", ifname)
	}

	count := int(binary.NativeEndian.Uint32(info[16:]))

	strs := make([]byte, 12+count*ethGStringLen)
	binary.NativeEndian.PutUint32(strs[0:], ethtoolGStrings)
	binary.NativeEndian.PutUint32(strs[4:], ethSSFeatures)
	binary.NativeEndian.PutUint32(strs[8:], uint32(count))

	if err := ethtoolCall(ifname, unsafe.Pointer(&strs[0])); err != nil {
		return nil, fmt.Errorf("ETHTOOL_GSTRINGS %s: %w", ifname, err)
	}

	if got := int(binary.NativeEndian.Uint32(strs[8:])); got != count {
		return nil, fmt.Errorf("ETHTOOL_GSTRINGS %s: kernel returned %d feature names, want %d", ifname, got, count)
	}

	names := make([]string, count)

	for i := range names {
		raw := strs[12+i*ethGStringLen : 12+(i+1)*ethGStringLen]
		end := 0

		for end < len(raw) && raw[end] != 0 {
			end++
		}

		names[i] = string(raw[:end])
	}

	return names, nil
}

func getFeatures(ifname string, words int) (available, active []uint32, err error) {
	buf := make([]byte, 8+words*16)
	binary.NativeEndian.PutUint32(buf[0:], ethtoolGFeatures)
	binary.NativeEndian.PutUint32(buf[4:], uint32(words))

	if err := ethtoolCall(ifname, unsafe.Pointer(&buf[0])); err != nil {
		return nil, nil, fmt.Errorf("ETHTOOL_GFEATURES %s: %w", ifname, err)
	}

	if got := int(binary.NativeEndian.Uint32(buf[4:])); got != words {
		return nil, nil, fmt.Errorf("ETHTOOL_GFEATURES %s: kernel has %d feature words, want %d", ifname, got, words)
	}

	available = make([]uint32, words)
	active = make([]uint32, words)

	for w := range words {
		available[w] = binary.NativeEndian.Uint32(buf[8+w*16:])
		active[w] = binary.NativeEndian.Uint32(buf[8+w*16+8:])
	}

	return available, active, nil
}

func clearFeatures(ifname string, valid []uint32) error {
	buf := make([]byte, 8+len(valid)*8)
	binary.NativeEndian.PutUint32(buf[0:], ethtoolSFeatures)
	binary.NativeEndian.PutUint32(buf[4:], uint32(len(valid)))

	for w, bits := range valid {
		binary.NativeEndian.PutUint32(buf[8+w*8:], bits)
	}

	if err := ethtoolCall(ifname, unsafe.Pointer(&buf[0])); err != nil {
		return fmt.Errorf("ETHTOOL_SFEATURES %s: %w", ifname, err)
	}

	return nil
}

func disableReceiveMerging(ifname string) (disabled, stillOn []string, err error) {
	names, err := featureNames(ifname)
	if err != nil {
		return nil, nil, err
	}

	words := (len(names) + 31) / 32

	available, active, err := getFeatures(ifname, words)
	if err != nil {
		return nil, nil, err
	}

	bits := make(map[string]int, len(mergingFeatures))

	for i, name := range names {
		for _, want := range mergingFeatures {
			if name == want {
				bits[name] = i
			}
		}
	}

	valid := make([]uint32, words)

	for _, name := range mergingFeatures {
		i, ok := bits[name]
		if !ok {
			continue
		}

		w, bit := i/32, uint32(1)<<(i%32)

		if active[w]&bit == 0 {
			continue
		}

		if available[w]&bit == 0 {
			stillOn = append(stillOn, name)
			continue
		}

		valid[w] |= bit
	}

	if !anyBitSet(valid) {
		return nil, stillOn, nil
	}

	if err := clearFeatures(ifname, valid); err != nil {
		return nil, stillOn, err
	}

	_, active, err = getFeatures(ifname, words)
	if err != nil {
		return nil, stillOn, err
	}

	for _, name := range mergingFeatures {
		i, ok := bits[name]
		if !ok {
			continue
		}

		w, bit := i/32, uint32(1)<<(i%32)

		if valid[w]&bit == 0 {
			continue
		}

		if active[w]&bit != 0 {
			stillOn = append(stillOn, name)
		} else {
			disabled = append(disabled, name)
		}
	}

	return disabled, stillOn, nil
}

func anyBitSet(words []uint32) bool {
	for _, w := range words {
		if w != 0 {
			return true
		}
	}

	return false
}

// Covers only the receive-side merge: a veth or virtio peer that offloads
// segmentation delivers merged packets with GRO off, which no local feature
// reports.
func disableMergedPackets(mode string, ifnames ...string) {
	for _, ifname := range ifnames {
		if lower, ok := mergingLowerDevice(ifname); ok {
			warnMergingOnLowerDevice(ifname, lower, mode)

			continue
		}

		if mode != config.DatapathTCX {
			continue
		}

		disabled, stillOn, err := disableReceiveMerging(ifname)
		if err != nil {
			logger.UpfLog.Warn("could not turn off packet merging on a datapath interface: "+
				"merged packets cannot be encapsulated or decapsulated into valid GTP-U and are "+
				"dropped, counted in app_upf_datapath_drop_total under encap_gso and decap_gso; "+
				"turn off GRO, hardware GRO and LRO on it with ethtool",
				zap.String("iface", ifname), zap.String("mode", mode), zap.Error(err))

			continue
		}

		if len(disabled) > 0 {
			logger.UpfLog.Info("turned off packet merging on a datapath interface",
				zap.String("iface", ifname), zap.String("mode", mode), zap.Strings("features", disabled))
		}

		if len(stillOn) > 0 {
			logger.UpfLog.Warn("packet merging is fixed on by the driver or hypervisor of a datapath interface "+
				"and cannot be changed: merged packets cannot be encapsulated or decapsulated into valid "+
				"GTP-U and are dropped, counted in app_upf_datapath_drop_total under encap_gso and decap_gso",
				zap.String("iface", ifname), zap.String("mode", mode), zap.Strings("features", stillOn))
		}
	}
}

func mergingLowerDevice(ifname string) (string, bool) {
	l, err := netlink.LinkByName(ifname)
	if err != nil {
		return "", false
	}

	return lowerDevice(l, netlink.LinkByIndex)
}

func lowerDevice(l netlink.Link, byIndex func(int) (netlink.Link, error)) (string, bool) {
	switch l.Type() {
	case "macvlan", "macvtap", "ipvlan":
	case "ipvtap":
		return "", true
	default:
		return "", false
	}

	attrs := l.Attrs()
	if attrs.ParentIndex <= 0 || attrs.NetNsID >= 0 {
		return "", true
	}

	parent, err := byIndex(attrs.ParentIndex)
	if err != nil {
		return "", true
	}

	return parent.Attrs().Name, true
}

func warnMergingOnLowerDevice(ifname, lower, mode string) {
	fields := []zap.Field{zap.String("iface", ifname), zap.String("mode", mode)}
	if lower != "" {
		fields = append(fields, zap.String("lower_device", lower))
	}

	logger.UpfLog.Warn("packet merging for a datapath interface happens on its lower device, which is left unchanged: "+
		"merged packets cannot be encapsulated or decapsulated into valid GTP-U and are dropped, "+
		"counted in app_upf_datapath_drop_total under encap_gso and decap_gso; "+
		"turn off GRO, hardware GRO and LRO on the lower device", fields...)
}
