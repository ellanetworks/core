// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: Apache-2.0

//go:build linux

package ebpf

import (
	"net/netip"
	"testing"
	"time"
)

func trafficClassForwarded(t *testing.T, obj *BpfObjects, classID uint16, dir Direction) TrafficClassCounters {
	t.Helper()

	c, err := obj.ReadTrafficClassForwarded(classID, dir)
	if err != nil {
		t.Fatalf("read traffic class %d %s: %v", classID, dir, err)
	}

	return c
}

func trafficClassDrops(t *testing.T, obj *BpfObjects) map[TrafficClassDrop]uint64 {
	t.Helper()

	drops, err := obj.ReadTrafficClassDrops()
	if err != nil {
		t.Fatalf("read traffic class drops: %v", err)
	}

	return drops
}

func classifiedDownlinkPDR(classID uint16) PdrInfo {
	pdr := ipv4OuterDownlinkPDR(defaultTEID, classifierLocal, classifierENB, defaultQFI)
	pdr.Qer.ClassID = classID

	return pdr
}

func TestTrafficClassCountsDownlinkForwards(t *testing.T) {
	requireProgTestRun(t)

	const classID = 7

	obj := loadProgram(t, 1, 0)
	if err := obj.PutPdrDownlink(netip.AddrFrom4(classifierUE), classifiedDownlinkPDR(classID)); err != nil {
		t.Fatalf("install downlink PDR: %v", err)
	}

	frame := downlinkUDP(remotePort, localPort)
	l3 := uint64(len(frame) - ethHdrLen)

	for range 2 {
		if action := runXDP(t, obj.UpfEntryFunc, frame); action == ActionDrop || action == ActionAborted {
			t.Fatalf("downlink packet got XDP action %d, want a forwarding action", action)
		}
	}

	got := trafficClassForwarded(t, obj, classID, Downlink)
	if want := (TrafficClassCounters{Packets: 2, Bytes: 2 * l3}); got != want {
		t.Errorf("downlink counters = %+v, want %+v", got, want)
	}

	if got := trafficClassForwarded(t, obj, classID, Uplink); got != (TrafficClassCounters{}) {
		t.Errorf("uplink counters = %+v, want zero", got)
	}

	if drops := trafficClassDrops(t, obj); len(drops) != 0 {
		t.Errorf("drops = %v, want none", drops)
	}
}

func TestTrafficClassFollowsTheSelectedQoSFlow(t *testing.T) {
	requireProgTestRun(t)

	const (
		defaultClass = 3
		bearerClass  = 4
	)

	obj := loadProgram(t, 1, 0)

	pdr := classifiedDownlinkPDR(defaultClass)
	pdr.Flags = PdrFlagSDF

	if err := obj.PutPdrDownlink(netip.AddrFrom4(classifierUE), pdr); err != nil {
		t.Fatalf("install downlink PDR: %v", err)
	}

	target := voiceTarget()
	target.Qer.ClassID = bearerClass

	c := Classifier{Rules: []ClassifierRule{voiceRule(ClassifierDownlink, 0)}, Targets: []ClassifierTarget{target}}
	if err := obj.PutClassifier(0, c); err != nil {
		t.Fatalf("install the classifier: %v", err)
	}

	runXDP(t, obj.UpfEntryFunc, downlinkUDP(remotePort, localPort))
	runXDP(t, obj.UpfEntryFunc, downlinkUDP(remotePort, localPort+1))
	runXDP(t, obj.UpfEntryFunc, downlinkUDP(remotePort, localPort+1))

	if got := trafficClassForwarded(t, obj, bearerClass, Downlink).Packets; got != 1 {
		t.Errorf("bearer class packets = %d, want 1", got)
	}

	if got := trafficClassForwarded(t, obj, defaultClass, Downlink).Packets; got != 2 {
		t.Errorf("default class packets = %d, want 2", got)
	}
}

func TestTrafficClassCountsDropsByReason(t *testing.T) {
	requireProgTestRun(t)

	const (
		downlinkClass = 9
		uplinkClass   = 11
		teid          = 0x54434c31
	)

	obj := loadProgram(t, 1, 0)

	dl := classifiedDownlinkPDR(downlinkClass)
	dl.Qer.GateStatusDL = 1

	if err := obj.PutPdrDownlink(netip.AddrFrom4(classifierUE), dl); err != nil {
		t.Fatalf("install downlink PDR: %v", err)
	}

	ul := PdrInfo{
		IMSI:         "001010000000001",
		Far:          FarInfo{Action: 0x02},
		Qer:          QerInfo{GateStatusUL: 1, ClassID: uplinkClass},
		UEIPv4:       canonicalUEv4,
		UEIPv6Prefix: canonicalUEv6Prefix,
	}
	if err := obj.PutPdrUplink(teid, ul); err != nil {
		t.Fatalf("install uplink PDR: %v", err)
	}

	for range 3 {
		runXDP(t, obj.UpfEntryFunc, downlinkUDP(remotePort, localPort))
	}

	runXDP(t, obj.UpfEntryFunc, uplinkGPDU(teid, innerIPv4UDP([4]byte{8, 8, 8, 8}, 53)))

	drops := trafficClassDrops(t, obj)

	want := map[TrafficClassDrop]uint64{
		{ClassID: downlinkClass, Direction: Downlink, Reason: "qer_gate_closed"}: 3,
		{ClassID: uplinkClass, Direction: Uplink, Reason: "qer_gate_closed"}:     1,
	}

	if len(drops) != len(want) {
		t.Fatalf("drops = %v, want %v", drops, want)
	}

	for k, v := range want {
		if drops[k] != v {
			t.Errorf("drops[%+v] = %d, want %d", k, drops[k], v)
		}
	}
}

func TestTrafficClassDoesNotOutliveItsPacket(t *testing.T) {
	requireProgTestRun(t)

	const classID = 13

	obj := loadProgram(t, 1, 0)

	dl := classifiedDownlinkPDR(classID)
	dl.Qer.GateStatusDL = 1

	if err := obj.PutPdrDownlink(netip.AddrFrom4(classifierUE), dl); err != nil {
		t.Fatalf("install downlink PDR: %v", err)
	}

	runXDP(t, obj.UpfEntryFunc, downlinkUDP(remotePort, localPort))
	runXDP(t, obj.UpfEntryFunc, uplinkGPDU(0x54434c32, innerIPv4UDP([4]byte{8, 8, 8, 8}, 53)))

	drops := trafficClassDrops(t, obj)
	if len(drops) != 1 || drops[TrafficClassDrop{ClassID: classID, Direction: Downlink, Reason: "qer_gate_closed"}] != 1 {
		t.Errorf("drops = %v, want only the one gated downlink packet", drops)
	}
}

func TestTrafficClassIgnoresUnclassifiedSessions(t *testing.T) {
	requireProgTestRun(t)

	obj := loadProgram(t, 1, 0)

	dl := classifiedDownlinkPDR(0)
	dl.Qer.GateStatusDL = 1

	if err := obj.PutPdrDownlink(netip.AddrFrom4(classifierUE), dl); err != nil {
		t.Fatalf("install downlink PDR: %v", err)
	}

	runXDP(t, obj.UpfEntryFunc, downlinkUDP(remotePort, localPort))

	if drops := trafficClassDrops(t, obj); len(drops) != 0 {
		t.Errorf("drops = %v, want none for a session without a traffic class", drops)
	}
}

func TestTrafficClassCountsBothLegsOfALocalSwitch(t *testing.T) {
	requireProgTestRun(t)

	const (
		senderTEID  = uint32(0x0D0D0011)
		senderClass = 21
		recvClass   = 22
		bearerClass = 23
	)

	tests := []struct {
		name       string
		classified bool
		wantClass  uint16
	}{
		{"default flow", false, recvClass},
		{"receiver's bearer selected", true, bearerClass},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			obj := loadN3N6Program(t)

			ul := PdrInfo{
				IMSI:         "001010000000001",
				Flags:        PdrFlagLocalSwitch,
				Far:          FarInfo{Action: 0x02},
				Qer:          QerInfo{ClassID: senderClass},
				UEIPv4:       netip.AddrFrom4(classifierRemote),
				UEIPv6Prefix: canonicalUEv6Prefix,
			}
			if err := obj.PutPdrUplink(senderTEID, ul); err != nil {
				t.Fatalf("install the sender's uplink PDR: %v", err)
			}

			dl := classifiedDownlinkPDR(recvClass)
			dl.Flags = PdrFlagLocalSwitch
			dl.SEID = 9

			if tc.classified {
				dl.Flags |= PdrFlagSDF

				target := voiceTarget()
				target.Qer.ClassID = bearerClass

				c := Classifier{Rules: []ClassifierRule{voiceRule(ClassifierDownlink, 0)}, Targets: []ClassifierTarget{target}}
				if err := obj.PutClassifier(9, c); err != nil {
					t.Fatalf("install the classifier: %v", err)
				}
			}

			if err := obj.PutPdrDownlink(netip.AddrFrom4(classifierUE), dl); err != nil {
				t.Fatalf("install the receiver's downlink PDR: %v", err)
			}

			inner := ipv4Packet(classifierRemote, classifierUE, protoUDP, udpDatagram(remotePort, localPort, nil))

			if action, _ := runXDPOut(t, obj.UpfEntryFunc, uplinkGPDU(senderTEID, inner)); action == ActionDrop || action == ActionAborted {
				t.Fatalf("local switch got XDP action %d, want a forwarding action", action)
			}

			want := TrafficClassCounters{Packets: 1, Bytes: uint64(len(inner))}

			if got := trafficClassForwarded(t, obj, senderClass, Uplink); got != want {
				t.Errorf("sender uplink = %+v, want %+v", got, want)
			}

			if got := trafficClassForwarded(t, obj, tc.wantClass, Downlink); got != want {
				t.Errorf("receiver downlink = %+v, want %+v", got, want)
			}
		})
	}
}

func TestTrafficClassCountsUplinkForwards(t *testing.T) {
	requireProgTestRun(t)

	const (
		teid    = 0x54434c33
		classID = 31
	)

	f := setupT2(t, false)

	pdr := PdrInfo{
		IMSI:   "001010000000001",
		Far:    FarInfo{Action: 0x02},
		Qer:    QerInfo{ClassID: classID},
		UEIPv4: netip.AddrFrom4(ueIP),
	}
	if err := f.obj.PutPdrUplink(teid, pdr); err != nil {
		t.Fatalf("install uplink PDR: %v", err)
	}

	capFD := f.captureN6(t)

	inner := ipv4Packet(ueIP, serverIP, 6, tcpSegmentChecksummed(ueIP, serverIP, 1400, 80, bytesOf(40)))
	f.injectUplink(t, uplinkGPDU(teid, inner))

	if captureMatching(capFD, time.Second, func(fr []byte) bool {
		return isInnerIPv4(fr, 6, serverIP)
	}) == nil {
		t.Fatal("uplink did not egress on N6")
	}

	want := TrafficClassCounters{Packets: 1, Bytes: uint64(len(inner))}
	if got := trafficClassForwarded(t, f.obj, classID, Uplink); got != want {
		t.Errorf("uplink counters = %+v, want %+v", got, want)
	}
}

func TestTrafficClassMapsMatchTheGoConstant(t *testing.T) {
	spec, err := LoadN3N6Entrypoint()
	if err != nil {
		t.Fatalf("load collection spec: %v", err)
	}

	if got, want := spec.Maps["traffic_class_forwarded"].MaxEntries, uint32(2*MaxTrafficClasses); got != want {
		t.Errorf("traffic_class_forwarded max_entries = %d, want %d", got, want)
	}
}
