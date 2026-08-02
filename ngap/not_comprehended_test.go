// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"errors"
	"testing"

	"github.com/ellanetworks/core/per"
)

// choiceExtensionValue encodes a CHOICE that selects its choice-Extensions
// alternative, carrying one ProtocolIE-SingleContainer entry.
func choiceExtensionValue(t *testing.T, alternatives, index int, id ProtocolIEID) []byte {
	t.Helper()

	w := per.NewWriter()
	if err := per.EncodeConstrainedWholeNumber(w, per.Aligned, 0, int64(alternatives-1), int64(index)); err != nil {
		t.Fatal(err)
	}

	if err := encodeContainerField(w, per.Aligned, ieField{
		id: id, crit: CriticalityReject, raw: []byte{0x00},
	}); err != nil {
		t.Fatal(err)
	}

	return perBytes(w)
}

// TS 38.413 §10.3.1 case 6 — "receives IEs or IE groups for a functionality that
// is not supported" — is an abstract syntax error "handled based on received
// Criticality information", not a transfer syntax error. For a reject IE that
// means the procedure is rejected via its unsuccessful outcome, so the peer gets
// an NG SETUP FAILURE rather than an ERROR INDICATION.
func TestChoiceExtensionOnRejectIEIsAbstractSyntaxError(t *testing.T) {
	// id-GlobalRANNodeID is CRITICALITY reject in NGSetupRequestIEs. A TNGF,
	// TWIF or W-AGF identifies itself through GlobalRANNodeID's
	// choice-Extensions, which this library does not model.
	value := container(t, ieField{
		id:   idGlobalRANNodeID,
		crit: CriticalityReject,
		raw:  choiceExtensionValue(t, globalRANNodeIDAlternatives, globalRANNodeIDChoiceExtensions, idGlobalRANNodeID),
	})

	_, err := ParseNGSetupRequest(value)
	if err == nil {
		t.Fatal("parse succeeded, want a rejection")
	}

	var ase *AbstractSyntaxError
	if !errors.As(err, &ase) {
		t.Fatalf("error = %T (%v), want *AbstractSyntaxError so the reject is reported in NG SETUP FAILURE", err, err)
	}

	if ase.Cause.Value != CauseProtocolAbstractSyntaxErrorReject {
		t.Errorf("cause = %s, want abstract-syntax-error-reject", ase.Cause)
	}

	if len(ase.IEs) != 1 || ase.IEs[0].IEID != idGlobalRANNodeID ||
		ase.IEs[0].TypeOfError != TypeOfErrorNotUnderstood {
		t.Errorf("diagnostics = %+v, want one not-understood entry for GlobalRANNodeID", ase.IEs)
	}
}

// The same construct on an ignore IE must not cost the message: §10.3.4.2 says
// to ignore the IE's content, report it, and continue with the procedure.
func TestChoiceExtensionOnIgnoreIEIsIgnored(t *testing.T) {
	// id-Cause is CRITICALITY ignore in NGResetIEs.
	value := container(t,
		ieField{
			id:   idCause,
			crit: CriticalityIgnore,
			raw:  choiceExtensionValue(t, causeAlternatives, causeChoiceExtensions, idCause),
		},
		ieField{id: idResetType, crit: CriticalityReject, val: ResetType{All: true}},
	)

	msg, err := ParseNGReset(value)
	if err != nil {
		t.Fatalf("parse: %v, want the message delivered with the IE ignored", err)
	}

	if msg.Cause != nil {
		t.Errorf("Cause = %v, want nil (content ignored)", msg.Cause)
	}

	if !msg.ResetType.All {
		t.Error("ResetType lost, want the rest of the message still decoded")
	}

	var reported bool

	for _, ie := range msg.Diagnostics().IEs {
		if ie.ID == idCause && ie.TypeOfError == TypeOfErrorNotUnderstood {
			reported = true
		}
	}

	if !reported {
		t.Errorf("not-comprehended Cause missing from diagnostics: %+v", msg.Diagnostics().IEs)
	}
}

// TS 38.413 §10.3.1 has the receiver "read the remaining message and ... then
// for each detected Abstract Syntax Error" act, and §10.3.4.2 wants a
// Criticality Diagnostics entry "for each reported IE/IE group" — so two
// not-comprehended reject IEs must produce two entries, not just the first.
func TestAllNotComprehendedRejectIEsAreReported(t *testing.T) {
	value := container(t,
		ieField{id: ProtocolIEID(40001), crit: CriticalityReject, raw: []byte{0x00}},
		ieField{id: ProtocolIEID(40002), crit: CriticalityReject, raw: []byte{0x00}},
	)

	_, err := ParseNGSetupRequest(value)

	var ase *AbstractSyntaxError
	if !errors.As(err, &ase) {
		t.Fatalf("error = %T (%v), want *AbstractSyntaxError", err, err)
	}

	if len(ase.IEs) != 2 {
		t.Fatalf("reported %d IEs, want 2: %+v", len(ase.IEs), ase.IEs)
	}

	for i, want := range []ProtocolIEID{40001, 40002} {
		if ase.IEs[i].IEID != want || ase.IEs[i].TypeOfError != TypeOfErrorNotUnderstood {
			t.Errorf("entry %d = %+v, want not-understood %s", i, ase.IEs[i], want)
		}
	}
}
