// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"errors"
	"testing"

	"github.com/ellanetworks/core/per"
)

// A real TAI extension id this version does not model.
const idNRCGI ProtocolIEID = 82

// A one-item TAIListForPaging whose TAI carries one unmodeled iE-Extension.
func extendedTAIList(t *testing.T, crit Criticality) []byte {
	t.Helper()

	w := per.NewWriter()

	if err := per.EncodeConstrainedWholeNumber(w, per.Aligned, 1, maxnoofTAIforPaging, 1); err != nil {
		t.Fatalf("list length: %v", err)
	}

	w.WriteBit(false) // TAIListForPagingItem is extensible: no extension additions
	w.WriteBit(false) // item iE-Extensions absent
	w.WriteBit(false) // TAI is extensible: no extension additions
	w.WriteBit(true)  // TAI iE-Extensions present

	if err := (PLMNIdentity{0x00, 0xf1, 0x10}).MarshalPER(w, per.Aligned); err != nil {
		t.Fatalf("plmn: %v", err)
	}

	if err := (TAC(1)).MarshalPER(w, per.Aligned); err != nil {
		t.Fatalf("tac: %v", err)
	}

	if err := per.EncodeConstrainedWholeNumber(w, per.Aligned, 1, maxProtocolExtensions, 1); err != nil {
		t.Fatalf("ext count: %v", err)
	}

	if err := per.EncodeConstrainedWholeNumber(w, per.Aligned, 0, maxProtocolIEs, int64(idNRCGI)); err != nil {
		t.Fatalf("ext id: %v", err)
	}

	if err := per.EncodeEnumerated(w, per.Aligned, criticalityRootCount, false, int64(crit)); err != nil {
		t.Fatalf("ext criticality: %v", err)
	}

	if err := per.EncodeOpenTypeBytes(w, per.Aligned, []byte{0x00}); err != nil {
		t.Fatalf("ext value: %v", err)
	}

	w.AlignToByte()

	return w.Bytes()
}

// §10.3.2 treats a not-comprehended item on its own criticality, so a reject
// extension rejects even inside an ignore IE, and §9.3.1.3 names the
// extension's id rather than the container's.
func TestRejectExtensionInsideIgnoreIERejects(t *testing.T) {
	_, err := ParsePaging(container(t,
		ieField{id: idUEPagingIdentity, crit: CriticalityIgnore, val: &FiveGSTMSI{AMFSetID: 1, AMFPointer: 1, FiveGTMSI: 0xdeadbeef}},
		ieField{id: idTAIListForPaging, crit: CriticalityIgnore, raw: extendedTAIList(t, CriticalityReject)},
	))

	var ase *AbstractSyntaxError
	if !errors.As(err, &ase) {
		t.Fatalf("err = %v, want the procedure rejected", err)
	}

	if len(ase.IEs) != 1 {
		t.Fatalf("reported %+v, want exactly the extension", ase.IEs)
	}

	if ase.IEs[0].IEID != idNRCGI {
		t.Errorf("diagnostics name IE %d, want the extension %d", ase.IEs[0].IEID, idNRCGI)
	}

	if ase.IEs[0].IECriticality != CriticalityReject {
		t.Errorf("diagnostics report criticality %v, want the extension's reject", ase.IEs[0].IECriticality)
	}
}

// §10.3.4.2: an ignore extension is skipped and its IE still delivered.
func TestIgnoreExtensionKeepsItsIE(t *testing.T) {
	msg, err := ParsePaging(container(t,
		ieField{id: idUEPagingIdentity, crit: CriticalityIgnore, val: &FiveGSTMSI{AMFSetID: 1, AMFPointer: 1, FiveGTMSI: 0xdeadbeef}},
		ieField{id: idTAIListForPaging, crit: CriticalityIgnore, raw: extendedTAIList(t, CriticalityIgnore)},
	))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	if len(msg.TAIListForPaging) != 1 {
		t.Fatalf("TAIListForPaging = %+v, thrown away over an extension its own criticality says to ignore", msg.TAIListForPaging)
	}

	if msg.TAIListForPaging[0].TAC != 1 {
		t.Errorf("TAI.TAC = %d, want 1", msg.TAIListForPaging[0].TAC)
	}
}
