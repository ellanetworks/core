// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package eps

import (
	"encoding/hex"

	"github.com/ellanetworks/core/internal/decoder/utils"
	"github.com/ellanetworks/core/nas/eps"
)

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
	IMSVoPS bool            `json:"ims_vops"`
	EMCBS   bool            `json:"emc_bs"`
	EPCLCS  bool            `json:"epc_lcs"`
	CSLCS   utils.EnumField `json:"cs_lcs"`
	ESRPS   bool            `json:"esr_ps"`
	ERwoPDN bool            `json:"er_wo_pdn"`
	CPCIoT  bool            `json:"cp_ciot"`
	IWKN26  bool            `json:"iwk_n26,omitempty"`
	EPCO    bool            `json:"epco,omitempty"`
	RestHex string          `json:"rest_hex,omitempty"`
}

func networkFeatureSupport(n *eps.NetworkFeatureSupport) *NetworkFeatureSupport {
	if n == nil {
		return nil
	}

	return &NetworkFeatureSupport{
		IMSVoPS: n.IMSVoPS, EMCBS: n.EMCBS, EPCLCS: n.EPCLCS, CSLCS: utils.NamedEnum(n.CSLCS, n.CSLCSName()),
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
