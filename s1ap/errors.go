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
// A parser returns this error whenever a mandatory IE is absent, whatever its
// criticality, together with the partially-decoded message. Callers that treat
// any error as fatal therefore never read an unpopulated mandatory field. A
// caller that wants §10.3.5's "ignore" behaviour — carry on using the IEs that
// are present — opts in explicitly by checking that nothing reject-criticality
// is missing:
//
//	msg, err := s1ap.ParseUplinkNASTransport(value)
//
//	var missing *s1ap.MissingMandatoryIEsError
//	switch {
//	case errors.As(err, &missing) && len(missing.RejectedIEs()) == 0:
//		// §10.3.5 "Ignore IE": continue from the IEs that are present,
//		// remembering that the ones in missing.IEs are not.
//	case err != nil:
//		return err
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

// requireIEs reports any mandatory IE the message did not carry. Absence is
// always an error, whatever the IE's criticality: a decoded message must have
// every mandatory field populated, so the codec never hands back a struct in
// which a mandatory IE is silently the zero value. Deciding what to do about
// the absence — §10.3.5's reject-versus-continue — is the receiving node's
// call, made from the returned error.
//
// Callers list one check per mandatory IE of the message, in the order the
// §9.1 table gives them, with the criticality that table assigns.
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
