// Copyright 2026 Ella Networks

package ngap_test

import (
	"context"
	"testing"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/amf/ngap"
	"github.com/ellanetworks/core/internal/amf/ngap/decode"
	"github.com/ellanetworks/core/internal/amf/sctp"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/free5gc/ngap/ngapType"
)

// TestHandleUEContextReleaseComplete_HandoverTargetNilTargetUe verifies that
// after a handover failure, the target UE (which only has SourceUe set, not
// TargetUe) can be cleanly released without panicking.
func TestHandleUEContextReleaseComplete_HandoverTargetNilTargetUe(t *testing.T) {
	ran := newTestRadio()
	amfInstance := newTestAMF()

	amfUe := amf.NewAmfUe()
	amfUe.ForceState(amf.Registered)
	amfUe.Log = logger.AmfLog

	sourceRanUe := &amf.RanUe{
		RanUeNgapID: 1,
		AmfUeNgapID: 100,
		Radio:       ran,
		Log:         logger.AmfLog,
	}
	amfUe.AttachRanUe(sourceRanUe)
	ran.RanUEs[sourceRanUe.RanUeNgapID] = sourceRanUe

	targetRanUe := &amf.RanUe{
		RanUeNgapID: 2,
		AmfUeNgapID: 200,
		Radio:       ran,
		Log:         logger.AmfLog,
	}
	ran.RanUEs[targetRanUe.RanUeNgapID] = targetRanUe

	err := amf.AttachSourceUeTargetUe(sourceRanUe, targetRanUe)
	if err != nil {
		t.Fatalf("AttachSourceUeTargetUe: %v", err)
	}

	targetRanUe.ReleaseAction = amf.UeContextReleaseHandover

	amfInstance.Radios = map[*sctp.SCTPConn]*amf.Radio{new(sctp.SCTPConn): ran}

	amfID := int64(200)
	ranID := int64(2)
	msg := decode.UEContextReleaseComplete{
		AMFUENGAPID: &amfID,
		RANUENGAPID: &ranID,
	}

	ngap.HandleUEContextReleaseComplete(context.Background(), amfInstance, ran, msg)

	if _, exists := ran.RanUEs[targetRanUe.RanUeNgapID]; exists {
		t.Fatal("expected target RanUe to be removed after release complete")
	}
}

// TestHandleUEContextReleaseComplete_SmContextNotFound verifies that a
// UEContextReleaseComplete referencing a PDU session ID that has no SmContext
// does NOT panic.
func TestHandleUEContextReleaseComplete_SmContextNotFound(t *testing.T) {
	ran := newTestRadio()
	amfInstance := newTestAMF()

	amfUe := amf.NewAmfUe()
	amfUe.ForceState(amf.Registered)
	amfUe.Log = logger.AmfLog

	ranUe := &amf.RanUe{
		RanUeNgapID: 1,
		AmfUeNgapID: 100,
		Radio:       ran,
		Log:         logger.AmfLog,
	}
	amfUe.AttachRanUe(ranUe)
	ran.RanUEs[1] = ranUe

	amfInstance.Radios = map[*sctp.SCTPConn]*amf.Radio{new(sctp.SCTPConn): ran}

	amfID := int64(100)
	ranID := int64(1)
	msg := decode.UEContextReleaseComplete{
		AMFUENGAPID: &amfID,
		RANUENGAPID: &ranID,
		PDUSessionResourceList: &ngapType.PDUSessionResourceListCxtRelCpl{
			List: []ngapType.PDUSessionResourceItemCxtRelCpl{
				{
					PDUSessionID: ngapType.PDUSessionID{Value: 5},
				},
			},
		},
	}

	ngap.HandleUEContextReleaseComplete(context.Background(), amfInstance, ran, msg)

	if _, exists := ran.RanUEs[ranUe.RanUeNgapID]; exists {
		t.Fatal("expected RanUe to be removed after release complete")
	}
}
