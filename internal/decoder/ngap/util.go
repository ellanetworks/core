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
	Error       string    `json:"error,omitempty"`
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
		return makeEnum(int(id), "AllowedNSSAI", false)
	case ngapType.ProtocolIEIDAMFName:
		return makeEnum(int(id), "AMFName", false)
	case ngapType.ProtocolIEIDAMFOverloadResponse:
		return makeEnum(int(id), "AMFOverloadResponse", false)
	case ngapType.ProtocolIEIDAMFSetID:
		return makeEnum(int(id), "AMFSetID", false)
	case ngapType.ProtocolIEIDAMFTNLAssociationFailedToSetupList:
		return makeEnum(int(id), "AMFTNLAssociationFailedToSetupList", false)
	case ngapType.ProtocolIEIDAMFTNLAssociationSetupList:
		return makeEnum(int(id), "AMFTNLAssociationSetupList", false)
	case ngapType.ProtocolIEIDAMFTNLAssociationToAddList:
		return makeEnum(int(id), "AMFTNLAssociationToAddList", false)
	case ngapType.ProtocolIEIDAMFTNLAssociationToRemoveList:
		return makeEnum(int(id), "AMFTNLAssociationToRemoveList", false)
	case ngapType.ProtocolIEIDAMFTNLAssociationToUpdateList:
		return makeEnum(int(id), "AMFTNLAssociationToUpdateList", false)
	case ngapType.ProtocolIEIDAMFTrafficLoadReductionIndication:
		return makeEnum(int(id), "AMFTrafficLoadReductionIndication", false)
	case ngapType.ProtocolIEIDAMFUENGAPID:
		return makeEnum(int(id), "AMFUENGAPID", false)
	case ngapType.ProtocolIEIDAssistanceDataForPaging:
		return makeEnum(int(id), "AssistanceDataForPaging", false)
	case ngapType.ProtocolIEIDBroadcastCancelledAreaList:
		return makeEnum(int(id), "BroadcastCancelledAreaList", false)
	case ngapType.ProtocolIEIDBroadcastCompletedAreaList:
		return makeEnum(int(id), "BroadcastCompletedAreaList", false)
	case ngapType.ProtocolIEIDCancelAllWarningMessages:
		return makeEnum(int(id), "CancelAllWarningMessages", false)
	case ngapType.ProtocolIEIDCause:
		return makeEnum(int(id), "Cause", false)
	case ngapType.ProtocolIEIDCellIDListForRestart:
		return makeEnum(int(id), "CellIDListForRestart", false)
	case ngapType.ProtocolIEIDConcurrentWarningMessageInd:
		return makeEnum(int(id), "ConcurrentWarningMessageInd", false)
	case ngapType.ProtocolIEIDCoreNetworkAssistanceInformation:
		return makeEnum(int(id), "CoreNetworkAssistanceInformation", false)
	case ngapType.ProtocolIEIDCriticalityDiagnostics:
		return makeEnum(int(id), "CriticalityDiagnostics", false)
	case ngapType.ProtocolIEIDDataCodingScheme:
		return makeEnum(int(id), "DataCodingScheme", false)
	case ngapType.ProtocolIEIDDefaultPagingDRX:
		return makeEnum(int(id), "DefaultPagingDRX", false)
	case ngapType.ProtocolIEIDDirectForwardingPathAvailability:
		return makeEnum(int(id), "DirectForwardingPathAvailability", false)
	case ngapType.ProtocolIEIDEmergencyAreaIDListForRestart:
		return makeEnum(int(id), "EmergencyAreaIDListForRestart", false)
	case ngapType.ProtocolIEIDEmergencyFallbackIndicator:
		return makeEnum(int(id), "EmergencyFallbackIndicator", false)
	case ngapType.ProtocolIEIDEUTRACGI:
		return makeEnum(int(id), "EUTRACGI", false)
	case ngapType.ProtocolIEIDFiveGSTMSI:
		return makeEnum(int(id), "FiveGSTMSI", false)
	case ngapType.ProtocolIEIDGlobalRANNodeID:
		return makeEnum(int(id), "GlobalRANNodeID", false)
	case ngapType.ProtocolIEIDGUAMI:
		return makeEnum(int(id), "GUAMI", false)
	case ngapType.ProtocolIEIDHandoverType:
		return makeEnum(int(id), "HandoverType", false)
	case ngapType.ProtocolIEIDIMSVoiceSupportIndicator:
		return makeEnum(int(id), "IMSVoiceSupportIndicator", false)
	case ngapType.ProtocolIEIDIndexToRFSP:
		return makeEnum(int(id), "IndexToRFSP", false)
	case ngapType.ProtocolIEIDInfoOnRecommendedCellsAndRANNodesForPaging:
		return makeEnum(int(id), "InfoOnRecommendedCellsAndRANNodesForPaging", false)
	case ngapType.ProtocolIEIDLocationReportingRequestType:
		return makeEnum(int(id), "LocationReportingRequestType", false)
	case ngapType.ProtocolIEIDMaskedIMEISV:
		return makeEnum(int(id), "MaskedIMEISV", false)
	case ngapType.ProtocolIEIDMessageIdentifier:
		return makeEnum(int(id), "MessageIdentifier", false)
	case ngapType.ProtocolIEIDMobilityRestrictionList:
		return makeEnum(int(id), "MobilityRestrictionList", false)
	case ngapType.ProtocolIEIDNASC:
		return makeEnum(int(id), "NASC", false)
	case ngapType.ProtocolIEIDNASPDU:
		return makeEnum(int(id), "NASPDU", false)
	case ngapType.ProtocolIEIDNASSecurityParametersFromNGRAN:
		return makeEnum(int(id), "NASSecurityParametersFromNGRAN", false)
	case ngapType.ProtocolIEIDNewAMFUENGAPID:
		return makeEnum(int(id), "NewAMFUENGAPID", false)
	case ngapType.ProtocolIEIDNewSecurityContextInd:
		return makeEnum(int(id), "NewSecurityContextInd", false)
	case ngapType.ProtocolIEIDNGAPMessage:
		return makeEnum(int(id), "NGAPMessage", false)
	case ngapType.ProtocolIEIDNGRANCGI:
		return makeEnum(int(id), "NGRANCGI", false)
	case ngapType.ProtocolIEIDNGRANTraceID:
		return makeEnum(int(id), "NGRANTraceID", false)
	case ngapType.ProtocolIEIDNRCGI:
		return makeEnum(int(id), "NRCGI", false)
	case ngapType.ProtocolIEIDNRPPaPDU:
		return makeEnum(int(id), "NRPPaPDU", false)
	case ngapType.ProtocolIEIDNumberOfBroadcastsRequested:
		return makeEnum(int(id), "NumberOfBroadcastsRequested", false)
	case ngapType.ProtocolIEIDOldAMF:
		return makeEnum(int(id), "OldAMF", false)
	case ngapType.ProtocolIEIDOverloadStartNSSAIList:
		return makeEnum(int(id), "OverloadStartNSSAIList", false)
	case ngapType.ProtocolIEIDPagingDRX:
		return makeEnum(int(id), "PagingDRX", false)
	case ngapType.ProtocolIEIDPagingOrigin:
		return makeEnum(int(id), "PagingOrigin", false)
	case ngapType.ProtocolIEIDPagingPriority:
		return makeEnum(int(id), "PagingPriority", false)
	case ngapType.ProtocolIEIDPDUSessionResourceAdmittedList:
		return makeEnum(int(id), "PDUSessionResourceAdmittedList", false)
	case ngapType.ProtocolIEIDPDUSessionResourceFailedToModifyListModRes:
		return makeEnum(int(id), "PDUSessionResourceFailedToModifyListModRes", false)
	case ngapType.ProtocolIEIDPDUSessionResourceFailedToSetupListCxtRes:
		return makeEnum(int(id), "PDUSessionResourceFailedToSetupListCxtRes", false)
	case ngapType.ProtocolIEIDPDUSessionResourceFailedToSetupListHOAck:
		return makeEnum(int(id), "PDUSessionResourceFailedToSetupListHOAck", false)
	case ngapType.ProtocolIEIDPDUSessionResourceFailedToSetupListPSReq:
		return makeEnum(int(id), "PDUSessionResourceFailedToSetupListPSReq", false)
	case ngapType.ProtocolIEIDPDUSessionResourceFailedToSetupListSURes:
		return makeEnum(int(id), "PDUSessionResourceFailedToSetupListSURes", false)
	case ngapType.ProtocolIEIDPDUSessionResourceHandoverList:
		return makeEnum(int(id), "PDUSessionResourceHandoverList", false)
	case ngapType.ProtocolIEIDPDUSessionResourceListCxtRelCpl:
		return makeEnum(int(id), "PDUSessionResourceListCxtRelCpl", false)
	case ngapType.ProtocolIEIDPDUSessionResourceListHORqd:
		return makeEnum(int(id), "PDUSessionResourceListHORqd", false)
	case ngapType.ProtocolIEIDPDUSessionResourceModifyListModCfm:
		return makeEnum(int(id), "PDUSessionResourceModifyListModCfm", false)
	case ngapType.ProtocolIEIDPDUSessionResourceModifyListModInd:
		return makeEnum(int(id), "PDUSessionResourceModifyListModInd", false)
	case ngapType.ProtocolIEIDPDUSessionResourceModifyListModReq:
		return makeEnum(int(id), "PDUSessionResourceModifyListModReq", false)
	case ngapType.ProtocolIEIDPDUSessionResourceModifyListModRes:
		return makeEnum(int(id), "PDUSessionResourceModifyListModRes", false)
	case ngapType.ProtocolIEIDPDUSessionResourceNotifyList:
		return makeEnum(int(id), "PDUSessionResourceNotifyList", false)
	case ngapType.ProtocolIEIDPDUSessionResourceReleasedListNot:
		return makeEnum(int(id), "PDUSessionResourceReleasedListNot", false)
	case ngapType.ProtocolIEIDPDUSessionResourceReleasedListPSAck:
		return makeEnum(int(id), "PDUSessionResourceReleasedListPSAck", false)
	case ngapType.ProtocolIEIDPDUSessionResourceReleasedListPSFail:
		return makeEnum(int(id), "PDUSessionResourceReleasedListPSFail", false)
	case ngapType.ProtocolIEIDPDUSessionResourceReleasedListRelRes:
		return makeEnum(int(id), "PDUSessionResourceReleasedListRelRes", false)
	case ngapType.ProtocolIEIDPDUSessionResourceSetupListCxtReq:
		return makeEnum(int(id), "PDUSessionResourceSetupListCxtReq", false)
	case ngapType.ProtocolIEIDPDUSessionResourceSetupListCxtRes:
		return makeEnum(int(id), "PDUSessionResourceSetupListCxtRes", false)
	case ngapType.ProtocolIEIDPDUSessionResourceSetupListHOReq:
		return makeEnum(int(id), "PDUSessionResourceSetupListHOReq", false)
	case ngapType.ProtocolIEIDPDUSessionResourceSetupListSUReq:
		return makeEnum(int(id), "PDUSessionResourceSetupListSUReq", false)
	case ngapType.ProtocolIEIDPDUSessionResourceSetupListSURes:
		return makeEnum(int(id), "PDUSessionResourceSetupListSURes", false)
	case ngapType.ProtocolIEIDPDUSessionResourceToBeSwitchedDLList:
		return makeEnum(int(id), "PDUSessionResourceToBeSwitchedDLList", false)
	case ngapType.ProtocolIEIDPDUSessionResourceSwitchedList:
		return makeEnum(int(id), "PDUSessionResourceSwitchedList", false)
	case ngapType.ProtocolIEIDPDUSessionResourceToReleaseListHOCmd:
		return makeEnum(int(id), "PDUSessionResourceToReleaseListHOCmd", false)
	case ngapType.ProtocolIEIDPDUSessionResourceToReleaseListRelCmd:
		return makeEnum(int(id), "PDUSessionResourceToReleaseListRelCmd", false)
	case ngapType.ProtocolIEIDPLMNSupportList:
		return makeEnum(int(id), "PLMNSupportList", false)
	case ngapType.ProtocolIEIDPWSFailedCellIDList:
		return makeEnum(int(id), "PWSFailedCellIDList", false)
	case ngapType.ProtocolIEIDRANNodeName:
		return makeEnum(int(id), "RANNodeName", false)
	case ngapType.ProtocolIEIDRANPagingPriority:
		return makeEnum(int(id), "RANPagingPriority", false)
	case ngapType.ProtocolIEIDRANStatusTransferTransparentContainer:
		return makeEnum(int(id), "RANStatusTransferTransparentContainer", false)
	case ngapType.ProtocolIEIDRANUENGAPID:
		return makeEnum(int(id), "RANUENGAPID", false)
	case ngapType.ProtocolIEIDRelativeAMFCapacity:
		return makeEnum(int(id), "RelativeAMFCapacity", false)
	case ngapType.ProtocolIEIDRepetitionPeriod:
		return makeEnum(int(id), "RepetitionPeriod", false)
	case ngapType.ProtocolIEIDResetType:
		return makeEnum(int(id), "ResetType", false)
	case ngapType.ProtocolIEIDRoutingID:
		return makeEnum(int(id), "RoutingID", false)
	case ngapType.ProtocolIEIDRRCEstablishmentCause:
		return makeEnum(int(id), "RRCEstablishmentCause", false)
	case ngapType.ProtocolIEIDRRCInactiveTransitionReportRequest:
		return makeEnum(int(id), "RRCInactiveTransitionReportRequest", false)
	case ngapType.ProtocolIEIDRRCState:
		return makeEnum(int(id), "RRCState", false)
	case ngapType.ProtocolIEIDSecurityContext:
		return makeEnum(int(id), "SecurityContext", false)
	case ngapType.ProtocolIEIDSecurityKey:
		return makeEnum(int(id), "SecurityKey", false)
	case ngapType.ProtocolIEIDSerialNumber:
		return makeEnum(int(id), "SerialNumber", false)
	case ngapType.ProtocolIEIDServedGUAMIList:
		return makeEnum(int(id), "ServedGUAMIList", false)
	case ngapType.ProtocolIEIDSliceSupportList:
		return makeEnum(int(id), "SliceSupportList", false)
	case ngapType.ProtocolIEIDSONConfigurationTransferDL:
		return makeEnum(int(id), "SONConfigurationTransferDL", false)
	case ngapType.ProtocolIEIDSONConfigurationTransferUL:
		return makeEnum(int(id), "SONConfigurationTransferUL", false)
	case ngapType.ProtocolIEIDSourceAMFUENGAPID:
		return makeEnum(int(id), "SourceAMFUENGAPID", false)
	case ngapType.ProtocolIEIDSourceToTargetTransparentContainer:
		return makeEnum(int(id), "SourceToTargetTransparentContainer", false)
	case ngapType.ProtocolIEIDSupportedTAList:
		return makeEnum(int(id), "SupportedTAList", false)
	case ngapType.ProtocolIEIDTAIListForPaging:
		return makeEnum(int(id), "TAIListForPaging", false)
	case ngapType.ProtocolIEIDTAIListForRestart:
		return makeEnum(int(id), "TAIListForRestart", false)
	case ngapType.ProtocolIEIDTargetID:
		return makeEnum(int(id), "TargetID", false)
	case ngapType.ProtocolIEIDTargetToSourceTransparentContainer:
		return makeEnum(int(id), "TargetToSourceTransparentContainer", false)
	case ngapType.ProtocolIEIDTimeToWait:
		return makeEnum(int(id), "TimeToWait", false)
	case ngapType.ProtocolIEIDTraceActivation:
		return makeEnum(int(id), "TraceActivation", false)
	case ngapType.ProtocolIEIDTraceCollectionEntityIPAddress:
		return makeEnum(int(id), "TraceCollectionEntityIPAddress", false)
	case ngapType.ProtocolIEIDUEAggregateMaximumBitRate:
		return makeEnum(int(id), "UEAggregateMaximumBitRate", false)
	case ngapType.ProtocolIEIDUEAssociatedLogicalNGConnectionList:
		return makeEnum(int(id), "UEAssociatedLogicalNGConnectionList", false)
	case ngapType.ProtocolIEIDUEContextRequest:
		return makeEnum(int(id), "UEContextRequest", false)
	case ngapType.ProtocolIEIDUENGAPIDs:
		return makeEnum(int(id), "UENGAPIDs", false)
	case ngapType.ProtocolIEIDUEPagingIdentity:
		return makeEnum(int(id), "UEPagingIdentity", false)
	case ngapType.ProtocolIEIDUEPresenceInAreaOfInterestList:
		return makeEnum(int(id), "UEPresenceInAreaOfInterestList", false)
	case ngapType.ProtocolIEIDUERadioCapability:
		return makeEnum(int(id), "UERadioCapability", false)
	case ngapType.ProtocolIEIDUERadioCapabilityForPaging:
		return makeEnum(int(id), "UERadioCapabilityForPaging", false)
	case ngapType.ProtocolIEIDUESecurityCapabilities:
		return makeEnum(int(id), "UESecurityCapabilities", false)
	case ngapType.ProtocolIEIDUnavailableGUAMIList:
		return makeEnum(int(id), "UnavailableGUAMIList", false)
	case ngapType.ProtocolIEIDUserLocationInformation:
		return makeEnum(int(id), "UserLocationInformation", false)
	case ngapType.ProtocolIEIDWarningAreaList:
		return makeEnum(int(id), "WarningAreaList", false)
	case ngapType.ProtocolIEIDWarningMessageContents:
		return makeEnum(int(id), "WarningMessageContents", false)
	case ngapType.ProtocolIEIDWarningSecurityInfo:
		return makeEnum(int(id), "WarningSecurityInfo", false)
	case ngapType.ProtocolIEIDWarningType:
		return makeEnum(int(id), "WarningType", false)
	case ngapType.ProtocolIEIDAdditionalULNGUUPTNLInformation:
		return makeEnum(int(id), "AdditionalULNGUUPTNLInformation", false)
	case ngapType.ProtocolIEIDDataForwardingNotPossible:
		return makeEnum(int(id), "DataForwardingNotPossible", false)
	case ngapType.ProtocolIEIDDLNGUUPTNLInformation:
		return makeEnum(int(id), "DLNGUUPTNLInformation", false)
	case ngapType.ProtocolIEIDNetworkInstance:
		return makeEnum(int(id), "NetworkInstance", false)
	case ngapType.ProtocolIEIDPDUSessionAggregateMaximumBitRate:
		return makeEnum(int(id), "PDUSessionAggregateMaximumBitRate", false)
	case ngapType.ProtocolIEIDPDUSessionResourceFailedToModifyListModCfm:
		return makeEnum(int(id), "PDUSessionResourceFailedToModifyListModCfm", false)
	case ngapType.ProtocolIEIDPDUSessionResourceFailedToSetupListCxtFail:
		return makeEnum(int(id), "PDUSessionResourceFailedToSetupListCxtFail", false)
	case ngapType.ProtocolIEIDPDUSessionResourceListCxtRelReq:
		return makeEnum(int(id), "PDUSessionResourceListCxtRelReq", false)
	case ngapType.ProtocolIEIDPDUSessionType:
		return makeEnum(int(id), "PDUSessionType", false)
	case ngapType.ProtocolIEIDQosFlowAddOrModifyRequestList:
		return makeEnum(int(id), "QosFlowAddOrModifyRequestList", false)
	case ngapType.ProtocolIEIDQosFlowSetupRequestList:
		return makeEnum(int(id), "QosFlowSetupRequestList", false)
	case ngapType.ProtocolIEIDQosFlowToReleaseList:
		return makeEnum(int(id), "QosFlowToReleaseList", false)
	case ngapType.ProtocolIEIDSecurityIndication:
		return makeEnum(int(id), "SecurityIndication", false)
	case ngapType.ProtocolIEIDULNGUUPTNLInformation:
		return makeEnum(int(id), "ULNGUUPTNLInformation", false)
	case ngapType.ProtocolIEIDULNGUUPTNLModifyList:
		return makeEnum(int(id), "ULNGUUPTNLModifyList", false)
	case ngapType.ProtocolIEIDWarningAreaCoordinates:
		return makeEnum(int(id), "WarningAreaCoordinates", false)
	case ngapType.ProtocolIEIDPDUSessionResourceSecondaryRATUsageList:
		return makeEnum(int(id), "PDUSessionResourceSecondaryRATUsageList", false)
	case ngapType.ProtocolIEIDHandoverFlag:
		return makeEnum(int(id), "HandoverFlag", false)
	case ngapType.ProtocolIEIDSecondaryRATUsageInformation:
		return makeEnum(int(id), "SecondaryRATUsageInformation", false)
	case ngapType.ProtocolIEIDPDUSessionResourceReleaseResponseTransfer:
		return makeEnum(int(id), "PDUSessionResourceReleaseResponseTransfer", false)
	case ngapType.ProtocolIEIDRedirectionVoiceFallback:
		return makeEnum(int(id), "RedirectionVoiceFallback", false)
	case ngapType.ProtocolIEIDUERetentionInformation:
		return makeEnum(int(id), "UERetentionInformation", false)
	case ngapType.ProtocolIEIDSNSSAI:
		return makeEnum(int(id), "SNSSAI", false)
	case ngapType.ProtocolIEIDPSCellInformation:
		return makeEnum(int(id), "PSCellInformation", false)
	case ngapType.ProtocolIEIDLastEUTRANPLMNIdentity:
		return makeEnum(int(id), "LastEUTRANPLMNIdentity", false)
	case ngapType.ProtocolIEIDMaximumIntegrityProtectedDataRateDL:
		return makeEnum(int(id), "MaximumIntegrityProtectedDataRateDL", false)
	case ngapType.ProtocolIEIDAdditionalDLForwardingUPTNLInformation:
		return makeEnum(int(id), "AdditionalDLForwardingUPTNLInformation", false)
	case ngapType.ProtocolIEIDAdditionalDLUPTNLInformationForHOList:
		return makeEnum(int(id), "AdditionalDLUPTNLInformationForHOList", false)
	case ngapType.ProtocolIEIDAdditionalNGUUPTNLInformation:
		return makeEnum(int(id), "AdditionalNGUUPTNLInformation", false)
	case ngapType.ProtocolIEIDAdditionalDLQosFlowPerTNLInformation:
		return makeEnum(int(id), "AdditionalDLQosFlowPerTNLInformation", false)
	case ngapType.ProtocolIEIDSecurityResult:
		return makeEnum(int(id), "SecurityResult", false)
	case ngapType.ProtocolIEIDENDCSONConfigurationTransferDL:
		return makeEnum(157, "ENDCSONConfigurationTransferDL", false)
	case ngapType.ProtocolIEIDENDCSONConfigurationTransferUL:
		return makeEnum(158, "ENDCSONConfigurationTransferUL", false)
	default:
		return makeEnum(int(id), "", true)
	}
}

func causeToEnum(cause ngapType.Cause) EnumField {
	switch cause.Present {
	case ngapType.CausePresentRadioNetwork:
		return radioNetworkCauseToEnum(*cause.RadioNetwork)
	case ngapType.CausePresentTransport:
		return transportCauseToEnum(*cause.Transport)
	case ngapType.CausePresentNas:
		return nasCauseToEnum(*cause.Nas)
	case ngapType.CausePresentProtocol:
		return protocolCauseToEnum(*cause.Protocol)
	case ngapType.CausePresentMisc:
		return miscCauseToEnum(*cause.Misc)
	default:
		return makeEnum(cause.Present, "", true)
	}
}

func radioNetworkCauseToEnum(cause ngapType.CauseRadioNetwork) EnumField {
	switch cause.Value {
	case ngapType.CauseRadioNetworkPresentUnspecified:
		return makeEnum(int(cause.Value), "Unspecified", false)
	case ngapType.CauseRadioNetworkPresentTxnrelocoverallExpiry:
		return makeEnum(int(cause.Value), "TxNRelocOverallExpiry", false)
	case ngapType.CauseRadioNetworkPresentSuccessfulHandover:
		return makeEnum(int(cause.Value), "SuccessfulHandover", false)
	case ngapType.CauseRadioNetworkPresentReleaseDueToNgranGeneratedReason:
		return makeEnum(int(cause.Value), "ReleaseDueToNgranGeneratedReason", false)
	case ngapType.CauseRadioNetworkPresentReleaseDueTo5gcGeneratedReason:
		return makeEnum(int(cause.Value), "ReleaseDueTo5gcGeneratedReason", false)
	case ngapType.CauseRadioNetworkPresentHandoverCancelled:
		return makeEnum(int(cause.Value), "HandoverCancelled", false)
	case ngapType.CauseRadioNetworkPresentPartialHandover:
		return makeEnum(int(cause.Value), "PartialHandover", false)
	case ngapType.CauseRadioNetworkPresentHoFailureInTarget5GCNgranNodeOrTargetSystem:
		return makeEnum(int(cause.Value), "HoFailureInTarget5GCNgranNodeOrTargetSystem", false)
	case ngapType.CauseRadioNetworkPresentHoTargetNotAllowed:
		return makeEnum(int(cause.Value), "HoTargetNotAllowed", false)
	case ngapType.CauseRadioNetworkPresentTngrelocoverallExpiry:
		return makeEnum(int(cause.Value), "TngRelocOverallExpiry", false)
	case ngapType.CauseRadioNetworkPresentTngrelocprepExpiry:
		return makeEnum(int(cause.Value), "TngRelocPrepExpiry", false)
	case ngapType.CauseRadioNetworkPresentCellNotAvailable:
		return makeEnum(int(ngapType.CauseRadioNetworkPresentCellNotAvailable), "CellNotAvailable", false)
	case ngapType.CauseRadioNetworkPresentUnknownTargetID:
		return makeEnum(int(ngapType.CauseRadioNetworkPresentUnknownTargetID), "UnknownTargetID", false)
	case ngapType.CauseRadioNetworkPresentNoRadioResourcesAvailableInTargetCell:
		return makeEnum(int(ngapType.CauseRadioNetworkPresentNoRadioResourcesAvailableInTargetCell), "NoRadioResourcesAvailableInTargetCell", false)
	case ngapType.CauseRadioNetworkPresentUnknownLocalUENGAPID:
		return makeEnum(int(ngapType.CauseRadioNetworkPresentUnknownLocalUENGAPID), "UnknownLocalUENGAPID", false)
	case ngapType.CauseRadioNetworkPresentInconsistentRemoteUENGAPID:
		return makeEnum(int(ngapType.CauseRadioNetworkPresentInconsistentRemoteUENGAPID), "InconsistentRemoteUENGAPID", false)
	case ngapType.CauseRadioNetworkPresentHandoverDesirableForRadioReason:
		return makeEnum(int(ngapType.CauseRadioNetworkPresentHandoverDesirableForRadioReason), "HandoverDesirableForRadioReason", false)
	case ngapType.CauseRadioNetworkPresentTimeCriticalHandover:
		return makeEnum(int(ngapType.CauseRadioNetworkPresentTimeCriticalHandover), "TimeCriticalHandover", false)
	case ngapType.CauseRadioNetworkPresentResourceOptimisationHandover:
		return makeEnum(int(ngapType.CauseRadioNetworkPresentResourceOptimisationHandover), "ResourceOptimisationHandover", false)
	case ngapType.CauseRadioNetworkPresentReduceLoadInServingCell:
		return makeEnum(int(ngapType.CauseRadioNetworkPresentReduceLoadInServingCell), "ReduceLoadInServingCell", false)
	case ngapType.CauseRadioNetworkPresentUserInactivity:
		return makeEnum(int(ngapType.CauseRadioNetworkPresentUserInactivity), "UserInactivity", false)
	case ngapType.CauseRadioNetworkPresentRadioConnectionWithUeLost:
		return makeEnum(int(ngapType.CauseRadioNetworkPresentRadioConnectionWithUeLost), "RadioConnectionWithUeLost", false)
	case ngapType.CauseRadioNetworkPresentRadioResourcesNotAvailable:
		return makeEnum(int(ngapType.CauseRadioNetworkPresentRadioResourcesNotAvailable), "RadioResourcesNotAvailable", false)
	case ngapType.CauseRadioNetworkPresentInvalidQosCombination:
		return makeEnum(int(ngapType.CauseRadioNetworkPresentInvalidQosCombination), "InvalidQosCombination", false)
	case ngapType.CauseRadioNetworkPresentFailureInRadioInterfaceProcedure:
		return makeEnum(int(ngapType.CauseRadioNetworkPresentFailureInRadioInterfaceProcedure), "FailureInRadioInterfaceProcedure", false)
	case ngapType.CauseRadioNetworkPresentInteractionWithOtherProcedure:
		return makeEnum(int(ngapType.CauseRadioNetworkPresentInteractionWithOtherProcedure), "InteractionWithOtherProcedure", false)
	case ngapType.CauseRadioNetworkPresentUnknownPDUSessionID:
		return makeEnum(int(ngapType.CauseRadioNetworkPresentUnknownPDUSessionID), "UnknownPDUSessionID", false)
	case ngapType.CauseRadioNetworkPresentUnkownQosFlowID:
		return makeEnum(int(ngapType.CauseRadioNetworkPresentUnkownQosFlowID), "UnkownQosFlowID", false)
	case ngapType.CauseRadioNetworkPresentMultiplePDUSessionIDInstances:
		return makeEnum(int(ngapType.CauseRadioNetworkPresentMultiplePDUSessionIDInstances), "MultiplePDUSessionIDInstances", false)
	case ngapType.CauseRadioNetworkPresentMultipleQosFlowIDInstances:
		return makeEnum(int(ngapType.CauseRadioNetworkPresentMultipleQosFlowIDInstances), "MultipleQosFlowIDInstances", false)
	case ngapType.CauseRadioNetworkPresentEncryptionAndOrIntegrityProtectionAlgorithmsNotSupported:
		return makeEnum(int(ngapType.CauseRadioNetworkPresentEncryptionAndOrIntegrityProtectionAlgorithmsNotSupported), "EncryptionAndOrIntegrityProtectionAlgorithmsNotSupported", false)
	case ngapType.CauseRadioNetworkPresentNgIntraSystemHandoverTriggered:
		return makeEnum(int(ngapType.CauseRadioNetworkPresentNgIntraSystemHandoverTriggered), "NgIntraSystemHandoverTriggered", false)
	case ngapType.CauseRadioNetworkPresentNgInterSystemHandoverTriggered:
		return makeEnum(int(ngapType.CauseRadioNetworkPresentNgInterSystemHandoverTriggered), "NgInterSystemHandoverTriggered", false)
	case ngapType.CauseRadioNetworkPresentXnHandoverTriggered:
		return makeEnum(int(ngapType.CauseRadioNetworkPresentXnHandoverTriggered), "XnHandoverTriggered", false)
	case ngapType.CauseRadioNetworkPresentNotSupported5QIValue:
		return makeEnum(int(ngapType.CauseRadioNetworkPresentNotSupported5QIValue), "NotSupported5QIValue", false)
	case ngapType.CauseRadioNetworkPresentUeContextTransfer:
		return makeEnum(int(ngapType.CauseRadioNetworkPresentUeContextTransfer), "UeContextTransfer", false)
	case ngapType.CauseRadioNetworkPresentImsVoiceEpsFallbackOrRatFallbackTriggered:
		return makeEnum(int(ngapType.CauseRadioNetworkPresentImsVoiceEpsFallbackOrRatFallbackTriggered), "ImsVoiceEpsFallbackOrRatFallbackTriggered", false)
	case ngapType.CauseRadioNetworkPresentUpIntegrityProtectionNotPossible:
		return makeEnum(int(ngapType.CauseRadioNetworkPresentUpIntegrityProtectionNotPossible), "UpIntegrityProtectionNotPossible", false)
	case ngapType.CauseRadioNetworkPresentUpConfidentialityProtectionNotPossible:
		return makeEnum(int(ngapType.CauseRadioNetworkPresentUpConfidentialityProtectionNotPossible), "UpConfidentialityProtectionNotPossible", false)
	case ngapType.CauseRadioNetworkPresentSliceNotSupported:
		return makeEnum(int(ngapType.CauseRadioNetworkPresentSliceNotSupported), "SliceNotSupported", false)
	case ngapType.CauseRadioNetworkPresentUeInRrcInactiveStateNotReachable:
		return makeEnum(int(ngapType.CauseRadioNetworkPresentUeInRrcInactiveStateNotReachable), "UeInRrcInactiveStateNotReachable", false)
	case ngapType.CauseRadioNetworkPresentRedirection:
		return makeEnum(int(ngapType.CauseRadioNetworkPresentRedirection), "Redirection", false)
	case ngapType.CauseRadioNetworkPresentResourcesNotAvailableForTheSlice:
		return makeEnum(int(ngapType.CauseRadioNetworkPresentResourcesNotAvailableForTheSlice), "ResourcesNotAvailableForTheSlice", false)
	case ngapType.CauseRadioNetworkPresentUeMaxIntegrityProtectedDataRateReason:
		return makeEnum(int(ngapType.CauseRadioNetworkPresentUeMaxIntegrityProtectedDataRateReason), "UeMaxIntegrityProtectedDataRateReason", false)
	case ngapType.CauseRadioNetworkPresentReleaseDueToCnDetectedMobility:
		return makeEnum(int(ngapType.CauseRadioNetworkPresentReleaseDueToCnDetectedMobility), "ReleaseDueToCnDetectedMobility", false)
	case ngapType.CauseRadioNetworkPresentN26InterfaceNotAvailable:
		return makeEnum(int(ngapType.CauseRadioNetworkPresentN26InterfaceNotAvailable), "N26InterfaceNotAvailable", false)
	case ngapType.CauseRadioNetworkPresentReleaseDueToPreEmption:
		return makeEnum(int(ngapType.CauseRadioNetworkPresentReleaseDueToPreEmption), "ReleaseDueToPreEmption", false)
	default:
		return makeEnum(int(cause.Value), "", true)
	}
}

func transportCauseToEnum(cause ngapType.CauseTransport) EnumField {
	switch cause.Value {
	case ngapType.CauseTransportPresentTransportResourceUnavailable:
		return makeEnum(int(cause.Value), "TransportResourceUnavailable", false)
	case ngapType.CauseTransportPresentUnspecified:
		return makeEnum(int(cause.Value), "Unspecified", false)
	default:
		return makeEnum(int(cause.Value), "", true)
	}
}

func nasCauseToEnum(cause ngapType.CauseNas) EnumField {
	switch cause.Value {
	case ngapType.CauseNasPresentNormalRelease:
		return makeEnum(int(cause.Value), "NormalRelease", false)
	case ngapType.CauseNasPresentAuthenticationFailure:
		return makeEnum(int(cause.Value), "AuthenticationFailure", false)
	case ngapType.CauseNasPresentDeregister:
		return makeEnum(int(cause.Value), "Deregister", false)
	case ngapType.CauseNasPresentUnspecified:
		return makeEnum(int(cause.Value), "Unspecified", false)
	default:
		return makeEnum(int(cause.Value), "", true)
	}
}

func protocolCauseToEnum(cause ngapType.CauseProtocol) EnumField {
	switch cause.Value {
	case ngapType.CauseProtocolPresentTransferSyntaxError:
		return makeEnum(int(cause.Value), "TransferSyntaxError", false)
	case ngapType.CauseProtocolPresentAbstractSyntaxErrorReject:
		return makeEnum(int(cause.Value), "AbstractSyntaxErrorReject", false)
	case ngapType.CauseProtocolPresentAbstractSyntaxErrorIgnoreAndNotify:
		return makeEnum(int(cause.Value), "AbstractSyntaxErrorIgnoreAndNotify", false)
	case ngapType.CauseProtocolPresentMessageNotCompatibleWithReceiverState:
		return makeEnum(int(cause.Value), "MessageNotCompatibleWithReceiverState", false)
	case ngapType.CauseProtocolPresentSemanticError:
		return makeEnum(int(cause.Value), "SemanticError", false)
	case ngapType.CauseProtocolPresentAbstractSyntaxErrorFalselyConstructedMessage:
		return makeEnum(int(cause.Value), "AbstractSyntaxErrorFalselyConstructedMessage", false)
	case ngapType.CauseProtocolPresentUnspecified:
		return makeEnum(int(cause.Value), "Unspecified", false)
	default:
		return makeEnum(int(cause.Value), "", true)
	}
}

func miscCauseToEnum(cause ngapType.CauseMisc) EnumField {
	switch cause.Value {
	case ngapType.CauseMiscPresentControlProcessingOverload:
		return makeEnum(int(cause.Value), "ControlProcessingOverload", false)
	case ngapType.CauseMiscPresentNotEnoughUserPlaneProcessingResources:
		return makeEnum(int(cause.Value), "NotEnoughUserPlaneProcessingResources", false)
	case ngapType.CauseMiscPresentHardwareFailure:
		return makeEnum(int(cause.Value), "HardwareFailure", false)
	case ngapType.CauseMiscPresentOmIntervention:
		return makeEnum(int(cause.Value), "OmIntervention", false)
	case ngapType.CauseMiscPresentUnknownPLMN:
		return makeEnum(int(cause.Value), "UnknownPLMN", false)
	case ngapType.CauseMiscPresentUnspecified:
		return makeEnum(int(cause.Value), "Unspecified", false)
	default:
		return makeEnum(int(cause.Value), "", true)
	}
}
