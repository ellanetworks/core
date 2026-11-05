// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"context"
	"testing"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/ngap"
	"github.com/ellanetworks/core/sctp"
)

func TestHandoverRequestAcknowledgeReleasesAStaleHolderOfTheRANUENGAPID(t *testing.T) {
	targetRan, sourceNGAPSender, amfInstance := setupHandoverAckTestContext(t)

	stale := amf.NewUeConnForTest(targetRan, 9, 50)
	targetUe := amfInstance.FindUEByAmfUeNgapID(targetRan, 1)

	msg := admittedAckMsg(t)
	msg.RANUENGAPID = ngap.Ptr(ngap.RANUENGAPID(9))

	HandleHandoverRequestAcknowledge(context.Background(), amfInstance, targetRan, msg)

	if len(sourceNGAPSender.SentHandoverCommands) != 1 {
		t.Fatalf("expected 1 HandoverCommand, got %d", len(sourceNGAPSender.SentHandoverCommands))
	}

	holders := amfInstance.ConnsByRanUeNgapID(targetRan, 9)
	if len(holders) != 1 || holders[0] != targetUe {
		t.Fatalf("RAN-UE-NGAP-ID 9 held by %v, want only the handover target", holders)
	}

	if amfInstance.LookupUeConn(stale.AmfUeNgapID) != nil {
		t.Fatal("the stale connection holding the RAN-UE-NGAP-ID was not released")
	}
}

func TestPathSwitchRequestReleasesAStaleHolderOfTheRANUENGAPID(t *testing.T) {
	const pduSessionID = uint8(1)

	sourceRan := &amf.Radio{Conn: &fakeNGAPSender{}}
	targetSender := &fakeNGAPSender{}
	targetRan := &amf.Radio{Conn: targetSender}

	amfUe := newValidUeContext()
	amfUe.SmContextList[pduSessionID] = &amf.SmContext{Ref: "imsi-001010000000001-1", Snssai: &models.Snssai{Sst: 1}}

	fakeSmf := &fakeSmfSbi{PathSwitchResponse: []byte{0xAA}}
	amfInstance := newTestAMFWithSmf(fakeSmf)
	amfInstance.SetRadioForTest(new(sctp.SCTPConn), sourceRan)
	amfInstance.SetRadioForTest(new(sctp.SCTPConn), targetRan)
	sourceRan.BindAMFForTest(amfInstance)
	targetRan.BindAMFForTest(amfInstance)

	ueConn := amf.NewUeConnForTest(sourceRan, 1, 10)
	ueConn.AMFForTest().AttachUeConn(t.Context(), amfUe, ueConn)

	stale := amf.NewUeConnForTest(targetRan, 2, 50)

	transfer, err := buildPathSwitchRequestTransfer(5000, []byte{10, 0, 0, 2})
	if err != nil {
		t.Fatal(err)
	}

	msg := buildPathSwitchRequest(ngap.Ptr(ngap.AMFUENGAPID(10)), ngap.Ptr(ngap.RANUENGAPID(2)),
		ngap.PDUSessionResourceToBeSwitchedDLList{{PDUSessionID: ngap.PDUSessionID(pduSessionID), Transfer: transfer}}, nil, nil)

	HandlePathSwitchRequest(context.Background(), amfInstance, targetRan, msg)

	if len(targetSender.SentPathSwitchRequestAcknowledges) != 1 {
		t.Fatalf("expected 1 PathSwitchRequestAcknowledge, got %d", len(targetSender.SentPathSwitchRequestAcknowledges))
	}

	holders := amfInstance.ConnsByRanUeNgapID(targetRan, 2)
	if len(holders) != 1 || holders[0] != ueConn {
		t.Fatalf("RAN-UE-NGAP-ID 2 held by %v, want only the switched connection", holders)
	}

	if amfInstance.LookupUeConn(stale.AmfUeNgapID) != nil {
		t.Fatal("the stale connection holding the RAN-UE-NGAP-ID was not released")
	}
}
