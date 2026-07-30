// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"fmt"
	"strings"
)

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
// Cause and IEs are what the rejection must report. Decoded carries the IEs
// that did arrive, which §10.3.5 and §10.3.6 require when the unsuccessful
// outcome cannot be addressed without them.
type AbstractSyntaxError struct {
	Procedure ProcedureCode
	Trigger   TriggeringMessage
	Cause     Cause
	IEs       []CriticalityDiagnosticsIEItem
	Decoded   []RawIE
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

// Diagnostics returns the CriticalityDiagnostics a receiver reports the
// rejection with (TS 36.413 §10.3.5).
func (e *AbstractSyntaxError) Diagnostics() CriticalityDiagnostics {
	proc, trigger, crit := e.Procedure, e.Trigger, CriticalityReject

	return CriticalityDiagnostics{
		ProcedureCode:             &proc,
		TriggeringMessage:         &trigger,
		ProcedureCriticality:      &crit,
		IEsCriticalityDiagnostics: e.IEs,
	}
}

// UEIDs returns the UE S1AP IDs that decoded before the rejection. An ERROR
// INDICATION for UE-associated signalling names the association it concerns
// with them (TS 36.413 §8.7.2.2); either is nil when it did not arrive.
func (e *AbstractSyntaxError) UEIDs() (*MMEUES1APID, *ENBUES1APID) {
	var (
		mmeID *MMEUES1APID
		enbID *ENBUES1APID
	)

	for _, ie := range e.Decoded {
		switch ie.ID {
		case idMMEUES1APID:
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
	IEs []CriticalityDiagnosticsIEItem

	// Truncated reports that recording stopped at maxDiagnosticIEs.
	Truncated bool
}

// Empty reports whether the message arrived without any abstract syntax error.
func (d Diagnostics) Empty() bool { return len(d.IEs) == 0 }

// ReportRequired reports whether TS 36.413 §10.3.4.2 and §10.3.5 oblige the
// receiver to tell the sender, which only notify criticality does.
func (d Diagnostics) ReportRequired() bool {
	for _, ie := range d.IEs {
		if ie.IECriticality == CriticalityNotify {
			return true
		}
	}

	return false
}

func (d *Diagnostics) record(id ProtocolIEID, crit Criticality, kind TypeOfError) {
	if len(d.IEs) >= maxDiagnosticIEs {
		d.Truncated = true

		return
	}

	d.IEs = append(d.IEs, CriticalityDiagnosticsIEItem{
		IECriticality: crit,
		IEID:          id,
		TypeOfError:   kind,
	})
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
