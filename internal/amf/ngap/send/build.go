package send

import (
	"context"
	"fmt"

	"github.com/ellanetworks/core/internal/amf/util"
	"github.com/ellanetworks/core/internal/models"
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
