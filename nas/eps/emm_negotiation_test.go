// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package eps

import (
	"bytes"
	"testing"
	"time"
)

func TestSecurityModeRoundTrips(t *testing.T) {
	t.Run("Command", func(t *testing.T) {
		in := &SecurityModeCommand{
			CipheringAlgorithm: 2, IntegrityAlgorithm: 2, NASKeySetIdentifier: 0x07,
			ReplayedUESecurityCapabilities: []byte{0xf0, 0xf0, 0xc0, 0xc0},
			IMEISVRequested:                true,
			HASHMME:                        []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08},
		}

		b, err := in.Marshal()
		if err != nil {
			t.Fatal(err)
		}

		out, err := ParseSecurityModeCommand(b)
		if err != nil {
			t.Fatal(err)
		}

		if out.CipheringAlgorithm != 2 || out.IntegrityAlgorithm != 2 || out.NASKeySetIdentifier != 7 ||
			!bytes.Equal(out.ReplayedUESecurityCapabilities, in.ReplayedUESecurityCapabilities) ||
			!out.IMEISVRequested || !bytes.Equal(out.HASHMME, in.HASHMME) {
			t.Fatalf("mismatch:\n in  %+v\n out %+v", in, out)
		}
	})

	t.Run("Complete", func(t *testing.T) {
		imeisv := []byte{0x03, 0x53, 0x60, 0x83, 0x12, 0x34, 0x56, 0x78, 0xf0} // IMEISV mobile identity
		in := &SecurityModeComplete{IMEISV: imeisv}

		b, _ := in.Marshal()

		out, err := ParseSecurityModeComplete(b)
		if err != nil || !bytes.Equal(out.IMEISV, imeisv) {
			t.Fatalf("got %+v err %v", out, err)
		}
	})

	t.Run("Complete with replayed NAS message container", func(t *testing.T) {
		imeisv := []byte{0x03, 0x53, 0x60, 0x83, 0x12, 0x34, 0x56, 0x78, 0xf0}
		replayed := bytes.Repeat([]byte{0xAB}, 300) // exercises the two-octet TLV-E length
		in := &SecurityModeComplete{IMEISV: imeisv, ReplayedNASMessage: replayed}

		b, err := in.Marshal()
		if err != nil {
			t.Fatal(err)
		}

		out, err := ParseSecurityModeComplete(b)
		if err != nil {
			t.Fatal(err)
		}

		if !bytes.Equal(out.IMEISV, imeisv) || !bytes.Equal(out.ReplayedNASMessage, replayed) {
			t.Fatalf("mismatch:\n in  %+v\n out %+v", in, out)
		}
	})

	t.Run("Reject", func(t *testing.T) {
		b, _ := (&SecurityModeReject{Cause: 24}).Marshal()

		out, err := ParseSecurityModeReject(b)
		if err != nil || out.Cause != 24 {
			t.Fatalf("got %+v err %v", out, err)
		}
	})
}

func TestAttachNetworkRoundTrips(t *testing.T) {
	t.Run("Accept", func(t *testing.T) {
		cause := uint8(18) // CS domain not available
		in := &AttachAccept{
			EPSAttachResult:     AttachResultEPS,
			T3412:               0x23,
			TAIList:             []byte{0x00, 0x02, 0xf1, 0x10, 0x00, 0x01},
			ESMMessageContainer: []byte{0x02, 0x01, 0xc2},
			GUTI:                &EPSMobileIdentity{Type: IdentityGUTI, MCC: "001", MNC: "01", MMEGroupID: 2, MMECode: 1, MTMSI: 0x030003e6},
			EMMCause:            &cause,
		}

		b, err := in.Marshal()
		if err != nil {
			t.Fatal(err)
		}

		out, err := ParseAttachAccept(b)
		if err != nil {
			t.Fatal(err)
		}

		if out.EPSAttachResult != in.EPSAttachResult || out.T3412 != in.T3412 ||
			!bytes.Equal(out.TAIList, in.TAIList) || !bytes.Equal(out.ESMMessageContainer, in.ESMMessageContainer) ||
			out.GUTI == nil || *out.GUTI != *in.GUTI ||
			out.EMMCause == nil || *out.EMMCause != *in.EMMCause {
			t.Fatalf("mismatch:\n in  %+v\n out %+v", in, out)
		}
	})

	t.Run("Complete", func(t *testing.T) {
		in := &AttachComplete{ESMMessageContainer: []byte{0x02, 0x01, 0xc3, 0x00}}

		b, _ := in.Marshal()

		out, err := ParseAttachComplete(b)
		if err != nil || !bytes.Equal(out.ESMMessageContainer, in.ESMMessageContainer) {
			t.Fatalf("got %+v err %v", out, err)
		}
	})

	t.Run("Reject", func(t *testing.T) {
		in := &AttachReject{Cause: 11}

		b, _ := in.Marshal()

		out, err := ParseAttachReject(b)
		if err != nil || out.Cause != 11 {
			t.Fatalf("got %+v err %v", out, err)
		}
	})

	t.Run("RejectWithT3402", func(t *testing.T) {
		t3402, err := EncodeGPRSTimer(12 * time.Minute)
		if err != nil {
			t.Fatal(err)
		}

		in := &AttachReject{Cause: 11, T3402: t3402}

		b, err := in.Marshal()
		if err != nil {
			t.Fatal(err)
		}

		out, err := ParseAttachReject(b)
		if err != nil || out.Cause != 11 || out.T3402 != t3402 {
			t.Fatalf("T3402 round-trip: got %+v (want cause 11, T3402 %#x), err %v", out, t3402, err)
		}

		// The ATTACH REJECT T3402 is IEI 0x16 "GPRS timer 2", TLV (TS 24.301
		// §8.2.3.1) — not the ATTACH ACCEPT's IEI 0x17 TV.
		want := []byte{b[0], b[1], 11, 0x16, 0x01, t3402}
		if !bytes.Equal(b, want) {
			t.Fatalf("ATTACH REJECT T3402 encoding = % x, want % x", b, want)
		}
	})
}

// TestEPSNetworkFeatureSupportRoundTrips checks the EPS network feature support
// IE (TS 24.301) encodes as IEI 0x64, length 1, octet 3 bit 1 for the
// IMS VoPS indicator and survives a round trip in ATTACH ACCEPT and TRACKING
// AREA UPDATE ACCEPT.
func TestEPSNetworkFeatureSupportRoundTrips(t *testing.T) {
	wantIE := []byte{0x64, 0x01, 0x01} // IEI, length, IMS VoPS = supported

	t.Run("AttachAccept", func(t *testing.T) {
		in := &AttachAccept{
			EPSAttachResult:          AttachResultEPS,
			T3412:                    0x23,
			TAIList:                  []byte{0x00, 0x02, 0xf1, 0x10, 0x00, 0x01},
			ESMMessageContainer:      []byte{0x02, 0x01, 0xc2},
			EPSNetworkFeatureSupport: &EPSNetworkFeatureSupport{IMSVoPS: true},
		}

		b, err := in.Marshal()
		if err != nil {
			t.Fatal(err)
		}

		if !bytes.Contains(b, wantIE) {
			t.Fatalf("encoded %x does not contain EPS network feature support IE %x", b, wantIE)
		}

		out, err := ParseAttachAccept(b)
		if err != nil {
			t.Fatal(err)
		}

		if out.EPSNetworkFeatureSupport == nil || !out.EPSNetworkFeatureSupport.IMSVoPS {
			t.Fatalf("IMS VoPS not decoded: %+v", out.EPSNetworkFeatureSupport)
		}
	})

	t.Run("TrackingAreaUpdateAccept", func(t *testing.T) {
		in := &TrackingAreaUpdateAccept{
			EPSUpdateResult:          EPSUpdateResultTA,
			EPSNetworkFeatureSupport: &EPSNetworkFeatureSupport{IMSVoPS: true},
		}

		b, err := in.Marshal()
		if err != nil {
			t.Fatal(err)
		}

		if !bytes.Contains(b, wantIE) {
			t.Fatalf("encoded %x does not contain EPS network feature support IE %x", b, wantIE)
		}

		out, err := ParseTrackingAreaUpdateAccept(b)
		if err != nil {
			t.Fatal(err)
		}

		if out.EPSNetworkFeatureSupport == nil || !out.EPSNetworkFeatureSupport.IMSVoPS {
			t.Fatalf("IMS VoPS not decoded: %+v", out.EPSNetworkFeatureSupport)
		}
	})

	t.Run("Absent", func(t *testing.T) {
		b, err := (&AttachAccept{EPSAttachResult: AttachResultEPS}).Marshal()
		if err != nil {
			t.Fatal(err)
		}

		out, err := ParseAttachAccept(b)
		if err != nil {
			t.Fatal(err)
		}

		if out.EPSNetworkFeatureSupport != nil {
			t.Fatalf("EPS network feature support should be absent, got %+v", out.EPSNetworkFeatureSupport)
		}
	})
}
