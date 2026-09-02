// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"encoding/hex"

	epsdec "github.com/ellanetworks/core/internal/decoder/eps"
	"github.com/ellanetworks/core/internal/decoder/utils"
	naslib "github.com/ellanetworks/core/nas"
	"github.com/ellanetworks/core/nas/eps"
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

type EPSBearerContextStatusItem struct {
	EPSBearerIdentity int  `json:"eps_bearer_identity"`
	Active            bool `json:"active"`
}

func epsBearerContextStatus(s *naslib.EPSBearerContextStatus) []EPSBearerContextStatusItem {
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

// RawOctets is an element carried through as bytes: its contents belong to
// another protocol (an EAP packet, RFC 3748) or are a blob the network does not
// interpret (the SOR container, additional information). Hex matches how every
// other raw value in the decoders is rendered.
type RawOctets struct {
	Hex string `json:"hex"`
}

func rawOctets(b []byte) *RawOctets {
	if len(b) == 0 {
		return nil
	}

	return &RawOctets{Hex: hex.EncodeToString(b)}
}

// S1UENetworkCapability is the E-UTRA capability a 5GS UE replays so the network
// can move it to S1 mode (TS 24.501 §9.11.3.48, deferring to TS 24.301
// §9.9.3.34). Hex keeps the octets; the lists read them.
type S1UENetworkCapability struct {
	Hex string   `json:"hex"`
	EEA []string `json:"eea,omitempty"`
	EIA []string `json:"eia,omitempty"`
	UEA []string `json:"uea,omitempty"`
	UIA []string `json:"uia,omitempty"`
	// UCS2NoPreference is the octet 6 bit 8 of TS 24.301 §9.9.3.34: set means the
	// UE has no preference between the default alphabet and UCS2, clear means it
	// prefers the default alphabet. It is not a statement of UCS2 support.
	UCS2NoPreference bool   `json:"ucs2_no_preference,omitempty"`
	Error            string `json:"error,omitempty"`
}

func s1UENetworkCapability(b []byte) *S1UENetworkCapability {
	if len(b) == 0 {
		return nil
	}

	out := &S1UENetworkCapability{Hex: hex.EncodeToString(b)}

	capability, err := eps.ParseUENetworkCapability(b)
	if err != nil {
		out.Error = err.Error()

		return out
	}

	out.EEA = algorithmNames("EEA", capability.EEA)
	out.EIA = algorithmNames("EIA", capability.EIA)

	if capability.HasUMTS {
		out.UEA = algorithmNames("UEA", capability.UEA)
		out.UIA = algorithmNames("UIA", capability.UIA)
		out.UCS2NoPreference = capability.UCS2
	}

	return out
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
