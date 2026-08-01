// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"encoding/hex"
	"strings"

	"github.com/ellanetworks/core/internal/decoder/utils"
	"github.com/ellanetworks/core/ngap"
	"github.com/free5gc/aper"
	"github.com/free5gc/ngap/ngapType"
)

// Helpers for the procedures rendered from the in-house NGAP library. They sit
// beside the reference-decoder equivalents in util.go until every procedure has
// moved across.

// ProtocolIE-ID values the migrated renderers cite (TS 38.413, NGAP-Constants).
const (
	idAMFName                ngap.ProtocolIEID = 1
	idAMFUENGAPID            ngap.ProtocolIEID = 10
	idCause                  ngap.ProtocolIEID = 15
	idCriticalityDiagnostics ngap.ProtocolIEID = 19
	idDefaultPagingDRX       ngap.ProtocolIEID = 21
	idFiveGSTMSI             ngap.ProtocolIEID = 26
	idGlobalRANNodeID        ngap.ProtocolIEID = 27
	idPLMNSupportList        ngap.ProtocolIEID = 80
	idRANNodeName            ngap.ProtocolIEID = 82
	idRANUENGAPID            ngap.ProtocolIEID = 85
	idRelativeAMFCapacity    ngap.ProtocolIEID = 86
	idServedGUAMIList        ngap.ProtocolIEID = 96
	idSupportedTAList        ngap.ProtocolIEID = 102
	idTimeToWait             ngap.ProtocolIEID = 107
	idUERetentionInformation ngap.ProtocolIEID = 147
)

// libIE renders one modeled IE. Criticality and IE ids keep the decoder's own
// label vocabulary — the enumerations are numerically identical, so a migrated
// procedure renders exactly as it did before, and as the ones still on the
// reference decoder still do.
func libIE(id ngap.ProtocolIEID, crit ngap.Criticality, value any) IE {
	return IE{
		ID:          protocolIEIDToEnum(int64(id)),
		Criticality: criticalityToEnum(aper.Enumerated(crit)),
		Value:       value,
	}
}

// bitsHex renders the low bits of v as a left-aligned bit string in hex, the
// form bitStringToHex produces for the reference decoder.
func bitsHex(v uint64, bits int) string {
	b := make([]byte, (bits+7)/8)

	for i := range bits {
		if v&(1<<uint(bits-1-i)) != 0 {
			b[i/8] |= 1 << uint(7-i%8)
		}
	}

	return hex.EncodeToString(b)[:(bits+3)/4]
}

// unmodeledIEs renders the IEs the library preserved but does not model, so a
// capture shows what a peer sent instead of dropping it. TS 38.413 §10.3.4.2
// lets a receiver carry on past these, which is why they reach here at all.
func unmodeledIEs(raw []ngap.RawIE) []IE {
	if len(raw) == 0 {
		return nil
	}

	out := make([]IE, len(raw))
	for i, r := range raw {
		out[i] = IE{
			ID:          protocolIEIDToEnum(int64(r.ID)),
			Criticality: criticalityToEnum(aper.Enumerated(r.Criticality)),
			Value:       hex.EncodeToString(r.Value),
			ValueType:   "unmodeled",
		}
	}

	return out
}

// plmnIDToDecoder splits a PLMN identity into MCC/MNC digit strings.
func plmnIDToDecoder(id ngap.PLMNIdentity) PLMNID {
	h := strings.Split(hex.EncodeToString(id[:]), "")

	out := PLMNID{Mcc: h[1] + h[0] + h[3]}

	if h[2] == "f" {
		out.Mnc = h[5] + h[4]
	} else {
		out.Mnc = h[2] + h[5] + h[4]
	}

	return out
}

// libCause renders a cause through the decoder's existing tables. The group
// index differs by one — the reference type reserves 0 for "nothing" — and the
// value within a group is the same number.
func libCause(c ngap.Cause) utils.EnumField {
	var out ngapType.Cause

	out.Present = int(c.Group) + 1
	v := aper.Enumerated(c.Value)

	switch c.Group {
	case ngap.CauseGroupRadioNetwork:
		out.RadioNetwork = &ngapType.CauseRadioNetwork{Value: v}
	case ngap.CauseGroupTransport:
		out.Transport = &ngapType.CauseTransport{Value: v}
	case ngap.CauseGroupNAS:
		out.Nas = &ngapType.CauseNas{Value: v}
	case ngap.CauseGroupProtocol:
		out.Protocol = &ngapType.CauseProtocol{Value: v}
	case ngap.CauseGroupMisc:
		out.Misc = &ngapType.CauseMisc{Value: v}
	default:
		return utils.MakeEnum(uint64(c.Value), "", true)
	}

	return causeToEnum(out)
}

// buildLibCriticalityDiagnostics renders the library's Criticality Diagnostics.
func buildLibCriticalityDiagnostics(cd ngap.CriticalityDiagnostics) CriticalityDiagnostics {
	out := CriticalityDiagnostics{}

	if cd.ProcedureCode != nil {
		e := procedureCodeToEnum(int64(*cd.ProcedureCode))
		out.ProcedureCode = &e
	}

	if cd.TriggeringMessage != nil {
		e := triggeringMessageToString(aper.Enumerated(*cd.TriggeringMessage))
		out.TriggeringMessage = &e
	}

	if cd.ProcedureCriticality != nil {
		e := criticalityToEnum(aper.Enumerated(*cd.ProcedureCriticality))
		out.ProcedureCriticality = &e
	}

	for _, item := range cd.IEsCriticalityDiagnostics {
		out.IEsCriticalityDiagnostics = append(out.IEsCriticalityDiagnostics, IEsCriticalityDiagnostics{
			IECriticality: criticalityToEnum(aper.Enumerated(item.IECriticality)),
			IEID:          protocolIEIDToEnum(int64(item.IEID)),
			TypeOfError:   typeOfErrorToString(aper.Enumerated(item.TypeOfError)),
		})
	}

	return out
}

// pduValue returns the open-type payload of an NGAP PDU, or nil when the
// envelope does not decode — the caller's parse then reports it.
func pduValue(raw []byte) []byte {
	pdu, err := ngap.Unmarshal(raw)
	if err != nil {
		return nil
	}

	switch m := pdu.(type) {
	case *ngap.InitiatingMessage:
		return m.Value
	case *ngap.SuccessfulOutcome:
		return m.Value
	case *ngap.UnsuccessfulOutcome:
		return m.Value
	}

	return nil
}
