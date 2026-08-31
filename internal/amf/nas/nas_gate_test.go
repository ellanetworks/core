// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"context"
	"testing"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/nasreply"
	"github.com/ellanetworks/core/nas"
	"github.com/ellanetworks/core/nas/fgs"
)

// TS 24.501 §5.6.1.5
func TestHandleServiceRequest_NoContext_SendsServiceReject(t *testing.T) {
	ngapSender := &fakeNGAPSender{}
	amfInstance := amf.New(&fakeDBInstance{
		Operator: &db.Operator{Mcc: "001", Mnc: "01", SupportedTACs: `["000001"]`},
	}, nil, nil)
	radio := amf.Radio{Log: logger.AmfLog, Conn: ngapSender}
	radio.BindAMFForTest(amfInstance)

	ueConn, err := amfInstance.NewUeConn(&radio, 0)
	if err != nil {
		t.Fatalf("could not create ueConn: %v", err)
	}

	HandleNAS(context.Background(), amfInstance, ueConn, encodePlainServiceRequest(t))

	if ueConn.UeContext() != nil {
		t.Fatal("no-context service request minted a UE context; the bare connection would leak")
	}

	if len(ngapSender.SentDownlinkNASTransport) != 1 {
		t.Fatalf("expected 1 downlink (SERVICE REJECT), got %d", len(ngapSender.SentDownlinkNASTransport))
	}

	pdu := ngapSender.SentDownlinkNASTransport[0].NASPDU
	if len(pdu) < 4 || pdu[2] != uint8(fgs.MsgServiceReject) {
		t.Fatalf("downlink is not a plain SERVICE REJECT: % x", pdu)
	}

	if pdu[3] != 0x09 {
		t.Errorf("5GMM cause = 0x%02x, want #9 (UE identity cannot be derived by the network)", pdu[3])
	}
}

// TS 24.501 §5.6.1.8
func TestHandleServiceRequest_ProtocolError_SendsServiceReject96(t *testing.T) {
	malformed := []byte{0x7e, 0x00, 0x4c}

	if !isServiceRequest(malformed) {
		t.Fatal("a truncated plain SERVICE REQUEST must still be recognized by message type")
	}

	ngapSender := &fakeNGAPSender{}
	amfInstance := amf.New(&fakeDBInstance{
		Operator: &db.Operator{Mcc: "001", Mnc: "01", SupportedTACs: `["000001"]`},
	}, nil, nil)
	radio := amf.Radio{Log: logger.AmfLog, Conn: ngapSender}
	radio.BindAMFForTest(amfInstance)

	ueConn, err := amfInstance.NewUeConn(&radio, 0)
	if err != nil {
		t.Fatalf("could not create ueConn: %v", err)
	}

	HandleNAS(context.Background(), amfInstance, ueConn, malformed)

	if len(ngapSender.SentDownlinkNASTransport) != 1 {
		t.Fatalf("expected 1 downlink (SERVICE REJECT), got %d", len(ngapSender.SentDownlinkNASTransport))
	}

	pdu := ngapSender.SentDownlinkNASTransport[0].NASPDU
	if len(pdu) < 4 || pdu[2] != uint8(fgs.MsgServiceReject) {
		t.Fatalf("downlink is not a plain SERVICE REJECT: % x", pdu)
	}

	if pdu[3] != 0x60 {
		t.Errorf("5GMM cause = 0x%02x, want #96 (invalid mandatory information)", pdu[3])
	}
}

func encodePlainServiceRequest(t *testing.T) []byte {
	t.Helper()

	sr := &fgs.ServiceRequest{
		ServiceType:    fgs.ServiceTypeSignalling,
		NgKSI:          nas.KeySetIdentifier{Value: 1},
		MobileIdentity: serviceRequest5GSTMSI(),
	}

	payload, err := sr.MarshalBinary()
	if err != nil {
		t.Fatalf("encode plain ServiceRequest: %v", err)
	}

	return payload
}

func TestHandleNAS_PlainNonRegistration_BindsNoContext(t *testing.T) {
	amfInstance := amf.New(nil, nil, nil)
	ue := &amf.UeConn{}

	HandleNAS(context.Background(), amfInstance, ue, encodePlainStatus5GMM(t))

	if ue.UeContext() != nil {
		t.Fatal("plain non-registration message minted a UE context; the bare connection would leak")
	}
}

// TS 24.501 §7.4
func TestHandleGmmMessage_UnimplementedType_ReturnsStatus97(t *testing.T) {
	ue, _, err := buildUeAndRadio()
	if err != nil {
		t.Fatal(err)
	}

	reject, err := (&fgs.RegistrationReject{Cause: fgs.GMMCausePLMNNotAllowed}).MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}

	d := HandleGmmMessage(context.Background(), amf.New(nil, nil, nil), ue, uint8(fgs.MsgRegistrationReject), reject, true, false)

	if d.Action != nasreply.ActionStatus || d.Domain != nasreply.DomainMM || d.Cause != nasreply.CauseMessageTypeNotImplemented {
		t.Errorf("disposition = %+v, want a 5GMM STATUS #97 (message type non-existent or not implemented)", d)
	}
}

// TS 24.501 §7.4
func TestDispositionForUnresolved_UnknownTypeStatus97(t *testing.T) {
	tests := []struct {
		name   string
		nasPdu []byte
		want   uint8
	}{
		{"unknown type 0xff", []byte{0x7e, 0x00, 0xff}, nasreply.CauseMessageTypeNotImplemented},
		{"defined type, malformed body", []byte{0x7e, 0x00, uint8(fgs.MsgRegistrationRequest)}, nasreply.CauseInvalidMandatoryInfo},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := dispositionForUnresolved(tt.nasPdu)
			if d.Action != nasreply.ActionStatus || d.Domain != nasreply.DomainMM || d.Cause != tt.want {
				t.Errorf("disposition = %+v, want a 5GMM STATUS cause #%d", d, tt.want)
			}
		})
	}
}

func encodePlainStatus5GMM(t *testing.T) []byte {
	t.Helper()

	payload, err := (&fgs.GMMStatus{}).MarshalBinary()
	if err != nil {
		t.Fatalf("encode plain 5GMM STATUS: %v", err)
	}

	return payload
}
