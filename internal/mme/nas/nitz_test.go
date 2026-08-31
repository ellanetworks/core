// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"context"
	"testing"
	"time"

	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/mme"
	"github.com/ellanetworks/core/internal/udm"
	"github.com/ellanetworks/core/nas"
	"github.com/ellanetworks/core/nas/eps"
)

type spnBearerStore struct{ fakeBearerStore }

func (spnBearerStore) GetOperator(_ context.Context) (*db.Operator, error) {
	return &db.Operator{
		Mcc: "001", Mnc: "01", SupportedTACs: `["1"]`,
		Ciphering: `["AES"]`, Integrity: `["AES"]`,
		SpnFullName: "Ella", SpnShortName: "Ella",
		AmfRegionID: 1, AmfSetID: 1,
	}, nil
}

// TS 24.301 §5.4.5
func TestSendNITZ(t *testing.T) {
	m := mme.New(udm.New(newFakeCredStore(), noopKeyResolver), spnBearerStore{}, &fakeSessionManager{})
	ue, cc := securedUE(t, m)

	sendNITZ(context.Background(), m, ue, ue.Conn())

	if len(cc.sent) != 1 {
		t.Fatalf("expected one EMM INFORMATION downlink, got %d", len(cc.sent))
	}

	dl := decodeDownlinkNAS(t, cc.sent[0])

	plain, err := unprotected(eps.Unprotect(dl, nas.MakeCount(0, dl[5]), nas.DirectionDownlink, mustSecurityContext(t, ue.EIA(), ue.EEA(), ue.KnasIntForTest(), ue.KnasEncForTest())))
	if err != nil {
		t.Fatalf("unprotect EMM INFORMATION: %v", err)
	}

	if mt, err := eps.PeekMessageType(plain); err != nil || mt != eps.MsgEMMInformation {
		t.Fatalf("downlink message = %#x (err %v), want EMM INFORMATION", mt, err)
	}
}

// TS 24.301 §8.2.13
func TestSendNITZNoSPN(t *testing.T) {
	m := newTestMME(t)
	ue, cc := securedUE(t, m)

	sendNITZ(context.Background(), m, ue, ue.Conn())

	if len(cc.sent) != 1 {
		t.Fatalf("expected one EMM INFORMATION downlink, got %d", len(cc.sent))
	}

	info := decodeEMMInformation(t, ue, cc.sent[0])

	if info.FullNameForNetwork != nil || info.ShortNameForNetwork != nil {
		t.Error("expected no network name when no SPN is configured")
	}

	if info.LocalTimeZone == nil || info.UniversalTime == nil || info.DaylightSavingTime == nil {
		t.Fatalf("expected all three time elements, got zone %v time %v dst %v",
			info.LocalTimeZone, info.UniversalTime, info.DaylightSavingTime)
	}
}

// TS 24.301 §8.2.13.4
func TestSendNITZTime(t *testing.T) {
	m := mme.New(udm.New(newFakeCredStore(), noopKeyResolver), spnBearerStore{}, &fakeSessionManager{})
	ue, cc := securedUE(t, m)

	before := time.Now()

	sendNITZ(context.Background(), m, ue, ue.Conn())

	after := time.Now()

	if len(cc.sent) != 1 {
		t.Fatalf("expected one EMM INFORMATION downlink, got %d", len(cc.sent))
	}

	info := decodeEMMInformation(t, ue, cc.sent[0])

	if info.LocalTimeZone == nil || info.UniversalTime == nil || info.DaylightSavingTime == nil {
		t.Fatalf("expected all three time elements, got zone %v time %v dst %v",
			info.LocalTimeZone, info.UniversalTime, info.DaylightSavingTime)
	}

	sent, ok := info.UniversalTime.Time()
	if !ok {
		t.Fatal("the universal time element did not decode")
	}

	if sent.Before(before.Truncate(time.Second)) || sent.After(after) {
		t.Errorf("universal time %s is outside [%s, %s]", sent, before, after)
	}

	if *info.LocalTimeZone != info.UniversalTime.Zone {
		t.Errorf("local time zone %#02x disagrees with the universal time's %#02x",
			byte(*info.LocalTimeZone), byte(info.UniversalTime.Zone))
	}

	if _, ok := info.DaylightSavingTime.Adjustment(); !ok {
		t.Errorf("daylight saving time is the reserved value %#02x", byte(*info.DaylightSavingTime))
	}
}

func decodeEMMInformation(t *testing.T, ue *mme.UeContext, sent []byte) *eps.EMMInformation {
	t.Helper()

	dl := decodeDownlinkNAS(t, sent)

	plain, err := unprotected(eps.Unprotect(dl, nas.MakeCount(0, dl[5]), nas.DirectionDownlink, mustSecurityContext(t, ue.EIA(), ue.EEA(), ue.KnasIntForTest(), ue.KnasEncForTest())))
	if err != nil {
		t.Fatalf("unprotect EMM INFORMATION: %v", err)
	}

	info, err := eps.ParseEMMInformation(plain)
	if err != nil {
		t.Fatalf("parse EMM INFORMATION: %v", err)
	}

	return info
}
