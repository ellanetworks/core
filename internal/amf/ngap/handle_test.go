// Copyright 2024 Ella Networks

package ngap_test

import (
	"context"
	"encoding/hex"
	"fmt"

	"github.com/ellanetworks/core/internal/amf/ngap/send"
	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/models"
	"github.com/free5gc/aper"
	"github.com/free5gc/nas/nasType"
	"github.com/free5gc/ngap/ngapType"
)

type FakeSCTPConn struct{}

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

type FakeDBInstance struct {
	Operator *db.Operator
}

type SmfPathSwitchCall struct {
	SmContextRef string
	N2Data       []byte
}

type SmfHandoverFailedCall struct {
	SmContextRef string
	N2Data       []byte
}

type FakeSmfSbi struct {
	PathSwitchResponse  []byte
	PathSwitchErr       error
	HandoverFailedErr   error
	PathSwitchCalls     []*SmfPathSwitchCall
	HandoverFailedCalls []*SmfHandoverFailedCall
}

func (f *FakeSmfSbi) ActivateSmContext(smContextRef string) ([]byte, error) {
	return nil, nil
}

func (f *FakeSmfSbi) ReleaseSmContext(ctx context.Context, smContextRef string) error {
	return nil
}

func (f *FakeSmfSbi) UpdateSmContextXnHandoverPathSwitchReq(ctx context.Context, smContextRef string, n2Data []byte) ([]byte, error) {
	f.PathSwitchCalls = append(f.PathSwitchCalls, &SmfPathSwitchCall{
		SmContextRef: smContextRef,
		N2Data:       n2Data,
	})

	return f.PathSwitchResponse, f.PathSwitchErr
}

func (f *FakeSmfSbi) UpdateSmContextHandoverFailed(smContextRef string, n2Data []byte) error {
	f.HandoverFailedCalls = append(f.HandoverFailedCalls, &SmfHandoverFailedCall{
		SmContextRef: smContextRef,
		N2Data:       n2Data,
	})

	return f.HandoverFailedErr
}

func (fdb *FakeDBInstance) GetOperator(ctx context.Context) (*db.Operator, error) {
	return fdb.Operator, nil
}

func (fdb *FakeDBInstance) GetDataNetworkByID(ctx context.Context, id int) (*db.DataNetwork, error) {
	return &db.DataNetwork{
		ID:   id,
		Name: "TestDataNetwork",
	}, nil
}

func (fdb *FakeDBInstance) GetPolicyByID(ctx context.Context, id int) (*db.Policy, error) {
	return &db.Policy{
		ID:   id,
		Name: "TestPolicy",
	}, nil
}

func (fdb *FakeDBInstance) GetSubscriber(ctx context.Context, imsi string) (*db.Subscriber, error) {
	return &db.Subscriber{
		Imsi: imsi,
	}, nil
}

type NGSetupFailure struct {
	Cause *ngapType.Cause
}

type NGSetupResponse struct {
	Guami               *models.Guami
	PlmnSupported       *models.PlmnSupportItem
	AmfName             string
	AmfRelativeCapacity int64
}

type NGResetAcknowledge struct {
	PartOfNGInterface *ngapType.UEAssociatedLogicalNGConnectionList
}

type ErrorIndication struct {
	Cause                  *ngapType.Cause
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
	AmfUeNgapID int64
	RanUeNgapID int64
	Container   ngapType.TargetToSourceTransparentContainer
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
	SupportedPLMN                     *models.PlmnSupportItem
}

type FakeNGAPSender struct {
	SentNGSetupFailures               []*NGSetupFailure
	SentNGSetupResponses              []*NGSetupResponse
	SentNGResetAcknowledges           []*NGResetAcknowledge
	SentHandoverRequests              []*HandoverRequest
	SentHandoverCommands              []*HandoverCommand
	SentErrorIndications              []*ErrorIndication
	SentHandoverPreparationFailures   []*HandoverPreparationFailure
	SentUEContextReleaseCommands      []*UEContextReleaseCommand
	SentPathSwitchRequestFailures     []*PathSwitchRequestFailure
	SentPathSwitchRequestAcknowledges []*PathSwitchRequestAcknowledge
}

func (fng *FakeNGAPSender) SendToRan(ctx context.Context, packet []byte, msgType send.NGAPProcedure) error {
	return nil
}

func (fng *FakeNGAPSender) SendNGSetupFailure(ctx context.Context, cause *ngapType.Cause) error {
	fng.SentNGSetupFailures = append(fng.SentNGSetupFailures, &NGSetupFailure{Cause: cause})
	return nil
}

func (fng *FakeNGAPSender) SendNGSetupResponse(ctx context.Context, guami *models.Guami, plmnSupported *models.PlmnSupportItem, amfName string, amfRelativeCapacity int64) error {
	fng.SentNGSetupResponses = append(fng.SentNGSetupResponses, &NGSetupResponse{
		Guami:               guami,
		PlmnSupported:       plmnSupported,
		AmfName:             amfName,
		AmfRelativeCapacity: amfRelativeCapacity,
	})

	return nil
}

func (fng *FakeNGAPSender) SendNGResetAcknowledge(ctx context.Context, partOfNGInterface *ngapType.UEAssociatedLogicalNGConnectionList) error {
	fng.SentNGResetAcknowledges = append(fng.SentNGResetAcknowledges, &NGResetAcknowledge{
		PartOfNGInterface: partOfNGInterface,
	})

	return nil
}

func (fng *FakeNGAPSender) SendErrorIndication(ctx context.Context, cause *ngapType.Cause, criticalityDiagnostics *ngapType.CriticalityDiagnostics) error {
	fng.SentErrorIndications = append(fng.SentErrorIndications, &ErrorIndication{
		Cause:                  cause,
		CriticalityDiagnostics: criticalityDiagnostics,
	})

	return nil
}

func (fng *FakeNGAPSender) SendRanConfigurationUpdateAcknowledge(ctx context.Context, criticalityDiagnostics *ngapType.CriticalityDiagnostics) error {
	return nil
}

func (fng *FakeNGAPSender) SendRanConfigurationUpdateFailure(ctx context.Context, cause ngapType.Cause, criticalityDiagnostics *ngapType.CriticalityDiagnostics) error {
	return nil
}

func (fng *FakeNGAPSender) SendDownlinkRanConfigurationTransfer(ctx context.Context, transfer *ngapType.SONConfigurationTransfer) error {
	return nil
}

func (fng *FakeNGAPSender) SendPathSwitchRequestFailure(ctx context.Context, amfUeNgapID int64, ranUeNgapID int64, pduSessionResourceReleasedList *ngapType.PDUSessionResourceReleasedListPSFail, criticalityDiagnostics *ngapType.CriticalityDiagnostics) error {
	fng.SentPathSwitchRequestFailures = append(fng.SentPathSwitchRequestFailures, &PathSwitchRequestFailure{
		AmfUeNgapID:                    amfUeNgapID,
		RanUeNgapID:                    ranUeNgapID,
		PduSessionResourceReleasedList: pduSessionResourceReleasedList,
		CriticalityDiagnostics:         criticalityDiagnostics,
	})

	return nil
}

func (fng *FakeNGAPSender) SendAMFStatusIndication(ctx context.Context, unavailableGUAMIList ngapType.UnavailableGUAMIList) error {
	return nil
}

func (fng *FakeNGAPSender) SendUEContextReleaseCommand(
	ctx context.Context,
	amfUeNgapID int64,
	ranUeNgapID int64,
	causePresent int,
	cause aper.Enumerated,
) error {
	fng.SentUEContextReleaseCommands = append(fng.SentUEContextReleaseCommands, &UEContextReleaseCommand{
		AmfUeNgapID:  amfUeNgapID,
		RanUeNgapID:  ranUeNgapID,
		CausePresent: causePresent,
		Cause:        cause,
	})

	return nil
}

func (fng *FakeNGAPSender) SendDownlinkNasTransport(ctx context.Context, amfUeNgapID int64, ranUeNgapID int64, nasPdu []byte, mobilityRestrictionList *ngapType.MobilityRestrictionList) error {
	return nil
}

func (fng *FakeNGAPSender) SendPDUSessionResourceReleaseCommand(ctx context.Context, amfUENgapID int64, ranUENgapID int64, nasPdu []byte, pduSessionResourceReleasedList ngapType.PDUSessionResourceToReleaseListRelCmd) error {
	return nil
}

func (fng *FakeNGAPSender) SendHandoverCancelAcknowledge(ctx context.Context, amfUENGAPID int64, ranUENGAPID int64) error {
	return nil
}

func (fng *FakeNGAPSender) SendPDUSessionResourceModifyConfirm(ctx context.Context, amfUENGAPID int64, ranUENGAPID int64, pduSessionResourceModifyConfirmList ngapType.PDUSessionResourceModifyListModCfm, pduSessionResourceFailedToModifyList ngapType.PDUSessionResourceFailedToModifyListModCfm) error {
	return nil
}

func (fng *FakeNGAPSender) SendPDUSessionResourceSetupRequest(ctx context.Context, amfUeNgapID int64, ranUeNgapID int64, ambrUplink string, ambrDownlink string, nasPdu []byte, pduSessionResourceSetupRequestList ngapType.PDUSessionResourceSetupListSUReq) error {
	return nil
}

func (fng *FakeNGAPSender) SendHandoverPreparationFailure(ctx context.Context, amfUeNgapID int64, ranUeNgapID int64, cause ngapType.Cause, criticalityDiagnostics *ngapType.CriticalityDiagnostics) error {
	fng.SentHandoverPreparationFailures = append(fng.SentHandoverPreparationFailures, &HandoverPreparationFailure{
		AmfUeNgapID: amfUeNgapID,
		RanUeNgapID: ranUeNgapID,
		Cause:       cause,
	})

	return nil
}

func (fng *FakeNGAPSender) SendLocationReportingControl(ctx context.Context, amfUENgapID int64, ranUENgapID int64, eventType ngapType.EventType) error {
	return nil
}

func (fng *FakeNGAPSender) SendHandoverCommand(ctx context.Context, amfUeNgapID int64, ranUeNgapID int64, handOverType ngapType.HandoverType, pduSessionResourceHandoverList ngapType.PDUSessionResourceHandoverList, pduSessionResourceToReleaseList ngapType.PDUSessionResourceToReleaseListHOCmd, container ngapType.TargetToSourceTransparentContainer) error {
	fng.SentHandoverCommands = append(fng.SentHandoverCommands, &HandoverCommand{
		AmfUeNgapID: amfUeNgapID,
		RanUeNgapID: ranUeNgapID,
		Container:   container,
	})

	return nil
}

func (fng *FakeNGAPSender) SendInitialContextSetupRequest(ctx context.Context, amfUeNgapID int64, ranUeNgapID int64, ambrUplink string, ambrDownlink string, allowedNssai *models.Snssai, kgnb []byte, plmnID models.PlmnID, ueRadioCapability string, ueRadioCapabilityForPaging *models.UERadioCapabilityForPaging, ueSecurityCapability *nasType.UESecurityCapability, nasPdu []byte, pduSessionResourceSetupRequestList *ngapType.PDUSessionResourceSetupListCxtReq, supportedGUAMI *models.Guami) error {
	return nil
}

func (fng *FakeNGAPSender) SendPathSwitchRequestAcknowledge(ctx context.Context, amfUeNgapID int64, ranUeNgapID int64, ueSecurityCapability *nasType.UESecurityCapability, ncc uint8, nh []byte, pduSessionResourceSwitchedList ngapType.PDUSessionResourceSwitchedList, pduSessionResourceReleasedList ngapType.PDUSessionResourceReleasedListPSAck, supportedPLMN *models.PlmnSupportItem) error {
	fng.SentPathSwitchRequestAcknowledges = append(fng.SentPathSwitchRequestAcknowledges, &PathSwitchRequestAcknowledge{
		AmfUeNgapID:                       amfUeNgapID,
		RanUeNgapID:                       ranUeNgapID,
		UESecurityCapability:              ueSecurityCapability,
		NCC:                               ncc,
		NH:                                nh,
		PDUSessionResourceSwitchedList:    pduSessionResourceSwitchedList,
		PDUSessionResourceReleasedListAck: pduSessionResourceReleasedList,
		SupportedPLMN:                     supportedPLMN,
	})

	return nil
}

func (fng *FakeNGAPSender) SendHandoverRequest(
	ctx context.Context,
	amfUeNgapID int64,
	handOverType ngapType.HandoverType,
	uplinkAmbr string,
	downlinkAmbr string,
	ueSecurityCapability *nasType.UESecurityCapability,
	ncc uint8,
	nh []byte,
	cause ngapType.Cause,
	pduSessionResourceSetupListHOReq ngapType.PDUSessionResourceSetupListHOReq,
	sourceToTargetTransparentContainer ngapType.SourceToTargetTransparentContainer,
	supportedPLMN *models.PlmnSupportItem,
	supportedGUAMI *models.Guami,
) error {
	fng.SentHandoverRequests = append(fng.SentHandoverRequests, &HandoverRequest{
		AmfUeNgapID: amfUeNgapID,
	})

	return nil
}
