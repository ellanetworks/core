// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"bytes"
	"errors"
	"testing"

	"github.com/ellanetworks/core/per"
	"github.com/ellanetworks/core/per/ap"
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

	if err := TAC(1).MarshalPER(w, per.Aligned); err != nil {
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
		ieField{ID: IDMMEUES1APID, Criticality: CriticalityReject, Value: MMEUES1APID(1)},
		ieField{ID: IDENBUES1APID, Criticality: CriticalityReject, Value: ENBUES1APID(2)},
		ieField{ID: IDNASPDU, Criticality: CriticalityReject, Value: NASPDU{0x7e, 0x00}},
		ieField{ID: IDEUTRANCGI, Criticality: CriticalityIgnore, Value: &EUTRANCGI{PLMNIdentity: PLMNIdentity{0x00, 0xf1, 0x10}, CellID: 0x0abcde1}},
		ieField{ID: IDTAI, Criticality: CriticalityIgnore, Value: ap.RawValue(extendedTAI(t, CriticalityReject))},
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
		ieField{ID: IDMMEUES1APID, Criticality: CriticalityReject, Value: MMEUES1APID(1)},
		ieField{ID: IDENBUES1APID, Criticality: CriticalityReject, Value: ENBUES1APID(2)},
		ieField{ID: IDNASPDU, Criticality: CriticalityReject, Value: NASPDU{0x7e, 0x00}},
		ieField{ID: IDEUTRANCGI, Criticality: CriticalityIgnore, Value: &EUTRANCGI{PLMNIdentity: PLMNIdentity{0x00, 0xf1, 0x10}, CellID: 0x0abcde1}},
		ieField{ID: IDTAI, Criticality: CriticalityIgnore, Value: ap.RawValue(extendedTAI(t, CriticalityIgnore))},
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

func erabHOReqWithUnknownExtension(t *testing.T, crit Criticality) []byte {
	t.Helper()

	in := ERABToBeSetupItemHOReq{
		ERABID:                    1,
		TransportLayerAddress:     tlaOf(10, 0, 0, 1),
		GTPTEID:                   1,
		QoS:                       ERABLevelQoSParameters{QCI: 9},
		DataForwardingNotPossible: Ptr(DataForwardingNotPossibleTrue),
	}
	in.iEExtensions.Append(ap.Field[ProtocolIEID]{ID: idRATType, Criticality: crit, Value: []byte{0x00}})

	b, err := per.Marshal(&in, per.Aligned)
	if err != nil {
		t.Fatal(err)
	}

	return b
}

func TestERABToBeSetupItemHOReqUnmodeledRejectExtension(t *testing.T) {
	in := erabHOReqWithUnknownExtension(t, CriticalityReject)

	var item ERABToBeSetupItemHOReq
	if err := item.UnmarshalPER(per.NewReader(in), per.Aligned); err != nil {
		t.Fatalf("decode is lenient, got %v", err)
	}

	if item.DataForwardingNotPossible == nil {
		t.Fatal("Data Forwarding Not Possible: got absent, want present alongside the unknown extension")
	}

	d := ap.Diagnose[ProtocolIEID](&item)
	if len(d.IEs) != 1 || d.IEs[0].ID != idRATType || d.IEs[0].Criticality != CriticalityReject || d.IEs[0].Kind != ap.KindNotComprehended {
		t.Fatalf("diagnostics = %+v, want the reject extension not comprehended", d.IEs)
	}

	var ase *ap.AbstractSyntaxError[ProtocolIEID]
	if !errors.As(d.Check(), &ase) {
		t.Fatalf("Check() = %v, want the reject extension to reject", d.Check())
	}

	out, err := per.Marshal(&item, per.Aligned)
	if err != nil || !bytes.Equal(out, in) {
		t.Fatalf("re-encode = %x, %v; want %x", out, err, in)
	}
}

func TestERABToBeSetupItemHOReqUnmodeledIgnoreExtension(t *testing.T) {
	var item ERABToBeSetupItemHOReq

	if err := item.UnmarshalPER(per.NewReader(erabHOReqWithUnknownExtension(t, CriticalityIgnore)), per.Aligned); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if item.DataForwardingNotPossible == nil {
		t.Fatal("Data Forwarding Not Possible: got absent, want present alongside the ignored extension")
	}

	if err := ap.Diagnose[ProtocolIEID](&item).Check(); err != nil {
		t.Fatalf("Check() = %v, want an ignore extension to pass", err)
	}
}
