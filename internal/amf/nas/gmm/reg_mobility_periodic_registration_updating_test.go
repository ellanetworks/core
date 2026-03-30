// Copyright 2026 Ella Networks

package gmm

import (
	"context"
	"fmt"
	"testing"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/models"
	"github.com/free5gc/nas"
	"github.com/free5gc/nas/nasMessage"
	"github.com/free5gc/nas/nasType"
	"github.com/free5gc/nas/security"
)

// failingSubscriberDB is a FakeDBInstance variant that returns an error for GetSubscriber.
type failingSubscriberDB struct {
	Operator *db.Operator
}

func (fdb *failingSubscriberDB) GetOperator(ctx context.Context) (*db.Operator, error) {
	if fdb.Operator == nil {
		return nil, fmt.Errorf("could not get operator")
	}

	return fdb.Operator, nil
}

func (fdb *failingSubscriberDB) GetDataNetworkByID(ctx context.Context, id int) (*db.DataNetwork, error) {
	return &db.DataNetwork{ID: id, Name: "TestDataNetwork"}, nil
}

func (fdb *failingSubscriberDB) GetSubscriber(ctx context.Context, imsi string) (*db.Subscriber, error) {
	return nil, fmt.Errorf("subscriber not found")
}

func (fdb *failingSubscriberDB) ListNetworkSlices(ctx context.Context) ([]db.NetworkSlice, error) {
	return []db.NetworkSlice{
		{ID: 1, Sst: 1, Name: "default"},
	}, nil
}

func (fdb *failingSubscriberDB) ListProfileNetworkConfigs(ctx context.Context, profileID int) ([]db.ProfileNetworkConfig, error) {
	return nil, fmt.Errorf("subscriber not found")
}

// decryptAndDecodeNasPdu decrypts a ciphered NAS PDU using the UE's security context
// and decodes it, returning the NAS message. It verifies the security header is
// IntegrityProtectedAndCiphered. The dlCountOffset parameter specifies the offset
// from ue.ULCount.Get() to use as the DL count (0 for the first message, 1 for
// the second, etc.).
func decryptAndDecodeNasPdu(t *testing.T, ue *amf.AmfUe, nasPdu []byte, dlCountOffset uint32) *nas.Message {
	t.Helper()

	nm := new(nas.Message)
	nm.SecurityHeaderType = nas.GetSecurityHeaderType(nasPdu) & 0x0f

	if nm.SecurityHeaderType != nas.SecurityHeaderTypeIntegrityProtectedAndCiphered {
		t.Fatalf("expected IntegrityProtectedAndCiphered, got security header type %d", nm.SecurityHeaderType)
	}

	payload := make([]byte, len(nasPdu))
	copy(payload, nasPdu)
	payload = payload[7:]

	if err := security.NASEncrypt(ue.CipheringAlg, ue.KnasEnc, ue.ULCount.Get()+dlCountOffset, security.Bearer3GPP, security.DirectionDownlink, payload); err != nil {
		t.Fatalf("could not decrypt NAS message: %v", err)
	}

	if err := nm.PlainNasDecode(&payload); err != nil {
		t.Fatalf("could not decode NAS message: %v", err)
	}

	return nm
}

// buildMobilityRegUeAndAMF creates a UE and AMF configured for mobility/periodic
// registration updating tests. The UE has security context, a valid registration
// request, Pei, Supi, and matching Tai. The AMF has a valid Operator, FakeSmf, and
// UEs map. Returns the UE, ngapSender, fakeSmf, and AMF.
func buildMobilityRegUeAndAMF(t *testing.T) (*amf.AmfUe, *FakeNGAPSender, *FakeSmf, *amf.AMF) {
	t.Helper()

	supi := mustSUPIFromPrefixed("imsi-001019756139935")
	fakeSmf := &FakeSmf{}
	amfInstance := amf.New(
		&FakeDBInstance{
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

	ue.Supi = supi
	ue.Pei = "imei-490154203237518"
	ue.Tai = ue.RanUe().Tai
	ue.SecurityContextAvailable = true
	ue.NgKsi.Ksi = 1
	key := [16]uint8{0x0D, 0x0E, 0x0A, 0x0D, 0x0B, 0x0E, 0x0E, 0x0F, 0x0F, 0x0E, 0x0E, 0x0D, 0x0C, 0x0A, 0x0F, 0x0E}
	algo := security.AlgCiphering128NEA2
	ue.KnasEnc = key
	ue.KnasInt = key
	ue.CipheringAlg = algo
	ue.IntegrityAlg = security.AlgIntegrity128NIA0

	registrationRequest, err := buildTestRegistrationRequestMessage(algo, &key, ue.ULCount.Get())
	if err != nil {
		t.Fatalf("could not build registration request message: %v", err)
	}

	ue.RegistrationRequest = registrationRequest.RegistrationRequest
	ue.RegistrationType5GS = nasMessage.RegistrationType5GSMobilityRegistrationUpdating
	ue.RegistrationRequest.Capability5GMM = &nasType.Capability5GMM{}

	return ue, ngapSender, fakeSmf, amfInstance
}

func TestMobilityReg_DerivateAnKeyError(t *testing.T) {
	ue, _, _, amfInstance := buildMobilityRegUeAndAMF(t)

	ue.Kamf = "not-valid-hex"

	err := HandleMobilityAndPeriodicRegistrationUpdating(context.TODO(), amfInstance, ue)
	if err == nil {
		t.Fatal("expected error for invalid Kamf, got nil")
	}

	if got := err.Error(); got != "error deriving AnKey: could not decode kamf: encoding/hex: invalid byte 0x6e ('n') in decoding string of length 13" {
		// Just check prefix to be resilient
		if len(got) < 20 || got[:20] != "error deriving AnKey" {
			t.Fatalf("unexpected error: %v", err)
		}
	}
}

func TestMobilityReg_GetOperatorInfoError(t *testing.T) {
	ue, _, _, amfInstance := buildMobilityRegUeAndAMF(t)

	amfInstance.DBInstance = &FakeDBInstance{Operator: nil}

	err := HandleMobilityAndPeriodicRegistrationUpdating(context.TODO(), amfInstance, ue)
	if err == nil {
		t.Fatal("expected error for nil Operator, got nil")
	}

	expected := "error getting operator info: failed to get operator: could not get operator"
	if err.Error() != expected {
		t.Fatalf("expected error: %s, got: %v", expected, err)
	}
}

func TestMobilityReg_NilCapability5GMM_Mobility_SendsReject(t *testing.T) {
	ue, ngapSender, _, amfInstance := buildMobilityRegUeAndAMF(t)

	ue.RegistrationRequest.Capability5GMM = nil
	ue.RegistrationType5GS = nasMessage.RegistrationType5GSMobilityRegistrationUpdating

	err := HandleMobilityAndPeriodicRegistrationUpdating(context.TODO(), amfInstance, ue)
	if err == nil {
		t.Fatal("expected error for nil Capability5GMM, got nil")
	}

	if err.Error() != "Capability5GMM is nil" {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(ngapSender.SentDownlinkNASTransport) != 1 {
		t.Fatalf("expected 1 DownlinkNASTransport (RegistrationReject), got %d", len(ngapSender.SentDownlinkNASTransport))
	}

	resp := ngapSender.SentDownlinkNASTransport[0]
	nm := new(nas.Message)
	nm.SecurityHeaderType = nas.GetSecurityHeaderType(resp.NasPdu) & 0x0f

	if nm.SecurityHeaderType != nas.SecurityHeaderTypePlainNas {
		t.Fatalf("expected plain NAS, got security header type %d", nm.SecurityHeaderType)
	}

	err = nm.PlainNasDecode(&resp.NasPdu)
	if err != nil {
		t.Fatalf("could not decode NAS message: %v", err)
	}

	if nm.GmmHeader.GetMessageType() != nas.MsgTypeRegistrationReject {
		t.Fatalf("expected RegistrationReject, got %v", nm.GmmHeader.GetMessageType())
	}
}

func TestMobilityReg_NilCapability5GMM_Periodic_Continues(t *testing.T) {
	ue, ngapSender, _, amfInstance := buildMobilityRegUeAndAMF(t)

	ue.RegistrationRequest.Capability5GMM = nil
	ue.RegistrationType5GS = nasMessage.RegistrationType5GSPeriodicRegistrationUpdating

	err := HandleMobilityAndPeriodicRegistrationUpdating(context.TODO(), amfInstance, ue)
	if err != nil {
		t.Fatalf("expected no error for periodic reg with nil Capability5GMM, got: %v", err)
	}

	// Should send DownlinkNasTransport with RegistrationAccept (happy path, no UeContextRequest)
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

	ue.UeRadioCapability = "some-capability"
	ue.UeRadioCapabilityForPaging = &models.UERadioCapabilityForPaging{}

	updateType := nasType.NewUpdateType5GS(nasMessage.RegistrationRequestUpdateType5GSType)
	updateType.SetNGRanRcu(nasMessage.NGRanRadioCapabilityUpdateNeeded)
	ue.RegistrationRequest.UpdateType5GS = updateType

	err := HandleMobilityAndPeriodicRegistrationUpdating(context.TODO(), amfInstance, ue)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if ue.UeRadioCapability != "" {
		t.Fatalf("expected UeRadioCapability to be cleared, got %q", ue.UeRadioCapability)
	}

	if ue.UeRadioCapabilityForPaging != nil {
		t.Fatalf("expected UeRadioCapabilityForPaging to be nil")
	}

	// Should send DownlinkNasTransport with RegistrationAccept
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

	ue.RegistrationRequest.MICOIndication = nasType.NewMICOIndication(nasMessage.RegistrationRequestMICOIndicationType)

	err := HandleMobilityAndPeriodicRegistrationUpdating(context.TODO(), amfInstance, ue)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should send DownlinkNasTransport with RegistrationAccept
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
	drxParams.SetDRXValue(0x03) // some DRX value
	ue.RegistrationRequest.RequestedDRXParameters = drxParams

	err := HandleMobilityAndPeriodicRegistrationUpdating(context.TODO(), amfInstance, ue)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if ue.UESpecificDRX != 0x03 {
		t.Fatalf("expected UESpecificDRX to be 0x03, got 0x%02x", ue.UESpecificDRX)
	}

	// Should send DownlinkNasTransport with RegistrationAccept
	if len(ngapSender.SentDownlinkNASTransport) != 1 {
		t.Fatalf("expected 1 DownlinkNASTransport, got %d", len(ngapSender.SentDownlinkNASTransport))
	}

	nm := decryptAndDecodeNasPdu(t, ue, ngapSender.SentDownlinkNASTransport[0].NasPdu, 0)
	if nm.GmmHeader.GetMessageType() != nas.MsgTypeRegistrationAccept {
		t.Fatalf("expected RegistrationAccept, got %v", nm.GmmHeader.GetMessageType())
	}
}

func TestMobilityReg_EmptyPei_SendsIdentityRequest(t *testing.T) {
	ue, ngapSender, _, amfInstance := buildMobilityRegUeAndAMF(t)

	ue.Pei = ""

	err := HandleMobilityAndPeriodicRegistrationUpdating(context.TODO(), amfInstance, ue)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if len(ngapSender.SentDownlinkNASTransport) != 1 {
		t.Fatalf("expected 1 DownlinkNASTransport (IdentityRequest), got %d", len(ngapSender.SentDownlinkNASTransport))
	}

	resp := ngapSender.SentDownlinkNASTransport[0]
	nm := new(nas.Message)
	nm.SecurityHeaderType = nas.GetSecurityHeaderType(resp.NasPdu) & 0x0f

	if nm.SecurityHeaderType != nas.SecurityHeaderTypePlainNas {
		t.Fatalf("expected plain NAS, got security header type %d", nm.SecurityHeaderType)
	}

	err = nm.PlainNasDecode(&resp.NasPdu)
	if err != nil {
		t.Fatalf("could not decode NAS message: %v", err)
	}

	if nm.GmmHeader.GetMessageType() != nas.MsgTypeIdentityRequest {
		t.Fatalf("expected IdentityRequest, got %v", nm.GmmHeader.GetMessageType())
	}
}

func TestMobilityReg_GetSubscriberBitrateError(t *testing.T) {
	ue, _, _, amfInstance := buildMobilityRegUeAndAMF(t)

	// Override DBInstance with one that returns an error for GetSubscriber
	amfInstance.DBInstance = &failingSubscriberDB{
		Operator: &db.Operator{
			Mcc:           "001",
			Mnc:           "01",
			SupportedTACs: "[\"000001\"]",
		},
	}

	err := HandleMobilityAndPeriodicRegistrationUpdating(context.TODO(), amfInstance, ue)
	if err == nil {
		t.Fatal("expected error for GetSubscriberBitrate failure, got nil")
	}

	if got := err.Error(); len(got) < 30 || got[:30] != "failed to get subscriber data:" {
		t.Fatalf("unexpected error prefix: %v", err)
	}
}

func TestMobilityReg_UplinkDataStatus_ActivateSuccess_UeContextRequest(t *testing.T) {
	ue, ngapSender, fakeSmf, amfInstance := buildMobilityRegUeAndAMF(t)

	// Set up PDU session 2
	snssai := &models.Snssai{Sst: 1}

	_ = ue.CreateSmContext(2, "ref-2", snssai)

	// UplinkDataStatus: PSI 2 has uplink data (bit 2 in byte 0 = 0x04)
	ue.RegistrationRequest.UplinkDataStatus = &nasType.UplinkDataStatus{
		Len:    2,
		Buffer: []uint8{0x04, 0x00},
	}

	ue.RanUe().UeContextRequest = true

	err := HandleMobilityAndPeriodicRegistrationUpdating(context.TODO(), amfInstance, ue)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(fakeSmf.ActivateSmContextCalls) != 1 {
		t.Fatalf("expected 1 ActivateSmContext call, got %d", len(fakeSmf.ActivateSmContextCalls))
	}

	if fakeSmf.ActivateSmContextCalls[0].SmContextRef != "ref-2" {
		t.Fatalf("expected SmContextRef 'ref-2', got %q", fakeSmf.ActivateSmContextCalls[0].SmContextRef)
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

	ue.RegistrationRequest.UplinkDataStatus = &nasType.UplinkDataStatus{
		Len:    2,
		Buffer: []uint8{0x04, 0x00},
	}

	ue.RanUe().UeContextRequest = false

	err := HandleMobilityAndPeriodicRegistrationUpdating(context.TODO(), amfInstance, ue)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

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

	ue.RegistrationRequest.UplinkDataStatus = &nasType.UplinkDataStatus{
		Len:    2,
		Buffer: []uint8{0x04, 0x00},
	}

	fakeSmf.ActivateSmContextError = fmt.Errorf("activate error")

	err := HandleMobilityAndPeriodicRegistrationUpdating(context.TODO(), amfInstance, ue)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

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
	ue.RegistrationRequest.PDUSessionStatus = &nasType.PDUSessionStatus{
		Len:    2,
		Buffer: []uint8{0x00, 0x00},
	}

	err := HandleMobilityAndPeriodicRegistrationUpdating(context.TODO(), amfInstance, ue)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(fakeSmf.ReleaseSmContextCalls) != 1 {
		t.Fatalf("expected 1 ReleaseSmContext call, got %d", len(fakeSmf.ReleaseSmContextCalls))
	}

	if fakeSmf.ReleaseSmContextCalls[0].SmContextRef != "ref-2" {
		t.Fatalf("expected SmContextRef 'ref-2', got %q", fakeSmf.ReleaseSmContextCalls[0].SmContextRef)
	}

	if len(fakeSmf.ReleasedSmContext) != 1 || fakeSmf.ReleasedSmContext[0] != "ref-2" {
		t.Fatalf("expected ReleasedSmContext to contain 'ref-2', got %v", fakeSmf.ReleasedSmContext)
	}

	// Should send DownlinkNasTransport with RegistrationAccept
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
	ue.RegistrationRequest.PDUSessionStatus = &nasType.PDUSessionStatus{
		Len:    2,
		Buffer: []uint8{0x04, 0x00},
	}

	err := HandleMobilityAndPeriodicRegistrationUpdating(context.TODO(), amfInstance, ue)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(fakeSmf.ReleaseSmContextCalls) != 0 {
		t.Fatalf("expected 0 ReleaseSmContext calls, got %d", len(fakeSmf.ReleaseSmContextCalls))
	}

	// Should send DownlinkNasTransport with RegistrationAccept
	if len(ngapSender.SentDownlinkNASTransport) != 1 {
		t.Fatalf("expected 1 DownlinkNASTransport, got %d", len(ngapSender.SentDownlinkNASTransport))
	}

	nm := decryptAndDecodeNasPdu(t, ue, ngapSender.SentDownlinkNASTransport[0].NasPdu, 0)
	if nm.GmmHeader.GetMessageType() != nas.MsgTypeRegistrationAccept {
		t.Fatalf("expected RegistrationAccept, got %v", nm.GmmHeader.GetMessageType())
	}
}

func TestMobilityReg_PDUSessionStatus_ReleaseError(t *testing.T) {
	ue, _, fakeSmf, amfInstance := buildMobilityRegUeAndAMF(t)

	snssai := &models.Snssai{Sst: 1}
	_ = ue.CreateSmContext(2, "ref-2", snssai)

	ue.RegistrationRequest.PDUSessionStatus = &nasType.PDUSessionStatus{
		Len:    2,
		Buffer: []uint8{0x00, 0x00}, // PSI 2 inactive → triggers release
	}

	fakeSmf.ReleaseSmContextError = fmt.Errorf("release error")

	err := HandleMobilityAndPeriodicRegistrationUpdating(context.TODO(), amfInstance, ue)
	if err == nil {
		t.Fatal("expected error for ReleaseSmContext failure, got nil")
	}

	expected := "failed to release sm context: release error"
	if err.Error() != expected {
		t.Fatalf("expected error: %s, got: %v", expected, err)
	}
}

func TestMobilityReg_AllowedPDUSessionStatus_N1N2_NilN2Info_NonEmptySuList(t *testing.T) {
	ue, ngapSender, fakeSmf, amfInstance := buildMobilityRegUeAndAMF(t)

	snssai := &models.Snssai{Sst: 1}
	_ = ue.CreateSmContext(2, "ref-2", snssai)

	// UplinkDataStatus with PSI 2 + no UeContextRequest → populates suList
	ue.RegistrationRequest.UplinkDataStatus = &nasType.UplinkDataStatus{
		Len:    2,
		Buffer: []uint8{0x04, 0x00},
	}
	ue.RanUe().UeContextRequest = false

	// AllowedPDUSessionStatus + N1N2Message with nil N2Info
	ue.RegistrationRequest.AllowedPDUSessionStatus = &nasType.AllowedPDUSessionStatus{
		Len:    2,
		Buffer: []uint8{0x04, 0x00},
	}
	ue.N1N2Message = &models.N1N2MessageTransferRequest{
		PduSessionID:            3,
		BinaryDataN1Message:     []byte{0x01, 0x02},
		BinaryDataN2Information: nil,
	}

	err := HandleMobilityAndPeriodicRegistrationUpdating(context.TODO(), amfInstance, ue)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

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

	// PDUSessionResourceSetupRequest carries RegistrationAccept
	nmSetup := decryptAndDecodeNasPdu(t, ue, ngapSender.SentPDUSessionResourceSetupRequest[0].NasPdu, 0)
	if nmSetup.GmmHeader.GetMessageType() != nas.MsgTypeRegistrationAccept {
		t.Fatalf("expected RegistrationAccept in PDUSessionResourceSetupRequest, got %v", nmSetup.GmmHeader.GetMessageType())
	}

	// DLNASTransport carries DLNASTransport (N1 message)
	nmDL := decryptAndDecodeNasPdu(t, ue, ngapSender.SentDownlinkNASTransport[0].NasPdu, 1)
	if nmDL.GmmHeader.GetMessageType() != nas.MsgTypeDLNASTransport {
		t.Fatalf("expected DLNASTransport, got %v", nmDL.GmmHeader.GetMessageType())
	}

	// N1N2Message should be cleared
	if ue.N1N2Message != nil {
		t.Fatal("expected N1N2Message to be nil after processing")
	}
}

func TestMobilityReg_AllowedPDUSessionStatus_N1N2_NilN2Info_EmptySuList(t *testing.T) {
	ue, ngapSender, _, amfInstance := buildMobilityRegUeAndAMF(t)

	// No UplinkDataStatus → suList remains empty

	ue.RegistrationRequest.AllowedPDUSessionStatus = &nasType.AllowedPDUSessionStatus{
		Len:    2,
		Buffer: []uint8{0x04, 0x00},
	}
	ue.N1N2Message = &models.N1N2MessageTransferRequest{
		PduSessionID:            3,
		BinaryDataN1Message:     []byte{0x01, 0x02},
		BinaryDataN2Information: nil,
	}

	// UeContextRequest=false so SendRegistrationAccept sends DownlinkNasTransport
	ue.RanUe().UeContextRequest = false

	err := HandleMobilityAndPeriodicRegistrationUpdating(context.TODO(), amfInstance, ue)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Empty suList → calls SendRegistrationAccept (which sends DLNASTransport since UeContextRequest=false)
	// Then also sends DLNASTransport for N1 message
	if len(ngapSender.SentDownlinkNASTransport) != 2 {
		t.Fatalf("expected 2 DownlinkNASTransport (RegistrationAccept + N1 DLNASTransport), got %d", len(ngapSender.SentDownlinkNASTransport))
	}

	// First DLNASTransport is RegistrationAccept
	nmAccept := decryptAndDecodeNasPdu(t, ue, ngapSender.SentDownlinkNASTransport[0].NasPdu, 0)
	if nmAccept.GmmHeader.GetMessageType() != nas.MsgTypeRegistrationAccept {
		t.Fatalf("expected RegistrationAccept in first DLNASTransport, got %v", nmAccept.GmmHeader.GetMessageType())
	}

	// Second DLNASTransport is DLNASTransport (N1 message)
	nmN1 := decryptAndDecodeNasPdu(t, ue, ngapSender.SentDownlinkNASTransport[1].NasPdu, 1)
	if nmN1.GmmHeader.GetMessageType() != nas.MsgTypeDLNASTransport {
		t.Fatalf("expected DLNASTransport in second DLNASTransport, got %v", nmN1.GmmHeader.GetMessageType())
	}

	if ue.N1N2Message != nil {
		t.Fatal("expected N1N2Message to be nil after processing")
	}
}

func TestMobilityReg_AllowedPDUSessionStatus_N1N2_WithN2Info_MissingSmContext(t *testing.T) {
	ue, _, _, amfInstance := buildMobilityRegUeAndAMF(t)

	ue.RegistrationRequest.AllowedPDUSessionStatus = &nasType.AllowedPDUSessionStatus{
		Len:    2,
		Buffer: []uint8{0x04, 0x00},
	}

	// N1N2 with N2Info, but no SmContext for PduSessionID 3
	ue.N1N2Message = &models.N1N2MessageTransferRequest{
		PduSessionID:            3,
		BinaryDataN1Message:     []byte{0x01, 0x02},
		BinaryDataN2Information: []byte{0x03, 0x04},
	}

	err := HandleMobilityAndPeriodicRegistrationUpdating(context.TODO(), amfInstance, ue)
	if err == nil {
		t.Fatal("expected error for missing SmContext, got nil")
	}

	expected := "pdu Session Id does not Exists"
	if err.Error() != expected {
		t.Fatalf("expected error: %s, got: %v", expected, err)
	}

	// N1N2Message should be cleared
	if ue.N1N2Message != nil {
		t.Fatal("expected N1N2Message to be nil after error")
	}
}

func TestMobilityReg_AllowedPDUSessionStatus_N1N2_WithN2Info_SmContextExists(t *testing.T) {
	ue, ngapSender, _, amfInstance := buildMobilityRegUeAndAMF(t)

	snssai := &models.Snssai{Sst: 1}
	_ = ue.CreateSmContext(3, "ref-3", snssai)

	ue.RegistrationRequest.AllowedPDUSessionStatus = &nasType.AllowedPDUSessionStatus{
		Len:    2,
		Buffer: []uint8{0x08, 0x00}, // PSI 3
	}

	ue.N1N2Message = &models.N1N2MessageTransferRequest{
		PduSessionID:            3,
		SNssai:                  snssai,
		BinaryDataN1Message:     []byte{0x01, 0x02},
		BinaryDataN2Information: []byte{0x03, 0x04},
	}

	ue.RanUe().UeContextRequest = false

	err := HandleMobilityAndPeriodicRegistrationUpdating(context.TODO(), amfInstance, ue)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// The N2Info path appends to suList, then falls through to the final block
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

	ue.RanUe().UeContextRequest = true

	err := HandleMobilityAndPeriodicRegistrationUpdating(context.TODO(), amfInstance, ue)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

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

	// UplinkDataStatus with PSI 2
	ue.RegistrationRequest.UplinkDataStatus = &nasType.UplinkDataStatus{
		Len:    2,
		Buffer: []uint8{0x04, 0x00},
	}

	ue.RanUe().UeContextRequest = false

	err := HandleMobilityAndPeriodicRegistrationUpdating(context.TODO(), amfInstance, ue)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

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

	ue.RanUe().UeContextRequest = false

	err := HandleMobilityAndPeriodicRegistrationUpdating(context.TODO(), amfInstance, ue)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(ngapSender.SentDownlinkNASTransport) != 1 {
		t.Fatalf("expected 1 DownlinkNASTransport, got %d", len(ngapSender.SentDownlinkNASTransport))
	}

	if len(ngapSender.SentPDUSessionResourceSetupRequest) != 0 {
		t.Fatalf("expected 0 PDUSessionResourceSetupRequest, got %d", len(ngapSender.SentPDUSessionResourceSetupRequest))
	}

	if len(ngapSender.SentInitialContextSetupRequest) != 0 {
		t.Fatalf("expected 0 InitialContextSetupRequest, got %d", len(ngapSender.SentInitialContextSetupRequest))
	}

	// Decode the NAS message to verify it's a RegistrationAccept
	resp := ngapSender.SentDownlinkNASTransport[0]
	nm := decryptAndDecodeNasPdu(t, ue, resp.NasPdu, 0)

	if nm.GmmHeader.GetMessageType() != nas.MsgTypeRegistrationAccept {
		t.Fatalf("expected RegistrationAccept, got %v", nm.GmmHeader.GetMessageType())
	}
}
