// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas_test

import (
	"bytes"
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/ellanetworks/core/internal/decoder/nas"
	"github.com/ellanetworks/core/nas/fgs"
	"github.com/ellanetworks/core/nas/nastest"
)

// Regenerate with: go test ./internal/decoder/nas/ -run TestDecoderGolden -update
var updateGolden = flag.Bool("update", false, "regenerate decoder golden JSON fixtures")

func mustEnc(t *testing.T, fn func() ([]byte, error)) []byte {
	t.Helper()

	b, err := fn()
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	return b
}

// goldenCorpus builds the raw NAS PDUs the golden test renders: the security-header
// variants plus the plain 5GMM message types.
func goldenCorpus(t *testing.T) map[string][]byte {
	t.Helper()

	// A known-good plain REGISTRATION REQUEST wire capture.
	regReq, err := decodeB64("fgBBeQANAQDxEAAAAABEdGhXJS4E8PDw8A==")
	if err != nil {
		t.Fatalf("decode registration request: %v", err)
	}

	corpus := map[string][]byte{
		"registration_request":   regReq,
		"secprot_integrity_only": append([]byte{0x7e, 0x01, 0xde, 0xad, 0xbe, 0xef, 0x05}, regReq...),
		"secprot_ciphered_nea0":  append([]byte{0x7e, 0x02, 0xde, 0xad, 0xbe, 0xef, 0x05}, regReq...),
		"too_short":              {0x7e, 0x00},
		"too_short_1byte":        {0x7e}, // guarded; would previously panic reading raw[1]
		"empty":                  {},
		"unsupported_sht":        append([]byte{0x7e, 0x0a, 0x00, 0x00, 0x00, 0x00, 0x00}, regReq...),
		"plain_unknown_gmm_type": {0x7e, 0x00, 0x01}, // plain 5GMM, unrecognized message type → header only (matches 4G)
	}

	corpus["identity_request"] = mustEnc(t, (&fgs.IdentityRequest{IdentityType: fgs.IdentitySUCI}).MarshalBinary)
	corpus["authentication_reject"] = mustEnc(t, (&fgs.AuthenticationReject{}).MarshalBinary)
	corpus["registration_reject"] = mustEnc(t, (&fgs.RegistrationReject{Cause: 0x0b}).MarshalBinary) // PLMN not allowed
	corpus["service_reject"] = mustEnc(t, (&fgs.ServiceReject{Cause: 0x09}).MarshalBinary)           // UE identity cannot be derived
	corpus["registration_complete"] = mustEnc(t, (&fgs.RegistrationComplete{}).MarshalBinary)

	// Authentication messages carrying their optional IEs, built with the fgs
	// adversarial builder (RAND is TV-3/16, AUTN is TLV/16, RES* TLV/16, AUTS TLV/14,
	// EAP TLV-E — TS 24.501 §8.2.1-8.2.4).
	rand16 := bytes.Repeat([]byte{0xab}, 16)
	autn16 := bytes.Repeat([]byte{0xcd}, 16)
	res16 := bytes.Repeat([]byte{0xef}, 16)
	auts14 := bytes.Repeat([]byte{0x12}, 14)
	eap := []byte{0x02, 0x00, 0x00, 0x05, 0x01}

	corpus["authentication_request_full"] = nastest.BuildGMM(fgs.MsgAuthenticationRequest).
		U8(0x10).LV([]byte{0x00, 0x00}).TV(0x21, rand16).TLV(0x20, autn16).TLVE(0x78, eap).Bytes()
	corpus["authentication_response_full"] = nastest.BuildGMM(fgs.MsgAuthenticationResponse).
		TLV(0x2d, res16).TLVE(0x78, eap).Bytes()
	corpus["authentication_failure_full"] = nastest.BuildGMM(fgs.MsgAuthenticationFailure).
		U8(21).TLV(0x30, auts14).Bytes()
	corpus["authentication_reject_eap"] = nastest.BuildGMM(fgs.MsgAuthenticationReject).
		TLVE(0x78, eap).Bytes()

	// Reject / complete messages carrying their optional IEs.
	corpus["registration_reject_full"] = nastest.BuildGMM(fgs.MsgRegistrationReject).
		U8(11). // cause: PLMN not allowed
		TLV(0x5f, []byte{0x0a}).TLV(0x16, []byte{0x0b}).TLVE(0x78, eap).Bytes()
	corpus["registration_complete_full"] = nastest.BuildGMM(fgs.MsgRegistrationComplete).
		TLVE(0x73, bytes.Repeat([]byte{0x11}, 17)).Bytes() // SOR transparent container (min len 17)
	corpus["service_reject_full"] = nastest.BuildGMM(fgs.MsgServiceReject).
		U8(9). // cause: UE identity cannot be derived
		TLV(0x50, []byte{0x02, 0x00}).TLV(0x5f, []byte{0x0a}).TLVE(0x78, eap).Bytes()

	// IDENTITY RESPONSE with each 5GS mobile-identity type, exercising every
	// buildMobileIdentity branch (SUCI/GUTI/IMEI/5G-S-TMSI/IMEISV).
	identities := map[string][]byte{
		"identity_response_suci":   {0x01, 0x00, 0xf1, 0x10, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x10},
		"identity_response_guti":   {0xf2, 0x00, 0xf1, 0x10, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07},
		"identity_response_imei":   {0x4b, 0x09, 0x51, 0x24, 0x30, 0x32, 0x57, 0x81},
		"identity_response_stmsi":  {0xf4, 0x00, 0x01, 0x02, 0x03, 0x04, 0x05},
		"identity_response_imeisv": {0x15, 0x32, 0x54, 0x76, 0x98, 0x10, 0x32, 0x54, 0xf6},
	}
	for name, mi := range identities {
		corpus[name] = nastest.BuildGMM(fgs.MsgIdentityResponse).LVE(mi).Bytes()
	}

	// REGISTRATION REQUEST with a spread of optional IEs: rendered values
	// (UE security capability, NAS message container) and presence-only IEs
	// (5GMM capability, requested NSSAI, PDU session status, MICO type-1).
	corpus["registration_request_full"] = nastest.BuildGMM(fgs.MsgRegistrationRequest).
		U8(0x79).
		LVE([]byte{0xf2, 0x00, 0xf1, 0x10, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07}). // 5G-GUTI
		TLV(0x10, []byte{0x01}).                                                       // 5GMM capability
		TLV(0x2e, []byte{0xe0, 0xe0}).                                                 // UE security capability (NEA0-2 / NIA0-2)
		TLV(0x2f, []byte{0x01, 0x01}).                                                 // requested NSSAI
		TLV(0x50, []byte{0x02, 0x00}).                                                 // PDU session status
		TLVE(0x71, []byte{0x2e, 0x01}).                                                // NAS message container
		TV1(0xb0, 0x01).                                                               // MICO indication (type 1)
		Bytes()

	// SERVICE REQUEST: service-type/ngKSI octet, 5G-S-TMSI, and PSI-bitmap optional
	// IEs (uplink data / PDU session / allowed PDU session status).
	corpus["service_request_full"] = nastest.BuildGMM(fgs.MsgServiceRequest).
		U8(0x1a).                                              // service type Data, TSC Mapped, ngKSI 2
		LVE([]byte{0xf4, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06}). // 5G-S-TMSI
		TLV(0x40, []byte{0x02, 0x00}).                         // uplink data status
		TLV(0x50, []byte{0x04, 0x00}).                         // PDU session status
		TLV(0x25, []byte{0x08, 0x00}).                         // allowed PDU session status
		TLVE(0x71, []byte{0x2e, 0x01}).                        // NAS message container
		Bytes()

	// SERVICE ACCEPT: PDU session status + reactivation result (PSI bitmaps),
	// reactivation-result error cause (id/cause pairs), and EAP message.
	corpus["service_accept_full"] = nastest.BuildGMM(fgs.MsgServiceAccept).
		TLV(0x50, []byte{0x02, 0x00}).  // PDU session status
		TLV(0x26, []byte{0x04, 0x00}).  // PDU session reactivation result
		TLVE(0x72, []byte{0x01, 0x0b}). // error cause: PDU session 1, cause PLMN not allowed
		TLVE(0x78, eap).                // EAP message
		Bytes()

	// SECURITY MODE COMMAND: selected algorithms, replayed capabilities, and the
	// optional IMEISV request / EPS algorithms / additional 5G security info / ABBA.
	corpus["security_mode_command_full"] = nastest.BuildGMM(fgs.MsgSecurityModeCommand).
		U8(0x21).                                  // ciphering NEA2, integrity NIA1
		U8(0x07).                                  // ngKSI octet
		LV([]byte{0xe0, 0xe0}).                    // replayed UE security capabilities
		TV1(0xe0, 0x01).                           // IMEISV request (type 1)
		TV(0x57, []byte{0x01}).                    // selected EPS NAS security algorithms
		TLV(0x36, []byte{0x03}).                   // additional 5G security information (RINMR+HDP)
		TLVE(0x78, eap).                           // EAP message
		TLV(0x38, []byte{0x00, 0x00}).             // ABBA
		TLV(0x19, []byte{0x00, 0x00, 0x00, 0x00}). // replayed S1 UE security capabilities
		Bytes()

	// SECURITY MODE COMPLETE: IMEISV (9-octet mobile identity) + NAS message container.
	corpus["security_mode_complete_full"] = nastest.BuildGMM(fgs.MsgSecurityModeComplete).
		TLVE(0x77, []byte{0x15, 0x32, 0x54, 0x76, 0x98, 0x10, 0x32, 0x54, 0xf6}). // IMEISV 1234567890123456
		TLVE(0x71, []byte{0x7e, 0x00, 0x41}).                                     // NAS message container
		Bytes()

	// REGISTRATION ACCEPT with a spread of optional IEs: decoded ones (5G-GUTI, TAI
	// list, allowed NSSAI, network feature support) and presence-only ones (rejected
	// NSSAI, PDU session status, MICO type-1, T3512, EAP message).
	guti := []byte{0xf2, 0x00, 0xf1, 0x10, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07}
	taiList := []byte{0x00, 0x00, 0xf1, 0x10, 0x00, 0x00, 0x01} // type 0, one TAI
	corpus["registration_accept_full"] = nastest.BuildGMM(fgs.MsgRegistrationAccept).
		LV([]byte{0x01}).              // 5GS registration result: 3GPP only
		TLVE(0x77, guti).              // 5G-GUTI
		TLV(0x54, taiList).            // TAI list
		TLV(0x15, []byte{0x01, 0x01}). // allowed NSSAI (one SST)
		TLV(0x21, []byte{0x01, 0x00}). // network feature support (IMS VoPS 3GPP)
		TLV(0x50, []byte{0x02, 0x00}). // PDU session status (presence-only)
		TV1(0xb0, 0x01).               // MICO indication (type 1)
		TLV(0x5e, []byte{0x05}).       // T3512 value (presence-only)
		TLVE(0x78, eap).               // EAP message (presence-only)
		Bytes()

	// UL NAS TRANSPORT with routing IEs (PDU session id, old PDU session id, request
	// type, S-NSSAI, DNN) around an N1 SM payload container.
	ulGSM := nastest.BuildGSM(1, 0, fgs.MsgPDUSessionEstablishmentRequest).U8(0x11).U8(0xff).Bytes()
	corpus["ul_nas_transport_full"] = nastest.BuildGMM(fgs.MsgULNASTransport).
		U8(0x01).                                                                // payload container type: N1 SM information
		LVE(ulGSM).                                                              // payload container
		TV(0x12, []byte{0x05}).                                                  // PDU session ID
		TV(0x59, []byte{0x06}).                                                  // old PDU session ID
		TV1(0x80, 0x01).                                                         // request type (type 1): initial request
		TLV(0x22, []byte{0x01, 0xaa, 0xbb, 0xcc}).                               // S-NSSAI (SST + SD)
		TLV(0x25, []byte{0x08, 0x69, 0x6e, 0x74, 0x65, 0x72, 0x6e, 0x65, 0x74}). // DNN "internet"
		TLV(0x24, []byte{0x01}).                                                 // additional information
		Bytes()

	// PDU SESSION ESTABLISHMENT ACCEPT carried in a DL NAS TRANSPORT payload container.
	acc := &fgs.PDUSessionEstablishmentAccept{
		PDUSessionID:   1,
		PTI:            0,
		PDUSessionType: fgs.PDUSessionTypeIPv4,
		SSCMode:        1,
		QoSRules:       fgs.QoSRules{fgs.DefaultQoSRule(1, 9)},
		SessionAMBR:    fgs.SessionAMBR{DownlinkUnit: 0x06, Downlink: 1000, UplinkUnit: 0x06, Uplink: 1000},
		PDUAddress:     &fgs.PDUAddress{SessionType: fgs.PDUSessionTypeIPv4, IPv4: [4]byte{10, 45, 0, 2}},
		SNSSAI:         &fgs.SNSSAI{SST: 1, SD: &[3]byte{0xaa, 0xbb, 0xcc}},
		DNN:            new(fgs.DNN("internet")),
	}

	accBytes, err := acc.MarshalBinary()
	if err != nil {
		t.Fatalf("marshal establishment accept: %v", err)
	}

	corpus["pdu_session_establishment_accept_full"] = nastest.BuildGMM(fgs.MsgDLNASTransport).
		U8(0x01).LVE(accBytes).TV(0x12, []byte{0x01}).Bytes()

	// DL NAS TRANSPORT carrying an N1 SM payload container plus routing/diagnostic IEs.
	dlGSM := nastest.BuildGSM(1, 0, fgs.MsgPDUSessionEstablishmentRequest).U8(0x11).U8(0xff).Bytes()
	corpus["dl_nas_transport_full"] = nastest.BuildGMM(fgs.MsgDLNASTransport).
		U8(0x01).                // payload container type: N1 SM information
		LVE(dlGSM).              // payload container
		TV(0x12, []byte{0x05}).  // PDU session ID
		TLV(0x37, []byte{0x21}). // back-off timer value
		TV(0x58, []byte{0x16}).  // 5GMM cause
		Bytes()

	// PDU SESSION ESTABLISHMENT REQUEST carried in a UL NAS TRANSPORT payload container
	// (a plain GSM message is only ever nested inside a transport, TS 24.501 §8.3).
	pco := []byte{0x80, 0x00, 0x01, 0x00, 0x00, 0x0e, 0x00} // config octet + PCSCFIPv6 + MSISDN request (both UL)
	gsmRequest := nastest.BuildGSM(1, 0, fgs.MsgPDUSessionEstablishmentRequest).
		U8(0x11).U8(0xff).            // integrity protection max data rate (uplink, downlink)
		TV1(0x90, 0x01).              // PDU session type IPv4 (type 1)
		TV1(0xa0, 0x01).              // SSC mode 1 (type 1)
		TLV(0x28, []byte{0x03}).      // 5GSM capability (RqoS + MH6PDU)
		TV(0x55, []byte{0x00, 0x10}). // maximum number of supported packet filters (TV3, len 2)
		TV1(0xb0, 0x01).              // always-on PDU session requested (type 1)
		TLV(0x39, []byte{0x00}).      // SM PDU DN request container
		TLVE(0x7b, pco).              // extended protocol configuration options
		Bytes()
	corpus["pdu_session_establishment_request_full"] = nastest.BuildGMM(fgs.MsgULNASTransport).
		U8(0x01).               // payload container type: N1 SM information
		LVE(gsmRequest).        // payload container carrying the GSM message
		TV(0x12, []byte{0x01}). // PDU session ID
		Bytes()

	return corpus
}

func TestDecoderGolden(t *testing.T) {
	dir := filepath.Join("testdata", "golden")

	if *updateGolden {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("mkdir golden dir: %v", err)
		}
	}

	corpus := goldenCorpus(t)

	names := make([]string, 0, len(corpus))
	for name := range corpus {
		names = append(names, name)
	}

	sort.Strings(names)

	for _, name := range names {
		t.Run(name, func(t *testing.T) {
			got, err := json.MarshalIndent(nas.DecodeNASMessage(corpus[name]), "", "  ")
			if err != nil {
				t.Fatalf("marshal decoded JSON: %v", err)
			}

			got = append(got, '\n')
			path := filepath.Join(dir, name+".json")

			if *updateGolden {
				if err := os.WriteFile(path, got, 0o644); err != nil {
					t.Fatalf("write golden %q: %v", path, err)
				}

				return
			}

			want, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read golden %q (run with -update to create): %v", path, err)
			}

			if !bytes.Equal(got, want) {
				t.Errorf("decoder JSON changed for %q.\n--- got ---\n%s\n--- want ---\n%s", name, got, want)
			}
		})
	}
}
