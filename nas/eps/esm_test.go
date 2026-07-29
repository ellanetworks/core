// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package eps

import (
	"bytes"
	"reflect"
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

	ar, err := ParseAttachRequest(sp.UnverifiedPayload)
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

	if pc.EPSBearerIdentity != 0 || pc.PTI != 0x15 || pc.RequestType != 1 || pc.PDNType != 1 {
		t.Fatalf("PDN connectivity request mismatch: %+v", pc)
	}

	out, err := pc.MarshalBinary()
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
			EPSBearerIdentity: 5, PTI: 0,
			EPSQoS:          EPSQoS{QCI: 9},
			AccessPointName: APN("iot"),
			PDNAddress:      PDNAddress{PDNType: PDNTypeIPv4, IPv4: [4]byte{10, 45, 0, 1}}, // PDN type IPv4 + 10.45.0.2
		}

		b, err := in.MarshalBinary()
		if err != nil {
			t.Fatal(err)
		}

		out, err := ParseActivateDefaultEPSBearerContextRequest(b)
		if err != nil {
			t.Fatal(err)
		}

		if out.EPSBearerIdentity != 5 || out.PTI != 0 ||
			out.EPSQoS.QCI != in.EPSQoS.QCI || out.AccessPointName != in.AccessPointName || !reflect.DeepEqual(out.PDNAddress, in.PDNAddress) {
			t.Fatalf("mismatch:\n in  %+v\n out %+v", in, out)
		}
	})

	t.Run("ActivateDefaultAccept", func(t *testing.T) {
		in := &ActivateDefaultEPSBearerContextAccept{EPSBearerIdentity: 5, PTI: 0}

		b, _ := in.MarshalBinary()

		out, err := ParseActivateDefaultEPSBearerContextAccept(b)
		if err != nil || out.EPSBearerIdentity != 5 {
			t.Fatalf("got %+v err %v", out, err)
		}
	})

	t.Run("ActivateDefaultReject", func(t *testing.T) {
		in := &ActivateDefaultEPSBearerContextReject{EPSBearerIdentity: 5, Cause: 26}

		b, _ := in.MarshalBinary()

		out, err := ParseActivateDefaultEPSBearerContextReject(b)
		if err != nil || out.Cause != 26 || out.EPSBearerIdentity != 5 {
			t.Fatalf("got %+v err %v", out, err)
		}
	})

	t.Run("PDNConnectivityReject", func(t *testing.T) {
		in := &PDNConnectivityReject{PTI: 0x15, Cause: 27}

		b, _ := in.MarshalBinary()

		out, err := ParsePDNConnectivityReject(b)
		if err != nil || out.Cause != 27 || out.PTI != 0x15 {
			t.Fatalf("got %+v err %v", out, err)
		}
	})

	t.Run("InfoRequest", func(t *testing.T) {
		b, _ := (&ESMInformationRequest{PTI: 1}).MarshalBinary()

		out, err := ParseESMInformationRequest(b)
		if err != nil || out.PTI != 1 {
			t.Fatalf("got %+v err %v", out, err)
		}
	})

	t.Run("InfoResponse", func(t *testing.T) {
		in := &ESMInformationResponse{PTI: 1, AccessPointName: ptr(APN("iot"))}

		b, _ := in.MarshalBinary()

		out, err := ParseESMInformationResponse(b)
		if err != nil || out.AccessPointName == nil || *out.AccessPointName != *in.AccessPointName {
			t.Fatalf("got %+v err %v", out, err)
		}
	})

	t.Run("Status", func(t *testing.T) {
		in := &ESMStatus{EPSBearerIdentity: 5, Cause: 43}

		b, _ := in.MarshalBinary()

		out, err := ParseESMStatus(b)
		if err != nil || out.Cause != 43 {
			t.Fatalf("got %+v err %v", out, err)
		}
	})
}

func TestParseESMHeader(t *testing.T) {
	// EBI 5, PTI 2, Activate Default EPS Bearer Context Accept (0xC2).
	hdr, err := ParseESMHeader([]byte{0x52, 0x02, 0xC2})
	if err != nil {
		t.Fatalf("ParseESMHeader error: %v", err)
	}

	if hdr.EPSBearerIdentity != 5 || hdr.PTI != 2 || hdr.MessageType != MsgActivateDefaultEPSBearerContextAccept {
		t.Errorf("ParseESMHeader = %+v", hdr)
	}

	// The header round-trips.
	raw, err := hdr.MarshalBinary()
	if err != nil {
		t.Fatalf("MarshalBinary: %v", err)
	}

	if !bytes.Equal(raw, []byte{0x52, 0x02, 0xC2}) {
		t.Errorf("MarshalBinary = %#x, want 5202c2", raw)
	}

	// A non-ESM protocol discriminator is rejected.
	if _, err := ParseESMHeader([]byte{0x07, 0x00, 0x00}); err == nil {
		t.Error("non-ESM PD: want error")
	}
}
