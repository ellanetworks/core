// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"context"
	"testing"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/sctp"
	"github.com/ellanetworks/core/ngap"
)

// TestHandleUEContextReleaseComplete_HandoverTargetNilTargetUe verifies that
// after a handover failure, the target UE (which only has SourceUe set, not
// TargetUe) can be cleanly released without panicking.
func TestHandleUEContextReleaseComplete_HandoverTargetNilTargetUe(t *testing.T) {
	amfInstance := newTestAMF()
	ran := newTestRadio(amfInstance)

	amfUe := amf.NewUeContext()
	amfUe.ForceStateForTest(amf.Registered)

	sourceUeConn := amf.NewUeConnForTest(ran, 1, 100, logger.AmfLog)
	sourceUeConn.AMFForTest().AttachUeConn(amfUe, sourceUeConn)

	targetUeConn := amf.NewUeConnForTest(ran, 2, 200, logger.AmfLog)

	err := amf.SetHandoverForTest(sourceUeConn, targetUeConn)
	if err != nil {
		t.Fatalf("SetHandoverForTest: %v", err)
	}

	targetUeConn.ReleaseAction = amf.UeContextReleaseHandover

	amfInstance.SetRadioForTest(new(sctp.SCTPConn), ran)

	amfID := ngap.AMFUENGAPID(200)
	ranID := ngap.RANUENGAPID(2)
	msg := &ngap.UEContextReleaseComplete{
		AMFUENGAPID: &amfID,
		RANUENGAPID: &ranID,
	}

	HandleUEContextReleaseComplete(context.Background(), amfInstance, ran, msg)

	if amfInstance.FindUEByRanUeNgapID(ran, targetUeConn.RanUeNgapID) != nil {
		t.Fatal("expected target UeConn to be removed after release complete")
	}
}

// TestHandleUEContextReleaseComplete_SmContextNotFound verifies that a
// UEContextReleaseComplete referencing a PDU session ID that has no SmContext
// does NOT panic.
func TestHandleUEContextReleaseComplete_SmContextNotFound(t *testing.T) {
	amfInstance := newTestAMF()
	ran := newTestRadio(amfInstance)

	amfUe := amf.NewUeContext()
	amfUe.ForceStateForTest(amf.Registered)

	ueConn := amf.NewUeConnForTest(ran, 1, 100, logger.AmfLog)
	ueConn.AMFForTest().AttachUeConn(amfUe, ueConn)

	amfInstance.SetRadioForTest(new(sctp.SCTPConn), ran)

	amfID := ngap.AMFUENGAPID(100)
	ranID := ngap.RANUENGAPID(1)
	msg := &ngap.UEContextReleaseComplete{
		AMFUENGAPID:            &amfID,
		RANUENGAPID:            &ranID,
		PDUSessionResourceList: ngap.PDUSessionResourceListCxtRelCpl{{PDUSessionID: 5}},
	}

	HandleUEContextReleaseComplete(context.Background(), amfInstance, ran, msg)

	if amfInstance.FindUEByRanUeNgapID(ran, ueConn.RanUeNgapID) != nil {
		t.Fatal("expected UeConn to be removed after release complete")
	}
}

// Both UE NGAP IDs are mandatory but ignore criticality, so an absent one
// reaches the handler. The procedure has no unsuccessful outcome, so the
// rejection is reported with an ERROR INDICATION (TS 38.413 §10.3.5).
func TestHandleUEContextReleaseComplete_MissingUENGAPIDs(t *testing.T) {
	amfInstance := newTestAMF()
	ran := newTestRadio(amfInstance)
	sender := ran.Conn.(*fakeNGAPSender)

	amfID := ngap.AMFUENGAPID(1)

	HandleUEContextReleaseComplete(context.Background(), amfInstance, ran,
		&ngap.UEContextReleaseComplete{AMFUENGAPID: &amfID})

	if len(sender.SentErrorIndications) != 1 {
		t.Fatalf("expected 1 ErrorIndication, got %d", len(sender.SentErrorIndications))
	}
}
