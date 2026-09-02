// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	epsdec "github.com/ellanetworks/core/internal/decoder/eps"
	"github.com/ellanetworks/core/internal/decoder/utils"
	"github.com/ellanetworks/core/nas/fgs"
)

type RegistrationRequest struct {
	NasKeySetIdentifier    uint8                 `json:"nas_key_set_identifier,omitempty"`
	RegistrationType5GS    utils.EnumField       `json:"registration_type_5gs"`
	FollowOnRequestPending bool                  `json:"follow_on_request_pending"`
	MobileIdentity5GS      MobileIdentity        `json:"mobile_identity_5gs"`
	UESecurityCapability   *UESecurityCapability `json:"ue_security_capability,omitempty"`
	NASMessageContainer    *NASMessageContainer  `json:"nas_message_container,omitempty"`

	Capability5GMM          *GMMCapability                      `json:"capability_5gmm,omitempty"`
	RequestedNSSAI          []SNSSAI                            `json:"requested_nssai,omitempty"`
	S1UENetworkCapability   *epsdec.UENetworkCapability         `json:"s1_ue_network_capability,omitempty"`
	UplinkDataStatus        []PDUSessionStatusPDU               `json:"uplink_data_status,omitempty"`
	PDUSessionStatus        []PDUSessionStatusPDU               `json:"pdu_session_status,omitempty"`
	MICOIndication          *MICOIndication                     `json:"mico_indication,omitempty"`
	UEStatus                *UEStatus                           `json:"ue_status,omitempty"`
	AdditionalGUTI          *MobileIdentity                     `json:"additional_guti,omitempty"`
	AllowedPDUSessionStatus []PDUSessionStatusPDU               `json:"allowed_pdu_session_status,omitempty"`
	RequestedDRXParameters  *DRXParameter                       `json:"requested_drx_parameters,omitempty"`
	EPSNASMessageContainer  *EPSNASMessageContainer             `json:"eps_nas_message_container,omitempty"`
	UpdateType5GS           *UpdateType5GS                      `json:"update_type_5gs,omitempty"`
	EPSBearerContextStatus  []epsdec.EPSBearerContextStatusItem `json:"eps_bearer_context_status,omitempty"`

	UnrecognizedIEs []utils.RawIE `json:"unrecognized_ies,omitempty"`
}

func buildRegistrationRequest(msg *fgs.RegistrationRequest) *RegistrationRequest {
	out := &RegistrationRequest{
		FollowOnRequestPending: msg.FOR,
		NasKeySetIdentifier:    msg.NgKSI.Value,
		RegistrationType5GS:    registrationType5GSEnum(msg.RegistrationType),
		MobileIdentity5GS:      buildMobileIdentity(msg.MobileIdentity),
		NASMessageContainer:    nasMessageContainer(msg.NASMessageContainer),
	}

	if msg.UESecurityCapability != nil {
		out.UESecurityCapability = buildUESecurityCapability(*msg.UESecurityCapability)
	}

	out.S1UENetworkCapability = epsdec.UENetworkCapabilityFromBytes(msg.S1UENetworkCapability)
	out.EPSBearerContextStatus = epsdec.EPSBearerContextStatus(msg.EPSBearerContextStatus)
	out.EPSNASMessageContainer = epsNASMessageContainer(msg.EPSNASMessageContainer)

	if msg.AdditionalGUTI != nil {
		guti := buildMobileIdentity(*msg.AdditionalGUTI)
		out.AdditionalGUTI = &guti
	}

	out.RequestedNSSAI = nssai(msg.RequestedNSSAI)
	out.Capability5GMM = gmmCapability(msg.GMMCapability)
	out.UEStatus = ueStatus(msg.UEStatus)
	out.MICOIndication = micoIndication(msg.MICOIndication)
	out.UpdateType5GS = updateType5GS(msg.UpdateType5GS)
	out.RequestedDRXParameters = drxParameter(msg.RequestedDRXParameters)
	out.UplinkDataStatus = decodePDUSessionStatus(msg.UplinkDataStatus)
	out.PDUSessionStatus = decodePDUSessionStatus(msg.PDUSessionStatus)
	out.AllowedPDUSessionStatus = decodePDUSessionStatus(msg.AllowedPDUSessionStatus)

	out.UnrecognizedIEs = utils.RawIEs(msg.Unrecognized)

	return out
}

func registrationType5GSEnum(t fgs.RegistrationType) utils.EnumField {
	return utils.NamedEnum(uint8(t), t.Name())
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
