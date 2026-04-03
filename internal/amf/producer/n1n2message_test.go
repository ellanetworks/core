// Copyright 2026 Ella Networks
//
// SPDX-License-Identifier: Apache-2.0

package producer_test

import (
	"context"
	"testing"

	"github.com/ellanetworks/core/etsi"
	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/amf/ngap/send"
	"github.com/ellanetworks/core/internal/amf/producer"
	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/smf"
	"github.com/free5gc/aper"
	"github.com/free5gc/nas/nasType"
	"github.com/free5gc/ngap/ngapType"
	"go.uber.org/zap"
)

// --- Fakes ---

type fakeNGAPSender struct {
	pduSessionSetupCalls      int
	initialContextSetupCalls  int
	downlinkNasTransportCalls int
}

func (f *fakeNGAPSender) SendToRan(context.Context, []byte, send.NGAPProcedure) error { return nil }
func (f *fakeNGAPSender) SendNGSetupFailure(context.Context, *ngapType.Cause) error   { return nil }
func (f *fakeNGAPSender) SendNGSetupResponse(context.Context, *models.Guami, *models.PlmnSupportItem, string, int64) error {
	return nil
}

func (f *fakeNGAPSender) SendNGResetAcknowledge(context.Context, *ngapType.UEAssociatedLogicalNGConnectionList) error {
	return nil
}

func (f *fakeNGAPSender) SendErrorIndication(context.Context, *ngapType.Cause, *ngapType.CriticalityDiagnostics) error {
	return nil
}

func (f *fakeNGAPSender) SendRanConfigurationUpdateAcknowledge(context.Context, *ngapType.CriticalityDiagnostics) error {
	return nil
}

func (f *fakeNGAPSender) SendRanConfigurationUpdateFailure(context.Context, ngapType.Cause, *ngapType.CriticalityDiagnostics) error {
	return nil
}

func (f *fakeNGAPSender) SendDownlinkRanConfigurationTransfer(context.Context, *ngapType.SONConfigurationTransfer) error {
	return nil
}

func (f *fakeNGAPSender) SendPathSwitchRequestFailure(context.Context, int64, int64, *ngapType.PDUSessionResourceReleasedListPSFail, *ngapType.CriticalityDiagnostics) error {
	return nil
}

func (f *fakeNGAPSender) SendAMFStatusIndication(context.Context, ngapType.UnavailableGUAMIList) error {
	return nil
}

func (f *fakeNGAPSender) SendUEContextReleaseCommand(context.Context, int64, int64, int, aper.Enumerated) error {
	return nil
}

func (f *fakeNGAPSender) SendDownlinkNasTransport(_ context.Context, _, _ int64, _ []byte, _ *ngapType.MobilityRestrictionList) error {
	f.downlinkNasTransportCalls++
	return nil
}

func (f *fakeNGAPSender) SendPDUSessionResourceReleaseCommand(context.Context, int64, int64, []byte, ngapType.PDUSessionResourceToReleaseListRelCmd) error {
	return nil
}

func (f *fakeNGAPSender) SendHandoverCancelAcknowledge(context.Context, int64, int64) error {
	return nil
}

func (f *fakeNGAPSender) SendPDUSessionResourceModifyConfirm(context.Context, int64, int64, ngapType.PDUSessionResourceModifyListModCfm, ngapType.PDUSessionResourceFailedToModifyListModCfm) error {
	return nil
}

func (f *fakeNGAPSender) SendPDUSessionResourceSetupRequest(_ context.Context, _, _ int64, _, _ string, _ []byte, _ ngapType.PDUSessionResourceSetupListSUReq) error {
	f.pduSessionSetupCalls++
	return nil
}

func (f *fakeNGAPSender) SendHandoverPreparationFailure(context.Context, int64, int64, ngapType.Cause, *ngapType.CriticalityDiagnostics) error {
	return nil
}

func (f *fakeNGAPSender) SendLocationReportingControl(context.Context, int64, int64, ngapType.EventType) error {
	return nil
}

func (f *fakeNGAPSender) SendHandoverCommand(context.Context, int64, int64, ngapType.HandoverType, ngapType.PDUSessionResourceHandoverList, ngapType.PDUSessionResourceToReleaseListHOCmd, ngapType.TargetToSourceTransparentContainer) error {
	return nil
}

func (f *fakeNGAPSender) SendInitialContextSetupRequest(_ context.Context, _, _ int64, _, _ string, _ *models.Snssai, _ []byte, _ models.PlmnID, _ string, _ *models.UERadioCapabilityForPaging, _ *nasType.UESecurityCapability, _ []byte, _ *ngapType.PDUSessionResourceSetupListCxtReq, _ *models.Guami) error {
	f.initialContextSetupCalls++
	return nil
}

func (f *fakeNGAPSender) SendPathSwitchRequestAcknowledge(context.Context, int64, int64, *nasType.UESecurityCapability, uint8, []byte, ngapType.PDUSessionResourceSwitchedList, ngapType.PDUSessionResourceReleasedListPSAck, *models.PlmnSupportItem) error {
	return nil
}

func (f *fakeNGAPSender) SendHandoverRequest(context.Context, int64, ngapType.HandoverType, string, string, *nasType.UESecurityCapability, uint8, []byte, ngapType.Cause, ngapType.PDUSessionResourceSetupListHOReq, ngapType.SourceToTargetTransparentContainer, *models.PlmnSupportItem, *models.Guami) error {
	return nil
}

type fakeDBInstance struct {
	operator *db.Operator
}

func (f *fakeDBInstance) GetOperator(context.Context) (*db.Operator, error) {
	return f.operator, nil
}

func (f *fakeDBInstance) GetSubscriber(context.Context, string) (*db.Subscriber, error) {
	return nil, nil
}

func (f *fakeDBInstance) GetDataNetworkByID(context.Context, int) (*db.DataNetwork, error) {
	return nil, nil
}

func (f *fakeDBInstance) GetProfileByID(context.Context, int) (*db.Profile, error) {
	return nil, nil
}

func (f *fakeDBInstance) ListAllNetworkSlices(context.Context) ([]db.NetworkSlice, error) {
	return nil, nil
}

func (f *fakeDBInstance) GetPolicyByProfileAndSlice(context.Context, int, int) (*db.Policy, error) {
	return nil, nil
}

type fakeSmf struct{}

func (f *fakeSmf) GetSession(string) *smf.SMContext      { return nil }
func (f *fakeSmf) SessionsByDNN(string) []*smf.SMContext { return nil }
func (f *fakeSmf) SessionCount() int                     { return 0 }
func (f *fakeSmf) CreateSmContext(context.Context, etsi.SUPI, uint8, string, *models.Snssai, []byte) (string, []byte, error) {
	return "", nil, nil
}
func (f *fakeSmf) ActivateSmContext(context.Context, string) ([]byte, error) { return nil, nil }
func (f *fakeSmf) DeactivateSmContext(context.Context, string) error         { return nil }
func (f *fakeSmf) ReleaseSmContext(context.Context, string) error            { return nil }
func (f *fakeSmf) UpdateSmContextN1Msg(context.Context, string, []byte) (*smf.UpdateResult, error) {
	return nil, nil
}

func (f *fakeSmf) UpdateSmContextN2InfoPduResSetupRsp(context.Context, string, []byte) error {
	return nil
}

func (f *fakeSmf) UpdateSmContextN2InfoPduResSetupFail(context.Context, string, []byte) error {
	return nil
}
func (f *fakeSmf) UpdateSmContextN2InfoPduResRelRsp(context.Context, string) error { return nil }
func (f *fakeSmf) UpdateSmContextCauseDuplicatePDUSessionID(context.Context, string) ([]byte, error) {
	return nil, nil
}

func (f *fakeSmf) UpdateSmContextN2HandoverPreparing(context.Context, string, []byte) ([]byte, error) {
	return nil, nil
}

func (f *fakeSmf) UpdateSmContextN2HandoverPrepared(context.Context, string, []byte) ([]byte, error) {
	return nil, nil
}

func (f *fakeSmf) UpdateSmContextXnHandoverPathSwitchReq(context.Context, string, []byte) ([]byte, error) {
	return nil, nil
}
func (f *fakeSmf) UpdateSmContextHandoverFailed(context.Context, string, []byte) error { return nil }

// --- Helpers ---

func mustSUPI(t *testing.T, imsi string) etsi.SUPI {
	t.Helper()

	s, err := etsi.NewSUPIFromIMSI(imsi)
	if err != nil {
		t.Fatalf("bad IMSI: %v", err)
	}

	return s
}

func addUE(t *testing.T, amfInstance *amf.AMF, imsi string, setup func(*amf.AmfUe)) *amf.AmfUe {
	t.Helper()
	supi := mustSUPI(t, imsi)
	ue := amf.NewAmfUe()
	ue.Supi = supi

	ue.Log = zap.NewNop()
	if setup != nil {
		setup(ue)
	}

	if err := amfInstance.AddAmfUeToUePool(ue); err != nil {
		t.Fatalf("AddAmfUeToUePool: %v", err)
	}

	return ue
}

func newReq() models.N1N2MessageTransferRequest {
	return models.N1N2MessageTransferRequest{
		PduSessionID:            1,
		SNssai:                  &models.Snssai{Sst: 1, Sd: "010203"},
		BinaryDataN1Message:     []byte{0x01, 0x02},
		BinaryDataN2Information: []byte{0x03, 0x04},
	}
}

// --- TransferN1N2Message tests ---

func TestTransferN1N2Message_UENotFound(t *testing.T) {
	amfInstance := amf.New(nil, nil, nil)
	supi := mustSUPI(t, "001010000000001")

	err := producer.TransferN1N2Message(context.Background(), amfInstance, supi, newReq())
	if err == nil {
		t.Fatal("expected error for missing UE")
	}
}

func TestTransferN1N2Message_UENotConnected(t *testing.T) {
	amfInstance := amf.New(nil, nil, &fakeSmf{})

	ue := addUE(t, amfInstance, "001010000000002", nil)

	err := producer.TransferN1N2Message(context.Background(), amfInstance, ue.Supi, newReq())
	if err == nil {
		t.Fatal("expected error for UE not connected to RAN")
	}
}

func TestTransferN1N2Message_InitialContextAlreadySent(t *testing.T) {
	sender := &fakeNGAPSender{}
	amfInstance := amf.New(nil, nil, &fakeSmf{})

	ue := addUE(t, amfInstance, "001010000000003", func(u *amf.AmfUe) {
		u.Ambr = &models.Ambr{Uplink: "1000000", Downlink: "1000000"}
	})

	ranUe := &amf.RanUe{
		AmfUeNgapID:                    1,
		RanUeNgapID:                    1,
		SentInitialContextSetupRequest: true,
		Radio:                          &amf.Radio{NGAPSender: sender},
		Log:                            zap.NewNop(),
	}
	ue.AttachRanUe(ranUe)

	err := producer.TransferN1N2Message(context.Background(), amfInstance, ue.Supi, newReq())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if sender.pduSessionSetupCalls != 1 {
		t.Fatalf("expected 1 PDUSessionResourceSetupRequest, got %d", sender.pduSessionSetupCalls)
	}
}

func TestTransferN1N2Message_InitialContextNotYetSent(t *testing.T) {
	sender := &fakeNGAPSender{}
	fakeDB := &fakeDBInstance{
		operator: &db.Operator{
			Mcc: "001",
			Mnc: "01",
		},
	}
	amfInstance := amf.New(fakeDB, nil, &fakeSmf{})

	ue := addUE(t, amfInstance, "001010000000004", func(u *amf.AmfUe) {
		u.Ambr = &models.Ambr{Uplink: "1000000", Downlink: "1000000"}
	})

	ranUe := &amf.RanUe{
		AmfUeNgapID:                    1,
		RanUeNgapID:                    1,
		SentInitialContextSetupRequest: false,
		Radio:                          &amf.Radio{NGAPSender: sender},
		Log:                            zap.NewNop(),
	}
	ue.AttachRanUe(ranUe)

	err := producer.TransferN1N2Message(context.Background(), amfInstance, ue.Supi, newReq())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if sender.initialContextSetupCalls != 1 {
		t.Fatalf("expected 1 InitialContextSetupRequest, got %d", sender.initialContextSetupCalls)
	}

	if !ranUe.SentInitialContextSetupRequest {
		t.Fatal("expected SentInitialContextSetupRequest to be set to true")
	}
}

// --- N2MessageTransferOrPage tests ---

func TestN2MessageTransferOrPage_UENotFound(t *testing.T) {
	amfInstance := amf.New(nil, nil, nil)
	supi := mustSUPI(t, "001010000000005")

	err := producer.N2MessageTransferOrPage(context.Background(), amfInstance, supi, newReq())
	if err == nil {
		t.Fatal("expected error for missing UE")
	}
}

func TestN2MessageTransferOrPage_OnGoingPaging(t *testing.T) {
	amfInstance := amf.New(nil, nil, &fakeSmf{})

	ue := addUE(t, amfInstance, "001010000000006", nil)
	ue.SetOnGoing(amf.OnGoingProcedurePaging)

	err := producer.N2MessageTransferOrPage(context.Background(), amfInstance, ue.Supi, newReq())
	if err == nil {
		t.Fatal("expected error for ongoing paging")
	}
}

func TestN2MessageTransferOrPage_OnGoingRegistration(t *testing.T) {
	amfInstance := amf.New(nil, nil, &fakeSmf{})

	ue := addUE(t, amfInstance, "001010000000007", nil)
	ue.SetOnGoing(amf.OnGoingProcedureRegistration)

	err := producer.N2MessageTransferOrPage(context.Background(), amfInstance, ue.Supi, newReq())
	if err == nil {
		t.Fatal("expected error for ongoing registration")
	}
}

func TestN2MessageTransferOrPage_OnGoingN2Handover(t *testing.T) {
	amfInstance := amf.New(nil, nil, &fakeSmf{})

	ue := addUE(t, amfInstance, "001010000000008", nil)
	ue.SetOnGoing(amf.OnGoingProcedureN2Handover)

	err := producer.N2MessageTransferOrPage(context.Background(), amfInstance, ue.Supi, newReq())
	if err == nil {
		t.Fatal("expected error for ongoing N2 handover")
	}
}

func TestN2MessageTransferOrPage_ConnectedUE_InitialCtxSent(t *testing.T) {
	sender := &fakeNGAPSender{}
	amfInstance := amf.New(nil, nil, &fakeSmf{})

	ue := addUE(t, amfInstance, "001010000000009", func(u *amf.AmfUe) {
		u.Ambr = &models.Ambr{Uplink: "1000000", Downlink: "1000000"}
	})

	ranUe := &amf.RanUe{
		AmfUeNgapID:                    1,
		RanUeNgapID:                    1,
		SentInitialContextSetupRequest: true,
		Radio:                          &amf.Radio{NGAPSender: sender},
		Log:                            zap.NewNop(),
	}
	ue.AttachRanUe(ranUe)

	err := producer.N2MessageTransferOrPage(context.Background(), amfInstance, ue.Supi, newReq())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if sender.pduSessionSetupCalls != 1 {
		t.Fatalf("expected 1 PDUSessionResourceSetupRequest, got %d", sender.pduSessionSetupCalls)
	}
}

func TestN2MessageTransferOrPage_NotRegistered_NoPaging(t *testing.T) {
	amfInstance := amf.New(nil, nil, &fakeSmf{})

	ue := addUE(t, amfInstance, "001010000000010", nil)

	err := producer.N2MessageTransferOrPage(context.Background(), amfInstance, ue.Supi, newReq())
	if err == nil {
		t.Fatal("expected error for UE not in registered state")
	}
}

// --- TransferN1Msg tests ---

func TestTransferN1Msg_UENotFound(t *testing.T) {
	amfInstance := amf.New(nil, nil, nil)
	supi := mustSUPI(t, "001010000000011")

	err := producer.TransferN1Msg(context.Background(), amfInstance, supi, []byte{0x01}, 1)
	if err == nil {
		t.Fatal("expected error for missing UE")
	}
}

func TestTransferN1Msg_UENotConnected(t *testing.T) {
	amfInstance := amf.New(nil, nil, &fakeSmf{})

	ue := addUE(t, amfInstance, "001010000000012", nil)

	err := producer.TransferN1Msg(context.Background(), amfInstance, ue.Supi, []byte{0x01}, 1)
	if err == nil {
		t.Fatal("expected error for UE not connected to RAN")
	}
}

func TestTransferN1Msg_Success(t *testing.T) {
	sender := &fakeNGAPSender{}
	amfInstance := amf.New(nil, nil, &fakeSmf{})

	ue := addUE(t, amfInstance, "001010000000013", nil)

	ranUe := &amf.RanUe{
		AmfUeNgapID: 1,
		RanUeNgapID: 1,
		Radio:       &amf.Radio{NGAPSender: sender},
		Log:         zap.NewNop(),
	}
	ue.AttachRanUe(ranUe)

	err := producer.TransferN1Msg(context.Background(), amfInstance, ue.Supi, []byte{0x01}, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if sender.downlinkNasTransportCalls != 1 {
		t.Fatalf("expected 1 DownlinkNasTransport, got %d", sender.downlinkNasTransportCalls)
	}
}
