// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap_test

import (
	"context"
	"encoding/hex"
	"fmt"

	"github.com/ellanetworks/core/etsi"
	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/amf/util"
	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/sctp"
	"github.com/ellanetworks/core/internal/smf"
	"github.com/free5gc/aper"
	"github.com/free5gc/nas/nasType"
	"github.com/free5gc/ngap"
	"github.com/free5gc/ngap/ngapType"
)

func getMccAndMncInOctets(mccStr string, mncStr string) ([]byte, error) {
	mcc := reverse(mccStr)
	mnc := reverse(mncStr)

	var res string

	if len(mnc) == 2 {
		res = fmt.Sprintf("%c%cf%c%c%c", mcc[1], mcc[2], mcc[0], mnc[0], mnc[1])
	} else {
		res = fmt.Sprintf("%c%c%c%c%c%c", mcc[1], mcc[2], mnc[2], mcc[0], mnc[0], mnc[1])
	}

	resu, err := hex.DecodeString(res)
	if err != nil {
		return nil, fmt.Errorf("could not decode mcc/mnc to octets: %v", err)
	}

	return resu, nil
}

func reverse(s string) string {
	var aux string

	for _, valor := range s {
		aux = string(valor) + aux
	}

	return aux
}

func getSliceInBytes(sst int32, sd string) ([]byte, []byte, error) {
	sstBytes := []byte{byte(sst)}

	if sd != "" {
		sdBytes, err := hex.DecodeString(sd)
		if err != nil {
			return sstBytes, nil, fmt.Errorf("could not decode sd to bytes: %v", err)
		}

		return sstBytes, sdBytes, nil
	}

	return sstBytes, nil, nil
}

type fakeDBInstance struct {
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
	PathSwitchResponse       []byte
	PathSwitchErr            error
	HandoverFailedErr        error
	PathSwitchCalls          []*SmfPathSwitchCall
	ModifyIndicationResponse []byte
	ModifyIndicationErr      error
	ModifyIndicationCalls    []*SmfN2InfoCall
	HandoverFailedCalls      []*SmfHandoverFailedCall
	PduResSetupRspCalls      []*SmfN2InfoCall
	PduResSetupFailCalls     []*SmfN2InfoCall
	PduResRelRspCalls        []string
	DeactivateSmContextCalls []string
	N2HandoverCompleteCalls  []string
	N2HandoverCompleteErr    error
	ReleaseSmContextCalls    []string
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

func (f *fakeSmfSbi) UpdateSmContextHandoverFailed(_ context.Context, smContextRef string, n2Data []byte) error {
	f.HandoverFailedCalls = append(f.HandoverFailedCalls, &SmfHandoverFailedCall{
		SmContextRef: smContextRef,
		N2Data:       n2Data,
	})

	return f.HandoverFailedErr
}

func (f *fakeSmfSbi) UpdateSmContextN1Msg(ctx context.Context, smContextRef string, n1Msg []byte) (*smf.UpdateResult, error) {
	return nil, nil
}

func (f *fakeSmfSbi) CreateSmContext(ctx context.Context, supi etsi.SUPI, pduSessionID uint8, dnn string, snssai *models.Snssai, n1Msg []byte) (string, []byte, error) {
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

func (f *fakeSmfSbi) UpdateSmContextN2InfoPduResRelRsp(_ context.Context, smContextRef string) error {
	f.PduResRelRspCalls = append(f.PduResRelRspCalls, smContextRef)
	return nil
}

func (f *fakeSmfSbi) UpdateSmContextN2HandoverPreparing(_ context.Context, _ string, _ []byte) ([]byte, error) {
	return nil, nil
}

func (f *fakeSmfSbi) UpdateSmContextN2HandoverPrepared(_ context.Context, _ string, _ []byte) ([]byte, error) {
	return nil, nil
}

func (f *fakeSmfSbi) UpdateSmContextN2HandoverComplete(_ context.Context, smContextRef string) error {
	f.N2HandoverCompleteCalls = append(f.N2HandoverCompleteCalls, smContextRef)
	return f.N2HandoverCompleteErr
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
	return &db.Profile{ID: id, Name: "TestProfile"}, nil
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
	return &db.Policy{ID: "policy-1", Name: "TestPolicy", ProfileID: profileID, SliceID: sliceID, DataNetworkID: "dn-1"}, nil
}

func (fdb *fakeDBInstance) ListPoliciesByProfile(_ context.Context, _ string) ([]db.Policy, error) {
	return []db.Policy{{ID: "policy-1", Name: "TestPolicy", ProfileID: "profile-1", SliceID: "slice-1", DataNetworkID: "dn-1"}}, nil
}

func (fdb *fakeDBInstance) NodeID() int { return 0 }

type NGSetupFailure struct {
	Cause *ngapType.Cause
}

type NGSetupResponse struct {
	Guami               *models.Guami
	SnssaiList          []models.Snssai
	AmfName             string
	AmfRelativeCapacity int64
}

type NGResetAcknowledge struct {
	PartOfNGInterface *ngapType.UEAssociatedLogicalNGConnectionList
}

type ErrorIndication struct {
	AmfUeNgapID            *int64
	RanUeNgapID            *int64
	Cause                  *ngapType.Cause
	CriticalityDiagnostics *ngapType.CriticalityDiagnostics
}

type RanConfigurationUpdateAcknowledge struct {
	CriticalityDiagnostics *ngapType.CriticalityDiagnostics
}

type RanConfigurationUpdateFailure struct {
	Cause                  ngapType.Cause
	CriticalityDiagnostics *ngapType.CriticalityDiagnostics
}

type HandoverPreparationFailure struct {
	AmfUeNgapID int64
	RanUeNgapID int64
	Cause       ngapType.Cause
}

type HandoverRequest struct {
	AmfUeNgapID int64
}

type HandoverCommand struct {
	AmfUeNgapID   int64
	RanUeNgapID   int64
	HandoverList  ngapType.PDUSessionResourceHandoverList
	ToReleaseList ngapType.PDUSessionResourceToReleaseListHOCmd
	Container     ngapType.TargetToSourceTransparentContainer
}

type UEContextReleaseCommand struct {
	AmfUeNgapID  int64
	RanUeNgapID  int64
	CausePresent int
	Cause        aper.Enumerated
}

type PathSwitchRequestFailure struct {
	AmfUeNgapID                    int64
	RanUeNgapID                    int64
	PduSessionResourceReleasedList *ngapType.PDUSessionResourceReleasedListPSFail
	CriticalityDiagnostics         *ngapType.CriticalityDiagnostics
}

type PathSwitchRequestAcknowledge struct {
	AmfUeNgapID                       int64
	RanUeNgapID                       int64
	UESecurityCapability              *nasType.UESecurityCapability
	NCC                               uint8
	NH                                []byte
	PDUSessionResourceSwitchedList    ngapType.PDUSessionResourceSwitchedList
	PDUSessionResourceReleasedListAck ngapType.PDUSessionResourceReleasedListPSAck
	SnssaiList                        []models.Snssai
}

type HandoverCancelAcknowledge struct {
	AmfUeNgapID int64
	RanUeNgapID int64
}

type DownlinkNASTransportMsg struct {
	AmfUeNgapID int64
	RanUeNgapID int64
	NasPdu      []byte
}

type fakeNGAPSender struct {
	SentNGSetupFailures                []*NGSetupFailure
	SentNGSetupResponses               []*NGSetupResponse
	SentNGResetAcknowledges            []*NGResetAcknowledge
	SentHandoverRequests               []*HandoverRequest
	SentHandoverCommands               []*HandoverCommand
	SentErrorIndications               []*ErrorIndication
	SentHandoverPreparationFailures    []*HandoverPreparationFailure
	SentHandoverCancelAcknowledges     []*HandoverCancelAcknowledge
	SentUEContextReleaseCommands       []*UEContextReleaseCommand
	SentPathSwitchRequestFailures      []*PathSwitchRequestFailure
	SentPathSwitchRequestAcknowledges  []*PathSwitchRequestAcknowledge
	SentRanConfigurationUpdateAcks     []*RanConfigurationUpdateAcknowledge
	SentRanConfigurationUpdateFailures []*RanConfigurationUpdateFailure
	SentDownlinkRanConfigTransfers     []*ngapType.SONConfigurationTransfer
	SentDownlinkRanStatusTransfers     []*ngapType.DownlinkRANStatusTransfer
	SentPDUSessionModifyConfirms       []PDUSessionModifyConfirm
	SentDownlinkNASTransport           []*DownlinkNASTransportMsg
}

type PDUSessionModifyConfirm struct {
	AmfUeNgapID                          int64
	RanUeNgapID                          int64
	PDUSessionResourceModifyConfirmList  ngapType.PDUSessionResourceModifyListModCfm
	PDUSessionResourceFailedToModifyList ngapType.PDUSessionResourceFailedToModifyListModCfm
}

// WriteMsg decodes the sent NGAP PDU and records it in the bucket matching its
// procedure, so tests assert on the outbound message the same way the sender's
// typed buckets did.
func (fng *fakeNGAPSender) WriteMsg(b []byte, _ *sctp.SndRcvInfo) (int, error) {
	pdu, err := ngap.Decoder(b)
	if err != nil {
		panic(fmt.Sprintf("fakeNGAPSender: decode NGAP PDU: %v", err))
	}

	switch pdu.Present {
	case ngapType.NGAPPDUPresentInitiatingMessage:
		fng.captureInitiating(pdu.InitiatingMessage)
	case ngapType.NGAPPDUPresentSuccessfulOutcome:
		fng.captureSuccessful(pdu.SuccessfulOutcome)
	case ngapType.NGAPPDUPresentUnsuccessfulOutcome:
		fng.captureUnsuccessful(pdu.UnsuccessfulOutcome)
	}

	return len(b), nil
}

func (fng *fakeNGAPSender) captureInitiating(m *ngapType.InitiatingMessage) {
	switch m.ProcedureCode.Value {
	case ngapType.ProcedureCodeErrorIndication:
		ei := &ErrorIndication{}

		for _, ie := range m.Value.ErrorIndication.ProtocolIEs.List {
			switch ie.Id.Value {
			case ngapType.ProtocolIEIDAMFUENGAPID:
				id := ie.Value.AMFUENGAPID.Value
				ei.AmfUeNgapID = &id
			case ngapType.ProtocolIEIDRANUENGAPID:
				id := ie.Value.RANUENGAPID.Value
				ei.RanUeNgapID = &id
			case ngapType.ProtocolIEIDCause:
				ei.Cause = ie.Value.Cause
			case ngapType.ProtocolIEIDCriticalityDiagnostics:
				ei.CriticalityDiagnostics = ie.Value.CriticalityDiagnostics
			}
		}

		fng.SentErrorIndications = append(fng.SentErrorIndications, ei)

	case ngapType.ProcedureCodeDownlinkRANStatusTransfer:
		fng.SentDownlinkRanStatusTransfers = append(fng.SentDownlinkRanStatusTransfers, m.Value.DownlinkRANStatusTransfer)

	case ngapType.ProcedureCodeDownlinkNASTransport:
		msg := &DownlinkNASTransportMsg{}

		for _, ie := range m.Value.DownlinkNASTransport.ProtocolIEs.List {
			switch ie.Id.Value {
			case ngapType.ProtocolIEIDAMFUENGAPID:
				msg.AmfUeNgapID = ie.Value.AMFUENGAPID.Value
			case ngapType.ProtocolIEIDRANUENGAPID:
				msg.RanUeNgapID = ie.Value.RANUENGAPID.Value
			case ngapType.ProtocolIEIDNASPDU:
				msg.NasPdu = ie.Value.NASPDU.Value
			}
		}

		fng.SentDownlinkNASTransport = append(fng.SentDownlinkNASTransport, msg)

	case ngapType.ProcedureCodeUEContextRelease:
		cmd := &UEContextReleaseCommand{}

		for _, ie := range m.Value.UEContextReleaseCommand.ProtocolIEs.List {
			switch ie.Id.Value {
			case ngapType.ProtocolIEIDUENGAPIDs:
				pair := ie.Value.UENGAPIDs.UENGAPIDPair
				if pair != nil {
					cmd.AmfUeNgapID = pair.AMFUENGAPID.Value
					cmd.RanUeNgapID = pair.RANUENGAPID.Value
				}
			case ngapType.ProtocolIEIDCause:
				cmd.CausePresent, cmd.Cause = causePresentAndValue(ie.Value.Cause)
			}
		}

		fng.SentUEContextReleaseCommands = append(fng.SentUEContextReleaseCommands, cmd)

	case ngapType.ProcedureCodeDownlinkRANConfigurationTransfer:
		for _, ie := range m.Value.DownlinkRANConfigurationTransfer.ProtocolIEs.List {
			if ie.Id.Value == ngapType.ProtocolIEIDSONConfigurationTransferDL {
				fng.SentDownlinkRanConfigTransfers = append(fng.SentDownlinkRanConfigTransfers, ie.Value.SONConfigurationTransferDL)
			}
		}

	case ngapType.ProcedureCodeHandoverResourceAllocation:
		req := &HandoverRequest{}

		for _, ie := range m.Value.HandoverRequest.ProtocolIEs.List {
			if ie.Id.Value == ngapType.ProtocolIEIDAMFUENGAPID {
				req.AmfUeNgapID = ie.Value.AMFUENGAPID.Value
			}
		}

		fng.SentHandoverRequests = append(fng.SentHandoverRequests, req)
	}
}

func (fng *fakeNGAPSender) captureSuccessful(m *ngapType.SuccessfulOutcome) {
	switch m.ProcedureCode.Value {
	case ngapType.ProcedureCodeNGSetup:
		fng.SentNGSetupResponses = append(fng.SentNGSetupResponses, decodeNGSetupResponse(m.Value.NGSetupResponse))

	case ngapType.ProcedureCodeNGReset:
		ack := &NGResetAcknowledge{}

		for _, ie := range m.Value.NGResetAcknowledge.ProtocolIEs.List {
			if ie.Id.Value == ngapType.ProtocolIEIDUEAssociatedLogicalNGConnectionList {
				ack.PartOfNGInterface = ie.Value.UEAssociatedLogicalNGConnectionList
			}
		}

		fng.SentNGResetAcknowledges = append(fng.SentNGResetAcknowledges, ack)

	case ngapType.ProcedureCodeRANConfigurationUpdate:
		ackMsg := &RanConfigurationUpdateAcknowledge{}

		for _, ie := range m.Value.RANConfigurationUpdateAcknowledge.ProtocolIEs.List {
			if ie.Id.Value == ngapType.ProtocolIEIDCriticalityDiagnostics {
				ackMsg.CriticalityDiagnostics = ie.Value.CriticalityDiagnostics
			}
		}

		fng.SentRanConfigurationUpdateAcks = append(fng.SentRanConfigurationUpdateAcks, ackMsg)

	case ngapType.ProcedureCodeHandoverCancel:
		ack := &HandoverCancelAcknowledge{}

		for _, ie := range m.Value.HandoverCancelAcknowledge.ProtocolIEs.List {
			switch ie.Id.Value {
			case ngapType.ProtocolIEIDAMFUENGAPID:
				ack.AmfUeNgapID = ie.Value.AMFUENGAPID.Value
			case ngapType.ProtocolIEIDRANUENGAPID:
				ack.RanUeNgapID = ie.Value.RANUENGAPID.Value
			}
		}

		fng.SentHandoverCancelAcknowledges = append(fng.SentHandoverCancelAcknowledges, ack)

	case ngapType.ProcedureCodePDUSessionResourceModifyIndication:
		confirm := PDUSessionModifyConfirm{}

		for _, ie := range m.Value.PDUSessionResourceModifyConfirm.ProtocolIEs.List {
			switch ie.Id.Value {
			case ngapType.ProtocolIEIDAMFUENGAPID:
				confirm.AmfUeNgapID = ie.Value.AMFUENGAPID.Value
			case ngapType.ProtocolIEIDRANUENGAPID:
				confirm.RanUeNgapID = ie.Value.RANUENGAPID.Value
			case ngapType.ProtocolIEIDPDUSessionResourceModifyListModCfm:
				confirm.PDUSessionResourceModifyConfirmList = *ie.Value.PDUSessionResourceModifyListModCfm
			case ngapType.ProtocolIEIDPDUSessionResourceFailedToModifyListModCfm:
				confirm.PDUSessionResourceFailedToModifyList = *ie.Value.PDUSessionResourceFailedToModifyListModCfm
			}
		}

		fng.SentPDUSessionModifyConfirms = append(fng.SentPDUSessionModifyConfirms, confirm)

	case ngapType.ProcedureCodeHandoverPreparation:
		cmd := &HandoverCommand{}

		for _, ie := range m.Value.HandoverCommand.ProtocolIEs.List {
			switch ie.Id.Value {
			case ngapType.ProtocolIEIDAMFUENGAPID:
				cmd.AmfUeNgapID = ie.Value.AMFUENGAPID.Value
			case ngapType.ProtocolIEIDRANUENGAPID:
				cmd.RanUeNgapID = ie.Value.RANUENGAPID.Value
			case ngapType.ProtocolIEIDPDUSessionResourceHandoverList:
				cmd.HandoverList = *ie.Value.PDUSessionResourceHandoverList
			case ngapType.ProtocolIEIDPDUSessionResourceToReleaseListHOCmd:
				cmd.ToReleaseList = *ie.Value.PDUSessionResourceToReleaseListHOCmd
			case ngapType.ProtocolIEIDTargetToSourceTransparentContainer:
				cmd.Container = *ie.Value.TargetToSourceTransparentContainer
			}
		}

		fng.SentHandoverCommands = append(fng.SentHandoverCommands, cmd)

	case ngapType.ProcedureCodePathSwitchRequest:
		fng.SentPathSwitchRequestAcknowledges = append(fng.SentPathSwitchRequestAcknowledges, decodePathSwitchAck(m.Value.PathSwitchRequestAcknowledge))
	}
}

func (fng *fakeNGAPSender) captureUnsuccessful(m *ngapType.UnsuccessfulOutcome) {
	switch m.ProcedureCode.Value {
	case ngapType.ProcedureCodeNGSetup:
		failure := &NGSetupFailure{}

		for _, ie := range m.Value.NGSetupFailure.ProtocolIEs.List {
			if ie.Id.Value == ngapType.ProtocolIEIDCause {
				failure.Cause = ie.Value.Cause
			}
		}

		fng.SentNGSetupFailures = append(fng.SentNGSetupFailures, failure)

	case ngapType.ProcedureCodeRANConfigurationUpdate:
		failure := &RanConfigurationUpdateFailure{}

		for _, ie := range m.Value.RANConfigurationUpdateFailure.ProtocolIEs.List {
			switch ie.Id.Value {
			case ngapType.ProtocolIEIDCause:
				failure.Cause = *ie.Value.Cause
			case ngapType.ProtocolIEIDCriticalityDiagnostics:
				failure.CriticalityDiagnostics = ie.Value.CriticalityDiagnostics
			}
		}

		fng.SentRanConfigurationUpdateFailures = append(fng.SentRanConfigurationUpdateFailures, failure)

	case ngapType.ProcedureCodePathSwitchRequest:
		failure := &PathSwitchRequestFailure{}

		for _, ie := range m.Value.PathSwitchRequestFailure.ProtocolIEs.List {
			switch ie.Id.Value {
			case ngapType.ProtocolIEIDAMFUENGAPID:
				failure.AmfUeNgapID = ie.Value.AMFUENGAPID.Value
			case ngapType.ProtocolIEIDRANUENGAPID:
				failure.RanUeNgapID = ie.Value.RANUENGAPID.Value
			case ngapType.ProtocolIEIDPDUSessionResourceReleasedListPSFail:
				failure.PduSessionResourceReleasedList = ie.Value.PDUSessionResourceReleasedListPSFail
			case ngapType.ProtocolIEIDCriticalityDiagnostics:
				failure.CriticalityDiagnostics = ie.Value.CriticalityDiagnostics
			}
		}

		fng.SentPathSwitchRequestFailures = append(fng.SentPathSwitchRequestFailures, failure)

	case ngapType.ProcedureCodeHandoverPreparation:
		failure := &HandoverPreparationFailure{}

		for _, ie := range m.Value.HandoverPreparationFailure.ProtocolIEs.List {
			switch ie.Id.Value {
			case ngapType.ProtocolIEIDAMFUENGAPID:
				failure.AmfUeNgapID = ie.Value.AMFUENGAPID.Value
			case ngapType.ProtocolIEIDRANUENGAPID:
				failure.RanUeNgapID = ie.Value.RANUENGAPID.Value
			case ngapType.ProtocolIEIDCause:
				failure.Cause = *ie.Value.Cause
			}
		}

		fng.SentHandoverPreparationFailures = append(fng.SentHandoverPreparationFailures, failure)
	}
}

// causePresentAndValue splits an NGAP Cause into its present discriminator and
// the enumerated value of the chosen cause group (TS 38.413).
func causePresentAndValue(cause *ngapType.Cause) (int, aper.Enumerated) {
	if cause == nil {
		return ngapType.CausePresentNothing, 0
	}

	switch cause.Present {
	case ngapType.CausePresentRadioNetwork:
		return cause.Present, cause.RadioNetwork.Value
	case ngapType.CausePresentTransport:
		return cause.Present, cause.Transport.Value
	case ngapType.CausePresentNas:
		return cause.Present, cause.Nas.Value
	case ngapType.CausePresentProtocol:
		return cause.Present, cause.Protocol.Value
	case ngapType.CausePresentMisc:
		return cause.Present, cause.Misc.Value
	default:
		return cause.Present, 0
	}
}

func decodeNGSetupResponse(m *ngapType.NGSetupResponse) *NGSetupResponse {
	out := &NGSetupResponse{}

	for _, ie := range m.ProtocolIEs.List {
		switch ie.Id.Value {
		case ngapType.ProtocolIEIDAMFName:
			out.AmfName = ie.Value.AMFName.Value
		case ngapType.ProtocolIEIDServedGUAMIList:
			if len(ie.Value.ServedGUAMIList.List) > 0 {
				guami := ie.Value.ServedGUAMIList.List[0].GUAMI
				plmn := util.PlmnIDToModels(guami.PLMNIdentity)
				out.Guami = &models.Guami{PlmnID: &plmn}
			}
		case ngapType.ProtocolIEIDRelativeAMFCapacity:
			out.AmfRelativeCapacity = ie.Value.RelativeAMFCapacity.Value
		case ngapType.ProtocolIEIDPLMNSupportList:
			for _, plmnItem := range ie.Value.PLMNSupportList.List {
				for _, slice := range plmnItem.SliceSupportList.List {
					out.SnssaiList = append(out.SnssaiList, util.SNssaiToModels(slice.SNSSAI))
				}
			}
		}
	}

	return out
}

func decodePathSwitchAck(m *ngapType.PathSwitchRequestAcknowledge) *PathSwitchRequestAcknowledge {
	out := &PathSwitchRequestAcknowledge{}

	for _, ie := range m.ProtocolIEs.List {
		switch ie.Id.Value {
		case ngapType.ProtocolIEIDAMFUENGAPID:
			out.AmfUeNgapID = ie.Value.AMFUENGAPID.Value
		case ngapType.ProtocolIEIDRANUENGAPID:
			out.RanUeNgapID = ie.Value.RANUENGAPID.Value
		case ngapType.ProtocolIEIDUESecurityCapabilities:
			out.UESecurityCapability = ngapUESecCapToNas(ie.Value.UESecurityCapabilities)
		case ngapType.ProtocolIEIDSecurityContext:
			out.NCC = uint8(ie.Value.SecurityContext.NextHopChainingCount.Value)
		case ngapType.ProtocolIEIDPDUSessionResourceSwitchedList:
			out.PDUSessionResourceSwitchedList = *ie.Value.PDUSessionResourceSwitchedList
		case ngapType.ProtocolIEIDPDUSessionResourceReleasedListPSAck:
			out.PDUSessionResourceReleasedListAck = *ie.Value.PDUSessionResourceReleasedListPSAck
		}
	}

	return out
}

// ngapUESecCapToNas rebuilds the NAS UE security capability from the NGAP IE the
// AMF sends in Path Switch Request Acknowledge (TS 33.501, mirrors the AMF's own
// ngapToNasUESecurityCapability).
func ngapUESecCapToNas(received *ngapType.UESecurityCapabilities) *nasType.UESecurityCapability {
	if received == nil {
		return nil
	}

	out := &nasType.UESecurityCapability{}
	out.SetLen(2)

	encByte := received.NRencryptionAlgorithms.Value.Bytes[0]
	intByte := received.NRintegrityProtectionAlgorithms.Value.Bytes[0]

	out.SetEA1_128_5G((encByte & 0x80) >> 7)
	out.SetEA2_128_5G((encByte & 0x40) >> 6)
	out.SetEA3_128_5G((encByte & 0x20) >> 5)
	out.SetIA1_128_5G((intByte & 0x80) >> 7)
	out.SetIA2_128_5G((intByte & 0x40) >> 6)
	out.SetIA3_128_5G((intByte & 0x20) >> 5)

	return out
}

// newTestRadio creates a minimal Radio with a sender, bound to a, so its UEs live
// in a's registry index. Pass the same AMF a handler is invoked with.
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

// newTestAMF creates a minimal AMF context for testing.
func newTestAMF() *amf.AMF {
	return amf.New(nil, nil, nil)
}

// NASCall records one invocation of the NAS handler.
type NASCall struct {
	UeConn *amf.UeConn
	NASPDU []byte
}

// fakeNASHandler records inbound NAS calls. By default it models a message that
// establishes a UE context — binding a fresh one to a bare connection, as a real
// registration would — so the NGAP layer keeps the connection. Set LeavesBare to
// model a message that establishes none (undecodable, or an identity the network
// cannot resolve), which the NGAP layer then releases.
type fakeNASHandler struct {
	Calls               []NASCall
	LeavesBare          bool
	ServiceRequest      bool // IsServiceRequest returns this (route to HandleServiceRequest)
	ServiceRequestCalls []NASCall
}

func (f *fakeNASHandler) HandleNAS(_ context.Context, ue *amf.UeConn, nasPdu []byte) {
	f.Calls = append(f.Calls, NASCall{UeConn: ue, NASPDU: nasPdu})

	if !f.LeavesBare && ue.UeContext() == nil {
		ue.AMFForTest().AttachUeConn(amf.NewUeContext(), ue)
	}
}

func (f *fakeNASHandler) IsServiceRequest(_ []byte) bool { return f.ServiceRequest }

func (f *fakeNASHandler) HandleServiceRequest(_ context.Context, ue *amf.UeConn, nasPdu []byte) {
	f.ServiceRequestCalls = append(f.ServiceRequestCalls, NASCall{UeConn: ue, NASPDU: nasPdu})

	if !f.LeavesBare && ue.UeContext() == nil {
		ue.AMFForTest().AttachUeConn(amf.NewUeContext(), ue)
	}
}

// newTestAMFWithNAS creates a minimal AMF with a fakeNASHandler wired in.
func newTestAMFWithNAS(nasHandler *fakeNASHandler) *amf.AMF {
	a := newTestAMF()
	a.NAS = nasHandler

	return a
}
