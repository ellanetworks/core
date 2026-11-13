// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"bytes"
	"testing"

	"github.com/ellanetworks/core/per"
)

func handUEContextReleaseCommand(t *testing.T, ran uint32) []byte {
	t.Helper()

	enc := per.Aligned

	ids := per.NewWriter()
	mustNil(t, per.EncodeConstrainedWholeNumber(ids, enc, 0, ueNGAPIDsAlternatives-1, ueNGAPIDsPair))
	ids.WriteBit(true)
	ids.WriteBit(true)
	mustNil(t, AMFUENGAPID(7).MarshalPER(ids, enc))
	mustNil(t, RANUENGAPID(ran).MarshalPER(ids, enc))
	mustNil(t, per.EncodeConstrainedWholeNumber(ids, enc, 1, maxProtocolExtensions, 1))
	mustNil(t, per.EncodeConstrainedWholeNumber(ids, enc, 0, maxProtocolIEs, 999))
	mustNil(t, per.EncodeEnumerated(ids, enc, criticalityRootCount, false, int64(CriticalityIgnore)))
	mustNil(t, per.EncodeOpenTypeBytes(ids, enc, []byte{0xab, 0xcd}))
	mustNil(t, per.EncodeNormallySmallLength(ids, enc, 2, func(int64) error {
		ids.WriteBit(false)
		ids.WriteBit(true)

		return nil
	}))
	mustNil(t, per.EncodeOpenTypeBytes(ids, enc, []byte{0x11, 0x22}))

	cause, err := per.Marshal(&Cause{Group: CauseGroupMisc, Value: 0})
	mustNil(t, err)

	body := per.NewWriter()
	body.WriteBit(false)
	mustNil(t, encodeIEContainer(body, enc, []ieField{
		{id: IDUENGAPIDs, crit: CriticalityReject, raw: ids.Bytes()},
		{id: IDCause, crit: CriticalityIgnore, raw: cause},
	}))

	b, err := Marshal(&InitiatingMessage{ProcedureCode: ProcUEContextRelease, Criticality: CriticalityReject, Value: body.Bytes()})
	mustNil(t, err)

	return b
}

func TestRawPDUUENGAPIDsKeepPairExtensions(t *testing.T) {
	in := handUEContextReleaseCommand(t, 1)

	p, err := ParseRawPDU(in)
	mustNil(t, err)

	ids, ok, err := p.UENGAPIDs()
	if err != nil || !ok {
		t.Fatalf("UENGAPIDs() = %v, %v", ok, err)
	}

	if !ids.Pair || ids.AMFUENGAPID != 7 || ids.RANUENGAPID != 1 {
		t.Fatalf("UENGAPIDs() = %+v", ids)
	}

	mustNil(t, p.SetUENGAPIDs(ids))

	got, err := p.Marshal()
	mustNil(t, err)

	if !bytes.Equal(got, in) {
		t.Fatalf("unchanged set:\n got %x\nwant %x", got, in)
	}

	ids.RANUENGAPID = 9
	mustNil(t, p.SetUENGAPIDs(ids))

	got, err = p.Marshal()
	mustNil(t, err)

	if want := handUEContextReleaseCommand(t, 9); !bytes.Equal(got, want) {
		t.Fatalf("changed set:\n got %x\nwant %x", got, want)
	}
}
