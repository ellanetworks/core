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
	"github.com/free5gc/nas"
	"github.com/free5gc/nas/nasMessage"
)

// A SERVICE REQUEST that resolves no 5GMM context (e.g. the UE deregistered, or an unknown
// 5G-S-TMSI) must be answered with a SERVICE REJECT #9, never dropped
// (TS 24.501 §5.6.1.5, §4.4.4.3). The dedicated pre-context handler never mints a context.
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

	HandleServiceRequest(context.Background(), amfInstance, ueConn, encodePlainServiceRequest(t))

	if ueConn.UeContext() != nil {
		t.Fatal("no-context service request minted a UE context; the bare connection would leak")
	}

	if len(ngapSender.SentDownlinkNASTransport) != 1 {
		t.Fatalf("expected 1 downlink (SERVICE REJECT), got %d", len(ngapSender.SentDownlinkNASTransport))
	}

	pdu := ngapSender.SentDownlinkNASTransport[0].NasPdu
	if len(pdu) < 4 || pdu[2] != nas.MsgTypeServiceReject {
		t.Fatalf("downlink is not a plain SERVICE REJECT: % x", pdu)
	}

	if pdu[3] != nasMessage.Cause5GMMUEIdentityCannotBeDerivedByTheNetwork {
		t.Errorf("5GMM cause = 0x%02x, want #9 (UE identity cannot be derived by the network)", pdu[3])
	}
}

// A recognizable but malformed SERVICE REQUEST (truncated — a protocol error) must still be
// classified by message type and answered with SERVICE REJECT #96 "invalid mandatory
// information", never dropped (TS 24.501 §5.6.1.8 b).
func TestHandleServiceRequest_ProtocolError_SendsServiceReject96(t *testing.T) {
	malformed := []byte{0x7e, 0x00, 0x4c} // plain 5GMM, message type ServiceRequest, no IEs

	if !IsServiceRequest(malformed) {
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

	HandleServiceRequest(context.Background(), amfInstance, ueConn, malformed)

	if len(ngapSender.SentDownlinkNASTransport) != 1 {
		t.Fatalf("expected 1 downlink (SERVICE REJECT), got %d", len(ngapSender.SentDownlinkNASTransport))
	}

	pdu := ngapSender.SentDownlinkNASTransport[0].NasPdu
	if len(pdu) < 4 || pdu[2] != nas.MsgTypeServiceReject {
		t.Fatalf("downlink is not a plain SERVICE REJECT: % x", pdu)
	}

	if pdu[3] != nasMessage.Cause5GMMInvalidMandatoryInformation {
		t.Errorf("5GMM cause = 0x%02x, want #96 (invalid mandatory information)", pdu[3])
	}
}

func encodePlainServiceRequest(t *testing.T) []byte {
	t.Helper()

	m := nas.NewMessage()
	m.GmmMessage = nas.NewGmmMessage()
	m.GmmHeader.SetMessageType(nas.MsgTypeServiceRequest)

	sr := nasMessage.NewServiceRequest(0)
	sr.SetExtendedProtocolDiscriminator(nasMessage.Epd5GSMobilityManagementMessage)
	sr.SetSecurityHeaderType(nas.SecurityHeaderTypePlainNas)
	sr.SetSpareHalfOctet(0)
	sr.SetMessageType(nas.MsgTypeServiceRequest)
	sr.SetServiceTypeValue(nasMessage.ServiceTypeSignalling)
	sr.SetNasKeySetIdentifiler(1)
	sr.TMSI5GS.SetLen(7)
	sr.SetTypeOfIdentity(4) // 5G-S-TMSI
	sr.SetAMFPointer(0)
	sr.SetAMFSetID(0)
	sr.SetTMSI5G([4]uint8{0xDE, 0xAD, 0xBE, 0xEF})

	m.ServiceRequest = sr

	payload, err := m.PlainNasEncode()
	if err != nil {
		t.Fatalf("encode plain ServiceRequest: %v", err)
	}

	return payload
}

// A plain NAS message on a fresh connection that is not a REGISTRATION REQUEST cannot
// establish a context; HandleNAS must reject it (error) and bind no UE context, so the
// NGAP layer releases the bare RAN connection and leaks no per-message context.
func TestHandleNAS_PlainNonRegistration_BindsNoContext(t *testing.T) {
	amfInstance := amf.New(nil, nil, nil)
	ue := &amf.UeConn{}

	HandleNAS(context.Background(), amfInstance, ue, encodePlainStatus5GMM(t))

	if ue.UeContext() != nil {
		t.Fatal("plain non-registration message minted a UE context; the bare connection would leak")
	}
}

// TS 24.501 §7.4: a message type the AMF does not implement inbound resolves to a 5GMM
// STATUS #97 disposition from the dispatcher, which the ingress finalizer sends. Mirrors the
// MME's EMM dispatch default.
func TestHandleGmmMessage_UnimplementedType_ReturnsStatus97(t *testing.T) {
	ue, _, err := buildUeAndRadio()
	if err != nil {
		t.Fatal(err)
	}

	msg := nas.NewGmmMessage()
	msg.SetMessageType(nas.MsgTypeRegistrationReject) // a downlink type never handled inbound

	d := HandleGmmMessage(context.Background(), amf.New(nil, nil, nil), ue, msg, nil, true)

	if d.Action != nasreply.ActionStatus || d.Domain != nasreply.DomainMM || d.Cause != nasreply.CauseMessageTypeNotImplemented {
		t.Errorf("disposition = %+v, want a 5GMM STATUS #97 (message type non-existent or not implemented)", d)
	}
}

// TS 24.501 §7.4: a fresh-connection 5GMM message with an undefined message type is
// answered with a 5GMM STATUS #97, whereas a defined type whose body is malformed draws
// #96 (§7.5.1).
func TestDispositionForUnresolved_UnknownTypeStatus97(t *testing.T) {
	tests := []struct {
		name   string
		nasPdu []byte
		want   uint8
	}{
		{"unknown type 0xff", []byte{0x7e, 0x00, 0xff}, nasreply.CauseMessageTypeNotImplemented},
		{"defined type, malformed body", []byte{0x7e, 0x00, nas.MsgTypeRegistrationRequest}, nasreply.CauseInvalidMandatoryInfo},
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

	m := nas.NewMessage()
	m.GmmMessage = nas.NewGmmMessage()
	m.GmmHeader.SetMessageType(nas.MsgTypeStatus5GMM)

	st := nasMessage.NewStatus5GMM(0)
	st.SetExtendedProtocolDiscriminator(nasMessage.Epd5GSMobilityManagementMessage)
	st.SetSecurityHeaderType(nas.SecurityHeaderTypePlainNas)
	st.SetSpareHalfOctet(0)
	st.SetMessageType(nas.MsgTypeStatus5GMM)

	m.Status5GMM = st

	payload, err := m.PlainNasEncode()
	if err != nil {
		t.Fatalf("encode plain 5GMM STATUS: %v", err)
	}

	return payload
}
