// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"fmt"
	"strings"
)

// MissingIE names an IE that a message did not carry, together with the
// criticality TS 36.413 assigns it in that message's §9.1 table.
type MissingIE struct {
	ID          ProtocolIEID
	Criticality Criticality
}

// MissingMandatoryIEsError reports the mandatory IEs absent from a decoded
// message. It names the procedure and every missing IE with its criticality,
// which is what a receiver needs to build Criticality Diagnostics
// (TS 36.413 §10.3.5).
//
// A parser returns this error only when the procedure must be rejected, i.e.
// when at least one absent IE has reject criticality. Absences that are
// exclusively ignore-criticality are not errors: §10.3.5 requires the receiver
// to carry on, and the affected fields are simply left at their zero value or
// nil. When the error is returned it lists every missing mandatory IE, ignore
// ones included, so the diagnostics can be complete.
type MissingMandatoryIEsError struct {
	Procedure ProcedureCode
	IEs       []MissingIE
}

func (e *MissingMandatoryIEsError) Error() string {
	ids := make([]string, len(e.IEs))
	for i, ie := range e.IEs {
		ids[i] = fmt.Sprintf("%s (%s)", ie.ID, ie.Criticality)
	}

	return fmt.Sprintf("s1ap: %s missing mandatory IE(s): %s", e.Procedure, strings.Join(ids, ", "))
}

// RejectedIEs returns the missing IEs whose criticality requires rejecting the
// procedure, in the order the message table lists them.
func (e *MissingMandatoryIEsError) RejectedIEs() []MissingIE {
	var out []MissingIE

	for _, ie := range e.IEs {
		if ie.Criticality == CriticalityReject {
			out = append(out, ie)
		}
	}

	return out
}

// ieCheck records whether one mandatory IE was present, with the criticality
// its message table assigns it.
type ieCheck struct {
	id   ProtocolIEID
	crit Criticality
	seen bool
}

// requireIEs applies TS 36.413 §10.3.5 to a decoded message: it returns a
// [MissingMandatoryIEsError] when a reject-criticality IE is absent, and nil
// when every absence is ignore-criticality (the receiver must then carry on).
// The returned error lists all missing mandatory IEs, not just the rejecting
// ones, so the caller can report complete diagnostics.
//
// Callers list one check per mandatory IE of the message, in the order the
// §9.1 table gives them, with the criticality that table assigns.
func requireIEs(procedure ProcedureCode, checks ...ieCheck) error {
	var (
		missing []MissingIE
		reject  bool
	)

	for _, c := range checks {
		if c.seen {
			continue
		}

		missing = append(missing, MissingIE{ID: c.id, Criticality: c.crit})

		if c.crit == CriticalityReject {
			reject = true
		}
	}

	if !reject {
		return nil
	}

	return &MissingMandatoryIEsError{Procedure: procedure, IEs: missing}
}
