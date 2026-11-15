// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"fmt"
	"strings"

	"github.com/ellanetworks/core/per/ap"
)

// Octets that are not a decodable PER encoding; nothing of the message is
// recoverable (TS 38.413 §10.2).
type TransferSyntaxError = ap.TransferSyntaxError[ProcedureCode]

// A message that decoded but whose procedure must be rejected: a missing
// reject-criticality IE (TS 38.413 §10.3.5), an unhandled reject-criticality IE
// (§10.3.4.2), or a falsely constructed message (§10.3.6).
type AbstractSyntaxError struct {
	Procedure ProcedureCode
	Trigger   TriggeringMessage
	Cause     Cause
	IEs       []CriticalityDiagnosticsIEItem

	// Modeled IEs that arrived before the rejection.
	decoded []RawIE
}

func (e *AbstractSyntaxError) Error() string {
	if len(e.IEs) == 0 {
		return fmt.Sprintf("ngap: %s: %s", e.Procedure, e.Cause)
	}

	ids := make([]string, len(e.IEs))
	for i, ie := range e.IEs {
		ids[i] = fmt.Sprintf("%s (%s, %s)", ie.IEID, ie.IECriticality, ie.TypeOfError)
	}

	return fmt.Sprintf("ngap: %s: %s: %s", e.Procedure, e.Cause, strings.Join(ids, ", "))
}

// §9.3.1.3 admits the Procedure Code and Triggering Message only in an
// ERROR INDICATION.
func (e *AbstractSyntaxError) ErrorIndicationDiagnostics() CriticalityDiagnostics {
	proc, trigger := e.Procedure, e.Trigger
	d := e.OutcomeDiagnostics()
	d.ProcedureCode, d.TriggeringMessage = &proc, &trigger

	return d
}

// §9.3.1.3 keeps the Procedure Code out of a response to the procedure that
// caused the error, and the Triggering Message out of all but an ERROR
// INDICATION.
func (e *AbstractSyntaxError) OutcomeDiagnostics() CriticalityDiagnostics {
	crit := ProcedureCriticality(e.Procedure)

	return CriticalityDiagnostics{
		ProcedureCriticality:      &crit,
		IEsCriticalityDiagnostics: reportableIEs(e.IEs),
	}
}

// §10.3.5 and §10.3.6 fall back to the Error Indication procedure only where
// the procedure defines no message to reject with.
func (e *AbstractSyntaxError) HasUnsuccessfulOutcome() bool {
	return hasUnsuccessfulOutcome(e.Procedure)
}

// Of the IE Criticality, §9.3.1.3 says "The value 'ignore' is not applicable".
func reportableIEs(ies []CriticalityDiagnosticsIEItem) []CriticalityDiagnosticsIEItem {
	out := make([]CriticalityDiagnosticsIEItem, 0, len(ies))

	for _, ie := range ies {
		if ie.IECriticality == CriticalityIgnore {
			continue
		}

		out = append(out, ie)
	}

	if len(out) == 0 {
		return nil
	}

	return out
}

// An ERROR INDICATION for UE-associated signalling names the association it
// concerns with these (TS 38.413 §8.4.4.2).
func (e *AbstractSyntaxError) UEIDs() (*AMFUENGAPID, *RANUENGAPID) {
	var (
		amfID *AMFUENGAPID
		ranID *RANUENGAPID
	)

	for _, ie := range e.decoded {
		switch ie.ID {
		// PATH SWITCH REQUEST identifies the association by the source AMF id
		// (TS 38.413 §9.2.3.8); no message carries both. id-NewAMF-UE-NGAP-ID
		// is deliberately absent: UE CONTEXT MODIFICATION REQUEST carries it
		// alongside id-AMF-UE-NGAP-ID to assign a new id, so it names the
		// association being moved to rather than the one to answer about.
		case IDAMFUENGAPID, IDSourceAMFUENGAPID:
			var v AMFUENGAPID
			if perIEDecode(ie.Value, &v) == nil {
				amfID = &v
			}
		case IDRANUENGAPID:
			var v RANUENGAPID
			if perIEDecode(ie.Value, &v) == nil {
				ranID = &v
			}
		}
	}

	return amfID, ranID
}

type Diagnostics struct {
	ap.Diagnostics[ProtocolIEID]
}

type DiagnosticIE = ap.Diagnostic[ProtocolIEID]

// §9.3.1.3 forbids reporting an IE Criticality of "ignore".
func (d Diagnostics) Report() []CriticalityDiagnosticsIEItem {
	return criticalityDiagnosticsItems(d.Notify())
}

func (t TypeOfError) String() string { return t.Name() }

// PathSwitchSessions returns the sessions a rejected PATH SWITCH REQUEST asked
// to switch. §9.2.3.10 makes the released list mandatory in PATH SWITCH REQUEST
// FAILURE, so that message can only be built where this list survived the
// decode; §10.3.5 falls back to the Error Indication procedure where it did not.
func (e *AbstractSyntaxError) PathSwitchSessions() PDUSessionResourceToBeSwitchedDLList {
	for _, ie := range e.decoded {
		if ie.ID != IDPDUSessionResourceToBeSwitchedDLList {
			continue
		}

		var v PDUSessionResourceToBeSwitchedDLList
		if perIEDecode(ie.Value, &v) == nil {
			return v
		}
	}

	return nil
}
