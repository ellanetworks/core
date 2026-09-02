// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"encoding/hex"
	"fmt"

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
	SST            int32   `json:"sst"`
	SD             *string `json:"sd,omitempty"`
	MappedHPLMNSST *int32  `json:"mapped_hplmn_sst,omitempty"`
	MappedHPLMNSD  *string `json:"mapped_hplmn_sd,omitempty"`
}

type RegistrationAccept struct {
	RegistrationResult5GS    utils.EnumField           `json:"registration_result_5gs"`
	GUTI5G                   *GUTI5GContent            `json:"guti_5g,omitempty"`
	EquivalentPLMNs          []PLMNID                  `json:"equivalent_plmns,omitempty"`
	TAIList                  []TAI                     `json:"tai_list,omitempty"`
	AllowedNSSAI             []SNSSAI                  `json:"allowed_nssai,omitempty"`
	NetworkFeatureSupport5GS *NetworkFeatureSupport5GS `json:"network_feature_support_5gs,omitempty"`

	ConfiguredNSSAI                 []SNSSAI                        `json:"configured_nssai,omitempty"`
	PDUSessionStatus                []PDUSessionStatusPDU           `json:"pdu_session_status,omitempty"`
	PDUSessionReactivationResult    []PDUSessionReactivateResultPDU `json:"pdu_session_reactivation_result,omitempty"`
	MICOIndication                  *MICOIndication                 `json:"mico_indication,omitempty"`
	T3512Value                      *GPRSTimer3Value                `json:"t3512_value,omitempty"`
	Non3GppDeregistrationTimerValue *GPRSTimer2Value                `json:"non_3gpp_deregistration_timer_value,omitempty"`
	T3502Value                      *GPRSTimer2Value                `json:"t3502_value,omitempty"`
	SORTransparentContainer         *RawOctets                      `json:"sor_transparent_container,omitempty"`
	EAPMessage                      *RawOctets                      `json:"eap_message,omitempty"`
	NegotiatedDRXParameters         *DRXParameter                   `json:"negotiated_drx_parameters,omitempty"`
	EPSBearerContextStatus          []EPSBearerContextStatusItem    `json:"eps_bearer_context_status,omitempty"`

	UnrecognizedIEs []utils.RawIE `json:"unrecognized_ies,omitempty"`
}

func registrationResult5GSEnum(value fgs.RegistrationResult) utils.EnumField {
	value &= 0x07

	return utils.NamedEnum(uint8(value), value.Name())
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

	out.SORTransparentContainer = rawOctets(msg.SORTransparentContainer)
	out.EAPMessage = rawOctets(msg.EAP)
	out.ConfiguredNSSAI = nssai(msg.ConfiguredNSSAI)
	out.EPSBearerContextStatus = epsBearerContextStatus(msg.EPSBearerContextStatus)
	out.MICOIndication = micoIndication(msg.MICOIndication)
	out.NegotiatedDRXParameters = drxParameter(msg.NegotiatedDRX)
	out.T3512Value = gprsTimer3(msg.T3512)
	out.Non3GppDeregistrationTimerValue = gprsTimer2(msg.Non3GppDeregistrationTimer)
	out.T3502Value = gprsTimer2(msg.T3502)
	out.PDUSessionStatus = decodePDUSessionStatus(msg.PDUSessionStatus)
	out.PDUSessionReactivationResult = decodePDUSessionReactivationResult(msg.PDUSessionReactivationResult)

	out.UnrecognizedIEs = utils.RawIEsExcept(msg.Unrecognized, ieiEquivalentPLMNs)

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

// nssai decodes an NSSAI IE value: a sequence of length-prefixed S-NSSAIs
// (TS 24.501 §9.11.3.37), used for the allowed, requested and configured lists.
func nssai(list fgs.NSSAI) []SNSSAI {
	if len(list) == 0 {
		return nil
	}

	out := make([]SNSSAI, 0, len(list))
	for _, s := range list {
		out = append(out, snssaiFromNAS(s))
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
