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

func TestHandleErrorIndication_EmptyIEs(t *testing.T) {
	amfInstance := newTestAMF()
	ran := newTestRadio(amfInstance)
	sender := ran.Conn.(*fakeNGAPSender)

	HandleErrorIndication(context.Background(), amfInstance, ran, &ngap.ErrorIndication{})

	if len(sender.SentErrorIndications) != 0 {
		t.Fatalf("expected no ErrorIndication, got %d", len(sender.SentErrorIndications))
	}
}

func TestHandleErrorIndication_WithCause(t *testing.T) {
	amfInstance := newTestAMF()
	ran := newTestRadio(amfInstance)
	sender := ran.Conn.(*fakeNGAPSender)

	msg := &ngap.ErrorIndication{
		Cause: ngap.Ptr(ngap.Cause{
			Group: ngap.CauseGroupRadioNetwork, Value: ngap.CauseRadioNetworkUnspecified,
		}),
	}

	HandleErrorIndication(context.Background(), amfInstance, ran, msg)

	if len(sender.SentErrorIndications) != 0 {
		t.Fatalf("expected no ErrorIndication sent back, got %d", len(sender.SentErrorIndications))
	}
}

// TS 38.413 §8.7
func TestHandleErrorIndication_ReleasesNamedUE(t *testing.T) {
	amfInstance := newTestAMF()
	ran := newTestRadio(amfInstance)
	sender := ran.Conn.(*fakeNGAPSender)
	ueConn := amf.NewUeConnForTest(ran, 2, 10, logger.AmfLog)

	amfID := ngap.AMFUENGAPID(10)
	msg := &ngap.ErrorIndication{
		AMFUENGAPID: &amfID,
		Cause: ngap.Ptr(ngap.Cause{
			Group: ngap.CauseGroupRadioNetwork, Value: ngap.CauseRadioNetworkUnspecified,
		}),
	}

	HandleErrorIndication(context.Background(), amfInstance, ran, msg)

	if len(sender.SentUEContextReleaseCommands) != 1 {
		t.Fatalf("expected the named UE released, got %d UEContextReleaseCommands", len(sender.SentUEContextReleaseCommands))
	}

	if ueConn.ReleaseAction != amf.UeContextN2NormalRelease {
		t.Fatalf("expected ReleaseAction = UeContextN2NormalRelease, got %d", ueConn.ReleaseAction)
	}
}

func TestHandleErrorIndication_UnknownUENoRelease(t *testing.T) {
	amfInstance := newTestAMF()
	ran := newTestRadio(amfInstance)
	sender := ran.Conn.(*fakeNGAPSender)

	amfID := ngap.AMFUENGAPID(999)
	msg := &ngap.ErrorIndication{
		AMFUENGAPID: &amfID,
		Cause: ngap.Ptr(ngap.Cause{
			Group: ngap.CauseGroupRadioNetwork, Value: ngap.CauseRadioNetworkUnspecified,
		}),
	}

	HandleErrorIndication(context.Background(), amfInstance, ran, msg)

	if len(sender.SentUEContextReleaseCommands) != 0 {
		t.Fatalf("expected no release for an unknown UE, got %d", len(sender.SentUEContextReleaseCommands))
	}
}

func TestHandleErrorIndication_WithCriticalityDiagnostics(t *testing.T) {
	amfInstance := newTestAMF()
	ran := newTestRadio(amfInstance)
	sender := ran.Conn.(*fakeNGAPSender)

	msg := &ngap.ErrorIndication{
		CriticalityDiagnostics: &ngap.CriticalityDiagnostics{},
	}

	HandleErrorIndication(context.Background(), amfInstance, ran, msg)

	if len(sender.SentErrorIndications) != 0 {
		t.Fatalf("expected no ErrorIndication sent back, got %d", len(sender.SentErrorIndications))
	}
}
