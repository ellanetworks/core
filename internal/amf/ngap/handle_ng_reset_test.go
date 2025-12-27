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
		// RanUePool:       make(map[int64]*ctxt.RanUe),
		RanUePool: map[int64]*ctxt.RanUe{
			0: {RanUeNgapID: 0, AmfUeNgapID: 0},
			1: {RanUeNgapID: 1, AmfUeNgapID: 1},
		},
		SupportedTAList: make([]ctxt.SupportedTAI, 0),
	}

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
		t.Errorf("expected PartOfNGInterface to be nil, but got %v", fakeNGAPSender.SentNGResetAcknowledges[0].PartOfNGInterface)
	}

	// Re-add this validation
	// if len(ran.RanUePool) != 0 {
	// 	t.Errorf("expected all UEs to be removed from the RAN, but got %d", len(ran.RanUePool))
	// }
}
