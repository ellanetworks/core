// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"bytes"
	"errors"
	"testing"

	"github.com/ellanetworks/core/per"
	"github.com/ellanetworks/core/per/ap/apimpl"
)

// A CHOICE selecting its choice-Extensions alternative, carrying one entry.
func choiceExtensionValue(t *testing.T, alternatives, index int, id ProtocolIEID) []byte {
	t.Helper()

	return choiceExtensionValueCrit(t, alternatives, index, id, CriticalityReject)
}

func choiceExtensionValueCrit(t *testing.T, alternatives, index int, id ProtocolIEID, crit Criticality) []byte {
	t.Helper()

	w := per.NewWriter()
	if err := per.EncodeConstrainedWholeNumber(w, per.Aligned, 0, int64(alternatives-1), int64(index)); err != nil {
		t.Fatal(err)
	}

	if err := encodeContainerField(w, ieField{
		ID: id, Criticality: crit, Value: apimpl.RawValue([]byte{0x00}),
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
		ID:          IDGlobalRANNodeID,
		Criticality: CriticalityReject,
		Value:       apimpl.RawValue(choiceExtensionValue(t, globalRANNodeIDAlternatives, globalRANNodeIDChoiceExtensions, IDGlobalRANNodeID)),
	})

	_, err := ParseNGSetupRequest(value)
	if err == nil {
		t.Fatal("parse succeeded, want a rejection")
	}

	var ase *AbstractSyntaxError
	if !errors.As(err, &ase) {
		t.Fatalf("error = %T (%v), want *AbstractSyntaxError so the reject is reported in NG SETUP FAILURE", err, err)
	}

	if !ase.Cause.Equal(CauseProtocolAbstractSyntaxErrorReject.Cause()) {
		t.Errorf("cause = %s, want abstract-syntax-error-reject", ase.Cause)
	}

	if len(ase.IEs) != 1 || ase.IEs[0].IEID != IDGlobalRANNodeID ||
		ase.IEs[0].TypeOfError != TypeOfErrorNotUnderstood {
		t.Errorf("diagnostics = %+v, want one not-understood entry for GlobalRANNodeID", ase.IEs)
	}
}

func TestChoiceExtensionInsideIgnoreIEIsJudgedOnItsOwnCriticality(t *testing.T) {
	for _, crit := range []Criticality{CriticalityIgnore, CriticalityReject} {
		value := container(t,
			ieField{
				ID:          IDCause,
				Criticality: CriticalityIgnore,
				Value:       apimpl.RawValue(choiceExtensionValueCrit(t, causeAlternatives, causeChoiceExtensions, 9999, crit)),
			},
			ieField{ID: IDResetType, Criticality: CriticalityReject, Value: &ResetType{NGInterface: Ptr(ResetAllResetAll)}},
		)

		msg, err := ParseNGReset(value)

		if crit == CriticalityReject {
			var ase *AbstractSyntaxError
			if !errors.As(err, &ase) {
				t.Fatalf("reject choice-Extensions: err = %v, want *AbstractSyntaxError", err)
			}

			continue
		}

		if err != nil {
			t.Fatalf("parse: %v, want the message delivered", err)
		}

		if msg.Cause != nil {
			t.Errorf("Cause = %+v, want an ignore IE with no comprehended alternative treated as not received", msg.Cause)
		}

		decoded, err := DecodeNGReset(value)
		if err != nil {
			t.Fatal(err)
		}

		if decoded.Cause == nil || decoded.Cause.ChoiceExtension == nil || decoded.Cause.ChoiceExtension.ID != 9999 {
			t.Errorf("decoded Cause = %+v, want the unknown choice-Extensions kept", decoded.Cause)
		}

		if msg.ResetType.NGInterface == nil {
			t.Error("ResetType lost, want the rest of the message still decoded")
		}

		var reported bool

		for _, ie := range msg.Diagnostics().IEs {
			if ie.ID == 9999 && ie.Criticality == CriticalityIgnore {
				reported = true
			}
		}

		if !reported {
			t.Errorf("unknown choice-Extensions IE missing from diagnostics: %+v", msg.Diagnostics().IEs)
		}

		b, err := msg.Marshal()
		if err != nil {
			t.Fatal(err)
		}

		pdu, err := Unmarshal(b)
		if err != nil {
			t.Fatal(err)
		}

		if !bytes.Equal(pdu.value(), value) {
			t.Errorf("re-encode = % x, want % x", pdu.value(), value)
		}
	}
}

// §10.3.4.2 wants an entry "for each reported IE/IE group", so two offenders
// produce two entries, not just the first.
func TestAllNotComprehendedRejectIEsAreReported(t *testing.T) {
	value := container(t,
		ieField{ID: ProtocolIEID(40001), Criticality: CriticalityReject, Value: apimpl.RawValue([]byte{0x00})},
		ieField{ID: ProtocolIEID(40002), Criticality: CriticalityReject, Value: apimpl.RawValue([]byte{0x00})},
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
