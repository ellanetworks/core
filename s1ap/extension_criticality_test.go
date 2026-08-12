// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"errors"
	"testing"

	"github.com/ellanetworks/core/per"
)

// A real reject-criticality SupportedTAs-Item extension this version does not
// model, sent by a Release-13 eNB.
const idRATType ProtocolIEID = 232

// A TAI whose iE-Extensions container carries one unmodeled extension.
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

// §10.3.2 treats a not-comprehended item on its own criticality, so a reject
// extension rejects even inside an ignore IE, and §9.2.1.21 names the
// extension's id rather than the container's.
func TestRejectExtensionInsideIgnoreIERejects(t *testing.T) {
	_, err := ParseUplinkNASTransport(container(t,
		ieField{id: IDMMEUES1APID, crit: CriticalityReject, val: MMEUES1APID(1)},
		ieField{id: IDENBUES1APID, crit: CriticalityReject, val: ENBUES1APID(2)},
		ieField{id: IDNASPDU, crit: CriticalityReject, val: NASPDU{0x7e, 0x00}},
		ieField{id: IDEUTRANCGI, crit: CriticalityIgnore, val: &EUTRANCGI{PLMNIdentity: PLMNIdentity{0x00, 0xf1, 0x10}, CellID: 0x0abcde1}},
		ieField{id: IDTAI, crit: CriticalityIgnore, raw: extendedTAI(t, CriticalityReject)},
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

// §10.3.4.2: an ignore extension is skipped and its IE still delivered.
func TestIgnoreExtensionKeepsItsIE(t *testing.T) {
	msg, err := ParseUplinkNASTransport(container(t,
		ieField{id: IDMMEUES1APID, crit: CriticalityReject, val: MMEUES1APID(1)},
		ieField{id: IDENBUES1APID, crit: CriticalityReject, val: ENBUES1APID(2)},
		ieField{id: IDNASPDU, crit: CriticalityReject, val: NASPDU{0x7e, 0x00}},
		ieField{id: IDEUTRANCGI, crit: CriticalityIgnore, val: &EUTRANCGI{PLMNIdentity: PLMNIdentity{0x00, 0xf1, 0x10}, CellID: 0x0abcde1}},
		ieField{id: IDTAI, crit: CriticalityIgnore, raw: extendedTAI(t, CriticalityIgnore)},
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

// erabHOReqExtensions encodes a ProtocolExtensionContainer for
// ERABToBeSetupItemHOReq holding one unmodeled extension, optionally followed
// by the modeled id-Data-Forwarding-Not-Possible.
func erabHOReqExtensions(t *testing.T, unmodeledCrit Criticality, withModeled bool) []byte {
	t.Helper()

	count := int64(1)
	if withModeled {
		count = 2
	}

	w := per.NewWriter()

	if err := per.EncodeConstrainedWholeNumber(w, per.Aligned, 1, maxProtocolExtensions, count); err != nil {
		t.Fatalf("ext count: %v", err)
	}

	if err := per.EncodeConstrainedWholeNumber(w, per.Aligned, 0, maxProtocolIEs, int64(idRATType)); err != nil {
		t.Fatalf("ext id: %v", err)
	}

	if err := per.EncodeEnumerated(w, per.Aligned, criticalityRootCount, false, int64(unmodeledCrit)); err != nil {
		t.Fatalf("ext criticality: %v", err)
	}

	if err := per.EncodeOpenTypeBytes(w, per.Aligned, []byte{0x00}); err != nil {
		t.Fatalf("ext value: %v", err)
	}

	if withModeled {
		if err := per.EncodeConstrainedWholeNumber(w, per.Aligned, 0, maxProtocolIEs, int64(IDDataForwardingNotPossible)); err != nil {
			t.Fatalf("modeled ext id: %v", err)
		}

		if err := per.EncodeEnumerated(w, per.Aligned, criticalityRootCount, false, int64(CriticalityIgnore)); err != nil {
			t.Fatalf("modeled ext criticality: %v", err)
		}

		if err := per.EncodeOpenTypeBytes(w, per.Aligned, []byte{0x00}); err != nil {
			t.Fatalf("modeled ext value: %v", err)
		}
	}

	w.AlignToByte()

	return w.Bytes()
}

func TestERABToBeSetupItemHOReqUnmodeledRejectExtension(t *testing.T) {
	var ext ERABToBeSetupItemHOReqExtIEs

	err := ext.UnmarshalPER(per.NewReader(erabHOReqExtensions(t, CriticalityReject, false)), per.Aligned)

	var nc *notComprehendedIE
	if !errors.As(err, &nc) {
		t.Fatalf("err = %v, want the extension not comprehended", err)
	}

	if nc.ID != idRATType {
		t.Errorf("reported id = %d, want %d", nc.ID, idRATType)
	}
}

func TestERABToBeSetupItemHOReqUnmodeledIgnoreExtension(t *testing.T) {
	var ext ERABToBeSetupItemHOReqExtIEs

	if err := ext.UnmarshalPER(per.NewReader(erabHOReqExtensions(t, CriticalityIgnore, true)), per.Aligned); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if ext.DataForwardingNotPossible == nil {
		t.Fatal("Data Forwarding Not Possible: got absent, want present alongside the ignored extension")
	}
}
