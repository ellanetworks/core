// Copyright 2026 Ella Networks

package ngap_test

import (
	"context"
	"testing"

	"github.com/ellanetworks/core/internal/amf/ngap"
	"github.com/ellanetworks/core/internal/amf/ngap/decode"
	"github.com/free5gc/ngap/ngapType"
)

// TestHandleRanConfigurationUpdate_NoSupportedTAs verifies that when the
// SupportedTAList IE is absent, the handler sends a RANConfigurationUpdateFailure
// with Misc/Unspecified cause and no acknowledge.
func TestHandleRanConfigurationUpdate_NoSupportedTAs(t *testing.T) {
	ran := newTestRadio()
	amfInstance := newTestAMF()
	sender := ran.NGAPSender.(*FakeNGAPSender)

	ngap.HandleRanConfigurationUpdate(context.Background(), amfInstance, ran, decode.RANConfigurationUpdate{})

	if len(sender.SentRanConfigurationUpdateAcks) != 0 {
		t.Fatalf("expected 0 acknowledges, got %d", len(sender.SentRanConfigurationUpdateAcks))
	}

	if len(sender.SentRanConfigurationUpdateFailures) != 1 {
		t.Fatalf("expected 1 failure, got %d", len(sender.SentRanConfigurationUpdateFailures))
	}

	failure := sender.SentRanConfigurationUpdateFailures[0]
	if failure.Cause.Present != ngapType.CausePresentMisc {
		t.Fatalf("expected Misc cause, got present=%d", failure.Cause.Present)
	}

	if failure.Cause.Misc == nil || failure.Cause.Misc.Value != ngapType.CauseMiscPresentUnspecified {
		t.Fatalf("expected Misc/Unspecified cause, got %+v", failure.Cause.Misc)
	}
}
