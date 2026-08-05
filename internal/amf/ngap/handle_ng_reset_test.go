// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"context"
	"testing"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/ngap"
)

func miscCause() *ngap.Cause {
	return ngap.Ptr(ngap.Cause{
		Group: ngap.CauseGroupMisc, Value: ngap.CauseMiscHardwareFailure,
	})
}

func TestHandleNGReset_ResetNGInterface(t *testing.T) {
	sender := &fakeNGAPSender{}

	ran := &amf.Radio{
		Log:  logger.AmfLog,
		Conn: sender,
	}
	amfInstance := amf.New(nil, nil, nil)
	ran.BindAMFForTest(amfInstance)
	amf.NewUeConnForTest(ran, 0, 0, logger.AmfLog)
	amf.NewUeConnForTest(ran, 1, 1, logger.AmfLog)

	msg := &ngap.NGReset{
		Cause:     miscCause(),
		ResetType: ngap.ResetType{All: true},
	}

	HandleNGReset(context.Background(), amfInstance, ran, msg)

	if len(sender.SentNGResetAcknowledges) != 1 {
		t.Fatalf("expected 1 NGResetAcknowledge to be sent, but got %d", len(sender.SentNGResetAcknowledges))
	}

	if sender.SentNGResetAcknowledges[0].ConnectionList != nil {
		t.Fatalf("expected ConnectionList to be nil, but got %v", sender.SentNGResetAcknowledges[0].ConnectionList)
	}

	if ran.NumUEsForTest() != 0 {
		t.Fatalf("expected all UEs to be removed from the RAN, but got %d", ran.NumUEsForTest())
	}
}

func TestHandleNGReset_PartOfNGInterface(t *testing.T) {
	sender := &fakeNGAPSender{}

	ran := &amf.Radio{
		Log:  logger.AmfLog,
		Conn: sender,
	}
	amfInstance := amf.New(nil, nil, nil)
	ran.BindAMFForTest(amfInstance)
	amf.NewUeConnForTest(ran, 0, 0, logger.AmfLog)
	amf.NewUeConnForTest(ran, 1, 1, logger.AmfLog)

	partOfNG := ngap.UEAssociatedLogicalNGConnectionList{{
		AMFUENGAPID: ngap.Ptr(ngap.AMFUENGAPID(0)),
		RANUENGAPID: ngap.Ptr(ngap.RANUENGAPID(0)),
	}}

	msg := &ngap.NGReset{
		Cause:     miscCause(),
		ResetType: ngap.ResetType{Part: partOfNG},
	}

	HandleNGReset(context.Background(), amfInstance, ran, msg)

	if len(sender.SentNGResetAcknowledges) != 1 {
		t.Fatalf("expected 1 NGResetAcknowledge to be sent, but got %d", len(sender.SentNGResetAcknowledges))
	}

	if sender.SentNGResetAcknowledges[0].ConnectionList == nil {
		t.Fatalf("expected ConnectionList to be not nil")
	}

	if len(sender.SentNGResetAcknowledges[0].ConnectionList) != 1 {
		t.Fatalf("expected 1 UE in ConnectionList, but got %d", len(sender.SentNGResetAcknowledges[0].ConnectionList))
	}

	if id := sender.SentNGResetAcknowledges[0].ConnectionList[0].RANUENGAPID; id == nil || *id != 0 {
		t.Fatalf("expected RANUENGAPID to be 0, but got %v", id)
	}

	if ran.NumUEsForTest() != 1 {
		t.Fatalf("expected 1 UE to remain in the RAN, but got %d", ran.NumUEsForTest())
	}
}

// TestHandleNGReset_PartOfNGInterface_UnknownUE verifies that a ConnectionList
// reset referencing a RANUENGAPID that does not match any UE context does NOT
// panic or remove the wrong UE. This exercises the missing-continue bug where
// ueConn is nil after the lookup but Remove() is called anyway.
func TestHandleNGReset_PartOfNGInterface_UnknownUE(t *testing.T) {
	sender := &fakeNGAPSender{}

	ran := &amf.Radio{
		Log:  logger.AmfLog,
		Conn: sender,
	}
	amfInstance := amf.New(nil, nil, nil)
	ran.BindAMFForTest(amfInstance)

	partOfNG := ngap.UEAssociatedLogicalNGConnectionList{{
		AMFUENGAPID: ngap.Ptr(ngap.AMFUENGAPID(999)),
		RANUENGAPID: ngap.Ptr(ngap.RANUENGAPID(999)),
	}}

	msg := &ngap.NGReset{
		Cause:     miscCause(),
		ResetType: ngap.ResetType{Part: partOfNG},
	}

	HandleNGReset(context.Background(), amfInstance, ran, msg)

	if len(sender.SentNGResetAcknowledges) != 1 {
		t.Fatalf("expected 1 NGResetAcknowledge, got %d", len(sender.SentNGResetAcknowledges))
	}

	// TS 38.413 §8.7.4.2.2: the acknowledge "shall include also unknown
	// UE-associated logical NG-connections". The gNB cannot reuse a UE NGAP ID
	// until the AMF confirms it, so dropping the ones this AMF never held would
	// strand them.
	list := sender.SentNGResetAcknowledges[0].ConnectionList
	if list == nil || len(list) != 1 {
		t.Fatalf("acknowledge must echo the unknown connection, got %+v", list)
	}

	item := list[0]
	if item.AMFUENGAPID == nil || *item.AMFUENGAPID != 999 {
		t.Errorf("echoed AMF-UE-NGAP-ID = %+v, want 999", item.AMFUENGAPID)
	}

	if item.RANUENGAPID == nil || *item.RANUENGAPID != 999 {
		t.Errorf("echoed RAN-UE-NGAP-ID = %+v, want 999", item.RANUENGAPID)
	}
}
