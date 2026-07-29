// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package eps

import (
	"bytes"
	"reflect"
	"testing"
)

// TestUENetworkCapabilityGolden parses the UE network capability from the real
// captured Attach Request and checks the algorithm-selection helpers — this is
// what the MME uses to pick NAS algorithms.
func TestUENetworkCapabilityGolden(t *testing.T) {
	sp, err := ParseSecurityProtectedMessage(loadCapture(t, "attach_request_nas.hex"))
	if err != nil {
		t.Fatal(err)
	}

	ar, err := ParseAttachRequest(sp.UnverifiedPayload)
	if err != nil {
		t.Fatal(err)
	}

	uecap := ar.UENetworkCapability
	if !uecap.SupportsEEA(0) || !uecap.SupportsEEA(2) || !uecap.SupportsEIA(2) {
		t.Fatalf("captured UE caps EEA=%#x EIA=%#x, expected EEA0/EEA2/EIA2 support", uecap.EEA, uecap.EIA)
	}

	// The capture re-encodes byte-for-byte, so the codec loses nothing it decoded.
	again, err := uecap.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}

	if reparsed, err := ParseUENetworkCapability(again); err != nil || !reparsed.Equal(uecap) {
		t.Fatalf("UE network capability round-trip mismatch: %+v (%v)", reparsed, err)
	}
}

func TestSessionIERoundTrips(t *testing.T) {
	t.Run("PDNAddress", func(t *testing.T) {
		for _, in := range []PDNAddress{
			{PDNType: PDNTypeIPv4, IPv4: [4]byte{10, 45, 0, 2}},
			{PDNType: PDNTypeIPv6, IPv6IID: [8]byte{1, 2, 3, 4, 5, 6, 7, 8}},
			{PDNType: PDNTypeIPv4v6, IPv4: [4]byte{10, 45, 0, 2}, IPv6IID: [8]byte{1, 2, 3, 4, 5, 6, 7, 8}},
		} {
			out, err := ParsePDNAddress(mustBytes(in.MarshalBinary()))
			if err != nil || !reflect.DeepEqual(out, in) {
				t.Fatalf("type %d: got %+v err %v", in.PDNType, out, err)
			}
		}
	})

	t.Run("EPSQoS", func(t *testing.T) {
		in := EPSQoS{QCI: 9}

		out, err := ParseEPSQoS(mustBytes(in.MarshalBinary()))
		if err != nil || out.QCI != 9 {
			t.Fatalf("got %+v err %v", out, err)
		}
	})

	t.Run("APN", func(t *testing.T) {
		for _, apn := range []string{"internet", "ims.mnc001.mcc001.gprs"} {
			enc, err := APN(apn).MarshalBinary()
			if err != nil {
				t.Fatal(err)
			}

			if got, err := ParseAPN(enc); err != nil || string(got) != apn {
				t.Fatalf("round-trip %q -> %q err %v", apn, got, err)
			}
		}
	})

	t.Run("APNAMBR", func(t *testing.T) {
		in := APNAMBR{DownlinkOctet: 0xfe, UplinkOctet: 0xfe, Extended: []byte{0x01, 0x02}}

		out, err := ParseAPNAMBR(mustBytes(in.MarshalBinary()))
		if err != nil || out.DownlinkOctet != 0xfe || out.UplinkOctet != 0xfe || !bytes.Equal(out.Extended, in.Extended) {
			t.Fatalf("got %+v err %v", out, err)
		}
	})
}

// TestActivateDefaultBearerComposition checks the typed session IEs compose into
// the ESM message the MME must build for the default bearer.
func TestActivateDefaultBearerComposition(t *testing.T) {
	pdn := PDNAddress{PDNType: PDNTypeIPv4, IPv4: [4]byte{10, 45, 0, 2}}

	in := &ActivateDefaultEPSBearerContextRequest{
		EPSBearerIdentity: 5,
		PTI:               1,
		EPSQoS:            EPSQoS{QCI: 9},
		AccessPointName:   APN("internet"),
		PDNAddress:        pdn,
	}

	raw, err := in.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}

	out, err := ParseActivateDefaultEPSBearerContextRequest(raw)
	if err != nil {
		t.Fatal(err)
	}

	if out.EPSQoS.QCI != 9 {
		t.Fatalf("EPS QoS: %+v", out.EPSQoS)
	}

	if out.AccessPointName != "internet" {
		t.Fatalf("APN: %q", out.AccessPointName)
	}

	if !reflect.DeepEqual(out.PDNAddress, pdn) {
		t.Fatalf("PDN address: %+v", out.PDNAddress)
	}
}
