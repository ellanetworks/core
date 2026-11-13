// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"bytes"
	"testing"

	"github.com/ellanetworks/core/per"
)

type handItem struct {
	mme, enb      *uint32
	extCrit       Criticality
	withExt       bool
	withAdditions bool
}

func encodeHandItem(t *testing.T, it handItem) []byte {
	t.Helper()

	w := per.NewWriter()
	enc := per.Aligned

	w.WriteBit(it.withAdditions)
	w.WriteBit(it.mme != nil)
	w.WriteBit(it.enb != nil)
	w.WriteBit(it.withExt)

	if it.mme != nil {
		if err := MMEUES1APID(*it.mme).MarshalPER(w, enc); err != nil {
			t.Fatal(err)
		}
	}

	if it.enb != nil {
		if err := ENBUES1APID(*it.enb).MarshalPER(w, enc); err != nil {
			t.Fatal(err)
		}
	}

	if it.withExt {
		mustNil(t, per.EncodeConstrainedWholeNumber(w, enc, 1, maxProtocolExtensions, 2))

		for i, id := range []int64{999, 1000} {
			mustNil(t, per.EncodeConstrainedWholeNumber(w, enc, 0, maxProtocolIEs, id))
			mustNil(t, per.EncodeEnumerated(w, enc, criticalityRootCount, false, int64(it.extCrit)))
			mustNil(t, per.EncodeOpenTypeBytes(w, enc, bytes.Repeat([]byte{0xab}, 1+i*200)))
		}
	}

	if it.withAdditions {
		mustNil(t, per.EncodeNormallySmallLength(w, enc, 3, func(int64) error {
			w.WriteBit(true)
			w.WriteBit(false)
			w.WriteBit(true)

			return nil
		}))
		mustNil(t, per.EncodeOpenTypeBytes(w, enc, []byte{0x11}))
		mustNil(t, per.EncodeOpenTypeBytes(w, enc, []byte{0x22, 0x33}))
	}

	return w.Bytes()
}

func mustNil(t *testing.T, err error) {
	t.Helper()

	if err != nil {
		t.Fatal(err)
	}
}

func encodeHandList(t *testing.T, w *per.Writer, crit Criticality, items []handItem) {
	t.Helper()

	mustNil(t, per.EncodeLength(w, per.Aligned, 1, maxnoofIndividualS1ConnectionsToReset, true, int64(len(items)), func(int64) error {
		for _, it := range items {
			if err := encodeContainerField(w, per.Aligned, ieField{id: IDUEAssociatedLogicalS1ConnectionItem, crit: crit, raw: encodeHandItem(t, it)}); err != nil {
				return err
			}
		}

		return nil
	}))
}

func handReset(t *testing.T, items []handItem) []byte {
	t.Helper()

	rt := per.NewWriter()
	rt.WriteBit(false)
	mustNil(t, per.EncodeConstrainedWholeNumber(rt, per.Aligned, 0, resetTypeChoiceRootCount-1, resetTypePartOfS1Interface))
	encodeHandList(t, rt, CriticalityReject, items)

	cause, err := per.Marshal(&Cause{Group: CauseGroupMisc, Value: 0})
	mustNil(t, err)

	body := per.NewWriter()
	body.WriteBit(false)
	mustNil(t, encodeIEContainer(body, per.Aligned, []ieField{
		{id: IDCause, crit: CriticalityIgnore, raw: cause},
		{id: IDResetType, crit: CriticalityReject, raw: rt.Bytes()},
	}))

	b, err := Marshal(&InitiatingMessage{ProcedureCode: ProcReset, Criticality: CriticalityReject, Value: body.Bytes()})
	mustNil(t, err)

	return b
}

func handResetAck(t *testing.T, items []handItem) []byte {
	t.Helper()

	list := per.NewWriter()
	encodeHandList(t, list, CriticalityIgnore, items)

	body := per.NewWriter()
	body.WriteBit(false)
	mustNil(t, encodeIEContainer(body, per.Aligned, []ieField{
		{id: IDUEAssociatedLogicalS1ConnectionListResAck, crit: CriticalityIgnore, raw: list.Bytes()},
	}))

	b, err := Marshal(&SuccessfulOutcome{ProcedureCode: ProcReset, Criticality: CriticalityReject, Value: body.Bytes()})
	mustNil(t, err)

	return b
}

func u32(v uint32) *uint32 { return &v }

func resetFixtures(mme uint32) []handItem {
	return []handItem{
		{mme: u32(mme), enb: u32(7), extCrit: CriticalityIgnore, withExt: true, withAdditions: true},
		{enb: u32(8), extCrit: CriticalityNotify, withExt: true},
		{mme: u32(mme + 1), withAdditions: true},
		{mme: u32(mme + 2), enb: u32(9)},
	}
}

func TestResetPreservesItemExtensions(t *testing.T) {
	in := handReset(t, resetFixtures(5))

	pdu, err := Unmarshal(in)
	mustNil(t, err)

	m, err := ParseReset(pdu.value())
	mustNil(t, err)

	out, err := m.Marshal()
	mustNil(t, err)

	if !bytes.Equal(out, in) {
		t.Fatalf("unmodified Reset re-encoded differently\n got % x\nwant % x", out, in)
	}

	for i := range m.ResetType.Part {
		if m.ResetType.Part[i].MMEUES1APID != nil {
			v := *m.ResetType.Part[i].MMEUES1APID + 0x10000000 - 5
			m.ResetType.Part[i].MMEUES1APID = &v
		}
	}

	out, err = m.Marshal()
	mustNil(t, err)

	if want := handReset(t, resetFixtures(0x10000000)); !bytes.Equal(out, want) {
		t.Fatalf("rewritten Reset differs\n got % x\nwant % x", out, want)
	}
}

func TestResetAcknowledgePreservesItemExtensions(t *testing.T) {
	in := handResetAck(t, resetFixtures(70000))

	pdu, err := Unmarshal(in)
	mustNil(t, err)

	m, err := ParseResetAcknowledge(pdu.value())
	mustNil(t, err)

	out, err := m.Marshal()
	mustNil(t, err)

	if !bytes.Equal(out, in) {
		t.Fatalf("unmodified Reset Acknowledge re-encoded differently\n got % x\nwant % x", out, in)
	}

	for i := range m.ConnectionList {
		if m.ConnectionList[i].MMEUES1APID != nil {
			v := *m.ConnectionList[i].MMEUES1APID - 70000 + 3
			m.ConnectionList[i].MMEUES1APID = &v
		}
	}

	out, err = m.Marshal()
	mustNil(t, err)

	if want := handResetAck(t, resetFixtures(3)); !bytes.Equal(out, want) {
		t.Fatalf("rewritten Reset Acknowledge differs\n got % x\nwant % x", out, want)
	}
}

func TestResetItemRejectExtensionNotComprehended(t *testing.T) {
	in := handReset(t, []handItem{{mme: u32(1), extCrit: CriticalityReject, withExt: true}})

	pdu, err := Unmarshal(in)
	mustNil(t, err)

	if _, err := ParseReset(pdu.value()); err == nil {
		t.Fatal("reject-criticality item extension accepted")
	}
}
