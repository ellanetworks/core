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
	"github.com/ellanetworks/core/internal/sctp"
	"github.com/free5gc/ngap/ngapType"
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

	amfID := int64(200)
	ranID := int64(2)
	msg := decode.UEContextReleaseComplete{
		AMFUENGAPID: &amfID,
		RANUENGAPID: &ranID,
	}

	ngap.HandleUEContextReleaseComplete(context.Background(), amfInstance, ran, msg)

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

	if amfInstance.FindUEByRanUeNgapID(ran, ueConn.RanUeNgapID) != nil {
		t.Fatal("expected UeConn to be removed after release complete")
	}
}
