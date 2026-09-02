// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package fgs

import (
	"encoding/hex"

	epsdec "github.com/ellanetworks/core/internal/decoder/nas/eps"
	"github.com/ellanetworks/core/internal/decoder/utils"
	"github.com/ellanetworks/core/nas/fgs"
)

type GMMCapability struct {
	SGC        bool   `json:"sgc"`
	HCCPCIoT   bool   `json:"hc_cp_ciot"`
	N3Data     bool   `json:"n3_data"`
	CPCIoT     bool   `json:"cp_ciot"`
	RestrictEC bool   `json:"restrict_ec"`
	LPP        bool   `json:"lpp"`
	HOAttach   bool   `json:"ho_attach"`
	S1Mode     bool   `json:"s1_mode"`
	RestHex    string `json:"rest_hex,omitempty"`
}

type UEStatus struct {
	S1ModeReg bool `json:"s1_mode_reg"`
	N1ModeReg bool `json:"n1_mode_reg"`
}

type MICOIndication struct {
	RAAI  bool `json:"raai"`
	SPRTI bool `json:"sprti"`
}

type UpdateType5GS struct {
	SMSRequested bool            `json:"sms_requested"`
	NGRANRCU     bool            `json:"ng_ran_rcu"`
	PNBCIoT5GS   utils.EnumField `json:"pnb_ciot_5gs"`
	PNBCIoTEPS   utils.EnumField `json:"pnb_ciot_eps"`
}

type DRXParameter struct {
	Value utils.EnumField `json:"value"`
}

func gmmCapability(c *fgs.GMMCapability) *GMMCapability {
	if c == nil {
		return nil
	}

	return &GMMCapability{
		SGC:        c.SGC,
		HCCPCIoT:   c.HCCPCIoT,
		N3Data:     c.N3Data,
		CPCIoT:     c.CPCIoT,
		RestrictEC: c.RestrictEC,
		LPP:        c.LPP,
		HOAttach:   c.HOAttach,
		S1Mode:     c.S1Mode,
		RestHex:    hex.EncodeToString(c.Rest),
	}
}

func ueStatus(s *fgs.UEStatus) *UEStatus {
	if s == nil {
		return nil
	}

	return &UEStatus{S1ModeReg: s.S1ModeReg, N1ModeReg: s.N1ModeReg}
}

func micoIndication(m *fgs.MICOIndication) *MICOIndication {
	if m == nil {
		return nil
	}

	return &MICOIndication{RAAI: m.RAAI, SPRTI: m.SPRTI}
}

func updateType5GS(u *fgs.UpdateType5GS) *UpdateType5GS {
	if u == nil {
		return nil
	}

	return &UpdateType5GS{
		SMSRequested: u.SMSRequested,
		NGRANRCU:     u.NGRANRCU,
		PNBCIoT5GS:   utils.NamedEnum(uint8(u.PNBCIoT5GS), u.PNBCIoT5GS.String()),
		PNBCIoTEPS:   utils.NamedEnum(uint8(u.PNBCIoTEPS), u.PNBCIoTEPS.String()),
	}
}

func drxParameter(d *fgs.DRXParameter) *DRXParameter {
	if d == nil {
		return nil
	}

	return &DRXParameter{Value: utils.NamedEnum(uint8(d.Value), d.Value.String())}
}

// EPSNASMessageContainer is the complete EPS NAS message a 5GS registration
// carries for interworking (TS 24.501 §9.11.3.24).
type EPSNASMessageContainer struct {
	Protocol string             `json:"protocol"`
	RawHex   string             `json:"raw_hex"`
	Decoded  *epsdec.NASMessage `json:"decoded,omitempty"`
}

func epsNASMessageContainer(b []byte) *EPSNASMessageContainer {
	if len(b) == 0 {
		return nil
	}

	return &EPSNASMessageContainer{
		Protocol: "EPS-NAS",
		RawHex:   hex.EncodeToString(b),
		Decoded:  epsdec.DecodeEPSNASMessage(b),
	}
}
