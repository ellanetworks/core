// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"context"
	"fmt"
	"testing"

	"github.com/ellanetworks/core/etsi"
	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/models"
	"github.com/free5gc/nas"
	"github.com/free5gc/nas/nasMessage"
	"github.com/free5gc/nas/nasType"
	"github.com/free5gc/nas/security"
)

// failingSubscriberDB is a fakeDBInstance variant that returns an error for GetSubscriber.
type failingSubscriberDB struct {
	Operator *db.Operator
}

func (fdb *failingSubscriberDB) GetOperator(ctx context.Context) (*db.Operator, error) {
	if fdb.Operator == nil {
		return nil, fmt.Errorf("could not get operator")
	}

	return fdb.Operator, nil
}

func (fdb *failingSubscriberDB) GetDataNetworkByID(ctx context.Context, id string) (*db.DataNetwork, error) {
	return &db.DataNetwork{ID: id, Name: "TestDataNetwork"}, nil
}

func (fdb *failingSubscriberDB) GetNetworkSliceByID(_ context.Context, id string) (*db.NetworkSlice, error) {
	return &db.NetworkSlice{ID: id, Name: "TestSlice", Sst: 1}, nil
}

func (fdb *failingSubscriberDB) ListNetworkSlicesByIDs(_ context.Context, ids []string) ([]db.NetworkSlice, error) {
	var out []db.NetworkSlice
	for _, id := range ids {
		out = append(out, db.NetworkSlice{ID: id, Name: "TestSlice", Sst: 1})
	}

	return out, nil
}

func (fdb *failingSubscriberDB) GetSubscriber(ctx context.Context, imsi string) (*db.Subscriber, error) {
	return nil, fmt.Errorf("subscriber not found")
}

func (fdb *failingSubscriberDB) GetProfileByID(ctx context.Context, id string) (*db.Profile, error) {
	return &db.Profile{ID: id, Name: "TestProfile", Allow4G: true, Allow5G: true}, nil
}

func (fdb *failingSubscriberDB) ListAllNetworkSlices(ctx context.Context) ([]db.NetworkSlice, error) {
	return []db.NetworkSlice{{ID: "slice-1", Sst: 1, Name: "default"}}, nil
}

func (fdb *failingSubscriberDB) GetPolicyByProfileAndSlice(ctx context.Context, profileID, sliceID string) (*db.Policy, error) {
	return &db.Policy{ID: "policy-1", Name: "TestPolicy", ProfileID: profileID, SliceID: sliceID, DataNetworkID: "dn-1"}, nil
}

func (fdb *failingSubscriberDB) ListPoliciesByProfile(_ context.Context, _ string) ([]db.Policy, error) {
	return []db.Policy{{ID: "policy-1", Name: "TestPolicy", ProfileID: "profile-1", SliceID: "slice-1", DataNetworkID: "dn-1"}}, nil
}

func (fdb *failingSubscriberDB) NodeID() int { return 0 }

// decryptAndDecodeNasPdu decrypts a ciphered NAS PDU using the UE's security context
// and decodes it, returning the NAS message. It verifies the security header is
// IntegrityProtectedAndCiphered. The dlCountOffset parameter specifies the offset
// from ue.ULCount.Get() to use as the DL count (0 for the first message, 1 for
// the second, etc.).
func decryptAndDecodeNasPdu(t *testing.T, ue *amf.UeContext, nasPdu []byte, dlCountOffset uint32) *nas.Message {
	t.Helper()

	nm := new(nas.Message)
	nm.SecurityHeaderType = nas.GetSecurityHeaderType(nasPdu) & 0x0f

	if nm.SecurityHeaderType != nas.SecurityHeaderTypeIntegrityProtectedAndCiphered {
		t.Fatalf("expected IntegrityProtectedAndCiphered, got security header type %d", nm.SecurityHeaderType)
	}

	payload := make([]byte, len(nasPdu))
	copy(payload, nasPdu)
	payload = payload[7:]

	if err := security.NASEncrypt(ue.CipheringAlgForTest(), ue.KnasEncForTest(), ue.ULCountForTest().Value()+dlCountOffset, security.Bearer3GPP, security.DirectionDownlink, payload); err != nil {
		t.Fatalf("could not decrypt NAS message: %v", err)
	}

	if err := nm.PlainNasDecode(&payload); err != nil {
		t.Fatalf("could not decode NAS message: %v", err)
	}

	return nm
}

// buildMobilityRegUeAndAMF creates a UE and amf.AMF configured for mobility/periodic
// registration updating tests. The UE has security context, a valid registration
// request, Pei, Supi, and matching Tai. The amf.AMF has a valid Operator, fakeSmf, and
// UEs map. Returns the UE, ngapSender, fakeSmf, and amf.AMF.
func buildMobilityRegUeAndAMF(t *testing.T) (*amf.UeContext, *fakeNGAPSender, *fakeSmf, *amf.AMF) {
	t.Helper()

	supi := mustSUPIFromPrefixed("imsi-001019756139935")
	fakeSmf := &fakeSmf{}
	amfInstance := amf.New(
		&fakeDBInstance{
			Operator: &db.Operator{
				Mcc:           "001",
				Mnc:           "01",
				SupportedTACs: "[\"000001\"]",
			},
		},
		nil,
		fakeSmf,
	)

	ue, ngapSender, err := buildUeAndRadio()
	if err != nil {
		t.Fatalf("could not create UE and radio: %v", err)
	}

	ue.SetSupiForTest(supi)
	ue.Imei, _ = etsi.NewIMEIFromPEI("imei-490154203237518")
	ue.Tai = ue.Conn().Tai
	ue.SetSecuredForTest(true)
	{
		ng := ue.NgKsiForTest()
		ng.Ksi = 1
		ue.SetNgKsiForTest(ng)
	}

	key := [16]uint8{0x0D, 0x0E, 0x0A, 0x0D, 0x0B, 0x0E, 0x0E, 0x0F, 0x0F, 0x0E, 0x0E, 0x0D, 0x0C, 0x0A, 0x0F, 0x0E}
	algo := security.AlgCiphering128NEA2

	ue.SetKnasEncForTest(key)
	ue.SetKnasIntForTest(key)
	ue.SetCipheringAlgForTest(algo)
	ue.SetIntegrityAlgForTest(security.AlgIntegrity128NIA0)

	registrationRequest, err := buildTestRegistrationRequestMessage(algo, &key, ue.ULCountForTest().Value())
	if err != nil {
		t.Fatalf("could not build registration request message: %v", err)
	}

	ue.Conn().RegistrationRequest = registrationRequest.RegistrationRequest
	ue.Conn().RegistrationType5GS = nasMessage.RegistrationType5GSMobilityRegistrationUpdating
	ue.Conn().RegistrationRequest.Capability5GMM = &nasType.Capability5GMM{}

	return ue, ngapSender, fakeSmf, amfInstance
}

func TestMobilityReg_GetOperatorInfoError(t *testing.T) {
	ue, _, _, amfInstance := buildMobilityRegUeAndAMF(t)

	amfInstance.DBInstance = &fakeDBInstance{Operator: nil}

	HandleMobilityAndPeriodicRegistrationUpdating(context.TODO(), amfInstance, ue)

	if ue.State() != amf.Deregistered {
		t.Fatalf("UE should be released to Deregistered, got %v", ue.State())
	}
}

// A mobility registration update with no 5GMM capability IE is valid: the IE
// is optional and re-sent only on change (TS 24.501), so the
// amf.AMF accepts it.
func TestMobilityReg_NilCapability5GMM_Mobility_Continues(t *testing.T) {
	ue, ngapSender, _, amfInstance := buildMobilityRegUeAndAMF(t)

	ue.Conn().RegistrationRequest.Capability5GMM = nil
	ue.Conn().RegistrationType5GS = nasMessage.RegistrationType5GSMobilityRegistrationUpdating

	HandleMobilityAndPeriodicRegistrationUpdating(context.TODO(), amfInstance, ue)

	if len(ngapSender.SentDownlinkNASTransport) != 1 {
		t.Fatalf("expected 1 DownlinkNASTransport, got %d", len(ngapSender.SentDownlinkNASTransport))
	}

	nm := decryptAndDecodeNasPdu(t, ue, ngapSender.SentDownlinkNASTransport[0].NasPdu, 0)
	if nm.GmmHeader.GetMessageType() != nas.MsgTypeRegistrationAccept {
		t.Fatalf("expected RegistrationAccept, got %v", nm.GmmHeader.GetMessageType())
	}
}

func TestMobilityReg_NilCapability5GMM_Periodic_Continues(t *testing.T) {
	ue, ngapSender, _, amfInstance := buildMobilityRegUeAndAMF(t)

	ue.Conn().RegistrationRequest.Capability5GMM = nil
	ue.Conn().RegistrationType5GS = nasMessage.RegistrationType5GSPeriodicRegistrationUpdating

	HandleMobilityAndPeriodicRegistrationUpdating(context.TODO(), amfInstance, ue)

	if len(ngapSender.SentDownlinkNASTransport) != 1 {
		t.Fatalf("expected 1 DownlinkNASTransport, got %d", len(ngapSender.SentDownlinkNASTransport))
	}

	nm := decryptAndDecodeNasPdu(t, ue, ngapSender.SentDownlinkNASTransport[0].NasPdu, 0)
	if nm.GmmHeader.GetMessageType() != nas.MsgTypeRegistrationAccept {
		t.Fatalf("expected RegistrationAccept, got %v", nm.GmmHeader.GetMessageType())
	}
}

func TestMobilityReg_UpdateType5GS_ClearsRadioCapability(t *testing.T) {
	ue, ngapSender, _, amfInstance := buildMobilityRegUeAndAMF(t)

	ue.RadioCapability = []byte("some-capability")
	ue.RadioCapabilityForPaging = &models.UERadioCapabilityForPaging{}

	updateType := nasType.NewUpdateType5GS(nasMessage.RegistrationRequestUpdateType5GSType)
	updateType.SetNGRanRcu(nasMessage.NGRanRadioCapabilityUpdateNeeded)
	ue.Conn().RegistrationRequest.UpdateType5GS = updateType

	HandleMobilityAndPeriodicRegistrationUpdating(context.TODO(), amfInstance, ue)

	if len(ue.RadioCapability) != 0 {
		t.Fatalf("expected RadioCapability to be cleared, got %x", ue.RadioCapability)
	}

	if ue.RadioCapabilityForPaging != nil {
		t.Fatalf("expected RadioCapabilityForPaging to be nil")
	}

	if len(ngapSender.SentDownlinkNASTransport) != 1 {
		t.Fatalf("expected 1 DownlinkNASTransport, got %d", len(ngapSender.SentDownlinkNASTransport))
	}

	nm := decryptAndDecodeNasPdu(t, ue, ngapSender.SentDownlinkNASTransport[0].NasPdu, 0)
	if nm.GmmHeader.GetMessageType() != nas.MsgTypeRegistrationAccept {
		t.Fatalf("expected RegistrationAccept, got %v", nm.GmmHeader.GetMessageType())
	}
}

func TestMobilityReg_MICOIndication(t *testing.T) {
	ue, ngapSender, _, amfInstance := buildMobilityRegUeAndAMF(t)

	ue.Conn().RegistrationRequest.MICOIndication = nasType.NewMICOIndication(nasMessage.RegistrationRequestMICOIndicationType)

	HandleMobilityAndPeriodicRegistrationUpdating(context.TODO(), amfInstance, ue)

	if len(ngapSender.SentDownlinkNASTransport) != 1 {
		t.Fatalf("expected 1 DownlinkNASTransport, got %d", len(ngapSender.SentDownlinkNASTransport))
	}

	nm := decryptAndDecodeNasPdu(t, ue, ngapSender.SentDownlinkNASTransport[0].NasPdu, 0)
	if nm.GmmHeader.GetMessageType() != nas.MsgTypeRegistrationAccept {
		t.Fatalf("expected RegistrationAccept, got %v", nm.GmmHeader.GetMessageType())
	}
}

func TestMobilityReg_RequestedDRXParameters(t *testing.T) {
	ue, ngapSender, _, amfInstance := buildMobilityRegUeAndAMF(t)

	drxParams := nasType.NewRequestedDRXParameters(nasMessage.RegistrationRequestRequestedDRXParametersType)
	drxParams.SetDRXValue(0x03)
	ue.Conn().RegistrationRequest.RequestedDRXParameters = drxParams

	HandleMobilityAndPeriodicRegistrationUpdating(context.TODO(), amfInstance, ue)

	if ue.DRXParameter != 0x03 {
		t.Fatalf("expected DRXParameter to be 0x03, got 0x%02x", ue.DRXParameter)
	}

	if len(ngapSender.SentDownlinkNASTransport) != 1 {
		t.Fatalf("expected 1 DownlinkNASTransport, got %d", len(ngapSender.SentDownlinkNASTransport))
	}

	nm := decryptAndDecodeNasPdu(t, ue, ngapSender.SentDownlinkNASTransport[0].NasPdu, 0)
	if nm.GmmHeader.GetMessageType() != nas.MsgTypeRegistrationAccept {
		t.Fatalf("expected RegistrationAccept, got %v", nm.GmmHeader.GetMessageType())
	}
}

func TestMobilityReg_GetSubscriberProfileError(t *testing.T) {
	ue, _, _, amfInstance := buildMobilityRegUeAndAMF(t)

	amfInstance.DBInstance = &failingSubscriberDB{
		Operator: &db.Operator{
			Mcc:           "001",
			Mnc:           "01",
			SupportedTACs: "[\"000001\"]",
		},
	}

	HandleMobilityAndPeriodicRegistrationUpdating(context.TODO(), amfInstance, ue)

	if ue.State() != amf.Deregistered {
		t.Fatalf("UE should be released to Deregistered, got %v", ue.State())
	}
}

func TestMobilityReg_EmptyAllowedNssai_RejectsRegistration(t *testing.T) {
	ue, ngapSender, _, amfInstance := buildMobilityRegUeAndAMF(t)

	amfInstance.DBInstance = &emptyPolicyDB{fakeDBInstance: &fakeDBInstance{
		Operator: &db.Operator{
			Mcc:           "001",
			Mnc:           "01",
			SupportedTACs: "[\"000001\"]",
		},
	}}

	HandleMobilityAndPeriodicRegistrationUpdating(context.TODO(), amfInstance, ue)

	if ue.State() != amf.Deregistered {
		t.Fatalf("UE should be released to Deregistered after the reject, got %v", ue.State())
	}

	if len(ngapSender.SentDownlinkNASTransport) != 1 {
		t.Fatalf("expected 1 DownlinkNAS transport, got %d", len(ngapSender.SentDownlinkNASTransport))
	}

	resp := ngapSender.SentDownlinkNASTransport[0]
	nm := new(nas.Message)
	nm.SecurityHeaderType = nas.GetSecurityHeaderType(resp.NasPdu) & 0x0f

	if nm.SecurityHeaderType != nas.SecurityHeaderTypePlainNas {
		t.Fatalf("expected plain NAS, got security header type %d", nm.SecurityHeaderType)
	}

	if err := nm.PlainNasDecode(&resp.NasPdu); err != nil {
		t.Fatalf("could not decode plain NAS message: %v", err)
	}

	if nm.GmmHeader.GetMessageType() != nas.MsgTypeRegistrationReject {
		t.Fatalf("expected RegistrationReject, got %v", nm.GmmHeader.GetMessageType())
	}

	if nm.RegistrationReject == nil {
		t.Fatal("expected RegistrationReject payload")
	}

	if got, want := nm.RegistrationReject.GetCauseValue(), nasMessage.Cause5GMM5GSServicesNotAllowed; got != want {
		t.Fatalf("expected cause %d, got %d", want, got)
	}
}

func TestMobilityReg_UplinkDataStatus_ActivateSuccess_UeContextRequest(t *testing.T) {
	ue, ngapSender, fakeSmf, amfInstance := buildMobilityRegUeAndAMF(t)
	ue.AllowedNssai = []models.Snssai{{Sst: 1, Sd: "010203"}}
	setTestUESecurityCapability(ue)

	snssai := &models.Snssai{Sst: 1}

	_ = ue.CreateSmContext(2, "ref-2", snssai)

	// UplinkDataStatus: PSI 2 has uplink data (bit 2 in byte 0 = 0x04)
	ue.Conn().RegistrationRequest.UplinkDataStatus = &nasType.UplinkDataStatus{
		Len:    2,
		Buffer: []uint8{0x04, 0x00},
	}

	ue.Conn().UeContextRequest = true

	HandleMobilityAndPeriodicRegistrationUpdating(context.TODO(), amfInstance, ue)

	if len(fakeSmf.ActivateSmContextCalls) != 1 {
		t.Fatalf("expected 1 ActivateSmContext call, got %d", len(fakeSmf.ActivateSmContextCalls))
	}

	if fakeSmf.ActivateSmContextCalls[0].SmContextRef != "ref-2" {
		t.Fatalf("expected amf.SmContextRef 'ref-2', got %q", fakeSmf.ActivateSmContextCalls[0].SmContextRef)
	}

	// UeContextRequest=true → sends InitialContextSetupRequest
	if len(ngapSender.SentInitialContextSetupRequest) != 1 {
		t.Fatalf("expected 1 InitialContextSetupRequest, got %d", len(ngapSender.SentInitialContextSetupRequest))
	}

	nm := decryptAndDecodeNasPdu(t, ue, ngapSender.SentInitialContextSetupRequest[0].NasPdu, 0)
	if nm.GmmHeader.GetMessageType() != nas.MsgTypeRegistrationAccept {
		t.Fatalf("expected RegistrationAccept, got %v", nm.GmmHeader.GetMessageType())
	}
}

func TestMobilityReg_UplinkDataStatus_ActivateSuccess_NoUeContextRequest(t *testing.T) {
	ue, ngapSender, fakeSmf, amfInstance := buildMobilityRegUeAndAMF(t)

	snssai := &models.Snssai{Sst: 1}
	_ = ue.CreateSmContext(2, "ref-2", snssai)

	ue.Conn().RegistrationRequest.UplinkDataStatus = &nasType.UplinkDataStatus{
		Len:    2,
		Buffer: []uint8{0x04, 0x00},
	}

	ue.Conn().UeContextRequest = false

	HandleMobilityAndPeriodicRegistrationUpdating(context.TODO(), amfInstance, ue)

	if len(fakeSmf.ActivateSmContextCalls) != 1 {
		t.Fatalf("expected 1 ActivateSmContext call, got %d", len(fakeSmf.ActivateSmContextCalls))
	}

	// UeContextRequest=false + non-empty suList → sends PDUSessionResourceSetupRequest
	if len(ngapSender.SentPDUSessionResourceSetupRequest) != 1 {
		t.Fatalf("expected 1 PDUSessionResourceSetupRequest, got %d", len(ngapSender.SentPDUSessionResourceSetupRequest))
	}

	nm := decryptAndDecodeNasPdu(t, ue, ngapSender.SentPDUSessionResourceSetupRequest[0].NasPdu, 0)
	if nm.GmmHeader.GetMessageType() != nas.MsgTypeRegistrationAccept {
		t.Fatalf("expected RegistrationAccept, got %v", nm.GmmHeader.GetMessageType())
	}
}

func TestMobilityReg_UplinkDataStatus_ActivateError(t *testing.T) {
	ue, ngapSender, fakeSmf, amfInstance := buildMobilityRegUeAndAMF(t)

	snssai := &models.Snssai{Sst: 1}
	_ = ue.CreateSmContext(2, "ref-2", snssai)

	ue.Conn().RegistrationRequest.UplinkDataStatus = &nasType.UplinkDataStatus{
		Len:    2,
		Buffer: []uint8{0x04, 0x00},
	}

	fakeSmf.ActivateSmContextError = fmt.Errorf("activate error")

	HandleMobilityAndPeriodicRegistrationUpdating(context.TODO(), amfInstance, ue)

	if len(fakeSmf.ActivateSmContextCalls) != 1 {
		t.Fatalf("expected 1 ActivateSmContext call, got %d", len(fakeSmf.ActivateSmContextCalls))
	}

	// Even with error, the function continues and sends RegistrationAccept
	if len(ngapSender.SentDownlinkNASTransport) != 1 {
		t.Fatalf("expected 1 DownlinkNASTransport, got %d", len(ngapSender.SentDownlinkNASTransport))
	}

	nm := decryptAndDecodeNasPdu(t, ue, ngapSender.SentDownlinkNASTransport[0].NasPdu, 0)
	if nm.GmmHeader.GetMessageType() != nas.MsgTypeRegistrationAccept {
		t.Fatalf("expected RegistrationAccept, got %v", nm.GmmHeader.GetMessageType())
	}
}

func TestMobilityReg_PDUSessionStatus_InactiveSession_ReleaseSmContext(t *testing.T) {
	ue, ngapSender, fakeSmf, amfInstance := buildMobilityRegUeAndAMF(t)

	snssai := &models.Snssai{Sst: 1}
	_ = ue.CreateSmContext(2, "ref-2", snssai)

	// PDUSessionStatus: PSI 2 is NOT active (bit 2 unset = 0x00)
	ue.Conn().RegistrationRequest.PDUSessionStatus = &nasType.PDUSessionStatus{
		Len:    2,
		Buffer: []uint8{0x00, 0x00},
	}

	HandleMobilityAndPeriodicRegistrationUpdating(context.TODO(), amfInstance, ue)

	if len(fakeSmf.ReleaseSmContextCalls) != 1 {
		t.Fatalf("expected 1 ReleaseSmContext call, got %d", len(fakeSmf.ReleaseSmContextCalls))
	}

	if fakeSmf.ReleaseSmContextCalls[0].SmContextRef != "ref-2" {
		t.Fatalf("expected amf.SmContextRef 'ref-2', got %q", fakeSmf.ReleaseSmContextCalls[0].SmContextRef)
	}

	if len(fakeSmf.ReleasedSmContext) != 1 || fakeSmf.ReleasedSmContext[0] != "ref-2" {
		t.Fatalf("expected ReleasedSmContext to contain 'ref-2', got %v", fakeSmf.ReleasedSmContext)
	}

	if len(ngapSender.SentDownlinkNASTransport) != 1 {
		t.Fatalf("expected 1 DownlinkNASTransport, got %d", len(ngapSender.SentDownlinkNASTransport))
	}

	nm := decryptAndDecodeNasPdu(t, ue, ngapSender.SentDownlinkNASTransport[0].NasPdu, 0)
	if nm.GmmHeader.GetMessageType() != nas.MsgTypeRegistrationAccept {
		t.Fatalf("expected RegistrationAccept, got %v", nm.GmmHeader.GetMessageType())
	}
}

func TestMobilityReg_PDUSessionStatus_ActiveSession_NoRelease(t *testing.T) {
	ue, ngapSender, fakeSmf, amfInstance := buildMobilityRegUeAndAMF(t)

	snssai := &models.Snssai{Sst: 1}
	_ = ue.CreateSmContext(2, "ref-2", snssai)

	// PDUSessionStatus: PSI 2 IS active (bit 2 set = 0x04)
	ue.Conn().RegistrationRequest.PDUSessionStatus = &nasType.PDUSessionStatus{
		Len:    2,
		Buffer: []uint8{0x04, 0x00},
	}

	HandleMobilityAndPeriodicRegistrationUpdating(context.TODO(), amfInstance, ue)

	if len(fakeSmf.ReleaseSmContextCalls) != 0 {
		t.Fatalf("expected 0 ReleaseSmContext calls, got %d", len(fakeSmf.ReleaseSmContextCalls))
	}

	if len(ngapSender.SentDownlinkNASTransport) != 1 {
		t.Fatalf("expected 1 DownlinkNASTransport, got %d", len(ngapSender.SentDownlinkNASTransport))
	}

	nm := decryptAndDecodeNasPdu(t, ue, ngapSender.SentDownlinkNASTransport[0].NasPdu, 0)
	if nm.GmmHeader.GetMessageType() != nas.MsgTypeRegistrationAccept {
		t.Fatalf("expected RegistrationAccept, got %v", nm.GmmHeader.GetMessageType())
	}
}

func TestMobilityReg_PDUSessionStatus_ReleaseError(t *testing.T) {
	ue, ngapSender, fakeSmf, amfInstance := buildMobilityRegUeAndAMF(t)

	snssai := &models.Snssai{Sst: 1}
	_ = ue.CreateSmContext(2, "ref-2", snssai)

	ue.Conn().RegistrationRequest.PDUSessionStatus = &nasType.PDUSessionStatus{
		Len:    2,
		Buffer: []uint8{0x00, 0x00}, // PSI 2 inactive → triggers release
	}

	fakeSmf.ReleaseSmContextError = fmt.Errorf("release error")

	HandleMobilityAndPeriodicRegistrationUpdating(context.TODO(), amfInstance, ue)

	// A ReleaseSmContext failure aborts the update before any Registration Accept is sent.
	if len(fakeSmf.ReleaseSmContextCalls) != 1 {
		t.Fatalf("expected one ReleaseSmContext attempt, got %d", len(fakeSmf.ReleaseSmContextCalls))
	}

	if len(ngapSender.SentDownlinkNASTransport) != 0 {
		t.Fatalf("expected no downlink after release failure, got %d", len(ngapSender.SentDownlinkNASTransport))
	}

	if len(ngapSender.SentPDUSessionResourceSetupRequest) != 0 {
		t.Fatalf("expected no PDU session resource setup after release failure, got %d", len(ngapSender.SentPDUSessionResourceSetupRequest))
	}
}

func TestMobilityReg_AllowedPDUSessionStatus_N1N2_NilN2Info_NonEmptySuList(t *testing.T) {
	ue, ngapSender, fakeSmf, amfInstance := buildMobilityRegUeAndAMF(t)

	snssai := &models.Snssai{Sst: 1}
	_ = ue.CreateSmContext(2, "ref-2", snssai)

	// UplinkDataStatus with PSI 2 + no UeContextRequest → populates suList
	ue.Conn().RegistrationRequest.UplinkDataStatus = &nasType.UplinkDataStatus{
		Len:    2,
		Buffer: []uint8{0x04, 0x00},
	}
	ue.Conn().UeContextRequest = false

	ue.Conn().RegistrationRequest.AllowedPDUSessionStatus = &nasType.AllowedPDUSessionStatus{
		Len:    2,
		Buffer: []uint8{0x04, 0x00},
	}
	ue.SetN1N2Message(&models.N1N2MessageTransferRequest{
		PduSessionID:            3,
		BinaryDataN1Message:     []byte{0x01, 0x02},
		BinaryDataN2Information: nil,
	})

	HandleMobilityAndPeriodicRegistrationUpdating(context.TODO(), amfInstance, ue)

	if len(fakeSmf.ActivateSmContextCalls) != 1 {
		t.Fatalf("expected 1 ActivateSmContext call, got %d", len(fakeSmf.ActivateSmContextCalls))
	}

	// suList non-empty → PDUSessionResourceSetupRequest + DLNASTransport
	if len(ngapSender.SentPDUSessionResourceSetupRequest) != 1 {
		t.Fatalf("expected 1 PDUSessionResourceSetupRequest, got %d", len(ngapSender.SentPDUSessionResourceSetupRequest))
	}

	if len(ngapSender.SentDownlinkNASTransport) != 1 {
		t.Fatalf("expected 1 DownlinkNASTransport (DLNASTransport for N1), got %d", len(ngapSender.SentDownlinkNASTransport))
	}

	nmSetup := decryptAndDecodeNasPdu(t, ue, ngapSender.SentPDUSessionResourceSetupRequest[0].NasPdu, 0)
	if nmSetup.GmmHeader.GetMessageType() != nas.MsgTypeRegistrationAccept {
		t.Fatalf("expected RegistrationAccept in PDUSessionResourceSetupRequest, got %v", nmSetup.GmmHeader.GetMessageType())
	}

	nmDL := decryptAndDecodeNasPdu(t, ue, ngapSender.SentDownlinkNASTransport[0].NasPdu, 1)
	if nmDL.GmmHeader.GetMessageType() != nas.MsgTypeDLNASTransport {
		t.Fatalf("expected DLNASTransport, got %v", nmDL.GmmHeader.GetMessageType())
	}

	if ue.N1N2Message() != nil {
		t.Fatal("expected N1N2Message to be nil after processing")
	}
}

func TestMobilityReg_AllowedPDUSessionStatus_N1N2_NilN2Info_EmptySuList(t *testing.T) {
	ue, ngapSender, _, amfInstance := buildMobilityRegUeAndAMF(t)

	// No UplinkDataStatus → suList remains empty

	ue.Conn().RegistrationRequest.AllowedPDUSessionStatus = &nasType.AllowedPDUSessionStatus{
		Len:    2,
		Buffer: []uint8{0x04, 0x00},
	}
	ue.SetN1N2Message(&models.N1N2MessageTransferRequest{
		PduSessionID:            3,
		BinaryDataN1Message:     []byte{0x01, 0x02},
		BinaryDataN2Information: nil,
	})

	// UeContextRequest=false so amf.SendRegistrationAccept sends DownlinkNasTransport
	ue.Conn().UeContextRequest = false

	HandleMobilityAndPeriodicRegistrationUpdating(context.TODO(), amfInstance, ue)

	// Empty suList → calls amf.SendRegistrationAccept (which sends DLNASTransport since UeContextRequest=false)
	// Then also sends DLNASTransport for N1 message
	if len(ngapSender.SentDownlinkNASTransport) != 2 {
		t.Fatalf("expected 2 DownlinkNASTransport (RegistrationAccept + N1 DLNASTransport), got %d", len(ngapSender.SentDownlinkNASTransport))
	}

	nmAccept := decryptAndDecodeNasPdu(t, ue, ngapSender.SentDownlinkNASTransport[0].NasPdu, 0)
	if nmAccept.GmmHeader.GetMessageType() != nas.MsgTypeRegistrationAccept {
		t.Fatalf("expected RegistrationAccept in first DLNASTransport, got %v", nmAccept.GmmHeader.GetMessageType())
	}

	nmN1 := decryptAndDecodeNasPdu(t, ue, ngapSender.SentDownlinkNASTransport[1].NasPdu, 1)
	if nmN1.GmmHeader.GetMessageType() != nas.MsgTypeDLNASTransport {
		t.Fatalf("expected DLNASTransport in second DLNASTransport, got %v", nmN1.GmmHeader.GetMessageType())
	}

	if ue.N1N2Message() != nil {
		t.Fatal("expected N1N2Message to be nil after processing")
	}
}

func TestMobilityReg_AllowedPDUSessionStatus_N1N2_WithN2Info_MissingSmContext(t *testing.T) {
	ue, _, _, amfInstance := buildMobilityRegUeAndAMF(t)

	ue.Conn().RegistrationRequest.AllowedPDUSessionStatus = &nasType.AllowedPDUSessionStatus{
		Len:    2,
		Buffer: []uint8{0x04, 0x00},
	}

	// N1N2 with N2Info, but no amf.SmContext for PduSessionID 3
	ue.SetN1N2Message(&models.N1N2MessageTransferRequest{
		PduSessionID:            3,
		BinaryDataN1Message:     []byte{0x01, 0x02},
		BinaryDataN2Information: []byte{0x03, 0x04},
	})

	HandleMobilityAndPeriodicRegistrationUpdating(context.TODO(), amfInstance, ue)

	if ue.State() != amf.Deregistered {
		t.Fatalf("UE should be released to Deregistered, got %v", ue.State())
	}
}

func TestMobilityReg_AllowedPDUSessionStatus_N1N2_WithN2Info_SmContextExists(t *testing.T) {
	ue, ngapSender, _, amfInstance := buildMobilityRegUeAndAMF(t)

	snssai := &models.Snssai{Sst: 1}
	_ = ue.CreateSmContext(3, "ref-3", snssai)

	ue.Conn().RegistrationRequest.AllowedPDUSessionStatus = &nasType.AllowedPDUSessionStatus{
		Len:    2,
		Buffer: []uint8{0x08, 0x00}, // PSI 3
	}

	ue.SetN1N2Message(&models.N1N2MessageTransferRequest{
		PduSessionID:            3,
		SNssai:                  snssai,
		BinaryDataN1Message:     []byte{0x01, 0x02},
		BinaryDataN2Information: []byte{0x03, 0x04},
	})

	ue.Conn().UeContextRequest = false

	HandleMobilityAndPeriodicRegistrationUpdating(context.TODO(), amfInstance, ue)

	// UeContextRequest=false + non-empty suList → PDUSessionResourceSetupRequest
	if len(ngapSender.SentPDUSessionResourceSetupRequest) != 1 {
		t.Fatalf("expected 1 PDUSessionResourceSetupRequest, got %d", len(ngapSender.SentPDUSessionResourceSetupRequest))
	}

	nm := decryptAndDecodeNasPdu(t, ue, ngapSender.SentPDUSessionResourceSetupRequest[0].NasPdu, 1)
	if nm.GmmHeader.GetMessageType() != nas.MsgTypeRegistrationAccept {
		t.Fatalf("expected RegistrationAccept, got %v", nm.GmmHeader.GetMessageType())
	}
}

func TestMobilityReg_UeContextRequest_True_InitialContextSetup(t *testing.T) {
	ue, ngapSender, _, amfInstance := buildMobilityRegUeAndAMF(t)
	ue.AllowedNssai = []models.Snssai{{Sst: 1, Sd: "010203"}}
	setTestUESecurityCapability(ue)

	ue.Conn().UeContextRequest = true

	HandleMobilityAndPeriodicRegistrationUpdating(context.TODO(), amfInstance, ue)

	if len(ngapSender.SentInitialContextSetupRequest) != 1 {
		t.Fatalf("expected 1 InitialContextSetupRequest, got %d", len(ngapSender.SentInitialContextSetupRequest))
	}

	nm := decryptAndDecodeNasPdu(t, ue, ngapSender.SentInitialContextSetupRequest[0].NasPdu, 0)
	if nm.GmmHeader.GetMessageType() != nas.MsgTypeRegistrationAccept {
		t.Fatalf("expected RegistrationAccept, got %v", nm.GmmHeader.GetMessageType())
	}

	if len(ngapSender.SentDownlinkNASTransport) != 0 {
		t.Fatalf("expected 0 DownlinkNASTransport, got %d", len(ngapSender.SentDownlinkNASTransport))
	}
}

func TestMobilityReg_NoUeContextRequest_NonEmptySuList(t *testing.T) {
	ue, ngapSender, fakeSmf, amfInstance := buildMobilityRegUeAndAMF(t)

	snssai := &models.Snssai{Sst: 1}
	_ = ue.CreateSmContext(2, "ref-2", snssai)

	ue.Conn().RegistrationRequest.UplinkDataStatus = &nasType.UplinkDataStatus{
		Len:    2,
		Buffer: []uint8{0x04, 0x00},
	}

	ue.Conn().UeContextRequest = false

	HandleMobilityAndPeriodicRegistrationUpdating(context.TODO(), amfInstance, ue)

	if len(fakeSmf.ActivateSmContextCalls) != 1 {
		t.Fatalf("expected 1 ActivateSmContext call, got %d", len(fakeSmf.ActivateSmContextCalls))
	}

	if len(ngapSender.SentPDUSessionResourceSetupRequest) != 1 {
		t.Fatalf("expected 1 PDUSessionResourceSetupRequest, got %d", len(ngapSender.SentPDUSessionResourceSetupRequest))
	}

	nm := decryptAndDecodeNasPdu(t, ue, ngapSender.SentPDUSessionResourceSetupRequest[0].NasPdu, 0)
	if nm.GmmHeader.GetMessageType() != nas.MsgTypeRegistrationAccept {
		t.Fatalf("expected RegistrationAccept, got %v", nm.GmmHeader.GetMessageType())
	}

	if len(ngapSender.SentInitialContextSetupRequest) != 0 {
		t.Fatalf("expected 0 InitialContextSetupRequest, got %d", len(ngapSender.SentInitialContextSetupRequest))
	}
}

func TestMobilityReg_NoUeContextRequest_EmptySuList_DownlinkNasTransport(t *testing.T) {
	ue, ngapSender, _, amfInstance := buildMobilityRegUeAndAMF(t)

	ue.Conn().UeContextRequest = false

	HandleMobilityAndPeriodicRegistrationUpdating(context.TODO(), amfInstance, ue)

	if len(ngapSender.SentDownlinkNASTransport) != 1 {
		t.Fatalf("expected 1 DownlinkNASTransport, got %d", len(ngapSender.SentDownlinkNASTransport))
	}

	if len(ngapSender.SentPDUSessionResourceSetupRequest) != 0 {
		t.Fatalf("expected 0 PDUSessionResourceSetupRequest, got %d", len(ngapSender.SentPDUSessionResourceSetupRequest))
	}

	if len(ngapSender.SentInitialContextSetupRequest) != 0 {
		t.Fatalf("expected 0 InitialContextSetupRequest, got %d", len(ngapSender.SentInitialContextSetupRequest))
	}

	resp := ngapSender.SentDownlinkNASTransport[0]
	nm := decryptAndDecodeNasPdu(t, ue, resp.NasPdu, 0)

	if nm.GmmHeader.GetMessageType() != nas.MsgTypeRegistrationAccept {
		t.Fatalf("expected RegistrationAccept, got %v", nm.GmmHeader.GetMessageType())
	}
}

// multiSliceDB returns multiple policies spanning two different slices,
// causing SubscriberProfile to return a multi-element AllowedNssai.
type multiSliceDB struct {
	Operator *db.Operator
}

func (m *multiSliceDB) GetOperator(ctx context.Context) (*db.Operator, error) {
	return m.Operator, nil
}

func (m *multiSliceDB) GetDataNetworkByID(_ context.Context, id string) (*db.DataNetwork, error) {
	return &db.DataNetwork{ID: id, Name: "TestDataNetwork"}, nil
}

func (m *multiSliceDB) GetNetworkSliceByID(_ context.Context, id string) (*db.NetworkSlice, error) {
	sd1, sd2 := "010203", "aabbcc"
	slices := map[string]*db.NetworkSlice{
		"slice-1": {ID: "slice-1", Name: "slice-a", Sst: 1, Sd: &sd1},
		"slice-2": {ID: "slice-2", Name: "slice-b", Sst: 2, Sd: &sd2},
	}

	s, ok := slices[id]
	if !ok {
		return nil, fmt.Errorf("slice %s not found", id)
	}

	return s, nil
}

func (m *multiSliceDB) ListNetworkSlicesByIDs(_ context.Context, ids []string) ([]db.NetworkSlice, error) {
	sd1, sd2 := "010203", "aabbcc"
	slices := map[string]db.NetworkSlice{
		"slice-1": {ID: "slice-1", Name: "slice-a", Sst: 1, Sd: &sd1},
		"slice-2": {ID: "slice-2", Name: "slice-b", Sst: 2, Sd: &sd2},
	}

	var out []db.NetworkSlice

	for _, id := range ids {
		if s, ok := slices[id]; ok {
			out = append(out, s)
		}
	}

	return out, nil
}

func (m *multiSliceDB) GetSubscriber(_ context.Context, imsi string) (*db.Subscriber, error) {
	return &db.Subscriber{Imsi: imsi, ProfileID: "profile-1"}, nil
}

func (m *multiSliceDB) GetProfileByID(_ context.Context, id string) (*db.Profile, error) {
	return &db.Profile{ID: id, Name: "TestProfile"}, nil
}

func (m *multiSliceDB) ListAllNetworkSlices(_ context.Context) ([]db.NetworkSlice, error) {
	sd1, sd2 := "010203", "aabbcc"

	return []db.NetworkSlice{
		{ID: "slice-1", Name: "slice-a", Sst: 1, Sd: &sd1},
		{ID: "slice-2", Name: "slice-b", Sst: 2, Sd: &sd2},
	}, nil
}

func (m *multiSliceDB) GetPolicyByProfileAndSlice(_ context.Context, profileID, sliceID string) (*db.Policy, error) {
	return &db.Policy{ID: sliceID, Name: "TestPolicy", ProfileID: profileID, SliceID: sliceID, DataNetworkID: "dn-1"}, nil
}

func (m *multiSliceDB) ListPoliciesByProfile(_ context.Context, _ string) ([]db.Policy, error) {
	return []db.Policy{
		{ID: "policy-1", Name: "Policy-A", ProfileID: "profile-1", SliceID: "slice-1", DataNetworkID: "dn-1"},
		{ID: "policy-2", Name: "Policy-B", ProfileID: "profile-1", SliceID: "slice-2", DataNetworkID: "dn-2"},
	}, nil
}

func (m *multiSliceDB) NodeID() int { return 0 }

func TestMobilityReg_MultiSlice_AllowedNssaiContainsAllSlices(t *testing.T) {
	supi := mustSUPIFromPrefixed("imsi-001019756139935")
	fakeSmf := &fakeSmf{}
	dbInstance := &multiSliceDB{
		Operator: &db.Operator{
			Mcc:           "001",
			Mnc:           "01",
			SupportedTACs: "[\"000001\"]",
		},
	}
	amfInstance := amf.New(dbInstance, nil, fakeSmf)

	ue, ngapSender, err := buildUeAndRadio()
	if err != nil {
		t.Fatalf("could not create UE and radio: %v", err)
	}

	ue.SetSupiForTest(supi)
	ue.Imei, _ = etsi.NewIMEIFromPEI("imei-490154203237518")
	ue.Tai = ue.Conn().Tai
	ue.SetSecuredForTest(true)
	{
		ng := ue.NgKsiForTest()
		ng.Ksi = 1
		ue.SetNgKsiForTest(ng)
	}

	key := [16]uint8{0x0D, 0x0E, 0x0A, 0x0D, 0x0B, 0x0E, 0x0E, 0x0F, 0x0F, 0x0E, 0x0E, 0x0D, 0x0C, 0x0A, 0x0F, 0x0E}
	algo := security.AlgCiphering128NEA2

	ue.SetKnasEncForTest(key)
	ue.SetKnasIntForTest(key)
	ue.SetCipheringAlgForTest(algo)
	ue.SetIntegrityAlgForTest(security.AlgIntegrity128NIA0)

	registrationRequest, err := buildTestRegistrationRequestMessage(algo, &key, ue.ULCountForTest().Value())
	if err != nil {
		t.Fatalf("could not build registration request message: %v", err)
	}

	ue.Conn().RegistrationRequest = registrationRequest.RegistrationRequest
	ue.Conn().RegistrationType5GS = nasMessage.RegistrationType5GSMobilityRegistrationUpdating
	ue.Conn().RegistrationRequest.Capability5GMM = &nasType.Capability5GMM{}

	HandleMobilityAndPeriodicRegistrationUpdating(context.TODO(), amfInstance, ue)

	if len(ue.AllowedNssai) != 2 {
		t.Fatalf("expected 2 allowed NSSAIs, got %d", len(ue.AllowedNssai))
	}

	if ue.AllowedNssai[0].Sst != 1 || ue.AllowedNssai[0].Sd != "010203" {
		t.Fatalf("expected first slice SST=1 SD=010203, got SST=%d SD=%s", ue.AllowedNssai[0].Sst, ue.AllowedNssai[0].Sd)
	}

	if ue.AllowedNssai[1].Sst != 2 || ue.AllowedNssai[1].Sd != "aabbcc" {
		t.Fatalf("expected second slice SST=2 SD=aabbcc, got SST=%d SD=%s", ue.AllowedNssai[1].Sst, ue.AllowedNssai[1].Sd)
	}

	if len(ngapSender.SentDownlinkNASTransport) != 1 {
		t.Fatalf("expected 1 DownlinkNASTransport, got %d", len(ngapSender.SentDownlinkNASTransport))
	}

	nm := decryptAndDecodeNasPdu(t, ue, ngapSender.SentDownlinkNASTransport[0].NasPdu, 0)
	if nm.GmmHeader.GetMessageType() != nas.MsgTypeRegistrationAccept {
		t.Fatalf("expected RegistrationAccept, got %v", nm.GmmHeader.GetMessageType())
	}

	regAccept := nm.RegistrationAccept
	if regAccept.AllowedNSSAI == nil {
		t.Fatal("expected AllowedNSSAI in RegistrationAccept, got nil")
	}

	// 2 S-NSSAIs with SD: each is 5 bytes (1 len + 1 SST + 3 SD) = 10 bytes total
	if regAccept.AllowedNSSAI.GetLen() != 10 {
		t.Fatalf("expected AllowedNSSAI length 10, got %d", regAccept.AllowedNSSAI.GetLen())
	}
}
