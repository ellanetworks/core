// Copyright 2026 Ella Networks
// SPDX-License-Identifier: Apache-2.0

//go:build linux

package ebpf

import (
	"bytes"
	"encoding/binary"
	"testing"
)

// GTP-U control-message handling and ICMPv6 Router Solicitation interception.
// PMTU / frag-needed generation needs a small-MTU device and is deferred to the
// netns harness.

// TestGTPControlMessages checks GTP-U control-message dispatch: an echo request
// is answered (XDP_TX, addresses/ports swapped, type set to echo response);
// other control messages are passed to the kernel.
func TestGTPControlMessages(t *testing.T) {
	requireProgTestRun(t)

	obj := loadN3N6Program(t)

	const (
		gtpEchoRequest     = 1
		gtpEchoResponse    = 2
		gtpErrorIndication = 26
		gtpEndMarker       = 254
	)

	t.Run("echo request gets response", func(t *testing.T) {
		in := gtpControlFrame(gtpEchoRequest)

		action, out := runXDPOut(t, obj.UpfN3N6EntrypointFunc, in)

		if action != XDP_TX {
			t.Fatalf("got XDP action %d, want XDP_TX (%d)", action, XDP_TX)
		}

		if len(out) != len(in) {
			t.Fatalf("frame length changed: got %d, want %d", len(out), len(in))
		}

		if out[ethHdrLen+20+8+1] != gtpEchoResponse {
			t.Errorf("GTP message type = %d, want %d (echo response)", out[ethHdrLen+20+8+1], gtpEchoResponse)
		}

		if !bytes.Equal(out[26:30], testUPFN3IP[:]) || !bytes.Equal(out[30:34], testGNBIP[:]) {
			t.Errorf("outer IPs not swapped: src=%v dst=%v", out[26:30], out[30:34])
		}

		if src, dst := binary.BigEndian.Uint16(out[34:36]), binary.BigEndian.Uint16(out[36:38]); src != GTPUDPPort || dst != 3000 {
			t.Errorf("UDP ports not swapped: src=%d dst=%d, want %d/%d", src, dst, GTPUDPPort, 3000)
		}

		if !bytes.Equal(out[0:6], []byte{0x02, 0, 0, 0, 0, 0x01}) || !bytes.Equal(out[6:12], []byte{0x02, 0, 0, 0, 0, 0x02}) {
			t.Errorf("MAC addresses not swapped: dst=%x src=%x", out[0:6], out[6:12])
		}
	})

	passThrough := []struct {
		name    string
		msgType uint8
	}{
		{"echo response passes", gtpEchoResponse},
		{"error indication passes", gtpErrorIndication},
		{"end marker passes", gtpEndMarker},
	}

	for _, tc := range passThrough {
		t.Run(tc.name, func(t *testing.T) {
			if action := runXDP(t, obj.UpfN3N6EntrypointFunc, gtpControlFrame(tc.msgType)); action != XDP_PASS {
				t.Fatalf("got XDP action %d, want XDP_PASS (%d)", action, XDP_PASS)
			}
		})
	}
}

// TestRouterSolicitationIntercept checks that an inner ICMPv6 Router
// Solicitation, after decapsulation, is intercepted and dropped (the RA is
// generated in userspace).
func TestRouterSolicitationIntercept(t *testing.T) {
	requireProgTestRun(t)

	const teid = 0x52530001

	obj := loadN3N6Program(t)
	putForwardingUplinkPDR(t, obj, teid, 0)

	action := runXDP(t, obj.UpfN3N6EntrypointFunc, uplinkGPDU(teid, innerIPv6ICMPv6RS(testUEv6)))

	if action != XDP_DROP {
		t.Fatalf("Router Solicitation not intercepted: got XDP action %d, want XDP_DROP (%d)", action, XDP_DROP)
	}
}
