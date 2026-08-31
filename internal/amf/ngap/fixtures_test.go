// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"context"
	"fmt"

	"github.com/ellanetworks/core/etsi"
	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/sctp"
	"github.com/ellanetworks/core/internal/smf"
	"github.com/ellanetworks/core/nas/fgs"
	"github.com/ellanetworks/core/ngap"
)

var operatorPLMN = ngap.PLMNIdentity{0x00, 0xf1, 0x10}

type fakeDBInstance struct {
	BarFrom5G   bool
	Operator    *db.Operator
	OperatorErr error
	Slices      []db.NetworkSlice
	SlicesErr   error
}

type SmfPathSwitchCall struct {
	SmContextRef string
	N2Data       []byte
}

type SmfHandoverFailedCall struct {
	SmContextRef string
	N2Data       []byte
}

type SmfN2InfoCall struct {
	SmContextRef string
	N2Data       []byte
}

type fakeSmfSbi struct {
	*smf.SMF
	PathSwitchResponse          []byte
	PathSwitchErr               error
	HandoverFailedErr           error
	PathSwitchCalls             []*SmfPathSwitchCall
	ModifyIndicationResponse    []byte
	ModifyIndicationErr         error
	ModifyIndicationCalls       []*SmfN2InfoCall
	HandoverFailedCalls         []*SmfHandoverFailedCall
	PduResSetupRspCalls         []*SmfN2InfoCall
	PduResSetupFailCalls        []*SmfN2InfoCall
	PduResRelRspCalls           []string
	PduResRelRspRemoved         bool
	DeactivateSmContextCalls    []string
	N2HandoverCompleteCalls     []string
	N2HandoverCanceledCalls     []string
	N2HandoverCompleteErr       error
	ReleaseSmContextCalls       []string
	N2HandoverPreparingCalls    []string
	N2HandoverPreparingData     []*SmfN2InfoCall
	N2HandoverPreparingErr      error
	N2HandoverPreparingErrByRef map[string]error
	N2HandoverPreparedCalls     []string
	N2HandoverPreparedData      []*SmfN2InfoCall
	N2HandoverPreparedErr       error
	N2HandoverFailedCalls       []*SmfN2InfoCall
	N2HandoverFailedErr         error
	N2HandoverCanceledErr       error
	ReleaseSmContextErr         error
	PrepareFromEPSResponse      []byte
	PrepareFromEPSErr           error
	PrepareFromEPSCalls         []*SmfPrepareFromEPSCall
}

type SmfPrepareFromEPSCall struct {
	Supi              etsi.SUPI
	PDUSessionID      uint8
	EPSBearerIdentity uint8
	Dnn               string
	Snssai            *models.Snssai
}

func (f *fakeSmfSbi) TransferIdleTo5GS(context.Context, etsi.SUPI, uint8, uint8, string, *models.Snssai) (string, error) {
	return "", nil
}

func (f *fakeSmfSbi) PrepareSmContextFromEPS(_ context.Context, supi etsi.SUPI, pduSessionID, epsBearerIdentity uint8, dnn string, snssai *models.Snssai) (string, []byte, error) {
	f.PrepareFromEPSCalls = append(f.PrepareFromEPSCalls, &SmfPrepareFromEPSCall{
		Supi:              supi,
		PDUSessionID:      pduSessionID,
		EPSBearerIdentity: epsBearerIdentity,
		Dnn:               dnn,
		Snssai:            snssai,
	})

	if f.PrepareFromEPSErr != nil {
		return "", nil, f.PrepareFromEPSErr
	}

	return fmt.Sprintf("ref-from-eps-%d", pduSessionID), f.PrepareFromEPSResponse, nil
}

func (f *fakeSmfSbi) ActivateSmContext(_ context.Context, smContextRef string) ([]byte, error) {
	return nil, nil
}

func (f *fakeSmfSbi) ClearPagingSuppression(_ context.Context, _ etsi.SUPI, _ uint8) error {
	return nil
}

func (f *fakeSmfSbi) ReleaseSmContext(ctx context.Context, smContextRef string) error {
	f.ReleaseSmContextCalls = append(f.ReleaseSmContextCalls, smContextRef)
	return nil
}

func (f *fakeSmfSbi) UpdateSmContextXnHandoverPathSwitchReq(ctx context.Context, smContextRef string, n2Data []byte) ([]byte, error) {
	f.PathSwitchCalls = append(f.PathSwitchCalls, &SmfPathSwitchCall{
		SmContextRef: smContextRef,
		N2Data:       n2Data,
	})

	return f.PathSwitchResponse, f.PathSwitchErr
}

func (f *fakeSmfSbi) UpdateSmContextN2ModifyIndication(ctx context.Context, smContextRef string, n2Data []byte) ([]byte, error) {
	f.ModifyIndicationCalls = append(f.ModifyIndicationCalls, &SmfN2InfoCall{
		SmContextRef: smContextRef,
		N2Data:       n2Data,
	})

	return f.ModifyIndicationResponse, f.ModifyIndicationErr
}

func (f *fakeSmfSbi) UpdateSmContextXnHandoverFailed(_ context.Context, smContextRef string, n2Data []byte) error {
	f.HandoverFailedCalls = append(f.HandoverFailedCalls, &SmfHandoverFailedCall{
		SmContextRef: smContextRef,
		N2Data:       n2Data,
	})

	return f.HandoverFailedErr
}

func (f *fakeSmfSbi) UpdateSmContextN2HandoverFailed(_ context.Context, smContextRef string, n2Data []byte) error {
	f.N2HandoverFailedCalls = append(f.N2HandoverFailedCalls, &SmfN2InfoCall{SmContextRef: smContextRef, N2Data: n2Data})

	return f.N2HandoverFailedErr
}

func (f *fakeSmfSbi) UpdateSmContextN1Msg(ctx context.Context, smContextRef string, n1Msg []byte) (*smf.UpdateResult, error) {
	return nil, nil
}

func (f *fakeSmfSbi) CreateSmContext(ctx context.Context, supi etsi.SUPI, pduSessionID uint8, dnn string, snssai *models.Snssai, requestType fgs.RequestType, n1Msg []byte, _ uint8) (string, []byte, error) {
	return "", nil, nil
}

func (f *fakeSmfSbi) UpdateSmContextCauseDuplicatePDUSessionID(ctx context.Context, smContextRef string) ([]byte, error) {
	return nil, nil
}

func (f *fakeSmfSbi) DeactivateSmContext(_ context.Context, smContextRef string) error {
	f.DeactivateSmContextCalls = append(f.DeactivateSmContextCalls, smContextRef)
	return nil
}

func (f *fakeSmfSbi) UpdateSmContextN2InfoPduResSetupRsp(_ context.Context, smContextRef string, n2Data []byte) error {
	f.PduResSetupRspCalls = append(f.PduResSetupRspCalls, &SmfN2InfoCall{SmContextRef: smContextRef, N2Data: n2Data})
	return nil
}

func (f *fakeSmfSbi) UpdateSmContextN2InfoPduResSetupFail(_ context.Context, smContextRef string, n2Data []byte) error {
	f.PduResSetupFailCalls = append(f.PduResSetupFailCalls, &SmfN2InfoCall{SmContextRef: smContextRef, N2Data: n2Data})
	return nil
}

func (f *fakeSmfSbi) UpdateSmContextN2InfoPduResRelRsp(_ context.Context, smContextRef string) (bool, error) {
	f.PduResRelRspCalls = append(f.PduResRelRspCalls, smContextRef)
	return f.PduResRelRspRemoved, nil
}

func (f *fakeSmfSbi) UpdateSmContextN2HandoverPreparing(_ context.Context, smContextRef string, n2Data []byte) ([]byte, error) {
	f.N2HandoverPreparingCalls = append(f.N2HandoverPreparingCalls, smContextRef)
	f.N2HandoverPreparingData = append(f.N2HandoverPreparingData, &SmfN2InfoCall{SmContextRef: smContextRef, N2Data: n2Data})

	if err, ok := f.N2HandoverPreparingErrByRef[smContextRef]; ok {
		return nil, err
	}

	return nil, f.N2HandoverPreparingErr
}

func (f *fakeSmfSbi) UpdateSmContextN2HandoverPrepared(_ context.Context, smContextRef string, n2Data []byte) ([]byte, error) {
	f.N2HandoverPreparedCalls = append(f.N2HandoverPreparedCalls, smContextRef)
	f.N2HandoverPreparedData = append(f.N2HandoverPreparedData, &SmfN2InfoCall{SmContextRef: smContextRef, N2Data: n2Data})

	return nil, f.N2HandoverPreparedErr
}

func (f *fakeSmfSbi) UpdateSmContextN2HandoverComplete(_ context.Context, smContextRef string) error {
	f.N2HandoverCompleteCalls = append(f.N2HandoverCompleteCalls, smContextRef)
	return f.N2HandoverCompleteErr
}

func (f *fakeSmfSbi) UpdateSmContextN2HandoverCanceled(_ context.Context, smContextRef string) error {
	f.N2HandoverCanceledCalls = append(f.N2HandoverCanceledCalls, smContextRef)
	return nil
}

func (fdb *fakeDBInstance) GetOperator(ctx context.Context) (*db.Operator, error) {
	if fdb.OperatorErr != nil {
		return nil, fdb.OperatorErr
	}

	return fdb.Operator, nil
}

func (fdb *fakeDBInstance) GetDataNetworkByID(ctx context.Context, id string) (*db.DataNetwork, error) {
	return &db.DataNetwork{
		ID:   id,
		Name: "TestDataNetwork",
	}, nil
}

func (fdb *fakeDBInstance) GetNetworkSliceByID(_ context.Context, id string) (*db.NetworkSlice, error) {
	return &db.NetworkSlice{ID: id, Name: "TestSlice", Sst: 1}, nil
}

func (fdb *fakeDBInstance) ListNetworkSlicesByIDs(_ context.Context, ids []string) ([]db.NetworkSlice, error) {
	var out []db.NetworkSlice
	for _, id := range ids {
		out = append(out, db.NetworkSlice{ID: id, Name: "TestSlice", Sst: 1})
	}

	return out, nil
}

func (fdb *fakeDBInstance) GetSubscriber(ctx context.Context, imsi string) (*db.Subscriber, error) {
	return &db.Subscriber{
		Imsi: imsi,
	}, nil
}

func (fdb *fakeDBInstance) GetProfileByID(ctx context.Context, id string) (*db.Profile, error) {
	return &db.Profile{ID: id, Name: "TestProfile", Allow4G: true, Allow5G: !fdb.BarFrom5G, UeAmbrDownlink: "200 Mbps", UeAmbrUplink: "100 Mbps"}, nil
}

func (fdb *fakeDBInstance) ListAllNetworkSlices(ctx context.Context) ([]db.NetworkSlice, error) {
	if fdb.SlicesErr != nil {
		return nil, fdb.SlicesErr
	}

	if fdb.Slices != nil {
		return fdb.Slices, nil
	}

	return []db.NetworkSlice{{ID: "slice-1", Name: "default", Sst: 1}}, nil
}

func (fdb *fakeDBInstance) GetPolicyByProfileAndSlice(ctx context.Context, profileID, sliceID string) (*db.Policy, error) {
	return &db.Policy{ID: "policy-1", Name: "TestPolicy", ProfileID: profileID, SliceID: sliceID, DataNetworkID: "dn-1", SessionAmbrDownlink: "200 Mbps", SessionAmbrUplink: "100 Mbps"}, nil
}

func (fdb *fakeDBInstance) ListPoliciesByProfile(_ context.Context, _ string) ([]db.Policy, error) {
	return []db.Policy{{ID: "policy-1", Name: "TestPolicy", ProfileID: "profile-1", SliceID: "slice-1", DataNetworkID: "dn-1"}}, nil
}

func (fdb *fakeDBInstance) NodeID() int { return 0 }

type fakeNGAPSender struct {
	SentNGSetupFailures                []*ngap.NGSetupFailure
	SentNGSetupResponses               []*ngap.NGSetupResponse
	SentNGResetAcknowledges            []*ngap.NGResetAcknowledge
	SentHandoverRequests               []*ngap.HandoverRequest
	SentHandoverCommands               []*ngap.HandoverCommand
	SentErrorIndications               []*ngap.ErrorIndication
	SentHandoverPreparationFailures    []*ngap.HandoverPreparationFailure
	SentHandoverCancelAcknowledges     []*ngap.HandoverCancelAcknowledge
	SentUEContextReleaseCommands       []*ngap.UEContextReleaseCommand
	SentPathSwitchRequestFailures      []*ngap.PathSwitchRequestFailure
	SentPathSwitchRequestAcknowledges  []*ngap.PathSwitchRequestAcknowledge
	SentRanConfigurationUpdateAcks     []*ngap.RANConfigurationUpdateAcknowledge
	SentRanConfigurationUpdateFailures []*ngap.RANConfigurationUpdateFailure
	SentDownlinkRanConfigTransfers     []*ngap.DownlinkRANConfigurationTransfer
	SentDownlinkRanStatusTransfers     []*ngap.DownlinkRANStatusTransfer
	SentPDUSessionModifyConfirms       []*ngap.PDUSessionResourceModifyConfirm
	SentDownlinkNASTransport           []*ngap.DownlinkNASTransport
}

func capture[M any](bucket *[]*M, parse func([]byte) (*M, error), value []byte, name string) {
	m, err := parse(value)
	if err != nil {
		panic(fmt.Sprintf("fakeNGAPSender: parse %s: %v", name, err))
	}

	*bucket = append(*bucket, m)
}

func (fng *fakeNGAPSender) WriteMsg(b []byte, _ *sctp.SndRcvInfo) (int, error) {
	pdu, err := ngap.Unmarshal(b)
	if err != nil {
		panic(fmt.Sprintf("fakeNGAPSender: unmarshal NGAP PDU: %v", err))
	}

	switch m := pdu.(type) {
	case *ngap.InitiatingMessage:
		fng.captureInitiating(m)
	case *ngap.SuccessfulOutcome:
		fng.captureSuccessful(m)
	case *ngap.UnsuccessfulOutcome:
		fng.captureUnsuccessful(m)
	}

	return len(b), nil
}

func (fng *fakeNGAPSender) captureInitiating(m *ngap.InitiatingMessage) {
	switch m.ProcedureCode {
	case ngap.ProcErrorIndication:
		capture(&fng.SentErrorIndications, ngap.ParseErrorIndication, m.Value, "Error Indication")
	case ngap.ProcDownlinkRANStatusTransfer:
		capture(&fng.SentDownlinkRanStatusTransfers, ngap.ParseDownlinkRANStatusTransfer, m.Value, "Downlink RAN Status Transfer")
	case ngap.ProcDownlinkNASTransport:
		capture(&fng.SentDownlinkNASTransport, ngap.ParseDownlinkNASTransport, m.Value, "Downlink NAS Transport")
	case ngap.ProcUEContextRelease:
		capture(&fng.SentUEContextReleaseCommands, ngap.ParseUEContextReleaseCommand, m.Value, "UE Context Release Command")
	case ngap.ProcDownlinkRANConfigurationTransfer:
		capture(&fng.SentDownlinkRanConfigTransfers, ngap.ParseDownlinkRANConfigurationTransfer, m.Value, "Downlink RAN Configuration Transfer")
	case ngap.ProcHandoverResourceAllocation:
		capture(&fng.SentHandoverRequests, ngap.ParseHandoverRequest, m.Value, "Handover Request")
	}
}

func (fng *fakeNGAPSender) captureSuccessful(m *ngap.SuccessfulOutcome) {
	switch m.ProcedureCode {
	case ngap.ProcNGSetup:
		capture(&fng.SentNGSetupResponses, ngap.ParseNGSetupResponse, m.Value, "NG Setup Response")
	case ngap.ProcNGReset:
		capture(&fng.SentNGResetAcknowledges, ngap.ParseNGResetAcknowledge, m.Value, "NG Reset Acknowledge")
	case ngap.ProcRANConfigurationUpdate:
		capture(&fng.SentRanConfigurationUpdateAcks, ngap.ParseRANConfigurationUpdateAcknowledge, m.Value, "RAN Configuration Update Acknowledge")
	case ngap.ProcHandoverCancel:
		capture(&fng.SentHandoverCancelAcknowledges, ngap.ParseHandoverCancelAcknowledge, m.Value, "Handover Cancel Acknowledge")
	case ngap.ProcPDUSessionResourceModifyIndication:
		capture(&fng.SentPDUSessionModifyConfirms, ngap.ParsePDUSessionResourceModifyConfirm, m.Value, "PDU Session Resource Modify Confirm")
	case ngap.ProcHandoverPreparation:
		capture(&fng.SentHandoverCommands, ngap.ParseHandoverCommand, m.Value, "Handover Command")
	case ngap.ProcPathSwitchRequest:
		capture(&fng.SentPathSwitchRequestAcknowledges, ngap.ParsePathSwitchRequestAcknowledge, m.Value, "Path Switch Request Acknowledge")
	}
}

func (fng *fakeNGAPSender) captureUnsuccessful(m *ngap.UnsuccessfulOutcome) {
	switch m.ProcedureCode {
	case ngap.ProcNGSetup:
		capture(&fng.SentNGSetupFailures, ngap.ParseNGSetupFailure, m.Value, "NG Setup Failure")
	case ngap.ProcRANConfigurationUpdate:
		capture(&fng.SentRanConfigurationUpdateFailures, ngap.ParseRANConfigurationUpdateFailure, m.Value, "RAN Configuration Update Failure")
	case ngap.ProcPathSwitchRequest:
		capture(&fng.SentPathSwitchRequestFailures, ngap.ParsePathSwitchRequestFailure, m.Value, "Path Switch Request Failure")
	case ngap.ProcHandoverPreparation:
		capture(&fng.SentHandoverPreparationFailures, ngap.ParseHandoverPreparationFailure, m.Value, "Handover Preparation Failure")
	}
}

func newTestRadio(a *amf.AMF) *amf.Radio {
	sender := &fakeNGAPSender{}
	ran := &amf.Radio{
		Log:  logger.AmfLog,
		Conn: sender,
	}
	ran.BindAMFForTest(amf.New(nil, nil, nil))
	ran.BindAMFForTest(a)

	return ran
}

func newTestAMF() *amf.AMF {
	return amf.New(nil, nil, nil)
}

type NASCall struct {
	UeConn *amf.UeConn
	NASPDU []byte
}

type fakeNASHandler struct {
	Calls      []NASCall
	LeavesBare bool
}

func (f *fakeNASHandler) HandleNAS(_ context.Context, ue *amf.UeConn, nasPdu []byte) {
	f.Calls = append(f.Calls, NASCall{UeConn: ue, NASPDU: nasPdu})

	if !f.LeavesBare && ue.UeContext() == nil {
		ue.AMFForTest().AttachUeConn(amf.NewUeContext(), ue)
	}
}

func newTestAMFWithNAS(nasHandler *fakeNASHandler) *amf.AMF {
	a := newTestAMF()
	a.NAS = nasHandler

	return a
}
