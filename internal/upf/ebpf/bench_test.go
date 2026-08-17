// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: Apache-2.0

//go:build linux

package ebpf

import (
	"encoding/binary"
	"fmt"
	"net"
	"net/netip"
	"os"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"
	"unsafe"

	"github.com/cilium/ebpf"
	"github.com/cilium/ebpf/asm"
	"golang.org/x/sys/unix"
)

const (
	benchIterations = 3000
	benchWarmup     = 500

	verifyIterations = 20
	verifyWarmup     = 5
)

func benchmarking() bool { return os.Getenv("ELLA_BENCH") != "" }

func benchSampleSize() (warmup, iterations int) {
	if benchmarking() {
		return benchWarmup, benchIterations
	}

	return verifyWarmup, verifyIterations
}

const (
	benchN3Dev  = "ellbn3"
	benchN3Peer = "ellbn3p"
	benchN6Dev  = "ellbn6"
	benchN6Peer = "ellbn6p"

	benchN3MAC = "02:00:00:00:0b:aa"
	benchN6MAC = "02:00:00:00:0b:bb"

	benchN6IPv6      = "2001:db8:66::1/64"
	benchServerV6    = "2001:4860:4861::8888"
	benchServerV6Net = "2001:4860:4861::/48"
)

var (
	benchUPFN3IP = [4]byte{198, 18, 1, 1}
	benchGNBIP   = [4]byte{198, 18, 1, 9}
	benchN6IP    = [4]byte{198, 18, 0, 1}
	benchServer  = [4]byte{203, 0, 113, 50}

	benchUE5G = [4]byte{10, 45, 0, 1}
	benchUE4G = [4]byte{10, 45, 0, 2}

	benchUEv65G = netip.MustParseAddr("2001:db8:a1::")
	benchUEv64G = netip.MustParseAddr("2001:db8:a2::")
)

func benchUEv6Dst(prefix netip.Addr) [16]byte {
	a := prefix.As16()
	a[15] = 1

	return a
}

func gtpHeaderPlain(teid uint32, inner []byte) []byte {
	const gtpHdrLen = 8

	gtp := make([]byte, gtpHdrLen)
	gtp[0] = 0x30
	gtp[1] = 0xFF
	binary.BigEndian.PutUint16(gtp[2:4], uint16(len(inner)))
	binary.BigEndian.PutUint32(gtp[4:8], teid)

	return append(gtp, inner...)
}

func uplinkGPDUPlain(teid uint32, inner []byte) []byte {
	return gtpV4Outer(gtpHeaderPlain(teid, inner))
}

func benchNopProgram(t *testing.T) *ebpf.Program {
	t.Helper()

	prog, err := ebpf.NewProgram(&ebpf.ProgramSpec{
		Name: "bench_nop",
		Type: ebpf.XDP,
		Instructions: asm.Instructions{
			asm.Mov.Imm(asm.R0, ActionPass),
			asm.Return(),
		},
		License: "Apache-2.0",
	})
	if err != nil {
		t.Fatalf("load calibration program: %v", err)
	}

	t.Cleanup(func() { _ = prog.Close() })

	return prog
}

func ipCmdOK(t *testing.T, args ...string) {
	t.Helper()

	if out, err := ipCmd(args...); err != nil {
		t.Fatalf("ip %s: %v: %s", strings.Join(args, " "), err, out)
	}
}

func setupBenchNet(t *testing.T) (n3, n6 *net.Interface) {
	t.Helper()

	_, _ = ipCmd("link", "del", benchN3Dev)
	_, _ = ipCmd("link", "del", benchN6Dev)

	addVethPair(t, benchN3Dev, benchN3Peer)
	addVethPair(t, benchN6Dev, benchN6Peer)

	if err := writeSysctl("net.ipv4.ip_forward", "1"); err != nil {
		t.Fatalf("enable ipv4 forwarding: %v", err)
	}

	if err := writeSysctl("net.ipv6.conf.all.forwarding", "1"); err != nil {
		t.Fatalf("enable ipv6 forwarding: %v", err)
	}

	addAddr(t, benchN3Dev, addrCIDR(benchUPFN3IP))
	addNeigh(t, benchN3Dev, benchGNBIP, benchN3MAC)

	addAddr(t, benchN6Dev, addrCIDR(benchN6IP))
	addRoute(t, "203.0.113.0/24", benchN6Dev, benchN6IP)
	addNeigh(t, benchN6Dev, benchServer, benchN6MAC)

	ipCmdOK(t, "addr", "add", benchN6IPv6, "dev", benchN6Dev, "nodad")
	ipCmdOK(t, "route", "add", benchServerV6Net, "dev", benchN6Dev,
		"src", strings.TrimSuffix(benchN6IPv6, "/64"))
	ipCmdOK(t, "-6", "neigh", "add", benchServerV6, "dev", benchN6Dev,
		"lladdr", benchN6MAC, "nud", "permanent")

	return ifByName(t, benchN3Dev), ifByName(t, benchN6Dev)
}

const bpfProgTestRun = 10

type progTestRunAttr struct {
	progFD      uint32
	retval      uint32
	dataSizeIn  uint32
	dataSizeOut uint32
	dataIn      uint64
	dataOut     uint64
	repeat      uint32
	duration    uint32
	ctxSizeIn   uint32
	ctxSizeOut  uint32
	ctxIn       uint64
	ctxOut      uint64
	flags       uint32
	cpu         uint32
	batchSize   uint32
	_           uint32
}

func benchProgTestRun(fd int, packet, ctx []byte) (uint32, time.Duration, error) {
	attr := progTestRunAttr{
		progFD:     uint32(fd),
		dataSizeIn: uint32(len(packet)),
		dataIn:     uint64(uintptr(unsafe.Pointer(&packet[0]))),
		repeat:     1,
	}

	if len(ctx) > 0 {
		attr.ctxSizeIn = uint32(len(ctx))
		attr.ctxIn = uint64(uintptr(unsafe.Pointer(&ctx[0])))
	}

	_, _, errno := unix.Syscall(unix.SYS_BPF, bpfProgTestRun,
		uintptr(unsafe.Pointer(&attr)), unsafe.Sizeof(attr))

	runtime.KeepAlive(packet)
	runtime.KeepAlive(ctx)

	if errno != 0 {
		return 0, 0, errno
	}

	return attr.retval, time.Duration(attr.duration) * time.Nanosecond, nil
}

func benchSkbContext(ingressIfindex int) []byte {
	ctx, err := binary.Append(nil, binary.NativeEndian,
		skbRunContext{IngressIfindex: uint32(ingressIfindex)})
	if err != nil {
		panic(fmt.Sprintf("encode __sk_buff context: %v", err))
	}

	return ctx
}

type benchStats struct {
	action        uint32
	p10, p50, p90 time.Duration
}

func percentile(sorted []time.Duration, p int) time.Duration {
	if len(sorted) == 0 {
		return 0
	}

	i := (p * len(sorted)) / 100
	if i >= len(sorted) {
		i = len(sorted) - 1
	}

	return sorted[i]
}

func benchRun(t *testing.T, prog *ebpf.Program, packet, ctx []byte) benchStats {
	t.Helper()

	runtime.LockOSThread()

	defer runtime.UnlockOSThread()

	fd := prog.FD()
	warmup, iterations := benchSampleSize()

	var action uint32

	for range warmup {
		ret, _, err := benchProgTestRun(fd, packet, ctx)
		if err != nil {
			t.Fatalf("benchmark run: %v", err)
		}

		action = ret
	}

	samples := make([]time.Duration, 0, iterations)

	for range iterations {
		ret, d, err := benchProgTestRun(fd, packet, ctx)
		if err != nil {
			t.Fatalf("benchmark run: %v", err)
		}

		action = ret

		samples = append(samples, d)
	}

	slices.Sort(samples)

	return benchStats{
		action: action,
		p10:    percentile(samples, 10),
		p50:    percentile(samples, 50),
		p90:    percentile(samples, 90),
	}
}

type benchCase struct {
	direction string
	gen       string
	family    string
	packet    []byte
}

func (c benchCase) uplink() bool { return c.direction == "uplink" }

func (c benchCase) name(nat bool) string {
	natLabel := "no-nat"
	if nat {
		natLabel = "nat"
	}

	return fmt.Sprintf("%s/%s/%s/%s", c.direction, c.gen, natLabel, c.family)
}

const benchTEID uint32 = 0xB0000001

const (
	benchNATPort5G = 30001
	benchNATPort4G = 30002
	benchUEPort    = 40000
	benchSrvPort   = 53
)

type benchMaps struct {
	pdrsUplink      *ebpf.Map
	pdrsDownlinkIP4 *ebpf.Map
	pdrsDownlinkIP6 *ebpf.Map
	natCt           *ebpf.Map
}

func xdpBenchMaps(obj *BpfObjects) benchMaps {
	return benchMaps{obj.PdrsUplink, obj.PdrsDownlinkIp4, obj.PdrsDownlinkIp6, obj.NatCt}
}

func tcBenchMaps(obj *N3N6EntrypointTcObjects) benchMaps {
	return benchMaps{obj.PdrsUplink, obj.PdrsDownlinkIp4, obj.PdrsDownlinkIp6, obj.NatCt}
}

func putPDR(t *testing.T, m *ebpf.Map, key any, pdr PdrInfo, what string) {
	t.Helper()

	stored, err := ToN3N6EntrypointPdrInfo(pdr)
	if err != nil {
		t.Fatalf("build %s PDR: %v", what, err)
	}

	if err := m.Put(key, unsafe.Pointer(&stored)); err != nil {
		t.Fatalf("install %s PDR: %v", what, err)
	}
}

func installBenchPDRs(t *testing.T, m benchMaps) {
	t.Helper()

	putPDR(t, m.pdrsUplink, benchTEID, PdrInfo{
		OuterHeaderRemoval: 0,
		IMSI:               "001010000000001",
		Far:                FarInfo{Action: 0x02},
		Qer:                QerInfo{GateStatusUL: 0, MaxBitrateUL: 0},
		UEIPv4:             canonicalUEv4,
		UEIPv6Prefix:       canonicalUEv6Prefix,
	}, "uplink")

	s1u := func(p PdrInfo) PdrInfo {
		p.Far.OuterHeaderCreation |= 0x10

		return p
	}

	dl := func(qfi uint8) PdrInfo {
		return ipv4OuterDownlinkPDR(benchTEID, benchUPFN3IP, benchGNBIP, qfi)
	}

	putPDR(t, m.pdrsDownlinkIP4, benchUE5G, dl(5), "5G downlink")
	putPDR(t, m.pdrsDownlinkIP4, benchUE4G, s1u(dl(0)), "4G downlink")

	putPDR(t, m.pdrsDownlinkIP6, benchUEv65G.As16(), dl(5), "5G IPv6 downlink")
	putPDR(t, m.pdrsDownlinkIP6, benchUEv64G.As16(), s1u(dl(0)), "4G IPv6 downlink")
}

func seedBenchConntrack(t *testing.T, m benchMaps) {
	t.Helper()

	for _, mapping := range []struct {
		natPort uint16
		ue      [4]byte
	}{
		{benchNATPort5G, benchUE5G},
		{benchNATPort4G, benchUE4G},
	} {
		key := natFiveTuple(benchN6IP, benchServer, mapping.natPort, benchSrvPort, 17)
		entry := N3N6EntrypointNatEntry{
			Peer: natFiveTuple(mapping.ue, benchServer, benchUEPort, benchSrvPort, 17),
		}

		if err := m.natCt.Put(&key, &entry); err != nil {
			t.Fatalf("seed nat_ct for %v: %v", mapping.ue, err)
		}
	}
}

func benchCases(nat bool) []benchCase {
	innerV4 := ipv4Packet(canonicalUEv4.As4(), benchServer, 17,
		udpDatagramChecksummed(canonicalUEv4.As4(), benchServer,
			benchUEPort, benchSrvPort, []byte{1, 2, 3, 4}))

	serverV6 := netip.MustParseAddr(benchServerV6).As16()
	innerV6 := ipv6Packet(testUEv6Src, serverV6, 17,
		udpDatagram(benchUEPort, benchSrvPort, []byte{1, 2, 3, 4}))

	downlinkV4 := func(ue [4]byte, natPort uint16) []byte {
		dst := ue
		dport := uint16(benchUEPort)

		if nat {
			dst, dport = benchN6IP, natPort
		}

		return ethFrame(0x0800, ipv4Packet(benchServer, dst, 17,
			udpDatagramChecksummed(benchServer, dst, benchSrvPort, dport, []byte{1, 2, 3, 4})))
	}

	downlinkV6 := func(prefix netip.Addr) []byte {
		return ethFrame(0x86DD, ipv6Packet(serverV6, benchUEv6Dst(prefix), 17,
			udpDatagram(benchSrvPort, benchUEPort, []byte{1, 2, 3, 4})))
	}

	return []benchCase{
		{"uplink", "5G", "ipv4", uplinkGPDU(benchTEID, innerV4)},
		{"uplink", "5G", "ipv6", uplinkGPDU(benchTEID, innerV6)},
		{"uplink", "4G", "ipv4", uplinkGPDUPlain(benchTEID, innerV4)},
		{"uplink", "4G", "ipv6", uplinkGPDUPlain(benchTEID, innerV6)},
		{"downlink", "5G", "ipv4", downlinkV4(benchUE5G, benchNATPort5G)},
		{"downlink", "5G", "ipv6", downlinkV6(benchUEv65G)},
		{"downlink", "4G", "ipv4", downlinkV4(benchUE4G, benchNATPort4G)},
		{"downlink", "4G", "ipv6", downlinkV6(benchUEv64G)},
	}
}

func verdictName(build string, v uint32) string {
	if build == "tcx" {
		switch v {
		case tcActOK:
			return "TC_OK"
		case tcActShot:
			return "TC_SHOT"
		case tcActRedirect:
			return "TC_REDIR"
		default:
			return fmt.Sprintf("?%d", v)
		}
	}

	switch v {
	case ActionAborted:
		return "ABORTED"
	case ActionDrop:
		return "DROP"
	case ActionPass:
		return "PASS"
	case ActionTx:
		return "TX"
	case ActionRedirect:
		return "REDIRECT"
	default:
		return fmt.Sprintf("?%d", v)
	}
}

func forwarded(build string, v uint32) bool {
	if build == "tcx" {
		return v == tcActRedirect
	}

	return v == ActionRedirect || v == ActionTx
}

const loopbackIfindex = 1

type benchTarget struct {
	build string
	entry *ebpf.Program

	ctx []byte
}

func benchTargets(t *testing.T, build string, nat bool, n3, n6 *net.Interface) (uplink, downlink benchTarget) {
	t.Helper()

	if build == "tcx" {
		ctx := benchSkbContext(loopbackIfindex)

		up := loadTCProgramConfig(t, nat, loopbackIfindex, n6.Index)
		down := loadTCProgramConfig(t, nat, n3.Index, n6.Index)

		for _, obj := range []*N3N6EntrypointTcObjects{up, down} {
			installBenchPDRs(t, tcBenchMaps(obj))

			if nat {
				seedBenchConntrack(t, tcBenchMaps(obj))
			}
		}

		return benchTarget{build, up.UpfEntryFunc, ctx},
			benchTarget{build, down.UpfEntryFunc, ctx}
	}

	up := loadProgramConfig(t, false, nat, loopbackIfindex, n6.Index, 0, 0)
	down := loadProgramConfig(t, false, nat, n3.Index, n6.Index, 0, 0)

	for _, obj := range []*BpfObjects{up, down} {
		installBenchPDRs(t, xdpBenchMaps(obj))

		if nat {
			seedBenchConntrack(t, xdpBenchMaps(obj))
		}
	}

	return benchTarget{build, up.UpfEntryFunc, nil},
		benchTarget{build, down.UpfEntryFunc, nil}
}

func TestDatapathMatrixForwards(t *testing.T) {
	requireProgTestRun(t)

	if testAttachModeTCX() {
		t.Skip("covers both builds in one run; skipping the duplicate TCX fixture pass")
	}

	n3, n6 := setupBenchNet(t)

	var floor benchStats

	if benchmarking() {
		floor = benchRun(t, benchNopProgram(t), ethFrame(0x0800,
			ipv4Packet(benchServer, benchN6IP, 17, udpDatagram(1, 2, nil))), nil)

		t.Logf("MEASURE %-38s %s p50=%v p10=%v p90=%v",
			"floor/nop-program", verdictName("xdp", floor.action), floor.p50, floor.p10, floor.p90)

		t.Logf("%-38s %-9s %8s %8s %8s %8s", "CASE", "VERDICT", "p50", "net-p50", "p10", "p90")
	}

	for _, build := range []string{"xdp", "tcx"} {
		for _, nat := range []bool{false, true} {
			uplinkTgt, downlinkTgt := benchTargets(t, build, nat, n3, n6)

			for _, c := range benchCases(nat) {
				tgt := downlinkTgt
				if c.uplink() {
					tgt = uplinkTgt
				}

				runBenchCell(t, tgt, c, nat, floor)
			}
		}
	}
}

func runBenchCell(t *testing.T, tgt benchTarget, c benchCase, nat bool, floor benchStats) {
	t.Helper()

	stats := benchRun(t, tgt.entry, c.packet, tgt.ctx)
	name := tgt.build + "/" + c.name(nat)

	if benchmarking() {
		net := stats.p50 - floor.p50
		if net < 0 {
			net = 0
		}

		t.Logf("MEASURE %-38s %-9s %8v %8v %8v %8v",
			name, verdictName(tgt.build, stats.action), stats.p50, net, stats.p10, stats.p90)
	}

	if !forwarded(tgt.build, stats.action) {
		t.Errorf("%s got verdict %s, want a forwarding action: the frame did not traverse the datapath, so its timing is not a packet-processing cost",
			name, verdictName(tgt.build, stats.action))
	}
}
