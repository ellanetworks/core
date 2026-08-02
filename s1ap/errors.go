// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"errors"
	"fmt"
	"strings"
)

// errNotComprehended marks a decode failure that TS 36.413 §10.3.1 classifies as
// a not-comprehended IE (cases 1, 2 and 6) rather than a transfer syntax error.
// §10.3.1 requires these to be "handled based on received Criticality
// information", so the IE table applies the IE's criticality instead of
// abandoning the message.
var errNotComprehended = errors.New("s1ap: IE not comprehended")

// TransferSyntaxError reports octets that are not a decodable PER encoding.
// Nothing of the message is recoverable (TS 36.413 §10.2).
type TransferSyntaxError struct {
	Procedure ProcedureCode
	Err       error
}

func (e *TransferSyntaxError) Error() string {
	return fmt.Sprintf("s1ap: %s: transfer syntax error: %v", e.Procedure, e.Err)
}

func (e *TransferSyntaxError) Unwrap() error { return e.Err }

// AbstractSyntaxError reports a message that decoded but whose procedure must
// be rejected: a missing reject-criticality IE (TS 36.413 §10.3.5), an
// unhandled reject-criticality IE (§10.3.4.2), or a falsely constructed
// message (§10.3.6).
//
// Cause and IEs are what the rejection must report; [AbstractSyntaxError.UEIDs]
// recovers what §10.3.5 and §10.3.6 need to address the unsuccessful outcome.
type AbstractSyntaxError struct {
	Procedure ProcedureCode
	Trigger   TriggeringMessage
	Cause     Cause
	IEs       []CriticalityDiagnosticsIEItem

	// decoded holds the modeled IEs that arrived before the rejection.
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

// ErrorIndicationDiagnostics returns the CriticalityDiagnostics to report the
// rejection with in an ERROR INDICATION. TS 36.413 §9.2.1.21 admits the
// Procedure Code and Triggering Message only here.
func (e *AbstractSyntaxError) ErrorIndicationDiagnostics() CriticalityDiagnostics {
	proc, trigger := e.Procedure, e.Trigger
	d := e.OutcomeDiagnostics()
	d.ProcedureCode, d.TriggeringMessage = &proc, &trigger

	return d
}

// OutcomeDiagnostics returns the CriticalityDiagnostics to report the rejection
// with in the procedure's own unsuccessful outcome. TS 36.413 §9.2.1.21 keeps
// the Procedure Code out of a response to the procedure that caused the error,
// and the Triggering Message out of anything but an ERROR INDICATION.
func (e *AbstractSyntaxError) OutcomeDiagnostics() CriticalityDiagnostics {
	crit := ProcedureCriticality(e.Procedure)

	return CriticalityDiagnostics{
		ProcedureCriticality:      &crit,
		IEsCriticalityDiagnostics: reportableIEs(e.IEs),
	}
}

// HasUnsuccessfulOutcome reports whether the procedure defines a message to
// reject with. TS 36.413 §10.3.5 and §10.3.6 fall back to the Error Indication
// procedure only where it does not.
func (e *AbstractSyntaxError) HasUnsuccessfulOutcome() bool {
	return hasUnsuccessfulOutcome(e.Procedure)
}

// reportableIEs drops entries TS 36.413 §9.2.1.21 forbids on the wire: of the
// IE Criticality it says "The value 'ignore' shall not be used".
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

// UEIDs returns the UE S1AP IDs that decoded before the rejection. An ERROR
// INDICATION for UE-associated signalling names the association it concerns
// with them (TS 36.413 §8.7.2.2); either is nil when it did not arrive.
func (e *AbstractSyntaxError) UEIDs() (*MMEUES1APID, *ENBUES1APID) {
	var (
		mmeID *MMEUES1APID
		enbID *ENBUES1APID
	)

	for _, ie := range e.decoded {
		switch ie.ID {
		// PATH SWITCH REQUEST identifies the association by the source MME id
		// (TS 36.413 §9.1.5.8); no message carries both.
		case idMMEUES1APID, idSourceMMEUES1APID:
			var v MMEUES1APID
			if perIEDecode(ie.Value, &v) == nil {
				mmeID = &v
			}
		case idENBUES1APID:
			var v ENBUES1APID
			if perIEDecode(ie.Value, &v) == nil {
				enbID = &v
			}
		}
	}

	return mmeID, enbID
}

// Diagnostics accumulates the abstract syntax errors that did not prevent a
// message from being delivered: IEs absent with ignore or notify criticality
// (TS 36.413 §10.3.5) and IEs that were not comprehended with the same
// (§10.3.4.2).
type Diagnostics struct {
	IEs []DiagnosticIE

	// Truncated reports that recording stopped at maxDiagnosticIEs.
	Truncated bool

	// notify records that a notify-criticality entry was seen even where
	// Truncated dropped it, so a report is never starved by ignore entries.
	notify bool
}

// DiagnosticIE names an IE an abstract syntax error concerned, and what was
// wrong with it.
type DiagnosticIE struct {
	ID          ProtocolIEID
	Criticality Criticality
	TypeOfError TypeOfError
}

// Empty reports whether the message arrived without any abstract syntax error.
func (d Diagnostics) Empty() bool { return len(d.IEs) == 0 }

// ReportRequired reports whether TS 36.413 §10.3.4.2 and §10.3.5 oblige the
// receiver to tell the sender, which only notify criticality does.
func (d Diagnostics) ReportRequired() bool { return d.notify }

// Report returns the Criticality Diagnostics entries to send back. TS 36.413
// §9.2.1.21 forbids reporting an IE Criticality of "ignore", so only the
// notify-criticality entries appear.
func (d Diagnostics) Report() []CriticalityDiagnosticsIEItem {
	out := make([]CriticalityDiagnosticsIEItem, 0, len(d.IEs))

	for _, ie := range d.IEs {
		if ie.Criticality != CriticalityNotify {
			continue
		}

		out = append(out, CriticalityDiagnosticsIEItem{
			IECriticality: ie.Criticality,
			IEID:          ie.ID,
			TypeOfError:   ie.TypeOfError,
		})
	}

	if len(out) == 0 {
		return nil
	}

	return out
}

func (d *Diagnostics) record(id ProtocolIEID, crit Criticality, kind TypeOfError) {
	entry := DiagnosticIE{ID: id, Criticality: crit, TypeOfError: kind}

	if crit == CriticalityNotify {
		d.notify = true
	}

	if len(d.IEs) < maxDiagnosticIEs {
		d.IEs = append(d.IEs, entry)

		return
	}

	d.Truncated = true

	// TS 36.413 §10.3.4.2 wants an entry per reported IE, so a notify entry
	// displaces a silent one instead of being dropped behind the bound.
	if crit != CriticalityNotify {
		return
	}

	for i, ie := range d.IEs {
		if ie.Criticality != CriticalityNotify {
			d.IEs[i] = entry

			return
		}
	}
}

func (t TypeOfError) String() string {
	switch t {
	case TypeOfErrorNotUnderstood:
		return "not-understood"
	case TypeOfErrorMissing:
		return "missing"
	default:
		return fmt.Sprintf("TypeOfError(%d)", uint8(t))
	}
}
