// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: Apache-2.0

//go:build linux

package ebpf

import (
	"bytes"
	"encoding/binary"
	"net/netip"
	"testing"
)

func tcpFragment(src, dst [4]byte, id, offsetUnits uint16, more bool, transportLen int) []byte {
	payload := make([]byte, 20)
	if offsetUnits == 0 && transportLen > 0 {
		payload = payload[:transportLen]
	}

	pkt := ipv4Packet(src, dst, 6, payload)

	frag := offsetUnits
	if more {
		frag |= 0x2000
	}

	binary.BigEndian.PutUint16(pkt[4:6], id)
	binary.BigEndian.PutUint16(pkt[6:8], frag)
	binary.BigEndian.PutUint16(pkt[10:12], 0)
	binary.BigEndian.PutUint16(pkt[10:12], ipv4HeaderChecksum(pkt[:20]))

	return pkt
}

const (
	fragTEID  = 0x46524147
	fragSport = 4000
	fragDport = 53
)

func TestFragmentPortsRecoveredUplink(t *testing.T) {
	requireProgTestRun(t)

	const filterIndex = 1

	obj := loadN3N6Program(t)

	remote := [4]byte{8, 8, 8, 8}
	ue := canonicalUEv4.As4()

	putSDFFilter(t, obj, filterIndex, []SdfRule{
		sdfRuleIPv4(remote, 32, fragDport, fragDport, 17, SdfActionDeny),
	})
	putForwardingUplinkPDR(t, obj, fragTEID, filterIndex)

	first := innerIPv4FragmentID(ue, remote, 0x1234, 0, true, fragSport, fragDport)
	later := innerIPv4FragmentID(ue, remote, 0x1234, 2, false, 0, 0)

	if reason := uplinkDropReason(t, obj, fragTEID, first); reason != "sdf_filter" {
		t.Fatalf("first fragment drop reason = %q, want sdf_filter", reason)
	}

	if reason := uplinkDropReason(t, obj, fragTEID, later); reason != "sdf_filter" {
		t.Errorf("later fragment drop reason = %q, want sdf_filter: its ports were not recovered", reason)
	}
}

func TestFragmentUnresolvedIsDeliveredWhenPortsAreNotNeeded(t *testing.T) {
	requireProgTestRun(t)

	obj := loadN3N6Program(t)

	remote := [4]byte{8, 8, 8, 8}
	ue := canonicalUEv4.As4()

	putForwardingUplinkPDR(t, obj, fragTEID, 0)

	orphan := innerIPv4FragmentID(ue, remote, 0x4321, 2, false, 0, 0)

	if reason := uplinkDropReason(t, obj, fragTEID, orphan); reason != "" {
		t.Errorf("orphan fragment drop reason = %q, want delivered", reason)
	}
}

func TestFragmentUnresolvedCounted(t *testing.T) {
	requireProgTestRun(t)

	obj := loadN3N6Program(t)

	remote := [4]byte{8, 8, 8, 8}
	ue := canonicalUEv4.As4()

	putForwardingUplinkPDR(t, obj, fragTEID, 0)

	before := FragUnresolvedCount(obj, Uplink)

	orphan := innerIPv4FragmentID(ue, remote, 0x5555, 2, false, 0, 0)
	runXDP(t, obj.UpfEntryFunc, uplinkGPDU(fragTEID, orphan))

	if after := FragUnresolvedCount(obj, Uplink); after != before+1 {
		t.Errorf("frag_unresolved = %d, want %d", after, before+1)
	}
}

// RFC 6274 §4.1.2.6
func TestFragmentFirstWins(t *testing.T) {
	requireProgTestRun(t)

	const filterIndex = 1

	obj := loadN3N6Program(t)

	remote := [4]byte{8, 8, 8, 8}
	ue := canonicalUEv4.As4()

	putSDFFilter(t, obj, filterIndex, []SdfRule{
		sdfRuleIPv4(remote, 32, fragDport, fragDport, 17, SdfActionDeny),
	})
	putForwardingUplinkPDR(t, obj, fragTEID, filterIndex)

	first := innerIPv4FragmentID(ue, remote, 0x9999, 0, true, fragSport, fragDport)
	forged := innerIPv4FragmentID(ue, remote, 0x9999, 0, true, fragSport, 9999)
	later := innerIPv4FragmentID(ue, remote, 0x9999, 2, false, 0, 0)

	runXDP(t, obj.UpfEntryFunc, uplinkGPDU(fragTEID, first))
	runXDP(t, obj.UpfEntryFunc, uplinkGPDU(fragTEID, forged))

	if reason := uplinkDropReason(t, obj, fragTEID, later); reason != "sdf_filter" {
		t.Errorf("later fragment drop reason = %q, want sdf_filter: the forged first fragment replaced the recorded ports", reason)
	}
}

func TestFragmentSessionsDoNotCollide(t *testing.T) {
	requireProgTestRun(t)

	const (
		teidA       = 0x4652410A
		teidB       = 0x4652410B
		filterIndex = 1
	)

	obj := loadN3N6Program(t)

	remote := [4]byte{8, 8, 8, 8}
	ueA := netip.MustParseAddr("10.0.0.9")
	ueB := netip.MustParseAddr("10.0.0.10")

	putSDFFilter(t, obj, filterIndex, []SdfRule{
		sdfRuleIPv4(remote, 32, fragDport, fragDport, 17, SdfActionDeny),
	})

	putForwardingUplinkPDRUE(t, obj, teidA, filterIndex, ueA, canonicalUEv6Prefix)
	putForwardingUplinkPDRUE(t, obj, teidB, filterIndex, ueB, canonicalUEv6Prefix)

	firstA := innerIPv4FragmentID(ueA.As4(), remote, 0x7777, 0, true, fragSport, fragDport)
	firstB := innerIPv4FragmentID(ueB.As4(), remote, 0x7777, 0, true, fragSport, 8080)
	laterB := innerIPv4FragmentID(ueB.As4(), remote, 0x7777, 2, false, 0, 0)

	runXDP(t, obj.UpfEntryFunc, uplinkGPDU(teidA, firstA))
	runXDP(t, obj.UpfEntryFunc, uplinkGPDU(teidB, firstB))

	if reason := uplinkDropReason(t, obj, teidB, laterB); reason != "" {
		t.Errorf("session B's fragment drop reason = %q, want delivered: it resolved against session A's datagram", reason)
	}
}

// RFC 3128 §3
func TestFragmentTinyRejected(t *testing.T) {
	requireProgTestRun(t)

	obj := loadN3N6Program(t)

	remote := [4]byte{8, 8, 8, 8}
	ue := canonicalUEv4.As4()

	putForwardingUplinkPDR(t, obj, fragTEID, 0)

	tests := []struct {
		name  string
		inner []byte
	}{
		{"offset one", tcpFragment(ue, remote, 0x2222, 1, false, 0)},
		{"truncated first fragment", tcpFragment(ue, remote, 0x2223, 0, true, 8)},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			before := DropCount(obj, Uplink, "fragment_malformed")

			action := runXDP(t, obj.UpfEntryFunc, uplinkGPDU(fragTEID, tc.inner))
			if action == ActionRedirect || action == ActionTx {
				t.Errorf("got XDP action %d, want the fragment refused", action)
			}

			if after := DropCount(obj, Uplink, "fragment_malformed"); after != before+1 {
				t.Errorf("fragment_malformed = %d, want %d", after, before+1)
			}
		})
	}
}

func TestFragmentIPv6HeaderRestored(t *testing.T) {
	requireProgTestRun(t)

	const teid = 0x46524736

	obj := loadN3N6Program(t)

	putForwardingUplinkPDR(t, obj, teid, 0)

	inner := innerIPv6NonFirstFragment(testUEv6, 53)
	want := append([]byte(nil), inner[:8]...)

	_, out := runXDPOut(t, obj.UpfEntryFunc, uplinkGPDU(teid, inner))

	src := bytes.Index(out, testUEv6Src[:])
	if src < 8 {
		t.Fatalf("inner IPv6 header not found in the %d-byte output frame", len(out))
	}

	if got := out[src-8 : src]; !bytes.Equal(got, want) {
		t.Errorf("IPv6 header head = % x, want % x: the key overlay was not restored", got, want)
	}
}
