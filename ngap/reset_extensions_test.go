// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"bytes"
	"testing"

	"github.com/ellanetworks/core/per"
)

type handItem struct {
	amf           *uint64
	ran           *uint32
	extCrit       Criticality
	withExt       bool
	withAdditions bool
}

func mustNil(t *testing.T, err error) {
	t.Helper()

	if err != nil {
		t.Fatal(err)
	}
}

func encodeHandItem(t *testing.T, w *per.Writer, it handItem) {
	t.Helper()

	enc := per.Aligned

	w.WriteBit(it.withAdditions)
	w.WriteBit(it.amf != nil)
	w.WriteBit(it.ran != nil)
	w.WriteBit(it.withExt)

	if it.amf != nil {
		mustNil(t, AMFUENGAPID(*it.amf).MarshalPER(w, enc))
	}

	if it.ran != nil {
		mustNil(t, RANUENGAPID(*it.ran).MarshalPER(w, enc))
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
}

func encodeHandList(t *testing.T, w *per.Writer, items []handItem) {
	t.Helper()

	mustNil(t, per.EncodeLength(w, per.Aligned, 1, maxnoofNGConnectionsToReset, true, int64(len(items)), func(int64) error {
		for _, it := range items {
			encodeHandItem(t, w, it)
		}

		return nil
	}))
}

func handNGReset(t *testing.T, items []handItem) []byte {
	t.Helper()

	rt := per.NewWriter()
	mustNil(t, per.EncodeConstrainedWholeNumber(rt, per.Aligned, 0, resetTypeAlternatives-1, resetTypePartOfNGInterface))
	encodeHandList(t, rt, items)

	cause, err := per.Marshal(&Cause{Group: CauseGroupMisc, Value: 0})
	mustNil(t, err)

	body := per.NewWriter()
	body.WriteBit(false)
	mustNil(t, encodeIEContainer(body, per.Aligned, []ieField{
		{id: IDCause, crit: CriticalityIgnore, raw: cause},
		{id: IDResetType, crit: CriticalityReject, raw: rt.Bytes()},
	}))

	b, err := Marshal(&InitiatingMessage{ProcedureCode: ProcNGReset, Criticality: CriticalityReject, Value: body.Bytes()})
	mustNil(t, err)

	return b
}

func handNGResetAck(t *testing.T, items []handItem) []byte {
	t.Helper()

	list := per.NewWriter()
	encodeHandList(t, list, items)

	body := per.NewWriter()
	body.WriteBit(false)
	mustNil(t, encodeIEContainer(body, per.Aligned, []ieField{
		{id: IDUEAssociatedLogicalNGConnectionList, crit: CriticalityIgnore, raw: list.Bytes()},
	}))

	b, err := Marshal(&SuccessfulOutcome{ProcedureCode: ProcNGReset, Criticality: CriticalityReject, Value: body.Bytes()})
	mustNil(t, err)

	return b
}

func u64(v uint64) *uint64 { return &v }
func u32(v uint32) *uint32 { return &v }

func resetFixtures(amf uint64) []handItem {
	return []handItem{
		{amf: u64(amf), ran: u32(7), extCrit: CriticalityIgnore, withExt: true, withAdditions: true},
		{ran: u32(8), extCrit: CriticalityNotify, withExt: true},
		{amf: u64(amf + 1), withAdditions: true},
		{amf: u64(amf + 2), ran: u32(9)},
	}
}

func TestNGResetPreservesItemExtensions(t *testing.T) {
	in := handNGReset(t, resetFixtures(5))

	pdu, err := Unmarshal(in)
	mustNil(t, err)

	m, err := ParseNGReset(pdu.value())
	mustNil(t, err)

	out, err := m.Marshal()
	mustNil(t, err)

	if !bytes.Equal(out, in) {
		t.Fatalf("unmodified NG Reset re-encoded differently\n got % x\nwant % x", out, in)
	}

	for i := range m.ResetType.Part {
		if m.ResetType.Part[i].AMFUENGAPID != nil {
			v := *m.ResetType.Part[i].AMFUENGAPID - 5 + 0xFFFFFFFFFF - 2
			m.ResetType.Part[i].AMFUENGAPID = &v
		}
	}

	out, err = m.Marshal()
	mustNil(t, err)

	if want := handNGReset(t, resetFixtures(0xFFFFFFFFFF-2)); !bytes.Equal(out, want) {
		t.Fatalf("rewritten NG Reset differs\n got % x\nwant % x", out, want)
	}
}

func TestNGResetAcknowledgePreservesItemExtensions(t *testing.T) {
	in := handNGResetAck(t, resetFixtures(70000))

	pdu, err := Unmarshal(in)
	mustNil(t, err)

	m, err := ParseNGResetAcknowledge(pdu.value())
	mustNil(t, err)

	out, err := m.Marshal()
	mustNil(t, err)

	if !bytes.Equal(out, in) {
		t.Fatalf("unmodified NG Reset Acknowledge re-encoded differently\n got % x\nwant % x", out, in)
	}

	for i := range m.ConnectionList {
		if m.ConnectionList[i].AMFUENGAPID != nil {
			v := *m.ConnectionList[i].AMFUENGAPID - 70000 + 3
			m.ConnectionList[i].AMFUENGAPID = &v
		}
	}

	out, err = m.Marshal()
	mustNil(t, err)

	if want := handNGResetAck(t, resetFixtures(3)); !bytes.Equal(out, want) {
		t.Fatalf("rewritten NG Reset Acknowledge differs\n got % x\nwant % x", out, want)
	}
}

func TestNGResetItemRejectExtensionNotComprehended(t *testing.T) {
	in := handNGReset(t, []handItem{{amf: u64(1), extCrit: CriticalityReject, withExt: true}})

	pdu, err := Unmarshal(in)
	mustNil(t, err)

	if _, err := ParseNGReset(pdu.value()); err == nil {
		t.Fatal("reject-criticality item extension accepted")
	}
}
