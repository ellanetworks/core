// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package eps

import (
	"bytes"
	"testing"
)

// TestPDNConnectivityRequestGolden walks the full real capture end to end:
// S1AP NAS-PDU → security wrapper → Attach Request → its ESM container, which is
// a PDN CONNECTIVITY REQUEST, and checks it decodes and round-trips byte-exactly.
func TestPDNConnectivityRequestGolden(t *testing.T) {
	sp, err := ParseSecurityProtectedMessage(loadCapture(t, "attach_request_nas.hex"))
	if err != nil {
		t.Fatal(err)
	}

	ar, err := ParseAttachRequest(sp.Payload)
	if err != nil {
		t.Fatal(err)
	}

	if mt, err := PeekESMMessageType(ar.ESMMessageContainer); err != nil || mt != MsgPDNConnectivityRequest {
		t.Fatalf("ESM type = %#x, %v; want 0xd0", mt, err)
	}

	pc, err := ParsePDNConnectivityRequest(ar.ESMMessageContainer)
	if err != nil {
		t.Fatal(err)
	}

	if pc.EPSBearerIdentity != 0 || pc.ProcedureTransactionIdentity != 0x15 || pc.RequestType != 1 || pc.PDNType != 1 {
		t.Fatalf("PDN connectivity request mismatch: %+v", pc)
	}

	out, err := pc.Marshal()
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(out, ar.ESMMessageContainer) {
		t.Fatalf("round-trip mismatch:\n got  %x\n want %x", out, ar.ESMMessageContainer)
	}
}

func TestESMRoundTrips(t *testing.T) {
	t.Run("ActivateDefaultRequest", func(t *testing.T) {
		in := &ActivateDefaultEPSBearerContextRequest{
			EPSBearerIdentity: 5, ProcedureTransactionIdentity: 0,
			EPSQoS:          []byte{0x09},
			AccessPointName: []byte{0x03, 'i', 'o', 't'},
			PDNAddress:      []byte{0x01, 10, 45, 0, 2}, // PDN type IPv4 + 10.45.0.2
		}

		b, err := in.Marshal()
		if err != nil {
			t.Fatal(err)
		}

		out, err := ParseActivateDefaultEPSBearerContextRequest(b)
		if err != nil {
			t.Fatal(err)
		}

		if out.EPSBearerIdentity != 5 || out.ProcedureTransactionIdentity != 0 ||
			!bytes.Equal(out.EPSQoS, in.EPSQoS) || !bytes.Equal(out.AccessPointName, in.AccessPointName) ||
			!bytes.Equal(out.PDNAddress, in.PDNAddress) {
			t.Fatalf("mismatch:\n in  %+v\n out %+v", in, out)
		}
	})

	t.Run("ActivateDefaultAccept", func(t *testing.T) {
		in := &ActivateDefaultEPSBearerContextAccept{EPSBearerIdentity: 5, ProcedureTransactionIdentity: 0}

		b, _ := in.Marshal()

		out, err := ParseActivateDefaultEPSBearerContextAccept(b)
		if err != nil || out.EPSBearerIdentity != 5 {
			t.Fatalf("got %+v err %v", out, err)
		}
	})

	t.Run("ActivateDefaultReject", func(t *testing.T) {
		in := &ActivateDefaultEPSBearerContextReject{EPSBearerIdentity: 5, ESMCause: 26}

		b, _ := in.Marshal()

		out, err := ParseActivateDefaultEPSBearerContextReject(b)
		if err != nil || out.ESMCause != 26 || out.EPSBearerIdentity != 5 {
			t.Fatalf("got %+v err %v", out, err)
		}
	})

	t.Run("PDNConnectivityReject", func(t *testing.T) {
		in := &PDNConnectivityReject{ProcedureTransactionIdentity: 0x15, ESMCause: 27}

		b, _ := in.Marshal()

		out, err := ParsePDNConnectivityReject(b)
		if err != nil || out.ESMCause != 27 || out.ProcedureTransactionIdentity != 0x15 {
			t.Fatalf("got %+v err %v", out, err)
		}
	})

	t.Run("InfoRequest", func(t *testing.T) {
		b, _ := (&ESMInformationRequest{ProcedureTransactionIdentity: 1}).Marshal()

		out, err := ParseESMInformationRequest(b)
		if err != nil || out.ProcedureTransactionIdentity != 1 {
			t.Fatalf("got %+v err %v", out, err)
		}
	})

	t.Run("InfoResponse", func(t *testing.T) {
		in := &ESMInformationResponse{ProcedureTransactionIdentity: 1, AccessPointName: []byte("iot")}

		b, _ := in.Marshal()

		out, err := ParseESMInformationResponse(b)
		if err != nil || !bytes.Equal(out.AccessPointName, in.AccessPointName) {
			t.Fatalf("got %+v err %v", out, err)
		}
	})

	t.Run("Status", func(t *testing.T) {
		in := &ESMStatus{EPSBearerIdentity: 5, ESMCause: 43}

		b, _ := in.Marshal()

		out, err := ParseESMStatus(b)
		if err != nil || out.ESMCause != 43 {
			t.Fatalf("got %+v err %v", out, err)
		}
	})
}
