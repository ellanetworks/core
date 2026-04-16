// Copyright 2025 Ella Networks

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
	fakeNGAPSender := &FakeNGAPSender{}

	ran := &amf.Radio{
		Log:           logger.AmfLog,
		NGAPSender:    fakeNGAPSender,
		RanUEs:        make(map[int64]*amf.RanUe),
		SupportedTAIs: make([]amf.SupportedTAI, 0),
	}
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

	if len(fakeNGAPSender.SentNGResetAcknowledges) != 1 {
		t.Fatalf("expected 1 NGResetAcknowledge to be sent, but got %d", len(fakeNGAPSender.SentNGResetAcknowledges))
	}

	if fakeNGAPSender.SentNGResetAcknowledges[0].PartOfNGInterface != nil {
		t.Fatalf("expected PartOfNGInterface to be nil, but got %v", fakeNGAPSender.SentNGResetAcknowledges[0].PartOfNGInterface)
	}

	if len(ran.RanUEs) != 0 {
		t.Fatalf("expected all UEs to be removed from the RAN, but got %d", len(ran.RanUEs))
	}
}

func TestHandleNGReset_PartOfNGInterface(t *testing.T) {
	fakeNGAPSender := &FakeNGAPSender{}

	ran := &amf.Radio{
		Log:           logger.AmfLog,
		NGAPSender:    fakeNGAPSender,
		RanUEs:        make(map[int64]*amf.RanUe),
		SupportedTAIs: make([]amf.SupportedTAI, 0),
	}
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

	if len(fakeNGAPSender.SentNGResetAcknowledges) != 1 {
		t.Fatalf("expected 1 NGResetAcknowledge to be sent, but got %d", len(fakeNGAPSender.SentNGResetAcknowledges))
	}

	if fakeNGAPSender.SentNGResetAcknowledges[0].PartOfNGInterface == nil {
		t.Fatalf("expected PartOfNGInterface to be not nil")
	}

	if len(fakeNGAPSender.SentNGResetAcknowledges[0].PartOfNGInterface.List) != 1 {
		t.Fatalf("expected 1 UE in PartOfNGInterface, but got %d", len(fakeNGAPSender.SentNGResetAcknowledges[0].PartOfNGInterface.List))
	}

	if fakeNGAPSender.SentNGResetAcknowledges[0].PartOfNGInterface.List[0].RANUENGAPID.Value != 0 {
		t.Fatalf("expected RANUENGAPID to be 0, but got %d", fakeNGAPSender.SentNGResetAcknowledges[0].PartOfNGInterface.List[0].RANUENGAPID.Value)
	}

	if len(ran.RanUEs) != 1 {
		t.Fatalf("expected 1 UE to remain in the RAN, but got %d", len(ran.RanUEs))
	}
}

// TestHandleNGReset_PartOfNGInterface_UnknownUE verifies that a PartOfNGInterface
// reset referencing a RANUENGAPID that does not match any UE context does NOT
// panic or remove the wrong UE. This exercises the missing-continue bug where
// ranUe is nil after the lookup but Remove() is called anyway.
func TestHandleNGReset_PartOfNGInterface_UnknownUE(t *testing.T) {
	fakeNGAPSender := &FakeNGAPSender{}

	ran := &amf.Radio{
		Log:           logger.AmfLog,
		NGAPSender:    fakeNGAPSender,
		RanUEs:        map[int64]*amf.RanUe{},
		SupportedTAIs: make([]amf.SupportedTAI, 0),
	}

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

	if len(fakeNGAPSender.SentNGResetAcknowledges) != 1 {
		t.Fatalf("expected 1 NGResetAcknowledge, got %d", len(fakeNGAPSender.SentNGResetAcknowledges))
	}
}
