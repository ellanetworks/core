// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: Apache-2.0

//go:build linux

package ebpf

import (
	"bytes"
	"errors"
	"fmt"
	"net/netip"
	"runtime"
	"strings"
	"testing"
	"unsafe"

	"github.com/cilium/ebpf"
	"golang.org/x/sys/unix"
)

// TC verdicts (linux/pkt_cls.h). bpf_redirect returns TC_ACT_REDIRECT, which
// is also the datapath's transmit-back verdict.
const (
	tcActOK       = 0
	tcActShot     = 2
	tcActRedirect = 7
)

// loadTCProgramConfig loads the SCHED_CLS build of the datapath through the
// kernel verifier with the same configuration knobs as loadProgramConfig.
func loadTCProgramConfig(t *testing.T, masquerade bool, n3Ifindex, n6Ifindex int) *N3N6EntrypointTcObjects { //nolint:unparam // signature mirrors loadProgramConfig
	t.Helper()

	cfg := NewBpfObjects(false, masquerade, n3Ifindex, n6Ifindex, 0, 0)

	spec, err := LoadN3N6EntrypointTc()
	if err != nil {
		t.Fatalf("load TC spec: %v", err)
	}

	if m, ok := spec.Maps["csum_scratch"]; ok {
		m.MaxEntries = uint32(runtime.NumCPU())
	}

	var objs N3N6EntrypointTcObjects
	if err := cfg.loadAndAssignFromSpec(spec, &objs, nil); err != nil {
		var ve *ebpf.VerifierError
		if errors.As(err, &ve) {
			t.Fatalf("load TC objects: verifier error: %+v", ve)
		}

		t.Fatalf("load TC objects: %v", err)
	}

	t.Cleanup(func() { _ = objs.Close() })

	stages := map[uint32]*ebpf.Program{
		upfCallUplink:      objs.UpfUplinkFunc,
		upfCallDownlink:    objs.UpfDownlinkFunc,
		upfCallGtpuControl: objs.UpfGtpuControlFunc,
	}
	for idx, prog := range stages {
		if err := objs.UpfCalls.Put(idx, prog); err != nil {
			t.Fatalf("populate TC upf_calls[%d]: %v", idx, err)
		}
	}

	return &objs
}

func putTCPdrUplink(t *testing.T, objs *N3N6EntrypointTcObjects, teid uint32, pdr PdrInfo) {
	t.Helper()

	stored := ToN3N6EntrypointPdrInfo(pdr)
	if err := objs.PdrsUplink.Put(teid, unsafe.Pointer(&stored)); err != nil {
		t.Fatalf("install TC uplink PDR: %v", err)
	}
}

func putTCPdrDownlink(t *testing.T, objs *N3N6EntrypointTcObjects, ueAddr [4]byte, pdr PdrInfo) {
	t.Helper()

	stored := ToN3N6EntrypointPdrInfo(pdr)
	if err := objs.PdrsDownlinkIp4.Put(ueAddr, unsafe.Pointer(&stored)); err != nil {
		t.Fatalf("install TC downlink PDR: %v", err)
	}
}

// skbRunContext is the UAPI __sk_buff layout accepted as ctx_in by
// BPF_PROG_TEST_RUN for SCHED_CLS; only the kernel-settable fields may be
// non-zero (net/bpf/test_run.c convert___skb_to_skb).
type skbRunContext struct {
	Len, PktType, Mark, QueueMapping, Protocol, VlanPresent uint32
	VlanTci, VlanProto, Priority, IngressIfindex, Ifindex   uint32
	TcIndex                                                 uint32
	Cb                                                      [5]uint32
	Hash, TcClassid, Data, DataEnd, NapiID, Family          uint32
	RemoteIP4, LocalIP4                                     uint32
	RemoteIP6, LocalIP6                                     [4]uint32
	RemotePort, LocalPort, DataMeta                         uint32
	FlowKeys                                                uint64
	Tstamp                                                  uint64
	WireLen, GsoSegs                                        uint32
	Sk                                                      uint64
	GsoSize                                                 uint32
	TstampType                                              uint8
	_                                                       [3]byte
	Hwtstamp                                                uint64
}

func runTC(t *testing.T, prog *ebpf.Program, packet []byte) (uint32, []byte) {
	t.Helper()

	// ingress_ifindex 1 (loopback) mirrors what XDP BPF_PROG_TEST_RUN
	// provides, so bpf_fib_lookup resolves identically in both builds.
	opts := &ebpf.RunOptions{
		Data:    packet,
		DataOut: make([]byte, len(packet)+256),
		Context: skbRunContext{IngressIfindex: 1},
	}

	action, err := prog.Run(opts)
	if err != nil {
		if errors.Is(err, ebpf.ErrNotSupported) {
			t.Skipf("BPF_PROG_TEST_RUN for SCHED_CLS not supported on this kernel: %v", err)
		}

		t.Fatalf("run TC program: %v", err)
	}

	return action, opts.DataOut
}

// bpfFTestSKBChecksumComplete makes BPF_PROG_TEST_RUN present the skb as
// CHECKSUM_COMPLETE and recompute skb->csum afterwards, returning EBADMSG when
// the program's header rewrites did not carry the running sum along. Without
// it the test skb is CHECKSUM_NONE, where arithmetic that never touches
// skb->csum is indistinguishable from arithmetic that maintains it.
const bpfFTestSKBChecksumComplete = 1 << 2

// runTCChecksumComplete is runTC with that validation enabled. A kernel
// without the flag rejects the run with EINVAL.
func runTCChecksumComplete(t *testing.T, prog *ebpf.Program, packet []byte) (uint32, []byte) {
	t.Helper()

	opts := &ebpf.RunOptions{
		Data:    packet,
		DataOut: make([]byte, len(packet)+256),
		Context: skbRunContext{IngressIfindex: 1},
		Flags:   bpfFTestSKBChecksumComplete,
	}

	action, err := prog.Run(opts)
	if err != nil {
		if errors.Is(err, ebpf.ErrNotSupported) || errors.Is(err, unix.EINVAL) {
			t.Skipf("BPF_F_TEST_SKB_CHECKSUM_COMPLETE not supported on this kernel: %v", err)
		}

		if errors.Is(err, unix.EBADMSG) {
			t.Fatalf("skb->csum stale after the program ran: %v", err)
		}

		t.Fatalf("run TC program: %v", err)
	}

	return action, opts.DataOut
}

// verdictsEquivalent maps the XDP verdict encoding onto TC's: forwarding
// verdicts (TX and REDIRECT) both surface as TC_ACT_REDIRECT, and TC folds
// aborted into drop (both TC_ACT_SHOT). The verdicts are lossy in exactly this
// way, which is why the action counter is keyed on the datapath's own decision
// — see actionCounters.
func verdictsEquivalent(xdpAction, tcAction uint32) bool {
	switch xdpAction {
	case ActionPass:
		return tcAction == tcActOK
	case ActionDrop, ActionAborted:
		return tcAction == tcActShot
	case ActionTx, ActionRedirect:
		return tcAction == tcActRedirect
	}

	return false
}

// actionCounters sums a build's per-action counters over both directions and
// all CPUs. The two builds' statistics are distinct generated types with the
// same layout, so each caller passes its own reader.
func actionCounters(t *testing.T, read func(*ebpf.Map) [UPFMaxAction]uint64, uplink, downlink *ebpf.Map) [UPFMaxAction]uint64 {
	t.Helper()

	var total [UPFMaxAction]uint64

	for _, m := range []*ebpf.Map{uplink, downlink} {
		for i, v := range read(m) {
			total[i] += v
		}
	}

	return total
}

func xdpActionCounters(t *testing.T, obj *BpfObjects) [UPFMaxAction]uint64 {
	t.Helper()

	return actionCounters(t, func(m *ebpf.Map) [UPFMaxAction]uint64 {
		var stats []N3N6EntrypointUpfStatistic
		if err := m.Lookup(uint32(0), &stats); err != nil {
			t.Fatalf("read XDP statistics: %v", err)
		}

		var sum [UPFMaxAction]uint64

		for _, s := range stats {
			for i, v := range s.ForwardedActions {
				sum[i] += v
			}
		}

		return sum
	}, obj.UplinkStatistics, obj.DownlinkStatistics)
}

func tcActionCounters(t *testing.T, objs *N3N6EntrypointTcObjects) [UPFMaxAction]uint64 {
	t.Helper()

	return actionCounters(t, func(m *ebpf.Map) [UPFMaxAction]uint64 {
		var stats []N3N6EntrypointTcUpfStatistic
		if err := m.Lookup(uint32(0), &stats); err != nil {
			t.Fatalf("read TC statistics: %v", err)
		}

		var sum [UPFMaxAction]uint64

		for _, s := range stats {
			for i, v := range s.ForwardedActions {
				sum[i] += v
			}
		}

		return sum
	}, objs.UplinkStatistics, objs.DownlinkStatistics)
}

func actionDelta(before, after [UPFMaxAction]uint64) [UPFMaxAction]uint64 {
	var d [UPFMaxAction]uint64
	for i := range after {
		d[i] = after[i] - before[i]
	}

	return d
}

var actionNames = map[int]string{
	ActionAborted:  "aborted",
	ActionDrop:     "drop",
	ActionPass:     "pass",
	ActionTx:       "tx",
	ActionRedirect: "redirect",
}

func formatActions(d [UPFMaxAction]uint64) string {
	var b []string

	for i, v := range d {
		if v == 0 {
			continue
		}

		name, ok := actionNames[i]
		if !ok {
			name = fmt.Sprintf("action%d", i)
		}

		b = append(b, fmt.Sprintf("%s=%d", name, v))
	}

	if len(b) == 0 {
		return "none"
	}

	return strings.Join(b, " ")
}

// TestTCObjectsLoad is the verifier gate for the SCHED_CLS build: every
// program in the TC object must load.
func TestTCObjectsLoad(t *testing.T) {
	requireProgTestRun(t)

	objs := loadTCProgramConfig(t, true, 0, 1)

	for name, prog := range map[string]*ebpf.Program{
		"upf_entry":        objs.UpfEntryFunc,
		"upf_uplink":       objs.UpfUplinkFunc,
		"upf_downlink":     objs.UpfDownlinkFunc,
		"upf_gtpu_control": objs.UpfGtpuControlFunc,
		"veth_xdp":         objs.VethXdpFunc,
	} {
		if prog == nil {
			t.Errorf("TC program %s did not load", name)
		}
	}
}

// TestTCMatchesXDPOutput runs the same frames through the XDP and TC builds
// of the datapath with identical map state and requires byte-identical output
// packets and equivalent verdicts. This pins the TC build to the XDP build's
// behavior across decap, encap (PSC, S1-U, IPv6 outer), echo, and error
// indication.
func TestTCMatchesXDPOutput(t *testing.T) {
	requireProgTestRun(t)

	const (
		ulTEID = 0x54435831
		dlTEID = 0x54435832
		qfi    = 5
	)

	var (
		ueAddr = [4]byte{10, 45, 0, 7}
		local  = [4]byte{192, 168, 100, 1}
		remote = [4]byte{192, 168, 100, 9}
	)

	xdpObj := loadProgram(t, 0, 1)
	tcObjs := loadTCProgramConfig(t, false, 0, 1)

	putForwardingUplinkPDRUE(t, xdpObj, ulTEID, 0, canonicalUEv4, netip.Addr{})
	putTCPdrUplink(t, tcObjs, ulTEID, PdrInfo{
		OuterHeaderRemoval: 0,
		IMSI:               "001010000000001",
		Far:                FarInfo{Action: 0x02},
		Qer:                QerInfo{GateStatusUL: 0, MaxBitrateUL: 0},
		UEIPv4:             canonicalUEv4,
	})

	pscPDR := ipv4OuterDownlinkPDR(dlTEID, local, remote, qfi)
	s1uPDR := ipv4OuterDownlinkPDR(dlTEID, local, remote, 0)
	s1uPDR.Far.OuterHeaderCreation |= 0x10 // OHC_NO_PSC
	v6PDR := ipv6OuterDownlinkPDR(dlTEID, testUPFN3v6, testGNBv6, qfi)

	inner := innerIPv4UDP([4]byte{8, 8, 8, 8}, 53)
	downlink := ethFrame(0x0800, ipv4Packet([4]byte{8, 8, 8, 8}, ueAddr, 17,
		udpDatagramChecksummed([4]byte{8, 8, 8, 8}, ueAddr, 53, 4001, []byte{1, 2, 3, 4})))

	cases := []struct {
		name  string
		pdr   *PdrInfo
		frame []byte
	}{
		{name: "uplink decap", frame: uplinkGPDU(ulTEID, inner)},
		{name: "downlink encap PSC", pdr: &pscPDR, frame: downlink},
		{name: "downlink encap S1U", pdr: &s1uPDR, frame: downlink},
		{name: "downlink encap IPv6 outer", pdr: &v6PDR, frame: downlink},
		{name: "echo request", frame: gtpControlFrame(1)},
		{name: "error indication for unknown TEID", frame: uplinkGPDU(0x0BAD7E1D, inner)},
		{name: "no session passes to stack", frame: ethFrame(0x0800, innerIPv4UDP([4]byte{8, 8, 4, 4}, 53))},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.pdr != nil {
				if err := xdpObj.PutPdrDownlink(netip.AddrFrom4(ueAddr), *tc.pdr); err != nil {
					t.Fatalf("install downlink PDR: %v", err)
				}

				putTCPdrDownlink(t, tcObjs, ueAddr, *tc.pdr)
			}

			xdpBefore := xdpActionCounters(t, xdpObj)
			tcBefore := tcActionCounters(t, tcObjs)

			xdpAction, xdpOut := runXDPOut(t, xdpObj.UpfEntryFunc, tc.frame)
			tcAction, tcOut := runTC(t, tcObjs.UpfEntryFunc, tc.frame)

			if !verdictsEquivalent(xdpAction, tcAction) {
				t.Errorf("verdicts diverge: XDP %d, TC %d", xdpAction, tcAction)
			}

			// The verdicts are allowed to differ; the recorded action is
			// not. TC cannot express an abort or a transmit-back, so a
			// counter keyed on the verdict would report this frame as a
			// drop or a redirect under TCX and leave those two series at
			// zero forever.
			xdpDelta := actionDelta(xdpBefore, xdpActionCounters(t, xdpObj))
			tcDelta := actionDelta(tcBefore, tcActionCounters(t, tcObjs))

			if xdpDelta != tcDelta {
				t.Errorf("action counters diverge: XDP [%s], TC [%s]",
					formatActions(xdpDelta), formatActions(tcDelta))
			}

			if !bytes.Equal(xdpOut, tcOut) {
				t.Errorf("output frames diverge:\nXDP (%d bytes): %x\nTC  (%d bytes): %x",
					len(xdpOut), xdpOut, len(tcOut), tcOut)
			}
		})
	}
}

// TestTCMatchesXDPOutputDestinationNAT seeds the same conntrack mapping into
// both builds and requires byte-identical destination-NAT output: address and
// port rewrites plus the incremental checksum updates, which the TC build
// performs through bpf_l4_csum_replace.
func TestTCMatchesXDPOutputDestinationNAT(t *testing.T) {
	requireProgTestRun(t)

	const (
		dlTEID  = 0x54435833
		natPort = 2000
		uePort  = 40000
		srvPort = 53
		qfi     = 5
	)

	var (
		local  = [4]byte{192, 168, 100, 1}
		remote = [4]byte{192, 168, 100, 9}
	)

	xdpObj := loadProgramConfig(t, false, true, 0, 1, 0, 0)
	tcObjs := loadTCProgramConfig(t, true, 0, 1)

	pdr := ipv4OuterDownlinkPDR(dlTEID, local, remote, qfi)
	if err := xdpObj.PutPdrDownlink(netip.AddrFrom4(ueIP), pdr); err != nil {
		t.Fatalf("install downlink PDR: %v", err)
	}

	putTCPdrDownlink(t, tcObjs, ueIP, pdr)

	// The NAT-side conntrack entry as the uplink path would have created it:
	// keyed by the reply tuple, peering back to the UE's original tuple.
	key := natFiveTuple(natPublicIP, serverIP, natPort, srvPort, 17)
	entry := N3N6EntrypointNatEntry{
		Peer: natFiveTuple(ueIP, serverIP, uePort, srvPort, 17),
	}

	if err := xdpObj.NatCt.Put(&key, &entry); err != nil {
		t.Fatalf("seed XDP nat_ct: %v", err)
	}

	if err := tcObjs.NatCt.Put(&key, &entry); err != nil {
		t.Fatalf("seed TC nat_ct: %v", err)
	}

	frame := ethFrame(0x0800, ipv4Packet(serverIP, natPublicIP, 17,
		udpDatagramChecksummed(serverIP, natPublicIP, srvPort, natPort, []byte{9, 9, 9, 9})))

	xdpAction, xdpOut := runXDPOut(t, xdpObj.UpfEntryFunc, frame)
	tcAction, tcOut := runTC(t, tcObjs.UpfEntryFunc, frame)

	if !verdictsEquivalent(xdpAction, tcAction) {
		t.Errorf("verdicts diverge: XDP %d, TC %d", xdpAction, tcAction)
	}

	if !bytes.Equal(xdpOut, tcOut) {
		t.Errorf("NAT output frames diverge:\nXDP (%d bytes): %x\nTC  (%d bytes): %x",
			len(xdpOut), xdpOut, len(tcOut), tcOut)
	}

	if xdpAction == ActionDrop || xdpAction == ActionAborted {
		t.Fatalf("translated downlink packet dropped (XDP action %d)", xdpAction)
	}

	// The rewrites must be visible inside the encapsulated frame: inner
	// destination is the UE address and port again.
	innerIP := xdpOut[ethHdrLen+gtpV4EncapLen:]
	if !bytes.Equal(innerIP[16:20], ueIP[:]) {
		t.Errorf("inner daddr = %v, want %v", innerIP[16:20], ueIP)
	}

	if got := uint16(innerIP[22])<<8 | uint16(innerIP[23]); got != uePort {
		t.Errorf("inner dport = %d, want %d", got, uePort)
	}
}

// TestTCInBandVLANPassedToStack checks the QinQ guard: at TCX ingress the
// kernel keeps at most one tag out-of-band, so a frame whose bytes still
// carry a VLAN ethertype is beyond single-tag configs and belongs to the
// stack — matching the XDP build, whose parser finds no L3 branch for the
// inner tag and passes the frame.
func TestTCInBandVLANPassedToStack(t *testing.T) {
	requireProgTestRun(t)

	tcObjs := loadTCProgramConfig(t, false, 0, 1)

	for name, etherType := range map[string]uint16{"8021q": 0x8100, "8021ad": 0x88a8} {
		t.Run(name, func(t *testing.T) {
			tag := []byte{0x00, 0x64, 0x08, 0x00} // VID 100, inner IPv4
			frame := ethFrame(etherType, append(tag, innerIPv4UDP([4]byte{8, 8, 8, 8}, 53)...))

			action, out := runTC(t, tcObjs.UpfEntryFunc, frame)

			if action != tcActOK {
				t.Fatalf("in-band tagged frame got TC action %d, want TC_ACT_OK (%d)", action, tcActOK)
			}

			if !bytes.Equal(out, frame) {
				t.Errorf("frame modified on the pass-to-stack path")
			}
		})
	}
}

// TestTCSourceNATMaintainsChecksumComplete pins the uplink masquerade path to
// the checksum contract the TC hook actually runs under. Rewriting the source
// address moves the L4 checksum without a compensating change in the bytes
// skb->csum covers, so the update has to reach the kernel through
// bpf_l4_csum_replace for the running sum to stay valid.
func TestTCSourceNATMaintainsChecksumComplete(t *testing.T) {
	requireProgTestRun(t)

	const ulTEID = 0x54435834

	tcObjs := loadTCProgramConfig(t, true, 0, 1)

	putTCPdrUplink(t, tcObjs, ulTEID, PdrInfo{
		OuterHeaderRemoval: 0,
		IMSI:               "001010000000001",
		Far:                FarInfo{Action: 0x02},
		Qer:                QerInfo{GateStatusUL: 0, MaxBitrateUL: 0},
		UEIPv4:             canonicalUEv4,
	})

	for _, tc := range []struct {
		name  string
		proto uint8
		l4    []byte
	}{
		{name: "udp", proto: 17, l4: udpDatagramChecksummed(ueIP, serverIP, 40000, 53, []byte{1, 2, 3, 4})},
		{name: "tcp", proto: 6, l4: tcpSegmentChecksummed(ueIP, serverIP, 40000, 80, []byte{1, 2, 3, 4})},
	} {
		t.Run(tc.name, func(t *testing.T) {
			frame := uplinkGPDU(ulTEID, ipv4Packet(ueIP, serverIP, tc.proto, tc.l4))
			runTCChecksumComplete(t, tcObjs.UpfEntryFunc, frame)
		})
	}
}
