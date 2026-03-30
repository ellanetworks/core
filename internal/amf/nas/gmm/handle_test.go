// Copyright 2026 Ella Networks

package gmm

import (
	"context"
	"fmt"

	"github.com/ellanetworks/core/etsi"
	"github.com/ellanetworks/core/internal/amf/ngap/send"
	"github.com/ellanetworks/core/internal/ausf"
	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/smf"
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

func (fdb *FakeDBInstance) GetSubscriber(ctx context.Context, imsi string) (*db.Subscriber, error) {
	return &db.Subscriber{
		Imsi: imsi,
	}, nil
}

func (fdb *FakeDBInstance) ListNetworkSlices(ctx context.Context) ([]db.NetworkSlice, error) {
	return []db.NetworkSlice{
		{ID: 1, Sst: 1, Name: "default"},
	}, nil
}

func (fdb *FakeDBInstance) ListProfileNetworkConfigs(ctx context.Context, profileID int) ([]db.ProfileNetworkConfig, error) {
	return []db.ProfileNetworkConfig{
		{ProfileID: profileID, DataNetworkID: 1, SessionAmbrUplink: "1 Gbps", SessionAmbrDownlink: "2 Gbps"},
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

type NGUEContextReleaseCommand struct {
	AmfUeNGAPID int64
	RanUeNGAPID int64
	cause       aper.Enumerated
}

type NGPDUSessionResourceReleaseCommand struct {
	AmfUeNGAPID int64
	RanUeNGAPID int64
	NasPdu      []byte
	List        ngapType.PDUSessionResourceToReleaseListRelCmd
}

type FakeNGAPSender struct {
	SentDownlinkNASTransport             []*NGDLNasTransport
	SentPDUSessionResourceSetupRequest   []*NGPDUSessionResourceSetupRequest
	SentInitialContextSetupRequest       []*NGInitialContextSetupRequest
	SentUEContextReleaseCommand          []*NGUEContextReleaseCommand
	SentPDUSessionResourceReleaseCommand []*NGPDUSessionResourceReleaseCommand
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
	fng.SentUEContextReleaseCommand = append(
		fng.SentUEContextReleaseCommand,
		&NGUEContextReleaseCommand{
			AmfUeNGAPID: amfUeNgapID,
			RanUeNGAPID: ranUeNgapID,
			cause:       cause,
		},
	)

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
	fng.SentPDUSessionResourceReleaseCommand = append(
		fng.SentPDUSessionResourceReleaseCommand,
		&NGPDUSessionResourceReleaseCommand{
			AmfUeNGAPID: amfUENgapID,
			RanUeNGAPID: ranUENgapID,
			NasPdu:      nasPdu,
			List:        pduSessionResourceReleasedList,
		},
	)

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
	Supi    etsi.SUPI
	Kseaf   string
	Error   error
	AvKgAka *ausf.AuthResult
}

func (a *FakeAusf) Authenticate(ctx context.Context, suci string, plmn models.PlmnID, resync *ausf.ResyncInfo) (*ausf.AuthResult, error) {
	if a.Error != nil {
		return nil, a.Error
	}

	return a.AvKgAka, nil
}

func (a *FakeAusf) Confirm(ctx context.Context, resStar string, suci string) (etsi.SUPI, string, error) {
	if a.Error != nil {
		return etsi.InvalidSUPI, "", a.Error
	}

	return a.Supi, a.Kseaf, nil
}

func mustSUPIFromPrefixed(s string) etsi.SUPI { //nolint:unparam
	supi, err := etsi.NewSUPIFromPrefixed(s)
	if err != nil {
		panic("mustSUPIFromPrefixed: " + err.Error())
	}

	return supi
}

type SmfActivateSmContextCall struct {
	SmContextRef string
}

type SmfReleaseSmContextCall struct {
	SmContextRef string
}

type FakeSmf struct {
	Error             error
	ReleasedSmContext []string

	// ActivateSmContext fields
	ActivateSmContextResponse []byte
	ActivateSmContextError    error
	ActivateSmContextCalls    []SmfActivateSmContextCall

	// ReleaseSmContext fields
	ReleaseSmContextError error
	ReleaseSmContextCalls []SmfReleaseSmContextCall

	// UpdateSmContextN1Msg fields
	UpdateN1MsgResponse *smf.UpdateResult
	UpdateN1MsgError    error
	UpdateN1MsgCalls    []SmfUpdateN1MsgCall

	// CreateSmContext fields
	CreateSmContextRef     string
	CreateSmContextErrResp []byte
	CreateSmContextError   error
	CreateSmContextCalls   []SmfCreateSmContextCall

	// UpdateSmContextCauseDuplicatePDUSessionID fields
	DuplicatePDUResponse []byte
	DuplicatePDUError    error
	DuplicatePDUCalls    []SmfDuplicatePDUCall
}

type SmfUpdateN1MsgCall struct {
	SmContextRef string
	N1Msg        []byte
}

type SmfCreateSmContextCall struct {
	Supi         etsi.SUPI
	PduSessionID uint8
	Dnn          string
	Snssai       *models.Snssai
	N1Msg        []byte
}

type SmfDuplicatePDUCall struct {
	SmContextRef string
}

func (s *FakeSmf) ActivateSmContext(_ context.Context, smContextRef string) ([]byte, error) {
	s.ActivateSmContextCalls = append(s.ActivateSmContextCalls, SmfActivateSmContextCall{
		SmContextRef: smContextRef,
	})

	if s.ActivateSmContextError != nil {
		return nil, s.ActivateSmContextError
	}

	if s.Error != nil {
		return nil, s.Error
	}

	resp := s.ActivateSmContextResponse
	if resp == nil {
		resp = []byte{}
	}

	return resp, nil
}

func (s *FakeSmf) ReleaseSmContext(ctx context.Context, smContextRef string) error {
	s.ReleaseSmContextCalls = append(s.ReleaseSmContextCalls, SmfReleaseSmContextCall{
		SmContextRef: smContextRef,
	})

	if s.ReleaseSmContextError != nil {
		return s.ReleaseSmContextError
	}

	if s.Error != nil {
		return s.Error
	}

	s.ReleasedSmContext = append(s.ReleasedSmContext, smContextRef)

	return nil
}

func (s *FakeSmf) UpdateSmContextXnHandoverPathSwitchReq(ctx context.Context, smContextRef string, n2Data []byte) ([]byte, error) {
	return nil, s.Error
}

func (s *FakeSmf) UpdateSmContextHandoverFailed(_ context.Context, smContextRef string, n2Data []byte) error {
	return s.Error
}

func (s *FakeSmf) UpdateSmContextN1Msg(ctx context.Context, smContextRef string, n1Msg []byte) (*smf.UpdateResult, error) {
	s.UpdateN1MsgCalls = append(s.UpdateN1MsgCalls, SmfUpdateN1MsgCall{
		SmContextRef: smContextRef,
		N1Msg:        n1Msg,
	})

	return s.UpdateN1MsgResponse, s.UpdateN1MsgError
}

func (s *FakeSmf) CreateSmContext(ctx context.Context, supi etsi.SUPI, pduSessionID uint8, dnn string, snssai *models.Snssai, n1Msg []byte) (string, []byte, error) {
	s.CreateSmContextCalls = append(s.CreateSmContextCalls, SmfCreateSmContextCall{
		Supi:         supi,
		PduSessionID: pduSessionID,
		Dnn:          dnn,
		Snssai:       snssai,
		N1Msg:        n1Msg,
	})

	return s.CreateSmContextRef, s.CreateSmContextErrResp, s.CreateSmContextError
}

func (s *FakeSmf) UpdateSmContextCauseDuplicatePDUSessionID(ctx context.Context, smContextRef string) ([]byte, error) {
	s.DuplicatePDUCalls = append(s.DuplicatePDUCalls, SmfDuplicatePDUCall{
		SmContextRef: smContextRef,
	})

	return s.DuplicatePDUResponse, s.DuplicatePDUError
}

func (s *FakeSmf) DeactivateSmContext(_ context.Context, _ string) error {
	return s.Error
}

func (s *FakeSmf) UpdateSmContextN2InfoPduResSetupRsp(_ context.Context, _ string, _ []byte) error {
	return s.Error
}

func (s *FakeSmf) UpdateSmContextN2InfoPduResSetupFail(_ context.Context, _ string, _ []byte) error {
	return s.Error
}

func (s *FakeSmf) UpdateSmContextN2InfoPduResRelRsp(_ context.Context, _ string) error {
	return s.Error
}

func (s *FakeSmf) UpdateSmContextN2HandoverPreparing(_ context.Context, _ string, _ []byte) ([]byte, error) {
	return nil, s.Error
}

func (s *FakeSmf) UpdateSmContextN2HandoverPrepared(_ context.Context, _ string, _ []byte) ([]byte, error) {
	return nil, s.Error
}

func (s *FakeSmf) GetSession(_ string) *smf.SMContext { return nil }

func (s *FakeSmf) SessionsByDNN(_ string) []*smf.SMContext { return nil }

func (s *FakeSmf) SessionCount() int { return 0 }

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
