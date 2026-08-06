// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"context"
	"fmt"
	"reflect"
	"testing"

	"github.com/ellanetworks/core/etsi"
	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/nas"
	"github.com/ellanetworks/core/nas/fgs"
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
	return &db.Profile{ID: id, Name: "TestProfile", Allow4G: true, Allow5G: true, UeAmbrDownlink: "200 Mbps", UeAmbrUplink: "100 Mbps"}, nil
}

func (fdb *failingSubscriberDB) ListAllNetworkSlices(ctx context.Context) ([]db.NetworkSlice, error) {
	return []db.NetworkSlice{{ID: "slice-1", Sst: 1, Name: "default"}}, nil
}

func (fdb *failingSubscriberDB) GetPolicyByProfileAndSlice(ctx context.Context, profileID, sliceID string) (*db.Policy, error) {
	return &db.Policy{ID: "policy-1", Name: "TestPolicy", ProfileID: profileID, SliceID: sliceID, DataNetworkID: "dn-1", SessionAmbrDownlink: "200 Mbps", SessionAmbrUplink: "100 Mbps"}, nil
}

func (fdb *failingSubscriberDB) ListPoliciesByProfile(_ context.Context, _ string) ([]db.Policy, error) {
	return []db.Policy{{ID: "policy-1", Name: "TestPolicy", ProfileID: "profile-1", SliceID: "slice-1", DataNetworkID: "dn-1"}}, nil
}

func (fdb *failingSubscriberDB) NodeID() int { return 0 }

// decryptAndDecodeNasPdu decrypts a ciphered NAS PDU using the UE's security
// context and returns the plaintext 5GMM message. It verifies the security header
// is IntegrityProtectedAndCiphered. The dlCountOffset parameter specifies the
// offset from ue.ULCount() to use as the DL count (0 for the first message, 1 for
// the second, etc.).
func decryptAndDecodeNasPdu(t *testing.T, ue *amf.UeContext, nasPdu []byte, dlCountOffset uint32) []byte {
	t.Helper()

	if len(nasPdu) < 7 || fgs.SecurityHeaderType(nasPdu[1]&0x0f) != fgs.SHTIntegrityProtectedCiphered {
		t.Fatalf("expected IntegrityProtectedAndCiphered, got security header type %d", nasPdu[1]&0x0f)
	}

	sc := mustSecurityContext(t, ue.IntegrityAlgForTest(),
		ue.CipheringAlgForTest(), ue.KnasIntForTest(), ue.KnasEncForTest())

	plain, err := sc.Cipher(append([]byte(nil), nasPdu[7:]...), nas.Count(ue.ULCount()+dlCountOffset),
		nas.Bearer3GPP, nas.DirectionDownlink)
	if err != nil {
		t.Fatalf("could not decrypt NAS message: %v", err)
	}

	return plain
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
	algo := nas.CipheringAES

	ue.SetKnasEncForTest(key)
	ue.SetKnasIntForTest(key)
	ue.SetCipheringAlgForTest(algo)
	ue.SetIntegrityAlgForTest(nas.IntegrityNull)

	ue.Conn().RegistrationRequest = &fgs.RegistrationRequest{}
	ue.Conn().RegistrationType5GS = fgs.RegistrationTypeMobilityUpdating
	ue.Conn().RegistrationRequest.GMMCapability = &fgs.GMMCapability{}

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
func TestMobilityReg_NilGMMCapability_Mobility_Continues(t *testing.T) {
	ue, ngapSender, _, amfInstance := buildMobilityRegUeAndAMF(t)

	ue.Conn().RegistrationRequest.GMMCapability = nil
	ue.Conn().RegistrationType5GS = fgs.RegistrationTypeMobilityUpdating

	HandleMobilityAndPeriodicRegistrationUpdating(context.TODO(), amfInstance, ue)

	if len(ngapSender.SentDownlinkNASTransport) != 1 {
		t.Fatalf("expected 1 DownlinkNASTransport, got %d", len(ngapSender.SentDownlinkNASTransport))
	}

	nm := decryptAndDecodeNasPdu(t, ue, ngapSender.SentDownlinkNASTransport[0].NASPDU, 0)
	if nm[2] != uint8(fgs.MsgRegistrationAccept) {
		t.Fatalf("expected RegistrationAccept, got %v", nm[2])
	}
}

func TestMobilityReg_NilGMMCapability_Periodic_Continues(t *testing.T) {
	ue, ngapSender, _, amfInstance := buildMobilityRegUeAndAMF(t)

	ue.Conn().RegistrationRequest.GMMCapability = nil
	ue.Conn().RegistrationType5GS = fgs.RegistrationTypePeriodicUpdating

	HandleMobilityAndPeriodicRegistrationUpdating(context.TODO(), amfInstance, ue)

	if len(ngapSender.SentDownlinkNASTransport) != 1 {
		t.Fatalf("expected 1 DownlinkNASTransport, got %d", len(ngapSender.SentDownlinkNASTransport))
	}

	nm := decryptAndDecodeNasPdu(t, ue, ngapSender.SentDownlinkNASTransport[0].NASPDU, 0)
	if nm[2] != uint8(fgs.MsgRegistrationAccept) {
		t.Fatalf("expected RegistrationAccept, got %v", nm[2])
	}
}

func TestMobilityReg_UpdateType5GS_ClearsRadioCapability(t *testing.T) {
	ue, ngapSender, _, amfInstance := buildMobilityRegUeAndAMF(t)

	ue.RadioCapability = []byte("some-capability")
	ue.RadioCapabilityForPaging = &models.UERadioCapabilityForPaging{}

	ue.Conn().RegistrationRequest.UpdateType5GS = &fgs.UpdateType5GS{NGRANRCU: true}

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

	nm := decryptAndDecodeNasPdu(t, ue, ngapSender.SentDownlinkNASTransport[0].NASPDU, 0)
	if nm[2] != uint8(fgs.MsgRegistrationAccept) {
		t.Fatalf("expected RegistrationAccept, got %v", nm[2])
	}
}

func TestMobilityReg_MICOIndication(t *testing.T) {
	ue, ngapSender, _, amfInstance := buildMobilityRegUeAndAMF(t)

	ue.Conn().RegistrationRequest.MICOIndication = &fgs.MICOIndication{}

	HandleMobilityAndPeriodicRegistrationUpdating(context.TODO(), amfInstance, ue)

	if len(ngapSender.SentDownlinkNASTransport) != 1 {
		t.Fatalf("expected 1 DownlinkNASTransport, got %d", len(ngapSender.SentDownlinkNASTransport))
	}

	nm := decryptAndDecodeNasPdu(t, ue, ngapSender.SentDownlinkNASTransport[0].NASPDU, 0)
	if nm[2] != uint8(fgs.MsgRegistrationAccept) {
		t.Fatalf("expected RegistrationAccept, got %v", nm[2])
	}
}

func TestMobilityReg_RequestedDRXParameters(t *testing.T) {
	ue, ngapSender, _, amfInstance := buildMobilityRegUeAndAMF(t)

	ue.Conn().RegistrationRequest.RequestedDRXParameters = &fgs.DRXParameter{Value: fgs.DRXCycleParameterT128}

	HandleMobilityAndPeriodicRegistrationUpdating(context.TODO(), amfInstance, ue)

	if ue.DRXParameter != 0x03 {
		t.Fatalf("expected DRXParameter to be 0x03, got 0x%02x", ue.DRXParameter)
	}

	if len(ngapSender.SentDownlinkNASTransport) != 1 {
		t.Fatalf("expected 1 DownlinkNASTransport, got %d", len(ngapSender.SentDownlinkNASTransport))
	}

	nm := decryptAndDecodeNasPdu(t, ue, ngapSender.SentDownlinkNASTransport[0].NASPDU, 0)
	if nm[2] != uint8(fgs.MsgRegistrationAccept) {
		t.Fatalf("expected RegistrationAccept, got %v", nm[2])
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
	assertPlainGmm(t, resp.NASPDU, uint8(fgs.MsgRegistrationReject))

	reject, err := fgs.ParseRegistrationReject(resp.NASPDU)
	if err != nil {
		t.Fatalf("could not parse RegistrationReject: %v", err)
	}

	if got, want := int(reject.Cause), 0x07; got != want {
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
	ue.Conn().RegistrationRequest.UplinkDataStatus = mustBitmap([]uint8{0x04, 0x00})

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

	nm := decryptAndDecodeNasPdu(t, ue, *ngapSender.SentInitialContextSetupRequest[0].NASPDU, 0)
	if nm[2] != uint8(fgs.MsgRegistrationAccept) {
		t.Fatalf("expected RegistrationAccept, got %v", nm[2])
	}
}

func TestMobilityReg_UplinkDataStatus_ActivateSuccess_NoUeContextRequest(t *testing.T) {
	ue, ngapSender, fakeSmf, amfInstance := buildMobilityRegUeAndAMF(t)

	snssai := &models.Snssai{Sst: 1}
	_ = ue.CreateSmContext(2, "ref-2", snssai)

	ue.Conn().RegistrationRequest.UplinkDataStatus = mustBitmap([]uint8{0x04, 0x00})

	ue.Conn().UeContextRequest = false

	HandleMobilityAndPeriodicRegistrationUpdating(context.TODO(), amfInstance, ue)

	if len(fakeSmf.ActivateSmContextCalls) != 1 {
		t.Fatalf("expected 1 ActivateSmContext call, got %d", len(fakeSmf.ActivateSmContextCalls))
	}

	// UeContextRequest=false + non-empty suList → sends PDUSessionResourceSetupRequest
	if len(ngapSender.SentPDUSessionResourceSetupRequest) != 1 {
		t.Fatalf("expected 1 PDUSessionResourceSetupRequest, got %d", len(ngapSender.SentPDUSessionResourceSetupRequest))
	}

	nm := decryptAndDecodeNasPdu(t, ue, *ngapSender.SentPDUSessionResourceSetupRequest[0].NASPDU, 0)
	if nm[2] != uint8(fgs.MsgRegistrationAccept) {
		t.Fatalf("expected RegistrationAccept, got %v", nm[2])
	}
}

func TestMobilityReg_UplinkDataStatus_ActivateError(t *testing.T) {
	ue, ngapSender, fakeSmf, amfInstance := buildMobilityRegUeAndAMF(t)

	snssai := &models.Snssai{Sst: 1}
	_ = ue.CreateSmContext(2, "ref-2", snssai)

	ue.Conn().RegistrationRequest.UplinkDataStatus = mustBitmap([]uint8{0x04, 0x00})

	fakeSmf.ActivateSmContextError = fmt.Errorf("activate error")

	HandleMobilityAndPeriodicRegistrationUpdating(context.TODO(), amfInstance, ue)

	if len(fakeSmf.ActivateSmContextCalls) != 1 {
		t.Fatalf("expected 1 ActivateSmContext call, got %d", len(fakeSmf.ActivateSmContextCalls))
	}

	// Even with error, the function continues and sends RegistrationAccept
	if len(ngapSender.SentDownlinkNASTransport) != 1 {
		t.Fatalf("expected 1 DownlinkNASTransport, got %d", len(ngapSender.SentDownlinkNASTransport))
	}

	nm := decryptAndDecodeNasPdu(t, ue, ngapSender.SentDownlinkNASTransport[0].NASPDU, 0)
	if nm[2] != uint8(fgs.MsgRegistrationAccept) {
		t.Fatalf("expected RegistrationAccept, got %v", nm[2])
	}
}

func TestMobilityReg_PDUSessionStatus_InactiveSession_ReleaseSmContext(t *testing.T) {
	ue, ngapSender, fakeSmf, amfInstance := buildMobilityRegUeAndAMF(t)

	snssai := &models.Snssai{Sst: 1}
	_ = ue.CreateSmContext(2, "ref-2", snssai)

	// PDUSessionStatus: PSI 2 is NOT active (bit 2 unset = 0x00)
	ue.Conn().RegistrationRequest.PDUSessionStatus = mustBitmap([]uint8{0x00, 0x00})

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

	nm := decryptAndDecodeNasPdu(t, ue, ngapSender.SentDownlinkNASTransport[0].NASPDU, 0)
	if nm[2] != uint8(fgs.MsgRegistrationAccept) {
		t.Fatalf("expected RegistrationAccept, got %v", nm[2])
	}
}

func TestMobilityReg_PDUSessionStatus_ActiveSession_NoRelease(t *testing.T) {
	ue, ngapSender, fakeSmf, amfInstance := buildMobilityRegUeAndAMF(t)

	snssai := &models.Snssai{Sst: 1}
	_ = ue.CreateSmContext(2, "ref-2", snssai)

	// PDUSessionStatus: PSI 2 IS active (bit 2 set = 0x04)
	ue.Conn().RegistrationRequest.PDUSessionStatus = mustBitmap([]uint8{0x04, 0x00})

	HandleMobilityAndPeriodicRegistrationUpdating(context.TODO(), amfInstance, ue)

	if len(fakeSmf.ReleaseSmContextCalls) != 0 {
		t.Fatalf("expected 0 ReleaseSmContext calls, got %d", len(fakeSmf.ReleaseSmContextCalls))
	}

	if len(ngapSender.SentDownlinkNASTransport) != 1 {
		t.Fatalf("expected 1 DownlinkNASTransport, got %d", len(ngapSender.SentDownlinkNASTransport))
	}

	nm := decryptAndDecodeNasPdu(t, ue, ngapSender.SentDownlinkNASTransport[0].NASPDU, 0)
	if nm[2] != uint8(fgs.MsgRegistrationAccept) {
		t.Fatalf("expected RegistrationAccept, got %v", nm[2])
	}
}

func TestMobilityReg_PDUSessionStatus_ReleaseError(t *testing.T) {
	ue, ngapSender, fakeSmf, amfInstance := buildMobilityRegUeAndAMF(t)

	snssai := &models.Snssai{Sst: 1}
	_ = ue.CreateSmContext(2, "ref-2", snssai)

	ue.Conn().RegistrationRequest.PDUSessionStatus = mustBitmap([]byte{0x00, 0x00}) // PSI 2 inactive → triggers release

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
	ue.Conn().RegistrationRequest.UplinkDataStatus = mustBitmap([]uint8{0x04, 0x00})
	ue.Conn().UeContextRequest = false

	ue.Conn().RegistrationRequest.AllowedPDUSessionStatus = mustBitmap([]uint8{0x04, 0x00})
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

	nmSetup := decryptAndDecodeNasPdu(t, ue, *ngapSender.SentPDUSessionResourceSetupRequest[0].NASPDU, 0)
	if nmSetup[2] != uint8(fgs.MsgRegistrationAccept) {
		t.Fatalf("expected RegistrationAccept in PDUSessionResourceSetupRequest, got %v", nmSetup[2])
	}

	nmDL := decryptAndDecodeNasPdu(t, ue, ngapSender.SentDownlinkNASTransport[0].NASPDU, 1)
	if nmDL[2] != uint8(fgs.MsgDLNASTransport) {
		t.Fatalf("expected DLNASTransport, got %v", nmDL[2])
	}

	if ue.N1N2Message() != nil {
		t.Fatal("expected N1N2Message to be nil after processing")
	}
}

func TestMobilityReg_AllowedPDUSessionStatus_N1N2_NilN2Info_EmptySuList(t *testing.T) {
	ue, ngapSender, _, amfInstance := buildMobilityRegUeAndAMF(t)

	// No UplinkDataStatus → suList remains empty

	ue.Conn().RegistrationRequest.AllowedPDUSessionStatus = mustBitmap([]uint8{0x04, 0x00})
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

	nmAccept := decryptAndDecodeNasPdu(t, ue, ngapSender.SentDownlinkNASTransport[0].NASPDU, 0)
	if nmAccept[2] != uint8(fgs.MsgRegistrationAccept) {
		t.Fatalf("expected RegistrationAccept in first DLNASTransport, got %v", nmAccept[2])
	}

	nmN1 := decryptAndDecodeNasPdu(t, ue, ngapSender.SentDownlinkNASTransport[1].NASPDU, 1)
	if nmN1[2] != uint8(fgs.MsgDLNASTransport) {
		t.Fatalf("expected DLNASTransport in second DLNASTransport, got %v", nmN1[2])
	}

	if ue.N1N2Message() != nil {
		t.Fatal("expected N1N2Message to be nil after processing")
	}
}

func TestMobilityReg_AllowedPDUSessionStatus_N1N2_WithN2Info_MissingSmContext(t *testing.T) {
	ue, _, _, amfInstance := buildMobilityRegUeAndAMF(t)

	ue.Conn().RegistrationRequest.AllowedPDUSessionStatus = mustBitmap([]uint8{0x04, 0x00})

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

	ue.Conn().RegistrationRequest.AllowedPDUSessionStatus = mustBitmap([]byte{0x08, 0x00}) // PSI 3

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

	nm := decryptAndDecodeNasPdu(t, ue, *ngapSender.SentPDUSessionResourceSetupRequest[0].NASPDU, 1)
	if nm[2] != uint8(fgs.MsgRegistrationAccept) {
		t.Fatalf("expected RegistrationAccept, got %v", nm[2])
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

	nm := decryptAndDecodeNasPdu(t, ue, *ngapSender.SentInitialContextSetupRequest[0].NASPDU, 0)
	if nm[2] != uint8(fgs.MsgRegistrationAccept) {
		t.Fatalf("expected RegistrationAccept, got %v", nm[2])
	}

	if len(ngapSender.SentDownlinkNASTransport) != 0 {
		t.Fatalf("expected 0 DownlinkNASTransport, got %d", len(ngapSender.SentDownlinkNASTransport))
	}
}

func TestMobilityReg_NoUeContextRequest_NonEmptySuList(t *testing.T) {
	ue, ngapSender, fakeSmf, amfInstance := buildMobilityRegUeAndAMF(t)

	snssai := &models.Snssai{Sst: 1}
	_ = ue.CreateSmContext(2, "ref-2", snssai)

	ue.Conn().RegistrationRequest.UplinkDataStatus = mustBitmap([]uint8{0x04, 0x00})

	ue.Conn().UeContextRequest = false

	HandleMobilityAndPeriodicRegistrationUpdating(context.TODO(), amfInstance, ue)

	if len(fakeSmf.ActivateSmContextCalls) != 1 {
		t.Fatalf("expected 1 ActivateSmContext call, got %d", len(fakeSmf.ActivateSmContextCalls))
	}

	if len(ngapSender.SentPDUSessionResourceSetupRequest) != 1 {
		t.Fatalf("expected 1 PDUSessionResourceSetupRequest, got %d", len(ngapSender.SentPDUSessionResourceSetupRequest))
	}

	nm := decryptAndDecodeNasPdu(t, ue, *ngapSender.SentPDUSessionResourceSetupRequest[0].NASPDU, 0)
	if nm[2] != uint8(fgs.MsgRegistrationAccept) {
		t.Fatalf("expected RegistrationAccept, got %v", nm[2])
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
	nm := decryptAndDecodeNasPdu(t, ue, resp.NASPDU, 0)

	if nm[2] != uint8(fgs.MsgRegistrationAccept) {
		t.Fatalf("expected RegistrationAccept, got %v", nm[2])
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
	return &db.Profile{ID: id, Name: "TestProfile", UeAmbrDownlink: "200 Mbps", UeAmbrUplink: "100 Mbps"}, nil
}

func (m *multiSliceDB) ListAllNetworkSlices(_ context.Context) ([]db.NetworkSlice, error) {
	sd1, sd2 := "010203", "aabbcc"

	return []db.NetworkSlice{
		{ID: "slice-1", Name: "slice-a", Sst: 1, Sd: &sd1},
		{ID: "slice-2", Name: "slice-b", Sst: 2, Sd: &sd2},
	}, nil
}

func (m *multiSliceDB) GetPolicyByProfileAndSlice(_ context.Context, profileID, sliceID string) (*db.Policy, error) {
	return &db.Policy{ID: sliceID, Name: "TestPolicy", ProfileID: profileID, SliceID: sliceID, DataNetworkID: "dn-1", SessionAmbrDownlink: "200 Mbps", SessionAmbrUplink: "100 Mbps"}, nil
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
	algo := nas.CipheringAES

	ue.SetKnasEncForTest(key)
	ue.SetKnasIntForTest(key)
	ue.SetCipheringAlgForTest(algo)
	ue.SetIntegrityAlgForTest(nas.IntegrityNull)

	ue.Conn().RegistrationRequest = &fgs.RegistrationRequest{}
	ue.Conn().RegistrationType5GS = fgs.RegistrationTypeMobilityUpdating
	ue.Conn().RegistrationRequest.GMMCapability = &fgs.GMMCapability{}

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

	plain := decryptAndDecodeNasPdu(t, ue, ngapSender.SentDownlinkNASTransport[0].NASPDU, 0)
	if plain[2] != uint8(fgs.MsgRegistrationAccept) {
		t.Fatalf("expected RegistrationAccept, got %v", plain[2])
	}

	regAccept, err := fgs.ParseRegistrationAccept(plain)
	if err != nil {
		t.Fatalf("could not parse RegistrationAccept: %v", err)
	}

	want := fgs.NSSAI{{SST: 1, SD: &[3]byte{1, 2, 3}}, {SST: 2, SD: &[3]byte{0xaa, 0xbb, 0xcc}}}
	if !reflect.DeepEqual(regAccept.AllowedNSSAI, want) {
		t.Fatalf("AllowedNSSAI = %+v, want %+v", regAccept.AllowedNSSAI, want)
	}
}

// A UE moving from EPC lists the PDN connections it is about to transfer in the
// PDU session status, and this AMF holds none of them. Synchronising against
// that would report every one inactive, and the UE "shall perform a local
// release" of exactly the sessions it came to move (TS 24.501 §5.5.1.3.4) —
// defeating the address preservation the move exists for. TS 23.502 §4.11.2.3
// steps 3 and 7 have the AMF skip the synchronisation instead.
func TestMobilityReg_MovingFromEPC_SkipsPDUSessionStatusSync(t *testing.T) {
	ue, ngapSender, fakeSmf, amfInstance := buildMobilityRegUeAndAMF(t)

	snssai := &models.Snssai{Sst: 1}
	_ = ue.CreateSmContext(2, "ref-2", snssai)

	// The UE reports every PDU session inactive on the 5GS side, as it must:
	// the connections are still in EPS.
	ue.Conn().RegistrationRequest.PDUSessionStatus = mustBitmap([]uint8{0x00, 0x00})
	ue.Conn().RegistrationRequest.UEStatus = &fgs.UEStatus{S1ModeReg: true}

	HandleMobilityAndPeriodicRegistrationUpdating(context.TODO(), amfInstance, ue)

	if len(fakeSmf.ReleaseSmContextCalls) != 0 {
		t.Errorf("released %v, want nothing: the UE is moving its connections, not abandoning them", fakeSmf.ReleasedSmContext)
	}

	if len(ngapSender.SentDownlinkNASTransport) != 1 {
		t.Fatalf("expected 1 DownlinkNASTransport, got %d", len(ngapSender.SentDownlinkNASTransport))
	}

	plain := decryptAndDecodeNasPdu(t, ue, ngapSender.SentDownlinkNASTransport[0].NASPDU, 0)
	if plain[2] != uint8(fgs.MsgRegistrationAccept) {
		t.Fatalf("expected RegistrationAccept, got %v", plain[2])
	}

	accept, err := fgs.ParseRegistrationAccept(plain)
	if err != nil {
		t.Fatalf("parse RegistrationAccept: %v", err)
	}

	// Omitting the IE is what leaves the UE's sessions alone: the local release
	// is conditioned on the network reporting them inactive.
	if accept.PDUSessionStatus != nil {
		t.Errorf("Registration Accept carries a PDU session status %v, want none for a UE moving from EPC", accept.PDUSessionStatus)
	}
}
