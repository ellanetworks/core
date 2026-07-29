// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"strings"

	"github.com/ellanetworks/core/internal/decoder/utils"
	"github.com/ellanetworks/core/nas/fgs"
)

type RegistrationRequest struct {
	NasKeySetIdentifier  uint8                 `json:"nas_key_set_identifier,omitempty"`
	RegistrationType5GS  utils.EnumField       `json:"registration_type_5gs"`
	MobileIdentity5GS    MobileIdentity        `json:"mobile_identity_5gs"`
	UESecurityCapability *UESecurityCapability `json:"ue_security_capability,omitempty"`
	NASMessageContainer  []byte                `json:"nas_message_container,omitempty"`

	NoncurrentNativeNASKeySetIdentifier *UnsupportedIE `json:"noncurrent_native_nas_key_set_identifier,omitempty"`
	Capability5GMM                      *UnsupportedIE `json:"capability_5gmm,omitempty"`
	RequestedNSSAI                      *UnsupportedIE `json:"requested_nssai,omitempty"`
	LastVisitedRegisteredTAI            *UnsupportedIE `json:"last_visited_registered_tai,omitempty"`
	S1UENetworkCapability               *UnsupportedIE `json:"s1_ue_network_capability,omitempty"`
	UplinkDataStatus                    *UnsupportedIE `json:"uplink_data_status,omitempty"`
	PDUSessionStatus                    *UnsupportedIE `json:"pdu_session_status,omitempty"`
	MICOIndication                      *UnsupportedIE `json:"mico_indication,omitempty"`
	UEStatus                            *UnsupportedIE `json:"ue_status,omitempty"`
	AdditionalGUTI                      *UnsupportedIE `json:"additional_guti,omitempty"`
	AllowedPDUSessionStatus             *UnsupportedIE `json:"allowed_pdu_session_status,omitempty"`
	UesUsageSetting                     *UnsupportedIE `json:"ues_usage_setting,omitempty"`
	RequestedDRXParameters              *UnsupportedIE `json:"requested_drx_parameters,omitempty"`
	EPSNASMessageContainer              *UnsupportedIE `json:"eps_nas_message_container,omitempty"`
	LADNIndication                      *UnsupportedIE `json:"ladn_indication,omitempty"`
	PayloadContainer                    *UnsupportedIE `json:"payload_container,omitempty"`
	NetworkSlicingIndication            *UnsupportedIE `json:"network_slicing_indication,omitempty"`
	UpdateType5GS                       *UnsupportedIE `json:"update_type_5gs,omitempty"`
	EPSBearerContextStatus              *UnsupportedIE `json:"eps_bearer_context_status,omitempty"`
}

func buildRegistrationRequest(msg *fgs.RegistrationRequest) *RegistrationRequest {
	out := &RegistrationRequest{
		NasKeySetIdentifier: msg.NgKSI.Value,
		RegistrationType5GS: registrationType5GSEnum(msg.RegistrationType),
		MobileIdentity5GS:   buildMobileIdentity(msg.MobileIdentity),
		NASMessageContainer: msg.NASMessageContainer,
	}

	if msg.UESecurityCapability != nil {
		out.UESecurityCapability = buildUESecurityCapability(*msg.UESecurityCapability)
	}

	if msg.GMMCapability != nil {
		out.Capability5GMM = makeUnsupportedIE()
	}

	if len(msg.RequestedNSSAI) > 0 {
		out.RequestedNSSAI = makeUnsupportedIE()
	}

	if msg.UplinkDataStatus != nil {
		out.UplinkDataStatus = makeUnsupportedIE()
	}

	if msg.PDUSessionStatus != nil {
		out.PDUSessionStatus = makeUnsupportedIE()
	}

	if msg.AllowedPDUSessionStatus != nil {
		out.AllowedPDUSessionStatus = makeUnsupportedIE()
	}

	if msg.RequestedDRXParameters != nil {
		out.RequestedDRXParameters = makeUnsupportedIE()
	}

	if msg.MICOIndication != nil {
		out.MICOIndication = makeUnsupportedIE()
	}

	if msg.UpdateType5GS != nil {
		out.UpdateType5GS = makeUnsupportedIE()
	}

	for _, ie := range msg.Unrecognized {
		switch ie.IEI {
		case ieiNoncurrentNativeNASKSI:
			out.NoncurrentNativeNASKeySetIdentifier = makeUnsupportedIE()
		case ieiS1UENetworkCapability:
			out.S1UENetworkCapability = makeUnsupportedIE()
		case ieiUesUsageSetting:
			out.UesUsageSetting = makeUnsupportedIE()
		case ieiUEStatus:
			out.UEStatus = makeUnsupportedIE()
		case ieiLastVisitedTAI:
			out.LastVisitedRegisteredTAI = makeUnsupportedIE()
		case ieiEPSBearerContextStatus:
			out.EPSBearerContextStatus = makeUnsupportedIE()
		case ieiEPSNASMessageContainer:
			out.EPSNASMessageContainer = makeUnsupportedIE()
		case ieiLADNIndication:
			out.LADNIndication = makeUnsupportedIE()
		case ieiAdditionalGUTI:
			out.AdditionalGUTI = makeUnsupportedIE()
		case ieiPayloadContainer:
			out.PayloadContainer = makeUnsupportedIE()
		case ieiNetworkSlicingIndication:
			out.NetworkSlicingIndication = makeUnsupportedIE()
		}
	}

	return out
}

// registrationType5GSEnum renders the registration type for the capture. The
// codec's own name is the single definition; the table this replaced drifted
// from it, reporting a disaster roaming initial registration as "Reserved" and
// every type added since as unknown.
func registrationType5GSEnum(t fgs.RegistrationType) utils.EnumField {
	name := t.String()

	return utils.MakeEnum(uint8(t), name, strings.HasPrefix(name, "unknown"))
}

// buildUESecurityCapability renders the 5G integrity and ciphering algorithm bitmaps
// of the UE security capability IE value (TS 24.501 §9.11.3.54).
func buildUESecurityCapability(sc fgs.UESecurityCapability) *UESecurityCapability {
	return &UESecurityCapability{
		IntegrityAlgorithm: IntegrityAlgorithm{
			NIA0: sc.SupportsIA(0),
			NIA1: sc.SupportsIA(1),
			NIA2: sc.SupportsIA(2),
			NIA3: sc.SupportsIA(3),
		},
		CipheringAlgorithm: CipheringAlgorithm{
			NEA0: sc.SupportsEA(0),
			NEA1: sc.SupportsEA(1),
			NEA2: sc.SupportsEA(2),
			NEA3: sc.SupportsEA(3),
		},
	}
}
