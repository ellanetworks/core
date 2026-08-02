// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap_test

import (
	"context"
	"testing"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/amf/ngap"
	"github.com/ellanetworks/core/internal/logger"
	ngaplib "github.com/ellanetworks/core/ngap"
)

func miscCause() *ngaplib.Cause {
	return ngaplib.Ptr(ngaplib.Cause{
		Group: ngaplib.CauseGroupMisc, Value: ngaplib.CauseMiscHardwareFailure,
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

	msg := &ngaplib.NGReset{
		Cause:     miscCause(),
		ResetType: ngaplib.ResetType{All: true},
	}

	ngap.HandleNGReset(context.Background(), amfInstance, ran, msg)

	if len(sender.SentNGResetAcknowledges) != 1 {
		t.Fatalf("expected 1 NGResetAcknowledge to be sent, but got %d", len(sender.SentNGResetAcknowledges))
	}

	if sender.SentNGResetAcknowledges[0].PartOfNGInterface != nil {
		t.Fatalf("expected PartOfNGInterface to be nil, but got %v", sender.SentNGResetAcknowledges[0].PartOfNGInterface)
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

	partOfNG := ngaplib.UEAssociatedLogicalNGConnectionList{{
		AMFUENGAPID: ngaplib.Ptr(ngaplib.AMFUENGAPID(0)),
		RANUENGAPID: ngaplib.Ptr(ngaplib.RANUENGAPID(0)),
	}}

	msg := &ngaplib.NGReset{
		Cause:     miscCause(),
		ResetType: ngaplib.ResetType{Part: partOfNG},
	}

	ngap.HandleNGReset(context.Background(), amfInstance, ran, msg)

	if len(sender.SentNGResetAcknowledges) != 1 {
		t.Fatalf("expected 1 NGResetAcknowledge to be sent, but got %d", len(sender.SentNGResetAcknowledges))
	}

	if sender.SentNGResetAcknowledges[0].PartOfNGInterface == nil {
		t.Fatalf("expected PartOfNGInterface to be not nil")
	}

	if len(sender.SentNGResetAcknowledges[0].PartOfNGInterface.List) != 1 {
		t.Fatalf("expected 1 UE in PartOfNGInterface, but got %d", len(sender.SentNGResetAcknowledges[0].PartOfNGInterface.List))
	}

	if sender.SentNGResetAcknowledges[0].PartOfNGInterface.List[0].RANUENGAPID.Value != 0 {
		t.Fatalf("expected RANUENGAPID to be 0, but got %d", sender.SentNGResetAcknowledges[0].PartOfNGInterface.List[0].RANUENGAPID.Value)
	}

	if ran.NumUEsForTest() != 1 {
		t.Fatalf("expected 1 UE to remain in the RAN, but got %d", ran.NumUEsForTest())
	}
}

// TestHandleNGReset_PartOfNGInterface_UnknownUE verifies that a PartOfNGInterface
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

	partOfNG := ngaplib.UEAssociatedLogicalNGConnectionList{{
		AMFUENGAPID: ngaplib.Ptr(ngaplib.AMFUENGAPID(999)),
		RANUENGAPID: ngaplib.Ptr(ngaplib.RANUENGAPID(999)),
	}}

	msg := &ngaplib.NGReset{
		Cause:     miscCause(),
		ResetType: ngaplib.ResetType{Part: partOfNG},
	}

	ngap.HandleNGReset(context.Background(), amfInstance, ran, msg)

	if len(sender.SentNGResetAcknowledges) != 1 {
		t.Fatalf("expected 1 NGResetAcknowledge, got %d", len(sender.SentNGResetAcknowledges))
	}

	// TS 38.413 §8.7.4.2.2: the acknowledge "shall include also unknown
	// UE-associated logical NG-connections". The gNB cannot reuse a UE NGAP ID
	// until the AMF confirms it, so dropping the ones this AMF never held would
	// strand them.
	list := sender.SentNGResetAcknowledges[0].PartOfNGInterface
	if list == nil || len(list.List) != 1 {
		t.Fatalf("acknowledge must echo the unknown connection, got %+v", list)
	}

	item := list.List[0]
	if item.AMFUENGAPID == nil || item.AMFUENGAPID.Value != 999 {
		t.Errorf("echoed AMF-UE-NGAP-ID = %+v, want 999", item.AMFUENGAPID)
	}

	if item.RANUENGAPID == nil || item.RANUENGAPID.Value != 999 {
		t.Errorf("echoed RAN-UE-NGAP-ID = %+v, want 999", item.RANUENGAPID)
	}
}
