// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package eps

import (
	"encoding/hex"
	"testing"

	"github.com/ellanetworks/core/nas"
	"github.com/ellanetworks/core/nas/eps"
)

func decodeHex(t *testing.T, h string) *NASMessage {
	t.Helper()

	raw, err := hex.DecodeString(h)
	if err != nil {
		t.Fatal(err)
	}

	return DecodeEPSNASMessage(raw)
}

// TestDecodeAttachRequest uses a NAS-PDU captured from a live deployment: an
// integrity-protected combined ATTACH REQUEST carrying a GUTI and a PDN
// Connectivity Request in its ESM container.
func TestDecodeAttachRequest(t *testing.T) {
	msg := decodeHex(t, "17d74e9de8050741020bf699f9100001010000000207f070000018008000330236d011272d8080211001000010810600000000830600000000000d00000a00000500001000001100001a01010023000024005299f91000015c0a009011034f1886f15d0106e0c1")

	if msg.SecurityHeader.SecurityHeaderType.Value != int64(eps.SHTIntegrityProtected) {
		t.Fatalf("SHT = %q", msg.SecurityHeader.SecurityHeaderType.Label)
	}

	if msg.EMMMessage == nil || msg.EMMMessage.EMMHeader.MessageType.Value != int64(eps.MsgAttachRequest) {
		t.Fatalf("EMM = %+v", msg.EMMMessage)
	}

	ar := msg.EMMMessage.AttachRequest
	if ar == nil {
		t.Fatal("attach request not decoded")
	}

	if ar.AttachType.Value != int64(eps.AttachTypeCombined) {
		t.Fatalf("attach type = %q", ar.AttachType.Label)
	}

	if ar.MobileIdentity.Type != "guti" || ar.MobileIdentity.GUTI == nil ||
		ar.MobileIdentity.GUTI.MTMSI != 2 || ar.MobileIdentity.GUTI.MCC != "999" {
		t.Fatalf("mobile identity = %+v", ar.MobileIdentity)
	}

	if ar.ESMContainer == nil || ar.ESMContainer.ESMHeader.MessageType.Value != int64(eps.MsgPDNConnectivityRequest) {
		t.Fatalf("ESM container = %+v", ar.ESMContainer)
	}
}

// TestDecodeTrackingAreaUpdateRequest uses a live NAS-PDU: an integrity-protected
// TRACKING AREA UPDATE REQUEST.
func TestDecodeTrackingAreaUpdateRequest(t *testing.T) {
	msg := decodeHex(t, "17659a6d010d0748000bf699f910000101000000015807f07000001800805299f9100001570220005d0106e0c1")

	if msg.EMMMessage == nil || msg.EMMMessage.EMMHeader.MessageType.Value != int64(eps.MsgTrackingAreaUpdateRequest) {
		t.Fatalf("EMM = %+v", msg.EMMMessage)
	}

	if msg.EMMMessage.TrackingAreaUpdateRequest == nil {
		t.Fatal("TAU request not decoded")
	}
}

// TestDecodeEncrypted uses a live ciphered NAS-PDU (SHT=2): the decoder reports
// the security header and the encrypted flag, without an inner message.
func TestDecodeEncrypted(t *testing.T) {
	msg := decodeHex(t, "2774a88ff701128f7ddc4907f3")

	if !msg.Encrypted {
		t.Fatal("expected encrypted")
	}

	if msg.EMMMessage != nil {
		t.Fatal("must not decode an inner message when ciphered")
	}

	if msg.SecurityHeader.SecurityHeaderType.Value != int64(eps.SHTIntegrityProtectedCiphered) {
		t.Fatalf("SHT = %q", msg.SecurityHeader.SecurityHeaderType.Label)
	}
}

// TestDecodePlainIdentityRequest decodes a plain (unprotected) message built with
// the codec, exercising the non-wrapped path.
// TestDecodeEEA0NullCipher: a ciphered (SHT=2) wrapper around a plaintext body
// (EEA0 null cipher) decodes to its inner message, symmetric with the 5G decoder.
func TestDecodeEEA0NullCipher(t *testing.T) {
	plain, err := (&eps.IdentityRequest{IdentityType: 1}).MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}

	wire := append([]byte{uint8(eps.SHTIntegrityProtectedCiphered)<<4 | uint8(eps.PDEMM), 0xAA, 0xBB, 0xCC, 0xDD, 0x00}, plain...)

	msg := DecodeEPSNASMessage(wire)
	if msg.Encrypted {
		t.Fatal("EEA0 null-cipher payload should decode, not be marked encrypted")
	}

	if msg.EMMMessage == nil || msg.EMMMessage.IdentityRequest == nil {
		t.Fatalf("EEA0 inner = %+v", msg.EMMMessage)
	}
}

func TestDecodePlainIdentityRequest(t *testing.T) {
	b, err := (&eps.IdentityRequest{IdentityType: 1}).MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}

	msg := DecodeEPSNASMessage(b)

	if msg.SecurityHeader.SecurityHeaderType.Value != int64(eps.SHTPlain) {
		t.Fatalf("SHT = %q", msg.SecurityHeader.SecurityHeaderType.Label)
	}

	if msg.EMMMessage == nil || msg.EMMMessage.IdentityRequest == nil ||
		msg.EMMMessage.IdentityRequest.IdentityType != 1 {
		t.Fatalf("identity request = %+v", msg.EMMMessage)
	}
}

// The following NAS-PDUs are captured from a live deployment (the 999/01 test
// PLMN), one per message type the MME exchanges in the clear.

func TestDecodeAuthenticationRequest(t *testing.T) {
	msg := decodeHex(t, "075200ea3d8ec68864b3dce98a956efffb8adf103787800bf66780007f99d3a08c910b95")

	if msg.EMMMessage == nil || msg.EMMMessage.AuthenticationRequest == nil ||
		len(msg.EMMMessage.AuthenticationRequest.RAND) != 32 {
		t.Fatalf("authentication request = %+v", msg.EMMMessage)
	}
}

func TestDecodeAuthenticationResponse(t *testing.T) {
	msg := decodeHex(t, "075308333f7d3146a2c189")

	if msg.EMMMessage == nil || msg.EMMMessage.AuthenticationResponse == nil ||
		msg.EMMMessage.AuthenticationResponse.RES == "" {
		t.Fatalf("authentication response = %+v", msg.EMMMessage)
	}
}

func TestDecodeSecurityModeCommand(t *testing.T) {
	msg := decodeHex(t, "37d71eeb1400075d220004f0700000c1")

	if msg.SecurityHeader.SecurityHeaderType.Value != int64(eps.SHTIntegrityProtectedNewContext) {
		t.Fatalf("SHT = %q", msg.SecurityHeader.SecurityHeaderType.Label)
	}

	smc := msg.EMMMessage.SecurityModeCommand
	if smc == nil || smc.CipheringAlgorithm.Value != int64(nas.CipheringAES) || smc.IntegrityAlgorithm.Value != int64(nas.IntegrityAES) {
		t.Fatalf("security mode command = %+v", smc)
	}
}

func TestDecodeServiceRequest(t *testing.T) {
	msg := decodeHex(t, "c7038a84")

	if msg.EMMMessage == nil || msg.EMMMessage.ServiceRequest == nil {
		t.Fatalf("service request = %+v", msg.EMMMessage)
	}
}

func TestDecodeServiceReject(t *testing.T) {
	msg := decodeHex(t, "074e09")

	if msg.EMMMessage == nil || msg.EMMMessage.EMMHeader.MessageType.Value != int64(eps.MsgServiceReject) {
		t.Fatalf("EMM = %+v", msg.EMMMessage)
	}
}

func TestDecodePlainAttachRequest(t *testing.T) {
	msg := decodeHex(t, "07417108999910480000464407f070000018008000330265d011272d8080211001000010810600000000830600000000000d00000a00000500001000001100001a01010023000024005c0a005d0106c1")

	ar := msg.EMMMessage.AttachRequest
	if ar == nil || ar.MobileIdentity.Type != "imsi" {
		t.Fatalf("attach request = %+v", ar)
	}

	if ar.ESMContainer == nil || ar.ESMContainer.ESMHeader.MessageType.Value != int64(eps.MsgPDNConnectivityRequest) {
		t.Fatalf("ESM container = %+v", ar.ESMContainer)
	}
}

// The PDN address is only ever sent ciphered on the wire (in the Activate
// Default Bearer of the Attach Accept), so it is exercised with codec-built
// values rather than a capture.
func TestPDNAddressIPv4(t *testing.T) {
	a := pdnAddress(eps.PDNAddress{PDNType: 1, IPv4: [4]byte{10, 45, 0, 7}})
	if a == nil || a.Type.Value != int64(eps.PDNTypeIPv4) || a.IPv4 != "10.45.0.7" || a.IPv6InterfaceID != "" {
		t.Fatalf("pdn address = %+v", a)
	}
}

func TestPDNAddressIPv4v6(t *testing.T) {
	a := pdnAddress(eps.PDNAddress{
		PDNType: 3,
		IPv4:    [4]byte{10, 45, 0, 7},
		IPv6IID: [8]byte{0x00, 0x11, 0x22, 0x33, 0x44, 0x55, 0x66, 0x77},
	})
	if a == nil || a.Type.Value != int64(eps.PDNTypeIPv4v6) || a.IPv4 != "10.45.0.7" || a.IPv6InterfaceID != "0011:2233:4455:6677" {
		t.Fatalf("pdn address = %+v", a)
	}
}

func TestDecodeInvalid(t *testing.T) {
	if msg := DecodeEPSNASMessage([]byte{0x07}); msg.Error == "" {
		t.Fatal("expected an error for a too-short message")
	}
}

func TestDecodeTrackingAreaUpdateReject(t *testing.T) {
	msg := decodeHex(t, "074b09")

	rej := msg.EMMMessage.TrackingAreaUpdateReject
	if rej == nil {
		t.Fatalf("EMM = %+v", msg.EMMMessage)
	}

	if rej.EMMCause.Value != int64(eps.EMMCauseUEIdentityCannotBeDerived) {
		t.Errorf("EMM cause = %d %q", rej.EMMCause.Value, rej.EMMCause.Label)
	}

	if rej.EMMCause.Label == "" {
		t.Errorf("EMM cause carries no label")
	}
}

func TestDecodeEMMInformation(t *testing.T) {
	msg := decodeHex(t, "2701fb5493030761430d8545363b0c7296e9f7b77c3d0745058445363b0c")

	info := msg.EMMMessage.EMMInformation
	if info == nil {
		t.Fatalf("EMM = %+v", msg.EMMMessage)
	}

	if info.FullNameForNetwork == nil || info.FullNameForNetwork.Name != "Ella Networks" {
		t.Errorf("full network name = %+v", info.FullNameForNetwork)
	}

	if info.ShortNameForNetwork == nil || info.ShortNameForNetwork.Name != "Ella" {
		t.Errorf("short network name = %+v", info.ShortNameForNetwork)
	}
}

func TestDecodeESMInformationResponse(t *testing.T) {
	msg := decodeHex(t, "27378dcac30102bbda280908696e7465726e6574")

	resp := msg.ESMMessage.ESMInformationResponse
	if resp == nil {
		t.Fatalf("ESM = %+v", msg.ESMMessage)
	}

	if resp.AccessPointName == nil || *resp.AccessPointName != "internet" {
		t.Errorf("APN = %v", resp.AccessPointName)
	}
}

func TestDecodePDNDisconnectRequest(t *testing.T) {
	msg := decodeHex(t, "274ce5c4670502bdd206")

	req := msg.ESMMessage.PDNDisconnectRequest
	if req == nil {
		t.Fatalf("ESM = %+v", msg.ESMMessage)
	}

	if req.LinkedEPSBearerIdentity != 6 {
		t.Errorf("linked EPS bearer identity = %d, want 6", req.LinkedEPSBearerIdentity)
	}
}

func TestDecodeAttachCompleteCarriesESMContainer(t *testing.T) {
	msg := decodeHex(t, "2735e4e44602074300035200c2")

	complete := msg.EMMMessage.AttachComplete
	if complete == nil || complete.ESMContainer == nil {
		t.Fatalf("EMM = %+v", msg.EMMMessage)
	}

	if complete.ESMContainer.ESMHeader.MessageType.Value != int64(eps.MsgActivateDefaultEPSBearerContextAccept) {
		t.Errorf("ESM container = %+v", complete.ESMContainer.ESMHeader)
	}

	if complete.ESMContainer.ESMHeader.EPSBearerIdentity != 5 {
		t.Errorf("EPS bearer identity = %d, want 5", complete.ESMContainer.ESMHeader.EPSBearerIdentity)
	}
}

func TestDecodeEMMRejects(t *testing.T) {
	for _, c := range []struct {
		name string
		msg  interface{ MarshalBinary() ([]byte, error) }
		want func(*testing.T, *EMMMessage)
	}{
		{
			name: "AttachReject",
			msg:  &eps.AttachReject{Cause: eps.EMMCauseIllegalUE},
			want: func(t *testing.T, m *EMMMessage) {
				if m.AttachReject == nil || m.AttachReject.EMMCause.Value != int64(eps.EMMCauseIllegalUE) {
					t.Errorf("attach reject = %+v", m.AttachReject)
				}
			},
		},
		{
			name: "AuthenticationFailure",
			msg:  &eps.AuthenticationFailure{Cause: eps.EMMCauseMACFailure, AUTS: []byte{1, 2, 3}},
			want: func(t *testing.T, m *EMMMessage) {
				if m.AuthenticationFailure == nil || m.AuthenticationFailure.AUTS != "010203" {
					t.Errorf("authentication failure = %+v", m.AuthenticationFailure)
				}
			},
		},
		{
			name: "ServiceReject",
			msg:  &eps.ServiceReject{Cause: eps.EMMCauseUEIdentityCannotBeDerived},
			want: func(t *testing.T, m *EMMMessage) {
				if m.ServiceReject == nil || m.ServiceReject.EMMCause.Label == "" {
					t.Errorf("service reject = %+v", m.ServiceReject)
				}
			},
		},
		{
			name: "EMMStatus",
			msg:  &eps.EMMStatus{Cause: eps.EMMCauseIllegalUE},
			want: func(t *testing.T, m *EMMMessage) {
				if m.EMMStatus == nil {
					t.Errorf("EMM status = %+v", m.EMMStatus)
				}
			},
		},
		{
			// The network variant shares its message type with the UE one and only
			// parses in the downlink direction (TS 24.301 table 9.8.1).
			name: "DetachRequestNetwork",
			msg:  &eps.DetachRequestNetwork{TypeOfDetach: eps.DetachTypeReattachRequired},
			want: func(t *testing.T, m *EMMMessage) {
				if m.DetachRequestNetwork == nil {
					t.Fatalf("detach request (network) not decoded: %+v", m)
				}

				if m.DetachRequestNetwork.DetachType.Label == "" {
					t.Errorf("detach type carries no label")
				}
			},
		},
	} {
		t.Run(c.name, func(t *testing.T) {
			b, err := c.msg.MarshalBinary()
			if err != nil {
				t.Fatal(err)
			}

			msg := DecodeEPSNASMessage(b)
			if msg.EMMMessage == nil {
				t.Fatalf("no EMM message decoded")
			}

			c.want(t, msg.EMMMessage)
		})
	}
}

// Every ESM message here is network- or UE-originated with the same message
// type space, so each is decoded end to end rather than trusted to the dispatch
// table alone.
func TestDecodeESMMessages(t *testing.T) {
	for _, c := range []struct {
		name string
		msg  interface{ MarshalBinary() ([]byte, error) }
		want func(*testing.T, *ESMMessage)
	}{
		{
			name: "PDNConnectivityReject",
			msg:  &eps.PDNConnectivityReject{EPSBearerIdentity: 5, PTI: 1, Cause: eps.ESMCauseInsufficientResources},
			want: func(t *testing.T, m *ESMMessage) {
				if m.PDNConnectivityReject == nil || m.PDNConnectivityReject.ESMCause.Label == "" {
					t.Errorf("pdn connectivity reject = %+v", m.PDNConnectivityReject)
				}
			},
		},
		{
			name: "ESMStatus",
			msg:  &eps.ESMStatus{EPSBearerIdentity: 5, PTI: 1, Cause: eps.ESMCauseInvalidEPSBearerIdentity},
			want: func(t *testing.T, m *ESMMessage) {
				if m.ESMStatus == nil {
					t.Errorf("esm status = %+v", m.ESMStatus)
				}
			},
		},
		{
			name: "DeactivateEPSBearerContextRequest",
			msg:  &eps.DeactivateEPSBearerContextRequest{EPSBearerIdentity: 5, PTI: 1, Cause: eps.ESMCauseRegularDeactivation},
			want: func(t *testing.T, m *ESMMessage) {
				if m.DeactivateBearerRequest == nil {
					t.Errorf("deactivate bearer request = %+v", m.DeactivateBearerRequest)
				}
			},
		},
		{
			name: "ModifyEPSBearerContextRequest",
			msg: &eps.ModifyEPSBearerContextRequest{
				EPSBearerIdentity: 5, PTI: 1,
				NewEPSQoS: &eps.EPSQoS{QCI: 9},
			},
			want: func(t *testing.T, m *ESMMessage) {
				if m.ModifyEPSBearerContextRequest == nil || m.ModifyEPSBearerContextRequest.NewEPSQoS == nil {
					t.Errorf("modify eps bearer context request = %+v", m.ModifyEPSBearerContextRequest)
				}
			},
		},
		{
			name: "BearerResourceAllocationRequest",
			msg: &eps.BearerResourceAllocationRequest{
				EPSBearerIdentity: 0, PTI: 1, LinkedEPSBearerIdentity: 5,
				TrafficFlowAggregate:   []byte{0x01, 0x02},
				RequiredTrafficFlowQoS: eps.EPSQoS{QCI: 9},
			},
			want: func(t *testing.T, m *ESMMessage) {
				if m.BearerResourceAllocationRequest == nil {
					t.Fatalf("bearer resource allocation request not decoded")
				}

				if m.BearerResourceAllocationRequest.LinkedEPSBearerIdentity != 5 {
					t.Errorf("linked EPS bearer identity = %d, want 5", m.BearerResourceAllocationRequest.LinkedEPSBearerIdentity)
				}
			},
		},
		{
			name: "BearerResourceModificationRequest",
			msg: &eps.BearerResourceModificationRequest{
				EPSBearerIdentity: 0, PTI: 1, EPSBearerIdentityForPacketFilter: 5,
				TrafficFlowAggregate: []byte{0x03},
			},
			want: func(t *testing.T, m *ESMMessage) {
				if m.BearerResourceModificationRequest == nil {
					t.Errorf("bearer resource modification request = %+v", m.BearerResourceModificationRequest)
				}
			},
		},
	} {
		t.Run(c.name, func(t *testing.T) {
			b, err := c.msg.MarshalBinary()
			if err != nil {
				t.Fatal(err)
			}

			msg := DecodeEPSNASMessage(b)
			if msg.ESMMessage == nil {
				t.Fatalf("no ESM message decoded: %+v", msg)
			}

			c.want(t, msg.ESMMessage)
		})
	}
}
