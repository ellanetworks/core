// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package eps

import (
	"bytes"
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

	ar, err := ParseAttachRequest(sp.Payload)
	if err != nil {
		t.Fatal(err)
	}

	uecap, err := ParseUENetworkCapability(ar.UENetworkCapability)
	if err != nil {
		t.Fatal(err)
	}

	if !uecap.SupportsEEA(0) || !uecap.SupportsEEA(2) || !uecap.SupportsEIA(2) {
		t.Fatalf("captured UE caps EEA=%#x EIA=%#x, expected EEA0/EEA2/EIA2 support", uecap.EEA, uecap.EIA)
	}

	if !bytes.Equal(uecap.Marshal(), ar.UENetworkCapability) {
		t.Fatalf("UE network capability round-trip mismatch")
	}
}

func TestSessionIERoundTrips(t *testing.T) {
	t.Run("PDNAddress", func(t *testing.T) {
		for _, in := range []PDNAddress{
			{PDNType: PDNTypeIPv4, IPv4: [4]byte{10, 45, 0, 2}},
			{PDNType: PDNTypeIPv6, IPv6IID: [8]byte{1, 2, 3, 4, 5, 6, 7, 8}},
			{PDNType: PDNTypeIPv4v6, IPv4: [4]byte{10, 45, 0, 2}, IPv6IID: [8]byte{1, 2, 3, 4, 5, 6, 7, 8}},
		} {
			out, err := ParsePDNAddress(in.Marshal())
			if err != nil || out != in {
				t.Fatalf("type %d: got %+v err %v", in.PDNType, out, err)
			}
		}
	})

	t.Run("EPSQoS", func(t *testing.T) {
		in := EPSQoS{QCI: 9}

		out, err := ParseEPSQoS(in.Marshal())
		if err != nil || out.QCI != 9 {
			t.Fatalf("got %+v err %v", out, err)
		}
	})

	t.Run("APN", func(t *testing.T) {
		for _, apn := range []string{"internet", "ims.mnc001.mcc001.gprs"} {
			enc, err := EncodeAPN(apn)
			if err != nil {
				t.Fatal(err)
			}

			if got, err := DecodeAPN(enc); err != nil || got != apn {
				t.Fatalf("round-trip %q -> %q err %v", apn, got, err)
			}
		}
	})

	t.Run("APNAMBR", func(t *testing.T) {
		in := APNAMBR{DownlinkOctet: 0xfe, UplinkOctet: 0xfe, Extended: []byte{0x01, 0x02}}

		out, err := ParseAPNAMBR(in.Marshal())
		if err != nil || out.DownlinkOctet != 0xfe || out.UplinkOctet != 0xfe || !bytes.Equal(out.Extended, in.Extended) {
			t.Fatalf("got %+v err %v", out, err)
		}
	})

	t.Run("TAIList", func(t *testing.T) {
		in := TAIList{MCC: "001", MNC: "01", TACs: []uint16{0x0001, 0x3039}}

		b, err := in.Marshal()
		if err != nil {
			t.Fatal(err)
		}

		out, err := ParseTAIList(b)
		if err != nil || out.MCC != "001" || out.MNC != "01" || len(out.TACs) != 2 ||
			out.TACs[0] != 0x0001 || out.TACs[1] != 0x3039 {
			t.Fatalf("got %+v err %v", out, err)
		}
	})
}

// TestActivateDefaultBearerComposition checks the typed session IEs compose into
// the ESM message the MME must build for the default bearer.
func TestActivateDefaultBearerComposition(t *testing.T) {
	apn, err := EncodeAPN("internet")
	if err != nil {
		t.Fatal(err)
	}

	pdn := PDNAddress{PDNType: PDNTypeIPv4, IPv4: [4]byte{10, 45, 0, 2}}

	in := &ActivateDefaultEPSBearerContextRequest{
		EPSBearerIdentity:            5,
		ProcedureTransactionIdentity: 1,
		EPSQoS:                       EPSQoS{QCI: 9}.Marshal(),
		AccessPointName:              apn,
		PDNAddress:                   pdn.Marshal(),
	}

	raw, err := in.Marshal()
	if err != nil {
		t.Fatal(err)
	}

	out, err := ParseActivateDefaultEPSBearerContextRequest(raw)
	if err != nil {
		t.Fatal(err)
	}

	qos, err := ParseEPSQoS(out.EPSQoS)
	if err != nil || qos.QCI != 9 {
		t.Fatalf("EPS QoS: %+v err %v", qos, err)
	}

	gotAPN, err := DecodeAPN(out.AccessPointName)
	if err != nil || gotAPN != "internet" {
		t.Fatalf("APN: %q err %v", gotAPN, err)
	}

	gotPDN, err := ParsePDNAddress(out.PDNAddress)
	if err != nil || gotPDN != pdn {
		t.Fatalf("PDN address: %+v err %v", gotPDN, err)
	}
}
