// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/ellanetworks/core/internal/decoder/utils"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/nas"
	"github.com/ellanetworks/core/nas/fgs"
)

type PLMNID struct {
	Mcc string `json:"mcc"`
	Mnc string `json:"mnc"`
}

type TAI struct {
	PLMNID PLMNID `json:"plmn_id"`
	TAC    string `json:"tac"`
}

// GUTI5GContent is a decoded 5G-GUTI (TS 24.501 §9.11.3.4).
type GUTI5GContent struct {
	Mcc         string `json:"mcc"`
	Mnc         string `json:"mnc"`
	AMFRegionID uint8  `json:"amf_region_id"`
	AMFSetID    uint16 `json:"amf_set_id"`
	AMFPointer  uint8  `json:"amf_pointer"`
	TMSI        string `json:"tmsi"`
}

type NetworkFeatureSupport5GS struct {
	Emc          uint8 `json:"emc"`
	EmcN3        uint8 `json:"emc_n3"`
	Emf          uint8 `json:"emf"`
	ImsVoPS      uint8 `json:"ims_vops"`
	IwkN26       uint8 `json:"iwk_n26"`
	Mcsi         uint8 `json:"mcsi"`
	Mpsi         uint8 `json:"mpsi"`
	IMSVoPS3GPP  uint8 `json:"ims_vops_3gpp"`
	IMSVoPSN3GPP uint8 `json:"ims_vops_n3gpp"`
}

type SNSSAI struct {
	SST int32   `json:"sst"`
	SD  *string `json:"sd,omitempty"`
}

type RegistrationAccept struct {
	RegistrationResult5GS    utils.EnumField           `json:"registration_result_5gs"`
	GUTI5G                   *GUTI5GContent            `json:"guti_5g,omitempty"`
	EquivalentPLMNs          []PLMNID                  `json:"equivalent_plmns,omitempty"`
	TAIList                  []TAI                     `json:"tai_list,omitempty"`
	AllowedNSSAI             []SNSSAI                  `json:"allowed_nssai,omitempty"`
	NetworkFeatureSupport5GS *NetworkFeatureSupport5GS `json:"network_feature_support_5gs,omitempty"`

	RejectedNSSAI                            *UnsupportedIE `json:"rejected_nssai,omitempty"`
	ConfiguredNSSAI                          *UnsupportedIE `json:"configured_nssai,omitempty"`
	PDUSessionStatus                         *UnsupportedIE `json:"pdu_session_status,omitempty"`
	PDUSessionReactivationResult             *UnsupportedIE `json:"pdu_session_reactivation_result,omitempty"`
	PDUSessionReactivationResultErrorCause   *UnsupportedIE `json:"pdu_session_reactivation_result_error_cause,omitempty"`
	LADNInformation                          *UnsupportedIE `json:"ladn_information,omitempty"`
	MICOIndication                           *UnsupportedIE `json:"mico_indication,omitempty"`
	NetworkSlicingIndication                 *UnsupportedIE `json:"network_slicing_indication,omitempty"`
	ServiceAreaList                          *UnsupportedIE `json:"service_area_list,omitempty"`
	T3512Value                               *UnsupportedIE `json:"t3512_value,omitempty"`
	Non3GppDeregistrationTimerValue          *UnsupportedIE `json:"non_3gpp_deregistration_timer_value,omitempty"`
	T3502Value                               *UnsupportedIE `json:"t3502_value,omitempty"`
	EmergencyNumberList                      *UnsupportedIE `json:"emergency_number_list,omitempty"`
	ExtendedEmergencyNumberList              *UnsupportedIE `json:"extended_emergency_number_list,omitempty"`
	SORTransparentContainer                  *UnsupportedIE `json:"sor_transparent_container,omitempty"`
	EAPMessage                               *UnsupportedIE `json:"eap_message,omitempty"`
	NSSAIInclusionMode                       *UnsupportedIE `json:"nssai_inclusion_mode,omitempty"`
	OperatordefinedAccessCategoryDefinitions *UnsupportedIE `json:"operatordefined_access_category_definitions,omitempty"`
	NegotiatedDRXParameters                  *UnsupportedIE `json:"negotiated_drx_parameters,omitempty"`
	Non3GppNwPolicies                        *UnsupportedIE `json:"non_3gpp_nw_policies,omitempty"`
	EPSBearerContextStatus                   *UnsupportedIE `json:"eps_bearer_context_status,omitempty"`
}

func registrationResult5GSEnum(value fgs.RegistrationResult) utils.EnumField {
	value &= 0x07
	if name := value.String(); !strings.HasPrefix(name, "unknown") {
		return utils.MakeEnum(uint8(value), name, false)
	}

	return utils.MakeEnum(uint8(value), "", true)
}

func buildRegistrationAccept(msg *fgs.RegistrationAccept) *RegistrationAccept {
	out := &RegistrationAccept{
		RegistrationResult5GS: registrationResult5GSEnum(msg.RegistrationResult),
	}

	if msg.GUTI != nil {
		out.GUTI5G = buildGUTI5G(*msg.GUTI)
	}

	if v, ok := preservedIE(msg.Unrecognized, ieiEquivalentPLMNs); ok {
		out.EquivalentPLMNs = equivalentPlmnsFromRaw(v)
	}

	if msg.TAIList != nil {
		out.TAIList = taiList(*msg.TAIList)
	}

	if msg.AllowedNSSAI != nil {
		out.AllowedNSSAI = nssai(msg.AllowedNSSAI)
	}

	if msg.NetworkFeatureSupport != nil {
		nfs := networkFeatureSupport(*msg.NetworkFeatureSupport)
		out.NetworkFeatureSupport5GS = &nfs
	}

	presence := []struct {
		set  bool
		dest **UnsupportedIE
	}{
		{hasPreservedIE(msg.Unrecognized, ieiRejectedNSSAI), &out.RejectedNSSAI},
		{msg.ConfiguredNSSAI != nil, &out.ConfiguredNSSAI},
		{msg.PDUSessionStatus != nil, &out.PDUSessionStatus},
		{msg.PDUSessionReactivationResult != nil, &out.PDUSessionReactivationResult},
		{hasPreservedIE(msg.Unrecognized, ieiPDUReactErrCause), &out.PDUSessionReactivationResultErrorCause},
		{hasPreservedIE(msg.Unrecognized, ieiLADNInformation), &out.LADNInformation},
		{msg.MICOIndication != nil, &out.MICOIndication},
		{hasPreservedIE(msg.Unrecognized, ieiNetworkSlicingIndication), &out.NetworkSlicingIndication},
		{hasPreservedIE(msg.Unrecognized, ieiServiceAreaList), &out.ServiceAreaList},
		{msg.T3512 != nil, &out.T3512Value},
		{msg.Non3GppDeregistrationTimer != nil, &out.Non3GppDeregistrationTimerValue},
		{msg.T3502 != nil, &out.T3502Value},
		{hasPreservedIE(msg.Unrecognized, ieiEmergencyNumberList), &out.EmergencyNumberList},
		{hasPreservedIE(msg.Unrecognized, ieiExtEmergencyNumberList), &out.ExtendedEmergencyNumberList},
		{msg.SORTransparentContainer != nil, &out.SORTransparentContainer},
		{msg.EAP != nil, &out.EAPMessage},
		{hasPreservedIE(msg.Unrecognized, ieiNSSAIInclusionMode), &out.NSSAIInclusionMode},
		{hasPreservedIE(msg.Unrecognized, ieiOperatorAccessCategory), &out.OperatordefinedAccessCategoryDefinitions},
		{msg.NegotiatedDRX != nil, &out.NegotiatedDRXParameters},
		{hasPreservedIE(msg.Unrecognized, ieiNon3GppNwPolicies), &out.Non3GppNwPolicies},
		{msg.EPSBearerContextStatus != nil, &out.EPSBearerContextStatus},
	}

	for _, p := range presence {
		if p.set {
			*p.dest = makeUnsupportedIE()
		}
	}

	return out
}

func buildGUTI5G(id fgs.MobileIdentity) *GUTI5GContent {
	g := id.GUTI
	if g == nil {
		return nil
	}

	return &GUTI5GContent{
		Mcc:         g.PLMN.MCC,
		Mnc:         g.PLMN.MNC,
		AMFRegionID: g.AMFRegionID,
		AMFSetID:    g.AMFSetID,
		AMFPointer:  g.AMFPointer,
		TMSI:        hex.EncodeToString(g.TMSI[:]),
	}
}

func taiList(list fgs.TAIList) []TAI {
	tais := list.TAIs()

	out := make([]TAI, 0, len(tais))
	for _, t := range tais {
		out = append(out, TAI{
			PLMNID: PLMNID{Mcc: t.PLMN.MCC, Mnc: t.PLMN.MNC},
			TAC:    fmt.Sprintf("%06x", t.TAC),
		})
	}

	return out
}

// equivalentPlmnsFromRaw decodes the equivalent PLMNs IE value (a sequence of
// 3-octet PLMN identities).
// Information element identifiers of the REGISTRATION ACCEPT elements the nas
// library does not model (TS 24.501 table 8.2.7.1.1). They arrive among the
// message's unrecognized elements, which is where this decoder reads them.
const (
	ieiEquivalentPLMNs        uint8 = 0x4A
	ieiRejectedNSSAI          uint8 = 0x11
	ieiServiceAreaList        uint8 = 0x27
	ieiEmergencyNumberList    uint8 = 0x34
	ieiOperatorAccessCategory uint8 = 0x76
	ieiLADNInformation        uint8 = 0x79
	ieiExtEmergencyNumberList uint8 = 0x7A
	ieiNSSAIInclusionMode     uint8 = 0xA0
	ieiNon3GppNwPolicies      uint8 = 0xD0
)

// preservedIE returns the value of the preserved element with this IEI, and
// whether the message carried one.
func preservedIE(unrec []nas.RawIE, iei uint8) ([]byte, bool) {
	for _, ie := range unrec {
		if ie.IEI == iei {
			return ie.Value, true
		}
	}

	return nil, false
}

// hasPreservedIE reports whether the message carried an element with this IEI.
func hasPreservedIE(unrec []nas.RawIE, iei uint8) bool {
	_, ok := preservedIE(unrec, iei)

	return ok
}

func equivalentPlmnsFromRaw(v []byte) []PLMNID {
	if len(v) == 0 {
		logger.EllaLog.Warn("EquivalentPlmns length is zero")
		return nil
	}

	if len(v)%3 != 0 {
		logger.EllaLog.Warn("EquivalentPlmns length not multiple of 3")
		return nil
	}

	n := len(v) / 3
	out := make([]PLMNID, 0, n)

	for i := range n {
		base := i * 3

		plmn, err := nas.ParsePLMN([3]byte{v[base], v[base+1], v[base+2]})
		if err != nil {
			continue
		}

		out = append(out, PLMNID{Mcc: plmn.MCC, Mnc: plmn.MNC})
	}

	return out
}

// allowedNSSAIFromRaw decodes the allowed NSSAI IE value (a sequence of
// length-prefixed S-NSSAIs, TS 24.501 §9.11.3.37).
func nssai(list fgs.NSSAI) []SNSSAI {
	out := make([]SNSSAI, 0, len(list))
	for _, s := range list {
		item := SNSSAI{SST: int32(s.SST)}
		if s.SD != nil {
			sd := hex.EncodeToString(s.SD[:])
			item.SD = &sd
		}

		out = append(out, item)
	}

	return out
}

func networkFeatureSupport(nfs fgs.NetworkFeatureSupport) NetworkFeatureSupport5GS {
	return NetworkFeatureSupport5GS{
		Emc:          nfs.EMC,
		Emf:          nfs.EMF,
		IwkN26:       b2u(nfs.IWKN26),
		Mpsi:         b2u(nfs.MPSI),
		EmcN3:        b2u(nfs.EMCN3),
		Mcsi:         b2u(nfs.MCSI),
		IMSVoPS3GPP:  b2u(nfs.IMSVoPS3GPP),
		IMSVoPSN3GPP: b2u(nfs.IMSVoPSN3GPP),
	}
}
