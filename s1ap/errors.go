// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"fmt"
	"strings"
)

// MissingIE names an IE a message did not carry, with the criticality
// TS 36.413 §9.1 assigns it there.
type MissingIE struct {
	ID          ProtocolIEID
	Criticality Criticality
}

// MissingMandatoryIEsError reports the mandatory IEs a message did not carry.
// Its fields are what a receiver needs to answer with Criticality Diagnostics
// (TS 36.413 §10.3.5):
//
//	var missing *s1ap.MissingMandatoryIEsError
//	if errors.As(err, &missing) {
//		return abstractSyntaxError(missing.Procedure, missing.IEs)
//	}
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

// RejectedIEs returns the missing IEs the message table marks reject.
func (e *MissingMandatoryIEsError) RejectedIEs() []MissingIE {
	var out []MissingIE

	for _, ie := range e.IEs {
		if ie.Criticality == CriticalityReject {
			out = append(out, ie)
		}
	}

	return out
}

type ieCheck struct {
	id   ProtocolIEID
	crit Criticality
	seen bool
}

// requireIEs errors on any absent mandatory IE, including ignore-criticality
// ones that TS 36.413 §10.3.5 says to carry on without: a mandatory field must
// never read back as an unset zero value.
func requireIEs(procedure ProcedureCode, checks ...ieCheck) error {
	var missing []MissingIE

	for _, c := range checks {
		if !c.seen {
			missing = append(missing, MissingIE{ID: c.id, Criticality: c.crit})
		}
	}

	if len(missing) == 0 {
		return nil
	}

	return &MissingMandatoryIEsError{Procedure: procedure, IEs: missing}
}
