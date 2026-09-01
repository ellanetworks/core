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

// TS 38.413 §10.3.5
func TestHandleUEContextReleaseComplete_MissingUENGAPIDs(t *testing.T) {
	amfInstance := newTestAMF()
	ran := newTestRadio(amfInstance)
	sender := ran.Conn.(*fakeNGAPSender)

	amfID := ngap.AMFUENGAPID(1)

	HandleUEContextReleaseComplete(context.Background(), amfInstance, ran,
		&ngap.UEContextReleaseComplete{AMFUENGAPID: &amfID})

	if len(sender.SentErrorIndications) != 0 {
		t.Fatalf("expected no ErrorIndication, got %d", len(sender.SentErrorIndications))
	}
}

func TestHandleUEContextReleaseComplete_DeactivatesOnlyTheSessionsTheRANReported(t *testing.T) {
	smfSbi := &fakeSmfSbi{}
	amfInstance := newTestAMFWithSmfAndDB(smfSbi)
	ran := newTestRadio(amfInstance)

	amfUe := amf.NewUeContext()
	amfUe.ForceStateForTest(amf.Registered)
	amfUe.SmContextList[1] = &amf.SmContext{Ref: "ref-1", N2: amf.N2Active}
	amfUe.SmContextList[2] = &amf.SmContext{Ref: "ref-2", N2: amf.N2Active}

	ueConn := amf.NewUeConnForTest(ran, 1, 100, logger.AmfLog)
	ueConn.AMFForTest().AttachUeConn(amfUe, ueConn)
	amfInstance.SetRadioForTest(new(sctp.SCTPConn), ran)

	amfID := ngap.AMFUENGAPID(100)
	ranID := ngap.RANUENGAPID(1)

	HandleUEContextReleaseComplete(context.Background(), amfInstance, ran, &ngap.UEContextReleaseComplete{
		AMFUENGAPID:            &amfID,
		RANUENGAPID:            &ranID,
		PDUSessionResourceList: ngap.PDUSessionResourceListCxtRelCpl{{PDUSessionID: 1}},
	})

	if len(smfSbi.DeactivateSmContextCalls) != 1 || smfSbi.DeactivateSmContextCalls[0] != "ref-1" {
		t.Errorf("DeactivateSmContext calls = %v, want only ref-1: the NG-RAN node served only PDU session 1 (TS 23.502 4.2.6 step 5)",
			smfSbi.DeactivateSmContextCalls)
	}
}

func TestHandleUEContextReleaseComplete_NoReportedListDeactivatesEverySession(t *testing.T) {
	smfSbi := &fakeSmfSbi{}
	amfInstance := newTestAMFWithSmfAndDB(smfSbi)
	ran := newTestRadio(amfInstance)

	amfUe := amf.NewUeContext()
	amfUe.ForceStateForTest(amf.Registered)
	amfUe.SmContextList[1] = &amf.SmContext{Ref: "ref-1", N2: amf.N2Active}
	amfUe.SmContextList[2] = &amf.SmContext{Ref: "ref-2", N2: amf.N2Active}

	ueConn := amf.NewUeConnForTest(ran, 1, 100, logger.AmfLog)
	ueConn.AMFForTest().AttachUeConn(amfUe, ueConn)
	amfInstance.SetRadioForTest(new(sctp.SCTPConn), ran)

	amfID := ngap.AMFUENGAPID(100)
	ranID := ngap.RANUENGAPID(1)

	HandleUEContextReleaseComplete(context.Background(), amfInstance, ran, &ngap.UEContextReleaseComplete{
		AMFUENGAPID: &amfID,
		RANUENGAPID: &ranID,
	})

	if len(smfSbi.DeactivateSmContextCalls) != 2 {
		t.Errorf("DeactivateSmContext calls = %v, want both sessions when the NG-RAN node reports none",
			smfSbi.DeactivateSmContextCalls)
	}
}
