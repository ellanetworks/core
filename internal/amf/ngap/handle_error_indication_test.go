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

func TestHandleErrorIndication_EmptyIEs(t *testing.T) {
	amfInstance := newTestAMF()
	ran := newTestRadio(amfInstance)
	sender := ran.Conn.(*fakeNGAPSender)

	ngap.HandleErrorIndication(context.Background(), amfInstance, ran, decode.ErrorIndication{})

	if len(sender.SentErrorIndications) != 0 {
		t.Fatalf("expected no ErrorIndication, got %d", len(sender.SentErrorIndications))
	}
}

func TestHandleErrorIndication_WithCause(t *testing.T) {
	amfInstance := newTestAMF()
	ran := newTestRadio(amfInstance)
	sender := ran.Conn.(*fakeNGAPSender)

	msg := decode.ErrorIndication{
		Cause: &ngapType.Cause{
			Present:      ngapType.CausePresentRadioNetwork,
			RadioNetwork: &ngapType.CauseRadioNetwork{Value: ngapType.CauseRadioNetworkPresentUnspecified},
		},
	}

	ngap.HandleErrorIndication(context.Background(), amfInstance, ran, msg)

	if len(sender.SentErrorIndications) != 0 {
		t.Fatalf("expected no ErrorIndication sent back, got %d", len(sender.SentErrorIndications))
	}
}

// TestHandleErrorIndication_ReleasesNamedUE verifies that an Error Indication naming
// a known UE releases it to CM-IDLE (protocol error → clean re-establish on the next
// Service Request), mirroring the MME. TS 38.413 §8.7 is silent on the receive action.
func TestHandleErrorIndication_ReleasesNamedUE(t *testing.T) {
	amfInstance := newTestAMF()
	ran := newTestRadio(amfInstance)
	sender := ran.Conn.(*fakeNGAPSender)
	ueConn := amf.NewUeConnForTest(ran, 2, 10, logger.AmfLog)

	amfID := int64(10)
	msg := decode.ErrorIndication{
		AMFUENGAPID: &amfID,
		Cause: &ngapType.Cause{
			Present:      ngapType.CausePresentRadioNetwork,
			RadioNetwork: &ngapType.CauseRadioNetwork{Value: ngapType.CauseRadioNetworkPresentUnspecified},
		},
	}

	ngap.HandleErrorIndication(context.Background(), amfInstance, ran, msg)

	if len(sender.SentUEContextReleaseCommands) != 1 {
		t.Fatalf("expected the named UE released, got %d UEContextReleaseCommands", len(sender.SentUEContextReleaseCommands))
	}

	if ueConn.ReleaseAction != amf.UeContextN2NormalRelease {
		t.Fatalf("expected ReleaseAction = UeContextN2NormalRelease, got %d", ueConn.ReleaseAction)
	}
}

// TestHandleErrorIndication_UnknownUENoRelease verifies an Error Indication naming an
// unknown UE releases nothing.
func TestHandleErrorIndication_UnknownUENoRelease(t *testing.T) {
	amfInstance := newTestAMF()
	ran := newTestRadio(amfInstance)
	sender := ran.Conn.(*fakeNGAPSender)

	amfID := int64(999)
	msg := decode.ErrorIndication{
		AMFUENGAPID: &amfID,
		Cause: &ngapType.Cause{
			Present:      ngapType.CausePresentRadioNetwork,
			RadioNetwork: &ngapType.CauseRadioNetwork{Value: ngapType.CauseRadioNetworkPresentUnspecified},
		},
	}

	ngap.HandleErrorIndication(context.Background(), amfInstance, ran, msg)

	if len(sender.SentUEContextReleaseCommands) != 0 {
		t.Fatalf("expected no release for an unknown UE, got %d", len(sender.SentUEContextReleaseCommands))
	}
}

func TestHandleErrorIndication_WithCriticalityDiagnostics(t *testing.T) {
	amfInstance := newTestAMF()
	ran := newTestRadio(amfInstance)
	sender := ran.Conn.(*fakeNGAPSender)

	msg := decode.ErrorIndication{
		CriticalityDiagnostics: &ngapType.CriticalityDiagnostics{},
	}

	ngap.HandleErrorIndication(context.Background(), amfInstance, ran, msg)

	if len(sender.SentErrorIndications) != 0 {
		t.Fatalf("expected no ErrorIndication sent back, got %d", len(sender.SentErrorIndications))
	}
}
