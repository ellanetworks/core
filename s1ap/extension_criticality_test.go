// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"errors"
	"testing"

	"github.com/ellanetworks/core/per"
)

// idRATType is a real SupportedTAs-Item extension (TS 36.413), reject
// criticality, that this version does not model. Any unmodeled id would do; a
// real one keeps the vector honest about what a Release-13 eNB sends.
const idRATType ProtocolIEID = 232

// extendedTAI encodes a TAI whose iE-Extensions container carries one unmodeled
// extension with the given criticality.
func extendedTAI(t *testing.T, crit Criticality) []byte {
	t.Helper()

	w := per.NewWriter()
	w.WriteBit(false) // TAI is extensible: no extension additions
	w.WriteBit(true)  // iE-Extensions present

	if err := (PLMNIdentity{0x00, 0xf1, 0x10}).MarshalPER(w, per.Aligned); err != nil {
		t.Fatalf("plmn: %v", err)
	}

	if err := (TAC(1)).MarshalPER(w, per.Aligned); err != nil {
		t.Fatalf("tac: %v", err)
	}

	if err := per.EncodeConstrainedWholeNumber(w, per.Aligned, 1, maxProtocolExtensions, 1); err != nil {
		t.Fatalf("ext count: %v", err)
	}

	if err := per.EncodeConstrainedWholeNumber(w, per.Aligned, 0, maxProtocolIEs, int64(idRATType)); err != nil {
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

// TS 36.413 §10.3.2: "the entire item (IE or IE group) which is not (fully or
// partially) comprehended shall be treated in accordance with its own
// criticality information". A reject-criticality extension is its own item, so
// it rejects the procedure even though the TAI containing it is marked ignore —
// and §9.2.1.21 has the diagnostics name "the IE ID of the not understood or
// missing IE", which is the extension's, not the container's.
func TestRejectExtensionInsideIgnoreIERejects(t *testing.T) {
	_, err := ParseUplinkNASTransport(container(t,
		ieField{id: idMMEUES1APID, crit: CriticalityReject, val: MMEUES1APID(1)},
		ieField{id: idENBUES1APID, crit: CriticalityReject, val: ENBUES1APID(2)},
		ieField{id: idNASPDU, crit: CriticalityReject, val: NASPDU{0x7e, 0x00}},
		ieField{id: idEUTRANCGI, crit: CriticalityIgnore, val: &EUTRANCGI{PLMNIdentity: PLMNIdentity{0x00, 0xf1, 0x10}, CellID: 0x0abcde1}},
		ieField{id: idTAI, crit: CriticalityIgnore, raw: extendedTAI(t, CriticalityReject)},
	))

	var ase *AbstractSyntaxError
	if !errors.As(err, &ase) {
		t.Fatalf("err = %v, want the procedure rejected", err)
	}

	if len(ase.IEs) != 1 {
		t.Fatalf("reported %+v, want exactly the extension", ase.IEs)
	}

	if ase.IEs[0].IEID != idRATType {
		t.Errorf("diagnostics name IE %d, want the extension %d", ase.IEs[0].IEID, idRATType)
	}

	if ase.IEs[0].IECriticality != CriticalityReject {
		t.Errorf("diagnostics report criticality %v, want the extension's reject", ase.IEs[0].IECriticality)
	}
}

// An ignore-criticality extension is skipped and the IE holding it is still
// delivered — §10.3.4.2's "continue with the procedure" for the containing IE.
func TestIgnoreExtensionKeepsItsIE(t *testing.T) {
	msg, err := ParseUplinkNASTransport(container(t,
		ieField{id: idMMEUES1APID, crit: CriticalityReject, val: MMEUES1APID(1)},
		ieField{id: idENBUES1APID, crit: CriticalityReject, val: ENBUES1APID(2)},
		ieField{id: idNASPDU, crit: CriticalityReject, val: NASPDU{0x7e, 0x00}},
		ieField{id: idEUTRANCGI, crit: CriticalityIgnore, val: &EUTRANCGI{PLMNIdentity: PLMNIdentity{0x00, 0xf1, 0x10}, CellID: 0x0abcde1}},
		ieField{id: idTAI, crit: CriticalityIgnore, raw: extendedTAI(t, CriticalityIgnore)},
	))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	if msg.TAI == nil {
		t.Fatal("TAI was thrown away over an extension its own criticality says to ignore")
	}

	if msg.TAI.TAC != 1 {
		t.Errorf("TAI.TAC = %d, want 1", msg.TAI.TAC)
	}
}
