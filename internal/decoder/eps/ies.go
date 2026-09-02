// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package eps

import (
	"encoding/hex"

	"github.com/ellanetworks/core/internal/decoder/utils"
	"github.com/ellanetworks/core/nas"
	"github.com/ellanetworks/core/nas/eps"
)

// UENetworkCapability is the security and feature support a UE reports for S1
// mode (TS 24.301 §9.9.3.34).
type UENetworkCapability struct {
	Hex string   `json:"hex"`
	EEA []string `json:"eea,omitempty"`
	EIA []string `json:"eia,omitempty"`
	UEA []string `json:"uea,omitempty"`
	UIA []string `json:"uia,omitempty"`
	// UCS2NoPreference is octet 6 bit 8: set means the UE has no preference
	// between the default alphabet and UCS2, clear means it prefers the default
	// alphabet. It is not a statement of UCS2 support.
	UCS2NoPreference bool   `json:"ucs2_no_preference,omitempty"`
	Error            string `json:"error,omitempty"`
}

// UENetworkCapabilityFromBytes renders the element from the octets a 5GS message
// replays (TS 24.501 §9.11.3.48), keeping them so the UE's bidding-down
// comparison stays visible.
func UENetworkCapabilityFromBytes(b []byte) *UENetworkCapability {
	if len(b) == 0 {
		return nil
	}

	c, err := eps.ParseUENetworkCapability(b)
	if err != nil {
		return &UENetworkCapability{Hex: hex.EncodeToString(b), Error: err.Error()}
	}

	return ueNetworkCapability(c)
}

func ueNetworkCapability(c eps.UENetworkCapability) *UENetworkCapability {
	raw, err := c.MarshalBinary()
	if err != nil {
		raw = nil
	}

	out := &UENetworkCapability{
		Hex: hex.EncodeToString(raw),
		EEA: utils.AlgorithmNames("EEA", c.EEA),
		EIA: utils.AlgorithmNames("EIA", c.EIA),
	}

	if c.HasUMTS {
		out.UEA = utils.AlgorithmNames("UEA", c.UEA)
		out.UIA = utils.AlgorithmNames("UIA", c.UIA)
		out.UCS2NoPreference = c.UCS2
	}

	return out
}

// UESecurityCapability is the capability the network replays to the UE so it can
// detect a bidding-down attack (TS 24.301 §9.9.3.36).
type UESecurityCapability struct {
	Hex   string   `json:"hex"`
	EEA   []string `json:"eea,omitempty"`
	EIA   []string `json:"eia,omitempty"`
	UEA   []string `json:"uea,omitempty"`
	UIA   []string `json:"uia,omitempty"`
	GEA   []string `json:"gea,omitempty"`
	Error string   `json:"error,omitempty"`
}

// UESecurityCapabilityFromBytes renders the element from the octets a 5GS
// SECURITY MODE COMMAND replays (TS 24.501 §9.11.3.48A).
func UESecurityCapabilityFromBytes(b []byte) *UESecurityCapability {
	if len(b) == 0 {
		return nil
	}

	c, err := eps.ParseUESecurityCapability(b)
	if err != nil {
		return &UESecurityCapability{Hex: hex.EncodeToString(b), Error: err.Error()}
	}

	return ueSecurityCapability(c)
}

func ueSecurityCapability(c eps.UESecurityCapability) *UESecurityCapability {
	raw, err := c.MarshalBinary()
	if err != nil {
		raw = nil
	}

	out := &UESecurityCapability{
		Hex: hex.EncodeToString(raw),
		EEA: utils.AlgorithmNames("EEA", c.EEA),
		EIA: utils.AlgorithmNames("EIA", c.EIA),
	}

	if c.HasUMTS {
		out.UEA = utils.AlgorithmNames("UEA", c.UEA)
		out.UIA = utils.AlgorithmNames("UIA", c.UIA)
	}

	if c.HasGERAN {
		out.GEA = utils.AlgorithmNames("GEA", c.GEA)
	}

	return out
}

// MSNetworkCapability is the GPRS capability a UE reports (TS 24.008 §10.5.5.12).
type MSNetworkCapability struct {
	Hex string   `json:"hex"`
	GEA []string `json:"gea,omitempty"`
}

func msNetworkCapability(c *eps.MSNetworkCapability) *MSNetworkCapability {
	if c == nil {
		return nil
	}

	out := &MSNetworkCapability{Hex: hex.EncodeToString(c.Rest)}
	if c.GEA1 {
		out.GEA = append(out.GEA, "GEA1")
	}

	out.GEA = append(out.GEA, utils.AlgorithmNames("GEA", c.GEAExtended)...)

	return out
}

type AdditionalUpdateType struct {
	AUTV    bool            `json:"autv"`
	SAF     bool            `json:"saf"`
	PNBCIoT utils.EnumField `json:"pnb_ciot"`
}

func additionalUpdateType(a *eps.AdditionalUpdateType) *AdditionalUpdateType {
	if a == nil {
		return nil
	}

	return &AdditionalUpdateType{
		AUTV:    a.AUTV,
		SAF:     a.SAF,
		PNBCIoT: utils.NamedEnum(uint8(a.PNBCIoT), a.PNBCIoT.String()),
	}
}

type UEStatus struct {
	S1ModeReg bool `json:"s1_mode_reg"`
	N1ModeReg bool `json:"n1_mode_reg"`
}

func ueStatus(s *eps.UEStatus) *UEStatus {
	if s == nil {
		return nil
	}

	return &UEStatus{S1ModeReg: s.S1ModeReg, N1ModeReg: s.N1ModeReg}
}

type NetworkFeatureSupport struct {
	IMSVoPS bool   `json:"ims_vops"`
	EMCBS   bool   `json:"emc_bs"`
	EPCLCS  bool   `json:"epc_lcs"`
	CSLCS   uint8  `json:"cs_lcs"`
	ESRPS   bool   `json:"esr_ps"`
	ERwoPDN bool   `json:"er_wo_pdn"`
	CPCIoT  bool   `json:"cp_ciot"`
	IWKN26  bool   `json:"iwk_n26,omitempty"`
	EPCO    bool   `json:"epco,omitempty"`
	RestHex string `json:"rest_hex,omitempty"`
}

func networkFeatureSupport(n *eps.NetworkFeatureSupport) *NetworkFeatureSupport {
	if n == nil {
		return nil
	}

	return &NetworkFeatureSupport{
		IMSVoPS: n.IMSVoPS, EMCBS: n.EMCBS, EPCLCS: n.EPCLCS, CSLCS: n.CSLCS,
		ESRPS: n.ESRPS, ERwoPDN: n.ERwoPDN, CPCIoT: n.CPCIoT,
		IWKN26: n.IWKN26, EPCO: n.EPCO, RestHex: hex.EncodeToString(n.Rest),
	}
}

type TAI struct {
	PLMN string `json:"plmn"`
	TAC  uint16 `json:"tac"`
}

func taiList(list eps.TAIList) []TAI {
	var out []TAI

	for _, partial := range list {
		for _, t := range partial.TAIs {
			out = append(out, TAI{PLMN: t.PLMN.String(), TAC: t.TAC})
		}
	}

	return out
}

type EPSQoS struct {
	QCI         uint8  `json:"qci"`
	BitRatesHex string `json:"bit_rates_hex,omitempty"`
}

func epsQoS(q eps.EPSQoS) *EPSQoS {
	return &EPSQoS{QCI: q.QCI, BitRatesHex: hex.EncodeToString(q.BitRates)}
}

// APNAMBR is the aggregate maximum bit rate for the APN (TS 24.301 §9.9.4.2).
// The octets are kept as sent: the coding is piecewise and the extended octets
// override the first two.
type APNAMBR struct {
	DownlinkOctet uint8  `json:"downlink_octet"`
	UplinkOctet   uint8  `json:"uplink_octet"`
	ExtendedHex   string `json:"extended_hex,omitempty"`
}

func apnAMBR(a *eps.APNAMBR) *APNAMBR {
	if a == nil {
		return nil
	}

	return &APNAMBR{
		DownlinkOctet: a.DownlinkOctet,
		UplinkOctet:   a.UplinkOctet,
		ExtendedHex:   hex.EncodeToString(a.Extended),
	}
}

type EPSBearerContextStatusItem struct {
	EPSBearerIdentity int  `json:"eps_bearer_identity"`
	Active            bool `json:"active"`
}

// EPSBearerContextStatus renders the per-bearer state (TS 24.301 §9.9.2.1).
func EPSBearerContextStatus(s *nas.EPSBearerContextStatus) []EPSBearerContextStatusItem {
	if s == nil {
		return nil
	}

	// EBI(0) is spare (TS 24.301 §9.9.2.1), so there is no bearer 0 to report.
	out := make([]EPSBearerContextStatusItem, 0, len(s.Active)-1)
	for i := 1; i < len(s.Active); i++ {
		out = append(out, EPSBearerContextStatusItem{EPSBearerIdentity: i, Active: s.Active[i]})
	}

	return out
}
