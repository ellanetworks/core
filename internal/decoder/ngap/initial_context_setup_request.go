// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"encoding/hex"
	"fmt"

	"github.com/ellanetworks/core/internal/decoder/utils"
	"github.com/ellanetworks/core/ngap"
)

type ExpectedUEActivityBehaviour struct {
	ExpectedActivityPeriod                 *int64           `json:"expected_activity_period,omitempty"`
	ExpectedIdlePeriod                     *int64           `json:"expected_idle_period,omitempty"`
	SourceOfUEActivityBehaviourInformation *utils.EnumField `json:"source_of_ue_activity_behaviour_information,omitempty"`
}

type NGRANCGI struct {
	NRCGI    *NRCGI    `json:"nr_ran_cgi,omitempty"`
	EUTRACGI *EUTRACGI `json:"eutra_cgi,omitempty"`

	Error string `json:"error,omitempty"` // Reserved field for decoding errors
}

type ExpectedUEMovingTrajectoryItem struct {
	NGRANCGI         NGRANCGI `json:"ng_ran_cgi"`
	TimeStayedInCell *int64   `json:"time_stayed_in_cell,omitempty"`
}

type ExpectedUEBehaviour struct {
	ExpectedUEActivityBehaviour *ExpectedUEActivityBehaviour     `json:"expected_ue_activity_behaviour,omitempty"`
	ExpectedHOInterval          *utils.EnumField                 `json:"expected_ho_interval,omitempty"`
	ExpectedUEMobility          *utils.EnumField                 `json:"expected_ue_mobility,omitempty"`
	ExpectedUEMovingTrajectory  []ExpectedUEMovingTrajectoryItem `json:"expected_ue_moving_trajectory,omitempty"`
}

type CoreNetworkAssistanceInformation struct {
	UEIdentityIndexValue            string               `json:"ue_identity_index_value"`
	UESpecificDRX                   *utils.EnumField     `json:"ue_specific_drx,omitempty"`
	PeriodicRegistrationUpdateTimer string               `json:"periodic_registration_update_timer"`
	MICOModeIndication              *string              `json:"mico_mode_indication,omitempty"`
	TAIListForInactive              []TAI                `json:"tai_list_for_inactive,omitempty"`
	ExpectedUEBehaviour             *ExpectedUEBehaviour `json:"expected_ue_behaviour,omitempty"`
}

type MaximumBitRate struct {
	DownlinkNAggregateMaximumBitRate uint64 `json:"downlink_n_aggregate_maximum_bit_rate"`
	UplinkNAggregateMaximumBitRate   uint64 `json:"uplink_n_aggregate_maximum_bit_rate"`
	Unit                             string `json:"unit"`
}

type GTPTunnel struct {
	GTPTEID               uint32 `json:"gtp_teid"`
	TransportLayerAddress string `json:"transport_layer_address"`
}

type ULNGUUPTNLInformation struct {
	GTPTunnel GTPTunnel `json:"gtp_tunnel"`
}

type GBRQosInfo struct {
	MaximumFlowBitRateDL    int64 `json:"maximum_flow_bit_rate_dl"`
	MaximumFlowBitRateUL    int64 `json:"maximum_flow_bit_rate_ul"`
	GuaranteedFlowBitRateDL int64 `json:"guaranteed_flow_bit_rate_dl"`
	GuaranteedFlowBitRateUL int64 `json:"guaranteed_flow_bit_rate_ul"`
}

type QosFlowSetupRequest struct {
	QosId             int64       `json:"qos_id"`
	FiveQi            *int64      `json:"five_qi,omitempty"`
	PriArp            int64       `json:"pri_arp"`
	Dynamic           bool        `json:"dynamic,omitempty"`
	PriorityLevelQos  *int64      `json:"priority_level_qos,omitempty"`
	PacketDelayBudget *int64      `json:"packet_delay_budget,omitempty"`
	GBRQosInformation *GBRQosInfo `json:"gbr_qos_information,omitempty"`
}

type PDUSessionResourceSetupRequestTransfer struct {
	ULNGUUPTNLInformation   *ULNGUUPTNLInformation `json:"ul_ng_u_up_tnl_information,omitempty"`
	QosFlowSetupRequestList []QosFlowSetupRequest  `json:"qos_flow_setup_request_list,omitempty"`
	PduSType                *utils.EnumField       `json:"pdu_s_type,omitempty"`
	MaximumBitRate          *MaximumBitRate        `json:"maximum_bit_rate,omitempty"`
	SecurityIndication      *UnsupportedIE         `json:"security_indication,omitempty"`
	UnsupportedIEs          []string               `json:"unsupported_ies,omitempty"`
}

type PDUSessionResourceSetupCxtReq struct {
	PDUSessionID                           int64                                  `json:"pdu_session_id"`
	NASPDU                                 *NASPDU                                `json:"nas_pdu,omitempty"`
	SNSSAI                                 SNSSAI                                 `json:"snssai"`
	PDUSessionResourceSetupRequestTransfer PDUSessionResourceSetupRequestTransfer `json:"pdu_session_resource_setup_request_transfer"`

	Error string `json:"error,omitempty"`
}

type UESecurityCapabilities struct {
	NRencryptionAlgorithms             []string `json:"nr_encryption_algorithms"`
	NRintegrityProtectionAlgorithms    []string `json:"nr_integrity_protection_algorithms"`
	EUTRAencryptionAlgorithms          string   `json:"eutra_encryption_algorithms"`
	EUTRAintegrityProtectionAlgorithms string   `json:"eutra_integrity_protection_algorithms"`
}

// Initial Context Setup Request establishes the UE context on the NG-RAN node,
// optionally setting up PDU sessions with it (TS 38.413 §9.2.2.1). The optional
// IEs §9.2.2.1 also allows and this AMF never sends — Old AMF, Core Network
// Assistance Information, Mobility Restriction List, Index to RFSP — render as
// preserved-unmodeled if a capture from another core carries them.
func buildInitialContextSetupRequest(value []byte) NGAPMessageValue {
	m, err := ngap.ParseInitialContextSetupRequest(value)
	if err != nil {
		return NGAPMessageValue{Error: fmt.Sprintf("parse Initial Context Setup Request: %v", err)}
	}

	ies := []IE{
		ie(ngap.IDAMFUENGAPID, ngap.CriticalityReject, int64(m.AMFUENGAPID)),
		ie(ngap.IDRANUENGAPID, ngap.CriticalityReject, int64(m.RANUENGAPID)),
	}

	if m.UEAggregateMaximumBitRate != nil {
		ies = append(ies, ie(ngap.IDUEAggregateMaximumBitRate, ngap.CriticalityReject, UEAggregateMaximumBitRate{
			Downlink: int64(m.UEAggregateMaximumBitRate.DL),
			Uplink:   int64(m.UEAggregateMaximumBitRate.UL),
			Unit:     "bps",
		}))
	}

	ies = append(ies, ie(ngap.IDGUAMI, ngap.CriticalityReject, guami(m.GUAMI)))

	if m.PDUSessionResourceSetup != nil {
		out := make([]PDUSessionResourceSetupCxtReq, 0, len(m.PDUSessionResourceSetup))

		for _, item := range m.PDUSessionResourceSetup {
			entry := PDUSessionResourceSetupCxtReq{
				PDUSessionID: int64(item.PDUSessionID),
				SNSSAI:       buildSNSSAIValue(item.SNSSAI),
			}

			transfer, err := libSetupRequestTransfer(item.Transfer)
			if err != nil {
				entry.Error = fmt.Sprintf("failed to decode transfer: %v", err)
			} else {
				entry.PDUSessionResourceSetupRequestTransfer = *transfer
			}

			if item.NASPDU != nil {
				entry.NASPDU = ngap.Ptr(libNASPDU(*item.NASPDU))
			}

			out = append(out, entry)
		}

		ies = append(ies, ie(ngap.IDPDUSessionResourceSetupListCxtReq, ngap.CriticalityReject, out))
	}

	if m.AllowedNSSAI != nil {
		slices := make([]SNSSAI, 0, len(m.AllowedNSSAI))
		for _, item := range m.AllowedNSSAI {
			slices = append(slices, buildSNSSAIValue(item.SNSSAI))
		}

		ies = append(ies, ie(ngap.IDAllowedNSSAI, ngap.CriticalityReject, slices))
	}

	ies = append(ies,
		ie(ngap.IDUESecurityCapabilities, ngap.CriticalityReject, libUESecurityCapabilities(m.UESecurityCapabilities)),
		ie(ngap.IDSecurityKey, ngap.CriticalityReject, hex.EncodeToString(m.SecurityKey[:])),
	)

	if m.NASPDU != nil {
		ies = append(ies, ie(ngap.IDNASPDU, ngap.CriticalityIgnore, libNASPDU(*m.NASPDU)))
	}

	if m.UERadioCapability != nil {
		ies = append(ies, ie(ngap.IDUERadioCapability, ngap.CriticalityIgnore, []byte(m.UERadioCapability)))
	}

	if m.UERadioCapabilityForPaging != nil {
		ies = append(ies, ie(ngap.IDUERadioCapabilityForPaging, ngap.CriticalityIgnore,
			ueRadioCapabilityForPaging(*m.UERadioCapabilityForPaging)))
	}

	return NGAPMessageValue{IEs: append(ies, unmodeledIEs(m.UnknownIEs())...)}
}

// libUESecurityCapabilities renders the four algorithm bit strings
// (TS 38.413 §9.3.1.86). The NR pair is expanded to algorithm names; the E-UTRA
// pair stays hex, as this core does not negotiate E-UTRA algorithms.
func libUESecurityCapabilities(c ngap.UESecurityCapabilities) UESecurityCapabilities {
	return UESecurityCapabilities{
		NRencryptionAlgorithms:             nrAlgorithmNames(c.NREncryptionAlgorithms, "NEA"),
		NRintegrityProtectionAlgorithms:    nrAlgorithmNames(c.NRIntegrityProtectionAlgorithms, "NIA"),
		EUTRAencryptionAlgorithms:          bitsHex(uint64(c.EUTRAEncryptionAlgorithms), 16),
		EUTRAintegrityProtectionAlgorithms: bitsHex(uint64(c.EUTRAIntegrityProtectionAlgorithms), 16),
	}
}

// nrAlgorithmNames lists the supported algorithms named by the top three bits,
// which are 5G-xA1, xA2 and xA3 in order (TS 33.501 §5.11.2). An empty set means
// only the null algorithm is supported.
func nrAlgorithmNames(mask uint16, prefix string) []string {
	var algos []string

	for i := range 3 {
		if mask&(1<<uint(15-i)) != 0 {
			algos = append(algos, fmt.Sprintf("%s%d", prefix, i+1))
		}
	}

	if len(algos) == 0 {
		if prefix == "NIA" {
			return []string{"None or NIA0 (null integrity)"}
		}

		return []string{"None or NEA0 (null encryption)"}
	}

	return algos
}
