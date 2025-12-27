// Copyright 2024 Ella Networks

package ngap_test

import (
	"context"
	"testing"

	ctxt "github.com/ellanetworks/core/internal/amf/context"
	"github.com/ellanetworks/core/internal/amf/ngap"
	"github.com/ellanetworks/core/internal/logger"
)

func TestHandleNGReset_ResetNGInterface(t *testing.T) {
	fakeNGAPSender := &FakeNGAPSender{}

	ran := &ctxt.AmfRan{
		Log:        logger.AmfLog,
		NGAPSender: fakeNGAPSender,
		RanUePool: map[int64]*ctxt.RanUe{
			0: {RanUeNgapID: 0, AmfUeNgapID: 0, Ran: &ctxt.AmfRan{}},
			1: {RanUeNgapID: 1, AmfUeNgapID: 1, Ran: &ctxt.AmfRan{}},
		},
		SupportedTAList: make([]ctxt.SupportedTAI, 0),
	}

	ran.RanUePool[0].Ran = ran
	ran.RanUePool[1].Ran = ran

	msg, err := buildNGReset(&NGResetOpts{
		ResetType: ResetTypePresentNGInterface,
	})
	if err != nil {
		t.Fatalf("failed to build NGSetupRequest: %v", err)
	}

	ngap.HandleNGReset(context.Background(), ran, msg)

	if len(fakeNGAPSender.SentNGResetAcknowledges) != 1 {
		t.Fatalf("expected 1 NGResetAcknowledge to be sent, but got %d", len(fakeNGAPSender.SentNGResetAcknowledges))
	}

	if fakeNGAPSender.SentNGResetAcknowledges[0].PartOfNGInterface != nil {
		t.Fatalf("expected PartOfNGInterface to be nil, but got %v", fakeNGAPSender.SentNGResetAcknowledges[0].PartOfNGInterface)
	}

	if len(ran.RanUePool) != 0 {
		t.Fatalf("expected all UEs to be removed from the RAN, but got %d", len(ran.RanUePool))
	}
}

func TestHandleNGReset_PartOfNGInterface(t *testing.T) {
	fakeNGAPSender := &FakeNGAPSender{}

	ran := &ctxt.AmfRan{
		Log:        logger.AmfLog,
		NGAPSender: fakeNGAPSender,
		RanUePool: map[int64]*ctxt.RanUe{
			0: {RanUeNgapID: 0, AmfUeNgapID: 0, Ran: &ctxt.AmfRan{}},
			1: {RanUeNgapID: 1, AmfUeNgapID: 1, Ran: &ctxt.AmfRan{}},
		},
		SupportedTAList: make([]ctxt.SupportedTAI, 0),
	}

	ran.RanUePool[0].Ran = ran
	ran.RanUePool[1].Ran = ran

	msg, err := buildNGReset(&NGResetOpts{
		ResetType: ResetTypePresentPartOfNGInterface,
		PartOfNGInterface: []NGInterface{
			{RanUENgapID: 0, AmfUENgapID: 0},
		},
	})
	if err != nil {
		t.Fatalf("failed to build NGSetupRequest: %v", err)
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

	if len(ran.RanUePool) != 1 {
		t.Fatalf("expected 1 UE to remain in the RAN, but got %d", len(ran.RanUePool))
	}
}
