// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ue

import (
	"encoding/hex"
	"testing"

	"github.com/ellanetworks/core/nas"
	"github.com/ellanetworks/core/nas/fgs"
	"github.com/free5gc/openapi/models"
)

// goldenUESecurity returns a UESecurity with fully deterministic identity and
// key material so the builder outputs are reproducible. NEA0 (null cipher) keeps
// any NAS-message-container ciphering a byte-for-byte passthrough.
func goldenUESecurity() *UESecurity {
	sec := &UESecurity{
		Supi:         "imsi-001010000000001",
		Msin:         "0000000001",
		mcc:          "001",
		mnc:          "01",
		IntegrityAlg: AlgIntegrity128NIA2,
		CipheringAlg: AlgCiphering128NEA0,
		KnasEnc:      [16]byte{16, 15, 14, 13, 12, 11, 10, 9, 8, 7, 6, 5, 4, 3, 2, 1},
		KnasInt:      [16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16},
		ULCount:      nas.MakeCount(0, 3),
	}
	sec.NgKsi.Ksi = 1
	sec.UeSecurityCapability = fgs.UESecurityCapability{EA: 0xe0, IA: 0xe0} // NEA0/1/2, NIA0/1/2
	sec.Suci = fgs.SUCIIdentity(fgs.SUCI{
		PLMN: nas.PLMN{MCC: "001", MNC: "01"}, RoutingIndicator: "0000",
		SchemeOutput: []byte{0x00, 0x00, 0x00, 0x00, 0x10},
	})
	guti := fgs.GUTIIdentity(fgs.GUTI{
		PLMN: nas.PLMN{MCC: "001", MNC: "01"}, AMFRegionID: 1, AMFSetID: 0x008, AMFPointer: 0x03,
		TMSI: [4]byte{0x04, 0x05, 0x06, 0x07},
	})
	sec.Guti = &guti

	return sec
}

// goldenBuilders is the fixed set of builder invocations the golden test locks.
func goldenBuilders(t *testing.T) map[string][]byte {
	t.Helper()

	sec := goldenUESecurity()

	pduStatus := &[16]bool{true, false, true} // sessions 0 and 2 active

	idResp, err := BuildIdentityResponse(&IdentityResponseOpts{Suci: sec.Suci})
	if err != nil {
		t.Fatalf("BuildIdentityResponse: %v", err)
	}

	dereg, err := BuildDeregistrationRequest(&DeregistrationRequestOpts{Guti: sec.Guti, Ksi: 1})
	if err != nil {
		t.Fatalf("BuildDeregistrationRequest: %v", err)
	}

	regNoContainer, err := BuildRegistrationRequest(&RegistrationRequestOpts{
		RegistrationType:  uint8(fgs.RegistrationTypeInitial),
		IncludeCapability: true,
		UESecurity:        sec,
	})
	if err != nil {
		t.Fatalf("BuildRegistrationRequest (no container): %v", err)
	}

	regContainer, err := BuildRegistrationRequest(&RegistrationRequestOpts{
		RegistrationType:  uint8(fgs.RegistrationTypeInitial),
		IncludeCapability: true,
		UESecurity:        sec,
		PDUSessionStatus:  pduStatus,
	})
	if err != nil {
		t.Fatalf("BuildRegistrationRequest (container): %v", err)
	}

	svc, err := BuildServiceRequest(&ServiceRequestOpts{
		ServiceType:      uint8(fgs.ServiceTypeData),
		AMFSetID:         0x0102,
		AMFPointer:       0x03,
		TMSI5G:           [4]uint8{0x0a, 0x0b, 0x0c, 0x0d},
		PDUSessionStatus: pduStatus,
		UESecurity:       sec,
	})
	if err != nil {
		t.Fatalf("BuildServiceRequest: %v", err)
	}

	smc, err := BuildSecurityModeComplete(&SecurityModeCompleteOpts{
		UESecurity: sec,
		IMEISV:     "1234567890123456",
	})
	if err != nil {
		t.Fatalf("BuildSecurityModeComplete: %v", err)
	}

	ulnas, err := BuildUplinkNasTransport(&UplinkNasTransportOpts{
		PDUSessionID:     1,
		PayloadContainer: []byte{0x2e, 0x01, 0x01, 0xc1, 0xff, 0xff},
		DNN:              "internet",
		SNSSAI:           models.Snssai{Sst: 1, Sd: "010203"},
	})
	if err != nil {
		t.Fatalf("BuildUplinkNasTransport: %v", err)
	}

	pduEst, err := BuildPduSessionEstablishmentRequest(&PduSessionEstablishmentRequestOpts{
		PDUSessionID:   1,
		PDUSessionType: fgs.PDUSessionTypeIPv4,
	})
	if err != nil {
		t.Fatalf("BuildPduSessionEstablishmentRequest: %v", err)
	}

	ulnasLPP, err := BuildUplinkNasTransportLPP([]byte{0x01, 0x02, 0x03})
	if err != nil {
		t.Fatalf("BuildUplinkNasTransportLPP: %v", err)
	}

	return map[string][]byte{
		"identity_response":       idResp,
		"deregistration_request":  dereg,
		"registration_request":    regNoContainer,
		"registration_container":  regContainer,
		"service_request":         svc,
		"security_mode_complete":  smc,
		"ul_nas_transport":        ulnas,
		"ul_nas_transport_lpp":    ulnasLPP,
		"pdu_session_est_request": pduEst,
	}
}

var builderGolden = map[string]string{
	"identity_response":       "7e005c000d0100f110000000000000000010",
	"deregistration_request":  "7e004519000bf200f11001020304050607",
	"registration_request":    "7e004119000bf200f1100102030405060710012f2e02e0e0",
	"registration_container":  "7e004119000bf200f1100102030405060710012f2e02e0e07100207e004119000bf200f1100102030405060710012f2e02e0e04002050050020500",
	"service_request":         "7e004c110007f440830a0b0c0d40020500500205007100157e004c110007f440830a0b0c0d4002050050020500",
	"security_mode_complete":  "7e005e7700091532547698103254f67100187e004119000bf200f1100102030405060710012f2e02e0e0",
	"ul_nas_transport":        "7e00670100062e0101c1ffff120181220401010203250908696e7465726e6574",
	"ul_nas_transport_lpp":    "7e0067030003010203",
	"pdu_session_est_request": "2e0101c1ffff917b000a80000a00000d00000300",
}

func TestBuildersGolden(t *testing.T) {
	got := goldenBuilders(t)

	for name, wantHex := range builderGolden {
		b, ok := got[name]
		if !ok {
			t.Errorf("%s: builder not exercised", name)
			continue
		}

		if h := hex.EncodeToString(b); h != wantHex {
			t.Errorf("%s mismatch:\n got  %s\n want %s", name, h, wantHex)
		}
	}
}
