// Copyright 2026 Ella Networks

package ngap_test

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/amf/ngap"
	"github.com/ellanetworks/core/internal/amf/ngap/decode"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/free5gc/ngap/ngapType"
)

func TestHandleUplinkNasTransport_UnknownRanUe_SendsErrorIndication(t *testing.T) {
	ran := newTestRadio()
	sender := ran.NGAPSender.(*FakeNGAPSender)
	amfInstance := newTestAMF()

	ngap.HandleUplinkNasTransport(context.Background(), amfInstance, ran, decode.UplinkNASTransport{
		AMFUENGAPID: 1,
		RANUENGAPID: 1,
		NASPDU:      []byte{0x7E, 0x00, 0x55},
	})

	if len(sender.SentErrorIndications) != 1 {
		t.Fatalf("ErrorIndications sent = %d, want 1", len(sender.SentErrorIndications))
	}

	cause := sender.SentErrorIndications[0].Cause
	if cause == nil || cause.Present != ngapType.CausePresentRadioNetwork {
		t.Fatal("expected RadioNetwork cause")
	}

	if cause.RadioNetwork.Value != ngapType.CauseRadioNetworkPresentUnknownLocalUENGAPID {
		t.Errorf("cause = %d, want UnknownLocalUENGAPID (%d)",
			cause.RadioNetwork.Value, ngapType.CauseRadioNetworkPresentUnknownLocalUENGAPID)
	}
}

func TestHandleUplinkNasTransport_NilAmfUe_RemovesRanUe(t *testing.T) {
	ran := newTestRadio()
	amfInstance := newTestAMF()

	amf.NewRanUeForTest(ran, 1, 10, logger.AmfLog)

	ngap.HandleUplinkNasTransport(context.Background(), amfInstance, ran, decode.UplinkNASTransport{
		AMFUENGAPID: 10,
		RANUENGAPID: 1,
		NASPDU:      []byte{0x7E, 0x00, 0x55},
	})

	if _, exists := ran.RanUEs[1]; exists {
		t.Error("expected RanUe to be removed from radio's map")
	}
}

func TestHandleUplinkNasTransport_HappyPath_NASDispatched(t *testing.T) {
	fakeNAS := &FakeNASHandler{}
	amfInstance := newTestAMFWithNAS(fakeNAS)

	ran := newTestRadio()

	amfUe := amf.NewAmfUe()
	amfUe.Log = logger.AmfLog

	ranUe := amf.NewRanUeForTest(ran, 1, 10, logger.AmfLog)
	amfUe.AttachRanUe(ranUe)

	nasPDU := []byte{0xAA, 0xBB}

	ngap.HandleUplinkNasTransport(context.Background(), amfInstance, ran, decode.UplinkNASTransport{
		AMFUENGAPID: 10,
		RANUENGAPID: 1,
		NASPDU:      nasPDU,
	})

	if len(fakeNAS.Calls) != 1 {
		t.Fatalf("NAS calls = %d, want 1", len(fakeNAS.Calls))
	}

	if !bytes.Equal(fakeNAS.Calls[0].NASPDU, nasPDU) {
		t.Errorf("NAS PDU = %x, want %x", fakeNAS.Calls[0].NASPDU, nasPDU)
	}

	if ranUe.Radio() != ran {
		t.Error("ranUe.Radio not set to ran")
	}
}

func TestHandleUplinkNasTransport_LocationUpdatedBeforeNAS(t *testing.T) {
	fakeNAS := &FakeNASHandler{}
	amfInstance := newTestAMFWithNAS(fakeNAS)

	ran := newTestRadio()

	amfUe := amf.NewAmfUe()
	amfUe.Log = logger.AmfLog

	ranUe := amf.NewRanUeForTest(ran, 1, 10, logger.AmfLog)
	amfUe.AttachRanUe(ranUe)

	ngap.HandleUplinkNasTransport(context.Background(), amfInstance, ran, decode.UplinkNASTransport{
		AMFUENGAPID:             10,
		RANUENGAPID:             1,
		NASPDU:                  []byte{0xCC},
		UserLocationInformation: decode.UserLocationInformation{},
	})

	if len(fakeNAS.Calls) != 1 {
		t.Fatalf("NAS calls = %d, want 1", len(fakeNAS.Calls))
	}
}

func TestHandleUplinkNasTransport_NASError_SendsStatus5GMM(t *testing.T) {
	fakeNAS := &FakeNASHandler{Err: errors.New("NAS decode failed")}
	amfInstance := newTestAMFWithNAS(fakeNAS)

	ran := newTestRadio()
	sender := ran.NGAPSender.(*FakeNGAPSender)

	amfUe := amf.NewAmfUe()
	amfUe.Log = logger.AmfLog

	ranUe := amf.NewRanUeForTest(ran, 1, 10, logger.AmfLog)
	amfUe.AttachRanUe(ranUe)

	ngap.HandleUplinkNasTransport(context.Background(), amfInstance, ran, decode.UplinkNASTransport{
		AMFUENGAPID: 10,
		RANUENGAPID: 1,
		NASPDU:      []byte{0x7E, 0x00, 0xFF},
	})

	if len(fakeNAS.Calls) != 1 {
		t.Fatalf("NAS calls = %d, want 1", len(fakeNAS.Calls))
	}

	if len(sender.SentDownlinkNASTransport) != 1 {
		t.Fatalf("DownlinkNASTransport sent = %d, want 1", len(sender.SentDownlinkNASTransport))
	}

	nasPdu := sender.SentDownlinkNASTransport[0].NasPdu
	if len(nasPdu) < 4 {
		t.Fatalf("NAS PDU too short: %d bytes", len(nasPdu))
	}

	if nasPdu[2] != 0x64 {
		t.Errorf("NAS message type = 0x%02x, want 0x64 (5GMM STATUS)", nasPdu[2])
	}

	if nasPdu[3] != 0x6f {
		t.Errorf("5GMM cause = 0x%02x, want 0x6f (protocol error unspecified)", nasPdu[3])
	}
}
