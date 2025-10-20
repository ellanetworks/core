package ngap

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/omec-project/ngap/aper"
	"github.com/omec-project/ngap/ngapType"
)

const ntpToUnixOffset = 2208988800 // seconds between 1900-01-01 and 1970-01-01

type IE struct {
	ID          EnumField `json:"id"`
	Criticality EnumField `json:"criticality"`
	Value       any       `json:"value,omitempty"`
}

type UnknownIE struct {
	Reason string `json:"reason"`
}

func timeStampToRFC3339(timeStampNgap aper.OctetString) (string, error) {
	if len(timeStampNgap) != 4 {
		return "", fmt.Errorf("invalid NGAP timestamp length: got %d, want 4", len(timeStampNgap))
	}

	ntpSeconds := binary.BigEndian.Uint32(timeStampNgap)
	unixSeconds := int64(ntpSeconds) - ntpToUnixOffset
	t := time.Unix(unixSeconds, 0).UTC()
	return t.Format(time.RFC3339), nil
}

func bitStringToHex(bitString *aper.BitString) string {
	hexString := hex.EncodeToString(bitString.Bytes)
	hexLen := (bitString.BitLength + 3) / 4
	hexString = hexString[:hexLen]
	return hexString
}

func plmnIDToModels(ngapPlmnID ngapType.PLMNIdentity) PLMNID {
	value := ngapPlmnID.Value
	hexString := strings.Split(hex.EncodeToString(value), "")

	var modelsPlmnid PLMNID

	modelsPlmnid.Mcc = hexString[1] + hexString[0] + hexString[3]
	if hexString[2] == "f" {
		modelsPlmnid.Mnc = hexString[5] + hexString[4]
	} else {
		modelsPlmnid.Mnc = hexString[2] + hexString[5] + hexString[4]
	}

	return modelsPlmnid
}

func protocolIEIDToEnum(id int64) EnumField {
	switch id {
	case ngapType.ProtocolIEIDAllowedNSSAI:
		return EnumField{Label: "AllowedNSSAI", Value: int(id)}
	case ngapType.ProtocolIEIDAMFName:
		return EnumField{Label: "AMFName", Value: int(id)}
	case ngapType.ProtocolIEIDAMFOverloadResponse:
		return EnumField{Label: "AMFOverloadResponse", Value: int(id)}
	case ngapType.ProtocolIEIDAMFSetID:
		return EnumField{Label: "AMFSetID", Value: int(id)}
	case ngapType.ProtocolIEIDAMFTNLAssociationFailedToSetupList:
		return EnumField{Label: "AMFTNLAssociationFailedToSetupList", Value: int(id)}
	case ngapType.ProtocolIEIDAMFTNLAssociationSetupList:
		return EnumField{Label: "AMFTNLAssociationSetupList", Value: int(id)}
	case ngapType.ProtocolIEIDAMFTNLAssociationToAddList:
		return EnumField{Label: "AMFTNLAssociationToAddList", Value: int(id)}
	case ngapType.ProtocolIEIDAMFTNLAssociationToRemoveList:
		return EnumField{Label: "AMFTNLAssociationToRemoveList", Value: int(id)}
	case ngapType.ProtocolIEIDAMFTNLAssociationToUpdateList:
		return EnumField{Label: "AMFTNLAssociationToUpdateList", Value: int(id)}
	case ngapType.ProtocolIEIDAMFTrafficLoadReductionIndication:
		return EnumField{Label: "AMFTrafficLoadReductionIndication", Value: int(id)}
	case ngapType.ProtocolIEIDAMFUENGAPID:
		return EnumField{Label: "AMFUENGAPID", Value: int(id)}
	case ngapType.ProtocolIEIDAssistanceDataForPaging:
		return EnumField{Label: "AssistanceDataForPaging", Value: int(id)}
	case ngapType.ProtocolIEIDBroadcastCancelledAreaList:
		return EnumField{Label: "BroadcastCancelledAreaList", Value: int(id)}
	case ngapType.ProtocolIEIDBroadcastCompletedAreaList:
		return EnumField{Label: "BroadcastCompletedAreaList", Value: int(id)}
	case ngapType.ProtocolIEIDCancelAllWarningMessages:
		return EnumField{Label: "CancelAllWarningMessages", Value: int(id)}
	case ngapType.ProtocolIEIDCause:
		return EnumField{Label: "Cause", Value: int(id)}
	case ngapType.ProtocolIEIDCellIDListForRestart:
		return EnumField{Label: "CellIDListForRestart", Value: int(id)}
	case ngapType.ProtocolIEIDConcurrentWarningMessageInd:
		return EnumField{Label: "ConcurrentWarningMessageInd", Value: int(id)}
	case ngapType.ProtocolIEIDCoreNetworkAssistanceInformation:
		return EnumField{Label: "CoreNetworkAssistanceInformation", Value: int(id)}
	case ngapType.ProtocolIEIDCriticalityDiagnostics:
		return EnumField{Label: "CriticalityDiagnostics", Value: int(id)}
	case ngapType.ProtocolIEIDDataCodingScheme:
		return EnumField{Label: "DataCodingScheme", Value: int(id)}
	case ngapType.ProtocolIEIDDefaultPagingDRX:
		return EnumField{Label: "DefaultPagingDRX", Value: int(id)}
	case ngapType.ProtocolIEIDDirectForwardingPathAvailability:
		return EnumField{Label: "DirectForwardingPathAvailability", Value: int(id)}
	case ngapType.ProtocolIEIDEmergencyAreaIDListForRestart:
		return EnumField{Label: "EmergencyAreaIDListForRestart", Value: int(id)}
	case ngapType.ProtocolIEIDEmergencyFallbackIndicator:
		return EnumField{Label: "EmergencyFallbackIndicator", Value: int(id)}
	case ngapType.ProtocolIEIDEUTRACGI:
		return EnumField{Label: "EUTRACGI", Value: int(id)}
	case ngapType.ProtocolIEIDFiveGSTMSI:
		return EnumField{Label: "FiveGSTMSI", Value: int(id)}
	case ngapType.ProtocolIEIDGlobalRANNodeID:
		return EnumField{Label: "GlobalRANNodeID", Value: int(id)}
	case ngapType.ProtocolIEIDGUAMI:
		return EnumField{Label: "GUAMI", Value: int(id)}
	case ngapType.ProtocolIEIDHandoverType:
		return EnumField{Label: "HandoverType", Value: int(id)}
	case ngapType.ProtocolIEIDIMSVoiceSupportIndicator:
		return EnumField{Label: "IMSVoiceSupportIndicator", Value: int(id)}
	case ngapType.ProtocolIEIDIndexToRFSP:
		return EnumField{Label: "IndexToRFSP", Value: int(id)}
	case ngapType.ProtocolIEIDInfoOnRecommendedCellsAndRANNodesForPaging:
		return EnumField{Label: "InfoOnRecommendedCellsAndRANNodesForPaging", Value: int(id)}
	case ngapType.ProtocolIEIDLocationReportingRequestType:
		return EnumField{Label: "LocationReportingRequestType", Value: int(id)}
	case ngapType.ProtocolIEIDMaskedIMEISV:
		return EnumField{Label: "MaskedIMEISV", Value: int(id)}
	case ngapType.ProtocolIEIDMessageIdentifier:
		return EnumField{Label: "MessageIdentifier", Value: int(id)}
	case ngapType.ProtocolIEIDMobilityRestrictionList:
		return EnumField{Label: "MobilityRestrictionList", Value: int(id)}
	case ngapType.ProtocolIEIDNASC:
		return EnumField{Label: "NASC", Value: int(id)}
	case ngapType.ProtocolIEIDNASPDU:
		return EnumField{Label: "NASPDU", Value: int(id)}
	case ngapType.ProtocolIEIDNASSecurityParametersFromNGRAN:
		return EnumField{Label: "NASSecurityParametersFromNGRAN", Value: int(id)}
	case ngapType.ProtocolIEIDNewAMFUENGAPID:
		return EnumField{Label: "NewAMFUENGAPID", Value: int(id)}
	case ngapType.ProtocolIEIDNewSecurityContextInd:
		return EnumField{Label: "NewSecurityContextInd", Value: int(id)}
	case ngapType.ProtocolIEIDNGAPMessage:
		return EnumField{Label: "NGAPMessage", Value: int(id)}
	case ngapType.ProtocolIEIDNGRANCGI:
		return EnumField{Label: "NGRANCGI", Value: int(id)}
	case ngapType.ProtocolIEIDNGRANTraceID:
		return EnumField{Label: "NGRANTraceID", Value: int(id)}
	case ngapType.ProtocolIEIDNRCGI:
		return EnumField{Label: "NRCGI", Value: int(id)}
	case ngapType.ProtocolIEIDNRPPaPDU:
		return EnumField{Label: "NRPPaPDU", Value: int(id)}
	case ngapType.ProtocolIEIDNumberOfBroadcastsRequested:
		return EnumField{Label: "NumberOfBroadcastsRequested", Value: int(id)}
	case ngapType.ProtocolIEIDOldAMF:
		return EnumField{Label: "OldAMF", Value: int(id)}
	case ngapType.ProtocolIEIDOverloadStartNSSAIList:
		return EnumField{Label: "OverloadStartNSSAIList", Value: int(id)}
	case ngapType.ProtocolIEIDPagingDRX:
		return EnumField{Label: "PagingDRX", Value: int(id)}
	case ngapType.ProtocolIEIDPagingOrigin:
		return EnumField{Label: "PagingOrigin", Value: int(id)}
	case ngapType.ProtocolIEIDPagingPriority:
		return EnumField{Label: "PagingPriority", Value: int(id)}
	case ngapType.ProtocolIEIDPDUSessionResourceAdmittedList:
		return EnumField{Label: "PDUSessionResourceAdmittedList", Value: int(id)}
	case ngapType.ProtocolIEIDPDUSessionResourceFailedToModifyListModRes:
		return EnumField{Label: "PDUSessionResourceFailedToModifyListModRes", Value: int(id)}
	case ngapType.ProtocolIEIDPDUSessionResourceFailedToSetupListCxtRes:
		return EnumField{Label: "PDUSessionResourceFailedToSetupListCxtRes", Value: int(id)}
	case ngapType.ProtocolIEIDPDUSessionResourceFailedToSetupListHOAck:
		return EnumField{Label: "PDUSessionResourceFailedToSetupListHOAck", Value: int(id)}
	case ngapType.ProtocolIEIDPDUSessionResourceFailedToSetupListPSReq:
		return EnumField{Label: "PDUSessionResourceFailedToSetupListPSReq", Value: int(id)}
	case ngapType.ProtocolIEIDPDUSessionResourceFailedToSetupListSURes:
		return EnumField{Label: "PDUSessionResourceFailedToSetupListSURes", Value: int(id)}
	case ngapType.ProtocolIEIDPDUSessionResourceHandoverList:
		return EnumField{Label: "PDUSessionResourceHandoverList", Value: int(id)}
	case ngapType.ProtocolIEIDPDUSessionResourceListCxtRelCpl:
		return EnumField{Label: "PDUSessionResourceListCxtRelCpl", Value: int(id)}
	case ngapType.ProtocolIEIDPDUSessionResourceListHORqd:
		return EnumField{Label: "PDUSessionResourceListHORqd", Value: int(id)}
	case ngapType.ProtocolIEIDPDUSessionResourceModifyListModCfm:
		return EnumField{Label: "PDUSessionResourceModifyListModCfm", Value: int(id)}
	case ngapType.ProtocolIEIDPDUSessionResourceModifyListModInd:
		return EnumField{Label: "PDUSessionResourceModifyListModInd", Value: int(id)}
	case ngapType.ProtocolIEIDPDUSessionResourceModifyListModReq:
		return EnumField{Label: "PDUSessionResourceModifyListModReq", Value: int(id)}
	case ngapType.ProtocolIEIDPDUSessionResourceModifyListModRes:
		return EnumField{Label: "PDUSessionResourceModifyListModRes", Value: int(id)}
	case ngapType.ProtocolIEIDPDUSessionResourceNotifyList:
		return EnumField{Label: "PDUSessionResourceNotifyList", Value: int(id)}
	case ngapType.ProtocolIEIDPDUSessionResourceReleasedListNot:
		return EnumField{Label: "PDUSessionResourceReleasedListNot", Value: int(id)}
	case ngapType.ProtocolIEIDPDUSessionResourceReleasedListPSAck:
		return EnumField{Label: "PDUSessionResourceReleasedListPSAck", Value: int(id)}
	case ngapType.ProtocolIEIDPDUSessionResourceReleasedListPSFail:
		return EnumField{Label: "PDUSessionResourceReleasedListPSFail", Value: int(id)}
	case ngapType.ProtocolIEIDPDUSessionResourceReleasedListRelRes:
		return EnumField{Label: "PDUSessionResourceReleasedListRelRes", Value: int(id)}
	case ngapType.ProtocolIEIDPDUSessionResourceSetupListCxtReq:
		return EnumField{Label: "PDUSessionResourceSetupListCxtReq", Value: int(id)}
	case ngapType.ProtocolIEIDPDUSessionResourceSetupListCxtRes:
		return EnumField{Label: "PDUSessionResourceSetupListCxtRes", Value: int(id)}
	case ngapType.ProtocolIEIDPDUSessionResourceSetupListHOReq:
		return EnumField{Label: "PDUSessionResourceSetupListHOReq", Value: int(id)}
	case ngapType.ProtocolIEIDPDUSessionResourceSetupListSUReq:
		return EnumField{Label: "PDUSessionResourceSetupListSUReq", Value: int(id)}
	case ngapType.ProtocolIEIDPDUSessionResourceSetupListSURes:
		return EnumField{Label: "PDUSessionResourceSetupListSURes", Value: int(id)}
	case ngapType.ProtocolIEIDPDUSessionResourceToBeSwitchedDLList:
		return EnumField{Label: "PDUSessionResourceToBeSwitchedDLList", Value: int(id)}
	case ngapType.ProtocolIEIDPDUSessionResourceSwitchedList:
		return EnumField{Label: "PDUSessionResourceSwitchedList", Value: int(id)}
	case ngapType.ProtocolIEIDPDUSessionResourceToReleaseListHOCmd:
		return EnumField{Label: "PDUSessionResourceToReleaseListHOCmd", Value: int(id)}
	case ngapType.ProtocolIEIDPDUSessionResourceToReleaseListRelCmd:
		return EnumField{Label: "PDUSessionResourceToReleaseListRelCmd", Value: int(id)}
	case ngapType.ProtocolIEIDPLMNSupportList:
		return EnumField{Label: "PLMNSupportList", Value: int(id)}
	case ngapType.ProtocolIEIDPWSFailedCellIDList:
		return EnumField{Label: "PWSFailedCellIDList", Value: int(id)}
	case ngapType.ProtocolIEIDRANNodeName:
		return EnumField{Label: "RANNodeName", Value: int(id)}
	case ngapType.ProtocolIEIDRANPagingPriority:
		return EnumField{Label: "RANPagingPriority", Value: int(id)}
	case ngapType.ProtocolIEIDRANStatusTransferTransparentContainer:
		return EnumField{Label: "RANStatusTransferTransparentContainer", Value: int(id)}
	case ngapType.ProtocolIEIDRANUENGAPID:
		return EnumField{Label: "RANUENGAPID", Value: int(id)}
	case ngapType.ProtocolIEIDRelativeAMFCapacity:
		return EnumField{Label: "RelativeAMFCapacity", Value: int(id)}
	case ngapType.ProtocolIEIDRepetitionPeriod:
		return EnumField{Label: "RepetitionPeriod", Value: int(id)}
	case ngapType.ProtocolIEIDResetType:
		return EnumField{Label: "ResetType", Value: int(id)}
	case ngapType.ProtocolIEIDRoutingID:
		return EnumField{Label: "RoutingID", Value: int(id)}
	case ngapType.ProtocolIEIDRRCEstablishmentCause:
		return EnumField{Label: "RRCEstablishmentCause", Value: int(id)}
	case ngapType.ProtocolIEIDRRCInactiveTransitionReportRequest:
		return EnumField{Label: "RRCInactiveTransitionReportRequest", Value: int(id)}
	case ngapType.ProtocolIEIDRRCState:
		return EnumField{Label: "RRCState", Value: int(id)}
	case ngapType.ProtocolIEIDSecurityContext:
		return EnumField{Label: "SecurityContext", Value: int(id)}
	case ngapType.ProtocolIEIDSecurityKey:
		return EnumField{Label: "SecurityKey", Value: int(id)}
	case ngapType.ProtocolIEIDSerialNumber:
		return EnumField{Label: "SerialNumber", Value: int(id)}
	case ngapType.ProtocolIEIDServedGUAMIList:
		return EnumField{Label: "ServedGUAMIList", Value: int(id)}
	case ngapType.ProtocolIEIDSliceSupportList:
		return EnumField{Label: "SliceSupportList", Value: int(id)}
	case ngapType.ProtocolIEIDSONConfigurationTransferDL:
		return EnumField{Label: "SONConfigurationTransferDL", Value: int(id)}
	case ngapType.ProtocolIEIDSONConfigurationTransferUL:
		return EnumField{Label: "SONConfigurationTransferUL", Value: int(id)}
	case ngapType.ProtocolIEIDSourceAMFUENGAPID:
		return EnumField{Label: "SourceAMFUENGAPID", Value: int(id)}
	case ngapType.ProtocolIEIDSourceToTargetTransparentContainer:
		return EnumField{Label: "SourceToTargetTransparentContainer", Value: int(id)}
	case ngapType.ProtocolIEIDSupportedTAList:
		return EnumField{Label: "SupportedTAList", Value: int(id)}
	case ngapType.ProtocolIEIDTAIListForPaging:
		return EnumField{Label: "TAIListForPaging", Value: int(id)}
	case ngapType.ProtocolIEIDTAIListForRestart:
		return EnumField{Label: "TAIListForRestart", Value: int(id)}
	case ngapType.ProtocolIEIDTargetID:
		return EnumField{Label: "TargetID", Value: int(id)}
	case ngapType.ProtocolIEIDTargetToSourceTransparentContainer:
		return EnumField{Label: "TargetToSourceTransparentContainer", Value: int(id)}
	case ngapType.ProtocolIEIDTimeToWait:
		return EnumField{Label: "TimeToWait", Value: int(id)}
	case ngapType.ProtocolIEIDTraceActivation:
		return EnumField{Label: "TraceActivation", Value: int(id)}
	case ngapType.ProtocolIEIDTraceCollectionEntityIPAddress:
		return EnumField{Label: "TraceCollectionEntityIPAddress", Value: int(id)}
	case ngapType.ProtocolIEIDUEAggregateMaximumBitRate:
		return EnumField{Label: "UEAggregateMaximumBitRate", Value: int(id)}
	case ngapType.ProtocolIEIDUEAssociatedLogicalNGConnectionList:
		return EnumField{Label: "UEAssociatedLogicalNGConnectionList", Value: int(id)}
	case ngapType.ProtocolIEIDUEContextRequest:
		return EnumField{Label: "UEContextRequest", Value: int(id)}
	case ngapType.ProtocolIEIDUENGAPIDs:
		return EnumField{Label: "UENGAPIDs", Value: int(id)}
	case ngapType.ProtocolIEIDUEPagingIdentity:
		return EnumField{Label: "UEPagingIdentity", Value: int(id)}
	case ngapType.ProtocolIEIDUEPresenceInAreaOfInterestList:
		return EnumField{Label: "UEPresenceInAreaOfInterestList", Value: int(id)}
	case ngapType.ProtocolIEIDUERadioCapability:
		return EnumField{Label: "UERadioCapability", Value: int(id)}
	case ngapType.ProtocolIEIDUERadioCapabilityForPaging:
		return EnumField{Label: "UERadioCapabilityForPaging", Value: int(id)}
	case ngapType.ProtocolIEIDUESecurityCapabilities:
		return EnumField{Label: "UESecurityCapabilities", Value: int(id)}
	case ngapType.ProtocolIEIDUnavailableGUAMIList:
		return EnumField{Label: "UnavailableGUAMIList", Value: int(id)}
	case ngapType.ProtocolIEIDUserLocationInformation:
		return EnumField{Label: "UserLocationInformation", Value: int(id)}
	case ngapType.ProtocolIEIDWarningAreaList:
		return EnumField{Label: "WarningAreaList", Value: int(id)}
	case ngapType.ProtocolIEIDWarningMessageContents:
		return EnumField{Label: "WarningMessageContents", Value: int(id)}
	case ngapType.ProtocolIEIDWarningSecurityInfo:
		return EnumField{Label: "WarningSecurityInfo", Value: int(id)}
	case ngapType.ProtocolIEIDWarningType:
		return EnumField{Label: "WarningType", Value: int(id)}
	case ngapType.ProtocolIEIDAdditionalULNGUUPTNLInformation:
		return EnumField{Label: "AdditionalULNGUUPTNLInformation", Value: int(id)}
	case ngapType.ProtocolIEIDDataForwardingNotPossible:
		return EnumField{Label: "DataForwardingNotPossible", Value: int(id)}
	case ngapType.ProtocolIEIDDLNGUUPTNLInformation:
		return EnumField{Label: "DLNGUUPTNLInformation", Value: int(id)}
	case ngapType.ProtocolIEIDNetworkInstance:
		return EnumField{Label: "NetworkInstance", Value: int(id)}
	case ngapType.ProtocolIEIDPDUSessionAggregateMaximumBitRate:
		return EnumField{Label: "PDUSessionAggregateMaximumBitRate", Value: int(id)}
	case ngapType.ProtocolIEIDPDUSessionResourceFailedToModifyListModCfm:
		return EnumField{Label: "PDUSessionResourceFailedToModifyListModCfm", Value: int(id)}
	case ngapType.ProtocolIEIDPDUSessionResourceFailedToSetupListCxtFail:
		return EnumField{Label: "PDUSessionResourceFailedToSetupListCxtFail", Value: int(id)}
	case ngapType.ProtocolIEIDPDUSessionResourceListCxtRelReq:
		return EnumField{Label: "PDUSessionResourceListCxtRelReq", Value: int(id)}
	case ngapType.ProtocolIEIDPDUSessionType:
		return EnumField{Label: "PDUSessionType", Value: int(id)}
	case ngapType.ProtocolIEIDQosFlowAddOrModifyRequestList:
		return EnumField{Label: "QosFlowAddOrModifyRequestList", Value: int(id)}
	case ngapType.ProtocolIEIDQosFlowSetupRequestList:
		return EnumField{Label: "QosFlowSetupRequestList", Value: int(id)}
	case ngapType.ProtocolIEIDQosFlowToReleaseList:
		return EnumField{Label: "QosFlowToReleaseList", Value: int(id)}
	case ngapType.ProtocolIEIDSecurityIndication:
		return EnumField{Label: "SecurityIndication", Value: int(id)}
	case ngapType.ProtocolIEIDULNGUUPTNLInformation:
		return EnumField{Label: "ULNGUUPTNLInformation", Value: int(id)}
	case ngapType.ProtocolIEIDULNGUUPTNLModifyList:
		return EnumField{Label: "ULNGUUPTNLModifyList", Value: int(id)}
	case ngapType.ProtocolIEIDWarningAreaCoordinates:
		return EnumField{Label: "WarningAreaCoordinates", Value: int(id)}
	case ngapType.ProtocolIEIDPDUSessionResourceSecondaryRATUsageList:
		return EnumField{Label: "PDUSessionResourceSecondaryRATUsageList", Value: int(id)}
	case ngapType.ProtocolIEIDHandoverFlag:
		return EnumField{Label: "HandoverFlag", Value: int(id)}
	case ngapType.ProtocolIEIDSecondaryRATUsageInformation:
		return EnumField{Label: "SecondaryRATUsageInformation", Value: int(id)}
	case ngapType.ProtocolIEIDPDUSessionResourceReleaseResponseTransfer:
		return EnumField{Label: "PDUSessionResourceReleaseResponseTransfer", Value: int(id)}
	case ngapType.ProtocolIEIDRedirectionVoiceFallback:
		return EnumField{Label: "RedirectionVoiceFallback", Value: int(id)}
	case ngapType.ProtocolIEIDUERetentionInformation:
		return EnumField{Label: "UERetentionInformation", Value: int(id)}
	case ngapType.ProtocolIEIDSNSSAI:
		return EnumField{Label: "SNSSAI", Value: int(id)}
	case ngapType.ProtocolIEIDPSCellInformation:
		return EnumField{Label: "PSCellInformation", Value: int(id)}
	case ngapType.ProtocolIEIDLastEUTRANPLMNIdentity:
		return EnumField{Label: "LastEUTRANPLMNIdentity", Value: int(id)}
	case ngapType.ProtocolIEIDMaximumIntegrityProtectedDataRateDL:
		return EnumField{Label: "MaximumIntegrityProtectedDataRateDL", Value: int(id)}
	case ngapType.ProtocolIEIDAdditionalDLForwardingUPTNLInformation:
		return EnumField{Label: "AdditionalDLForwardingUPTNLInformation", Value: int(id)}
	case ngapType.ProtocolIEIDAdditionalDLUPTNLInformationForHOList:
		return EnumField{Label: "AdditionalDLUPTNLInformationForHOList", Value: int(id)}
	case ngapType.ProtocolIEIDAdditionalNGUUPTNLInformation:
		return EnumField{Label: "AdditionalNGUUPTNLInformation", Value: int(id)}
	case ngapType.ProtocolIEIDAdditionalDLQosFlowPerTNLInformation:
		return EnumField{Label: "AdditionalDLQosFlowPerTNLInformation", Value: int(id)}
	case ngapType.ProtocolIEIDSecurityResult:
		return EnumField{Label: "SecurityResult", Value: int(id)}
	case ngapType.ProtocolIEIDENDCSONConfigurationTransferDL:
		return EnumField{Label: "ENDCSONConfigurationTransferDL", Value: 157}
	case ngapType.ProtocolIEIDENDCSONConfigurationTransferUL:
		return EnumField{Label: "ENDCSONConfigurationTransferUL", Value: 158}
	default:
		return EnumField{Label: "Unknown", Value: int(id)}
	}
}

func causeToEnum(cause *ngapType.Cause) EnumField {
	if cause == nil {
		return EnumField{Label: "nil", Value: -1}
	}

	switch cause.Present {
	case ngapType.CausePresentRadioNetwork:
		return radioNetworkCauseToEnum(cause.RadioNetwork)
	case ngapType.CausePresentTransport:
		return transportCauseToEnum(cause.Transport)
	case ngapType.CausePresentNas:
		return nasCauseToEnum(cause.Nas)
	case ngapType.CausePresentProtocol:
		return protocolCauseToEnum(cause.Protocol)
	case ngapType.CausePresentMisc:
		return miscCauseToEnum(cause.Misc)
	default:
		return EnumField{Label: "Unknown", Value: cause.Present}
	}
}

func radioNetworkCauseToEnum(cause *ngapType.CauseRadioNetwork) EnumField {
	if cause == nil {
		return EnumField{Label: "nil", Value: -1}
	}

	switch cause.Value {
	case ngapType.CauseRadioNetworkPresentUnspecified:
		return EnumField{Label: "Unspecified", Value: int(ngapType.CauseRadioNetworkPresentUnspecified)}
	case ngapType.CauseRadioNetworkPresentTxnrelocoverallExpiry:
		return EnumField{Label: "TxNRelocOverallExpiry", Value: int(ngapType.CauseRadioNetworkPresentTxnrelocoverallExpiry)}
	case ngapType.CauseRadioNetworkPresentSuccessfulHandover:
		return EnumField{Label: "SuccessfulHandover", Value: int(ngapType.CauseRadioNetworkPresentSuccessfulHandover)}
	case ngapType.CauseRadioNetworkPresentReleaseDueToNgranGeneratedReason:
		return EnumField{Label: "ReleaseDueToNgranGeneratedReason", Value: int(ngapType.CauseRadioNetworkPresentReleaseDueToNgranGeneratedReason)}
	case ngapType.CauseRadioNetworkPresentReleaseDueTo5gcGeneratedReason:
		return EnumField{Label: "ReleaseDueTo5gcGeneratedReason", Value: int(ngapType.CauseRadioNetworkPresentReleaseDueTo5gcGeneratedReason)}
	case ngapType.CauseRadioNetworkPresentHandoverCancelled:
		return EnumField{Label: "HandoverCancelled", Value: int(ngapType.CauseRadioNetworkPresentHandoverCancelled)}
	case ngapType.CauseRadioNetworkPresentPartialHandover:
		return EnumField{Label: "PartialHandover", Value: int(ngapType.CauseRadioNetworkPresentPartialHandover)}
	case ngapType.CauseRadioNetworkPresentHoFailureInTarget5GCNgranNodeOrTargetSystem:
		return EnumField{Label: "HoFailureInTarget5GCNgranNodeOrTargetSystem", Value: int(ngapType.CauseRadioNetworkPresentHoFailureInTarget5GCNgranNodeOrTargetSystem)}
	case ngapType.CauseRadioNetworkPresentHoTargetNotAllowed:
		return EnumField{Label: "HoTargetNotAllowed", Value: int(ngapType.CauseRadioNetworkPresentHoTargetNotAllowed)}
	case ngapType.CauseRadioNetworkPresentTngrelocoverallExpiry:
		return EnumField{Label: "TngRelocOverallExpiry", Value: int(ngapType.CauseRadioNetworkPresentTngrelocoverallExpiry)}
	case ngapType.CauseRadioNetworkPresentTngrelocprepExpiry:
		return EnumField{Label: "TngRelocPrepExpiry", Value: int(ngapType.CauseRadioNetworkPresentTngrelocprepExpiry)}
	case ngapType.CauseRadioNetworkPresentCellNotAvailable:
		return EnumField{Label: "CellNotAvailable", Value: int(ngapType.CauseRadioNetworkPresentCellNotAvailable)}
	case ngapType.CauseRadioNetworkPresentUnknownTargetID:
		return EnumField{Label: "UnknownTargetID", Value: int(ngapType.CauseRadioNetworkPresentUnknownTargetID)}
	case ngapType.CauseRadioNetworkPresentNoRadioResourcesAvailableInTargetCell:
		return EnumField{Label: "NoRadioResourcesAvailableInTargetCell", Value: int(ngapType.CauseRadioNetworkPresentNoRadioResourcesAvailableInTargetCell)}
	case ngapType.CauseRadioNetworkPresentUnknownLocalUENGAPID:
		return EnumField{Label: "UnknownLocalUENGAPID", Value: int(ngapType.CauseRadioNetworkPresentUnknownLocalUENGAPID)}
	case ngapType.CauseRadioNetworkPresentInconsistentRemoteUENGAPID:
		return EnumField{Label: "InconsistentRemoteUENGAPID", Value: int(ngapType.CauseRadioNetworkPresentInconsistentRemoteUENGAPID)}
	case ngapType.CauseRadioNetworkPresentHandoverDesirableForRadioReason:
		return EnumField{Label: "HandoverDesirableForRadioReason", Value: int(ngapType.CauseRadioNetworkPresentHandoverDesirableForRadioReason)}
	case ngapType.CauseRadioNetworkPresentTimeCriticalHandover:
		return EnumField{Label: "TimeCriticalHandover", Value: int(ngapType.CauseRadioNetworkPresentTimeCriticalHandover)}
	case ngapType.CauseRadioNetworkPresentResourceOptimisationHandover:
		return EnumField{Label: "ResourceOptimisationHandover", Value: int(ngapType.CauseRadioNetworkPresentResourceOptimisationHandover)}
	case ngapType.CauseRadioNetworkPresentReduceLoadInServingCell:
		return EnumField{Label: "ReduceLoadInServingCell", Value: int(ngapType.CauseRadioNetworkPresentReduceLoadInServingCell)}
	case ngapType.CauseRadioNetworkPresentUserInactivity:
		return EnumField{Label: "UserInactivity", Value: int(ngapType.CauseRadioNetworkPresentUserInactivity)}
	case ngapType.CauseRadioNetworkPresentRadioConnectionWithUeLost:
		return EnumField{Label: "RadioConnectionWithUeLost", Value: int(ngapType.CauseRadioNetworkPresentRadioConnectionWithUeLost)}
	case ngapType.CauseRadioNetworkPresentRadioResourcesNotAvailable:
		return EnumField{Label: "RadioResourcesNotAvailable", Value: int(ngapType.CauseRadioNetworkPresentRadioResourcesNotAvailable)}
	case ngapType.CauseRadioNetworkPresentInvalidQosCombination:
		return EnumField{Label: "InvalidQosCombination", Value: int(ngapType.CauseRadioNetworkPresentInvalidQosCombination)}
	case ngapType.CauseRadioNetworkPresentFailureInRadioInterfaceProcedure:
		return EnumField{Label: "FailureInRadioInterfaceProcedure", Value: int(ngapType.CauseRadioNetworkPresentFailureInRadioInterfaceProcedure)}
	case ngapType.CauseRadioNetworkPresentInteractionWithOtherProcedure:
		return EnumField{Label: "InteractionWithOtherProcedure", Value: int(ngapType.CauseRadioNetworkPresentInteractionWithOtherProcedure)}
	case ngapType.CauseRadioNetworkPresentUnknownPDUSessionID:
		return EnumField{Label: "UnknownPDUSessionID", Value: int(ngapType.CauseRadioNetworkPresentUnknownPDUSessionID)}
	case ngapType.CauseRadioNetworkPresentUnkownQosFlowID:
		return EnumField{Label: "UnkownQosFlowID", Value: int(ngapType.CauseRadioNetworkPresentUnkownQosFlowID)}
	case ngapType.CauseRadioNetworkPresentMultiplePDUSessionIDInstances:
		return EnumField{Label: "MultiplePDUSessionIDInstances", Value: int(ngapType.CauseRadioNetworkPresentMultiplePDUSessionIDInstances)}
	case ngapType.CauseRadioNetworkPresentMultipleQosFlowIDInstances:
		return EnumField{Label: "MultipleQosFlowIDInstances", Value: int(ngapType.CauseRadioNetworkPresentMultipleQosFlowIDInstances)}
	case ngapType.CauseRadioNetworkPresentEncryptionAndOrIntegrityProtectionAlgorithmsNotSupported:
		return EnumField{Label: "EncryptionAndOrIntegrityProtectionAlgorithmsNotSupported", Value: int(ngapType.CauseRadioNetworkPresentEncryptionAndOrIntegrityProtectionAlgorithmsNotSupported)}
	case ngapType.CauseRadioNetworkPresentNgIntraSystemHandoverTriggered:
		return EnumField{Label: "NgIntraSystemHandoverTriggered", Value: int(ngapType.CauseRadioNetworkPresentNgIntraSystemHandoverTriggered)}
	case ngapType.CauseRadioNetworkPresentNgInterSystemHandoverTriggered:
		return EnumField{Label: "NgInterSystemHandoverTriggered", Value: int(ngapType.CauseRadioNetworkPresentNgInterSystemHandoverTriggered)}
	case ngapType.CauseRadioNetworkPresentXnHandoverTriggered:
		return EnumField{Label: "XnHandoverTriggered", Value: int(ngapType.CauseRadioNetworkPresentXnHandoverTriggered)}
	case ngapType.CauseRadioNetworkPresentNotSupported5QIValue:
		return EnumField{Label: "NotSupported5QIValue", Value: int(ngapType.CauseRadioNetworkPresentNotSupported5QIValue)}
	case ngapType.CauseRadioNetworkPresentUeContextTransfer:
		return EnumField{Label: "UeContextTransfer", Value: int(ngapType.CauseRadioNetworkPresentUeContextTransfer)}
	case ngapType.CauseRadioNetworkPresentImsVoiceEpsFallbackOrRatFallbackTriggered:
		return EnumField{Label: "ImsVoiceEpsFallbackOrRatFallbackTriggered", Value: int(ngapType.CauseRadioNetworkPresentImsVoiceEpsFallbackOrRatFallbackTriggered)}
	case ngapType.CauseRadioNetworkPresentUpIntegrityProtectionNotPossible:
		return EnumField{Label: "UpIntegrityProtectionNotPossible", Value: int(ngapType.CauseRadioNetworkPresentUpIntegrityProtectionNotPossible)}
	case ngapType.CauseRadioNetworkPresentUpConfidentialityProtectionNotPossible:
		return EnumField{Label: "UpConfidentialityProtectionNotPossible", Value: int(ngapType.CauseRadioNetworkPresentUpConfidentialityProtectionNotPossible)}
	case ngapType.CauseRadioNetworkPresentSliceNotSupported:
		return EnumField{Label: "SliceNotSupported", Value: int(ngapType.CauseRadioNetworkPresentSliceNotSupported)}
	case ngapType.CauseRadioNetworkPresentUeInRrcInactiveStateNotReachable:
		return EnumField{Label: "UeInRrcInactiveStateNotReachable", Value: int(ngapType.CauseRadioNetworkPresentUeInRrcInactiveStateNotReachable)}
	case ngapType.CauseRadioNetworkPresentRedirection:
		return EnumField{Label: "Redirection", Value: int(ngapType.CauseRadioNetworkPresentRedirection)}
	case ngapType.CauseRadioNetworkPresentResourcesNotAvailableForTheSlice:
		return EnumField{Label: "ResourcesNotAvailableForTheSlice", Value: int(ngapType.CauseRadioNetworkPresentResourcesNotAvailableForTheSlice)}
	case ngapType.CauseRadioNetworkPresentUeMaxIntegrityProtectedDataRateReason:
		return EnumField{Label: "UeMaxIntegrityProtectedDataRateReason", Value: int(ngapType.CauseRadioNetworkPresentUeMaxIntegrityProtectedDataRateReason)}
	case ngapType.CauseRadioNetworkPresentReleaseDueToCnDetectedMobility:
		return EnumField{Label: "ReleaseDueToCnDetectedMobility", Value: int(ngapType.CauseRadioNetworkPresentReleaseDueToCnDetectedMobility)}
	case ngapType.CauseRadioNetworkPresentN26InterfaceNotAvailable:
		return EnumField{Label: "N26InterfaceNotAvailable", Value: int(ngapType.CauseRadioNetworkPresentN26InterfaceNotAvailable)}
	case ngapType.CauseRadioNetworkPresentReleaseDueToPreEmption:
		return EnumField{Label: "ReleaseDueToPreEmption", Value: int(ngapType.CauseRadioNetworkPresentReleaseDueToPreEmption)}
	default:
		return EnumField{Label: "unknown", Value: int(cause.Value)}
	}
}

func transportCauseToEnum(cause *ngapType.CauseTransport) EnumField {
	if cause == nil {
		return EnumField{Label: "nil", Value: 0}
	}

	switch cause.Value {
	case ngapType.CauseTransportPresentTransportResourceUnavailable:
		return EnumField{Label: "TransportResourceUnavailable", Value: int(ngapType.CauseTransportPresentTransportResourceUnavailable)}
	case ngapType.CauseTransportPresentUnspecified:
		return EnumField{Label: "Unspecified", Value: int(ngapType.CauseTransportPresentUnspecified)}
	default:
		return EnumField{Label: "Unknown", Value: int(cause.Value)}
	}
}

func nasCauseToEnum(cause *ngapType.CauseNas) EnumField {
	if cause == nil {
		return EnumField{Label: "nil", Value: 0}
	}

	switch cause.Value {
	case ngapType.CauseNasPresentNormalRelease:
		return EnumField{Label: "NormalRelease", Value: int(ngapType.CauseNasPresentNormalRelease)}
	case ngapType.CauseNasPresentAuthenticationFailure:
		return EnumField{Label: "AuthenticationFailure", Value: int(ngapType.CauseNasPresentAuthenticationFailure)}
	case ngapType.CauseNasPresentDeregister:
		return EnumField{Label: "Deregister", Value: int(ngapType.CauseNasPresentDeregister)}
	case ngapType.CauseNasPresentUnspecified:
		return EnumField{Label: "Unspecified", Value: int(ngapType.CauseNasPresentUnspecified)}
	default:
		return EnumField{Label: "Unknown", Value: int(cause.Value)}
	}
}

func protocolCauseToEnum(cause *ngapType.CauseProtocol) EnumField {
	if cause == nil {
		return EnumField{Label: "nil", Value: 0}
	}

	switch cause.Value {
	case ngapType.CauseProtocolPresentTransferSyntaxError:
		return EnumField{Label: "TransferSyntaxError", Value: int(ngapType.CauseProtocolPresentTransferSyntaxError)}
	case ngapType.CauseProtocolPresentAbstractSyntaxErrorReject:
		return EnumField{Label: "AbstractSyntaxErrorReject", Value: int(ngapType.CauseProtocolPresentAbstractSyntaxErrorReject)}
	case ngapType.CauseProtocolPresentAbstractSyntaxErrorIgnoreAndNotify:
		return EnumField{Label: "AbstractSyntaxErrorIgnoreAndNotify", Value: int(ngapType.CauseProtocolPresentAbstractSyntaxErrorIgnoreAndNotify)}
	case ngapType.CauseProtocolPresentMessageNotCompatibleWithReceiverState:
		return EnumField{Label: "MessageNotCompatibleWithReceiverState", Value: int(ngapType.CauseProtocolPresentMessageNotCompatibleWithReceiverState)}
	case ngapType.CauseProtocolPresentSemanticError:
		return EnumField{Label: "SemanticError", Value: int(ngapType.CauseProtocolPresentSemanticError)}
	case ngapType.CauseProtocolPresentAbstractSyntaxErrorFalselyConstructedMessage:
		return EnumField{Label: "AbstractSyntaxErrorFalselyConstructedMessage", Value: int(ngapType.CauseProtocolPresentAbstractSyntaxErrorFalselyConstructedMessage)}
	case ngapType.CauseProtocolPresentUnspecified:
		return EnumField{Label: "Unspecified", Value: int(ngapType.CauseProtocolPresentUnspecified)}
	default:
		return EnumField{Label: "Unknown", Value: int(cause.Value)}
	}
}

func miscCauseToEnum(cause *ngapType.CauseMisc) EnumField {
	if cause == nil {
		return EnumField{Label: "nil", Value: 0}
	}

	switch cause.Value {
	case ngapType.CauseMiscPresentControlProcessingOverload:
		return EnumField{Label: "ControlProcessingOverload", Value: int(ngapType.CauseMiscPresentControlProcessingOverload)}
	case ngapType.CauseMiscPresentNotEnoughUserPlaneProcessingResources:
		return EnumField{Label: "NotEnoughUserPlaneProcessingResources", Value: int(ngapType.CauseMiscPresentNotEnoughUserPlaneProcessingResources)}
	case ngapType.CauseMiscPresentHardwareFailure:
		return EnumField{Label: "HardwareFailure", Value: int(ngapType.CauseMiscPresentHardwareFailure)}
	case ngapType.CauseMiscPresentOmIntervention:
		return EnumField{Label: "OmIntervention", Value: int(ngapType.CauseMiscPresentOmIntervention)}
	case ngapType.CauseMiscPresentUnknownPLMN:
		return EnumField{Label: "UnknownPLMN", Value: int(ngapType.CauseMiscPresentUnknownPLMN)}
	case ngapType.CauseMiscPresentUnspecified:
		return EnumField{Label: "Unspecified", Value: int(ngapType.CauseMiscPresentUnspecified)}
	default:
		return EnumField{Label: "Unknown", Value: int(cause.Value)}
	}
}
