package send

import (
	"context"
	"fmt"

	"github.com/ellanetworks/core/internal/amf/util"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/models"
	"github.com/free5gc/aper"
	"github.com/free5gc/ngap"
	"github.com/free5gc/ngap/ngapConvert"
	"github.com/free5gc/ngap/ngapType"
)

func BuildNGSetupResponse(ctx context.Context, guami *models.Guami, plmnSupported *models.PlmnSupportItem, amfName string, amfRelativeCapacity int64) ([]byte, error) {
	var pdu ngapType.NGAPPDU

	pdu.Present = ngapType.NGAPPDUPresentSuccessfulOutcome
	pdu.SuccessfulOutcome = new(ngapType.SuccessfulOutcome)

	successfulOutcome := pdu.SuccessfulOutcome
	successfulOutcome.ProcedureCode.Value = ngapType.ProcedureCodeNGSetup
	successfulOutcome.Criticality.Value = ngapType.CriticalityPresentReject
	successfulOutcome.Value.Present = ngapType.SuccessfulOutcomePresentNGSetupResponse
	successfulOutcome.Value.NGSetupResponse = new(ngapType.NGSetupResponse)

	nGSetupResponse := successfulOutcome.Value.NGSetupResponse
	nGSetupResponseIEs := &nGSetupResponse.ProtocolIEs

	ie := ngapType.NGSetupResponseIEs{}
	ie.Id.Value = ngapType.ProtocolIEIDAMFName
	ie.Criticality.Value = ngapType.CriticalityPresentReject
	ie.Value.Present = ngapType.NGSetupResponseIEsPresentAMFName
	ie.Value.AMFName = new(ngapType.AMFName)

	aMFName := ie.Value.AMFName
	aMFName.Value = amfName

	nGSetupResponseIEs.List = append(nGSetupResponseIEs.List, ie)

	// ServedGUAMIList
	ie = ngapType.NGSetupResponseIEs{}
	ie.Id.Value = ngapType.ProtocolIEIDServedGUAMIList
	ie.Criticality.Value = ngapType.CriticalityPresentReject
	ie.Value.Present = ngapType.NGSetupResponseIEsPresentServedGUAMIList
	ie.Value.ServedGUAMIList = new(ngapType.ServedGUAMIList)

	servedGUAMIList := ie.Value.ServedGUAMIList

	plmnID, err := util.PlmnIDToNgap(*guami.PlmnID)
	if err != nil {
		return nil, fmt.Errorf("error converting PLMN ID to NGAP: %+v", err)
	}

	servedGUAMIItem := ngapType.ServedGUAMIItem{}

	servedGUAMIItem.GUAMI.PLMNIdentity = *plmnID
	regionID, setID, prtID := ngapConvert.AmfIdToNgap(guami.AmfID)
	servedGUAMIItem.GUAMI.AMFRegionID.Value = regionID
	servedGUAMIItem.GUAMI.AMFSetID.Value = setID
	servedGUAMIItem.GUAMI.AMFPointer.Value = prtID
	servedGUAMIList.List = append(servedGUAMIList.List, servedGUAMIItem)

	nGSetupResponseIEs.List = append(nGSetupResponseIEs.List, ie)

	// relativeAMFCapacity
	ie = ngapType.NGSetupResponseIEs{}
	ie.Id.Value = ngapType.ProtocolIEIDRelativeAMFCapacity
	ie.Criticality.Value = ngapType.CriticalityPresentIgnore
	ie.Value.Present = ngapType.NGSetupResponseIEsPresentRelativeAMFCapacity
	ie.Value.RelativeAMFCapacity = new(ngapType.RelativeAMFCapacity)
	relativeAMFCapacity := ie.Value.RelativeAMFCapacity
	relativeAMFCapacity.Value = amfRelativeCapacity

	nGSetupResponseIEs.List = append(nGSetupResponseIEs.List, ie)

	// ServedGUAMIList
	ie = ngapType.NGSetupResponseIEs{}
	ie.Id.Value = ngapType.ProtocolIEIDPLMNSupportList
	ie.Criticality.Value = ngapType.CriticalityPresentReject
	ie.Value.Present = ngapType.NGSetupResponseIEsPresentPLMNSupportList
	ie.Value.PLMNSupportList = new(ngapType.PLMNSupportList)

	pLMNSupportList := ie.Value.PLMNSupportList

	pLMNSupportItem := ngapType.PLMNSupportItem{}
	plmnID, err = util.PlmnIDToNgap(plmnSupported.PlmnID)
	if err != nil {
		return nil, fmt.Errorf("error converting PLMN ID to NGAP: %+v", err)
	}
	pLMNSupportItem.PLMNIdentity = *plmnID

	snssaiNgap, err := util.SNssaiToNgap(plmnSupported.SNssai)
	if err != nil {
		return nil, fmt.Errorf("error converting SNssai to NGAP: %+v", err)
	}

	sliceSupportItem := ngapType.SliceSupportItem{
		SNSSAI: snssaiNgap,
	}

	pLMNSupportItem.SliceSupportList.List = append(pLMNSupportItem.SliceSupportList.List, sliceSupportItem)

	pLMNSupportList.List = append(pLMNSupportList.List, pLMNSupportItem)
	nGSetupResponseIEs.List = append(nGSetupResponseIEs.List, ie)

	return ngap.Encoder(pdu)
}

func BuildNGSetupFailure(cause *ngapType.Cause) ([]byte, error) {
	var pdu ngapType.NGAPPDU
	pdu.Present = ngapType.NGAPPDUPresentUnsuccessfulOutcome
	pdu.UnsuccessfulOutcome = new(ngapType.UnsuccessfulOutcome)

	unsuccessfulOutcome := pdu.UnsuccessfulOutcome
	unsuccessfulOutcome.ProcedureCode.Value = ngapType.ProcedureCodeNGSetup
	unsuccessfulOutcome.Criticality.Value = ngapType.CriticalityPresentReject
	unsuccessfulOutcome.Value.Present = ngapType.UnsuccessfulOutcomePresentNGSetupFailure
	unsuccessfulOutcome.Value.NGSetupFailure = new(ngapType.NGSetupFailure)

	nGSetupFailure := unsuccessfulOutcome.Value.NGSetupFailure
	nGSetupFailureIEs := &nGSetupFailure.ProtocolIEs

	// Cause
	ie := ngapType.NGSetupFailureIEs{}
	ie.Id.Value = ngapType.ProtocolIEIDCause
	ie.Criticality.Value = ngapType.CriticalityPresentIgnore
	ie.Value.Present = ngapType.NGSetupFailureIEsPresentCause
	ie.Value.Cause = cause

	nGSetupFailureIEs.List = append(nGSetupFailureIEs.List, ie)

	return ngap.Encoder(pdu)
}

func BuildNGResetAcknowledge(partOfNGInterface *ngapType.UEAssociatedLogicalNGConnectionList) ([]byte, error) {
	var pdu ngapType.NGAPPDU

	pdu.Present = ngapType.NGAPPDUPresentSuccessfulOutcome
	pdu.SuccessfulOutcome = new(ngapType.SuccessfulOutcome)

	successfulOutcome := pdu.SuccessfulOutcome
	successfulOutcome.ProcedureCode.Value = ngapType.ProcedureCodeNGReset
	successfulOutcome.Criticality.Value = ngapType.CriticalityPresentReject

	successfulOutcome.Value.Present = ngapType.SuccessfulOutcomePresentNGResetAcknowledge
	successfulOutcome.Value.NGResetAcknowledge = new(ngapType.NGResetAcknowledge)

	nGResetAcknowledge := successfulOutcome.Value.NGResetAcknowledge
	nGResetAcknowledgeIEs := &nGResetAcknowledge.ProtocolIEs

	// UE-associated Logical NG-connection List (optional)
	if partOfNGInterface != nil && len(partOfNGInterface.List) > 0 {
		ie := ngapType.NGResetAcknowledgeIEs{}
		ie.Id.Value = ngapType.ProtocolIEIDUEAssociatedLogicalNGConnectionList
		ie.Criticality.Value = ngapType.CriticalityPresentIgnore
		ie.Value.Present = ngapType.NGResetAcknowledgeIEsPresentUEAssociatedLogicalNGConnectionList
		ie.Value.UEAssociatedLogicalNGConnectionList = new(ngapType.UEAssociatedLogicalNGConnectionList)

		uEAssociatedLogicalNGConnectionList := ie.Value.UEAssociatedLogicalNGConnectionList

		for _, item := range partOfNGInterface.List {
			if item.AMFUENGAPID == nil && item.RANUENGAPID == nil {
				logger.AmfLog.Warn("[Build NG Reset Ack] No AmfUeNgapID & RanUeNgapID")
				continue
			}

			uEAssociatedLogicalNGConnectionItem := ngapType.UEAssociatedLogicalNGConnectionItem{}

			if item.AMFUENGAPID != nil {
				uEAssociatedLogicalNGConnectionItem.AMFUENGAPID = new(ngapType.AMFUENGAPID)
				uEAssociatedLogicalNGConnectionItem.AMFUENGAPID = item.AMFUENGAPID
			}
			if item.RANUENGAPID != nil {
				uEAssociatedLogicalNGConnectionItem.RANUENGAPID = new(ngapType.RANUENGAPID)
				uEAssociatedLogicalNGConnectionItem.RANUENGAPID = item.RANUENGAPID
			}

			uEAssociatedLogicalNGConnectionList.List = append(uEAssociatedLogicalNGConnectionList.List, uEAssociatedLogicalNGConnectionItem)
		}

		nGResetAcknowledgeIEs.List = append(nGResetAcknowledgeIEs.List, ie)
	}

	return ngap.Encoder(pdu)
}

func BuildErrorIndication(amfUeNgapID, ranUeNgapID *int64, cause *ngapType.Cause,
	criticalityDiagnostics *ngapType.CriticalityDiagnostics,
) ([]byte, error) {
	var pdu ngapType.NGAPPDU

	pdu.Present = ngapType.NGAPPDUPresentInitiatingMessage
	pdu.InitiatingMessage = new(ngapType.InitiatingMessage)

	initiatingMessage := pdu.InitiatingMessage
	initiatingMessage.ProcedureCode.Value = ngapType.ProcedureCodeErrorIndication
	initiatingMessage.Criticality.Value = ngapType.CriticalityPresentIgnore

	initiatingMessage.Value.Present = ngapType.InitiatingMessagePresentErrorIndication
	initiatingMessage.Value.ErrorIndication = new(ngapType.ErrorIndication)

	errorIndication := initiatingMessage.Value.ErrorIndication
	errorIndicationIEs := &errorIndication.ProtocolIEs

	if cause == nil && criticalityDiagnostics == nil {
		logger.AmfLog.Error(
			"[Build Error Indication] shall contain at least either the Cause or the Criticality Diagnostics")
	}

	if amfUeNgapID != nil {
		ie := ngapType.ErrorIndicationIEs{}
		ie.Id.Value = ngapType.ProtocolIEIDAMFUENGAPID
		ie.Criticality.Value = ngapType.CriticalityPresentIgnore
		ie.Value.Present = ngapType.ErrorIndicationIEsPresentAMFUENGAPID
		ie.Value.AMFUENGAPID = new(ngapType.AMFUENGAPID)

		aMFUENGAPID := ie.Value.AMFUENGAPID
		aMFUENGAPID.Value = *amfUeNgapID

		errorIndicationIEs.List = append(errorIndicationIEs.List, ie)
	}

	if ranUeNgapID != nil {
		ie := ngapType.ErrorIndicationIEs{}
		ie.Id.Value = ngapType.ProtocolIEIDRANUENGAPID
		ie.Criticality.Value = ngapType.CriticalityPresentIgnore
		ie.Value.Present = ngapType.ErrorIndicationIEsPresentRANUENGAPID
		ie.Value.RANUENGAPID = new(ngapType.RANUENGAPID)

		rANUENGAPID := ie.Value.RANUENGAPID
		rANUENGAPID.Value = *ranUeNgapID

		errorIndicationIEs.List = append(errorIndicationIEs.List, ie)
	}

	if cause != nil {
		ie := ngapType.ErrorIndicationIEs{}
		ie.Id.Value = ngapType.ProtocolIEIDCause
		ie.Criticality.Value = ngapType.CriticalityPresentIgnore
		ie.Value.Present = ngapType.ErrorIndicationIEsPresentCause
		ie.Value.Cause = new(ngapType.Cause)

		ie.Value.Cause = cause

		errorIndicationIEs.List = append(errorIndicationIEs.List, ie)
	}

	if criticalityDiagnostics != nil {
		ie := ngapType.ErrorIndicationIEs{}
		ie.Id.Value = ngapType.ProtocolIEIDCriticalityDiagnostics
		ie.Criticality.Value = ngapType.CriticalityPresentIgnore
		ie.Value.Present = ngapType.ErrorIndicationIEsPresentCriticalityDiagnostics
		ie.Value.CriticalityDiagnostics = new(ngapType.CriticalityDiagnostics)

		ie.Value.CriticalityDiagnostics = criticalityDiagnostics

		errorIndicationIEs.List = append(errorIndicationIEs.List, ie)
	}

	return ngap.Encoder(pdu)
}

func BuildRanConfigurationUpdateAcknowledge(
	criticalityDiagnostics *ngapType.CriticalityDiagnostics,
) ([]byte, error) {
	// criticality ->from received node when received node can't comprehend the IE or missing IE

	var pdu ngapType.NGAPPDU
	pdu.Present = ngapType.NGAPPDUPresentSuccessfulOutcome
	pdu.SuccessfulOutcome = new(ngapType.SuccessfulOutcome)

	successfulOutcome := pdu.SuccessfulOutcome
	successfulOutcome.ProcedureCode.Value = ngapType.ProcedureCodeRANConfigurationUpdate
	successfulOutcome.Criticality.Value = ngapType.CriticalityPresentReject
	successfulOutcome.Value.Present = ngapType.SuccessfulOutcomePresentRANConfigurationUpdateAcknowledge
	successfulOutcome.Value.RANConfigurationUpdateAcknowledge = new(ngapType.RANConfigurationUpdateAcknowledge)

	rANConfigurationUpdateAcknowledge := successfulOutcome.Value.RANConfigurationUpdateAcknowledge
	rANConfigurationUpdateAcknowledgeIEs := &rANConfigurationUpdateAcknowledge.ProtocolIEs

	// Criticality Doagnostics(Optional)
	if criticalityDiagnostics != nil {
		ie := ngapType.RANConfigurationUpdateAcknowledgeIEs{}
		ie.Id.Value = ngapType.ProtocolIEIDCriticalityDiagnostics
		ie.Criticality.Value = ngapType.CriticalityPresentIgnore
		ie.Value.Present = ngapType.RANConfigurationUpdateAcknowledgeIEsPresentCriticalityDiagnostics
		ie.Value.CriticalityDiagnostics = new(ngapType.CriticalityDiagnostics)

		ie.Value.CriticalityDiagnostics = criticalityDiagnostics
		rANConfigurationUpdateAcknowledgeIEs.List = append(rANConfigurationUpdateAcknowledgeIEs.List, ie)
	}

	return ngap.Encoder(pdu)
}

func BuildRanConfigurationUpdateFailure(
	cause ngapType.Cause, criticalityDiagnostics *ngapType.CriticalityDiagnostics,
) ([]byte, error) {
	// criticality ->from received node when received node can't comprehend the IE or missing IE
	// If the AMF cannot accept the update,
	// it shall respond with a RAN CONFIGURATION UPDATE FAILURE message and appropriate cause value.

	var pdu ngapType.NGAPPDU
	pdu.Present = ngapType.NGAPPDUPresentUnsuccessfulOutcome
	pdu.UnsuccessfulOutcome = new(ngapType.UnsuccessfulOutcome)

	unsuccessfulOutcome := pdu.UnsuccessfulOutcome
	unsuccessfulOutcome.ProcedureCode.Value = ngapType.ProcedureCodeRANConfigurationUpdate
	unsuccessfulOutcome.Criticality.Value = ngapType.CriticalityPresentReject
	unsuccessfulOutcome.Value.Present = ngapType.UnsuccessfulOutcomePresentRANConfigurationUpdateFailure
	unsuccessfulOutcome.Value.RANConfigurationUpdateFailure = new(ngapType.RANConfigurationUpdateFailure)

	rANConfigurationUpdateFailure := unsuccessfulOutcome.Value.RANConfigurationUpdateFailure
	rANConfigurationUpdateFailureIEs := &rANConfigurationUpdateFailure.ProtocolIEs

	// Cause
	ie := ngapType.RANConfigurationUpdateFailureIEs{}
	ie.Id.Value = ngapType.ProtocolIEIDCause
	ie.Criticality.Value = ngapType.CriticalityPresentIgnore
	ie.Value.Present = ngapType.RANConfigurationUpdateFailureIEsPresentCause
	ie.Value.Cause = &cause

	rANConfigurationUpdateFailureIEs.List = append(rANConfigurationUpdateFailureIEs.List, ie)

	// Time To Wait(Optional)
	ie = ngapType.RANConfigurationUpdateFailureIEs{}
	ie.Id.Value = ngapType.ProtocolIEIDTimeToWait
	ie.Criticality.Value = ngapType.CriticalityPresentIgnore
	ie.Value.Present = ngapType.RANConfigurationUpdateFailureIEsPresentTimeToWait
	ie.Value.TimeToWait = new(ngapType.TimeToWait)

	timeToWait := ie.Value.TimeToWait
	timeToWait.Value = ngapType.TimeToWaitPresentV1s

	rANConfigurationUpdateFailureIEs.List = append(rANConfigurationUpdateFailureIEs.List, ie)

	// Criticality Doagnostics(Optional)
	if criticalityDiagnostics != nil {
		ie = ngapType.RANConfigurationUpdateFailureIEs{}
		ie.Id.Value = ngapType.ProtocolIEIDCriticalityDiagnostics
		ie.Criticality.Value = ngapType.CriticalityPresentIgnore
		ie.Value.Present = ngapType.RANConfigurationUpdateFailureIEsPresentCriticalityDiagnostics
		ie.Value.CriticalityDiagnostics = new(ngapType.CriticalityDiagnostics)

		ie.Value.CriticalityDiagnostics = criticalityDiagnostics
		rANConfigurationUpdateFailureIEs.List = append(rANConfigurationUpdateFailureIEs.List, ie)
	}

	return ngap.Encoder(pdu)
}

func BuildDownlinkRanConfigurationTransfer(
	sONConfigurationTransfer *ngapType.SONConfigurationTransfer,
) ([]byte, error) {
	// sONConfigurationTransfer = sONConfigurationTransfer from uplink Ran Configuration Transfer

	var pdu ngapType.NGAPPDU
	pdu.Present = ngapType.NGAPPDUPresentInitiatingMessage
	pdu.InitiatingMessage = new(ngapType.InitiatingMessage)

	initiatingMessage := pdu.InitiatingMessage
	initiatingMessage.ProcedureCode.Value = ngapType.ProcedureCodeDownlinkRANConfigurationTransfer
	initiatingMessage.Criticality.Value = ngapType.CriticalityPresentIgnore
	initiatingMessage.Value.Present = ngapType.InitiatingMessagePresentDownlinkRANConfigurationTransfer
	initiatingMessage.Value.DownlinkRANConfigurationTransfer = new(ngapType.DownlinkRANConfigurationTransfer)

	downlinkRANConfigurationTransfer := initiatingMessage.Value.DownlinkRANConfigurationTransfer
	downlinkRANConfigurationTransferIEs := &downlinkRANConfigurationTransfer.ProtocolIEs

	// SON Configuration Transfer [optional]
	if sONConfigurationTransfer != nil {
		ie := ngapType.DownlinkRANConfigurationTransferIEs{}
		ie.Id.Value = ngapType.ProtocolIEIDSONConfigurationTransferDL
		ie.Criticality.Value = ngapType.CriticalityPresentIgnore
		ie.Value.Present = ngapType.DownlinkRANConfigurationTransferIEsPresentSONConfigurationTransferDL
		ie.Value.SONConfigurationTransferDL = new(ngapType.SONConfigurationTransfer)

		ie.Value.SONConfigurationTransferDL = sONConfigurationTransfer

		downlinkRANConfigurationTransferIEs.List = append(downlinkRANConfigurationTransferIEs.List, ie)
	}

	return ngap.Encoder(pdu)
}

// pduSessionResourceReleasedList: provided by AMF, and the transfer data is from SMF
// criticalityDiagnostics: from received node when received not comprehended IE or missing IE
func BuildPathSwitchRequestFailure(
	amfUeNgapID,
	ranUeNgapID int64,
	pduSessionResourceReleasedList *ngapType.PDUSessionResourceReleasedListPSFail,
	criticalityDiagnostics *ngapType.CriticalityDiagnostics,
) ([]byte, error) {
	var pdu ngapType.NGAPPDU
	pdu.Present = ngapType.NGAPPDUPresentUnsuccessfulOutcome
	pdu.UnsuccessfulOutcome = new(ngapType.UnsuccessfulOutcome)

	unsuccessfulOutcome := pdu.UnsuccessfulOutcome
	unsuccessfulOutcome.ProcedureCode.Value = ngapType.ProcedureCodePathSwitchRequest
	unsuccessfulOutcome.Criticality.Value = ngapType.CriticalityPresentReject

	unsuccessfulOutcome.Value.Present = ngapType.UnsuccessfulOutcomePresentPathSwitchRequestFailure
	unsuccessfulOutcome.Value.PathSwitchRequestFailure = new(ngapType.PathSwitchRequestFailure)

	pathSwitchRequestFailure := unsuccessfulOutcome.Value.PathSwitchRequestFailure
	pathSwitchRequestFailureIEs := &pathSwitchRequestFailure.ProtocolIEs

	// AMF UE NGAP ID
	ie := ngapType.PathSwitchRequestFailureIEs{}
	ie.Id.Value = ngapType.ProtocolIEIDAMFUENGAPID
	ie.Criticality.Value = ngapType.CriticalityPresentIgnore
	ie.Value.Present = ngapType.PathSwitchRequestFailureIEsPresentAMFUENGAPID
	ie.Value.AMFUENGAPID = new(ngapType.AMFUENGAPID)

	aMFUENGAPID := ie.Value.AMFUENGAPID
	aMFUENGAPID.Value = amfUeNgapID

	pathSwitchRequestFailureIEs.List = append(pathSwitchRequestFailureIEs.List, ie)

	// RAN UE NGAP ID
	ie = ngapType.PathSwitchRequestFailureIEs{}
	ie.Id.Value = ngapType.ProtocolIEIDRANUENGAPID
	ie.Criticality.Value = ngapType.CriticalityPresentIgnore
	ie.Value.Present = ngapType.PathSwitchRequestFailureIEsPresentRANUENGAPID
	ie.Value.RANUENGAPID = new(ngapType.RANUENGAPID)

	rANUENGAPID := ie.Value.RANUENGAPID
	rANUENGAPID.Value = ranUeNgapID

	pathSwitchRequestFailureIEs.List = append(pathSwitchRequestFailureIEs.List, ie)

	// PDU Session Resource Released List
	if pduSessionResourceReleasedList != nil {
		ie = ngapType.PathSwitchRequestFailureIEs{}
		ie.Id.Value = ngapType.ProtocolIEIDPDUSessionResourceReleasedListPSFail
		ie.Criticality.Value = ngapType.CriticalityPresentIgnore
		ie.Value.Present = ngapType.PathSwitchRequestFailureIEsPresentPDUSessionResourceReleasedListPSFail
		ie.Value.PDUSessionResourceReleasedListPSFail = pduSessionResourceReleasedList
		pathSwitchRequestFailureIEs.List = append(pathSwitchRequestFailureIEs.List, ie)
	}

	if criticalityDiagnostics != nil {
		ie = ngapType.PathSwitchRequestFailureIEs{}
		ie.Id.Value = ngapType.ProtocolIEIDCriticalityDiagnostics
		ie.Criticality.Value = ngapType.CriticalityPresentIgnore
		ie.Value.Present = ngapType.PathSwitchRequestFailureIEsPresentCriticalityDiagnostics
		ie.Value.CriticalityDiagnostics = criticalityDiagnostics
		pathSwitchRequestFailureIEs.List = append(pathSwitchRequestFailureIEs.List, ie)
	}

	return ngap.Encoder(pdu)
}

// An AMF shall be able to instruct other peer CP NFs, subscribed to receive such a notification,
// that it will be unavailable on this AMF and its corresponding target AMF(s).
// If CP NF does not subscribe to receive AMF unavailable notification, the CP NF may attempt
// forwarding the transaction towards the old AMF and detect that the AMF is unavailable. When
// it detects unavailable, it marks the AMF and its associated GUAMI(s) as unavailable.
// Defined in 23.501 5.21.2.2.2
func BuildAMFStatusIndication(unavailableGUAMIList ngapType.UnavailableGUAMIList) ([]byte, error) {
	var pdu ngapType.NGAPPDU

	pdu.Present = ngapType.NGAPPDUPresentInitiatingMessage
	pdu.InitiatingMessage = new(ngapType.InitiatingMessage)

	initiatingMessage := pdu.InitiatingMessage
	initiatingMessage.ProcedureCode.Value = ngapType.ProcedureCodeAMFStatusIndication
	initiatingMessage.Criticality.Value = ngapType.CriticalityPresentIgnore

	initiatingMessage.Value.Present = ngapType.InitiatingMessagePresentAMFStatusIndication
	initiatingMessage.Value.AMFStatusIndication = new(ngapType.AMFStatusIndication)

	aMFStatusIndication := initiatingMessage.Value.AMFStatusIndication
	aMFStatusIndicationIEs := &aMFStatusIndication.ProtocolIEs

	//	Unavailable GUAMI List
	ie := ngapType.AMFStatusIndicationIEs{}
	ie.Id.Value = ngapType.ProtocolIEIDUnavailableGUAMIList
	ie.Criticality.Value = ngapType.CriticalityPresentReject
	ie.Value.Present = ngapType.AMFStatusIndicationIEsPresentUnavailableGUAMIList
	ie.Value.UnavailableGUAMIList = new(ngapType.UnavailableGUAMIList)

	ie.Value.UnavailableGUAMIList = &unavailableGUAMIList

	aMFStatusIndicationIEs.List = append(aMFStatusIndicationIEs.List, ie)

	return ngap.Encoder(pdu)
}

func BuildPDUSessionResourceReleaseCommand(amfUENgapID int64, ranUENgapID int64, nasPdu []byte, pduSessionResourceReleasedList ngapType.PDUSessionResourceToReleaseListRelCmd) ([]byte, error) {
	var pdu ngapType.NGAPPDU
	pdu.Present = ngapType.NGAPPDUPresentInitiatingMessage
	pdu.InitiatingMessage = new(ngapType.InitiatingMessage)

	initiatingMessage := pdu.InitiatingMessage
	initiatingMessage.ProcedureCode.Value = ngapType.ProcedureCodePDUSessionResourceRelease
	initiatingMessage.Criticality.Value = ngapType.CriticalityPresentReject
	initiatingMessage.Value.Present = ngapType.InitiatingMessagePresentPDUSessionResourceReleaseCommand
	initiatingMessage.Value.PDUSessionResourceReleaseCommand = new(ngapType.PDUSessionResourceReleaseCommand)

	pDUSessionResourceReleaseCommand := initiatingMessage.Value.PDUSessionResourceReleaseCommand
	PDUSessionResourceReleaseCommandIEs := &pDUSessionResourceReleaseCommand.ProtocolIEs

	// AMFUENGAPID
	ie := ngapType.PDUSessionResourceReleaseCommandIEs{}
	ie.Id.Value = ngapType.ProtocolIEIDAMFUENGAPID
	ie.Criticality.Value = ngapType.CriticalityPresentReject
	ie.Value.Present = ngapType.PDUSessionResourceReleaseCommandIEsPresentAMFUENGAPID
	ie.Value.AMFUENGAPID = new(ngapType.AMFUENGAPID)

	aMFUENGAPID := ie.Value.AMFUENGAPID
	aMFUENGAPID.Value = amfUENgapID

	PDUSessionResourceReleaseCommandIEs.List = append(PDUSessionResourceReleaseCommandIEs.List, ie)

	// RANUENGAPID
	ie = ngapType.PDUSessionResourceReleaseCommandIEs{}
	ie.Id.Value = ngapType.ProtocolIEIDRANUENGAPID
	ie.Criticality.Value = ngapType.CriticalityPresentReject
	ie.Value.Present = ngapType.PDUSessionResourceReleaseCommandIEsPresentRANUENGAPID
	ie.Value.RANUENGAPID = new(ngapType.RANUENGAPID)

	rANUENGAPID := ie.Value.RANUENGAPID
	rANUENGAPID.Value = ranUENgapID

	PDUSessionResourceReleaseCommandIEs.List = append(PDUSessionResourceReleaseCommandIEs.List, ie)

	// NAS-PDU (optional)
	if nasPdu != nil {
		ie = ngapType.PDUSessionResourceReleaseCommandIEs{}
		ie.Id.Value = ngapType.ProtocolIEIDNASPDU
		ie.Criticality.Value = ngapType.CriticalityPresentIgnore
		ie.Value.Present = ngapType.PDUSessionResourceReleaseCommandIEsPresentNASPDU
		ie.Value.NASPDU = new(ngapType.NASPDU)

		ie.Value.NASPDU.Value = nasPdu

		PDUSessionResourceReleaseCommandIEs.List = append(PDUSessionResourceReleaseCommandIEs.List, ie)
	}

	// PDUSessionResourceToReleaseListRelCmd
	ie = ngapType.PDUSessionResourceReleaseCommandIEs{}
	ie.Id.Value = ngapType.ProtocolIEIDPDUSessionResourceToReleaseListRelCmd
	ie.Criticality.Value = ngapType.CriticalityPresentReject
	ie.Value.Present = ngapType.PDUSessionResourceReleaseCommandIEsPresentPDUSessionResourceToReleaseListRelCmd
	ie.Value.PDUSessionResourceToReleaseListRelCmd = &pduSessionResourceReleasedList
	PDUSessionResourceReleaseCommandIEs.List = append(PDUSessionResourceReleaseCommandIEs.List, ie)

	return ngap.Encoder(pdu)
}

func BuildUEContextReleaseCommand(amfUENGAPID int64, ranUENGAPID int64, causePresent int, cause aper.Enumerated) ([]byte, error) {
	var pdu ngapType.NGAPPDU

	pdu.Present = ngapType.NGAPPDUPresentInitiatingMessage
	pdu.InitiatingMessage = new(ngapType.InitiatingMessage)

	initiatingMessage := pdu.InitiatingMessage
	initiatingMessage.ProcedureCode.Value = ngapType.ProcedureCodeUEContextRelease
	initiatingMessage.Criticality.Value = ngapType.CriticalityPresentReject

	initiatingMessage.Value.Present = ngapType.InitiatingMessagePresentUEContextReleaseCommand
	initiatingMessage.Value.UEContextReleaseCommand = new(ngapType.UEContextReleaseCommand)

	ueContextReleaseCommand := initiatingMessage.Value.UEContextReleaseCommand
	ueContextReleaseCommandIEs := &ueContextReleaseCommand.ProtocolIEs

	// UE NGAP IDs
	ie := ngapType.UEContextReleaseCommandIEs{}
	ie.Id.Value = ngapType.ProtocolIEIDUENGAPIDs
	ie.Criticality.Value = ngapType.CriticalityPresentReject
	ie.Value.Present = ngapType.UEContextReleaseCommandIEsPresentUENGAPIDs
	ie.Value.UENGAPIDs = new(ngapType.UENGAPIDs)

	ueNGAPIDs := ie.Value.UENGAPIDs

	if ranUENGAPID == models.RanUeNgapIDUnspecified {
		ueNGAPIDs.Present = ngapType.UENGAPIDsPresentAMFUENGAPID
		ueNGAPIDs.AMFUENGAPID = new(ngapType.AMFUENGAPID)

		ueNGAPIDs.AMFUENGAPID.Value = amfUENGAPID
	} else {
		ueNGAPIDs.Present = ngapType.UENGAPIDsPresentUENGAPIDPair
		ueNGAPIDs.UENGAPIDPair = new(ngapType.UENGAPIDPair)

		ueNGAPIDs.UENGAPIDPair.AMFUENGAPID.Value = amfUENGAPID
		ueNGAPIDs.UENGAPIDPair.RANUENGAPID.Value = ranUENGAPID
	}

	ueContextReleaseCommandIEs.List = append(ueContextReleaseCommandIEs.List, ie)

	// Cause
	ie = ngapType.UEContextReleaseCommandIEs{}
	ie.Id.Value = ngapType.ProtocolIEIDCause
	ie.Criticality.Value = ngapType.CriticalityPresentIgnore
	ie.Value.Present = ngapType.UEContextReleaseCommandIEsPresentCause
	ngapCause := ngapType.Cause{
		Present: causePresent,
	}
	switch causePresent {
	case ngapType.CausePresentNothing:
		return nil, fmt.Errorf("cause present is nothing")
	case ngapType.CausePresentRadioNetwork:
		ngapCause.RadioNetwork = new(ngapType.CauseRadioNetwork)
		ngapCause.RadioNetwork.Value = cause
	case ngapType.CausePresentTransport:
		ngapCause.Transport = new(ngapType.CauseTransport)
		ngapCause.Transport.Value = cause
	case ngapType.CausePresentNas:
		ngapCause.Nas = new(ngapType.CauseNas)
		ngapCause.Nas.Value = cause
	case ngapType.CausePresentProtocol:
		ngapCause.Protocol = new(ngapType.CauseProtocol)
		ngapCause.Protocol.Value = cause
	case ngapType.CausePresentMisc:
		ngapCause.Misc = new(ngapType.CauseMisc)
		ngapCause.Misc.Value = cause
	default:
		return nil, fmt.Errorf("invalid cause present")
	}
	ie.Value.Cause = &ngapCause

	ueContextReleaseCommandIEs.List = append(ueContextReleaseCommandIEs.List, ie)

	return ngap.Encoder(pdu)
}

func BuildDownlinkNasTransport(amfUENGAPID int64, ranUENGAPID int64, nasPdu []byte, mobilityRestrictionList *ngapType.MobilityRestrictionList) ([]byte, error) {
	var pdu ngapType.NGAPPDU

	pdu.Present = ngapType.NGAPPDUPresentInitiatingMessage
	pdu.InitiatingMessage = new(ngapType.InitiatingMessage)

	initiatingMessage := pdu.InitiatingMessage
	initiatingMessage.ProcedureCode.Value = ngapType.ProcedureCodeDownlinkNASTransport
	initiatingMessage.Criticality.Value = ngapType.CriticalityPresentIgnore

	initiatingMessage.Value.Present = ngapType.InitiatingMessagePresentDownlinkNASTransport
	initiatingMessage.Value.DownlinkNASTransport = new(ngapType.DownlinkNASTransport)

	downlinkNasTransport := initiatingMessage.Value.DownlinkNASTransport
	downlinkNasTransportIEs := &downlinkNasTransport.ProtocolIEs

	// AMF UE NGAP ID
	ie := ngapType.DownlinkNASTransportIEs{}
	ie.Id.Value = ngapType.ProtocolIEIDAMFUENGAPID
	ie.Criticality.Value = ngapType.CriticalityPresentReject
	ie.Value.Present = ngapType.DownlinkNASTransportIEsPresentAMFUENGAPID
	ie.Value.AMFUENGAPID = new(ngapType.AMFUENGAPID)

	aMFUENGAPID := ie.Value.AMFUENGAPID
	aMFUENGAPID.Value = amfUENGAPID

	downlinkNasTransportIEs.List = append(downlinkNasTransportIEs.List, ie)

	// RAN UE NGAP ID
	ie = ngapType.DownlinkNASTransportIEs{}
	ie.Id.Value = ngapType.ProtocolIEIDRANUENGAPID
	ie.Criticality.Value = ngapType.CriticalityPresentReject
	ie.Value.Present = ngapType.DownlinkNASTransportIEsPresentRANUENGAPID
	ie.Value.RANUENGAPID = new(ngapType.RANUENGAPID)

	rANUENGAPID := ie.Value.RANUENGAPID
	rANUENGAPID.Value = ranUENGAPID

	downlinkNasTransportIEs.List = append(downlinkNasTransportIEs.List, ie)

	// NAS PDU
	ie = ngapType.DownlinkNASTransportIEs{}
	ie.Id.Value = ngapType.ProtocolIEIDNASPDU
	ie.Criticality.Value = ngapType.CriticalityPresentReject
	ie.Value.Present = ngapType.DownlinkNASTransportIEsPresentNASPDU
	ie.Value.NASPDU = new(ngapType.NASPDU)

	ie.Value.NASPDU.Value = nasPdu

	downlinkNasTransportIEs.List = append(downlinkNasTransportIEs.List, ie)

	// RAN Paging Priority (optional)
	// Mobility Restriction List (optional)
	if mobilityRestrictionList != nil {
		ie = ngapType.DownlinkNASTransportIEs{}
		ie.Id.Value = ngapType.ProtocolIEIDMobilityRestrictionList
		ie.Criticality.Value = ngapType.CriticalityPresentIgnore
		ie.Value.Present = ngapType.DownlinkNASTransportIEsPresentMobilityRestrictionList
		ie.Value.MobilityRestrictionList = mobilityRestrictionList
		downlinkNasTransportIEs.List = append(downlinkNasTransportIEs.List, ie)
	}
	// Index to RAT/Frequency Selection Priority (optional)
	// UE Aggregate Maximum Bit Rate (optional)
	// Allowed NSSAI (optional)

	return ngap.Encoder(pdu)
}
