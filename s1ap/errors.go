// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"errors"
	"fmt"
	"strings"
)

// A decode failure TS 36.413 §10.3.1 classifies as a not-comprehended IE (cases
// 1, 2 and 6), handled on criticality rather than by abandoning the message.
var errNotComprehended = errors.New("s1ap: IE not comprehended")

// §10.3.2 treats a not-comprehended item on its own criticality. For an
// unmodeled extension that item is the extension, not the IE containing it, so
// a reject extension rejects the procedure even inside an ignore IE. Decoders
// whose item is the containing IE return the bare sentinel instead.
type notComprehendedIE struct {
	ID   ProtocolIEID
	Crit Criticality
	What string
}

func (e *notComprehendedIE) Error() string {
	return fmt.Sprintf("%s: %s %s (criticality %s)", errNotComprehended, e.What, e.ID, e.Crit)
}

func (e *notComprehendedIE) Unwrap() error { return errNotComprehended }

// Octets that are not a decodable PER encoding; nothing of the message is
// recoverable (TS 36.413 §10.2).
type TransferSyntaxError struct {
	Procedure ProcedureCode
	Err       error
}

func (e *TransferSyntaxError) Error() string {
	return fmt.Sprintf("s1ap: %s: transfer syntax error: %v", e.Procedure, e.Err)
}

func (e *TransferSyntaxError) Unwrap() error { return e.Err }

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

// The abstract syntax errors that did not prevent delivery: IEs absent, or not
// comprehended, with ignore or notify criticality (TS 36.413 §10.3.5,
// §10.3.4.2).
//
// No IE in these tables carries notify criticality, so the notify paths below
// never fire today; they implement the protocol's rule, not this table's.
type Diagnostics struct {
	IEs []DiagnosticIE

	// Truncated reports that recording stopped at maxDiagnosticIEs.
	Truncated bool

	// Set even where Truncated dropped the entry, so a report is never starved
	// by ignore entries.
	notify bool
}

type DiagnosticIE struct {
	ID          ProtocolIEID
	Criticality Criticality
	TypeOfError TypeOfError
}

func (d Diagnostics) Empty() bool { return len(d.IEs) == 0 }

// Only notify criticality obliges the receiver to tell the sender
// (§10.3.4.2, §10.3.5).
func (d Diagnostics) ReportRequired() bool { return d.notify }

// §9.2.1.21 forbids reporting an IE Criticality of "ignore".
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

	// §10.3.4.2 wants an entry per reported IE, so a notify entry displaces a
	// silent one rather than being dropped behind the bound.
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
	if name, ok := typeOfErrorNames[t]; ok {
		return name
	}

	return fmt.Sprintf("TypeOfError(%d)", uint8(t))
}
