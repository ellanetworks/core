// Copyright 2026 Ella Networks

package gmm

import (
	"context"
	"fmt"

	"github.com/ellanetworks/core/etsi"
	"github.com/ellanetworks/core/internal/amf/ngap/send"
	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/models"
	"github.com/free5gc/aper"
	"github.com/free5gc/nas/nasType"
	"github.com/free5gc/ngap/ngapType"
)

type FakeDBInstance struct {
	Operator *db.Operator
}

func (fdb *FakeDBInstance) GetOperator(ctx context.Context) (*db.Operator, error) {
	if fdb.Operator == nil {
		return nil, fmt.Errorf("could not get operator")
	}

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

type NGDLNasTransport struct {
	AmfUeNGAPID int64
	RanUeNGAPID int64
	NasPdu      []byte
}

type NGPDUSessionResourceSetupRequest struct {
	AmfUeNGAPID int64
	RanUeNGAPID int64
	AmbrUL      string
	AmbrDL      string
	NasPdu      []byte
	SuList      ngapType.PDUSessionResourceSetupListSUReq
}

type NGInitialContextSetupRequest struct {
	AmfUeNGAPID int64
	RanUeNGAPID int64
	AmbrUL      string
	AmbrDL      string
	NasPdu      []byte
	CtxList     ngapType.PDUSessionResourceSetupListCxtReq
}

type FakeNGAPSender struct {
	SentDownlinkNASTransport           []*NGDLNasTransport
	SentPDUSessionResourceSetupRequest []*NGPDUSessionResourceSetupRequest
	SentInitialContextSetupRequest     []*NGInitialContextSetupRequest
}

func (fng *FakeNGAPSender) SendToRan(ctx context.Context, packet []byte, msgType send.NGAPProcedure) error {
	return nil
}

func (fng *FakeNGAPSender) SendNGSetupFailure(ctx context.Context, cause *ngapType.Cause) error {
	return nil
}

func (fng *FakeNGAPSender) SendNGSetupResponse(ctx context.Context, guami *models.Guami, plmnSupported *models.PlmnSupportItem, amfName string, amfRelativeCapacity int64) error {
	return nil
}

func (fng *FakeNGAPSender) SendNGResetAcknowledge(ctx context.Context, partOfNGInterface *ngapType.UEAssociatedLogicalNGConnectionList) error {
	return nil
}

func (fng *FakeNGAPSender) SendErrorIndication(ctx context.Context, cause *ngapType.Cause, criticalityDiagnostics *ngapType.CriticalityDiagnostics) error {
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
	return nil
}

func (fng *FakeNGAPSender) SendDownlinkNasTransport(ctx context.Context, amfUeNgapID int64, ranUeNgapID int64, nasPdu []byte, mobilityRestrictionList *ngapType.MobilityRestrictionList) error {
	fng.SentDownlinkNASTransport = append(
		fng.SentDownlinkNASTransport,
		&NGDLNasTransport{
			AmfUeNGAPID: amfUeNgapID,
			RanUeNGAPID: ranUeNgapID,
			NasPdu:      nasPdu,
		},
	)

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
	fng.SentPDUSessionResourceSetupRequest = append(
		fng.SentPDUSessionResourceSetupRequest,
		&NGPDUSessionResourceSetupRequest{
			AmfUeNGAPID: amfUeNgapID,
			RanUeNGAPID: ranUeNgapID,
			AmbrUL:      ambrUplink,
			AmbrDL:      ambrDownlink,
			NasPdu:      nasPdu,
			SuList:      pduSessionResourceSetupRequestList,
		},
	)

	return nil
}

func (fng *FakeNGAPSender) SendHandoverPreparationFailure(ctx context.Context, amfUeNgapID int64, ranUeNgapID int64, cause ngapType.Cause, criticalityDiagnostics *ngapType.CriticalityDiagnostics) error {
	return nil
}

func (fng *FakeNGAPSender) SendLocationReportingControl(ctx context.Context, amfUENgapID int64, ranUENgapID int64, eventType ngapType.EventType) error {
	return nil
}

func (fng *FakeNGAPSender) SendHandoverCommand(ctx context.Context, amfUeNgapID int64, ranUeNgapID int64, handOverType ngapType.HandoverType, pduSessionResourceHandoverList ngapType.PDUSessionResourceHandoverList, pduSessionResourceToReleaseList ngapType.PDUSessionResourceToReleaseListHOCmd, container ngapType.TargetToSourceTransparentContainer) error {
	return nil
}

func (fng *FakeNGAPSender) SendInitialContextSetupRequest(ctx context.Context, amfUeNgapID int64, ranUeNgapID int64, ambrUplink string, ambrDownlink string, allowedNssai *models.Snssai, kgnb []byte, plmnID models.PlmnID, ueRadioCapability string, ueRadioCapabilityForPaging *models.UERadioCapabilityForPaging, ueSecurityCapability *nasType.UESecurityCapability, nasPdu []byte, pduSessionResourceSetupRequestList *ngapType.PDUSessionResourceSetupListCxtReq, supportedGUAMI *models.Guami) error {
	fng.SentInitialContextSetupRequest = append(
		fng.SentInitialContextSetupRequest,
		&NGInitialContextSetupRequest{
			AmfUeNGAPID: amfUeNgapID,
			RanUeNGAPID: ranUeNgapID,
			AmbrUL:      ambrUplink,
			AmbrDL:      ambrDownlink,
			NasPdu:      nasPdu,
			CtxList:     *pduSessionResourceSetupRequestList,
		},
	)

	return nil
}

func (fng *FakeNGAPSender) SendPathSwitchRequestAcknowledge(ctx context.Context, amfUeNgapID int64, ranUeNgapID int64, ueSecurityCapability *nasType.UESecurityCapability, ncc uint8, nh []byte, pduSessionResourceSwitchedList ngapType.PDUSessionResourceSwitchedList, pduSessionResourceReleasedList ngapType.PDUSessionResourceReleasedListPSAck, supportedPLMN *models.PlmnSupportItem) error {
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
	return nil
}

type FakeAusf struct {
	Supi    string
	Kseaf   string
	Error   error
	AvKgAka *models.Av5gAka
}

func (a *FakeAusf) UeAuthPostRequestProcedure(ctx context.Context, suci string, snName string, resyncInfo *models.ResynchronizationInfo) (*models.Av5gAka, error) {
	if a.Error != nil {
		return nil, a.Error
	}

	return a.AvKgAka, nil
}

func (a *FakeAusf) Auth5gAkaComfirmRequestProcedure(resStar string, suci string) (string, string, error) {
	if a.Error != nil {
		return "", "", a.Error
	}

	return a.Supi, a.Kseaf, nil
}

type FakeSmf struct {
	Error error
}

func (s FakeSmf) ActivateSmContext(smContextRef string) ([]byte, error) {
	if s.Error != nil {
		return nil, s.Error
	}

	return []byte{}, nil
}

func (s FakeSmf) ReleaseSmContext(ctx context.Context, smContextRef string) error {
	return nil
}

func mustTestGuti(mcc string, mnc string, amfid string, tmsi uint32) etsi.GUTI {
	t, err := etsi.NewTMSI(tmsi)
	if err != nil {
		panic("invalid tmsi")
	}

	guti, err := etsi.NewGUTI(mcc, mnc, amfid, t)
	if err != nil {
		panic("invalid guti")
	}

	return guti
}
