// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package eps

import (
	"bytes"
	"reflect"
	"testing"
	"time"

	"github.com/ellanetworks/core/nas"
)

func TestSecurityModeRoundTrips(t *testing.T) {
	t.Run("Command", func(t *testing.T) {
		in := &SecurityModeCommand{
			CipheringAlgorithm: 2, IntegrityAlgorithm: 2, NASKeySetIdentifier: nas.NoKeySet,
			ReplayedUESecurityCapability: UESecurityCapability{EEA: 0xf0, EIA: 0xf0, HasUMTS: true, UEA: 0xc0, UIA: 0xc0},
			IMEISVRequested:              ptr(IMEISVRequested),
			HASHMME:                      []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08},
		}

		b, err := in.MarshalBinary()
		if err != nil {
			t.Fatal(err)
		}

		out, err := ParseSecurityModeCommand(b)
		if err != nil {
			t.Fatal(err)
		}

		if out.CipheringAlgorithm != 2 || out.IntegrityAlgorithm != 2 || out.NASKeySetIdentifier != nas.NoKeySet ||
			!out.ReplayedUESecurityCapability.Equal(in.ReplayedUESecurityCapability) ||
			out.IMEISVRequested == nil || !out.IMEISVRequested.Requested() ||
			!bytes.Equal(out.HASHMME, in.HASHMME) {
			t.Fatalf("mismatch:\n in  %+v\n out %+v", in, out)
		}
	})

	t.Run("Complete", func(t *testing.T) {
		imeisv := MobileIMEISV("3536083123456780")
		in := &SecurityModeComplete{IMEISV: &imeisv}

		b, _ := in.MarshalBinary()

		out, err := ParseSecurityModeComplete(b)
		if err != nil || out.IMEISV == nil || !reflect.DeepEqual(*out.IMEISV, imeisv) {
			t.Fatalf("got %+v err %v", out, err)
		}
	})

	t.Run("Complete with replayed NAS message container", func(t *testing.T) {
		imeisv := MobileIMEISV("3536083123456780")
		replayed := bytes.Repeat([]byte{0xAB}, 300) // exercises the two-octet TLV-E length
		in := &SecurityModeComplete{IMEISV: &imeisv, ReplayedNASMessageContainer: replayed}

		b, err := in.MarshalBinary()
		if err != nil {
			t.Fatal(err)
		}

		// The Replayed NAS message container IE is IEI 0x79 (TS 24.301
		// table 8.2.21.1), carried as a TLV-E; pin it on the wire so the round-trip
		// below cannot hide a spec-wrong IEI (both sides would share it otherwise).
		if !bytes.Contains(b, []byte{0x79, 0x01, 0x2C}) {
			t.Fatalf("replayed NAS container not encoded under IEI 0x79 with length 300: % x", b)
		}

		out, err := ParseSecurityModeComplete(b)
		if err != nil {
			t.Fatal(err)
		}

		if out.IMEISV == nil || !reflect.DeepEqual(*out.IMEISV, imeisv) || !bytes.Equal(out.ReplayedNASMessageContainer, replayed) {
			t.Fatalf("mismatch:\n in  %+v\n out %+v", in, out)
		}
	})

	t.Run("Reject", func(t *testing.T) {
		b, _ := (&SecurityModeReject{Cause: 24}).MarshalBinary()

		out, err := ParseSecurityModeReject(b)
		if err != nil || out.Cause != 24 {
			t.Fatalf("got %+v err %v", out, err)
		}
	})
}

func TestAttachNetworkRoundTrips(t *testing.T) {
	t.Run("Accept", func(t *testing.T) {
		cause := uint8(18) // CS domain not available
		guti := GUTIIdentity(GUTI{PLMN: nas.PLMN{MCC: "001", MNC: "01"}, MMEGroupID: 2, MMECode: 1, TMSI: [4]byte{0x03, 0x00, 0x03, 0xe6}})
		in := &AttachAccept{
			EPSAttachResult:     AttachResultEPS,
			T3412:               nas.GPRSTimer2{Unit: nas.GPRSTimer2Unit1Minute, Value: 3},
			TAIList:             testTAIList(),
			ESMMessageContainer: []byte{0x02, 0x01, 0xc2},
			GUTI:                &guti,
			Cause:               ptr(EMMCause(cause)),
		}

		b, err := in.MarshalBinary()
		if err != nil {
			t.Fatal(err)
		}

		out, err := ParseAttachAccept(b)
		if err != nil {
			t.Fatal(err)
		}

		if out.GUTI == nil {
			t.Fatal("GUTI absent")
		}

		if out.EPSAttachResult != in.EPSAttachResult || out.T3412 != in.T3412 ||
			!reflect.DeepEqual(out.TAIList, in.TAIList) || !bytes.Equal(out.ESMMessageContainer, in.ESMMessageContainer) ||
			!reflect.DeepEqual(*out.GUTI, guti) ||
			out.Cause == nil || *out.Cause != *in.Cause {
			t.Fatalf("mismatch:\n in  %+v\n out %+v", in, out)
		}
	})

	t.Run("Complete", func(t *testing.T) {
		in := &AttachComplete{ESMMessageContainer: []byte{0x02, 0x01, 0xc3, 0x00}}

		b, _ := in.MarshalBinary()

		out, err := ParseAttachComplete(b)
		if err != nil || !bytes.Equal(out.ESMMessageContainer, in.ESMMessageContainer) {
			t.Fatalf("got %+v err %v", out, err)
		}
	})

	t.Run("Reject", func(t *testing.T) {
		in := &AttachReject{Cause: 11}

		b, _ := in.MarshalBinary()

		out, err := ParseAttachReject(b)
		if err != nil || out.Cause != 11 {
			t.Fatalf("got %+v err %v", out, err)
		}
	})

	t.Run("RejectWithT3402", func(t *testing.T) {
		t3402, err := nas.GPRSTimer2FromDuration(12 * time.Minute)
		if err != nil {
			t.Fatal(err)
		}

		in := &AttachReject{Cause: 11, T3402: &t3402}

		b, err := in.MarshalBinary()
		if err != nil {
			t.Fatal(err)
		}

		out, err := ParseAttachReject(b)
		if err != nil || out.Cause != 11 || out.T3402 == nil || *out.T3402 != t3402 {
			t.Fatalf("T3402 round-trip: got %+v (want cause 11, T3402 %v), err %v", out, t3402, err)
		}

		// The ATTACH REJECT T3402 is IEI 0x16 "GPRS timer 2", TLV (TS 24.301
		// §8.2.3.1) — not the ATTACH ACCEPT's IEI 0x17 TV.
		octet, err := t3402.MarshalBinary()
		if err != nil {
			t.Fatal(err)
		}

		want := []byte{b[0], b[1], 11, 0x16, 0x01, octet[0]}
		if !bytes.Equal(b, want) {
			t.Fatalf("ATTACH REJECT T3402 encoding = % x, want % x", b, want)
		}
	})

	t.Run("RejectWithESMMessageContainer", func(t *testing.T) {
		esm, err := (&PDNConnectivityReject{PTI: 1, Cause: ESMCauseMissingOrUnknownAPN}).MarshalBinary()
		if err != nil {
			t.Fatal(err)
		}

		in := &AttachReject{Cause: EMMCauseESMFailure, ESMMessageContainer: esm}

		b, err := in.MarshalBinary()
		if err != nil {
			t.Fatal(err)
		}

		wantIE := append([]byte{0x78, byte(len(esm) >> 8), byte(len(esm))}, esm...)
		if !bytes.Contains(b, wantIE) {
			t.Fatalf("encoded %x does not contain the ESM message container IE %x", b, wantIE)
		}

		out, err := ParseAttachReject(b)
		if err != nil {
			t.Fatal(err)
		}

		if out.Cause != EMMCauseESMFailure {
			t.Errorf("EMM cause = %d, want %d", out.Cause, EMMCauseESMFailure)
		}

		if !bytes.Equal(out.ESMMessageContainer, esm) {
			t.Fatalf("ESM message container = % x, want % x", out.ESMMessageContainer, esm)
		}

		inner, err := ParsePDNConnectivityReject(out.ESMMessageContainer)
		if err != nil {
			t.Fatalf("parse the carried PDN Connectivity Reject: %v", err)
		}

		if inner.Cause != ESMCauseMissingOrUnknownAPN {
			t.Errorf("carried ESM cause = %d, want %d", inner.Cause, ESMCauseMissingOrUnknownAPN)
		}
	})
}

func TestNetworkFeatureSupportRejectsAnOverlongValue(t *testing.T) {
	for name, in := range map[string]NetworkFeatureSupport{
		"declared": {IMSVoPS: true, HasOctet4: true, Rest: []byte{0x00, 0x00}},
		"implied":  {IMSVoPS: true, IWKN26: true, Rest: []byte{0x00, 0x00}},
	} {
		t.Run(name, func(t *testing.T) {
			if b, err := in.MarshalBinary(); err == nil {
				t.Fatalf("MarshalBinary = % x, nil, want an error for a %d-octet value", b, len(b))
			}
		})
	}
}

// TestNetworkFeatureSupportRoundTrips checks the EPS network feature support
// IE (TS 24.301) encodes as IEI 0x64, length 1, octet 3 bit 1 for the
// IMS VoPS indicator and survives a round trip in ATTACH ACCEPT and TRACKING
// AREA UPDATE ACCEPT.
func TestNetworkFeatureSupportRoundTrips(t *testing.T) {
	wantIE := []byte{0x64, 0x01, 0x01} // IEI, length, IMS VoPS = supported

	t.Run("AttachAccept", func(t *testing.T) {
		in := &AttachAccept{
			EPSAttachResult:       AttachResultEPS,
			T3412:                 nas.GPRSTimer2{Unit: nas.GPRSTimer2Unit1Minute, Value: 3},
			TAIList:               testTAIList(),
			ESMMessageContainer:   []byte{0x02, 0x01, 0xc2},
			NetworkFeatureSupport: &NetworkFeatureSupport{IMSVoPS: true},
		}

		b, err := in.MarshalBinary()
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

		if out.NetworkFeatureSupport == nil || !out.NetworkFeatureSupport.IMSVoPS {
			t.Fatalf("IMS VoPS not decoded: %+v", out.NetworkFeatureSupport)
		}
	})

	t.Run("TrackingAreaUpdateAccept", func(t *testing.T) {
		in := &TrackingAreaUpdateAccept{
			EPSUpdateResult:       EPSUpdateResultTA,
			NetworkFeatureSupport: &NetworkFeatureSupport{IMSVoPS: true},
		}

		b, err := in.MarshalBinary()
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

		if out.NetworkFeatureSupport == nil || !out.NetworkFeatureSupport.IMSVoPS {
			t.Fatalf("IMS VoPS not decoded: %+v", out.NetworkFeatureSupport)
		}
	})

	t.Run("Absent", func(t *testing.T) {
		b, err := (&AttachAccept{EPSAttachResult: AttachResultEPS, TAIList: testTAIList()}).MarshalBinary()
		if err != nil {
			t.Fatal(err)
		}

		out, err := ParseAttachAccept(b)
		if err != nil {
			t.Fatal(err)
		}

		if out.NetworkFeatureSupport != nil {
			t.Fatalf("EPS network feature support should be absent, got %+v", out.NetworkFeatureSupport)
		}
	})
}
