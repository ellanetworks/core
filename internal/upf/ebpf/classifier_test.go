// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: Apache-2.0

//go:build linux

package ebpf

import (
	"net/netip"
	"testing"

	"github.com/ellanetworks/core/internal/models"
)

const (
	defaultTEID = uint32(0x0D0D0001)
	bearerTEID  = uint32(0x0D0D0002)
	bearerPdrID = 32
	bearerQerID = 16
	bearerUrrID = 2
	defaultQFI  = 5
	bearerQFI   = 1
	remotePort  = 30000
	localPort   = 40000
	protoUDP    = 17
)

var (
	classifierUE     = [4]byte{10, 45, 0, 7}
	classifierRemote = [4]byte{10, 45, 0, 8}
	classifierLocal  = [4]byte{192, 168, 100, 1}
	classifierENB    = [4]byte{192, 168, 100, 9}
)

func voiceRule(direction uint8, tunnel uint32) ClassifierRule {
	return ClassifierRule{
		Direction:   direction,
		Tunnel:      tunnel,
		Family:      ClassifierFamilyIPv4,
		Remote:      netip.PrefixFrom(netip.AddrFrom4(classifierRemote), 32),
		Protocol:    protoUDP,
		RemotePorts: PortRange{LowerBound: remotePort, UpperBound: remotePort},
		LocalPorts:  PortRange{LowerBound: localPort, UpperBound: localPort},
	}
}

func voiceTarget() ClassifierTarget {
	bearer := ipv4OuterDownlinkPDR(bearerTEID, classifierLocal, classifierENB, bearerQFI)

	return ClassifierTarget{PdrID: bearerPdrID, QerID: bearerQerID, UrrID: bearerUrrID, Far: bearer.Far, Qer: bearer.Qer}
}

func putClassifiedDownlink(t *testing.T, obj *BpfObjects, flags uint8, defaultQer QerInfo) {
	t.Helper()

	pdr := ipv4OuterDownlinkPDR(defaultTEID, classifierLocal, classifierENB, defaultQFI)
	pdr.Flags = flags
	pdr.Qer = defaultQer
	pdr.Qer.Qfi = defaultQFI

	if err := obj.PutPdrDownlink(netip.AddrFrom4(classifierUE), pdr); err != nil {
		t.Fatalf("install the default downlink PDR: %v", err)
	}

	c := Classifier{Rules: []ClassifierRule{voiceRule(ClassifierDownlink, 0)}, Targets: []ClassifierTarget{voiceTarget()}}
	if err := obj.PutClassifier(0, c); err != nil {
		t.Fatalf("install the classifier: %v", err)
	}
}

func downlinkUDP(sport, dport uint16) []byte {
	return ethFrame(0x0800, ipv4Packet(classifierRemote, classifierUE, protoUDP, udpDatagram(sport, dport, []byte{1, 2, 3, 4})))
}

func TestClassifierSteersDownlinkToTheBearer(t *testing.T) {
	requireProgTestRun(t)

	tests := []struct {
		name     string
		flags    uint8
		sport    uint16
		dport    uint16
		wantTEID uint32
		wantQFI  uint8
	}{
		{"matching packet on the bearer", PdrFlagSDF, remotePort, localPort, bearerTEID, bearerQFI},
		{"other local port on the default bearer", PdrFlagSDF, remotePort, localPort + 1, defaultTEID, defaultQFI},
		{"other remote port on the default bearer", PdrFlagSDF, remotePort + 1, localPort, defaultTEID, defaultQFI},
		{"session without SDF PDRs ignores the classifier", 0, remotePort, localPort, defaultTEID, defaultQFI},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			obj := loadProgram(t, 1, 0)
			putClassifiedDownlink(t, obj, tc.flags, QerInfo{})

			action, out := runXDPOut(t, obj.UpfEntryFunc, downlinkUDP(tc.sport, tc.dport))
			if action == ActionDrop || action == ActionAborted {
				t.Fatalf("downlink packet got XDP action %d, want a forwarding action", action)
			}

			f := parseGTPv4Frame(t, out)

			if f.teid != tc.wantTEID {
				t.Errorf("TEID = %#x, want %#x", f.teid, tc.wantTEID)
			}

			if f.qfi != tc.wantQFI {
				t.Errorf("QFI = %d, want %d", f.qfi, tc.wantQFI)
			}
		})
	}
}

func TestClassifierBearerTrafficSkipsTheSessionQER(t *testing.T) {
	requireProgTestRun(t)

	obj := loadProgram(t, 1, 0)
	putClassifiedDownlink(t, obj, PdrFlagSDF, QerInfo{GateStatusDL: 1})

	if action, _ := runXDPOut(t, obj.UpfEntryFunc, downlinkUDP(remotePort, localPort)); action == ActionDrop || action == ActionAborted {
		t.Errorf("bearer packet got XDP action %d; the session QER's closed gate must not apply to it", action)
	}

	if action, _ := runXDPOut(t, obj.UpfEntryFunc, downlinkUDP(remotePort, localPort+1)); action != ActionDrop {
		t.Errorf("default-bearer packet got XDP action %d, want ActionDrop by the closed session gate", action)
	}
}

func TestUplinkBearerBindingVerification(t *testing.T) {
	requireProgTestRun(t)

	const pdrID = 16

	obj := loadN3N6Program(t)

	pdr := PdrInfo{
		PdrID:        pdrID,
		IMSI:         "001010000000001",
		Flags:        PdrFlagSDF,
		Far:          FarInfo{Action: 0x02},
		UEIPv4:       canonicalUEv4,
		UEIPv6Prefix: canonicalUEv6Prefix,
	}
	if err := obj.PutPdrUplink(bearerTEID, pdr); err != nil {
		t.Fatalf("install the bearer uplink PDR: %v", err)
	}

	uplink := func(sport, dport uint16) []byte {
		return uplinkGPDU(bearerTEID, ipv4Packet(canonicalUEv4.As4(), classifierRemote, protoUDP, udpDatagram(sport, dport, nil)))
	}

	if action, _ := runXDPOut(t, obj.UpfEntryFunc, uplink(localPort, remotePort)); action != ActionDrop {
		t.Errorf("uplink without the session's SDF filters got XDP action %d, want ActionDrop", action)
	}

	c := Classifier{Rules: []ClassifierRule{voiceRule(ClassifierUplink, bearerTEID)}, Targets: []ClassifierTarget{{PdrID: pdrID}}}
	if err := obj.PutClassifier(0, c); err != nil {
		t.Fatalf("install the classifier: %v", err)
	}

	if action, _ := runXDPOut(t, obj.UpfEntryFunc, uplink(localPort, remotePort)); action == ActionDrop || action == ActionAborted {
		t.Errorf("uplink matching the bearer's filter got XDP action %d, want forwarded", action)
	}

	before := DropCount(obj, Uplink, "bearer_binding")

	if action, _ := runXDPOut(t, obj.UpfEntryFunc, uplink(localPort+1, remotePort)); action != ActionDrop {
		t.Errorf("uplink outside the bearer's filters got XDP action %d, want ActionDrop", action)
	}

	if after := DropCount(obj, Uplink, "bearer_binding"); after != before+1 {
		t.Errorf("bearer_binding drops = %d -> %d, want one more", before, after)
	}

	c.Rules[0].Tunnel = bearerTEID + 1
	if err := obj.PutClassifier(0, c); err != nil {
		t.Fatalf("reinstall the classifier: %v", err)
	}

	if action, _ := runXDPOut(t, obj.UpfEntryFunc, uplink(localPort, remotePort)); action != ActionDrop {
		t.Errorf("uplink matching another PDR's filter got XDP action %d, want ActionDrop", action)
	}
}

func TestLocalSwitchForcedBetweenFlaggedSessions(t *testing.T) {
	requireProgTestRun(t)

	tests := []struct {
		name       string
		ulFlags    uint8
		dlFlags    uint8
		classified bool
		wantSwitch bool
		wantTEID   uint32
	}{
		{"both sessions flagged", PdrFlagLocalSwitch, PdrFlagLocalSwitch, false, true, defaultTEID},
		{"only the sender flagged", PdrFlagLocalSwitch, 0, false, false, 0},
		{"only the receiver flagged", 0, PdrFlagLocalSwitch, false, false, 0},
		{"receiver's bearer selected", PdrFlagLocalSwitch, PdrFlagLocalSwitch | PdrFlagSDF, true, true, bearerTEID},
	}

	const senderTEID = uint32(0x0D0D0010)

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			obj := loadN3N6Program(t)

			ul := PdrInfo{
				IMSI:         "001010000000001",
				Flags:        tc.ulFlags,
				Far:          FarInfo{Action: 0x02},
				UEIPv4:       netip.AddrFrom4(classifierRemote),
				UEIPv6Prefix: canonicalUEv6Prefix,
			}
			if err := obj.PutPdrUplink(senderTEID, ul); err != nil {
				t.Fatalf("install the sender's uplink PDR: %v", err)
			}

			dl := ipv4OuterDownlinkPDR(defaultTEID, classifierLocal, classifierENB, defaultQFI)
			dl.Flags = tc.dlFlags
			dl.SEID = 9

			if err := obj.PutPdrDownlink(netip.AddrFrom4(classifierUE), dl); err != nil {
				t.Fatalf("install the receiver's downlink PDR: %v", err)
			}

			if tc.classified {
				c := Classifier{Rules: []ClassifierRule{voiceRule(ClassifierDownlink, 0)}, Targets: []ClassifierTarget{voiceTarget()}}
				if err := obj.PutClassifier(9, c); err != nil {
					t.Fatalf("install the classifier: %v", err)
				}
			}

			inner := ipv4Packet(classifierRemote, classifierUE, protoUDP, udpDatagram(remotePort, localPort, nil))

			action, out := runXDPOut(t, obj.UpfEntryFunc, uplinkGPDU(senderTEID, inner))

			switched := action != ActionDrop && action != ActionAborted && len(out) > ethHdrLen+len(inner)
			if switched != tc.wantSwitch {
				t.Fatalf("switched = %v (XDP action %d), want %v", switched, action, tc.wantSwitch)
			}

			if !tc.wantSwitch {
				return
			}

			if f := parseGTPv4Frame(t, out); f.teid != tc.wantTEID {
				t.Errorf("TEID = %#x, want %#x", f.teid, tc.wantTEID)
			}
		})
	}
}

func TestQERAveragingWindowAdmitsABurst(t *testing.T) {
	requireProgTestRun(t)

	const packets = 5

	tests := []struct {
		name     string
		windowMs uint32
		want     int
	}{
		{"default window admits one packet", 0, 1},
		{"2000 ms window admits the burst", 2000, packets},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			obj := loadProgram(t, 1, 0)

			pdr := ipv4OuterDownlinkPDR(defaultTEID, classifierLocal, classifierENB, defaultQFI)
			pdr.QerID = 1
			pdr.Qer.MaxBitrateDL = 64000
			pdr.Qer.AveragingWindowMs = tc.windowMs

			if err := obj.PutPdrDownlink(netip.AddrFrom4(classifierUE), pdr); err != nil {
				t.Fatalf("install the downlink PDR: %v", err)
			}

			admitted := 0

			for range packets {
				frame := ethFrame(0x0800, ipv4Packet(classifierRemote, classifierUE, protoUDP, udpDatagram(remotePort, localPort, make([]byte, 160))))
				if action, _ := runXDPOut(t, obj.UpfEntryFunc, frame); action != ActionDrop {
					admitted++
				}
			}

			if admitted != tc.want {
				t.Errorf("admitted %d of %d back-to-back packets, want %d", admitted, packets, tc.want)
			}
		})
	}
}

func TestClassifierLimitsMatchTheSessionModel(t *testing.T) {
	if MaxClassifierRules != models.MaxSessionSDFRules || MaxClassifierTargets != models.MaxSessionSDFPDRs {
		t.Fatalf("classifier holds %d rules in %d targets, the session model allows %d in %d",
			MaxClassifierRules, MaxClassifierTargets, models.MaxSessionSDFRules, models.MaxSessionSDFPDRs)
	}
}

func putBoundUplinkBearer(t *testing.T, obj *BpfObjects, pdrID uint16, qer QerInfo) {
	t.Helper()

	pdr := PdrInfo{
		PdrID:        uint32(pdrID),
		QerID:        uint32(pdrID),
		IMSI:         "001010000000001",
		Flags:        PdrFlagSDF,
		Far:          FarInfo{Action: 0x02},
		Qer:          qer,
		UEIPv4:       canonicalUEv4,
		UEIPv6Prefix: canonicalUEv6Prefix,
	}
	if err := obj.PutPdrUplink(bearerTEID, pdr); err != nil {
		t.Fatalf("install the bearer uplink PDR: %v", err)
	}

	if err := obj.PutClassifier(0, Classifier{
		Rules:   []ClassifierRule{voiceRule(ClassifierUplink, bearerTEID)},
		Targets: []ClassifierTarget{{PdrID: uint32(pdrID), QerID: uint32(pdrID), Qer: qer}},
	}); err != nil {
		t.Fatalf("install the classifier: %v", err)
	}
}

func TestUplinkRuleGateClosesOnlyItsOwnFlow(t *testing.T) {
	requireProgTestRun(t)

	obj := loadN3N6Program(t)
	putBoundUplinkBearer(t, obj, 16, QerInfo{})

	rtcp := voiceRule(ClassifierUplink, bearerTEID)
	rtcp.LocalPorts = PortRange{LowerBound: localPort + 1, UpperBound: localPort + 1}
	rtcp.RemotePorts = PortRange{LowerBound: remotePort + 1, UpperBound: remotePort + 1}
	rtcp.Target = 1

	if err := obj.PutClassifier(0, Classifier{
		Rules: []ClassifierRule{voiceRule(ClassifierUplink, bearerTEID), rtcp},
		Targets: []ClassifierTarget{
			{PdrID: 16, QerID: 16, Qer: QerInfo{GateStatusUL: 1}},
			{PdrID: 17, QerID: 17},
		},
	}); err != nil {
		t.Fatalf("install the classifier: %v", err)
	}

	uplink := func(sport, dport uint16) []byte {
		return uplinkGPDU(bearerTEID, ipv4Packet(canonicalUEv4.As4(), classifierRemote, protoUDP, udpDatagram(sport, dport, make([]byte, 160))))
	}

	before := DropCount(obj, Uplink, "qer_gate_closed")

	if action, _ := runXDPOut(t, obj.UpfEntryFunc, uplink(localPort, remotePort)); action != ActionDrop {
		t.Fatalf("RTP behind a closed uplink gate got XDP action %d, want ActionDrop", action)
	}

	if after := DropCount(obj, Uplink, "qer_gate_closed"); after != before+1 {
		t.Errorf("qer_gate_closed drops = %d -> %d, want one more", before, after)
	}

	if action, _ := runXDPOut(t, obj.UpfEntryFunc, uplink(localPort+1, remotePort+1)); action == ActionDrop || action == ActionAborted {
		t.Errorf("RTCP of the same bearer got XDP action %d, want it forwarded past the RTP rule's closed gate", action)
	}
}

func TestUplinkOutsideTheBearerDoesNotConsumeItsMBR(t *testing.T) {
	requireProgTestRun(t)

	obj := loadN3N6Program(t)
	putBoundUplinkBearer(t, obj, 16, QerInfo{MaxBitrateUL: 8000})

	uplink := func(sport uint16) []byte {
		return uplinkGPDU(bearerTEID, ipv4Packet(canonicalUEv4.As4(), classifierRemote, protoUDP, udpDatagram(sport, remotePort, make([]byte, 160))))
	}

	if action, _ := runXDPOut(t, obj.UpfEntryFunc, uplink(localPort+1)); action != ActionDrop {
		t.Fatalf("uplink outside the bearer's filters got XDP action %d, want ActionDrop", action)
	}

	if action, _ := runXDPOut(t, obj.UpfEntryFunc, uplink(localPort)); action == ActionDrop || action == ActionAborted {
		t.Errorf("the bearer's first packet got XDP action %d: a packet the bearer never carried consumed its MBR", action)
	}
}

func TestUplinkFragmentWithoutPortsIsUnfilterable(t *testing.T) {
	requireProgTestRun(t)

	obj := loadN3N6Program(t)
	putBoundUplinkBearer(t, obj, 16, QerInfo{})

	before := DropCount(obj, Uplink, "fragment_unfilterable")

	later := innerIPv4FragmentID(canonicalUEv4.As4(), classifierRemote, 0x4242, 2, false, 0, 0)
	if action, _ := runXDPOut(t, obj.UpfEntryFunc, uplinkGPDU(bearerTEID, later)); action != ActionDrop {
		t.Fatalf("a later fragment with no recorded ports got XDP action %d on a port-scoped bearer, want ActionDrop", action)
	}

	if after := DropCount(obj, Uplink, "fragment_unfilterable"); after != before+1 {
		t.Errorf("fragment_unfilterable drops = %d -> %d, want one more", before, after)
	}
}

func uplinkGPDUWithQFI(teid uint32, qfi uint8, inner []byte) []byte {
	gtp := gtpHeader(teid, inner)
	gtp[14] = qfi

	return gtpV4Outer(gtp)
}

func TestUplinkQoSFlowIsSelectedByQFIAndFilter(t *testing.T) {
	requireProgTestRun(t)

	obj := loadN3N6Program(t)

	session := PdrInfo{
		PdrID:        1,
		IMSI:         "001010000000001",
		Flags:        PdrFlagFallback,
		QFI:          1,
		Far:          FarInfo{Action: 0x02},
		UEIPv4:       canonicalUEv4,
		UEIPv6Prefix: canonicalUEv6Prefix,
	}
	if err := obj.PutPdrUplink(bearerTEID, session); err != nil {
		t.Fatalf("install the session uplink PDR: %v", err)
	}

	voice := voiceRule(ClassifierUplink, bearerTEID)
	voice.QFI = 5

	if err := obj.PutClassifier(0, Classifier{
		Rules:   []ClassifierRule{voice},
		Targets: []ClassifierTarget{{PdrID: 300, QerID: 300, Qer: QerInfo{GateStatusUL: 1}}},
	}); err != nil {
		t.Fatalf("install the classifier: %v", err)
	}

	uplink := func(qfi uint8, sport uint16) []byte {
		return uplinkGPDUWithQFI(bearerTEID, qfi, ipv4Packet(canonicalUEv4.As4(), classifierRemote, protoUDP, udpDatagram(sport, remotePort, make([]byte, 64))))
	}

	tests := []struct {
		name   string
		qfi    uint8
		sport  uint16
		reason string
	}{
		{"voice QFI matching the voice filter takes the flow's QER", 5, localPort, "qer_gate_closed"},
		{"voice QFI outside the flow's filters", 5, localPort + 1, "bearer_binding"},
		{"unknown QFI (TS 23.501 §5.7.1.7)", 7, localPort + 1, "bearer_binding"},
		{"default QFI matching the voice filter stays on the default flow", 1, localPort, ""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			before := map[string]uint64{}
			for _, r := range []string{"qer_gate_closed", "bearer_binding"} {
				before[r] = DropCount(obj, Uplink, r)
			}

			action, _ := runXDPOut(t, obj.UpfEntryFunc, uplink(tc.qfi, tc.sport))

			if tc.reason == "" {
				if action == ActionDrop || action == ActionAborted {
					t.Fatalf("got XDP action %d, want forwarded", action)
				}

				return
			}

			if action != ActionDrop || DropCount(obj, Uplink, tc.reason) != before[tc.reason]+1 {
				t.Fatalf("got XDP action %d, want a %s drop", action, tc.reason)
			}
		})
	}
}

func TestUplinkQFIIsVerifiedWithoutQoSFlows(t *testing.T) {
	requireProgTestRun(t)

	obj := loadN3N6Program(t)

	session := PdrInfo{
		PdrID:        1,
		IMSI:         "001010000000001",
		QFI:          1,
		Far:          FarInfo{Action: 0x02},
		UEIPv4:       canonicalUEv4,
		UEIPv6Prefix: canonicalUEv6Prefix,
	}
	if err := obj.PutPdrUplink(bearerTEID, session); err != nil {
		t.Fatalf("install the session uplink PDR: %v", err)
	}

	uplink := func(qfi uint8) []byte {
		return uplinkGPDUWithQFI(bearerTEID, qfi, ipv4Packet(canonicalUEv4.As4(), classifierRemote, protoUDP, udpDatagram(localPort, remotePort, make([]byte, 64))))
	}

	if action, _ := runXDPOut(t, obj.UpfEntryFunc, uplink(1)); action == ActionDrop || action == ActionAborted {
		t.Fatalf("default QFI got XDP action %d, want forwarded", action)
	}

	before := DropCount(obj, Uplink, "bearer_binding")

	if action, _ := runXDPOut(t, obj.UpfEntryFunc, uplink(5)); action != ActionDrop || DropCount(obj, Uplink, "bearer_binding") != before+1 {
		t.Fatalf("a QFI the session lacks got XDP action %d, want a bearer_binding drop (TS 23.501 §5.7.1.7)", action)
	}
}
