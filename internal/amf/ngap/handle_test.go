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
	"github.com/free5gc/ngap/ngapConvert"
	"github.com/free5gc/ngap/ngapType"
)

type FakeSCTPConn struct{}

type NGSetupRequestOpts struct {
	Name  string
	GnbID string
	ID    int64
	Mcc   string
	Mnc   string
	Tac   string
	Sst   int32
	Sd    string
}

func buildNGSetupRequest(opts *NGSetupRequestOpts) (*ngapType.NGAPPDU, error) {
	if opts.Mcc == "" {
		return nil, fmt.Errorf("MCC is required to build NGSetupRequest")
	}

	if opts.Mnc == "" {
		return nil, fmt.Errorf("MNC is required to build NGSetupRequest")
	}

	plmnID, err := getMccAndMncInOctets(opts.Mcc, opts.Mnc)
	if err != nil {
		return nil, fmt.Errorf("could not get plmnID in octets: %v", err)
	}

	if opts.Sst == 0 {
		return nil, fmt.Errorf("SST is required to build NGSetupRequest")
	}

	sst, sd, err := getSliceInBytes(opts.Sst, opts.Sd)
	if err != nil {
		return nil, fmt.Errorf("could not get slice info in bytes: %v", err)
	}

	pdu := ngapType.NGAPPDU{}
	pdu.Present = ngapType.NGAPPDUPresentInitiatingMessage
	pdu.InitiatingMessage = new(ngapType.InitiatingMessage)

	initiatingMessage := pdu.InitiatingMessage
	initiatingMessage.ProcedureCode.Value = ngapType.ProcedureCodeNGSetup
	initiatingMessage.Criticality.Value = ngapType.CriticalityPresentReject

	initiatingMessage.Value.Present = ngapType.InitiatingMessagePresentNGSetupRequest
	initiatingMessage.Value.NGSetupRequest = new(ngapType.NGSetupRequest)

	nGSetupRequest := initiatingMessage.Value.NGSetupRequest
	nGSetupRequestIEs := &nGSetupRequest.ProtocolIEs

	ie := ngapType.NGSetupRequestIEs{}
	ie.Id.Value = ngapType.ProtocolIEIDGlobalRANNodeID
	ie.Criticality.Value = ngapType.CriticalityPresentReject
	ie.Value.Present = ngapType.NGSetupRequestIEsPresentGlobalRANNodeID
	ie.Value.GlobalRANNodeID = new(ngapType.GlobalRANNodeID)

	globalRANNodeID := ie.Value.GlobalRANNodeID
	globalRANNodeID.Present = ngapType.GlobalRANNodeIDPresentGlobalGNBID
	globalRANNodeID.GlobalGNBID = new(ngapType.GlobalGNBID)

	globalGNBID := globalRANNodeID.GlobalGNBID
	globalGNBID.PLMNIdentity.Value = plmnID
	globalGNBID.GNBID.Present = ngapType.GNBIDPresentGNBID
	globalGNBID.GNBID.GNBID = new(aper.BitString)

	gNBID := globalGNBID.GNBID.GNBID

	*gNBID = ngapConvert.HexToBitString(opts.GnbID, 24)

	nGSetupRequestIEs.List = append(nGSetupRequestIEs.List, ie)

	// RANNodeName
	ie = ngapType.NGSetupRequestIEs{}
	ie.Id.Value = ngapType.ProtocolIEIDRANNodeName
	ie.Criticality.Value = ngapType.CriticalityPresentIgnore
	ie.Value.Present = ngapType.NGSetupRequestIEsPresentRANNodeName
	ie.Value.RANNodeName = new(ngapType.RANNodeName)

	rANNodeName := ie.Value.RANNodeName
	rANNodeName.Value = opts.Name

	nGSetupRequestIEs.List = append(nGSetupRequestIEs.List, ie)

	if opts.Tac != "" {
		tac, err := hex.DecodeString(opts.Tac)
		if err != nil {
			return nil, fmt.Errorf("could not get tac in bytes: %v", err)
		}

		ie = ngapType.NGSetupRequestIEs{}
		ie.Id.Value = ngapType.ProtocolIEIDSupportedTAList
		ie.Criticality.Value = ngapType.CriticalityPresentReject
		ie.Value.Present = ngapType.NGSetupRequestIEsPresentSupportedTAList
		ie.Value.SupportedTAList = new(ngapType.SupportedTAList)

		supportedTAList := ie.Value.SupportedTAList

		supportedTAItem := ngapType.SupportedTAItem{}
		supportedTAItem.TAC.Value = tac

		broadcastPLMNList := &supportedTAItem.BroadcastPLMNList
		broadcastPLMNItem := ngapType.BroadcastPLMNItem{}
		broadcastPLMNItem.PLMNIdentity.Value = plmnID
		sliceSupportList := &broadcastPLMNItem.TAISliceSupportList
		sliceSupportItem := ngapType.SliceSupportItem{}
		sliceSupportItem.SNSSAI.SST.Value = sst

		if sd != nil {
			sliceSupportItem.SNSSAI.SD = new(ngapType.SD)
			sliceSupportItem.SNSSAI.SD.Value = sd
		}

		sliceSupportList.List = append(sliceSupportList.List, sliceSupportItem)

		broadcastPLMNList.List = append(broadcastPLMNList.List, broadcastPLMNItem)

		supportedTAList.List = append(supportedTAList.List, supportedTAItem)

		nGSetupRequestIEs.List = append(nGSetupRequestIEs.List, ie)
	}

	// PagingDRX
	ie = ngapType.NGSetupRequestIEs{}
	ie.Id.Value = ngapType.ProtocolIEIDDefaultPagingDRX
	ie.Criticality.Value = ngapType.CriticalityPresentIgnore
	ie.Value.Present = ngapType.NGSetupRequestIEsPresentDefaultPagingDRX
	ie.Value.DefaultPagingDRX = new(ngapType.PagingDRX)

	pagingDRX := ie.Value.DefaultPagingDRX
	pagingDRX.Value = ngapType.PagingDRXPresentV128

	nGSetupRequestIEs.List = append(nGSetupRequestIEs.List, ie)

	return &pdu, nil
}

type ResetType int

const (
	ResetTypePresentNGInterface ResetType = iota
	ResetTypePresentPartOfNGInterface
)

type NGInterface struct {
	RanUENgapID int64
	AmfUENgapID int64
}

type NGResetOpts struct {
	ResetType         ResetType
	PartOfNGInterface []NGInterface
}

func buildNGReset(opts *NGResetOpts) (*ngapType.NGAPPDU, error) {
	pdu := ngapType.NGAPPDU{
		Present: ngapType.NGAPPDUPresentInitiatingMessage,
		InitiatingMessage: &ngapType.InitiatingMessage{
			ProcedureCode: ngapType.ProcedureCode{
				Value: ngapType.ProcedureCodeNGReset,
			},
			Criticality: ngapType.Criticality{
				Value: ngapType.CriticalityPresentReject,
			},
			Value: ngapType.InitiatingMessageValue{
				Present: ngapType.InitiatingMessagePresentNGReset,
				NGReset: &ngapType.NGReset{},
			},
		},
	}

	nGResetIEs := &pdu.InitiatingMessage.Value.NGReset.ProtocolIEs

	ie := ngapType.NGResetIEs{
		Id: ngapType.ProtocolIEID{
			Value: ngapType.ProtocolIEIDResetType,
		},
		Criticality: ngapType.Criticality{
			Value: ngapType.CriticalityPresentReject,
		},
		Value: ngapType.NGResetIEsValue{
			Present: ngapType.NGResetIEsPresentResetType,
		},
	}

	switch opts.ResetType {
	case ResetTypePresentNGInterface:
		ie.Value.ResetType = &ngapType.ResetType{
			Present: ngapType.ResetTypePresentNGInterface,
			NGInterface: &ngapType.ResetAll{
				Value: ngapType.ResetAllPresentResetAll,
			},
		}
	case ResetTypePresentPartOfNGInterface:
		ie.Value.ResetType = &ngapType.ResetType{
			Present:           ngapType.ResetTypePresentPartOfNGInterface,
			PartOfNGInterface: &ngapType.UEAssociatedLogicalNGConnectionList{},
		}
		for _, ngInterface := range opts.PartOfNGInterface {
			ueAssociatedLogicalNGConnectionItem := ngapType.UEAssociatedLogicalNGConnectionItem{}
			ueAssociatedLogicalNGConnectionItem.RANUENGAPID = &ngapType.RANUENGAPID{
				Value: ngInterface.RanUENgapID,
			}
			ueAssociatedLogicalNGConnectionItem.AMFUENGAPID = &ngapType.AMFUENGAPID{
				Value: ngInterface.AmfUENgapID,
			}
			ie.Value.ResetType.PartOfNGInterface.List = append(ie.Value.ResetType.PartOfNGInterface.List, ueAssociatedLogicalNGConnectionItem)
		}
	default:
		return nil, fmt.Errorf("unsupported ResetType: %v", opts.ResetType)
	}

	nGResetIEs.List = append(nGResetIEs.List, ie)

	ie = ngapType.NGResetIEs{
		Id: ngapType.ProtocolIEID{
			Value: ngapType.ProtocolIEIDCause,
		},
		Criticality: ngapType.Criticality{
			Value: ngapType.CriticalityPresentIgnore,
		},
		Value: ngapType.NGResetIEsValue{
			Present: ngapType.NGResetIEsPresentCause,
			Cause: &ngapType.Cause{
				Present: ngapType.CausePresentMisc,
				Misc: &ngapType.CauseMisc{
					Value: ngapType.CauseMiscPresentHardwareFailure,
				},
			},
		},
	}

	nGResetIEs.List = append(nGResetIEs.List, ie)

	return &pdu, nil
}

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

type FakeNGAPSender struct {
	SentNGSetupFailures             []*NGSetupFailure
	SentNGSetupResponses            []*NGSetupResponse
	SentNGResetAcknowledges         []*NGResetAcknowledge
	SentHandoverRequests            []*HandoverRequest
	SentErrorIndications            []*ErrorIndication
	SentHandoverPreparationFailures []*HandoverPreparationFailure
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
	return nil
}

func (fng *FakeNGAPSender) SendInitialContextSetupRequest(ctx context.Context, amfUeNgapID int64, ranUeNgapID int64, ambrUplink string, ambrDownlink string, allowedNssai *models.Snssai, kgnb []byte, plmnID models.PlmnID, ueRadioCapability string, ueRadioCapabilityForPaging *models.UERadioCapabilityForPaging, ueSecurityCapability *nasType.UESecurityCapability, nasPdu []byte, pduSessionResourceSetupRequestList *ngapType.PDUSessionResourceSetupListCxtReq, supportedGUAMI *models.Guami) error {
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
	fng.SentHandoverRequests = append(fng.SentHandoverRequests, &HandoverRequest{
		AmfUeNgapID: amfUeNgapID,
	})

	return nil
}
