// Copyright 2026 Ella Networks

package gmm

import (
	"fmt"
	"testing"

	amfContext "github.com/ellanetworks/core/internal/amf/context"
	"github.com/ellanetworks/core/internal/models"
	"github.com/free5gc/nas"
	"github.com/free5gc/nas/nasMessage"
	"github.com/free5gc/nas/nasType"
)

// buildTestULNASTransport creates a ULNASTransport message with the given payload container type,
// payload contents, and optional PDU session ID. If pduSessionID is non-nil, it sets
// the PduSessionID2Value field.
func buildTestULNASTransport(payloadContainerType uint8, payload []byte, pduSessionID *uint8) *nasMessage.ULNASTransport {
	msg := nasMessage.NewULNASTransport(0)
	msg.SetExtendedProtocolDiscriminator(nasMessage.Epd5GSMobilityManagementMessage)
	msg.SetSecurityHeaderType(0)
	msg.SetMessageType(nas.MsgTypeULNASTransport)
	msg.SetPayloadContainerType(payloadContainerType)

	if len(payload) > 0 {
		msg.PayloadContainer.SetLen(uint16(len(payload)))
		msg.SetPayloadContainerContents(payload)
	}

	if pduSessionID != nil {
		msg.PduSessionID2Value = nasType.NewPduSessionID2Value(nasMessage.ULNASTransportPduSessionID2ValueType)
		msg.SetPduSessionID2Value(*pduSessionID)
	}

	return msg
}

func setRequestType(msg *nasMessage.ULNASTransport, requestTypeValue uint8) {
	msg.RequestType = nasType.NewRequestType(nasMessage.ULNASTransportRequestTypeType)
	msg.SetRequestTypeValue(requestTypeValue)
}

func setOldPDUSessionID(msg *nasMessage.ULNASTransport, id uint8) {
	msg.OldPDUSessionID = nasType.NewOldPDUSessionID(nasMessage.ULNASTransportOldPDUSessionIDType)
	msg.SetOldPDUSessionID(id)
}

func pduSessionIDPtr(id uint8) *uint8 {
	return &id
}

func TestHandleULNASTransport_WrongState_Error(t *testing.T) {
	testcases := []amfContext.StateType{amfContext.Deregistered, amfContext.Authentication, amfContext.SecurityMode, amfContext.ContextSetup}
	for _, tc := range testcases {
		t.Run(string(tc), func(t *testing.T) {
			ue, _, err := buildUeAndRadio()
			if err != nil {
				t.Fatalf("could not build UE and radio: %v", err)
			}

			ue.SetState(tc)

			msg := buildTestULNASTransport(nasMessage.PayloadContainerTypeN1SMInfo, []byte{0x01}, pduSessionIDPtr(1))

			err = handleULNASTransport(t.Context(), &amfContext.AMF{}, ue, msg)
			if err == nil {
				t.Fatal("expected an error, got nil")
			}
		})
	}
}

func TestHandleULNASTransport_MacFailed_Error(t *testing.T) {
	ue, _, err := buildUeAndRadio()
	if err != nil {
		t.Fatalf("could not build UE and radio: %v", err)
	}

	ue.SetState(amfContext.Registered)
	ue.MacFailed = true

	msg := buildTestULNASTransport(nasMessage.PayloadContainerTypeN1SMInfo, []byte{0x01}, pduSessionIDPtr(1))

	expected := "NAS message integrity check failed"

	err = handleULNASTransport(t.Context(), &amfContext.AMF{}, ue, msg)
	if err == nil || err.Error() != expected {
		t.Fatalf("expected error: %s, got: %v", expected, err)
	}
}

func TestHandleULNASTransport_PayloadContainerTypeSMS_Error(t *testing.T) {
	ue, _, err := buildUeAndRadio()
	if err != nil {
		t.Fatalf("could not build UE and radio: %v", err)
	}

	ue.SetState(amfContext.Registered)

	msg := buildTestULNASTransport(nasMessage.PayloadContainerTypeSMS, []byte{0x01}, nil)

	err = handleULNASTransport(t.Context(), &amfContext.AMF{}, ue, msg)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
}

func TestHandleULNASTransport_PayloadContainerTypeLPP_Error(t *testing.T) {
	ue, _, err := buildUeAndRadio()
	if err != nil {
		t.Fatalf("could not build UE and radio: %v", err)
	}

	ue.SetState(amfContext.Registered)

	msg := buildTestULNASTransport(nasMessage.PayloadContainerTypeLPP, []byte{0x01}, nil)

	err = handleULNASTransport(t.Context(), &amfContext.AMF{}, ue, msg)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
}

func TestHandleULNASTransport_PayloadContainerTypeSOR_Error(t *testing.T) {
	ue, _, err := buildUeAndRadio()
	if err != nil {
		t.Fatalf("could not build UE and radio: %v", err)
	}

	ue.SetState(amfContext.Registered)

	msg := buildTestULNASTransport(nasMessage.PayloadContainerTypeSOR, []byte{0x01}, nil)

	err = handleULNASTransport(t.Context(), &amfContext.AMF{}, ue, msg)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
}

func TestHandleULNASTransport_PayloadContainerTypeMultiplePayload_Error(t *testing.T) {
	ue, _, err := buildUeAndRadio()
	if err != nil {
		t.Fatalf("could not build UE and radio: %v", err)
	}

	ue.SetState(amfContext.Registered)

	msg := buildTestULNASTransport(nasMessage.PayloadContainerTypeMultiplePayload, []byte{0x01}, nil)

	err = handleULNASTransport(t.Context(), &amfContext.AMF{}, ue, msg)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
}

func TestHandleULNASTransport_PayloadContainerTypeUEPolicy_NoError(t *testing.T) {
	ue, _, err := buildUeAndRadio()
	if err != nil {
		t.Fatalf("could not build UE and radio: %v", err)
	}

	ue.SetState(amfContext.Registered)

	msg := buildTestULNASTransport(nasMessage.PayloadContainerTypeUEPolicy, []byte{0x01}, nil)

	err = handleULNASTransport(t.Context(), &amfContext.AMF{}, ue, msg)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}

func TestHandleULNASTransport_PayloadContainerTypeUEParameterUpdate_NoError(t *testing.T) {
	ue, _, err := buildUeAndRadio()
	if err != nil {
		t.Fatalf("could not build UE and radio: %v", err)
	}

	ue.SetState(amfContext.Registered)

	// UPU ACK: first byte 0x01, then 16 bytes of MAC = 17 bytes total
	upuAck := make([]byte, 17)
	upuAck[0] = 0x01

	msg := buildTestULNASTransport(nasMessage.PayloadContainerTypeUEParameterUpdate, upuAck, nil)

	err = handleULNASTransport(t.Context(), &amfContext.AMF{}, ue, msg)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}

func TestTransport5GSMMessage_NilPduSessionID_Error(t *testing.T) {
	ue, _, err := buildUeAndRadio()
	if err != nil {
		t.Fatalf("could not build UE and radio: %v", err)
	}

	// No PduSessionID2Value set
	msg := buildTestULNASTransport(nasMessage.PayloadContainerTypeN1SMInfo, []byte{0x01}, nil)

	expected := "pdu session id is nil"

	err = transport5GSMMessage(t.Context(), &amfContext.AMF{}, ue, msg)
	if err == nil || err.Error() != expected {
		t.Fatalf("expected error: %s, got: %v", expected, err)
	}
}

func TestTransport5GSMMessage_OldPDUSessionID_Error(t *testing.T) {
	ue, _, err := buildUeAndRadio()
	if err != nil {
		t.Fatalf("could not build UE and radio: %v", err)
	}

	msg := buildTestULNASTransport(nasMessage.PayloadContainerTypeN1SMInfo, []byte{0x01}, pduSessionIDPtr(1))
	setOldPDUSessionID(msg, 2)

	expected := "old pdu session id is not supported"

	err = transport5GSMMessage(t.Context(), &amfContext.AMF{}, ue, msg)
	if err == nil || err.Error() != expected {
		t.Fatalf("expected error: %s, got: %v", expected, err)
	}
}

func TestTransport5GSMMessage_SmContextNotExists_Status5GSM_NoError(t *testing.T) {
	ue, _, err := buildUeAndRadio()
	if err != nil {
		t.Fatalf("could not build UE and radio: %v", err)
	}

	// Build a valid 5GSM Status message as the payload
	// Status5GSM: EPD (0x2E) + PDU session ID (0x01) + PTI (0x00) + message type (0xD6) + cause (0x24)
	status5gsmPayload := []byte{0x2E, 0x01, 0x00, 0xD6, 0x24}

	msg := buildTestULNASTransport(nasMessage.PayloadContainerTypeN1SMInfo, status5gsmPayload, pduSessionIDPtr(1))

	err = transport5GSMMessage(t.Context(), &amfContext.AMF{}, ue, msg)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}

func TestTransport5GSMMessage_EmergencyRequest_SendsDLNASTransport(t *testing.T) {
	ue, ngapSender, err := buildUeAndRadio()
	if err != nil {
		t.Fatalf("could not build UE and radio: %v", err)
	}

	smPayload := []byte{0x2E, 0x01, 0x00, 0xC1, 0x00}

	msg := buildTestULNASTransport(nasMessage.PayloadContainerTypeN1SMInfo, smPayload, pduSessionIDPtr(1))
	setRequestType(msg, nasMessage.ULNASTransportRequestTypeInitialEmergencyRequest)

	err = transport5GSMMessage(t.Context(), &amfContext.AMF{}, ue, msg)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if len(ngapSender.SentDownlinkNASTransport) != 1 {
		t.Fatalf("expected 1 downlink NAS transport, got: %d", len(ngapSender.SentDownlinkNASTransport))
	}

	resp := ngapSender.SentDownlinkNASTransport[0]
	nm := new(nas.Message)
	nm.SecurityHeaderType = nas.GetSecurityHeaderType(resp.NasPdu) & 0x0f

	if nm.SecurityHeaderType != nas.SecurityHeaderTypePlainNas {
		t.Fatalf("expected a plain NAS message")
	}

	err = nm.PlainNasDecode(&resp.NasPdu)
	if err != nil {
		t.Fatalf("could not decode plain NAS message: %v", err)
	}

	if nm.GmmHeader.GetMessageType() != nas.MsgTypeDLNASTransport {
		t.Fatalf("expected DLNASTransport message, got: %v", nm.GmmHeader.GetMessageType())
	}
}

func TestTransport5GSMMessage_ExistingEmergencyPduSession_SendsDLNASTransport(t *testing.T) {
	ue, ngapSender, err := buildUeAndRadio()
	if err != nil {
		t.Fatalf("could not build UE and radio: %v", err)
	}

	smPayload := []byte{0x2E, 0x01, 0x00, 0xC1, 0x00}

	msg := buildTestULNASTransport(nasMessage.PayloadContainerTypeN1SMInfo, smPayload, pduSessionIDPtr(1))
	setRequestType(msg, nasMessage.ULNASTransportRequestTypeExistingEmergencyPduSession)

	err = transport5GSMMessage(t.Context(), &amfContext.AMF{}, ue, msg)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if len(ngapSender.SentDownlinkNASTransport) != 1 {
		t.Fatalf("expected 1 downlink NAS transport, got: %d", len(ngapSender.SentDownlinkNASTransport))
	}

	resp := ngapSender.SentDownlinkNASTransport[0]
	nm := new(nas.Message)
	nm.SecurityHeaderType = nas.GetSecurityHeaderType(resp.NasPdu) & 0x0f

	if nm.SecurityHeaderType != nas.SecurityHeaderTypePlainNas {
		t.Fatalf("expected a plain NAS message")
	}

	err = nm.PlainNasDecode(&resp.NasPdu)
	if err != nil {
		t.Fatalf("could not decode plain NAS message: %v", err)
	}

	if nm.GmmHeader.GetMessageType() != nas.MsgTypeDLNASTransport {
		t.Fatalf("expected DLNASTransport message, got: %v", nm.GmmHeader.GetMessageType())
	}
}

func TestTransport5GSMMessage_ExistingPduSession_NotAllowedNssai_SendsDLNASTransport(t *testing.T) {
	ue, ngapSender, err := buildUeAndRadio()
	if err != nil {
		t.Fatalf("could not build UE and radio: %v", err)
	}

	// Set the UE's allowed NSSAI to SST=1, SD="010203"
	ue.AllowedNssai = &models.Snssai{Sst: 1, Sd: "010203"}

	// Create an SM context with a DIFFERENT NSSAI (SST=2)
	var pduSessionID uint8 = 5

	_ = ue.CreateSmContext(pduSessionID, "testref", &models.Snssai{Sst: 2, Sd: "040506"})

	smPayload := []byte{0x2E, 0x01, 0x00, 0xC1, 0x00}

	msg := buildTestULNASTransport(nasMessage.PayloadContainerTypeN1SMInfo, smPayload, pduSessionIDPtr(pduSessionID))
	setRequestType(msg, nasMessage.ULNASTransportRequestTypeExistingPduSession)

	err = transport5GSMMessage(t.Context(), &amfContext.AMF{}, ue, msg)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if len(ngapSender.SentDownlinkNASTransport) != 1 {
		t.Fatalf("expected 1 downlink NAS transport, got: %d", len(ngapSender.SentDownlinkNASTransport))
	}

	resp := ngapSender.SentDownlinkNASTransport[0]
	nm := new(nas.Message)
	nm.SecurityHeaderType = nas.GetSecurityHeaderType(resp.NasPdu) & 0x0f

	if nm.SecurityHeaderType != nas.SecurityHeaderTypePlainNas {
		t.Fatalf("expected a plain NAS message")
	}

	err = nm.PlainNasDecode(&resp.NasPdu)
	if err != nil {
		t.Fatalf("could not decode plain NAS message: %v", err)
	}

	if nm.GmmHeader.GetMessageType() != nas.MsgTypeDLNASTransport {
		t.Fatalf("expected DLNASTransport message, got: %v", nm.GmmHeader.GetMessageType())
	}
}

func TestTransport5GSMMessage_NoSmContext_ModificationRequest_SendsDLNASTransport(t *testing.T) {
	ue, ngapSender, err := buildUeAndRadio()
	if err != nil {
		t.Fatalf("could not build UE and radio: %v", err)
	}

	// No SM context for this PDU session ID
	smPayload := []byte{0x2E, 0x01, 0x00, 0xC1, 0x00}

	msg := buildTestULNASTransport(nasMessage.PayloadContainerTypeN1SMInfo, smPayload, pduSessionIDPtr(1))
	setRequestType(msg, nasMessage.ULNASTransportRequestTypeModificationRequest)

	err = transport5GSMMessage(t.Context(), &amfContext.AMF{}, ue, msg)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if len(ngapSender.SentDownlinkNASTransport) != 1 {
		t.Fatalf("expected 1 downlink NAS transport, got: %d", len(ngapSender.SentDownlinkNASTransport))
	}

	resp := ngapSender.SentDownlinkNASTransport[0]
	nm := new(nas.Message)
	nm.SecurityHeaderType = nas.GetSecurityHeaderType(resp.NasPdu) & 0x0f

	if nm.SecurityHeaderType != nas.SecurityHeaderTypePlainNas {
		t.Fatalf("expected a plain NAS message")
	}

	err = nm.PlainNasDecode(&resp.NasPdu)
	if err != nil {
		t.Fatalf("could not decode plain NAS message: %v", err)
	}

	if nm.GmmHeader.GetMessageType() != nas.MsgTypeDLNASTransport {
		t.Fatalf("expected DLNASTransport message, got: %v", nm.GmmHeader.GetMessageType())
	}
}

func TestTransport5GSMMessage_NoSmContext_ExistingPduSession_SendsDLNASTransport(t *testing.T) {
	ue, ngapSender, err := buildUeAndRadio()
	if err != nil {
		t.Fatalf("could not build UE and radio: %v", err)
	}

	// No SM context for this PDU session ID
	smPayload := []byte{0x2E, 0x01, 0x00, 0xC1, 0x00}

	msg := buildTestULNASTransport(nasMessage.PayloadContainerTypeN1SMInfo, smPayload, pduSessionIDPtr(1))
	setRequestType(msg, nasMessage.ULNASTransportRequestTypeExistingPduSession)

	err = transport5GSMMessage(t.Context(), &amfContext.AMF{}, ue, msg)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if len(ngapSender.SentDownlinkNASTransport) != 1 {
		t.Fatalf("expected 1 downlink NAS transport, got: %d", len(ngapSender.SentDownlinkNASTransport))
	}

	resp := ngapSender.SentDownlinkNASTransport[0]
	nm := new(nas.Message)
	nm.SecurityHeaderType = nas.GetSecurityHeaderType(resp.NasPdu) & 0x0f

	if nm.SecurityHeaderType != nas.SecurityHeaderTypePlainNas {
		t.Fatalf("expected a plain NAS message")
	}

	err = nm.PlainNasDecode(&resp.NasPdu)
	if err != nil {
		t.Fatalf("could not decode plain NAS message: %v", err)
	}

	if nm.GmmHeader.GetMessageType() != nas.MsgTypeDLNASTransport {
		t.Fatalf("expected DLNASTransport message, got: %v", nm.GmmHeader.GetMessageType())
	}
}

func TestTransport5GSMMessage_SmContextExists_InitialRequest_DeletesContextAndCreateNewOne(t *testing.T) {
	ue, _, err := buildUeAndRadio()
	if err != nil {
		t.Fatalf("could not build UE and radio: %v", err)
	}

	ue.Supi = mustSUPIFromPrefixed("imsi-001010000000001")

	var pduSessionID uint8 = 3

	snssai := &models.Snssai{Sst: 1, Sd: "010203"}
	_ = ue.CreateSmContext(pduSessionID, "testref", snssai)

	smPayload := []byte{0x2E, 0x03, 0x00, 0xC1, 0x00}

	msg := buildTestULNASTransport(nasMessage.PayloadContainerTypeN1SMInfo, smPayload, pduSessionIDPtr(pduSessionID))
	setRequestType(msg, nasMessage.ULNASTransportRequestTypeInitialRequest)

	fakeSmf := &FakeSmf{
		CreateSmContextRef: "new-ref-123",
	}

	amf := &amfContext.AMF{
		Smf:        fakeSmf,
		DBInstance: &FakeDBInstance{},
	}

	ue.AllowedNssai = snssai

	err = transport5GSMMessage(t.Context(), amf, ue, msg)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// The old context should have been deleted and a new one created
	smCtx, exists := ue.SmContextFindByPDUSessionID(pduSessionID)
	if !exists {
		t.Fatal("expected SM context to exist for the PDU session ID after re-creation")
	}

	if smCtx.Ref != "new-ref-123" {
		t.Fatalf("expected SM context ref to be 'new-ref-123', got: %s", smCtx.Ref)
	}

	if len(fakeSmf.CreateSmContextCalls) != 1 {
		t.Fatalf("expected 1 CreateSmContext call, got: %d", len(fakeSmf.CreateSmContextCalls))
	}
}

func TestForward5GSMMessageToSMF_UpdateError_ReturnsError(t *testing.T) {
	ue, _, err := buildUeAndRadio()
	if err != nil {
		t.Fatalf("could not build UE and radio: %v", err)
	}

	fakeSmf := &FakeSmf{
		UpdateN1MsgError: fmt.Errorf("smf unavailable"),
	}

	amf := &amfContext.AMF{Smf: fakeSmf}

	err = forward5GSMMessageToSMF(t.Context(), amf, ue, 1, "ref-1", []byte{0x01})
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	if len(fakeSmf.UpdateN1MsgCalls) != 1 {
		t.Fatalf("expected 1 UpdateSmContextN1Msg call, got: %d", len(fakeSmf.UpdateN1MsgCalls))
	}
}

func TestForward5GSMMessageToSMF_NilResponse_NoError(t *testing.T) {
	ue, _, err := buildUeAndRadio()
	if err != nil {
		t.Fatalf("could not build UE and radio: %v", err)
	}

	fakeSmf := &FakeSmf{
		UpdateN1MsgResponse: nil,
	}

	amf := &amfContext.AMF{Smf: fakeSmf}

	err = forward5GSMMessageToSMF(t.Context(), amf, ue, 1, "ref-1", []byte{0x01})
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}

func TestForward5GSMMessageToSMF_N1Only_SendsDLNASTransport(t *testing.T) {
	ue, ngapSender, err := buildUeAndRadio()
	if err != nil {
		t.Fatalf("could not build UE and radio: %v", err)
	}

	fakeSmf := &FakeSmf{
		UpdateN1MsgResponse: &models.UpdateSmContextResponse{
			BinaryDataN1SmMessage: []byte{0x2E, 0x01, 0x00, 0xD6, 0x24},
		},
	}

	amf := &amfContext.AMF{Smf: fakeSmf}

	err = forward5GSMMessageToSMF(t.Context(), amf, ue, 1, "ref-1", []byte{0x01})
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if len(ngapSender.SentDownlinkNASTransport) != 1 {
		t.Fatalf("expected 1 downlink NAS transport, got: %d", len(ngapSender.SentDownlinkNASTransport))
	}

	resp := ngapSender.SentDownlinkNASTransport[0]
	nm := new(nas.Message)
	nm.SecurityHeaderType = nas.GetSecurityHeaderType(resp.NasPdu) & 0x0f

	if nm.SecurityHeaderType != nas.SecurityHeaderTypePlainNas {
		t.Fatalf("expected a plain NAS message")
	}

	err = nm.PlainNasDecode(&resp.NasPdu)
	if err != nil {
		t.Fatalf("could not decode plain NAS message: %v", err)
	}

	if nm.GmmHeader.GetMessageType() != nas.MsgTypeDLNASTransport {
		t.Fatalf("expected DLNASTransport message, got: %v", nm.GmmHeader.GetMessageType())
	}
}

func TestForward5GSMMessageToSMF_N2NotPduResRel_ReturnsNil(t *testing.T) {
	ue, ngapSender, err := buildUeAndRadio()
	if err != nil {
		t.Fatalf("could not build UE and radio: %v", err)
	}

	fakeSmf := &FakeSmf{
		UpdateN1MsgResponse: &models.UpdateSmContextResponse{
			BinaryDataN2SmInformation: []byte{0x01, 0x02},
			N2SmInfoTypePduResRel:     false,
		},
	}

	amf := &amfContext.AMF{Smf: fakeSmf}

	err = forward5GSMMessageToSMF(t.Context(), amf, ue, 1, "ref-1", []byte{0x01})
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// No NGAP message should have been sent
	if len(ngapSender.SentDownlinkNASTransport) != 0 {
		t.Fatalf("expected 0 downlink NAS transport, got: %d", len(ngapSender.SentDownlinkNASTransport))
	}

	if len(ngapSender.SentPDUSessionResourceReleaseCommand) != 0 {
		t.Fatalf("expected 0 release commands, got: %d", len(ngapSender.SentPDUSessionResourceReleaseCommand))
	}
}

func TestForward5GSMMessageToSMF_N2PduResRel_SendsReleaseCommand(t *testing.T) {
	ue, ngapSender, err := buildUeAndRadio()
	if err != nil {
		t.Fatalf("could not build UE and radio: %v", err)
	}

	n2Data := []byte{0x00, 0x00, 0x04, 0x00, 0x00, 0x00, 0x00, 0x00}

	fakeSmf := &FakeSmf{
		UpdateN1MsgResponse: &models.UpdateSmContextResponse{
			BinaryDataN2SmInformation: n2Data,
			N2SmInfoTypePduResRel:     true,
		},
	}

	amf := &amfContext.AMF{Smf: fakeSmf}

	err = forward5GSMMessageToSMF(t.Context(), amf, ue, 1, "ref-1", []byte{0x01})
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if len(ngapSender.SentPDUSessionResourceReleaseCommand) != 1 {
		t.Fatalf("expected 1 release command, got: %d", len(ngapSender.SentPDUSessionResourceReleaseCommand))
	}
}

func TestForward5GSMMessageToSMF_N1AndN2PduResRel_SendsReleaseCommandWithN1(t *testing.T) {
	ue, ngapSender, err := buildUeAndRadio()
	if err != nil {
		t.Fatalf("could not build UE and radio: %v", err)
	}

	n2Data := []byte{0x00, 0x00, 0x04, 0x00, 0x00, 0x00, 0x00, 0x00}

	fakeSmf := &FakeSmf{
		UpdateN1MsgResponse: &models.UpdateSmContextResponse{
			BinaryDataN1SmMessage:     []byte{0x2E, 0x01, 0x00, 0xD6, 0x24},
			BinaryDataN2SmInformation: n2Data,
			N2SmInfoTypePduResRel:     true,
		},
	}

	amf := &amfContext.AMF{Smf: fakeSmf}

	err = forward5GSMMessageToSMF(t.Context(), amf, ue, 1, "ref-1", []byte{0x01})
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if len(ngapSender.SentPDUSessionResourceReleaseCommand) != 1 {
		t.Fatalf("expected 1 release command, got: %d", len(ngapSender.SentPDUSessionResourceReleaseCommand))
	}

	relCmd := ngapSender.SentPDUSessionResourceReleaseCommand[0]
	if relCmd.NasPdu == nil {
		t.Fatal("expected NAS PDU in release command, got nil")
	}

	// No DL NAS Transport should have been sent separately (the N1 goes in the release command)
	if len(ngapSender.SentDownlinkNASTransport) != 0 {
		t.Fatalf("expected 0 separate downlink NAS transport, got: %d", len(ngapSender.SentDownlinkNASTransport))
	}
}

func TestTransport5GSMMessage_SmContextExists_NoRequestType_ForwardsToSMF(t *testing.T) {
	ue, _, err := buildUeAndRadio()
	if err != nil {
		t.Fatalf("could not build UE and radio: %v", err)
	}

	var pduSessionID uint8 = 5

	_ = ue.CreateSmContext(pduSessionID, "test-ref", &models.Snssai{Sst: 1, Sd: "010203"})

	smPayload := []byte{0x2E, 0x05, 0x00, 0xC1, 0x00}

	msg := buildTestULNASTransport(nasMessage.PayloadContainerTypeN1SMInfo, smPayload, pduSessionIDPtr(pduSessionID))
	// No request type set

	fakeSmf := &FakeSmf{
		UpdateN1MsgResponse: nil, // nil response
	}

	amf := &amfContext.AMF{Smf: fakeSmf}

	err = transport5GSMMessage(t.Context(), amf, ue, msg)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if len(fakeSmf.UpdateN1MsgCalls) != 1 {
		t.Fatalf("expected 1 UpdateSmContextN1Msg call, got: %d", len(fakeSmf.UpdateN1MsgCalls))
	}

	if fakeSmf.UpdateN1MsgCalls[0].SmContextRef != "test-ref" {
		t.Fatalf("expected SmContextRef 'test-ref', got: %s", fakeSmf.UpdateN1MsgCalls[0].SmContextRef)
	}
}

func TestTransport5GSMMessage_SmContextExists_DuplicatePDU_Success(t *testing.T) {
	// When smContextExist=true and requestType=InitialRequest, the code first
	// deletes the local SM context, sets smContextExist=false,
	// then falls into the !smContextExist branch, which calls CreateSmContext.
	// The UpdateSmContextCauseDuplicatePDUSessionID path is unreachable
	// because the context is always deleted first. This test verifies the actual
	// behavior: delete + CreateSmContext.
	ue, ngapSender, err := buildUeAndRadio()
	if err != nil {
		t.Fatalf("could not build UE and radio: %v", err)
	}

	var pduSessionID uint8 = 3

	snssai := &models.Snssai{Sst: 1, Sd: "010203"}
	_ = ue.CreateSmContext(pduSessionID, "dup-ref", snssai)

	// Also create a second SM context that will NOT be deleted (to ensure the first IS deleted)
	_ = ue.CreateSmContext(7, "other-ref", snssai)

	smPayload := []byte{0x2E, 0x03, 0x00, 0xC1, 0x00}

	msg := buildTestULNASTransport(nasMessage.PayloadContainerTypeN1SMInfo, smPayload, pduSessionIDPtr(pduSessionID))
	setRequestType(msg, nasMessage.ULNASTransportRequestTypeInitialRequest)

	fakeSmf := &FakeSmf{
		CreateSmContextRef: "new-ref-after-dup",
	}

	amf := &amfContext.AMF{
		Smf:        fakeSmf,
		DBInstance: &FakeDBInstance{},
	}

	ue.Supi = mustSUPIFromPrefixed("imsi-001010000000001")
	ue.AllowedNssai = snssai

	err = transport5GSMMessage(t.Context(), amf, ue, msg)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// The duplicate PDU call should NOT have been made (dead code path)
	if len(fakeSmf.DuplicatePDUCalls) != 0 {
		t.Fatalf("expected 0 DuplicatePDU calls, got: %d", len(fakeSmf.DuplicatePDUCalls))
	}

	// CreateSmContext should have been called instead
	if len(fakeSmf.CreateSmContextCalls) != 1 {
		t.Fatalf("expected 1 CreateSmContext call, got: %d", len(fakeSmf.CreateSmContextCalls))
	}

	// No PDU Session Resource Release Command should have been sent
	if len(ngapSender.SentPDUSessionResourceReleaseCommand) != 0 {
		t.Fatalf("expected 0 release commands, got: %d", len(ngapSender.SentPDUSessionResourceReleaseCommand))
	}

	// SM context should exist with the new ref
	smCtx, exists := ue.SmContextFindByPDUSessionID(pduSessionID)
	if !exists {
		t.Fatal("expected SM context to exist after re-creation")
	}

	if smCtx.Ref != "new-ref-after-dup" {
		t.Fatalf("expected SM context ref 'new-ref-after-dup', got: %s", smCtx.Ref)
	}

	// The other SM context should still exist
	_, exists = ue.SmContextFindByPDUSessionID(7)
	if !exists {
		t.Fatal("expected SM context for PDU session 7 to still exist")
	}
}

func TestTransport5GSMMessage_SmContextExists_ExistingPduSession_AllowedNssai_ForwardsToSMF(t *testing.T) {
	ue, _, err := buildUeAndRadio()
	if err != nil {
		t.Fatalf("could not build UE and radio: %v", err)
	}

	snssai := &models.Snssai{Sst: 1, Sd: "010203"}
	ue.AllowedNssai = snssai

	var pduSessionID uint8 = 5

	_ = ue.CreateSmContext(pduSessionID, "existing-ref", snssai)

	smPayload := []byte{0x2E, 0x05, 0x00, 0xC1, 0x00}

	msg := buildTestULNASTransport(nasMessage.PayloadContainerTypeN1SMInfo, smPayload, pduSessionIDPtr(pduSessionID))
	setRequestType(msg, nasMessage.ULNASTransportRequestTypeExistingPduSession)

	fakeSmf := &FakeSmf{
		UpdateN1MsgResponse: nil,
	}

	amf := &amfContext.AMF{Smf: fakeSmf}

	err = transport5GSMMessage(t.Context(), amf, ue, msg)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if len(fakeSmf.UpdateN1MsgCalls) != 1 {
		t.Fatalf("expected 1 UpdateSmContextN1Msg call, got: %d", len(fakeSmf.UpdateN1MsgCalls))
	}

	if fakeSmf.UpdateN1MsgCalls[0].SmContextRef != "existing-ref" {
		t.Fatalf("expected SmContextRef 'existing-ref', got: %s", fakeSmf.UpdateN1MsgCalls[0].SmContextRef)
	}
}

func TestTransport5GSMMessage_SmContextExists_DefaultRequestType_ForwardsToSMF(t *testing.T) {
	ue, _, err := buildUeAndRadio()
	if err != nil {
		t.Fatalf("could not build UE and radio: %v", err)
	}

	var pduSessionID uint8 = 5

	_ = ue.CreateSmContext(pduSessionID, "default-ref", &models.Snssai{Sst: 1, Sd: "010203"})

	smPayload := []byte{0x2E, 0x05, 0x00, 0xC1, 0x00}

	msg := buildTestULNASTransport(nasMessage.PayloadContainerTypeN1SMInfo, smPayload, pduSessionIDPtr(pduSessionID))
	// Set an unusual request type value (7 is not one of the defined cases) to trigger the default case
	setRequestType(msg, 7)

	fakeSmf := &FakeSmf{
		UpdateN1MsgResponse: nil,
	}

	amf := &amfContext.AMF{Smf: fakeSmf}

	err = transport5GSMMessage(t.Context(), amf, ue, msg)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if len(fakeSmf.UpdateN1MsgCalls) != 1 {
		t.Fatalf("expected 1 UpdateSmContextN1Msg call, got: %d", len(fakeSmf.UpdateN1MsgCalls))
	}
}

func TestTransport5GSMMessage_NoSmContext_InitialRequest_WithSNSSAIAndDNN_CreateSmContext(t *testing.T) {
	ue, _, err := buildUeAndRadio()
	if err != nil {
		t.Fatalf("could not build UE and radio: %v", err)
	}

	ue.Supi = mustSUPIFromPrefixed("imsi-001010000000001")

	var pduSessionID uint8 = 1

	smPayload := []byte{0x2E, 0x01, 0x00, 0xC1, 0x00}

	msg := buildTestULNASTransport(nasMessage.PayloadContainerTypeN1SMInfo, smPayload, pduSessionIDPtr(pduSessionID))
	setRequestType(msg, nasMessage.ULNASTransportRequestTypeInitialRequest)

	// Set SNSSAI on the NAS message: IEI=0x22, Len=4, SST=1, SD=0x01,0x02,0x03
	msg.SNSSAI = nasType.NewSNSSAI(nasMessage.ULNASTransportSNSSAIType)
	msg.SNSSAI.SetLen(4)
	msg.SetSST(1)
	msg.SetSD([3]uint8{0x01, 0x02, 0x03})

	// Set DNN on the NAS message
	msg.DNN = nasType.NewDNN(nasMessage.ULNASTransportDNNType)
	dnnValue := "internet"
	msg.DNN.SetLen(uint8(len(dnnValue)))
	msg.SetDNN(dnnValue)

	fakeSmf := &FakeSmf{
		CreateSmContextRef: "new-ctx-ref",
	}

	amf := &amfContext.AMF{
		Smf:        fakeSmf,
		DBInstance: &FakeDBInstance{},
	}

	err = transport5GSMMessage(t.Context(), amf, ue, msg)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if len(fakeSmf.CreateSmContextCalls) != 1 {
		t.Fatalf("expected 1 CreateSmContext call, got: %d", len(fakeSmf.CreateSmContextCalls))
	}

	call := fakeSmf.CreateSmContextCalls[0]
	if call.PduSessionID != pduSessionID {
		t.Fatalf("expected PDU session ID %d, got: %d", pduSessionID, call.PduSessionID)
	}

	if call.Dnn != "internet" {
		t.Fatalf("expected DNN 'internet', got: %s", call.Dnn)
	}

	// Verify SM context was created on the UE
	smCtx, exists := ue.SmContextFindByPDUSessionID(pduSessionID)
	if !exists {
		t.Fatal("expected SM context to exist")
	}

	if smCtx.Ref != "new-ctx-ref" {
		t.Fatalf("expected SM context ref 'new-ctx-ref', got: %s", smCtx.Ref)
	}
}

func TestTransport5GSMMessage_NoSmContext_InitialRequest_DefaultSNSSAIAndDNN(t *testing.T) {
	ue, _, err := buildUeAndRadio()
	if err != nil {
		t.Fatalf("could not build UE and radio: %v", err)
	}

	ue.Supi = mustSUPIFromPrefixed("imsi-001010000000001")
	ue.AllowedNssai = &models.Snssai{Sst: 1, Sd: "aabbcc"}

	var pduSessionID uint8 = 2

	smPayload := []byte{0x2E, 0x02, 0x00, 0xC1, 0x00}

	msg := buildTestULNASTransport(nasMessage.PayloadContainerTypeN1SMInfo, smPayload, pduSessionIDPtr(pduSessionID))
	setRequestType(msg, nasMessage.ULNASTransportRequestTypeInitialRequest)

	// No SNSSAI or DNN set on the NAS message

	fakeSmf := &FakeSmf{
		CreateSmContextRef: "default-ctx-ref",
	}

	amf := &amfContext.AMF{
		Smf:        fakeSmf,
		DBInstance: &FakeDBInstance{},
	}

	err = transport5GSMMessage(t.Context(), amf, ue, msg)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if len(fakeSmf.CreateSmContextCalls) != 1 {
		t.Fatalf("expected 1 CreateSmContext call, got: %d", len(fakeSmf.CreateSmContextCalls))
	}

	call := fakeSmf.CreateSmContextCalls[0]

	// Should use default SNSSAI from UE context
	if call.Snssai.Sst != 1 || call.Snssai.Sd != "aabbcc" {
		t.Fatalf("expected default SNSSAI SST=1 SD=aabbcc, got: SST=%d SD=%s", call.Snssai.Sst, call.Snssai.Sd)
	}

	// Should use DNN from DB (FakeDBInstance returns "TestDataNetwork")
	if call.Dnn != "TestDataNetwork" {
		t.Fatalf("expected DNN 'TestDataNetwork', got: %s", call.Dnn)
	}

	smCtx, exists := ue.SmContextFindByPDUSessionID(pduSessionID)
	if !exists {
		t.Fatal("expected SM context to exist")
	}

	if smCtx.Ref != "default-ctx-ref" {
		t.Fatalf("expected SM context ref 'default-ctx-ref', got: %s", smCtx.Ref)
	}
}

func TestTransport5GSMMessage_NoSmContext_InitialRequest_NilAllowedNssai_Error(t *testing.T) {
	ue, _, err := buildUeAndRadio()
	if err != nil {
		t.Fatalf("could not build UE and radio: %v", err)
	}

	ue.Supi = mustSUPIFromPrefixed("imsi-001010000000001")
	ue.AllowedNssai = nil

	var pduSessionID uint8 = 1

	smPayload := []byte{0x2E, 0x01, 0x00, 0xC1, 0x00}

	msg := buildTestULNASTransport(nasMessage.PayloadContainerTypeN1SMInfo, smPayload, pduSessionIDPtr(pduSessionID))
	setRequestType(msg, nasMessage.ULNASTransportRequestTypeInitialRequest)

	// No SNSSAI set on the NAS message, and UE.AllowedNssai is nil

	fakeSmf := &FakeSmf{}

	amf := &amfContext.AMF{
		Smf:        fakeSmf,
		DBInstance: &FakeDBInstance{},
	}

	err = transport5GSMMessage(t.Context(), amf, ue, msg)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	expected := "allowed nssai is nil in UE context"
	if err.Error() != expected {
		t.Fatalf("expected error: %s, got: %s", expected, err.Error())
	}
}

func TestTransport5GSMMessage_NoSmContext_InitialRequest_CreateSmContext_ErrorResponse_SendsDLNAS(t *testing.T) {
	ue, ngapSender, err := buildUeAndRadio()
	if err != nil {
		t.Fatalf("could not build UE and radio: %v", err)
	}

	ue.Supi = mustSUPIFromPrefixed("imsi-001010000000001")
	ue.AllowedNssai = &models.Snssai{Sst: 1, Sd: "010203"}

	var pduSessionID uint8 = 1

	smPayload := []byte{0x2E, 0x01, 0x00, 0xC1, 0x00}

	msg := buildTestULNASTransport(nasMessage.PayloadContainerTypeN1SMInfo, smPayload, pduSessionIDPtr(pduSessionID))
	setRequestType(msg, nasMessage.ULNASTransportRequestTypeInitialRequest)

	// CreateSmContext returns an error response (N1 rejection message)
	fakeSmf := &FakeSmf{
		CreateSmContextErrResp: []byte{0x2E, 0x01, 0x00, 0xC2, 0x24},
		CreateSmContextError:   fmt.Errorf("policy not found"),
	}

	amf := &amfContext.AMF{
		Smf:        fakeSmf,
		DBInstance: &FakeDBInstance{},
	}

	err = transport5GSMMessage(t.Context(), amf, ue, msg)
	if err == nil {
		t.Fatal("expected an error, got nil")
	}

	// Should have sent a DL NAS Transport with the error response
	if len(ngapSender.SentDownlinkNASTransport) != 1 {
		t.Fatalf("expected 1 downlink NAS transport, got: %d", len(ngapSender.SentDownlinkNASTransport))
	}

	resp := ngapSender.SentDownlinkNASTransport[0]
	nm := new(nas.Message)
	nm.SecurityHeaderType = nas.GetSecurityHeaderType(resp.NasPdu) & 0x0f

	if nm.SecurityHeaderType != nas.SecurityHeaderTypePlainNas {
		t.Fatalf("expected a plain NAS message")
	}

	err = nm.PlainNasDecode(&resp.NasPdu)
	if err != nil {
		t.Fatalf("could not decode plain NAS message: %v", err)
	}

	if nm.GmmHeader.GetMessageType() != nas.MsgTypeDLNASTransport {
		t.Fatalf("expected DLNASTransport message, got: %v", nm.GmmHeader.GetMessageType())
	}

	// SM context should NOT have been created on the UE
	if _, exists := ue.SmContextFindByPDUSessionID(pduSessionID); exists {
		t.Fatal("expected SM context NOT to exist after rejection")
	}
}

func TestTransport5GSMMessage_NoSmContext_InitialRequest_CreateSmContext_ErrorOnly_NoSMContext(t *testing.T) {
	ue, _, err := buildUeAndRadio()
	if err != nil {
		t.Fatalf("could not build UE and radio: %v", err)
	}

	ue.Supi = mustSUPIFromPrefixed("imsi-001010000000001")
	ue.AllowedNssai = &models.Snssai{Sst: 1, Sd: "010203"}

	var pduSessionID uint8 = 1

	smPayload := []byte{0x2E, 0x01, 0x00, 0xC1, 0x00}

	msg := buildTestULNASTransport(nasMessage.PayloadContainerTypeN1SMInfo, smPayload, pduSessionIDPtr(pduSessionID))
	setRequestType(msg, nasMessage.ULNASTransportRequestTypeInitialRequest)

	// CreateSmContext returns error but no error response bytes
	fakeSmf := &FakeSmf{
		CreateSmContextError: fmt.Errorf("internal error"),
	}

	amf := &amfContext.AMF{
		Smf:        fakeSmf,
		DBInstance: &FakeDBInstance{},
	}

	// The code logs the error but does not return it (creates context with empty ref)
	err = transport5GSMMessage(t.Context(), amf, ue, msg)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// SM context still gets created (with empty ref) because errResponse is nil
	smCtx, exists := ue.SmContextFindByPDUSessionID(pduSessionID)
	if !exists {
		t.Fatal("expected SM context to exist (even with error, errResponse was nil)")
	}

	if smCtx.Ref != "" {
		t.Fatalf("expected empty SM context ref, got: %s", smCtx.Ref)
	}
}

// TestTransport5GSMMessage_NoSmContext_NilRequestType_Panic reproduces a nil-pointer
// dereference when a UE sends a ULNASTransport with N1SM payload for a PDU session
// that has no AMF-side SM context and omits the RequestType IE.
// The code reaches `switch requestType.GetRequestTypeValue()` with requestType == nil.
func TestTransport5GSMMessage_NoSmContext_NilRequestType_Panic(t *testing.T) {
	ue, _, err := buildUeAndRadio()
	if err != nil {
		t.Fatalf("could not build UE and radio: %v", err)
	}

	ue.SetState(amfContext.Registered)

	// Ensure no SM context exists for PDU session 5
	_, exists := ue.SmContextFindByPDUSessionID(5)
	if exists {
		t.Fatal("precondition failed: SM context should not exist for PDU session 5")
	}

	// Build ULNASTransport with N1SM payload, valid PDU session ID, but NO RequestType
	pduSessionID := uint8(5)
	smPayload := []byte{0x2e, 0x05, 0xc1, 0x00, 0x00} // minimal 5GSM header
	msg := buildTestULNASTransport(nasMessage.PayloadContainerTypeN1SMInfo, smPayload, &pduSessionID)
	// msg.RequestType is intentionally left nil

	amf := &amfContext.AMF{
		Smf:        &FakeSmf{},
		DBInstance: &FakeDBInstance{},
	}

	// This should NOT panic — it should return an error gracefully
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("transport5GSMMessage panicked with nil RequestType (no SM context): %v", r)
		}
	}()

	_ = transport5GSMMessage(t.Context(), amf, ue, msg)
}
