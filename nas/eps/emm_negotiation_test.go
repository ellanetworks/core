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

	t.Run("RejectWithESMContainer", func(t *testing.T) {
		esm, err := (&PDNConnectivityReject{PTI: 1, Cause: ESMCausePDNConnectionDoesNotExist}).MarshalBinary()
		if err != nil {
			t.Fatal(err)
		}

		in := &AttachReject{Cause: EMMCauseESMFailure, ESMMessageContainer: esm}

		b, err := in.MarshalBinary()
		if err != nil {
			t.Fatal(err)
		}

		// IEI 0x78, two-octet length (TLV-E, TS 24.301 table 8.2.3.1).
		want := append([]byte{b[0], b[1], byte(EMMCauseESMFailure), 0x78, 0x00, byte(len(esm))}, esm...)
		if !bytes.Equal(b, want) {
			t.Fatalf("ATTACH REJECT ESM container encoding = % x, want % x", b, want)
		}

		out, err := ParseAttachReject(b)
		if err != nil {
			t.Fatal(err)
		}

		if out.Cause != EMMCauseESMFailure || !bytes.Equal(out.ESMMessageContainer, esm) {
			t.Fatalf("ESM container round-trip: got %+v, want cause #19 with % x", out, esm)
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

// TestNetworkFeatureSupportIWKN26 pins the indicator at octet 4 bit 7 of the EPS
// network feature support IE (TS 24.301 §9.9.3.12A).
func TestNetworkFeatureSupportIWKN26(t *testing.T) {
	withoutOctet4, err := NetworkFeatureSupport{IMSVoPS: true}.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}

	if len(withoutOctet4) != 1 {
		t.Fatalf("element without octet 4 = % x, want one octet", withoutOctet4)
	}

	raw, err := NetworkFeatureSupport{IMSVoPS: true, IWKN26: true}.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}

	if want := []byte{0x01, 0x40}; !bytes.Equal(raw, want) {
		t.Fatalf("element with IWK N26 = % x, want % x", raw, want)
	}

	out, err := ParseNetworkFeatureSupport(raw)
	if err != nil {
		t.Fatal(err)
	}

	if !out.IWKN26 || !out.IMSVoPS || out.Octet4Spare != 0 {
		t.Fatalf("round-trip = %+v, want IMS VoPS and IWK N26 with no other octet-4 bit", out)
	}

	mixed, err := ParseNetworkFeatureSupport([]byte{0x01, 0xC1})
	if err != nil {
		t.Fatal(err)
	}

	if !mixed.IWKN26 || mixed.Octet4Spare != 0x81 {
		t.Fatalf("mixed octet 4 = %+v, want IWK N26 set and 0x81 spare", mixed)
	}

	again, err := mixed.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}

	if want := []byte{0x01, 0xC1}; !bytes.Equal(again, want) {
		t.Fatalf("re-encode = % x, want % x", again, want)
	}
}

// TestUENetworkCapabilityN1Mode pins the N1 mode bit at octet 9 bit 6
// (TS 24.301 §9.9.3.34).
func TestUENetworkCapabilityN1Mode(t *testing.T) {
	for _, tc := range []struct {
		name string
		rest []byte
		want bool
	}{
		{"supported", []byte{0x00, 0x00, 0x20}, true},
		{"not supported", []byte{0x00, 0x00, 0x00}, false},
		{"another octet-9 bit", []byte{0x00, 0x00, 0x10}, false},
		{"element stops before octet 9", []byte{0x00, 0x00}, false},
		{"no feature octets", nil, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := UENetworkCapability{EEA: 0xF0, EIA: 0x70, Rest: tc.rest}
			if got := c.N1Mode(); got != tc.want {
				t.Errorf("N1Mode() = %v, want %v", got, tc.want)
			}
		})
	}
}

// TS 24.301 §9.9.3.12A pins the ePCO indicator at octet 4 bit 4, alongside the
// IWK N26 indicator at bit 7.
func TestNetworkFeatureSupportEPCO(t *testing.T) {
	raw, err := (&NetworkFeatureSupport{IMSVoPS: true, EPCO: true}).MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}

	if len(raw) != 2 {
		t.Fatalf("value is %d octets, want 2", len(raw))
	}

	if raw[1]&(1<<3) == 0 {
		t.Errorf("octet 4 = %#02x, want bit 4 set", raw[1])
	}

	if raw[1]&(1<<6) != 0 {
		t.Errorf("octet 4 = %#02x, want the IWK N26 bit clear", raw[1])
	}

	back, err := ParseNetworkFeatureSupport(raw)
	if err != nil {
		t.Fatal(err)
	}

	if !back.EPCO || back.IWKN26 {
		t.Errorf("round-trip ePCO=%v IWKN26=%v, want true and false", back.EPCO, back.IWKN26)
	}
}

// TS 24.301 §6.5.1.2 lets the UE carry the PDU session identity in either the
// Protocol configuration options IE or the Extended one.
func TestPDNConnectivityRequestCarriesIdentityInExtendedPCO(t *testing.T) {
	epco := nas.ProtocolConfigurationOptions{
		Direction:      nas.PCOMSToNetwork,
		ConfigProtocol: 0,
		Containers:     []nas.PCOContainer{{ID: nas.PCOContainerPDUSessionID, Content: []byte{7}}},
	}

	raw, err := (&PDNConnectivityRequest{
		PTI: 1, RequestType: RequestTypeHandover, PDNType: PDNTypeIPv4,
		ExtendedProtocolConfigurationOptions: &epco,
	}).MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}

	back, err := ParsePDNConnectivityRequest(raw)
	if err != nil {
		t.Fatal(err)
	}

	if back.ExtendedProtocolConfigurationOptions == nil {
		t.Fatal("the extended container did not survive the round trip")
	}

	id, ok := back.ExtendedProtocolConfigurationOptions.PDUSessionID()
	if !ok || id != 7 {
		t.Errorf("PDU session identity = %d (present %v), want 7", id, ok)
	}
}

// TS 24.301 §9.9.3.34 places the ePCO bit at octet 8 bit 8; Rest starts at octet 7.
func TestUENetworkCapabilityEPCO(t *testing.T) {
	if (UENetworkCapability{Rest: []byte{0x00, 0x80}}).EPCO() != true {
		t.Error("EPCO() = false for octet 8 bit 8 set, want true")
	}

	if (UENetworkCapability{Rest: []byte{0x80, 0x00}}).EPCO() {
		t.Error("EPCO() = true for the bit set in octet 7, want false")
	}

	if (UENetworkCapability{Rest: []byte{0x00}}).EPCO() {
		t.Error("EPCO() = true with no octet 8, want false")
	}
}

// §8.3.14.4: the ESM INFORMATION RESPONSE may carry the extended element, which
// has to survive a round trip like the classic one.
func TestESMInformationResponseExtendedPCORoundTrip(t *testing.T) {
	epco := nas.ProtocolConfigurationOptions{
		Direction:  nas.PCOMSToNetwork,
		Containers: []nas.PCOContainer{{ID: nas.PCOContainerPDUSessionID, Content: []byte{4}}},
	}

	apn := APN("internet")

	raw, err := (&ESMInformationResponse{PTI: 2, AccessPointName: &apn, ExtendedProtocolConfigurationOptions: &epco}).MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}

	back, err := ParseESMInformationResponse(raw)
	if err != nil {
		t.Fatal(err)
	}

	if back.ExtendedProtocolConfigurationOptions == nil {
		t.Fatal("the extended element did not survive the round trip")
	}

	id, ok := back.ExtendedProtocolConfigurationOptions.PDUSessionID()
	if !ok || id != 4 {
		t.Errorf("PDU session identity = %d (present %v), want 4", id, ok)
	}
}

// The extended element carries a two-octet length, so a value above 255 octets
// frames correctly where the classic one could not (TS 24.008 §10.5.6.3A).
func TestExtendedPCOFramesAboveOneOctet(t *testing.T) {
	big := nas.ProtocolConfigurationOptions{
		Direction:  nas.PCOMSToNetwork,
		Containers: []nas.PCOContainer{{ID: nas.PCOContainerPDUSessionID, Content: []byte{1}}},
	}

	// A container length is one octet, so several are needed to pass 255 in total.
	for range 3 {
		big.Containers = append(big.Containers, nas.PCOContainer{ID: 0x000C, Content: make([]byte, 200)})
	}

	raw, err := (&PDNConnectivityRequest{
		PTI: 1, RequestType: RequestTypeInitialRequest, PDNType: PDNTypeIPv4,
		ExtendedProtocolConfigurationOptions: &big,
	}).MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}

	back, err := ParsePDNConnectivityRequest(raw)
	if err != nil {
		t.Fatal(err)
	}

	if back.ExtendedProtocolConfigurationOptions == nil {
		t.Fatal("a value above 255 octets did not survive the round trip")
	}

	if got := len(back.ExtendedProtocolConfigurationOptions.Containers); got != 4 {
		t.Errorf("containers = %d, want 4", got)
	}
}
