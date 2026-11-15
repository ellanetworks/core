// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"fmt"
	"strings"

	"github.com/ellanetworks/core/per/ap"
)

// Octets that are not a decodable PER encoding; nothing of the message is
// recoverable (TS 36.413 §10.2).
type TransferSyntaxError = ap.TransferSyntaxError[ProcedureCode]

// A message that decoded but whose procedure must be rejected: a missing
// reject-criticality IE (TS 36.413 §10.3.5), an unhandled reject-criticality IE
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
		return fmt.Sprintf("s1ap: %s: %s", e.Procedure, e.Cause)
	}

	ids := make([]string, len(e.IEs))
	for i, ie := range e.IEs {
		ids[i] = fmt.Sprintf("%s (%s, %s)", ie.IEID, ie.IECriticality, ie.TypeOfError)
	}

	return fmt.Sprintf("s1ap: %s: %s: %s", e.Procedure, e.Cause, strings.Join(ids, ", "))
}

// §9.2.1.21 admits the Procedure Code and Triggering Message only in an
// ERROR INDICATION.
func (e *AbstractSyntaxError) ErrorIndicationDiagnostics() CriticalityDiagnostics {
	proc, trigger := e.Procedure, e.Trigger
	d := e.OutcomeDiagnostics()
	d.ProcedureCode, d.TriggeringMessage = &proc, &trigger

	return d
}

// §9.2.1.21 keeps the Procedure Code out of a response to the procedure that
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

// Of the IE Criticality, §9.2.1.21 says "The value 'ignore' shall not be used".
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
// concerns with these (TS 36.413 §8.7.2.2).
func (e *AbstractSyntaxError) UEIDs() (*MMEUES1APID, *ENBUES1APID) {
	var (
		mmeID *MMEUES1APID
		enbID *ENBUES1APID
	)

	for _, ie := range e.decoded {
		switch ie.ID {
		// PATH SWITCH REQUEST identifies the association by the source MME id
		// (TS 36.413 §9.1.5.8); no message carries both.
		case IDMMEUES1APID, IDSourceMMEUES1APID:
			var v MMEUES1APID
			if perIEDecode(ie.Value, &v) == nil {
				mmeID = &v
			}
		case IDENBUES1APID:
			var v ENBUES1APID
			if perIEDecode(ie.Value, &v) == nil {
				enbID = &v
			}
		}
	}

	return mmeID, enbID
}

type Diagnostics struct {
	ap.Diagnostics[ProtocolIEID]
}

type DiagnosticIE = ap.Diagnostic[ProtocolIEID]

// §9.2.1.21 forbids reporting an IE Criticality of "ignore".
func (d Diagnostics) Report() []CriticalityDiagnosticsIEItem {
	return criticalityDiagnosticsItems(d.Notify())
}

func (t TypeOfError) String() string { return t.Name() }
