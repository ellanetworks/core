// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: Apache-2.0

//go:build linux

package ebpf

import (
	"net/netip"
	"testing"
	"time"
)

// TestUplinkOversizedReadsInnerHeader: the N6 MTU check runs before
// decapsulation, so its verdict has to be acted on afterwards. Until it was,
// Don't Fragment was read from the gNB's transport header — a field the
// subscriber does not control — so a UE packet with DF set was discarded as
// df_not_set whenever the gNB left DF clear on its own header.
func TestUplinkOversizedReadsInnerHeader(t *testing.T) {
	requireProgTestRun(t)

	const (
		ulTEID = 0x4D545530
		ueSP   = 1300
		srvDP  = 80
	)

	f := setupT2(t, false)
	putForwardingUplinkPDRUE(t, f.obj, ulTEID, 0, netip.AddrFrom4(ueIP), netip.Addr{})

	// N6 narrower than N3, which is what makes the branch reachable at all.
	if out, err := ipCmd("link", "set", t2N6Dev, "mtu", "1200"); err != nil {
		t.Fatalf("shrink N6 MTU: %v: %s", err, out)
	}

	// Inner packet sets DF; the outer transport header built by uplinkGPDU
	// leaves it clear. The two must not be confused.
	inner := withDF(ipv4Packet(ueIP, serverIP, 6,
		tcpSegmentChecksummed(ueIP, serverIP, ueSP, srvDP, bytesOf(1400))))

	n3 := f.captureN3(t)
	n6 := f.captureN6(t)

	beforeMTU := DropCount(f.obj, Uplink, "mtu_exceeded")
	beforeDF := DropCount(f.obj, Uplink, "df_not_set")

	f.injectUplink(t, uplinkGPDU(ulTEID, inner))

	time.Sleep(150 * time.Millisecond)

	afterMTU := DropCount(f.obj, Uplink, "mtu_exceeded")
	afterDF := DropCount(f.obj, Uplink, "df_not_set")

	if afterDF != beforeDF {
		t.Errorf("df_not_set fired (%d -> %d): DF was read from the gNB's transport header, not the UE's packet",
			beforeDF, afterDF)
	}

	if afterMTU != beforeMTU+1 {
		t.Errorf("mtu_exceeded = %d, want %d", afterMTU, beforeMTU+1)
	}

	// Nothing may leave: not the oversized packet, and not an ICMP built
	// from the tunnel header addressed to the gNB.
	if fr := captureMatching(n6, 300*time.Millisecond, func(fr []byte) bool {
		return isInnerIPv4(fr, 6, serverIP)
	}); fr != nil {
		t.Errorf("the oversized packet egressed on N6 despite exceeding its MTU")
	}

	if fr := captureMatching(n3, 300*time.Millisecond, func(fr []byte) bool {
		return len(fr) >= ethHdrLen+20 && fr[ethHdrLen+9] == 1 // ICMP
	}); fr != nil {
		t.Errorf("an ICMP error egressed on N3 toward the gNB, quoting the tunnel rather than the UE's packet")
	}
}
