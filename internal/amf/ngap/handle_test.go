// Copyright 2024 Ella Networks

package ngap_test

import (
	"context"
	"encoding/hex"
	"fmt"

	"github.com/ellanetworks/core/etsi"
	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/amf/ngap/send"
	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/smf"
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
	Slices   []db.NetworkSlice
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
	*smf.SMF
	PathSwitchResponse  []byte
	PathSwitchErr       error
	HandoverFailedErr   error
	PathSwitchCalls     []*SmfPathSwitchCall
	HandoverFailedCalls []*SmfHandoverFailedCall
}

func (f *FakeSmfSbi) ActivateSmContext(_ context.Context, smContextRef string) ([]byte, error) {
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

func (f *FakeSmfSbi) UpdateSmContextHandoverFailed(_ context.Context, smContextRef string, n2Data []byte) error {
	f.HandoverFailedCalls = append(f.HandoverFailedCalls, &SmfHandoverFailedCall{
		SmContextRef: smContextRef,
		N2Data:       n2Data,
	})

	return f.HandoverFailedErr
}

func (f *FakeSmfSbi) UpdateSmContextN1Msg(ctx context.Context, smContextRef string, n1Msg []byte) (*smf.UpdateResult, error) {
	return nil, nil
}

func (f *FakeSmfSbi) CreateSmContext(ctx context.Context, supi etsi.SUPI, pduSessionID uint8, dnn string, snssai *models.Snssai, n1Msg []byte) (string, []byte, error) {
	return "", nil, nil
}

func (f *FakeSmfSbi) UpdateSmContextCauseDuplicatePDUSessionID(ctx context.Context, smContextRef string) ([]byte, error) {
	return nil, nil
}

func (f *FakeSmfSbi) DeactivateSmContext(_ context.Context, _ string) error {
	return nil
}

func (f *FakeSmfSbi) UpdateSmContextN2InfoPduResSetupRsp(_ context.Context, _ string, _ []byte) error {
	return nil
}

func (f *FakeSmfSbi) UpdateSmContextN2InfoPduResSetupFail(_ context.Context, _ string, _ []byte) error {
	return nil
}

func (f *FakeSmfSbi) UpdateSmContextN2InfoPduResRelRsp(_ context.Context, _ string) error {
	return nil
}

func (f *FakeSmfSbi) UpdateSmContextN2HandoverPreparing(_ context.Context, _ string, _ []byte) ([]byte, error) {
	return nil, nil
}

func (f *FakeSmfSbi) UpdateSmContextN2HandoverPrepared(_ context.Context, _ string, _ []byte) ([]byte, error) {
	return nil, nil
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

func (fdb *FakeDBInstance) GetNetworkSliceByID(_ context.Context, id int) (*db.NetworkSlice, error) {
	return &db.NetworkSlice{ID: id, Name: "TestSlice", Sst: 1}, nil
}

func (fdb *FakeDBInstance) ListNetworkSlicesByIDs(_ context.Context, ids []int) ([]db.NetworkSlice, error) {
	var out []db.NetworkSlice
	for _, id := range ids {
		out = append(out, db.NetworkSlice{ID: id, Name: "TestSlice", Sst: 1})
	}

	return out, nil
}

func (fdb *FakeDBInstance) GetSubscriber(ctx context.Context, imsi string) (*db.Subscriber, error) {
	return &db.Subscriber{
		Imsi: imsi,
	}, nil
}

func (fdb *FakeDBInstance) GetProfileByID(ctx context.Context, id int) (*db.Profile, error) {
	return &db.Profile{ID: id, Name: "TestProfile"}, nil
}

func (fdb *FakeDBInstance) ListAllNetworkSlices(ctx context.Context) ([]db.NetworkSlice, error) {
	if fdb.Slices != nil {
		return fdb.Slices, nil
	}

	return []db.NetworkSlice{{ID: 1, Name: "default", Sst: 1}}, nil
}

func (fdb *FakeDBInstance) GetPolicyByProfileAndSlice(ctx context.Context, profileID, sliceID int) (*db.Policy, error) {
	return &db.Policy{ID: 1, Name: "TestPolicy", ProfileID: profileID, SliceID: sliceID, DataNetworkID: 1}, nil
}

func (fdb *FakeDBInstance) ListPoliciesByProfile(_ context.Context, _ int) ([]db.Policy, error) {
	return []db.Policy{{ID: 1, Name: "TestPolicy", ProfileID: 1, SliceID: 1, DataNetworkID: 1}}, nil
}

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
	SnssaiList                        []models.Snssai
}

type FakeNGAPSender struct {
	SentNGSetupFailures                []*NGSetupFailure
	SentNGSetupResponses               []*NGSetupResponse
	SentNGResetAcknowledges            []*NGResetAcknowledge
	SentHandoverRequests               []*HandoverRequest
	SentHandoverCommands               []*HandoverCommand
	SentErrorIndications               []*ErrorIndication
	SentHandoverPreparationFailures    []*HandoverPreparationFailure
	SentUEContextReleaseCommands       []*UEContextReleaseCommand
	SentPathSwitchRequestFailures      []*PathSwitchRequestFailure
	SentPathSwitchRequestAcknowledges  []*PathSwitchRequestAcknowledge
	SentRanConfigurationUpdateAcks     []*RanConfigurationUpdateAcknowledge
	SentRanConfigurationUpdateFailures []*RanConfigurationUpdateFailure
}

func (fng *FakeNGAPSender) SendToRan(ctx context.Context, packet []byte, msgType send.NGAPProcedure) error {
	return nil
}

func (fng *FakeNGAPSender) SendNGSetupFailure(ctx context.Context, cause *ngapType.Cause) error {
	fng.SentNGSetupFailures = append(fng.SentNGSetupFailures, &NGSetupFailure{Cause: cause})
	return nil
}

func (fng *FakeNGAPSender) SendNGSetupResponse(ctx context.Context, guami *models.Guami, snssaiList []models.Snssai, amfName string, amfRelativeCapacity int64) error {
	fng.SentNGSetupResponses = append(fng.SentNGSetupResponses, &NGSetupResponse{
		Guami:               guami,
		SnssaiList:          snssaiList,
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
	fng.SentRanConfigurationUpdateAcks = append(fng.SentRanConfigurationUpdateAcks, &RanConfigurationUpdateAcknowledge{
		CriticalityDiagnostics: criticalityDiagnostics,
	})

	return nil
}

func (fng *FakeNGAPSender) SendRanConfigurationUpdateFailure(ctx context.Context, cause ngapType.Cause, criticalityDiagnostics *ngapType.CriticalityDiagnostics) error {
	fng.SentRanConfigurationUpdateFailures = append(fng.SentRanConfigurationUpdateFailures, &RanConfigurationUpdateFailure{
		Cause:                  cause,
		CriticalityDiagnostics: criticalityDiagnostics,
	})

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

func (fng *FakeNGAPSender) SendInitialContextSetupRequest(ctx context.Context, amfUeNgapID int64, ranUeNgapID int64, ambrUplink string, ambrDownlink string, allowedNssai []models.Snssai, kgnb []byte, plmnID models.PlmnID, ueRadioCapability string, ueRadioCapabilityForPaging *models.UERadioCapabilityForPaging, ueSecurityCapability *nasType.UESecurityCapability, nasPdu []byte, pduSessionResourceSetupRequestList *ngapType.PDUSessionResourceSetupListCxtReq, supportedGUAMI *models.Guami) error {
	return nil
}

func (fng *FakeNGAPSender) SendPathSwitchRequestAcknowledge(ctx context.Context, amfUeNgapID int64, ranUeNgapID int64, ueSecurityCapability *nasType.UESecurityCapability, ncc uint8, nh []byte, pduSessionResourceSwitchedList ngapType.PDUSessionResourceSwitchedList, pduSessionResourceReleasedList ngapType.PDUSessionResourceReleasedListPSAck, snssaiList []models.Snssai) error {
	fng.SentPathSwitchRequestAcknowledges = append(fng.SentPathSwitchRequestAcknowledges, &PathSwitchRequestAcknowledge{
		AmfUeNgapID:                       amfUeNgapID,
		RanUeNgapID:                       ranUeNgapID,
		UESecurityCapability:              ueSecurityCapability,
		NCC:                               ncc,
		NH:                                nh,
		PDUSessionResourceSwitchedList:    pduSessionResourceSwitchedList,
		PDUSessionResourceReleasedListAck: pduSessionResourceReleasedList,
		SnssaiList:                        snssaiList,
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
	snssaiList []models.Snssai,
	supportedGUAMI *models.Guami,
) error {
	fng.SentHandoverRequests = append(fng.SentHandoverRequests, &HandoverRequest{
		AmfUeNgapID: amfUeNgapID,
	})

	return nil
}

// newTestRadio creates a minimal Radio with a FakeNGAPSender for testing.
func newTestRadio() *amf.Radio {
	sender := &FakeNGAPSender{}
	ran := &amf.Radio{
		Log:           logger.AmfLog,
		NGAPSender:    sender,
		RanUEs:        make(map[int64]*amf.RanUe),
		SupportedTAIs: make([]amf.SupportedTAI, 0),
	}

	return ran
}

// newTestAMF creates a minimal AMF context for testing.
func newTestAMF() *amf.AMF {
	return amf.New(nil, nil, nil)
}
