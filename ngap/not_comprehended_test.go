// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"errors"
	"testing"

	"github.com/ellanetworks/core/per"
)

// A CHOICE selecting its choice-Extensions alternative, carrying one entry.
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

// §10.3.1 case 6 is handled on criticality, so a reject IE rejects via the
// procedure's unsuccessful outcome rather than an ERROR INDICATION.
func TestChoiceExtensionOnRejectIEIsAbstractSyntaxError(t *testing.T) {
	// A TNGF, TWIF or W-AGF identifies itself through GlobalRANNodeID's
	// choice-Extensions, which this library does not model.
	value := container(t, ieField{
		id:   IDGlobalRANNodeID,
		crit: CriticalityReject,
		raw:  choiceExtensionValue(t, globalRANNodeIDAlternatives, globalRANNodeIDChoiceExtensions, IDGlobalRANNodeID),
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

	if len(ase.IEs) != 1 || ase.IEs[0].IEID != IDGlobalRANNodeID ||
		ase.IEs[0].TypeOfError != TypeOfErrorNotUnderstood {
		t.Errorf("diagnostics = %+v, want one not-understood entry for GlobalRANNodeID", ase.IEs)
	}
}

// §10.3.4.2: ignore the content, report it, continue with the procedure.
func TestChoiceExtensionOnIgnoreIEIsIgnored(t *testing.T) {
	// id-Cause is CRITICALITY ignore in NGResetIEs.
	value := container(t,
		ieField{
			id:   IDCause,
			crit: CriticalityIgnore,
			raw:  choiceExtensionValue(t, causeAlternatives, causeChoiceExtensions, IDCause),
		},
		ieField{id: IDResetType, crit: CriticalityReject, val: ResetType{All: true}},
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
		if ie.ID == IDCause && ie.TypeOfError == TypeOfErrorNotUnderstood {
			reported = true
		}
	}

	if !reported {
		t.Errorf("not-comprehended Cause missing from diagnostics: %+v", msg.Diagnostics().IEs)
	}
}

// §10.3.4.2 wants an entry "for each reported IE/IE group", so two offenders
// produce two entries, not just the first.
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
