// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"bytes"
	"context"
	"testing"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/ngap"
)

func TestHandleUplinkNASTransport_UnknownUeConn_SendsErrorIndication(t *testing.T) {
	amfInstance := newTestAMF()
	ran := newTestRadio(amfInstance)
	sender := ran.Conn.(*fakeNGAPSender)

	HandleUplinkNASTransport(context.Background(), amfInstance, ran, &ngap.UplinkNASTransport{
		AMFUENGAPID: 1,
		RANUENGAPID: 1,
		NASPDU:      ngap.NASPDU{0x7E, 0x00, 0x55},
	})

	if len(sender.SentErrorIndications) != 1 {
		t.Fatalf("ErrorIndications sent = %d, want 1", len(sender.SentErrorIndications))
	}

	cause := sender.SentErrorIndications[0].Cause

	want := ngap.Cause{Group: ngap.CauseGroupRadioNetwork, Value: ngap.CauseRadioNetworkUnknownLocalUENGAPID}
	if cause == nil || *cause != want {
		t.Errorf("cause = %v, want unknown-local-UE-NGAP-ID", cause)
	}
}

func TestHandleUplinkNASTransport_UnknownAmfUeNgapID_SendsErrorIndication(t *testing.T) {
	fakeNAS := &fakeNASHandler{}
	amfInstance := newTestAMFWithNAS(fakeNAS)
	ran := newTestRadio(amfInstance)
	sender := ran.Conn.(*fakeNGAPSender)

	amf.NewUeConnForTest(ran, 1, 10, logger.AmfLog)

	HandleUplinkNASTransport(context.Background(), amfInstance, ran, &ngap.UplinkNASTransport{
		AMFUENGAPID: 99999,
		RANUENGAPID: 1,
		NASPDU:      ngap.NASPDU{0x7E, 0x00, 0x55},
	})

	errInd := assertSingleErrorIndication(t, sender, ngap.CauseRadioNetworkUnknownLocalUENGAPID)
	assertErrorIndicationEchoesIDs(t, errInd, 99999, 1)

	if len(fakeNAS.Calls) != 0 {
		t.Errorf("NAS handler must not be invoked on ID mismatch, got %d calls", len(fakeNAS.Calls))
	}
}

func TestHandleUplinkNASTransport_InconsistentRanUeNgapID_SendsErrorIndication(t *testing.T) {
	fakeNAS := &fakeNASHandler{}
	amfInstance := newTestAMFWithNAS(fakeNAS)
	ran := newTestRadio(amfInstance)
	sender := ran.Conn.(*fakeNGAPSender)

	amf.NewUeConnForTest(ran, 1, 10, logger.AmfLog)

	HandleUplinkNASTransport(context.Background(), amfInstance, ran, &ngap.UplinkNASTransport{
		AMFUENGAPID: 10,
		RANUENGAPID: 2,
		NASPDU:      ngap.NASPDU{0x7E, 0x00, 0x55},
	})

	errInd := assertSingleErrorIndication(t, sender, ngap.CauseRadioNetworkInconsistentRemoteUEID)
	assertErrorIndicationEchoesIDs(t, errInd, 10, 2)

	if len(fakeNAS.Calls) != 0 {
		t.Errorf("NAS handler must not be invoked on ID mismatch, got %d calls", len(fakeNAS.Calls))
	}
}

func TestHandleUplinkNASTransport_NilUeContext_RemovesUeConn(t *testing.T) {
	amfInstance := newTestAMF()
	ran := newTestRadio(amfInstance)

	amf.NewUeConnForTest(ran, 1, 10, logger.AmfLog)

	HandleUplinkNASTransport(context.Background(), amfInstance, ran, &ngap.UplinkNASTransport{
		AMFUENGAPID: 10,
		RANUENGAPID: 1,
		NASPDU:      ngap.NASPDU{0x7E, 0x00, 0x55},
	})

	if amfInstance.FindUEByRanUeNgapID(ran, 1) != nil {
		t.Error("expected UeConn to be removed from radio's map")
	}
}

func TestHandleUplinkNASTransport_HappyPath_NASDispatched(t *testing.T) {
	fakeNAS := &fakeNASHandler{}
	amfInstance := newTestAMFWithNAS(fakeNAS)

	ran := newTestRadio(amfInstance)

	amfUe := amf.NewUeContext()

	ueConn := amf.NewUeConnForTest(ran, 1, 10, logger.AmfLog)
	ueConn.AMFForTest().AttachUeConn(amfUe, ueConn)

	nasPDU := []byte{0xAA, 0xBB}

	HandleUplinkNASTransport(context.Background(), amfInstance, ran, &ngap.UplinkNASTransport{
		AMFUENGAPID: 10,
		RANUENGAPID: 1,
		NASPDU:      ngap.NASPDU(nasPDU),
	})

	if len(fakeNAS.Calls) != 1 {
		t.Fatalf("NAS calls = %d, want 1", len(fakeNAS.Calls))
	}

	if !bytes.Equal(fakeNAS.Calls[0].NASPDU, nasPDU) {
		t.Errorf("NAS PDU = %x, want %x", fakeNAS.Calls[0].NASPDU, nasPDU)
	}

	if ueConn.Radio() != ran {
		t.Error("ueConn.Radio not set to ran")
	}
}

func TestHandleUplinkNASTransport_LocationUpdatedBeforeNAS(t *testing.T) {
	fakeNAS := &fakeNASHandler{}
	amfInstance := newTestAMFWithNAS(fakeNAS)

	ran := newTestRadio(amfInstance)

	amfUe := amf.NewUeContext()

	ueConn := amf.NewUeConnForTest(ran, 1, 10, logger.AmfLog)
	ueConn.AMFForTest().AttachUeConn(amfUe, ueConn)

	HandleUplinkNASTransport(context.Background(), amfInstance, ran, &ngap.UplinkNASTransport{
		AMFUENGAPID: 10,
		RANUENGAPID: 1,
		NASPDU:      ngap.NASPDU{0xCC},
		UserLocationInformation: &ngap.UserLocationInformation{
			Kind: ngap.UserLocationNR, PLMNIdentity: ngap.PLMNIdentity{0x00, 0xf1, 0x10},
			CellIdentity: 0x123456789,
			TAI:          ngap.TAI{PLMNIdentity: ngap.PLMNIdentity{0x00, 0xf1, 0x10}, TAC: 1},
		},
	})

	if len(fakeNAS.Calls) != 1 {
		t.Fatalf("NAS calls = %d, want 1", len(fakeNAS.Calls))
	}
}
