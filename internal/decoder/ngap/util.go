package ngap

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/ellanetworks/core/internal/decoder/utils"
	"github.com/omec-project/ngap/aper"
	"github.com/omec-project/ngap/ngapType"
)

const ntpToUnixOffset = 2208988800 // seconds between 1900-01-01 and 1970-01-01

type IE struct {
	ID          utils.EnumField[int64]  `json:"id"`
	Criticality utils.EnumField[uint64] `json:"criticality"`
	Value       any                     `json:"value,omitempty"`

	Error string `json:"error,omitempty"` // Reserved field for decoding errors
}

func criticalityToEnum(c aper.Enumerated) utils.EnumField[uint64] {
	switch c {
	case ngapType.CriticalityPresentReject:
		return utils.MakeEnum(uint64(c), "Reject", false)
	case ngapType.CriticalityPresentIgnore:
		return utils.MakeEnum(uint64(c), "Ignore", false)
	case ngapType.CriticalityPresentNotify:
		return utils.MakeEnum(uint64(c), "Notify", false)
	default:
		return utils.MakeEnum(uint64(c), "", true)
	}
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

func protocolIEIDToEnum(id int64) utils.EnumField[int64] {
	switch id {
	case ngapType.ProtocolIEIDAllowedNSSAI:
		return utils.MakeEnum(id, "AllowedNSSAI", false)
	case ngapType.ProtocolIEIDAMFName:
		return utils.MakeEnum(id, "AMFName", false)
	case ngapType.ProtocolIEIDAMFOverloadResponse:
		return utils.MakeEnum(id, "AMFOverloadResponse", false)
	case ngapType.ProtocolIEIDAMFSetID:
		return utils.MakeEnum(id, "AMFSetID", false)
	case ngapType.ProtocolIEIDAMFTNLAssociationFailedToSetupList:
		return utils.MakeEnum(id, "AMFTNLAssociationFailedToSetupList", false)
	case ngapType.ProtocolIEIDAMFTNLAssociationSetupList:
		return utils.MakeEnum(id, "AMFTNLAssociationSetupList", false)
	case ngapType.ProtocolIEIDAMFTNLAssociationToAddList:
		return utils.MakeEnum(id, "AMFTNLAssociationToAddList", false)
	case ngapType.ProtocolIEIDAMFTNLAssociationToRemoveList:
		return utils.MakeEnum(id, "AMFTNLAssociationToRemoveList", false)
	case ngapType.ProtocolIEIDAMFTNLAssociationToUpdateList:
		return utils.MakeEnum(id, "AMFTNLAssociationToUpdateList", false)
	case ngapType.ProtocolIEIDAMFTrafficLoadReductionIndication:
		return utils.MakeEnum(id, "AMFTrafficLoadReductionIndication", false)
	case ngapType.ProtocolIEIDAMFUENGAPID:
		return utils.MakeEnum(id, "AMFUENGAPID", false)
	case ngapType.ProtocolIEIDAssistanceDataForPaging:
		return utils.MakeEnum(id, "AssistanceDataForPaging", false)
	case ngapType.ProtocolIEIDBroadcastCancelledAreaList:
		return utils.MakeEnum(id, "BroadcastCancelledAreaList", false)
	case ngapType.ProtocolIEIDBroadcastCompletedAreaList:
		return utils.MakeEnum(id, "BroadcastCompletedAreaList", false)
	case ngapType.ProtocolIEIDCancelAllWarningMessages:
		return utils.MakeEnum(id, "CancelAllWarningMessages", false)
	case ngapType.ProtocolIEIDCause:
		return utils.MakeEnum(id, "Cause", false)
	case ngapType.ProtocolIEIDCellIDListForRestart:
		return utils.MakeEnum(id, "CellIDListForRestart", false)
	case ngapType.ProtocolIEIDConcurrentWarningMessageInd:
		return utils.MakeEnum(id, "ConcurrentWarningMessageInd", false)
	case ngapType.ProtocolIEIDCoreNetworkAssistanceInformation:
		return utils.MakeEnum(id, "CoreNetworkAssistanceInformation", false)
	case ngapType.ProtocolIEIDCriticalityDiagnostics:
		return utils.MakeEnum(id, "CriticalityDiagnostics", false)
	case ngapType.ProtocolIEIDDataCodingScheme:
		return utils.MakeEnum(id, "DataCodingScheme", false)
	case ngapType.ProtocolIEIDDefaultPagingDRX:
		return utils.MakeEnum(id, "DefaultPagingDRX", false)
	case ngapType.ProtocolIEIDDirectForwardingPathAvailability:
		return utils.MakeEnum(id, "DirectForwardingPathAvailability", false)
	case ngapType.ProtocolIEIDEmergencyAreaIDListForRestart:
		return utils.MakeEnum(id, "EmergencyAreaIDListForRestart", false)
	case ngapType.ProtocolIEIDEmergencyFallbackIndicator:
		return utils.MakeEnum(id, "EmergencyFallbackIndicator", false)
	case ngapType.ProtocolIEIDEUTRACGI:
		return utils.MakeEnum(id, "EUTRACGI", false)
	case ngapType.ProtocolIEIDFiveGSTMSI:
		return utils.MakeEnum(id, "FiveGSTMSI", false)
	case ngapType.ProtocolIEIDGlobalRANNodeID:
		return utils.MakeEnum(id, "GlobalRANNodeID", false)
	case ngapType.ProtocolIEIDGUAMI:
		return utils.MakeEnum(id, "GUAMI", false)
	case ngapType.ProtocolIEIDHandoverType:
		return utils.MakeEnum(id, "HandoverType", false)
	case ngapType.ProtocolIEIDIMSVoiceSupportIndicator:
		return utils.MakeEnum(id, "IMSVoiceSupportIndicator", false)
	case ngapType.ProtocolIEIDIndexToRFSP:
		return utils.MakeEnum(id, "IndexToRFSP", false)
	case ngapType.ProtocolIEIDInfoOnRecommendedCellsAndRANNodesForPaging:
		return utils.MakeEnum(id, "InfoOnRecommendedCellsAndRANNodesForPaging", false)
	case ngapType.ProtocolIEIDLocationReportingRequestType:
		return utils.MakeEnum(id, "LocationReportingRequestType", false)
	case ngapType.ProtocolIEIDMaskedIMEISV:
		return utils.MakeEnum(id, "MaskedIMEISV", false)
	case ngapType.ProtocolIEIDMessageIdentifier:
		return utils.MakeEnum(id, "MessageIdentifier", false)
	case ngapType.ProtocolIEIDMobilityRestrictionList:
		return utils.MakeEnum(id, "MobilityRestrictionList", false)
	case ngapType.ProtocolIEIDNASC:
		return utils.MakeEnum(id, "NASC", false)
	case ngapType.ProtocolIEIDNASPDU:
		return utils.MakeEnum(id, "NASPDU", false)
	case ngapType.ProtocolIEIDNASSecurityParametersFromNGRAN:
		return utils.MakeEnum(id, "NASSecurityParametersFromNGRAN", false)
	case ngapType.ProtocolIEIDNewAMFUENGAPID:
		return utils.MakeEnum(id, "NewAMFUENGAPID", false)
	case ngapType.ProtocolIEIDNewSecurityContextInd:
		return utils.MakeEnum(id, "NewSecurityContextInd", false)
	case ngapType.ProtocolIEIDNGAPMessage:
		return utils.MakeEnum(id, "NGAPMessage", false)
	case ngapType.ProtocolIEIDNGRANCGI:
		return utils.MakeEnum(id, "NGRANCGI", false)
	case ngapType.ProtocolIEIDNGRANTraceID:
		return utils.MakeEnum(id, "NGRANTraceID", false)
	case ngapType.ProtocolIEIDNRCGI:
		return utils.MakeEnum(id, "NRCGI", false)
	case ngapType.ProtocolIEIDNRPPaPDU:
		return utils.MakeEnum(id, "NRPPaPDU", false)
	case ngapType.ProtocolIEIDNumberOfBroadcastsRequested:
		return utils.MakeEnum(id, "NumberOfBroadcastsRequested", false)
	case ngapType.ProtocolIEIDOldAMF:
		return utils.MakeEnum(id, "OldAMF", false)
	case ngapType.ProtocolIEIDOverloadStartNSSAIList:
		return utils.MakeEnum(id, "OverloadStartNSSAIList", false)
	case ngapType.ProtocolIEIDPagingDRX:
		return utils.MakeEnum(id, "PagingDRX", false)
	case ngapType.ProtocolIEIDPagingOrigin:
		return utils.MakeEnum(id, "PagingOrigin", false)
	case ngapType.ProtocolIEIDPagingPriority:
		return utils.MakeEnum(id, "PagingPriority", false)
	case ngapType.ProtocolIEIDPDUSessionResourceAdmittedList:
		return utils.MakeEnum(id, "PDUSessionResourceAdmittedList", false)
	case ngapType.ProtocolIEIDPDUSessionResourceFailedToModifyListModRes:
		return utils.MakeEnum(id, "PDUSessionResourceFailedToModifyListModRes", false)
	case ngapType.ProtocolIEIDPDUSessionResourceFailedToSetupListCxtRes:
		return utils.MakeEnum(id, "PDUSessionResourceFailedToSetupListCxtRes", false)
	case ngapType.ProtocolIEIDPDUSessionResourceFailedToSetupListHOAck:
		return utils.MakeEnum(id, "PDUSessionResourceFailedToSetupListHOAck", false)
	case ngapType.ProtocolIEIDPDUSessionResourceFailedToSetupListPSReq:
		return utils.MakeEnum(id, "PDUSessionResourceFailedToSetupListPSReq", false)
	case ngapType.ProtocolIEIDPDUSessionResourceFailedToSetupListSURes:
		return utils.MakeEnum(id, "PDUSessionResourceFailedToSetupListSURes", false)
	case ngapType.ProtocolIEIDPDUSessionResourceHandoverList:
		return utils.MakeEnum(id, "PDUSessionResourceHandoverList", false)
	case ngapType.ProtocolIEIDPDUSessionResourceListCxtRelCpl:
		return utils.MakeEnum(id, "PDUSessionResourceListCxtRelCpl", false)
	case ngapType.ProtocolIEIDPDUSessionResourceListHORqd:
		return utils.MakeEnum(id, "PDUSessionResourceListHORqd", false)
	case ngapType.ProtocolIEIDPDUSessionResourceModifyListModCfm:
		return utils.MakeEnum(id, "PDUSessionResourceModifyListModCfm", false)
	case ngapType.ProtocolIEIDPDUSessionResourceModifyListModInd:
		return utils.MakeEnum(id, "PDUSessionResourceModifyListModInd", false)
	case ngapType.ProtocolIEIDPDUSessionResourceModifyListModReq:
		return utils.MakeEnum(id, "PDUSessionResourceModifyListModReq", false)
	case ngapType.ProtocolIEIDPDUSessionResourceModifyListModRes:
		return utils.MakeEnum(id, "PDUSessionResourceModifyListModRes", false)
	case ngapType.ProtocolIEIDPDUSessionResourceNotifyList:
		return utils.MakeEnum(id, "PDUSessionResourceNotifyList", false)
	case ngapType.ProtocolIEIDPDUSessionResourceReleasedListNot:
		return utils.MakeEnum(id, "PDUSessionResourceReleasedListNot", false)
	case ngapType.ProtocolIEIDPDUSessionResourceReleasedListPSAck:
		return utils.MakeEnum(id, "PDUSessionResourceReleasedListPSAck", false)
	case ngapType.ProtocolIEIDPDUSessionResourceReleasedListPSFail:
		return utils.MakeEnum(id, "PDUSessionResourceReleasedListPSFail", false)
	case ngapType.ProtocolIEIDPDUSessionResourceReleasedListRelRes:
		return utils.MakeEnum(id, "PDUSessionResourceReleasedListRelRes", false)
	case ngapType.ProtocolIEIDPDUSessionResourceSetupListCxtReq:
		return utils.MakeEnum(id, "PDUSessionResourceSetupListCxtReq", false)
	case ngapType.ProtocolIEIDPDUSessionResourceSetupListCxtRes:
		return utils.MakeEnum(id, "PDUSessionResourceSetupListCxtRes", false)
	case ngapType.ProtocolIEIDPDUSessionResourceSetupListHOReq:
		return utils.MakeEnum(id, "PDUSessionResourceSetupListHOReq", false)
	case ngapType.ProtocolIEIDPDUSessionResourceSetupListSUReq:
		return utils.MakeEnum(id, "PDUSessionResourceSetupListSUReq", false)
	case ngapType.ProtocolIEIDPDUSessionResourceSetupListSURes:
		return utils.MakeEnum(id, "PDUSessionResourceSetupListSURes", false)
	case ngapType.ProtocolIEIDPDUSessionResourceToBeSwitchedDLList:
		return utils.MakeEnum(id, "PDUSessionResourceToBeSwitchedDLList", false)
	case ngapType.ProtocolIEIDPDUSessionResourceSwitchedList:
		return utils.MakeEnum(id, "PDUSessionResourceSwitchedList", false)
	case ngapType.ProtocolIEIDPDUSessionResourceToReleaseListHOCmd:
		return utils.MakeEnum(id, "PDUSessionResourceToReleaseListHOCmd", false)
	case ngapType.ProtocolIEIDPDUSessionResourceToReleaseListRelCmd:
		return utils.MakeEnum(id, "PDUSessionResourceToReleaseListRelCmd", false)
	case ngapType.ProtocolIEIDPLMNSupportList:
		return utils.MakeEnum(id, "PLMNSupportList", false)
	case ngapType.ProtocolIEIDPWSFailedCellIDList:
		return utils.MakeEnum(id, "PWSFailedCellIDList", false)
	case ngapType.ProtocolIEIDRANNodeName:
		return utils.MakeEnum(id, "RANNodeName", false)
	case ngapType.ProtocolIEIDRANPagingPriority:
		return utils.MakeEnum(id, "RANPagingPriority", false)
	case ngapType.ProtocolIEIDRANStatusTransferTransparentContainer:
		return utils.MakeEnum(id, "RANStatusTransferTransparentContainer", false)
	case ngapType.ProtocolIEIDRANUENGAPID:
		return utils.MakeEnum(id, "RANUENGAPID", false)
	case ngapType.ProtocolIEIDRelativeAMFCapacity:
		return utils.MakeEnum(id, "RelativeAMFCapacity", false)
	case ngapType.ProtocolIEIDRepetitionPeriod:
		return utils.MakeEnum(id, "RepetitionPeriod", false)
	case ngapType.ProtocolIEIDResetType:
		return utils.MakeEnum(id, "ResetType", false)
	case ngapType.ProtocolIEIDRoutingID:
		return utils.MakeEnum(id, "RoutingID", false)
	case ngapType.ProtocolIEIDRRCEstablishmentCause:
		return utils.MakeEnum(id, "RRCEstablishmentCause", false)
	case ngapType.ProtocolIEIDRRCInactiveTransitionReportRequest:
		return utils.MakeEnum(id, "RRCInactiveTransitionReportRequest", false)
	case ngapType.ProtocolIEIDRRCState:
		return utils.MakeEnum(id, "RRCState", false)
	case ngapType.ProtocolIEIDSecurityContext:
		return utils.MakeEnum(id, "SecurityContext", false)
	case ngapType.ProtocolIEIDSecurityKey:
		return utils.MakeEnum(id, "SecurityKey", false)
	case ngapType.ProtocolIEIDSerialNumber:
		return utils.MakeEnum(id, "SerialNumber", false)
	case ngapType.ProtocolIEIDServedGUAMIList:
		return utils.MakeEnum(id, "ServedGUAMIList", false)
	case ngapType.ProtocolIEIDSliceSupportList:
		return utils.MakeEnum(id, "SliceSupportList", false)
	case ngapType.ProtocolIEIDSONConfigurationTransferDL:
		return utils.MakeEnum(id, "SONConfigurationTransferDL", false)
	case ngapType.ProtocolIEIDSONConfigurationTransferUL:
		return utils.MakeEnum(id, "SONConfigurationTransferUL", false)
	case ngapType.ProtocolIEIDSourceAMFUENGAPID:
		return utils.MakeEnum(id, "SourceAMFUENGAPID", false)
	case ngapType.ProtocolIEIDSourceToTargetTransparentContainer:
		return utils.MakeEnum(id, "SourceToTargetTransparentContainer", false)
	case ngapType.ProtocolIEIDSupportedTAList:
		return utils.MakeEnum(id, "SupportedTAList", false)
	case ngapType.ProtocolIEIDTAIListForPaging:
		return utils.MakeEnum(id, "TAIListForPaging", false)
	case ngapType.ProtocolIEIDTAIListForRestart:
		return utils.MakeEnum(id, "TAIListForRestart", false)
	case ngapType.ProtocolIEIDTargetID:
		return utils.MakeEnum(id, "TargetID", false)
	case ngapType.ProtocolIEIDTargetToSourceTransparentContainer:
		return utils.MakeEnum(id, "TargetToSourceTransparentContainer", false)
	case ngapType.ProtocolIEIDTimeToWait:
		return utils.MakeEnum(id, "TimeToWait", false)
	case ngapType.ProtocolIEIDTraceActivation:
		return utils.MakeEnum(id, "TraceActivation", false)
	case ngapType.ProtocolIEIDTraceCollectionEntityIPAddress:
		return utils.MakeEnum(id, "TraceCollectionEntityIPAddress", false)
	case ngapType.ProtocolIEIDUEAggregateMaximumBitRate:
		return utils.MakeEnum(id, "UEAggregateMaximumBitRate", false)
	case ngapType.ProtocolIEIDUEAssociatedLogicalNGConnectionList:
		return utils.MakeEnum(id, "UEAssociatedLogicalNGConnectionList", false)
	case ngapType.ProtocolIEIDUEContextRequest:
		return utils.MakeEnum(id, "UEContextRequest", false)
	case ngapType.ProtocolIEIDUENGAPIDs:
		return utils.MakeEnum(id, "UENGAPIDs", false)
	case ngapType.ProtocolIEIDUEPagingIdentity:
		return utils.MakeEnum(id, "UEPagingIdentity", false)
	case ngapType.ProtocolIEIDUEPresenceInAreaOfInterestList:
		return utils.MakeEnum(id, "UEPresenceInAreaOfInterestList", false)
	case ngapType.ProtocolIEIDUERadioCapability:
		return utils.MakeEnum(id, "UERadioCapability", false)
	case ngapType.ProtocolIEIDUERadioCapabilityForPaging:
		return utils.MakeEnum(id, "UERadioCapabilityForPaging", false)
	case ngapType.ProtocolIEIDUESecurityCapabilities:
		return utils.MakeEnum(id, "UESecurityCapabilities", false)
	case ngapType.ProtocolIEIDUnavailableGUAMIList:
		return utils.MakeEnum(id, "UnavailableGUAMIList", false)
	case ngapType.ProtocolIEIDUserLocationInformation:
		return utils.MakeEnum(id, "UserLocationInformation", false)
	case ngapType.ProtocolIEIDWarningAreaList:
		return utils.MakeEnum(id, "WarningAreaList", false)
	case ngapType.ProtocolIEIDWarningMessageContents:
		return utils.MakeEnum(id, "WarningMessageContents", false)
	case ngapType.ProtocolIEIDWarningSecurityInfo:
		return utils.MakeEnum(id, "WarningSecurityInfo", false)
	case ngapType.ProtocolIEIDWarningType:
		return utils.MakeEnum(id, "WarningType", false)
	case ngapType.ProtocolIEIDAdditionalULNGUUPTNLInformation:
		return utils.MakeEnum(id, "AdditionalULNGUUPTNLInformation", false)
	case ngapType.ProtocolIEIDDataForwardingNotPossible:
		return utils.MakeEnum(id, "DataForwardingNotPossible", false)
	case ngapType.ProtocolIEIDDLNGUUPTNLInformation:
		return utils.MakeEnum(id, "DLNGUUPTNLInformation", false)
	case ngapType.ProtocolIEIDNetworkInstance:
		return utils.MakeEnum(id, "NetworkInstance", false)
	case ngapType.ProtocolIEIDPDUSessionAggregateMaximumBitRate:
		return utils.MakeEnum(id, "PDUSessionAggregateMaximumBitRate", false)
	case ngapType.ProtocolIEIDPDUSessionResourceFailedToModifyListModCfm:
		return utils.MakeEnum(id, "PDUSessionResourceFailedToModifyListModCfm", false)
	case ngapType.ProtocolIEIDPDUSessionResourceFailedToSetupListCxtFail:
		return utils.MakeEnum(id, "PDUSessionResourceFailedToSetupListCxtFail", false)
	case ngapType.ProtocolIEIDPDUSessionResourceListCxtRelReq:
		return utils.MakeEnum(id, "PDUSessionResourceListCxtRelReq", false)
	case ngapType.ProtocolIEIDPDUSessionType:
		return utils.MakeEnum(id, "PDUSessionType", false)
	case ngapType.ProtocolIEIDQosFlowAddOrModifyRequestList:
		return utils.MakeEnum(id, "QosFlowAddOrModifyRequestList", false)
	case ngapType.ProtocolIEIDQosFlowSetupRequestList:
		return utils.MakeEnum(id, "QosFlowSetupRequestList", false)
	case ngapType.ProtocolIEIDQosFlowToReleaseList:
		return utils.MakeEnum(id, "QosFlowToReleaseList", false)
	case ngapType.ProtocolIEIDSecurityIndication:
		return utils.MakeEnum(id, "SecurityIndication", false)
	case ngapType.ProtocolIEIDULNGUUPTNLInformation:
		return utils.MakeEnum(id, "ULNGUUPTNLInformation", false)
	case ngapType.ProtocolIEIDULNGUUPTNLModifyList:
		return utils.MakeEnum(id, "ULNGUUPTNLModifyList", false)
	case ngapType.ProtocolIEIDWarningAreaCoordinates:
		return utils.MakeEnum(id, "WarningAreaCoordinates", false)
	case ngapType.ProtocolIEIDPDUSessionResourceSecondaryRATUsageList:
		return utils.MakeEnum(id, "PDUSessionResourceSecondaryRATUsageList", false)
	case ngapType.ProtocolIEIDHandoverFlag:
		return utils.MakeEnum(id, "HandoverFlag", false)
	case ngapType.ProtocolIEIDSecondaryRATUsageInformation:
		return utils.MakeEnum(id, "SecondaryRATUsageInformation", false)
	case ngapType.ProtocolIEIDPDUSessionResourceReleaseResponseTransfer:
		return utils.MakeEnum(id, "PDUSessionResourceReleaseResponseTransfer", false)
	case ngapType.ProtocolIEIDRedirectionVoiceFallback:
		return utils.MakeEnum(id, "RedirectionVoiceFallback", false)
	case ngapType.ProtocolIEIDUERetentionInformation:
		return utils.MakeEnum(id, "UERetentionInformation", false)
	case ngapType.ProtocolIEIDSNSSAI:
		return utils.MakeEnum(id, "SNSSAI", false)
	case ngapType.ProtocolIEIDPSCellInformation:
		return utils.MakeEnum(id, "PSCellInformation", false)
	case ngapType.ProtocolIEIDLastEUTRANPLMNIdentity:
		return utils.MakeEnum(id, "LastEUTRANPLMNIdentity", false)
	case ngapType.ProtocolIEIDMaximumIntegrityProtectedDataRateDL:
		return utils.MakeEnum(id, "MaximumIntegrityProtectedDataRateDL", false)
	case ngapType.ProtocolIEIDAdditionalDLForwardingUPTNLInformation:
		return utils.MakeEnum(id, "AdditionalDLForwardingUPTNLInformation", false)
	case ngapType.ProtocolIEIDAdditionalDLUPTNLInformationForHOList:
		return utils.MakeEnum(id, "AdditionalDLUPTNLInformationForHOList", false)
	case ngapType.ProtocolIEIDAdditionalNGUUPTNLInformation:
		return utils.MakeEnum(id, "AdditionalNGUUPTNLInformation", false)
	case ngapType.ProtocolIEIDAdditionalDLQosFlowPerTNLInformation:
		return utils.MakeEnum(id, "AdditionalDLQosFlowPerTNLInformation", false)
	case ngapType.ProtocolIEIDSecurityResult:
		return utils.MakeEnum(id, "SecurityResult", false)
	case ngapType.ProtocolIEIDENDCSONConfigurationTransferDL:
		return utils.MakeEnum(id, "ENDCSONConfigurationTransferDL", false)
	case ngapType.ProtocolIEIDENDCSONConfigurationTransferUL:
		return utils.MakeEnum(id, "ENDCSONConfigurationTransferUL", false)
	default:
		return utils.MakeEnum(id, "", true)
	}
}

func causeToEnum(cause ngapType.Cause) utils.EnumField[uint64] {
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
		return utils.MakeEnum(uint64(cause.Present), "", true)
	}
}

func radioNetworkCauseToEnum(cause ngapType.CauseRadioNetwork) utils.EnumField[uint64] {
	switch cause.Value {
	case ngapType.CauseRadioNetworkPresentUnspecified:
		return utils.MakeEnum(uint64(cause.Value), "Unspecified", false)
	case ngapType.CauseRadioNetworkPresentTxnrelocoverallExpiry:
		return utils.MakeEnum(uint64(cause.Value), "TxNRelocOverallExpiry", false)
	case ngapType.CauseRadioNetworkPresentSuccessfulHandover:
		return utils.MakeEnum(uint64(cause.Value), "SuccessfulHandover", false)
	case ngapType.CauseRadioNetworkPresentReleaseDueToNgranGeneratedReason:
		return utils.MakeEnum(uint64(cause.Value), "ReleaseDueToNgranGeneratedReason", false)
	case ngapType.CauseRadioNetworkPresentReleaseDueTo5gcGeneratedReason:
		return utils.MakeEnum(uint64(cause.Value), "ReleaseDueTo5gcGeneratedReason", false)
	case ngapType.CauseRadioNetworkPresentHandoverCancelled:
		return utils.MakeEnum(uint64(cause.Value), "HandoverCancelled", false)
	case ngapType.CauseRadioNetworkPresentPartialHandover:
		return utils.MakeEnum(uint64(cause.Value), "PartialHandover", false)
	case ngapType.CauseRadioNetworkPresentHoFailureInTarget5GCNgranNodeOrTargetSystem:
		return utils.MakeEnum(uint64(cause.Value), "HoFailureInTarget5GCNgranNodeOrTargetSystem", false)
	case ngapType.CauseRadioNetworkPresentHoTargetNotAllowed:
		return utils.MakeEnum(uint64(cause.Value), "HoTargetNotAllowed", false)
	case ngapType.CauseRadioNetworkPresentTngrelocoverallExpiry:
		return utils.MakeEnum(uint64(cause.Value), "TngRelocOverallExpiry", false)
	case ngapType.CauseRadioNetworkPresentTngrelocprepExpiry:
		return utils.MakeEnum(uint64(cause.Value), "TngRelocPrepExpiry", false)
	case ngapType.CauseRadioNetworkPresentCellNotAvailable:
		return utils.MakeEnum(uint64(ngapType.CauseRadioNetworkPresentCellNotAvailable), "CellNotAvailable", false)
	case ngapType.CauseRadioNetworkPresentUnknownTargetID:
		return utils.MakeEnum(uint64(ngapType.CauseRadioNetworkPresentUnknownTargetID), "UnknownTargetID", false)
	case ngapType.CauseRadioNetworkPresentNoRadioResourcesAvailableInTargetCell:
		return utils.MakeEnum(uint64(ngapType.CauseRadioNetworkPresentNoRadioResourcesAvailableInTargetCell), "NoRadioResourcesAvailableInTargetCell", false)
	case ngapType.CauseRadioNetworkPresentUnknownLocalUENGAPID:
		return utils.MakeEnum(uint64(ngapType.CauseRadioNetworkPresentUnknownLocalUENGAPID), "UnknownLocalUENGAPID", false)
	case ngapType.CauseRadioNetworkPresentInconsistentRemoteUENGAPID:
		return utils.MakeEnum(uint64(ngapType.CauseRadioNetworkPresentInconsistentRemoteUENGAPID), "InconsistentRemoteUENGAPID", false)
	case ngapType.CauseRadioNetworkPresentHandoverDesirableForRadioReason:
		return utils.MakeEnum(uint64(ngapType.CauseRadioNetworkPresentHandoverDesirableForRadioReason), "HandoverDesirableForRadioReason", false)
	case ngapType.CauseRadioNetworkPresentTimeCriticalHandover:
		return utils.MakeEnum(uint64(ngapType.CauseRadioNetworkPresentTimeCriticalHandover), "TimeCriticalHandover", false)
	case ngapType.CauseRadioNetworkPresentResourceOptimisationHandover:
		return utils.MakeEnum(uint64(ngapType.CauseRadioNetworkPresentResourceOptimisationHandover), "ResourceOptimisationHandover", false)
	case ngapType.CauseRadioNetworkPresentReduceLoadInServingCell:
		return utils.MakeEnum(uint64(ngapType.CauseRadioNetworkPresentReduceLoadInServingCell), "ReduceLoadInServingCell", false)
	case ngapType.CauseRadioNetworkPresentUserInactivity:
		return utils.MakeEnum(uint64(ngapType.CauseRadioNetworkPresentUserInactivity), "UserInactivity", false)
	case ngapType.CauseRadioNetworkPresentRadioConnectionWithUeLost:
		return utils.MakeEnum(uint64(ngapType.CauseRadioNetworkPresentRadioConnectionWithUeLost), "RadioConnectionWithUeLost", false)
	case ngapType.CauseRadioNetworkPresentRadioResourcesNotAvailable:
		return utils.MakeEnum(uint64(ngapType.CauseRadioNetworkPresentRadioResourcesNotAvailable), "RadioResourcesNotAvailable", false)
	case ngapType.CauseRadioNetworkPresentInvalidQosCombination:
		return utils.MakeEnum(uint64(ngapType.CauseRadioNetworkPresentInvalidQosCombination), "InvalidQosCombination", false)
	case ngapType.CauseRadioNetworkPresentFailureInRadioInterfaceProcedure:
		return utils.MakeEnum(uint64(ngapType.CauseRadioNetworkPresentFailureInRadioInterfaceProcedure), "FailureInRadioInterfaceProcedure", false)
	case ngapType.CauseRadioNetworkPresentInteractionWithOtherProcedure:
		return utils.MakeEnum(uint64(ngapType.CauseRadioNetworkPresentInteractionWithOtherProcedure), "InteractionWithOtherProcedure", false)
	case ngapType.CauseRadioNetworkPresentUnknownPDUSessionID:
		return utils.MakeEnum(uint64(ngapType.CauseRadioNetworkPresentUnknownPDUSessionID), "UnknownPDUSessionID", false)
	case ngapType.CauseRadioNetworkPresentUnkownQosFlowID:
		return utils.MakeEnum(uint64(ngapType.CauseRadioNetworkPresentUnkownQosFlowID), "UnkownQosFlowID", false)
	case ngapType.CauseRadioNetworkPresentMultiplePDUSessionIDInstances:
		return utils.MakeEnum(uint64(ngapType.CauseRadioNetworkPresentMultiplePDUSessionIDInstances), "MultiplePDUSessionIDInstances", false)
	case ngapType.CauseRadioNetworkPresentMultipleQosFlowIDInstances:
		return utils.MakeEnum(uint64(ngapType.CauseRadioNetworkPresentMultipleQosFlowIDInstances), "MultipleQosFlowIDInstances", false)
	case ngapType.CauseRadioNetworkPresentEncryptionAndOrIntegrityProtectionAlgorithmsNotSupported:
		return utils.MakeEnum(uint64(ngapType.CauseRadioNetworkPresentEncryptionAndOrIntegrityProtectionAlgorithmsNotSupported), "EncryptionAndOrIntegrityProtectionAlgorithmsNotSupported", false)
	case ngapType.CauseRadioNetworkPresentNgIntraSystemHandoverTriggered:
		return utils.MakeEnum(uint64(ngapType.CauseRadioNetworkPresentNgIntraSystemHandoverTriggered), "NgIntraSystemHandoverTriggered", false)
	case ngapType.CauseRadioNetworkPresentNgInterSystemHandoverTriggered:
		return utils.MakeEnum(uint64(ngapType.CauseRadioNetworkPresentNgInterSystemHandoverTriggered), "NgInterSystemHandoverTriggered", false)
	case ngapType.CauseRadioNetworkPresentXnHandoverTriggered:
		return utils.MakeEnum(uint64(ngapType.CauseRadioNetworkPresentXnHandoverTriggered), "XnHandoverTriggered", false)
	case ngapType.CauseRadioNetworkPresentNotSupported5QIValue:
		return utils.MakeEnum(uint64(ngapType.CauseRadioNetworkPresentNotSupported5QIValue), "NotSupported5QIValue", false)
	case ngapType.CauseRadioNetworkPresentUeContextTransfer:
		return utils.MakeEnum(uint64(ngapType.CauseRadioNetworkPresentUeContextTransfer), "UeContextTransfer", false)
	case ngapType.CauseRadioNetworkPresentImsVoiceEpsFallbackOrRatFallbackTriggered:
		return utils.MakeEnum(uint64(ngapType.CauseRadioNetworkPresentImsVoiceEpsFallbackOrRatFallbackTriggered), "ImsVoiceEpsFallbackOrRatFallbackTriggered", false)
	case ngapType.CauseRadioNetworkPresentUpIntegrityProtectionNotPossible:
		return utils.MakeEnum(uint64(ngapType.CauseRadioNetworkPresentUpIntegrityProtectionNotPossible), "UpIntegrityProtectionNotPossible", false)
	case ngapType.CauseRadioNetworkPresentUpConfidentialityProtectionNotPossible:
		return utils.MakeEnum(uint64(ngapType.CauseRadioNetworkPresentUpConfidentialityProtectionNotPossible), "UpConfidentialityProtectionNotPossible", false)
	case ngapType.CauseRadioNetworkPresentSliceNotSupported:
		return utils.MakeEnum(uint64(ngapType.CauseRadioNetworkPresentSliceNotSupported), "SliceNotSupported", false)
	case ngapType.CauseRadioNetworkPresentUeInRrcInactiveStateNotReachable:
		return utils.MakeEnum(uint64(ngapType.CauseRadioNetworkPresentUeInRrcInactiveStateNotReachable), "UeInRrcInactiveStateNotReachable", false)
	case ngapType.CauseRadioNetworkPresentRedirection:
		return utils.MakeEnum(uint64(ngapType.CauseRadioNetworkPresentRedirection), "Redirection", false)
	case ngapType.CauseRadioNetworkPresentResourcesNotAvailableForTheSlice:
		return utils.MakeEnum(uint64(ngapType.CauseRadioNetworkPresentResourcesNotAvailableForTheSlice), "ResourcesNotAvailableForTheSlice", false)
	case ngapType.CauseRadioNetworkPresentUeMaxIntegrityProtectedDataRateReason:
		return utils.MakeEnum(uint64(ngapType.CauseRadioNetworkPresentUeMaxIntegrityProtectedDataRateReason), "UeMaxIntegrityProtectedDataRateReason", false)
	case ngapType.CauseRadioNetworkPresentReleaseDueToCnDetectedMobility:
		return utils.MakeEnum(uint64(ngapType.CauseRadioNetworkPresentReleaseDueToCnDetectedMobility), "ReleaseDueToCnDetectedMobility", false)
	case ngapType.CauseRadioNetworkPresentN26InterfaceNotAvailable:
		return utils.MakeEnum(uint64(ngapType.CauseRadioNetworkPresentN26InterfaceNotAvailable), "N26InterfaceNotAvailable", false)
	case ngapType.CauseRadioNetworkPresentReleaseDueToPreEmption:
		return utils.MakeEnum(uint64(ngapType.CauseRadioNetworkPresentReleaseDueToPreEmption), "ReleaseDueToPreEmption", false)
	default:
		return utils.MakeEnum(uint64(cause.Value), "", true)
	}
}

func transportCauseToEnum(cause ngapType.CauseTransport) utils.EnumField[uint64] {
	switch cause.Value {
	case ngapType.CauseTransportPresentTransportResourceUnavailable:
		return utils.MakeEnum(uint64(cause.Value), "TransportResourceUnavailable", false)
	case ngapType.CauseTransportPresentUnspecified:
		return utils.MakeEnum(uint64(cause.Value), "Unspecified", false)
	default:
		return utils.MakeEnum(uint64(cause.Value), "", true)
	}
}

func nasCauseToEnum(cause ngapType.CauseNas) utils.EnumField[uint64] {
	switch cause.Value {
	case ngapType.CauseNasPresentNormalRelease:
		return utils.MakeEnum(uint64(cause.Value), "NormalRelease", false)
	case ngapType.CauseNasPresentAuthenticationFailure:
		return utils.MakeEnum(uint64(cause.Value), "AuthenticationFailure", false)
	case ngapType.CauseNasPresentDeregister:
		return utils.MakeEnum(uint64(cause.Value), "Deregister", false)
	case ngapType.CauseNasPresentUnspecified:
		return utils.MakeEnum(uint64(cause.Value), "Unspecified", false)
	default:
		return utils.MakeEnum(uint64(cause.Value), "", true)
	}
}

func protocolCauseToEnum(cause ngapType.CauseProtocol) utils.EnumField[uint64] {
	switch cause.Value {
	case ngapType.CauseProtocolPresentTransferSyntaxError:
		return utils.MakeEnum(uint64(cause.Value), "TransferSyntaxError", false)
	case ngapType.CauseProtocolPresentAbstractSyntaxErrorReject:
		return utils.MakeEnum(uint64(cause.Value), "AbstractSyntaxErrorReject", false)
	case ngapType.CauseProtocolPresentAbstractSyntaxErrorIgnoreAndNotify:
		return utils.MakeEnum(uint64(cause.Value), "AbstractSyntaxErrorIgnoreAndNotify", false)
	case ngapType.CauseProtocolPresentMessageNotCompatibleWithReceiverState:
		return utils.MakeEnum(uint64(cause.Value), "MessageNotCompatibleWithReceiverState", false)
	case ngapType.CauseProtocolPresentSemanticError:
		return utils.MakeEnum(uint64(cause.Value), "SemanticError", false)
	case ngapType.CauseProtocolPresentAbstractSyntaxErrorFalselyConstructedMessage:
		return utils.MakeEnum(uint64(cause.Value), "AbstractSyntaxErrorFalselyConstructedMessage", false)
	case ngapType.CauseProtocolPresentUnspecified:
		return utils.MakeEnum(uint64(cause.Value), "Unspecified", false)
	default:
		return utils.MakeEnum(uint64(cause.Value), "", true)
	}
}

func miscCauseToEnum(cause ngapType.CauseMisc) utils.EnumField[uint64] {
	switch cause.Value {
	case ngapType.CauseMiscPresentControlProcessingOverload:
		return utils.MakeEnum(uint64(cause.Value), "ControlProcessingOverload", false)
	case ngapType.CauseMiscPresentNotEnoughUserPlaneProcessingResources:
		return utils.MakeEnum(uint64(cause.Value), "NotEnoughUserPlaneProcessingResources", false)
	case ngapType.CauseMiscPresentHardwareFailure:
		return utils.MakeEnum(uint64(cause.Value), "HardwareFailure", false)
	case ngapType.CauseMiscPresentOmIntervention:
		return utils.MakeEnum(uint64(cause.Value), "OmIntervention", false)
	case ngapType.CauseMiscPresentUnknownPLMN:
		return utils.MakeEnum(uint64(cause.Value), "UnknownPLMN", false)
	case ngapType.CauseMiscPresentUnspecified:
		return utils.MakeEnum(uint64(cause.Value), "Unspecified", false)
	default:
		return utils.MakeEnum(uint64(cause.Value), "", true)
	}
}

func procedureCodeToEnum(code int64) utils.EnumField[int64] {
	switch code {
	case ngapType.ProcedureCodeAMFConfigurationUpdate:
		return utils.MakeEnum(code, "AMFConfigurationUpdate", false)
	case ngapType.ProcedureCodeAMFStatusIndication:
		return utils.MakeEnum(code, "AMFStatusIndication", false)
	case ngapType.ProcedureCodeCellTrafficTrace:
		return utils.MakeEnum(code, "CellTrafficTrace", false)
	case ngapType.ProcedureCodeDeactivateTrace:
		return utils.MakeEnum(code, "DeactivateTrace", false)
	case ngapType.ProcedureCodeDownlinkNASTransport:
		return utils.MakeEnum(code, "DownlinkNASTransport", false)
	case ngapType.ProcedureCodeDownlinkNonUEAssociatedNRPPaTransport:
		return utils.MakeEnum(code, "DownlinkNonUEAssociatedNRPPaTransport", false)
	case ngapType.ProcedureCodeDownlinkRANConfigurationTransfer:
		return utils.MakeEnum(code, "DownlinkRANConfigurationTransfer", false)
	case ngapType.ProcedureCodeDownlinkRANStatusTransfer:
		return utils.MakeEnum(code, "DownlinkRANStatusTransfer", false)
	case ngapType.ProcedureCodeDownlinkUEAssociatedNRPPaTransport:
		return utils.MakeEnum(code, "DownlinkUEAssociatedNRPPaTransport", false)
	case ngapType.ProcedureCodeErrorIndication:
		return utils.MakeEnum(code, "ErrorIndication", false)
	case ngapType.ProcedureCodeHandoverCancel:
		return utils.MakeEnum(code, "HandoverCancel", false)
	case ngapType.ProcedureCodeHandoverNotification:
		return utils.MakeEnum(code, "HandoverNotification", false)
	case ngapType.ProcedureCodeHandoverPreparation:
		return utils.MakeEnum(code, "HandoverPreparation", false)
	case ngapType.ProcedureCodeHandoverResourceAllocation:
		return utils.MakeEnum(code, "HandoverResourceAllocation", false)
	case ngapType.ProcedureCodeInitialContextSetup:
		return utils.MakeEnum(code, "InitialContextSetup", false)
	case ngapType.ProcedureCodeInitialUEMessage:
		return utils.MakeEnum(code, "InitialUEMessage", false)
	case ngapType.ProcedureCodeLocationReportingControl:
		return utils.MakeEnum(code, "LocationReportingControl", false)
	case ngapType.ProcedureCodeLocationReportingFailureIndication:
		return utils.MakeEnum(code, "LocationReportingFailureIndication", false)
	case ngapType.ProcedureCodeLocationReport:
		return utils.MakeEnum(code, "LocationReport", false)
	case ngapType.ProcedureCodeNASNonDeliveryIndication:
		return utils.MakeEnum(code, "NASNonDeliveryIndication", false)
	case ngapType.ProcedureCodeNGReset:
		return utils.MakeEnum(code, "NGReset", false)
	case ngapType.ProcedureCodeNGSetup:
		return utils.MakeEnum(code, "NGSetup", false)
	case ngapType.ProcedureCodeOverloadStart:
		return utils.MakeEnum(code, "OverloadStart", false)
	case ngapType.ProcedureCodeOverloadStop:
		return utils.MakeEnum(code, "OverloadStop", false)
	case ngapType.ProcedureCodePaging:
		return utils.MakeEnum(code, "Paging", false)
	case ngapType.ProcedureCodePathSwitchRequest:
		return utils.MakeEnum(code, "PathSwitchRequest", false)
	case ngapType.ProcedureCodePDUSessionResourceModify:
		return utils.MakeEnum(code, "PDUSessionResourceModify", false)
	case ngapType.ProcedureCodePDUSessionResourceModifyIndication:
		return utils.MakeEnum(code, "PDUSessionResourceModifyIndication", false)
	case ngapType.ProcedureCodePDUSessionResourceRelease:
		return utils.MakeEnum(code, "PDUSessionResourceRelease", false)
	case ngapType.ProcedureCodePDUSessionResourceSetup:
		return utils.MakeEnum(code, "PDUSessionResourceSetup", false)
	case ngapType.ProcedureCodePDUSessionResourceNotify:
		return utils.MakeEnum(code, "PDUSessionResourceNotify", false)
	case ngapType.ProcedureCodePrivateMessage:
		return utils.MakeEnum(code, "PrivateMessage", false)
	case ngapType.ProcedureCodePWSCancel:
		return utils.MakeEnum(code, "PWSCancel", false)
	case ngapType.ProcedureCodePWSFailureIndication:
		return utils.MakeEnum(code, "PWSFailureIndication", false)
	case ngapType.ProcedureCodePWSRestartIndication:
		return utils.MakeEnum(code, "PWSRestartIndication", false)
	case ngapType.ProcedureCodeRANConfigurationUpdate:
		return utils.MakeEnum(code, "RANConfigurationUpdate", false)
	case ngapType.ProcedureCodeRerouteNASRequest:
		return utils.MakeEnum(code, "RerouteNASRequest", false)
	case ngapType.ProcedureCodeRRCInactiveTransitionReport:
		return utils.MakeEnum(code, "RRCInactiveTransitionReport", false)
	case ngapType.ProcedureCodeTraceFailureIndication:
		return utils.MakeEnum(code, "TraceFailureIndication", false)
	case ngapType.ProcedureCodeTraceStart:
		return utils.MakeEnum(code, "TraceStart", false)
	case ngapType.ProcedureCodeUEContextModification:
		return utils.MakeEnum(code, "UEContextModification", false)
	case ngapType.ProcedureCodeUEContextRelease:
		return utils.MakeEnum(code, "UEContextRelease", false)
	case ngapType.ProcedureCodeUEContextReleaseRequest:
		return utils.MakeEnum(code, "UEContextReleaseRequest", false)
	case ngapType.ProcedureCodeUERadioCapabilityCheck:
		return utils.MakeEnum(code, "UERadioCapabilityCheck", false)
	case ngapType.ProcedureCodeUERadioCapabilityInfoIndication:
		return utils.MakeEnum(code, "UERadioCapabilityInfoIndication", false)
	case ngapType.ProcedureCodeUETNLABindingRelease:
		return utils.MakeEnum(code, "UETNLABindingRelease", false)
	case ngapType.ProcedureCodeUplinkNASTransport:
		return utils.MakeEnum(code, "UplinkNASTransport", false)
	case ngapType.ProcedureCodeUplinkNonUEAssociatedNRPPaTransport:
		return utils.MakeEnum(code, "UplinkNonUEAssociatedNRPPaTransport", false)
	case ngapType.ProcedureCodeUplinkRANConfigurationTransfer:
		return utils.MakeEnum(code, "UplinkRANConfigurationTransfer", false)
	case ngapType.ProcedureCodeUplinkRANStatusTransfer:
		return utils.MakeEnum(code, "UplinkRANStatusTransfer", false)
	case ngapType.ProcedureCodeUplinkUEAssociatedNRPPaTransport:
		return utils.MakeEnum(code, "UplinkUEAssociatedNRPPaTransport", false)
	case ngapType.ProcedureCodeWriteReplaceWarning:
		return utils.MakeEnum(code, "WriteReplaceWarning", false)
	case ngapType.ProcedureCodeSecondaryRATDataUsageReport:
		return utils.MakeEnum(code, "SecondaryRATDataUsageReport", false)
	default:
		return utils.MakeEnum(code, "", true)
	}
}

func initiatingMessageTypeToString(initMsg ngapType.InitiatingMessage) string {
	switch initMsg.Value.Present {
	case ngapType.InitiatingMessagePresentNothing:
		return "Nothing"
	case ngapType.InitiatingMessagePresentAMFConfigurationUpdate:
		return "AMFConfigurationUpdate"
	case ngapType.InitiatingMessagePresentHandoverCancel:
		return "HandoverCancel"
	case ngapType.InitiatingMessagePresentHandoverRequired:
		return "HandoverRequired"
	case ngapType.InitiatingMessagePresentHandoverRequest:
		return "HandoverRequest"
	case ngapType.InitiatingMessagePresentInitialContextSetupRequest:
		return "InitialContextSetupRequest"
	case ngapType.InitiatingMessagePresentNGReset:
		return "NGReset"
	case ngapType.InitiatingMessagePresentNGSetupRequest:
		return "NGSetupRequest"
	case ngapType.InitiatingMessagePresentPathSwitchRequest:
		return "PathSwitchRequest"
	case ngapType.InitiatingMessagePresentPDUSessionResourceModifyRequest:
		return "PDUSessionResourceModifyRequest"
	case ngapType.InitiatingMessagePresentPDUSessionResourceModifyIndication:
		return "PDUSessionResourceModifyIndication"
	case ngapType.InitiatingMessagePresentPDUSessionResourceReleaseCommand:
		return "PDUSessionResourceReleaseCommand"
	case ngapType.InitiatingMessagePresentPDUSessionResourceSetupRequest:
		return "PDUSessionResourceSetupRequest"
	case ngapType.InitiatingMessagePresentPWSCancelRequest:
		return "PWSCancelRequest"
	case ngapType.InitiatingMessagePresentRANConfigurationUpdate:
		return "RANConfigurationUpdate"
	case ngapType.InitiatingMessagePresentUEContextModificationRequest:
		return "UEContextModificationRequest"
	case ngapType.InitiatingMessagePresentUEContextReleaseCommand:
		return "UEContextReleaseCommand"
	case ngapType.InitiatingMessagePresentUERadioCapabilityCheckRequest:
		return "UERadioCapabilityCheckRequest"
	case ngapType.InitiatingMessagePresentWriteReplaceWarningRequest:
		return "WriteReplaceWarningRequest"
	case ngapType.InitiatingMessagePresentAMFStatusIndication:
		return "AMFStatusIndication"
	case ngapType.InitiatingMessagePresentCellTrafficTrace:
		return "CellTrafficTrace"
	case ngapType.InitiatingMessagePresentDeactivateTrace:
		return "DeactivateTrace"
	case ngapType.InitiatingMessagePresentDownlinkNASTransport:
		return "DownlinkNASTransport"
	case ngapType.InitiatingMessagePresentDownlinkNonUEAssociatedNRPPaTransport:
		return "DownlinkNonUEAssociatedNRPPaTransport"
	case ngapType.InitiatingMessagePresentDownlinkRANConfigurationTransfer:
		return "DownlinkRANConfigurationTransfer"
	case ngapType.InitiatingMessagePresentDownlinkRANStatusTransfer:
		return "DownlinkRANStatusTransfer"
	case ngapType.InitiatingMessagePresentDownlinkUEAssociatedNRPPaTransport:
		return "DownlinkUEAssociatedNRPPaTransport"
	case ngapType.InitiatingMessagePresentErrorIndication:
		return "ErrorIndication"
	case ngapType.InitiatingMessagePresentHandoverNotify:
		return "HandoverNotify"
	case ngapType.InitiatingMessagePresentInitialUEMessage:
		return "InitialUEMessage"
	case ngapType.InitiatingMessagePresentLocationReport:
		return "LocationReport"
	case ngapType.InitiatingMessagePresentLocationReportingControl:
		return "LocationReportingControl"
	case ngapType.InitiatingMessagePresentLocationReportingFailureIndication:
		return "LocationReportingFailureIndication"
	case ngapType.InitiatingMessagePresentNASNonDeliveryIndication:
		return "NASNonDeliveryIndication"
	case ngapType.InitiatingMessagePresentOverloadStart:
		return "OverloadStart"
	case ngapType.InitiatingMessagePresentOverloadStop:
		return "OverloadStop"
	case ngapType.InitiatingMessagePresentPaging:
		return "Paging"
	case ngapType.InitiatingMessagePresentPDUSessionResourceNotify:
		return "PDUSessionResourceNotify"
	case ngapType.InitiatingMessagePresentPrivateMessage:
		return "PrivateMessage"
	case ngapType.InitiatingMessagePresentPWSFailureIndication:
		return "PWSFailureIndication"
	case ngapType.InitiatingMessagePresentPWSRestartIndication:
		return "PWSRestartIndication"
	case ngapType.InitiatingMessagePresentRerouteNASRequest:
		return "RerouteNASRequest"
	case ngapType.InitiatingMessagePresentRRCInactiveTransitionReport:
		return "RRCInactiveTransitionReport"
	case ngapType.InitiatingMessagePresentSecondaryRATDataUsageReport:
		return "SecondaryRATDataUsageReport"
	case ngapType.InitiatingMessagePresentTraceFailureIndication:
		return "TraceFailureIndication"
	case ngapType.InitiatingMessagePresentTraceStart:
		return "TraceStart"
	case ngapType.InitiatingMessagePresentUEContextReleaseRequest:
		return "UEContextReleaseRequest"
	case ngapType.InitiatingMessagePresentUERadioCapabilityInfoIndication:
		return "UERadioCapabilityInfoIndication"
	case ngapType.InitiatingMessagePresentUETNLABindingReleaseRequest:
		return "UETNLABindingReleaseRequest"
	case ngapType.InitiatingMessagePresentUplinkNASTransport:
		return "UplinkNASTransport"
	case ngapType.InitiatingMessagePresentUplinkNonUEAssociatedNRPPaTransport:
		return "UplinkNonUEAssociatedNRPPaTransport"
	case ngapType.InitiatingMessagePresentUplinkRANConfigurationTransfer:
		return "UplinkRANConfigurationTransfer"
	case ngapType.InitiatingMessagePresentUplinkRANStatusTransfer:
		return "UplinkRANStatusTransfer"
	case ngapType.InitiatingMessagePresentUplinkUEAssociatedNRPPaTransport:
		return "UplinkUEAssociatedNRPPaTransport"
	default:
		return fmt.Sprintf("Unknown(%d)", initMsg.Value.Present)
	}
}

func successfulOutcomeTypeToString(sucMsg ngapType.SuccessfulOutcome) string {
	switch sucMsg.Value.Present {
	case ngapType.SuccessfulOutcomePresentNothing:
		return "Nothing"
	case ngapType.SuccessfulOutcomePresentAMFConfigurationUpdateAcknowledge:
		return "AMFConfigurationUpdateAcknowledge"
	case ngapType.SuccessfulOutcomePresentHandoverCancelAcknowledge:
		return "HandoverCancelAcknowledge"
	case ngapType.SuccessfulOutcomePresentHandoverCommand:
		return "HandoverCommand"
	case ngapType.SuccessfulOutcomePresentHandoverRequestAcknowledge:
		return "HandoverRequestAcknowledge"
	case ngapType.SuccessfulOutcomePresentInitialContextSetupResponse:
		return "InitialContextSetupResponse"
	case ngapType.SuccessfulOutcomePresentNGResetAcknowledge:
		return "NGResetAcknowledge"
	case ngapType.SuccessfulOutcomePresentNGSetupResponse:
		return "NGSetupResponse"
	case ngapType.SuccessfulOutcomePresentPathSwitchRequestAcknowledge:
		return "PathSwitchRequestAcknowledge"
	case ngapType.SuccessfulOutcomePresentPDUSessionResourceModifyResponse:
		return "PDUSessionResourceModifyResponse"
	case ngapType.SuccessfulOutcomePresentPDUSessionResourceModifyConfirm:
		return "PDUSessionResourceModifyConfirm"
	case ngapType.SuccessfulOutcomePresentPDUSessionResourceReleaseResponse:
		return "PDUSessionResourceReleaseResponse"
	case ngapType.SuccessfulOutcomePresentPDUSessionResourceSetupResponse:
		return "PDUSessionResourceSetupResponse"
	case ngapType.SuccessfulOutcomePresentPWSCancelResponse:
		return "PWSCancelResponse"
	case ngapType.SuccessfulOutcomePresentRANConfigurationUpdateAcknowledge:
		return "RANConfigurationUpdateAcknowledge"
	case ngapType.SuccessfulOutcomePresentUEContextModificationResponse:
		return "UEContextModificationResponse"
	case ngapType.SuccessfulOutcomePresentUEContextReleaseComplete:
		return "UEContextReleaseComplete"
	case ngapType.SuccessfulOutcomePresentUERadioCapabilityCheckResponse:
		return "UERadioCapabilityCheckResponse"
	case ngapType.SuccessfulOutcomePresentWriteReplaceWarningResponse:
		return "WriteReplaceWarningResponse"
	default:
		return fmt.Sprintf("Unknown(%d)", sucMsg.Value.Present)
	}
}

func unsuccessfulOutcomeTypeToString(unsucMsg ngapType.UnsuccessfulOutcome) string {
	switch unsucMsg.Value.Present {
	case ngapType.UnsuccessfulOutcomePresentNothing:
		return "Nothing"
	case ngapType.UnsuccessfulOutcomePresentAMFConfigurationUpdateFailure:
		return "AMFConfigurationUpdateFailure"
	case ngapType.UnsuccessfulOutcomePresentHandoverPreparationFailure:
		return "HandoverPreparationFailure"
	case ngapType.UnsuccessfulOutcomePresentHandoverFailure:
		return "HandoverFailure"
	case ngapType.UnsuccessfulOutcomePresentInitialContextSetupFailure:
		return "InitialContextSetupFailure"
	case ngapType.UnsuccessfulOutcomePresentNGSetupFailure:
		return "NGSetupFailure"
	case ngapType.UnsuccessfulOutcomePresentPathSwitchRequestFailure:
		return "PathSwitchRequestFailure"
	case ngapType.UnsuccessfulOutcomePresentRANConfigurationUpdateFailure:
		return "RANConfigurationUpdateFailure"
	case ngapType.UnsuccessfulOutcomePresentUEContextModificationFailure:
		return "UEContextModificationFailure"
	default:
		return fmt.Sprintf("Unknown(%d)", unsucMsg.Value.Present)
	}
}
