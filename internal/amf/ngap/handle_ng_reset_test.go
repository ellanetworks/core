// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap_test

import (
	"context"
	"testing"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/amf/ngap"
	"github.com/ellanetworks/core/internal/amf/ngap/decode"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/free5gc/ngap/ngapType"
)

func miscCause() ngapType.Cause {
	return ngapType.Cause{
		Present: ngapType.CausePresentMisc,
		Misc: &ngapType.CauseMisc{
			Value: ngapType.CauseMiscPresentHardwareFailure,
		},
	}
}

func TestHandleNGReset_ResetNGInterface(t *testing.T) {
	sender := &fakeNGAPSender{}

	ran := &amf.Radio{
		Log:           logger.AmfLog,
		Conn:          sender,
		SupportedTAIs: make([]amf.SupportedTAI, 0),
	}
	ran.BindAMFForTest(amf.New(nil, nil, nil))
	amf.NewRanUeForTest(ran, 0, 0, logger.AmfLog)
	amf.NewRanUeForTest(ran, 1, 1, logger.AmfLog)

	msg := decode.NGReset{
		Cause: miscCause(),
		ResetType: &ngapType.ResetType{
			Present:     ngapType.ResetTypePresentNGInterface,
			NGInterface: &ngapType.ResetAll{Value: ngapType.ResetAllPresentResetAll},
		},
	}

	ngap.HandleNGReset(context.Background(), ran, msg)

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
		Log:           logger.AmfLog,
		Conn:          sender,
		SupportedTAIs: make([]amf.SupportedTAI, 0),
	}
	ran.BindAMFForTest(amf.New(nil, nil, nil))
	amf.NewRanUeForTest(ran, 0, 0, logger.AmfLog)
	amf.NewRanUeForTest(ran, 1, 1, logger.AmfLog)

	partOfNG := &ngapType.UEAssociatedLogicalNGConnectionList{
		List: []ngapType.UEAssociatedLogicalNGConnectionItem{
			{
				AMFUENGAPID: &ngapType.AMFUENGAPID{Value: 0},
				RANUENGAPID: &ngapType.RANUENGAPID{Value: 0},
			},
		},
	}

	msg := decode.NGReset{
		Cause: miscCause(),
		ResetType: &ngapType.ResetType{
			Present:           ngapType.ResetTypePresentPartOfNGInterface,
			PartOfNGInterface: partOfNG,
		},
	}

	ngap.HandleNGReset(context.Background(), ran, msg)

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
// ranUe is nil after the lookup but Remove() is called anyway.
func TestHandleNGReset_PartOfNGInterface_UnknownUE(t *testing.T) {
	sender := &fakeNGAPSender{}

	ran := &amf.Radio{
		Log:           logger.AmfLog,
		Conn:          sender,
		SupportedTAIs: make([]amf.SupportedTAI, 0),
	}
	ran.BindAMFForTest(amf.New(nil, nil, nil))

	partOfNG := &ngapType.UEAssociatedLogicalNGConnectionList{
		List: []ngapType.UEAssociatedLogicalNGConnectionItem{
			{
				AMFUENGAPID: &ngapType.AMFUENGAPID{Value: 999},
				RANUENGAPID: &ngapType.RANUENGAPID{Value: 999},
			},
		},
	}

	msg := decode.NGReset{
		Cause: miscCause(),
		ResetType: &ngapType.ResetType{
			Present:           ngapType.ResetTypePresentPartOfNGInterface,
			PartOfNGInterface: partOfNG,
		},
	}

	ngap.HandleNGReset(context.Background(), ran, msg)

	if len(sender.SentNGResetAcknowledges) != 1 {
		t.Fatalf("expected 1 NGResetAcknowledge, got %d", len(sender.SentNGResetAcknowledges))
	}
}
