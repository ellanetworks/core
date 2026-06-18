// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"encoding/hex"
	"fmt"
	"net"

	"github.com/ellanetworks/core/internal/decoder/eps"
	"github.com/ellanetworks/core/internal/decoder/utils"
	"github.com/ellanetworks/core/s1ap"
)

// transportLayerAddress renders an S1AP Transport Layer Address (TS 36.413
// §9.2.2.1) — the S1-U endpoint — as a dotted IPv4 / IPv6 address (or both),
// matching the NGAP decoder. An unexpected length falls back to hex so no
// information is lost.
func transportLayerAddress(b []byte) string {
	switch len(b) {
	case 4, 16:
		return net.IP(b).String()
	case 20: // IPv4 followed by IPv6
		return net.IP(b[:4]).String() + "+" + net.IP(b[4:]).String()
	default:
		return hex.EncodeToString(b)
	}
}

// NASPDU carries a NAS message embedded in an S1AP IE: the raw hex plus the
// decoded EPS NAS (EMM/ESM). The protocol marker lets the shared UI renderer
// detect a NAS-PDU.
type NASPDU struct {
	Protocol string          `json:"protocol"` // always "NAS"
	RawHex   string          `json:"raw_hex"`
	Decoded  *eps.NASMessage `json:"decoded,omitempty"`
}

func nasPDU(b []byte) NASPDU {
	return NASPDU{Protocol: "NAS", RawHex: hex.EncodeToString(b), Decoded: eps.DecodeEPSNASMessage(b)}
}

// nasSummary returns a short "NAS=<type>" fragment for a message summary, or ""
// when the NAS message type is not available.
func nasSummary(b []byte) string {
	d := eps.DecodeEPSNASMessage(b)
	if d == nil {
		return ""
	}

	if d.EMMMessage != nil && d.EMMMessage.EMMHeader.MessageType.Label != "" {
		return ", NAS=" + d.EMMMessage.EMMHeader.MessageType.Label
	}

	if d.ESMMessage != nil && d.ESMMessage.ESMHeader.MessageType.Label != "" {
		return ", NAS=" + d.ESMMessage.ESMHeader.MessageType.Label
	}

	if d.Encrypted {
		return ", NAS=encrypted"
	}

	return ""
}

// TAI is a Tracking Area Identity (TS 36.413 §9.2.3.16).
type TAI struct {
	PLMNID PLMNID `json:"plmn_id"`
	TAC    uint16 `json:"tac"`
}

func tai(t s1ap.TAI) TAI {
	return TAI{PLMNID: plmnToID(t.PLMNIdentity), TAC: uint16(t.TAC)}
}

// EUTRANCGI is an E-UTRAN Cell Global Identifier (TS 36.413 §9.2.1.38).
type EUTRANCGI struct {
	PLMNID PLMNID `json:"plmn_id"`
	CellID string `json:"cell_id"` // 28-bit E-UTRAN cell identity, hex
}

func eutranCGI(c s1ap.EUTRANCGI) EUTRANCGI {
	return EUTRANCGI{PLMNID: plmnToID(c.PLMNIdentity), CellID: fmt.Sprintf("%07x", c.CellID)}
}

// STMSI is an S-Temporary Mobile Subscriber Identity (TS 36.413 §9.2.3.6).
type STMSI struct {
	MMEC  uint8  `json:"mmec"`
	MTMSI uint32 `json:"m_tmsi"`
}

func stmsi(s s1ap.STMSI) STMSI {
	return STMSI{MMEC: s.MMEC, MTMSI: s.MTMSI}
}

// UES1APIDs is the UE-associated identity pair (TS 36.413 §9.2.3.13).
type UES1APIDs struct {
	MMEUES1APID uint32 `json:"mme_ue_s1ap_id"`
	ENBUES1APID uint32 `json:"enb_ue_s1ap_id"`
}

func ues1apIDs(ids s1ap.UES1APIDs) UES1APIDs {
	return UES1APIDs{MMEUES1APID: uint32(ids.MMEUES1APID), ENBUES1APID: uint32(ids.ENBUES1APID)}
}

// CriticalityDiagnostics is the decoded CriticalityDiagnostics IE (TS 36.413
// §9.2.1.4): which procedure/message triggered the diagnostic and the offending
// IEs. Absent sub-fields are omitted.
type CriticalityDiagnostics struct {
	ProcedureCode        *int64                     `json:"procedure_code,omitempty"`
	TriggeringMessage    *utils.EnumField[uint64]   `json:"triggering_message,omitempty"`
	ProcedureCriticality *utils.EnumField[uint64]   `json:"procedure_criticality,omitempty"`
	IEs                  []CriticalityDiagnosticsIE `json:"ies,omitempty"`
}

// CriticalityDiagnosticsIE reports one offending IE (TS 36.413 §9.2.1.4).
type CriticalityDiagnosticsIE struct {
	IEID        int64                   `json:"ie_id"`
	Criticality utils.EnumField[uint64] `json:"criticality"`
	TypeOfError utils.EnumField[uint64] `json:"type_of_error"`
}

func triggeringMessageToEnum(t s1ap.TriggeringMessage) utils.EnumField[uint64] {
	switch t {
	case s1ap.TriggeringInitiatingMessage:
		return utils.MakeEnum(uint64(t), "initiating-message", false)
	case s1ap.TriggeringSuccessfulOutcome:
		return utils.MakeEnum(uint64(t), "successful-outcome", false)
	case s1ap.TriggeringUnsuccessfulOutcome:
		return utils.MakeEnum(uint64(t), "unsuccessful-outcome", false)
	default:
		return utils.MakeEnum(uint64(t), "", true)
	}
}

func typeOfErrorToEnum(t s1ap.TypeOfError) utils.EnumField[uint64] {
	switch t {
	case s1ap.TypeOfErrorNotUnderstood:
		return utils.MakeEnum(uint64(t), "not-understood", false)
	case s1ap.TypeOfErrorMissing:
		return utils.MakeEnum(uint64(t), "missing", false)
	default:
		return utils.MakeEnum(uint64(t), "", true)
	}
}

func criticalityDiagnostics(d s1ap.CriticalityDiagnostics) CriticalityDiagnostics {
	out := CriticalityDiagnostics{}

	if d.ProcedureCode != nil {
		pc := int64(*d.ProcedureCode)
		out.ProcedureCode = &pc
	}

	if d.TriggeringMessage != nil {
		tm := triggeringMessageToEnum(*d.TriggeringMessage)
		out.TriggeringMessage = &tm
	}

	if d.ProcedureCriticality != nil {
		pc := criticalityToEnum(*d.ProcedureCriticality)
		out.ProcedureCriticality = &pc
	}

	for _, it := range d.IEsCriticalityDiagnostics {
		out.IEs = append(out.IEs, CriticalityDiagnosticsIE{
			IEID:        int64(it.IEID),
			Criticality: criticalityToEnum(it.IECriticality),
			TypeOfError: typeOfErrorToEnum(it.TypeOfError),
		})
	}

	return out
}

// GUMMEI is a Globally Unique MME Identifier (TS 36.413 §9.2.3.9): the eNB's
// selected MME.
type GUMMEI struct {
	PLMNID     PLMNID `json:"plmn_id"`
	MMEGroupID uint16 `json:"mme_group_id"`
	MMECode    uint8  `json:"mme_code"`
}

func gummei(g s1ap.GUMMEI) GUMMEI {
	return GUMMEI{
		PLMNID:     plmnToID(g.PLMNIdentity),
		MMEGroupID: uint16(g.MMEGroupID[0])<<8 | uint16(g.MMEGroupID[1]),
		MMECode:    uint8(g.MMECode),
	}
}

func cnDomainToEnum(d s1ap.CNDomain) utils.EnumField[uint64] {
	switch d {
	case s1ap.CNDomainPS:
		return utils.MakeEnum(uint64(d), "ps", false)
	case s1ap.CNDomainCS:
		return utils.MakeEnum(uint64(d), "cs", false)
	default:
		return utils.MakeEnum(uint64(d), "", true)
	}
}

func rrcCauseToEnum(c s1ap.RRCEstablishmentCause) utils.EnumField[uint64] {
	names := map[s1ap.RRCEstablishmentCause]string{
		s1ap.RRCCauseEmergency:          "emergency",
		s1ap.RRCCauseHighPriorityAccess: "highPriorityAccess",
		s1ap.RRCCauseMTAccess:           "mt-Access",
		s1ap.RRCCauseMOSignalling:       "mo-Signalling",
		s1ap.RRCCauseMOData:             "mo-Data",
	}

	name, ok := names[c]

	return utils.MakeEnum(uint64(c), name, !ok)
}

// ie is a small constructor for an Information Element with a spec-fixed
// criticality.
func ie(id int64, crit s1ap.Criticality, value any) IE {
	return IE{ID: ieEnum(id), Criticality: criticalityToEnum(crit), Value: value}
}

// rawIEValue renders an unmodeled IE's open-type contents (TS 36.413 §9.3) as
// hex, so the value is shown rather than lost.
type rawIEValue struct {
	Hex string `json:"hex"`
}

// appendUnknownIEs appends a message's protocol IEs that the s1ap library does
// not model, as raw {id, criticality, hex} entries. This keeps an IE that is
// present on the wire visible (with its id flagged unknown) instead of silently
// dropping it.
func appendUnknownIEs(ies []IE, raw []s1ap.RawIE) []IE {
	for _, r := range raw {
		ies = append(ies, ie(int64(r.ID), r.Criticality, rawIEValue{Hex: hex.EncodeToString(r.Value)}))
	}

	return ies
}
