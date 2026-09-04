// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"fmt"
	"testing"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/nasreply"
	"github.com/ellanetworks/core/internal/smf"
	"github.com/ellanetworks/core/nas/fgs"
)

func encULNAS(t *testing.T, msg *fgs.ULNASTransport) []byte {
	t.Helper()

	b, err := msg.MarshalBinary()
	if err != nil {
		t.Fatalf("could not encode UL NAS Transport: %v", err)
	}

	return b
}

func fgsULNAS(t *testing.T, msg *fgs.ULNASTransport) *fgs.ULNASTransport {
	t.Helper()

	parsed, err := fgs.ParseULNASTransport(encULNAS(t, msg))
	if err != nil {
		t.Fatalf("could not parse UL NAS Transport: %v", err)
	}

	return parsed
}

func buildTestULNASTransport(payloadContainerType fgs.PayloadContainerType, payload []byte, pduSessionID *fgs.PDUSessionID) *fgs.ULNASTransport {
	return &fgs.ULNASTransport{
		PayloadContainerType: payloadContainerType,
		PayloadContainer:     payload,
		PDUSessionID:         pduSessionID,
	}
}

func setRequestType(msg *fgs.ULNASTransport, requestTypeValue fgs.RequestType) {
	msg.RequestType = &requestTypeValue
}

func setOldPDUSessionID(msg *fgs.ULNASTransport, id fgs.PDUSessionID) {
	msg.OldPDUSessionID = &id
}

func pduSessionIDPtr(id fgs.PDUSessionID) *fgs.PDUSessionID {
	return &id
}

func TestHandleULNASTransport_WrongState_Error(t *testing.T) {
	testcases := []amf.StateType{amf.Deregistered, amf.RegistrationInitiated, amf.DeregistrationInitiated}
	for _, tc := range testcases {
		t.Run(string(tc), func(t *testing.T) {
			ue, _, err := buildUeAndRadio()
			if err != nil {
				t.Fatalf("could not build UE and radio: %v", err)
			}

			ue.ForceStateForTest(tc)

			msg := buildTestULNASTransport(fgs.PayloadContainerTypeN1SMInfo, []byte{0x01}, pduSessionIDPtr(1))

			got := handleULNASTransport(t.Context(), amf.New(nil, nil, nil), ue, msg)
			if want := nasreply.Silent(nasreply.ReasonOutOfState); got != want {
				t.Fatalf("disposition = %+v, want %+v", got, want)
			}
		})
	}
}

func TestHandleULNASTransport_PayloadContainerTypeSMS_Handled(t *testing.T) {
	ue, _, err := buildUeAndRadio()
	if err != nil {
		t.Fatalf("could not build UE and radio: %v", err)
	}

	ue.ForceStateForTest(amf.Registered)

	msg := buildTestULNASTransport(fgs.PayloadContainerTypeSMS, []byte{0x01}, nil)

	if got := handleULNASTransport(t.Context(), amf.New(nil, nil, nil), ue, msg); got != nasreply.Handled() {
		t.Fatalf("disposition = %+v, want %+v", got, nasreply.Handled())
	}
}

func TestHandleULNASTransport_PayloadContainerTypeLPP_Handled(t *testing.T) {
	ue, _, err := buildUeAndRadio()
	if err != nil {
		t.Fatalf("could not build UE and radio: %v", err)
	}

	ue.ForceStateForTest(amf.Registered)

	msg := buildTestULNASTransport(fgs.PayloadContainerTypeLPP, []byte{0x01}, nil)

	if got := handleULNASTransport(t.Context(), amf.New(nil, nil, nil), ue, msg); got != nasreply.Handled() {
		t.Fatalf("disposition = %+v, want %+v", got, nasreply.Handled())
	}
}

func TestHandleULNASTransport_PayloadContainerTypeSOR_Handled(t *testing.T) {
	ue, _, err := buildUeAndRadio()
	if err != nil {
		t.Fatalf("could not build UE and radio: %v", err)
	}

	ue.ForceStateForTest(amf.Registered)

	msg := buildTestULNASTransport(fgs.PayloadContainerTypeSOR, []byte{0x01}, nil)

	if got := handleULNASTransport(t.Context(), amf.New(nil, nil, nil), ue, msg); got != nasreply.Handled() {
		t.Fatalf("disposition = %+v, want %+v", got, nasreply.Handled())
	}
}

func TestHandleULNASTransport_PayloadContainerTypeMultiplePayload_Handled(t *testing.T) {
	ue, _, err := buildUeAndRadio()
	if err != nil {
		t.Fatalf("could not build UE and radio: %v", err)
	}

	ue.ForceStateForTest(amf.Registered)

	msg := buildTestULNASTransport(fgs.PayloadContainerTypeMultiplePayload, []byte{0x01}, nil)

	if got := handleULNASTransport(t.Context(), amf.New(nil, nil, nil), ue, msg); got != nasreply.Handled() {
		t.Fatalf("disposition = %+v, want %+v", got, nasreply.Handled())
	}
}

func TestHandleULNASTransport_PayloadContainerTypeUEPolicy_Handled(t *testing.T) {
	ue, _, err := buildUeAndRadio()
	if err != nil {
		t.Fatalf("could not build UE and radio: %v", err)
	}

	ue.ForceStateForTest(amf.Registered)

	msg := buildTestULNASTransport(fgs.PayloadContainerTypeUEPolicy, []byte{0x01}, nil)

	if got := handleULNASTransport(t.Context(), amf.New(nil, nil, nil), ue, msg); got != nasreply.Handled() {
		t.Fatalf("disposition = %+v, want %+v", got, nasreply.Handled())
	}
}

func TestHandleULNASTransport_PayloadContainerTypeUEParameterUpdate_Handled(t *testing.T) {
	ue, _, err := buildUeAndRadio()
	if err != nil {
		t.Fatalf("could not build UE and radio: %v", err)
	}

	ue.ForceStateForTest(amf.Registered)

	upuAck := make([]byte, 17)
	upuAck[0] = 0x01

	msg := buildTestULNASTransport(fgs.PayloadContainerTypeUEParameterUpdate, upuAck, nil)

	if got := handleULNASTransport(t.Context(), amf.New(nil, nil, nil), ue, msg); got != nasreply.Handled() {
		t.Fatalf("disposition = %+v, want %+v", got, nasreply.Handled())
	}
}

func TestTransport5GSMMessage_NilPduSessionID_Dropped(t *testing.T) {
	ue, ngapSender, err := buildUeAndRadio()
	if err != nil {
		t.Fatalf("could not build UE and radio: %v", err)
	}

	msg := buildTestULNASTransport(fgs.PayloadContainerTypeN1SMInfo, []byte{0x01}, nil)

	transport5GSMMessage(t.Context(), amf.New(nil, nil, nil), ue, fgsULNAS(t, msg))

	if len(ngapSender.SentDownlinkNASTransport) != 0 {
		t.Fatalf("expected the message to be dropped, got %d downlinks", len(ngapSender.SentDownlinkNASTransport))
	}
}

func TestTransport5GSMMessage_OldPDUSessionID_Dropped(t *testing.T) {
	ue, ngapSender, err := buildUeAndRadio()
	if err != nil {
		t.Fatalf("could not build UE and radio: %v", err)
	}

	msg := buildTestULNASTransport(fgs.PayloadContainerTypeN1SMInfo, []byte{0x01}, pduSessionIDPtr(1))
	setOldPDUSessionID(msg, 2)

	transport5GSMMessage(t.Context(), amf.New(nil, nil, nil), ue, fgsULNAS(t, msg))

	if len(ngapSender.SentDownlinkNASTransport) != 0 {
		t.Fatalf("expected the message to be dropped, got %d downlinks", len(ngapSender.SentDownlinkNASTransport))
	}
}

func TestTransport5GSMMessage_SmContextNotExists_Status5GSM_Ignored(t *testing.T) {
	ue, ngapSender, err := buildUeAndRadio()
	if err != nil {
		t.Fatalf("could not build UE and radio: %v", err)
	}

	status5gsmPayload := []byte{0x2E, 0x01, 0x00, 0xD6, 0x24}

	msg := buildTestULNASTransport(fgs.PayloadContainerTypeN1SMInfo, status5gsmPayload, pduSessionIDPtr(1))

	transport5GSMMessage(t.Context(), amf.New(nil, nil, nil), ue, fgsULNAS(t, msg))

	if len(ngapSender.SentDownlinkNASTransport) != 0 {
		t.Fatalf("a 5GSM STATUS for an unknown session must be ignored, got %d downlinks", len(ngapSender.SentDownlinkNASTransport))
	}
}

func TestTransport5GSMMessage_EmergencyRequest_SendsDLNASTransport(t *testing.T) {
	ue, ngapSender, err := buildUeAndRadio()
	if err != nil {
		t.Fatalf("could not build UE and radio: %v", err)
	}

	smPayload := []byte{0x2E, 0x01, 0x00, 0xC1, 0x00}

	msg := buildTestULNASTransport(fgs.PayloadContainerTypeN1SMInfo, smPayload, pduSessionIDPtr(1))
	setRequestType(msg, fgs.RequestTypeInitialEmergencyRequest)

	transport5GSMMessage(t.Context(), amf.New(nil, nil, nil), ue, fgsULNAS(t, msg))

	if len(ngapSender.SentDownlinkNASTransport) != 1 {
		t.Fatalf("expected 1 downlink NAS transport, got: %d", len(ngapSender.SentDownlinkNASTransport))
	}

	resp := ngapSender.SentDownlinkNASTransport[0]
	assertPlainGmm(t, resp.NASPDU, uint8(fgs.MsgDLNASTransport))
}

func TestTransport5GSMMessage_ExistingEmergencyPduSession_SendsDLNASTransport(t *testing.T) {
	ue, ngapSender, err := buildUeAndRadio()
	if err != nil {
		t.Fatalf("could not build UE and radio: %v", err)
	}

	smPayload := []byte{0x2E, 0x01, 0x00, 0xC1, 0x00}

	msg := buildTestULNASTransport(fgs.PayloadContainerTypeN1SMInfo, smPayload, pduSessionIDPtr(1))
	setRequestType(msg, fgs.RequestTypeExistingEmergencyPDUSession)

	transport5GSMMessage(t.Context(), amf.New(nil, nil, nil), ue, fgsULNAS(t, msg))

	if len(ngapSender.SentDownlinkNASTransport) != 1 {
		t.Fatalf("expected 1 downlink NAS transport, got: %d", len(ngapSender.SentDownlinkNASTransport))
	}

	resp := ngapSender.SentDownlinkNASTransport[0]
	assertPlainGmm(t, resp.NASPDU, uint8(fgs.MsgDLNASTransport))
}

func TestTransport5GSMMessage_ExistingPduSession_NotAllowedNssai_SendsDLNASTransport(t *testing.T) {
	ue, ngapSender, err := buildUeAndRadio()
	if err != nil {
		t.Fatalf("could not build UE and radio: %v", err)
	}

	ue.AllowedNssai = []models.Snssai{{Sst: 1, Sd: "010203"}}

	var pduSessionID uint8 = 5

	_ = ue.CreateSmContext(pduSessionID, "testref", &models.Snssai{Sst: 2, Sd: "040506"}, "internet")

	smPayload := []byte{0x2E, 0x01, 0x00, 0xC1, 0x00}

	msg := buildTestULNASTransport(fgs.PayloadContainerTypeN1SMInfo, smPayload, pduSessionIDPtr(fgs.PDUSessionID(pduSessionID)))
	setRequestType(msg, fgs.RequestTypeExistingPDUSession)

	transport5GSMMessage(t.Context(), amf.New(nil, nil, nil), ue, fgsULNAS(t, msg))

	if len(ngapSender.SentDownlinkNASTransport) != 1 {
		t.Fatalf("expected 1 downlink NAS transport, got: %d", len(ngapSender.SentDownlinkNASTransport))
	}

	resp := ngapSender.SentDownlinkNASTransport[0]
	assertPlainGmm(t, resp.NASPDU, uint8(fgs.MsgDLNASTransport))
}

func TestTransport5GSMMessage_InitialRequest_NotAllowedNssai_NotForwarded(t *testing.T) {
	ue, ngapSender, err := buildUeAndRadio()
	if err != nil {
		t.Fatalf("could not build UE and radio: %v", err)
	}

	ue.SetSupiForTest(mustSUPIFromPrefixed("imsi-001010000000001"))
	ue.AllowedNssai = []models.Snssai{{Sst: 1, Sd: "010203"}}

	var pduSessionID uint8 = 1

	smPayload := []byte{0x2E, 0x01, 0x00, 0xC1, 0x00}

	msg := buildTestULNASTransport(fgs.PayloadContainerTypeN1SMInfo, smPayload, pduSessionIDPtr(fgs.PDUSessionID(pduSessionID)))
	setRequestType(msg, fgs.RequestTypeInitialRequest)

	msg.SNSSAI = &fgs.SNSSAI{SST: 2, SD: &[3]byte{4, 5, 6}}
	msg.DNN = new(fgs.DNN("internet"))

	fakeSmf := &fakeSmf{CreateSmContextRef: "new-ctx-ref"}

	transport5GSMMessage(t.Context(), amf.New(&fakeDBInstance{}, nil, fakeSmf), ue, fgsULNAS(t, msg))

	if len(fakeSmf.CreateSmContextCalls) != 0 {
		t.Fatalf("CreateSmContext call count is %d, want 0", len(fakeSmf.CreateSmContextCalls))
	}

	if _, exists := ue.SmContextFindByPDUSessionID(pduSessionID); exists {
		t.Error("an SM context was created for an S-NSSAI outside the allowed NSSAI")
	}

	if len(ngapSender.SentDownlinkNASTransport) != 1 {
		t.Fatalf("downlink NAS transport count is %d, want 1", len(ngapSender.SentDownlinkNASTransport))
	}

	assertPlainGmm(t, ngapSender.SentDownlinkNASTransport[0].NASPDU, uint8(fgs.MsgDLNASTransport))
}

func TestTransport5GSMMessage_ModificationRequest_NotAllowedNssai_NotForwarded(t *testing.T) {
	ue, ngapSender, err := buildUeAndRadio()
	if err != nil {
		t.Fatalf("could not build UE and radio: %v", err)
	}

	ue.SetSupiForTest(mustSUPIFromPrefixed("imsi-001010000000001"))
	ue.AllowedNssai = []models.Snssai{{Sst: 1, Sd: "010203"}}

	var pduSessionID uint8 = 5

	_ = ue.CreateSmContext(pduSessionID, "testref", &models.Snssai{Sst: 1, Sd: "010203"}, "internet")

	smPayload := []byte{0x2E, 0x01, 0x00, 0xC1, 0x00}

	msg := buildTestULNASTransport(fgs.PayloadContainerTypeN1SMInfo, smPayload, pduSessionIDPtr(fgs.PDUSessionID(pduSessionID)))
	setRequestType(msg, fgs.RequestTypeModificationRequest)

	msg.SNSSAI = &fgs.SNSSAI{SST: 2, SD: &[3]byte{4, 5, 6}}

	fakeSmf := &fakeSmf{}

	transport5GSMMessage(t.Context(), amf.New(&fakeDBInstance{}, nil, fakeSmf), ue, fgsULNAS(t, msg))

	if len(fakeSmf.UpdateN1MsgCalls) != 0 {
		t.Fatalf("UpdateSmContextN1Msg call count is %d, want 0", len(fakeSmf.UpdateN1MsgCalls))
	}

	if len(ngapSender.SentDownlinkNASTransport) != 1 {
		t.Fatalf("downlink NAS transport count is %d, want 1", len(ngapSender.SentDownlinkNASTransport))
	}

	assertPlainGmm(t, ngapSender.SentDownlinkNASTransport[0].NASPDU, uint8(fgs.MsgDLNASTransport))
}

func TestTransport5GSMMessage_NoSmContext_ModificationRequest_SendsDLNASTransport(t *testing.T) {
	ue, ngapSender, err := buildUeAndRadio()
	if err != nil {
		t.Fatalf("could not build UE and radio: %v", err)
	}

	smPayload := []byte{0x2E, 0x01, 0x00, 0xC1, 0x00}

	msg := buildTestULNASTransport(fgs.PayloadContainerTypeN1SMInfo, smPayload, pduSessionIDPtr(1))
	setRequestType(msg, fgs.RequestTypeModificationRequest)

	transport5GSMMessage(t.Context(), amf.New(nil, nil, nil), ue, fgsULNAS(t, msg))

	if len(ngapSender.SentDownlinkNASTransport) != 1 {
		t.Fatalf("expected 1 downlink NAS transport, got: %d", len(ngapSender.SentDownlinkNASTransport))
	}

	resp := ngapSender.SentDownlinkNASTransport[0]
	assertPlainGmm(t, resp.NASPDU, uint8(fgs.MsgDLNASTransport))
}

func TestTransport5GSMMessage_NoSmContext_NoRequestType_SendsDLNASTransport(t *testing.T) {
	ue, ngapSender, err := buildUeAndRadio()
	if err != nil {
		t.Fatalf("could not build UE and radio: %v", err)
	}

	smPayload := []byte{0x2E, 0x01, 0x00, 0xC9}

	msg := buildTestULNASTransport(fgs.PayloadContainerTypeN1SMInfo, smPayload, pduSessionIDPtr(1))

	transport5GSMMessage(t.Context(), amf.New(nil, nil, nil), ue, fgsULNAS(t, msg))

	if len(ngapSender.SentDownlinkNASTransport) != 1 {
		t.Fatalf("expected 1 downlink NAS transport, got: %d", len(ngapSender.SentDownlinkNASTransport))
	}

	resp := ngapSender.SentDownlinkNASTransport[0]
	dl := assertPlainDLTransport(t, resp.NASPDU)

	if dl.Cause == nil {
		t.Fatal("expected a DLNASTransport carrying a 5GMM cause")
	}

	if got := *dl.Cause; got != 0x5a {
		t.Fatalf("5GMM cause = %d, want %d (payload was not forwarded)", got, 0x5a)
	}
}

func TestTransport5GSMMessage_ReservedPduSessionID_SendsDLNASTransport(t *testing.T) {
	ue, ngapSender, err := buildUeAndRadio()
	if err != nil {
		t.Fatalf("could not build UE and radio: %v", err)
	}

	smPayload := []byte{0x2E, 0x10, 0x00, 0xC9}

	msg := buildTestULNASTransport(fgs.PayloadContainerTypeN1SMInfo, smPayload, pduSessionIDPtr(16))

	transport5GSMMessage(t.Context(), amf.New(nil, nil, nil), ue, fgsULNAS(t, msg))

	if len(ngapSender.SentDownlinkNASTransport) != 1 {
		t.Fatalf("expected 1 downlink NAS transport, got: %d", len(ngapSender.SentDownlinkNASTransport))
	}

	resp := ngapSender.SentDownlinkNASTransport[0]
	dl := assertPlainDLTransport(t, resp.NASPDU)

	if dl.Cause == nil || *dl.Cause != 0x5a {
		t.Fatalf("expected DLNASTransport with 5GMM cause #%d", 0x5a)
	}
}

func TestTransport5GSMMessage_NoSmContext_ExistingPduSession_SendsDLNASTransport(t *testing.T) {
	ue, ngapSender, err := buildUeAndRadio()
	if err != nil {
		t.Fatalf("could not build UE and radio: %v", err)
	}

	smPayload := []byte{0x2E, 0x01, 0x00, 0xC1, 0x00}

	msg := buildTestULNASTransport(fgs.PayloadContainerTypeN1SMInfo, smPayload, pduSessionIDPtr(1))
	setRequestType(msg, fgs.RequestTypeExistingPDUSession)

	transport5GSMMessage(t.Context(), amf.New(nil, nil, nil), ue, fgsULNAS(t, msg))

	if len(ngapSender.SentDownlinkNASTransport) != 1 {
		t.Fatalf("expected 1 downlink NAS transport, got: %d", len(ngapSender.SentDownlinkNASTransport))
	}

	resp := ngapSender.SentDownlinkNASTransport[0]
	assertPlainGmm(t, resp.NASPDU, uint8(fgs.MsgDLNASTransport))
}

func TestTransport5GSMMessage_SmContextExists_InitialRequest_DeletesContextAndCreateNewOne(t *testing.T) {
	ue, _, err := buildUeAndRadio()
	if err != nil {
		t.Fatalf("could not build UE and radio: %v", err)
	}

	ue.SetSupiForTest(mustSUPIFromPrefixed("imsi-001010000000001"))

	var pduSessionID uint8 = 3

	snssai := &models.Snssai{Sst: 1, Sd: "010203"}
	_ = ue.CreateSmContext(pduSessionID, "testref", snssai, "internet")

	smPayload := []byte{0x2E, 0x03, 0x00, 0xC1, 0x00}

	msg := buildTestULNASTransport(fgs.PayloadContainerTypeN1SMInfo, smPayload, pduSessionIDPtr(fgs.PDUSessionID(pduSessionID)))
	setRequestType(msg, fgs.RequestTypeInitialRequest)

	fakeSmf := &fakeSmf{
		CreateSmContextRef: "new-ref-123",
	}

	amfInstance := amf.New(&fakeDBInstance{}, nil, fakeSmf)

	ue.AllowedNssai = []models.Snssai{*snssai}

	transport5GSMMessage(t.Context(), amfInstance, ue, fgsULNAS(t, msg))

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

func TestTransport5GSMMessage_InitialRequest_SmfReturnsErrorAndReject_ForwardsRejectAndNoSmContext(t *testing.T) {
	ue, ngapSender, err := buildUeAndRadio()
	if err != nil {
		t.Fatalf("could not build UE and radio: %v", err)
	}

	ue.SetSupiForTest(mustSUPIFromPrefixed("imsi-001010000000001"))

	var pduSessionID uint8 = 4

	snssai := &models.Snssai{Sst: 1, Sd: "010203"}
	ue.AllowedNssai = []models.Snssai{*snssai}

	smPayload := []byte{0x2E, 0x03, 0x00, 0xC1, 0x00}
	msg := buildTestULNASTransport(fgs.PayloadContainerTypeN1SMInfo, smPayload, pduSessionIDPtr(fgs.PDUSessionID(pduSessionID)))
	setRequestType(msg, fgs.RequestTypeInitialRequest)

	smfReject := []byte{0xDE, 0xAD, 0xBE, 0xEF}
	fakeSmf := &fakeSmf{
		CreateSmContextErrResp: smfReject,
		CreateSmContextError:   fmt.Errorf("malformed NAS in establishment request"),
	}

	amfInstance := amf.New(&fakeDBInstance{}, nil, fakeSmf)

	transport5GSMMessage(t.Context(), amfInstance, ue, fgsULNAS(t, msg))

	if _, exists := ue.SmContextFindByPDUSessionID(pduSessionID); exists {
		t.Fatal("expected no SM context to be created on SMF reject")
	}

	if len(ngapSender.SentDownlinkNASTransport) != 1 {
		t.Fatalf("expected 1 downlink NAS transport carrying the SMF reject, got: %d", len(ngapSender.SentDownlinkNASTransport))
	}
}

func TestTransport5GSMMessage_InitialRequest_SmfReturnsErrorOnly_SendsFallbackAndNoSmContext(t *testing.T) {
	ue, ngapSender, err := buildUeAndRadio()
	if err != nil {
		t.Fatalf("could not build UE and radio: %v", err)
	}

	ue.SetSupiForTest(mustSUPIFromPrefixed("imsi-001010000000001"))

	var pduSessionID uint8 = 5

	snssai := &models.Snssai{Sst: 1, Sd: "010203"}
	ue.AllowedNssai = []models.Snssai{*snssai}

	smPayload := []byte{0x2E, 0x03, 0x00, 0xC1, 0x00}
	msg := buildTestULNASTransport(fgs.PayloadContainerTypeN1SMInfo, smPayload, pduSessionIDPtr(fgs.PDUSessionID(pduSessionID)))
	setRequestType(msg, fgs.RequestTypeInitialRequest)

	fakeSmf := &fakeSmf{
		CreateSmContextError: fmt.Errorf("smf is unavailable"),
	}

	amfInstance := amf.New(&fakeDBInstance{}, nil, fakeSmf)

	transport5GSMMessage(t.Context(), amfInstance, ue, fgsULNAS(t, msg))

	if _, exists := ue.SmContextFindByPDUSessionID(pduSessionID); exists {
		t.Fatal("expected no SM context to be created on SMF error")
	}

	if len(ngapSender.SentDownlinkNASTransport) != 1 {
		t.Fatalf("expected 1 fallback downlink NAS transport, got: %d", len(ngapSender.SentDownlinkNASTransport))
	}

	resp := ngapSender.SentDownlinkNASTransport[0]
	dl := assertPlainDLTransport(t, resp.NASPDU)

	if dl.Cause == nil {
		t.Fatal("expected DLNASTransport with 5GMM cause")
	}

	if got := *dl.Cause; got != 0x5a {
		t.Fatalf("expected 5GMM cause %d (payload was not forwarded), got %d", 0x5a, got)
	}
}

func TestForward5GSMMessageToSMF_UpdateError_ReturnsError(t *testing.T) {
	ue, _, err := buildUeAndRadio()
	if err != nil {
		t.Fatalf("could not build UE and radio: %v", err)
	}

	fakeSmf := &fakeSmf{
		UpdateN1MsgError: fmt.Errorf("smf unavailable"),
	}

	amfInstance := amf.New(nil, nil, fakeSmf)

	forward5GSMMessageToSMF(t.Context(), amfInstance, ue, 1, "ref-1", []byte{0x01})

	if len(fakeSmf.UpdateN1MsgCalls) != 1 {
		t.Fatalf("expected 1 UpdateSmContextN1Msg call, got: %d", len(fakeSmf.UpdateN1MsgCalls))
	}
}

func TestForward5GSMMessageToSMF_NilResponse_NoDownlink(t *testing.T) {
	ue, ngapSender, err := buildUeAndRadio()
	if err != nil {
		t.Fatalf("could not build UE and radio: %v", err)
	}

	fakeSmf := &fakeSmf{
		UpdateN1MsgResponse: nil,
	}

	amfInstance := amf.New(nil, nil, fakeSmf)

	forward5GSMMessageToSMF(t.Context(), amfInstance, ue, 1, "ref-1", []byte{0x01})

	if len(fakeSmf.UpdateN1MsgCalls) != 1 {
		t.Fatalf("expected 1 UpdateSmContextN1Msg call, got %d", len(fakeSmf.UpdateN1MsgCalls))
	}

	if len(ngapSender.SentDownlinkNASTransport) != 0 {
		t.Fatalf("an SMF that returns nothing must produce no downlink, got %d", len(ngapSender.SentDownlinkNASTransport))
	}
}

func TestForward5GSMMessageToSMF_N1Only_SendsDLNASTransport(t *testing.T) {
	ue, ngapSender, err := buildUeAndRadio()
	if err != nil {
		t.Fatalf("could not build UE and radio: %v", err)
	}

	fakeSmf := &fakeSmf{
		UpdateN1MsgResponse: &smf.UpdateResult{
			N1Msg: []byte{0x2E, 0x01, 0x00, 0xD6, 0x24},
		},
	}

	amfInstance := amf.New(nil, nil, fakeSmf)

	forward5GSMMessageToSMF(t.Context(), amfInstance, ue, 1, "ref-1", []byte{0x01})

	if len(ngapSender.SentDownlinkNASTransport) != 1 {
		t.Fatalf("expected 1 downlink NAS transport, got: %d", len(ngapSender.SentDownlinkNASTransport))
	}

	resp := ngapSender.SentDownlinkNASTransport[0]
	assertPlainGmm(t, resp.NASPDU, uint8(fgs.MsgDLNASTransport))
}

func TestForward5GSMMessageToSMF_N2NotPduResRel_ReturnsNil(t *testing.T) {
	ue, ngapSender, err := buildUeAndRadio()
	if err != nil {
		t.Fatalf("could not build UE and radio: %v", err)
	}

	fakeSmf := &fakeSmf{
		UpdateN1MsgResponse: &smf.UpdateResult{
			N2Msg:     []byte{0x01, 0x02},
			ReleaseN2: false,
		},
	}

	amfInstance := amf.New(nil, nil, fakeSmf)

	forward5GSMMessageToSMF(t.Context(), amfInstance, ue, 1, "ref-1", []byte{0x01})

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

	ue.Conn().MarkICSCompleted()

	n2Data := []byte{0x00, 0x00, 0x04, 0x00, 0x00, 0x00, 0x00, 0x00}

	fakeSmf := &fakeSmf{
		UpdateN1MsgResponse: &smf.UpdateResult{
			N2Msg:     n2Data,
			ReleaseN2: true,
		},
	}

	amfInstance := amf.New(nil, nil, fakeSmf)

	forward5GSMMessageToSMF(t.Context(), amfInstance, ue, 1, "ref-1", []byte{0x01})

	if len(ngapSender.SentPDUSessionResourceReleaseCommand) != 1 {
		t.Fatalf("expected 1 release command, got: %d", len(ngapSender.SentPDUSessionResourceReleaseCommand))
	}
}

func TestForward5GSMMessageToSMF_N1AndN2PduResRel_SendsReleaseCommandWithN1(t *testing.T) {
	ue, ngapSender, err := buildUeAndRadio()
	if err != nil {
		t.Fatalf("could not build UE and radio: %v", err)
	}

	ue.Conn().MarkICSCompleted()

	n2Data := []byte{0x00, 0x00, 0x04, 0x00, 0x00, 0x00, 0x00, 0x00}

	fakeSmf := &fakeSmf{
		UpdateN1MsgResponse: &smf.UpdateResult{
			N1Msg:     []byte{0x2E, 0x01, 0x00, 0xD6, 0x24},
			N2Msg:     n2Data,
			ReleaseN2: true,
		},
	}

	amfInstance := amf.New(nil, nil, fakeSmf)

	forward5GSMMessageToSMF(t.Context(), amfInstance, ue, 1, "ref-1", []byte{0x01})

	if len(ngapSender.SentPDUSessionResourceReleaseCommand) != 1 {
		t.Fatalf("expected 1 release command, got: %d", len(ngapSender.SentPDUSessionResourceReleaseCommand))
	}

	relCmd := ngapSender.SentPDUSessionResourceReleaseCommand[0]
	if relCmd.NASPDU == nil {
		t.Fatal("expected NAS PDU in release command, got nil")
	}

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

	_ = ue.CreateSmContext(pduSessionID, "test-ref", &models.Snssai{Sst: 1, Sd: "010203"}, "internet")

	smPayload := []byte{0x2E, 0x05, 0x00, 0xC1, 0x00}

	msg := buildTestULNASTransport(fgs.PayloadContainerTypeN1SMInfo, smPayload, pduSessionIDPtr(fgs.PDUSessionID(pduSessionID)))

	fakeSmf := &fakeSmf{
		UpdateN1MsgResponse: nil,
	}

	amfInstance := amf.New(nil, nil, fakeSmf)

	transport5GSMMessage(t.Context(), amfInstance, ue, fgsULNAS(t, msg))

	if len(fakeSmf.UpdateN1MsgCalls) != 1 {
		t.Fatalf("expected 1 UpdateSmContextN1Msg call, got: %d", len(fakeSmf.UpdateN1MsgCalls))
	}

	if fakeSmf.UpdateN1MsgCalls[0].SmContextRef != "test-ref" {
		t.Fatalf("expected amf.SmContextRef 'test-ref', got: %s", fakeSmf.UpdateN1MsgCalls[0].SmContextRef)
	}
}

func TestTransport5GSMMessage_SmContextExists_DuplicatePDU_Success(t *testing.T) {
	ue, ngapSender, err := buildUeAndRadio()
	if err != nil {
		t.Fatalf("could not build UE and radio: %v", err)
	}

	var pduSessionID uint8 = 3

	snssai := &models.Snssai{Sst: 1, Sd: "010203"}
	_ = ue.CreateSmContext(pduSessionID, "dup-ref", snssai, "internet")

	_ = ue.CreateSmContext(7, "other-ref", snssai, "internet")

	smPayload := []byte{0x2E, 0x03, 0x00, 0xC1, 0x00}

	msg := buildTestULNASTransport(fgs.PayloadContainerTypeN1SMInfo, smPayload, pduSessionIDPtr(fgs.PDUSessionID(pduSessionID)))
	setRequestType(msg, fgs.RequestTypeInitialRequest)

	fakeSmf := &fakeSmf{
		CreateSmContextRef: "new-ref-after-dup",
	}

	amfInstance := amf.New(&fakeDBInstance{}, nil, fakeSmf)

	ue.SetSupiForTest(mustSUPIFromPrefixed("imsi-001010000000001"))
	ue.AllowedNssai = []models.Snssai{*snssai}

	transport5GSMMessage(t.Context(), amfInstance, ue, fgsULNAS(t, msg))

	if len(fakeSmf.DuplicatePDUCalls) != 0 {
		t.Fatalf("expected 0 DuplicatePDU calls, got: %d", len(fakeSmf.DuplicatePDUCalls))
	}

	if len(fakeSmf.CreateSmContextCalls) != 1 {
		t.Fatalf("expected 1 CreateSmContext call, got: %d", len(fakeSmf.CreateSmContextCalls))
	}

	if len(ngapSender.SentPDUSessionResourceReleaseCommand) != 0 {
		t.Fatalf("expected 0 release commands, got: %d", len(ngapSender.SentPDUSessionResourceReleaseCommand))
	}

	smCtx, exists := ue.SmContextFindByPDUSessionID(pduSessionID)
	if !exists {
		t.Fatal("expected SM context to exist after re-creation")
	}

	if smCtx.Ref != "new-ref-after-dup" {
		t.Fatalf("expected SM context ref 'new-ref-after-dup', got: %s", smCtx.Ref)
	}

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
	ue.AllowedNssai = []models.Snssai{*snssai}

	var pduSessionID uint8 = 5

	_ = ue.CreateSmContext(pduSessionID, "existing-ref", snssai, "internet")

	smPayload := []byte{0x2E, 0x05, 0x00, 0xC1, 0x00}

	msg := buildTestULNASTransport(fgs.PayloadContainerTypeN1SMInfo, smPayload, pduSessionIDPtr(fgs.PDUSessionID(pduSessionID)))
	setRequestType(msg, fgs.RequestTypeExistingPDUSession)

	fakeSmf := &fakeSmf{
		UpdateN1MsgResponse: nil,
	}

	amfInstance := amf.New(nil, nil, fakeSmf)

	transport5GSMMessage(t.Context(), amfInstance, ue, fgsULNAS(t, msg))

	if len(fakeSmf.UpdateN1MsgCalls) != 1 {
		t.Fatalf("expected 1 UpdateSmContextN1Msg call, got: %d", len(fakeSmf.UpdateN1MsgCalls))
	}

	if fakeSmf.UpdateN1MsgCalls[0].SmContextRef != "existing-ref" {
		t.Fatalf("expected amf.SmContextRef 'existing-ref', got: %s", fakeSmf.UpdateN1MsgCalls[0].SmContextRef)
	}
}

func TestTransport5GSMMessage_SmContextExists_DefaultRequestType_ForwardsToSMF(t *testing.T) {
	ue, _, err := buildUeAndRadio()
	if err != nil {
		t.Fatalf("could not build UE and radio: %v", err)
	}

	var pduSessionID uint8 = 5

	_ = ue.CreateSmContext(pduSessionID, "default-ref", &models.Snssai{Sst: 1, Sd: "010203"}, "internet")

	smPayload := []byte{0x2E, 0x05, 0x00, 0xC1, 0x00}

	msg := buildTestULNASTransport(fgs.PayloadContainerTypeN1SMInfo, smPayload, pduSessionIDPtr(fgs.PDUSessionID(pduSessionID)))
	setRequestType(msg, 7)

	fakeSmf := &fakeSmf{
		UpdateN1MsgResponse: nil,
	}

	amfInstance := amf.New(nil, nil, fakeSmf)

	transport5GSMMessage(t.Context(), amfInstance, ue, fgsULNAS(t, msg))

	if len(fakeSmf.UpdateN1MsgCalls) != 1 {
		t.Fatalf("expected 1 UpdateSmContextN1Msg call, got: %d", len(fakeSmf.UpdateN1MsgCalls))
	}
}

func TestTransport5GSMMessage_NoSmContext_InitialRequest_WithSNSSAIAndDNN_CreateSmContext(t *testing.T) {
	ue, _, err := buildUeAndRadio()
	if err != nil {
		t.Fatalf("could not build UE and radio: %v", err)
	}

	ue.SetSupiForTest(mustSUPIFromPrefixed("imsi-001010000000001"))
	ue.AllowedNssai = []models.Snssai{{Sst: 1, Sd: "010203"}}

	var pduSessionID uint8 = 1

	smPayload := []byte{0x2E, 0x01, 0x00, 0xC1, 0x00}

	msg := buildTestULNASTransport(fgs.PayloadContainerTypeN1SMInfo, smPayload, pduSessionIDPtr(fgs.PDUSessionID(pduSessionID)))
	setRequestType(msg, fgs.RequestTypeInitialRequest)

	msg.SNSSAI = &fgs.SNSSAI{SST: 1, SD: &[3]byte{1, 2, 3}}

	msg.DNN = new(fgs.DNN("internet"))

	fakeSmf := &fakeSmf{
		CreateSmContextRef: "new-ctx-ref",
	}

	amfInstance := amf.New(&fakeDBInstance{}, nil, fakeSmf)

	transport5GSMMessage(t.Context(), amfInstance, ue, fgsULNAS(t, msg))

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

	ue.SetSupiForTest(mustSUPIFromPrefixed("imsi-001010000000001"))
	ue.AllowedNssai = []models.Snssai{{Sst: 1, Sd: "aabbcc"}}

	var pduSessionID uint8 = 2

	smPayload := []byte{0x2E, 0x02, 0x00, 0xC1, 0x00}

	msg := buildTestULNASTransport(fgs.PayloadContainerTypeN1SMInfo, smPayload, pduSessionIDPtr(fgs.PDUSessionID(pduSessionID)))
	setRequestType(msg, fgs.RequestTypeInitialRequest)

	fakeSmf := &fakeSmf{
		CreateSmContextRef: "default-ctx-ref",
	}

	amfInstance := amf.New(&fakeDBInstance{}, nil, fakeSmf)

	transport5GSMMessage(t.Context(), amfInstance, ue, fgsULNAS(t, msg))

	if len(fakeSmf.CreateSmContextCalls) != 1 {
		t.Fatalf("expected 1 CreateSmContext call, got: %d", len(fakeSmf.CreateSmContextCalls))
	}

	call := fakeSmf.CreateSmContextCalls[0]

	if call.Snssai.Sst != 1 || call.Snssai.Sd != "aabbcc" {
		t.Fatalf("expected default SNSSAI SST=1 SD=aabbcc, got: SST=%d SD=%s", call.Snssai.Sst, call.Snssai.Sd)
	}

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

func TestTransport5GSMMessage_NoSmContext_InitialRequest_NilAllowedNssai_PayloadNotForwarded(t *testing.T) {
	ue, ngapSender, err := buildUeAndRadio()
	if err != nil {
		t.Fatalf("could not build UE and radio: %v", err)
	}

	ue.SetSupiForTest(mustSUPIFromPrefixed("imsi-001010000000001"))
	ue.AllowedNssai = nil

	var pduSessionID uint8 = 1

	smPayload := []byte{0x2E, 0x01, 0x00, 0xC1, 0x00}

	msg := buildTestULNASTransport(fgs.PayloadContainerTypeN1SMInfo, smPayload, pduSessionIDPtr(fgs.PDUSessionID(pduSessionID)))
	setRequestType(msg, fgs.RequestTypeInitialRequest)

	fakeSmf := &fakeSmf{}

	amfInstance := amf.New(&fakeDBInstance{}, nil, fakeSmf)

	transport5GSMMessage(t.Context(), amfInstance, ue, fgsULNAS(t, msg))

	if len(fakeSmf.CreateSmContextCalls) != 0 {
		t.Fatalf("a UE with no allowed NSSAI must not reach the SMF, got %d calls", len(fakeSmf.CreateSmContextCalls))
	}

	if len(ngapSender.SentDownlinkNASTransport) != 1 {
		t.Fatalf("expected 1 downlink NAS transport, got %d", len(ngapSender.SentDownlinkNASTransport))
	}

	dl := assertPlainDLTransport(t, ngapSender.SentDownlinkNASTransport[0].NASPDU)
	if dl.Cause == nil || *dl.Cause != 0x5a {
		t.Fatalf("expected 5GMM cause %d (payload was not forwarded)", 0x5a)
	}
}

func TestTransport5GSMMessage_NoSmContext_InitialRequest_CreateSmContext_ErrorResponse_SendsDLNAS(t *testing.T) {
	ue, ngapSender, err := buildUeAndRadio()
	if err != nil {
		t.Fatalf("could not build UE and radio: %v", err)
	}

	ue.SetSupiForTest(mustSUPIFromPrefixed("imsi-001010000000001"))
	ue.AllowedNssai = []models.Snssai{{Sst: 1, Sd: "010203"}}

	var pduSessionID uint8 = 1

	smPayload := []byte{0x2E, 0x01, 0x00, 0xC1, 0x00}

	msg := buildTestULNASTransport(fgs.PayloadContainerTypeN1SMInfo, smPayload, pduSessionIDPtr(fgs.PDUSessionID(pduSessionID)))
	setRequestType(msg, fgs.RequestTypeInitialRequest)

	fakeSmf := &fakeSmf{
		CreateSmContextErrResp: []byte{0x2E, 0x01, 0x00, 0xC2, 0x24},
		CreateSmContextError:   fmt.Errorf("policy not found"),
	}

	amfInstance := amf.New(&fakeDBInstance{}, nil, fakeSmf)

	transport5GSMMessage(t.Context(), amfInstance, ue, fgsULNAS(t, msg))

	if len(ngapSender.SentDownlinkNASTransport) != 1 {
		t.Fatalf("expected 1 downlink NAS transport, got: %d", len(ngapSender.SentDownlinkNASTransport))
	}

	resp := ngapSender.SentDownlinkNASTransport[0]
	assertPlainGmm(t, resp.NASPDU, uint8(fgs.MsgDLNASTransport))

	if _, exists := ue.SmContextFindByPDUSessionID(pduSessionID); exists {
		t.Fatal("expected SM context NOT to exist after rejection")
	}
}

func TestTransport5GSMMessage_ExistingPduSession_MultiSliceAllowedNssai_MatchesSecondSlice(t *testing.T) {
	ue, _, err := buildUeAndRadio()
	if err != nil {
		t.Fatalf("could not build UE and radio: %v", err)
	}

	ue.AllowedNssai = []models.Snssai{
		{Sst: 1, Sd: "010203"},
		{Sst: 2, Sd: "aabbcc"},
		{Sst: 3, Sd: "ddeeff"},
	}

	var pduSessionID uint8 = 5

	_ = ue.CreateSmContext(pduSessionID, "existing-ref", &models.Snssai{Sst: 2, Sd: "aabbcc"}, "internet")

	smPayload := []byte{0x2E, 0x05, 0x00, 0xC1, 0x00}

	msg := buildTestULNASTransport(fgs.PayloadContainerTypeN1SMInfo, smPayload, pduSessionIDPtr(fgs.PDUSessionID(pduSessionID)))
	setRequestType(msg, fgs.RequestTypeExistingPDUSession)

	fakeSmf := &fakeSmf{
		UpdateN1MsgResponse: nil,
	}

	amfInstance := amf.New(nil, nil, fakeSmf)

	transport5GSMMessage(t.Context(), amfInstance, ue, fgsULNAS(t, msg))

	if len(fakeSmf.UpdateN1MsgCalls) != 1 {
		t.Fatalf("expected 1 UpdateSmContextN1Msg call, got: %d", len(fakeSmf.UpdateN1MsgCalls))
	}

	if fakeSmf.UpdateN1MsgCalls[0].SmContextRef != "existing-ref" {
		t.Fatalf("expected amf.SmContextRef 'existing-ref', got: %s", fakeSmf.UpdateN1MsgCalls[0].SmContextRef)
	}
}

func TestTransport5GSMMessage_ExistingPduSession_MultiSliceAllowedNssai_NotInList(t *testing.T) {
	ue, ngapSender, err := buildUeAndRadio()
	if err != nil {
		t.Fatalf("could not build UE and radio: %v", err)
	}

	ue.AllowedNssai = []models.Snssai{
		{Sst: 1, Sd: "010203"},
		{Sst: 2, Sd: "aabbcc"},
		{Sst: 3, Sd: "ddeeff"},
	}

	var pduSessionID uint8 = 5

	_ = ue.CreateSmContext(pduSessionID, "existing-ref", &models.Snssai{Sst: 9, Sd: "ffffff"}, "internet")

	smPayload := []byte{0x2E, 0x05, 0x00, 0xC1, 0x00}

	msg := buildTestULNASTransport(fgs.PayloadContainerTypeN1SMInfo, smPayload, pduSessionIDPtr(fgs.PDUSessionID(pduSessionID)))
	setRequestType(msg, fgs.RequestTypeExistingPDUSession)

	transport5GSMMessage(t.Context(), amf.New(nil, nil, nil), ue, fgsULNAS(t, msg))

	if len(ngapSender.SentDownlinkNASTransport) != 1 {
		t.Fatalf("expected 1 downlink NAS transport, got: %d", len(ngapSender.SentDownlinkNASTransport))
	}
}

func TestTransport5GSMMessage_NoSmContext_InitialRequest_MultiSliceDefaultSNSSAI(t *testing.T) {
	ue, _, err := buildUeAndRadio()
	if err != nil {
		t.Fatalf("could not build UE and radio: %v", err)
	}

	ue.SetSupiForTest(mustSUPIFromPrefixed("imsi-001010000000001"))

	ue.AllowedNssai = []models.Snssai{
		{Sst: 1, Sd: "aabbcc"},
		{Sst: 2, Sd: "010203"},
		{Sst: 3, Sd: "ddeeff"},
	}

	var pduSessionID uint8 = 2

	smPayload := []byte{0x2E, 0x02, 0x00, 0xC1, 0x00}

	msg := buildTestULNASTransport(fgs.PayloadContainerTypeN1SMInfo, smPayload, pduSessionIDPtr(fgs.PDUSessionID(pduSessionID)))
	setRequestType(msg, fgs.RequestTypeInitialRequest)

	fakeSmf := &fakeSmf{
		CreateSmContextRef: "multi-slice-ref",
	}

	amfInstance := amf.New(&fakeDBInstance{}, nil, fakeSmf)

	transport5GSMMessage(t.Context(), amfInstance, ue, fgsULNAS(t, msg))

	if len(fakeSmf.CreateSmContextCalls) != 1 {
		t.Fatalf("expected 1 CreateSmContext call, got: %d", len(fakeSmf.CreateSmContextCalls))
	}

	call := fakeSmf.CreateSmContextCalls[0]

	if call.Snssai.Sst != 1 || call.Snssai.Sd != "aabbcc" {
		t.Fatalf("expected default SNSSAI SST=1 SD=aabbcc, got: SST=%d SD=%s", call.Snssai.Sst, call.Snssai.Sd)
	}
}

func TestForward5GSMMessageToSMF_NoRANUEContext_DeliversN1WithoutReleaseCommand(t *testing.T) {
	ue, ngapSender, err := buildUeAndRadio()
	if err != nil {
		t.Fatalf("could not build UE and radio: %v", err)
	}

	fakeSmf := &fakeSmf{
		UpdateN1MsgResponse: &smf.UpdateResult{
			N1Msg:     []byte{0x2E, 0x01, 0x00, 0xD6, 0x24},
			N2Msg:     []byte{0x00, 0x00, 0x04, 0x00, 0x00, 0x00, 0x00, 0x00},
			ReleaseN2: true,
		},
	}

	amfInstance := amf.New(nil, nil, fakeSmf)

	forward5GSMMessageToSMF(t.Context(), amfInstance, ue, 1, "ref-1", []byte{0x01})

	if len(ngapSender.SentPDUSessionResourceReleaseCommand) != 0 {
		t.Errorf("release commands = %d, want 0: the NG-RAN node holds no UE context to release resources from",
			len(ngapSender.SentPDUSessionResourceReleaseCommand))
	}

	if len(ngapSender.SentDownlinkNASTransport) != 1 {
		t.Fatalf("downlink nas transports = %d, want 1: the UE still needs the 5GSM message", len(ngapSender.SentDownlinkNASTransport))
	}
}
