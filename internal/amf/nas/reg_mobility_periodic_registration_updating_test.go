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
	"github.com/ellanetworks/core/internal/interworking"
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
				Ciphering:     `["AES"]`,
				Integrity:     `["AES"]`,
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

	if err := amfInstance.CommitUEIdentity(context.Background(), ue, amf.MintAuthProofForRegistrationCommit()); err != nil {
		t.Fatalf("CommitUEIdentity: %v", err)
	}

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

	if ue.Conn().ReleaseAction != amf.UeContextReleaseAbortRegistration {
		t.Fatalf("ReleaseAction = %v, want the aborted-registration release", ue.Conn().ReleaseAction)
	}
}

// TS 24.501
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

	ue.Conn().RegistrationType5GS = fgs.RegistrationTypeMobilityUpdating

	contextSetup(context.TODO(), amfInstance, ue,
		&fgs.RegistrationRequest{UpdateType5GS: &fgs.UpdateType5GS{NGRANRCU: true}}, nil)

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

	if ue.Conn().ReleaseAction != amf.UeContextReleaseAbortRegistration {
		t.Fatalf("ReleaseAction = %v, want the aborted-registration release", ue.Conn().ReleaseAction)
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

	_ = ue.CreateSmContext(2, "ref-2", snssai, "internet")

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
	_ = ue.CreateSmContext(2, "ref-2", snssai, "internet")

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
	_ = ue.CreateSmContext(2, "ref-2", snssai, "internet")

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
	_ = ue.CreateSmContext(2, "ref-2", snssai, "internet")

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
	_ = ue.CreateSmContext(2, "ref-2", snssai, "internet")

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
	_ = ue.CreateSmContext(2, "ref-2", snssai, "internet")

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
	_ = ue.CreateSmContext(2, "ref-2", snssai, "internet")

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

	// TS 24.501 §5.3.4 d)
	regAccept, err := fgs.ParseRegistrationAccept(nmAccept)
	if err != nil {
		t.Fatalf("could not parse RegistrationAccept: %v", err)
	}

	if regAccept.TAIList == nil {
		t.Error("the accept carries no TAI list")
	}

	if len(ue.RegistrationArea) == 0 {
		t.Error("no registration area was allocated, so the AMF cannot page this UE")
	}
}

func TestMobilityReg_AllowedPDUSessionStatus_N1N2_WithN2Info_MissingSmContext(t *testing.T) {
	ue, _, _, amfInstance := buildMobilityRegUeAndAMF(t)

	ue.Conn().RegistrationRequest.AllowedPDUSessionStatus = mustBitmap([]uint8{0x04, 0x00})

	ue.SetN1N2Message(&models.N1N2MessageTransferRequest{
		PduSessionID:            3,
		BinaryDataN1Message:     []byte{0x01, 0x02},
		BinaryDataN2Information: []byte{0x03, 0x04},
	})

	HandleMobilityAndPeriodicRegistrationUpdating(context.TODO(), amfInstance, ue)

	if ue.Conn().ReleaseAction != amf.UeContextReleaseAbortRegistration {
		t.Fatalf("ReleaseAction = %v, want the aborted-registration release", ue.Conn().ReleaseAction)
	}
}

func TestMobilityReg_AllowedPDUSessionStatus_N1N2_WithN2Info_SmContextExists(t *testing.T) {
	ue, ngapSender, _, amfInstance := buildMobilityRegUeAndAMF(t)

	snssai := &models.Snssai{Sst: 1}
	_ = ue.CreateSmContext(3, "ref-3", snssai, "internet")

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
	_ = ue.CreateSmContext(2, "ref-2", snssai, "internet")

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
	return &db.Profile{ID: id, Name: "TestProfile", Allow4G: true, Allow5G: true, UeAmbrDownlink: "200 Mbps", UeAmbrUplink: "100 Mbps"}, nil
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

	if err := amfInstance.CommitUEIdentity(context.Background(), ue, amf.MintAuthProofForRegistrationCommit()); err != nil {
		t.Fatalf("CommitUEIdentity: %v", err)
	}

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

// The fresh K_gNB and the {NH, NCC} anchored on it are one derivation
// (TS 33.501 §6.9.2.1.1). Anchoring the chain on the replaced key hands every
// later handover an {NH, NCC} the UE cannot reproduce (§6.9.2.3.4).
func TestMobilityReg_ReanchorsASKeyChain(t *testing.T) {
	ue, _, _, amfInstance := buildMobilityRegUeAndAMF(t)

	ue.SetKamfForTest("0f0e0d0c0b0a09080706050403020100f0e0d0c0b0a090807060504030201000")

	stale := make([]uint8, 32)
	for i := range stale {
		stale[i] = 0xAA
	}

	ue.SetNHForTest(stale)
	ue.SetNCCForTest(5)
	ue.SetKgnbForTest(nil)

	HandleMobilityAndPeriodicRegistrationUpdating(context.TODO(), amfInstance, ue)

	if len(ue.KgnbForTest()) == 0 {
		t.Fatal("no K_gNB derived; the update did not re-key at all")
	}

	if nh := ue.NHForTest(); nh == [32]uint8(stale) {
		t.Error("NH still anchored on the previous K_gNB: it must be re-derived from the fresh one")
	}

	if ncc := ue.NCCForTest(); ncc != 1 {
		t.Errorf("NCC = %d, want 1: a fresh K_gNB starts a fresh chain", ncc)
	}
}

// TS 33.501 §6.8.1.3
func TestMobilityReg_KeepsTheMappedKeyChainOnAHandoverConnection(t *testing.T) {
	ue, _, _, amfInstance := buildMobilityRegUeAndAMF(t)

	ue.SetKamfForTest("0f0e0d0c0b0a09080706050403020100f0e0d0c0b0a090807060504030201000")

	mapped := make([]uint8, 32)
	for i := range mapped {
		mapped[i] = 0xAA
	}

	ue.SetNHForTest(mapped)
	ue.SetNCCForTest(1)
	ue.SetKgnbForTest(nil)
	ue.Conn().MarkICSCompleted()

	HandleMobilityAndPeriodicRegistrationUpdating(context.TODO(), amfInstance, ue)

	if nh := ue.NHForTest(); nh != [32]uint8(mapped) {
		t.Error("the {NH, NCC} the target gNB was keyed from was overwritten: the next handover or path switch would desynchronise the AS key chain")
	}

	if ncc := ue.NCCForTest(); ncc != 1 {
		t.Errorf("NCC = %d, want the stored 1", ncc)
	}

	if len(ue.KgnbForTest()) != 0 {
		t.Error("a K_gNB was derived on a connection that already carries an AS context")
	}
}

// TS 24.501 §5.5.1.3.4
func TestMobilityReg_ReportsTheEPSBearerContextStatusAfterAnArrivalFromEPS(t *testing.T) {
	ue, ngapSender, _, amfInstance := buildMobilityRegUeAndAMF(t)

	amfInstance.EPS = &fakeEPSPeer{}

	if err := ue.CreateSmContext(5, "ref-5", &models.Snssai{Sst: 1, Sd: "010203"}, "internet"); err != nil {
		t.Fatalf("CreateSmContext: %v", err)
	}

	ue.SetEPSBearerIdentity(5, 6)
	ue.Conn().EPSArrival = &amf.EPSArrival{}
	ue.Conn().MarkICSCompleted()

	HandleMobilityAndPeriodicRegistrationUpdating(context.TODO(), amfInstance, ue)

	if len(ngapSender.SentDownlinkNASTransport) != 1 {
		t.Fatalf("expected 1 DownlinkNASTransport, got %d", len(ngapSender.SentDownlinkNASTransport))
	}

	plain := decryptAndDecodeNasPdu(t, ue, ngapSender.SentDownlinkNASTransport[0].NASPDU, 0)

	regAccept, err := fgs.ParseRegistrationAccept(plain)
	if err != nil {
		t.Fatalf("could not parse RegistrationAccept: %v", err)
	}

	if regAccept.EPSBearerContextStatus == nil {
		t.Fatal("no EPS bearer context status: a UE whose PDN connection did not transfer keeps its QoS flow descriptions and rules for ever")
	}

	for ebi := 1; ebi < 16; ebi++ {
		want := ebi == 6
		if got := regAccept.EPSBearerContextStatus.Active[ebi]; got != want {
			t.Errorf("EBI(%d) = %v, want %v", ebi, got, want)
		}
	}
}

// TS 24.501 §8.2.7.31
func TestMobilityReg_OmitsTheEPSBearerContextStatusWithoutAnArrivalFromEPS(t *testing.T) {
	ue, ngapSender, _, amfInstance := buildMobilityRegUeAndAMF(t)

	amfInstance.EPS = &fakeEPSPeer{}

	if err := ue.CreateSmContext(5, "ref-5", &models.Snssai{Sst: 1, Sd: "010203"}, "internet"); err != nil {
		t.Fatalf("CreateSmContext: %v", err)
	}

	ue.SetEPSBearerIdentity(5, 6)

	HandleMobilityAndPeriodicRegistrationUpdating(context.TODO(), amfInstance, ue)

	if len(ngapSender.SentDownlinkNASTransport) != 1 {
		t.Fatalf("expected 1 DownlinkNASTransport, got %d", len(ngapSender.SentDownlinkNASTransport))
	}

	plain := decryptAndDecodeNasPdu(t, ue, ngapSender.SentDownlinkNASTransport[0].NASPDU, 0)

	regAccept, err := fgs.ParseRegistrationAccept(plain)
	if err != nil {
		t.Fatalf("could not parse RegistrationAccept: %v", err)
	}

	if regAccept.EPSBearerContextStatus != nil {
		t.Error("the EPS bearer context status went out on a registration that is not an inter-system change")
	}
}

func (f *fakeEPSPeer) CancelRegistration(_ context.Context, supi etsi.SUPI) {
	f.Cancelled = append(f.Cancelled, supi.IMSI())
}

type fakeEPSPeer struct {
	MMContextRequests []interworking.MMContextRequest
	MMContextResponse interworking.MMContextResponse
	MMContextErr      error
	Acked             bool
	AckedSupi         etsi.SUPI
	Transferred       []uint8
	Cancelled         []string
}

func (*fakeEPSPeer) ForwardRelocation(context.Context, interworking.ForwardRelocationRequest) (interworking.ForwardRelocationResponse, error) {
	return interworking.ForwardRelocationResponse{}, nil
}

func (*fakeEPSPeer) RelocationCancel(context.Context, etsi.SUPI, interworking.RelocationID) error {
	return nil
}

func (*fakeEPSPeer) RelocationComplete(context.Context, etsi.SUPI, interworking.RelocationID) error {
	return nil
}

func (p *fakeEPSPeer) MMContext(_ context.Context, req interworking.MMContextRequest) (interworking.MMContextResponse, error) {
	p.MMContextRequests = append(p.MMContextRequests, req)

	if p.MMContextErr != nil {
		return interworking.MMContextResponse{}, p.MMContextErr
	}

	return p.MMContextResponse, nil
}

func (p *fakeEPSPeer) MMContextAck(_ context.Context, supi etsi.SUPI, transferred []uint8) error {
	p.Acked, p.AckedSupi, p.Transferred = true, supi, transferred

	return nil
}

// TS 24.501 §5.5.1.3.2 d) / §5.5.1.3.4 and TS 23.502 §4.11.1.3.3 step 14
func TestMobilityReg_ReleasesPDUSessionsTheUEDeactivatedInEPS(t *testing.T) {
	ue, ngapSender, smf, amfInstance := buildMobilityRegUeAndAMF(t)

	amfInstance.EPS = &fakeEPSPeer{}

	for _, s := range []struct{ psi, ebi uint8 }{{1, 5}, {3, 6}} {
		if err := ue.CreateSmContext(s.psi, fmt.Sprintf("ref-%d", s.psi), &models.Snssai{Sst: 1, Sd: "010203"}, "internet"); err != nil {
			t.Fatalf("CreateSmContext: %v", err)
		}

		ue.SetEPSBearerIdentity(s.psi, s.ebi)
	}

	status := new(nas.EPSBearerContextStatus)
	status.Active[5] = true

	ue.Conn().EPSArrival = &amf.EPSArrival{}
	ue.Conn().RegistrationRequest.EPSBearerContextStatus = status
	ue.Conn().MarkICSCompleted()

	HandleMobilityAndPeriodicRegistrationUpdating(context.TODO(), amfInstance, ue)

	if got := smf.ReleaseSmContextCalls; len(got) != 1 || got[0].SmContextRef != "ref-3" {
		t.Errorf("released %v, want only the session of the bearer the UE dropped", got)
	}

	if _, still := ue.SmContextFindByPDUSessionID(3); still {
		t.Error("the AMF kept the SM context of a released session, so its EBI is still reported active")
	}

	plain := decryptAndDecodeNasPdu(t, ue, ngapSender.SentDownlinkNASTransport[0].NASPDU, 0)

	regAccept, err := fgs.ParseRegistrationAccept(plain)
	if err != nil {
		t.Fatalf("could not parse RegistrationAccept: %v", err)
	}

	if regAccept.EPSBearerContextStatus == nil {
		t.Fatal("no EPS bearer context status in the accept")
	}

	if !regAccept.EPSBearerContextStatus.Active[5] || regAccept.EPSBearerContextStatus.Active[6] {
		t.Errorf("returned status %v, want only EBI 5 active", regAccept.EPSBearerContextStatus)
	}
}

// TS 24.501 §8.2.6.23
func TestMobilityReg_IgnoresTheEPSBearerContextStatusWithoutAnArrivalFromEPS(t *testing.T) {
	ue, _, smf, amfInstance := buildMobilityRegUeAndAMF(t)

	amfInstance.EPS = &fakeEPSPeer{}

	if err := ue.CreateSmContext(3, "ref-3", &models.Snssai{Sst: 1, Sd: "010203"}, "internet"); err != nil {
		t.Fatalf("CreateSmContext: %v", err)
	}

	ue.SetEPSBearerIdentity(3, 6)
	ue.Conn().RegistrationRequest.EPSBearerContextStatus = new(nas.EPSBearerContextStatus)

	HandleMobilityAndPeriodicRegistrationUpdating(context.TODO(), amfInstance, ue)

	if len(smf.ReleaseSmContextCalls) != 0 {
		t.Errorf("released %v on a registration that is not an inter-system change", smf.ReleaseSmContextCalls)
	}
}

// TS 23.501 §5.3.4.1.1
func TestMobilityReg_RejectsASubscriberBarredFrom5G(t *testing.T) {
	ue, ngapSender, _, _ := buildMobilityRegUeAndAMF(t)

	amfInstance := amf.New(&fakeDBInstance{
		Operator:  &db.Operator{Mcc: "001", Mnc: "01", SupportedTACs: `["000001"]`},
		BarFrom5G: true,
	}, nil, &fakeSmf{})

	HandleMobilityAndPeriodicRegistrationUpdating(context.TODO(), amfInstance, ue)

	if len(ngapSender.SentDownlinkNASTransport) != 1 {
		t.Fatalf("expected 1 DownlinkNASTransport, got %d", len(ngapSender.SentDownlinkNASTransport))
	}

	pdu := ngapSender.SentDownlinkNASTransport[0].NASPDU
	if len(pdu) < 3 || pdu[2] != uint8(fgs.MsgRegistrationReject) {
		t.Fatalf("sent % x, want a Registration Reject", pdu)
	}
}

// TS 24.501 §5.5.1.3.4
func TestMobilityReg_AcceptedRegistrationIsFindableBySupi(t *testing.T) {
	for _, tc := range []struct {
		name string
		typ  fgs.RegistrationType
	}{
		{"mobility", fgs.RegistrationTypeMobilityUpdating},
		{"periodic", fgs.RegistrationTypePeriodicUpdating},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ue, ngapSender, _, amfInstance := buildMobilityRegUeAndAMF(t)
			ue.Conn().RegistrationType5GS = tc.typ

			HandleMobilityAndPeriodicRegistrationUpdating(t.Context(), amfInstance, ue)

			if len(ngapSender.SentDownlinkNASTransport) != 1 {
				t.Fatalf("DownlinkNASTransport count = %d, want the registration accept", len(ngapSender.SentDownlinkNASTransport))
			}

			held, ok := amfInstance.LookupUeBySupi(ue.Supi())
			if !ok || held != ue {
				t.Fatalf("LookupUeBySupi = (%p, %v), want the accepted context %p", held, ok, ue)
			}
		})
	}
}

func TestMobilityReg_AcceptedRegistrationCanReceiveAnN1N2Transfer(t *testing.T) {
	ue, _, _, amfInstance := buildMobilityRegUeAndAMF(t)

	HandleMobilityAndPeriodicRegistrationUpdating(t.Context(), amfInstance, ue)

	err := amfInstance.TransferN1N2Message(t.Context(), ue.Supi(), models.N1N2MessageTransferRequest{
		PduSessionID:        1,
		BinaryDataN1Message: []byte{0x2e, 0x01, 0x01},
	})
	if err != nil && err.Error() == "ue context not found" {
		t.Fatal("the SMF cannot reach a UE the AMF has just accepted: no PDU session can ever be established")
	}
}
