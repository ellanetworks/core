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

func timeStampToRFC3339(timeStampNgap aper.OctetString) (string, error) {
	if len(timeStampNgap) != 4 {
		return "", fmt.Errorf("invalid NGAP timestamp length: got %d, want 4", len(timeStampNgap))
	}

	ntpSeconds := binary.BigEndian.Uint32(timeStampNgap)
	unixSeconds := int64(ntpSeconds) - ntpToUnixOffset
	t := time.Unix(unixSeconds, 0).UTC()
	return t.Format(time.RFC3339), nil
}

func strPtr(s string) *string {
	return &s
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

func protocolIEIDToString(id int64) string {
	switch id {
	case ngapType.ProtocolIEIDAllowedNSSAI:
		return "AllowedNSSAI (0)"
	case ngapType.ProtocolIEIDAMFName:
		return "AMFName (1)"
	case ngapType.ProtocolIEIDAMFOverloadResponse:
		return "AMFOverloadResponse (2)"
	case ngapType.ProtocolIEIDAMFSetID:
		return "AMFSetID (3)"
	case ngapType.ProtocolIEIDAMFTNLAssociationFailedToSetupList:
		return "AMFTNLAssociationFailedToSetupList (4)"
	case ngapType.ProtocolIEIDAMFTNLAssociationSetupList:
		return "AMFTNLAssociationSetupList (5)"
	case ngapType.ProtocolIEIDAMFTNLAssociationToAddList:
		return "AMFTNLAssociationToAddList (6)"
	case ngapType.ProtocolIEIDAMFTNLAssociationToRemoveList:
		return "AMFTNLAssociationToRemoveList (7)"
	case ngapType.ProtocolIEIDAMFTNLAssociationToUpdateList:
		return "AMFTNLAssociationToUpdateList (8)"
	case ngapType.ProtocolIEIDAMFTrafficLoadReductionIndication:
		return "AMFTrafficLoadReductionIndication (9)"
	case ngapType.ProtocolIEIDAMFUENGAPID:
		return "AMFUENGAPID (10)"
	case ngapType.ProtocolIEIDAssistanceDataForPaging:
		return "AssistanceDataForPaging (11)"
	case ngapType.ProtocolIEIDBroadcastCancelledAreaList:
		return "BroadcastCancelledAreaList (12)"
	case ngapType.ProtocolIEIDBroadcastCompletedAreaList:
		return "BroadcastCompletedAreaList (13)"
	case ngapType.ProtocolIEIDCancelAllWarningMessages:
		return "CancelAllWarningMessages (14)"
	case ngapType.ProtocolIEIDCause:
		return "Cause (15)"
	case ngapType.ProtocolIEIDCellIDListForRestart:
		return "CellIDListForRestart (16)"
	case ngapType.ProtocolIEIDConcurrentWarningMessageInd:
		return "ConcurrentWarningMessageInd (17)"
	case ngapType.ProtocolIEIDCoreNetworkAssistanceInformation:
		return "CoreNetworkAssistanceInformation (18)"
	case ngapType.ProtocolIEIDCriticalityDiagnostics:
		return "CriticalityDiagnostics (19)"
	case ngapType.ProtocolIEIDDataCodingScheme:
		return "DataCodingScheme (20)"
	case ngapType.ProtocolIEIDDefaultPagingDRX:
		return "DefaultPagingDRX (21)"
	case ngapType.ProtocolIEIDDirectForwardingPathAvailability:
		return "DirectForwardingPathAvailability (22)"
	case ngapType.ProtocolIEIDEmergencyAreaIDListForRestart:
		return "EmergencyAreaIDListForRestart (23)"
	case ngapType.ProtocolIEIDEmergencyFallbackIndicator:
		return "EmergencyFallbackIndicator (24)"
	case ngapType.ProtocolIEIDEUTRACGI:
		return "EUTRACGI (25)"
	case ngapType.ProtocolIEIDFiveGSTMSI:
		return "FiveGSTMSI (26)"
	case ngapType.ProtocolIEIDGlobalRANNodeID:
		return "GlobalRANNodeID (27)"
	case ngapType.ProtocolIEIDGUAMI:
		return "GUAMI (28)"
	case ngapType.ProtocolIEIDHandoverType:
		return "HandoverType (29)"
	case ngapType.ProtocolIEIDIMSVoiceSupportIndicator:
		return "IMSVoiceSupportIndicator (30)"
	case ngapType.ProtocolIEIDIndexToRFSP:
		return "IndexToRFSP (31)"
	case ngapType.ProtocolIEIDInfoOnRecommendedCellsAndRANNodesForPaging:
		return "InfoOnRecommendedCellsAndRANNodesForPaging (32)"
	case ngapType.ProtocolIEIDLocationReportingRequestType:
		return "LocationReportingRequestType (33)"
	case ngapType.ProtocolIEIDMaskedIMEISV:
		return "MaskedIMEISV (34)"
	case ngapType.ProtocolIEIDMessageIdentifier:
		return "MessageIdentifier (35)"
	case ngapType.ProtocolIEIDMobilityRestrictionList:
		return "MobilityRestrictionList (36)"
	case ngapType.ProtocolIEIDNASC:
		return "NASC (37)"
	case ngapType.ProtocolIEIDNASPDU:
		return "NASPDU (38)"
	case ngapType.ProtocolIEIDNASSecurityParametersFromNGRAN:
		return "NASSecurityParametersFromNGRAN (39)"
	case ngapType.ProtocolIEIDNewAMFUENGAPID:
		return "NewAMFUENGAPID (40)"
	case ngapType.ProtocolIEIDNewSecurityContextInd:
		return "NewSecurityContextInd (41)"
	case ngapType.ProtocolIEIDNGAPMessage:
		return "NGAPMessage (42)"
	case ngapType.ProtocolIEIDNGRANCGI:
		return "NGRANCGI (43)"
	case ngapType.ProtocolIEIDNGRANTraceID:
		return "NGRANTraceID (44)"
	case ngapType.ProtocolIEIDNRCGI:
		return "NRCGI (45)"
	case ngapType.ProtocolIEIDNRPPaPDU:
		return "NRPPaPDU (46)"
	case ngapType.ProtocolIEIDNumberOfBroadcastsRequested:
		return "NumberOfBroadcastsRequested (47)"
	case ngapType.ProtocolIEIDOldAMF:
		return "OldAMF (48)"
	case ngapType.ProtocolIEIDOverloadStartNSSAIList:
		return "OverloadStartNSSAIList (49)"
	case ngapType.ProtocolIEIDPagingDRX:
		return "PagingDRX (50)"
	case ngapType.ProtocolIEIDPagingOrigin:
		return "PagingOrigin (51)"
	case ngapType.ProtocolIEIDPagingPriority:
		return "PagingPriority (52)"
	case ngapType.ProtocolIEIDPDUSessionResourceAdmittedList:
		return "PDUSessionResourceAdmittedList (53)"
	case ngapType.ProtocolIEIDPDUSessionResourceFailedToModifyListModRes:
		return "PDUSessionResourceFailedToModifyListModRes (54)"
	case ngapType.ProtocolIEIDPDUSessionResourceFailedToSetupListCxtRes:
		return "PDUSessionResourceFailedToSetupListCxtRes (55)"
	case ngapType.ProtocolIEIDPDUSessionResourceFailedToSetupListHOAck:
		return "PDUSessionResourceFailedToSetupListHOAck (56)"
	case ngapType.ProtocolIEIDPDUSessionResourceFailedToSetupListPSReq:
		return "PDUSessionResourceFailedToSetupListPSReq (57)"
	case ngapType.ProtocolIEIDPDUSessionResourceFailedToSetupListSURes:
		return "PDUSessionResourceFailedToSetupListSURes (58)"
	case ngapType.ProtocolIEIDPDUSessionResourceHandoverList:
		return "PDUSessionResourceHandoverList (59)"
	case ngapType.ProtocolIEIDPDUSessionResourceListCxtRelCpl:
		return "PDUSessionResourceListCxtRelCpl (60)"
	case ngapType.ProtocolIEIDPDUSessionResourceListHORqd:
		return "PDUSessionResourceListHORqd (61)"
	case ngapType.ProtocolIEIDPDUSessionResourceModifyListModCfm:
		return "PDUSessionResourceModifyListModCfm (62)"
	case ngapType.ProtocolIEIDPDUSessionResourceModifyListModInd:
		return "PDUSessionResourceModifyListModInd (63)"
	case ngapType.ProtocolIEIDPDUSessionResourceModifyListModReq:
		return "PDUSessionResourceModifyListModReq (64)"
	case ngapType.ProtocolIEIDPDUSessionResourceModifyListModRes:
		return "PDUSessionResourceModifyListModRes (65)"
	case ngapType.ProtocolIEIDPDUSessionResourceNotifyList:
		return "PDUSessionResourceNotifyList (66)"
	case ngapType.ProtocolIEIDPDUSessionResourceReleasedListNot:
		return "PDUSessionResourceReleasedListNot (67)"
	case ngapType.ProtocolIEIDPDUSessionResourceReleasedListPSAck:
		return "PDUSessionResourceReleasedListPSAck (68)"
	case ngapType.ProtocolIEIDPDUSessionResourceReleasedListPSFail:
		return "PDUSessionResourceReleasedListPSFail (69)"
	case ngapType.ProtocolIEIDPDUSessionResourceReleasedListRelRes:
		return "PDUSessionResourceReleasedListRelRes (70)"
	case ngapType.ProtocolIEIDPDUSessionResourceSetupListCxtReq:
		return "PDUSessionResourceSetupListCxtReq (71)"
	case ngapType.ProtocolIEIDPDUSessionResourceSetupListCxtRes:
		return "PDUSessionResourceSetupListCxtRes (72)"
	case ngapType.ProtocolIEIDPDUSessionResourceSetupListHOReq:
		return "PDUSessionResourceSetupListHOReq (73)"
	case ngapType.ProtocolIEIDPDUSessionResourceSetupListSUReq:
		return "PDUSessionResourceSetupListSUReq (74)"
	case ngapType.ProtocolIEIDPDUSessionResourceSetupListSURes:
		return "PDUSessionResourceSetupListSURes (75)"
	case ngapType.ProtocolIEIDPDUSessionResourceToBeSwitchedDLList:
		return "PDUSessionResourceToBeSwitchedDLList (76)"
	case ngapType.ProtocolIEIDPDUSessionResourceSwitchedList:
		return "PDUSessionResourceSwitchedList (77)"
	case ngapType.ProtocolIEIDPDUSessionResourceToReleaseListHOCmd:
		return "PDUSessionResourceToReleaseListHOCmd (78)"
	case ngapType.ProtocolIEIDPDUSessionResourceToReleaseListRelCmd:
		return "PDUSessionResourceToReleaseListRelCmd (79)"
	case ngapType.ProtocolIEIDPLMNSupportList:
		return "PLMNSupportList (80)"
	case ngapType.ProtocolIEIDPWSFailedCellIDList:
		return "PWSFailedCellIDList (81)"
	case ngapType.ProtocolIEIDRANNodeName:
		return "RANNodeName (82)"
	case ngapType.ProtocolIEIDRANPagingPriority:
		return "RANPagingPriority (83)"
	case ngapType.ProtocolIEIDRANStatusTransferTransparentContainer:
		return "RANStatusTransferTransparentContainer (84)"
	case ngapType.ProtocolIEIDRANUENGAPID:
		return "RANUENGAPID (85)"
	case ngapType.ProtocolIEIDRelativeAMFCapacity:
		return "RelativeAMFCapacity (86)"
	case ngapType.ProtocolIEIDRepetitionPeriod:
		return "RepetitionPeriod (87)"
	case ngapType.ProtocolIEIDResetType:
		return "ResetType (88)"
	case ngapType.ProtocolIEIDRoutingID:
		return "RoutingID (89)"
	case ngapType.ProtocolIEIDRRCEstablishmentCause:
		return "RRCEstablishmentCause (90)"
	case ngapType.ProtocolIEIDRRCInactiveTransitionReportRequest:
		return "RRCInactiveTransitionReportRequest (91)"
	case ngapType.ProtocolIEIDRRCState:
		return "RRCState (92)"
	case ngapType.ProtocolIEIDSecurityContext:
		return "SecurityContext (93)"
	case ngapType.ProtocolIEIDSecurityKey:
		return "SecurityKey (94)"
	case ngapType.ProtocolIEIDSerialNumber:
		return "SerialNumber (95)"
	case ngapType.ProtocolIEIDServedGUAMIList:
		return "ServedGUAMIList (96)"
	case ngapType.ProtocolIEIDSliceSupportList:
		return "SliceSupportList (97)"
	case ngapType.ProtocolIEIDSONConfigurationTransferDL:
		return "SONConfigurationTransferDL (98)"
	case ngapType.ProtocolIEIDSONConfigurationTransferUL:
		return "SONConfigurationTransferUL (99)"
	case ngapType.ProtocolIEIDSourceAMFUENGAPID:
		return "SourceAMFUENGAPID (100)"
	case ngapType.ProtocolIEIDSourceToTargetTransparentContainer:
		return "SourceToTargetTransparentContainer (101)"
	case ngapType.ProtocolIEIDSupportedTAList:
		return "SupportedTAList (102)"
	case ngapType.ProtocolIEIDTAIListForPaging:
		return "TAIListForPaging (103)"
	case ngapType.ProtocolIEIDTAIListForRestart:
		return "TAIListForRestart (104)"
	case ngapType.ProtocolIEIDTargetID:
		return "TargetID (105)"
	case ngapType.ProtocolIEIDTargetToSourceTransparentContainer:
		return "TargetToSourceTransparentContainer (106)"
	case ngapType.ProtocolIEIDTimeToWait:
		return "TimeToWait (107)"
	case ngapType.ProtocolIEIDTraceActivation:
		return "TraceActivation (108)"
	case ngapType.ProtocolIEIDTraceCollectionEntityIPAddress:
		return "TraceCollectionEntityIPAddress (109)"
	case ngapType.ProtocolIEIDUEAggregateMaximumBitRate:
		return "UEAggregateMaximumBitRate (110)"
	case ngapType.ProtocolIEIDUEAssociatedLogicalNGConnectionList:
		return "UEAssociatedLogicalNGConnectionList (111)"
	case ngapType.ProtocolIEIDUEContextRequest:
		return "UEContextRequest (112)"
	case ngapType.ProtocolIEIDUENGAPIDs:
		return "UENGAPIDs (114)"
	case ngapType.ProtocolIEIDUEPagingIdentity:
		return "UEPagingIdentity (115)"
	case ngapType.ProtocolIEIDUEPresenceInAreaOfInterestList:
		return "UEPresenceInAreaOfInterestList (116)"
	case ngapType.ProtocolIEIDUERadioCapability:
		return "UERadioCapability (117)"
	case ngapType.ProtocolIEIDUERadioCapabilityForPaging:
		return "UERadioCapabilityForPaging (118)"
	case ngapType.ProtocolIEIDUESecurityCapabilities:
		return "UESecurityCapabilities (119)"
	case ngapType.ProtocolIEIDUnavailableGUAMIList:
		return "UnavailableGUAMIList (120)"
	case ngapType.ProtocolIEIDUserLocationInformation:
		return "UserLocationInformation (121)"
	case ngapType.ProtocolIEIDWarningAreaList:
		return "WarningAreaList (122)"
	case ngapType.ProtocolIEIDWarningMessageContents:
		return "WarningMessageContents (123)"
	case ngapType.ProtocolIEIDWarningSecurityInfo:
		return "WarningSecurityInfo (124)"
	case ngapType.ProtocolIEIDWarningType:
		return "WarningType (125)"
	case ngapType.ProtocolIEIDAdditionalULNGUUPTNLInformation:
		return "AdditionalULNGUUPTNLInformation (126)"
	case ngapType.ProtocolIEIDDataForwardingNotPossible:
		return "DataForwardingNotPossible (127)"
	case ngapType.ProtocolIEIDDLNGUUPTNLInformation:
		return "DLNGUUPTNLInformation (128)"
	case ngapType.ProtocolIEIDNetworkInstance:
		return "NetworkInstance (129)"
	case ngapType.ProtocolIEIDPDUSessionAggregateMaximumBitRate:
		return "PDUSessionAggregateMaximumBitRate (130)"
	case ngapType.ProtocolIEIDPDUSessionResourceFailedToModifyListModCfm:
		return "PDUSessionResourceFailedToModifyListModCfm (131)"
	case ngapType.ProtocolIEIDPDUSessionResourceFailedToSetupListCxtFail:
		return "PDUSessionResourceFailedToSetupListCxtFail (132)"
	case ngapType.ProtocolIEIDPDUSessionResourceListCxtRelReq:
		return "PDUSessionResourceListCxtRelReq (133)"
	case ngapType.ProtocolIEIDPDUSessionType:
		return "PDUSessionType (134)"
	case ngapType.ProtocolIEIDQosFlowAddOrModifyRequestList:
		return "QosFlowAddOrModifyRequestList (135)"
	case ngapType.ProtocolIEIDQosFlowSetupRequestList:
		return "QosFlowSetupRequestList (136)"
	case ngapType.ProtocolIEIDQosFlowToReleaseList:
		return "QosFlowToReleaseList (137)"
	case ngapType.ProtocolIEIDSecurityIndication:
		return "SecurityIndication (138)"
	case ngapType.ProtocolIEIDULNGUUPTNLInformation:
		return "ULNGUUPTNLInformation (139)"
	case ngapType.ProtocolIEIDULNGUUPTNLModifyList:
		return "ULNGUUPTNLModifyList (140)"
	case ngapType.ProtocolIEIDWarningAreaCoordinates:
		return "WarningAreaCoordinates (141)"
	case ngapType.ProtocolIEIDPDUSessionResourceSecondaryRATUsageList:
		return "PDUSessionResourceSecondaryRATUsageList (142)"
	case ngapType.ProtocolIEIDHandoverFlag:
		return "HandoverFlag (143)"
	case ngapType.ProtocolIEIDSecondaryRATUsageInformation:
		return "SecondaryRATUsageInformation (144)"
	case ngapType.ProtocolIEIDPDUSessionResourceReleaseResponseTransfer:
		return "PDUSessionResourceReleaseResponseTransfer (145)"
	case ngapType.ProtocolIEIDRedirectionVoiceFallback:
		return "RedirectionVoiceFallback (146)"
	case ngapType.ProtocolIEIDUERetentionInformation:
		return "UERetentionInformation (147)"
	case ngapType.ProtocolIEIDSNSSAI:
		return "SNSSAI (148)"
	case ngapType.ProtocolIEIDPSCellInformation:
		return "PSCellInformation (149)"
	case ngapType.ProtocolIEIDLastEUTRANPLMNIdentity:
		return "LastEUTRANPLMNIdentity (150)"
	case ngapType.ProtocolIEIDMaximumIntegrityProtectedDataRateDL:
		return "MaximumIntegrityProtectedDataRateDL (151)"
	case ngapType.ProtocolIEIDAdditionalDLForwardingUPTNLInformation:
		return "AdditionalDLForwardingUPTNLInformation (152)"
	case ngapType.ProtocolIEIDAdditionalDLUPTNLInformationForHOList:
		return "AdditionalDLUPTNLInformationForHOList (153)"
	case ngapType.ProtocolIEIDAdditionalNGUUPTNLInformation:
		return "AdditionalNGUUPTNLInformation (154)"
	case ngapType.ProtocolIEIDAdditionalDLQosFlowPerTNLInformation:
		return "AdditionalDLQosFlowPerTNLInformation (155)"
	case ngapType.ProtocolIEIDSecurityResult:
		return "SecurityResult (156)"
	case ngapType.ProtocolIEIDENDCSONConfigurationTransferDL:
		return "ENDCSONConfigurationTransferDL (157)"
	case ngapType.ProtocolIEIDENDCSONConfigurationTransferUL:
		return "ENDCSONConfigurationTransferUL (158)"
	default:
		return fmt.Sprintf("Unknown (%d)", id)
	}
}

func causeToString(cause *ngapType.Cause) string {
	if cause == nil {
		return "nil"
	}

	switch cause.Present {
	case ngapType.CausePresentRadioNetwork:
		return radioNetworkCauseToString(cause.RadioNetwork)
	case ngapType.CausePresentTransport:
		return transportCauseToString(cause.Transport)
	case ngapType.CausePresentNas:
		return nasCauseToString(cause.Nas)
	case ngapType.CausePresentProtocol:
		return protocolCauseToString(cause.Protocol)
	case ngapType.CausePresentMisc:
		return miscCauseToString(cause.Misc)
	default:
		return fmt.Sprintf("Unknown (%d)", cause.Present)
	}
}

func radioNetworkCauseToString(cause *ngapType.CauseRadioNetwork) string {
	if cause == nil {
		return "nil"
	}

	switch cause.Value {
	case ngapType.CauseRadioNetworkPresentUnspecified:
		return "Unspecified (0)"
	case ngapType.CauseRadioNetworkPresentTxnrelocoverallExpiry:
		return "TxNRelocOverallExpiry (1)"
	case ngapType.CauseRadioNetworkPresentSuccessfulHandover:
		return "SuccessfulHandover (2)"
	case ngapType.CauseRadioNetworkPresentReleaseDueToNgranGeneratedReason:
		return "ReleaseDueToNgranGeneratedReason (3)"
	case ngapType.CauseRadioNetworkPresentReleaseDueTo5gcGeneratedReason:
		return "ReleaseDueTo5gcGeneratedReason (4)"
	case ngapType.CauseRadioNetworkPresentHandoverCancelled:
		return "HandoverCancelled (5)"
	case ngapType.CauseRadioNetworkPresentPartialHandover:
		return "PartialHandover (6)"
	case ngapType.CauseRadioNetworkPresentHoFailureInTarget5GCNgranNodeOrTargetSystem:
		return "HoFailureInTarget5GCNgranNodeOrTargetSystem (7)"
	case ngapType.CauseRadioNetworkPresentHoTargetNotAllowed:
		return "HoTargetNotAllowed (8)"
	case ngapType.CauseRadioNetworkPresentTngrelocoverallExpiry:
		return "TngRelocOverallExpiry (9)"
	case ngapType.CauseRadioNetworkPresentTngrelocprepExpiry:
		return "TngRelocPrepExpiry (10)"
	case ngapType.CauseRadioNetworkPresentCellNotAvailable:
		return "CellNotAvailable (11)"
	case ngapType.CauseRadioNetworkPresentUnknownTargetID:
		return "UnknownTargetID (12)"
	case ngapType.CauseRadioNetworkPresentNoRadioResourcesAvailableInTargetCell:
		return "NoRadioResourcesAvailableInTargetCell (13)"
	case ngapType.CauseRadioNetworkPresentUnknownLocalUENGAPID:
		return "UnknownLocalUENGAPID (14)"
	case ngapType.CauseRadioNetworkPresentInconsistentRemoteUENGAPID:
		return "InconsistentRemoteUENGAPID (15)"
	case ngapType.CauseRadioNetworkPresentHandoverDesirableForRadioReason:
		return "HandoverDesirableForRadioReason (16)"
	case ngapType.CauseRadioNetworkPresentTimeCriticalHandover:
		return "TimeCriticalHandover (17)"
	case ngapType.CauseRadioNetworkPresentResourceOptimisationHandover:
		return "ResourceOptimisationHandover (18)"
	case ngapType.CauseRadioNetworkPresentReduceLoadInServingCell:
		return "ReduceLoadInServingCell (19)"
	case ngapType.CauseRadioNetworkPresentUserInactivity:
		return "UserInactivity (20)"
	case ngapType.CauseRadioNetworkPresentRadioConnectionWithUeLost:
		return "RadioConnectionWithUeLost (21)"
	case ngapType.CauseRadioNetworkPresentRadioResourcesNotAvailable:
		return "RadioResourcesNotAvailable (22)"
	case ngapType.CauseRadioNetworkPresentInvalidQosCombination:
		return "InvalidQosCombination (23)"
	case ngapType.CauseRadioNetworkPresentFailureInRadioInterfaceProcedure:
		return "FailureInRadioInterfaceProcedure (24)"
	case ngapType.CauseRadioNetworkPresentInteractionWithOtherProcedure:
		return "InteractionWithOtherProcedure (25)"
	case ngapType.CauseRadioNetworkPresentUnknownPDUSessionID:
		return "UnknownPDUSessionID (26)"
	case ngapType.CauseRadioNetworkPresentUnkownQosFlowID:
		return "UnkownQosFlowID (27)"
	case ngapType.CauseRadioNetworkPresentMultiplePDUSessionIDInstances:
		return "MultiplePDUSessionIDInstances (28)"
	case ngapType.CauseRadioNetworkPresentMultipleQosFlowIDInstances:
		return "MultipleQosFlowIDInstances (29)"
	case ngapType.CauseRadioNetworkPresentEncryptionAndOrIntegrityProtectionAlgorithmsNotSupported:
		return "EncryptionAndOrIntegrityProtectionAlgorithmsNotSupported (30)"
	case ngapType.CauseRadioNetworkPresentNgIntraSystemHandoverTriggered:
		return "NgIntraSystemHandoverTriggered (31)"
	case ngapType.CauseRadioNetworkPresentNgInterSystemHandoverTriggered:
		return "NgInterSystemHandoverTriggered (32)"
	case ngapType.CauseRadioNetworkPresentXnHandoverTriggered:
		return "XnHandoverTriggered (33)"
	case ngapType.CauseRadioNetworkPresentNotSupported5QIValue:
		return "NotSupported5QIValue (34)"
	case ngapType.CauseRadioNetworkPresentUeContextTransfer:
		return "UeContextTransfer (35)"
	case ngapType.CauseRadioNetworkPresentImsVoiceEpsFallbackOrRatFallbackTriggered:
		return "ImsVoiceEpsFallbackOrRatFallbackTriggered (36)"
	case ngapType.CauseRadioNetworkPresentUpIntegrityProtectionNotPossible:
		return "UpIntegrityProtectionNotPossible (37)"
	case ngapType.CauseRadioNetworkPresentUpConfidentialityProtectionNotPossible:
		return "UpConfidentialityProtectionNotPossible (38)"
	case ngapType.CauseRadioNetworkPresentSliceNotSupported:
		return "SliceNotSupported (39)"
	case ngapType.CauseRadioNetworkPresentUeInRrcInactiveStateNotReachable:
		return "UeInRrcInactiveStateNotReachable (40)"
	case ngapType.CauseRadioNetworkPresentRedirection:
		return "Redirection (41)"
	case ngapType.CauseRadioNetworkPresentResourcesNotAvailableForTheSlice:
		return "ResourcesNotAvailableForTheSlice (42)"
	case ngapType.CauseRadioNetworkPresentUeMaxIntegrityProtectedDataRateReason:
		return "UeMaxIntegrityProtectedDataRateReason (43)"
	case ngapType.CauseRadioNetworkPresentReleaseDueToCnDetectedMobility:
		return "ReleaseDueToCnDetectedMobility (44)"
	case ngapType.CauseRadioNetworkPresentN26InterfaceNotAvailable:
		return "N26InterfaceNotAvailable (45)"
	case ngapType.CauseRadioNetworkPresentReleaseDueToPreEmption:
		return "ReleaseDueToPreEmption (46)"
	default:
		return fmt.Sprintf("Unknown (%d)", cause.Value)
	}
}

func transportCauseToString(cause *ngapType.CauseTransport) string {
	if cause == nil {
		return "nil"
	}

	switch cause.Value {
	case ngapType.CauseTransportPresentTransportResourceUnavailable:
		return "TransportResourceUnavailable (0)"
	case ngapType.CauseTransportPresentUnspecified:
		return "Unspecified (1)"
	default:
		return fmt.Sprintf("Unknown (%d)", cause.Value)
	}
}

func nasCauseToString(cause *ngapType.CauseNas) string {
	if cause == nil {
		return "nil"
	}

	switch cause.Value {
	case ngapType.CauseNasPresentNormalRelease:
		return "NormalRelease (0)"
	case ngapType.CauseNasPresentAuthenticationFailure:
		return "AuthenticationFailure (1)"
	case ngapType.CauseNasPresentDeregister:
		return "Deregister (2)"
	case ngapType.CauseNasPresentUnspecified:
		return "Unspecified (3)"
	default:
		return fmt.Sprintf("Unknown (%d)", cause.Value)
	}
}

func protocolCauseToString(cause *ngapType.CauseProtocol) string {
	if cause == nil {
		return "nil"
	}

	switch cause.Value {
	case ngapType.CauseProtocolPresentTransferSyntaxError:
		return "TransferSyntaxError (0)"
	case ngapType.CauseProtocolPresentAbstractSyntaxErrorReject:
		return "AbstractSyntaxErrorReject (1)"
	case ngapType.CauseProtocolPresentAbstractSyntaxErrorIgnoreAndNotify:
		return "AbstractSyntaxErrorIgnoreAndNotify (2)"
	case ngapType.CauseProtocolPresentMessageNotCompatibleWithReceiverState:
		return "MessageNotCompatibleWithReceiverState (3)"
	case ngapType.CauseProtocolPresentSemanticError:
		return "SemanticError (4)"
	case ngapType.CauseProtocolPresentAbstractSyntaxErrorFalselyConstructedMessage:
		return "AbstractSyntaxErrorFalselyConstructedMessage (5)"
	case ngapType.CauseProtocolPresentUnspecified:
		return "Unspecified (6)"
	default:
		return fmt.Sprintf("Unknown (%d)", cause.Value)
	}
}

func miscCauseToString(cause *ngapType.CauseMisc) string {
	if cause == nil {
		return "nil"
	}

	switch cause.Value {
	case ngapType.CauseMiscPresentControlProcessingOverload:
		return "ControlProcessingOverload (0)"
	case ngapType.CauseMiscPresentNotEnoughUserPlaneProcessingResources:
		return "NotEnoughUserPlaneProcessingResources (1)"
	case ngapType.CauseMiscPresentHardwareFailure:
		return "HardwareFailure (2)"
	case ngapType.CauseMiscPresentOmIntervention:
		return "OmIntervention (3)"
	case ngapType.CauseMiscPresentUnknownPLMN:
		return "UnknownPLMN (4)"
	case ngapType.CauseMiscPresentUnspecified:
		return "Unspecified (5)"
	default:
		return fmt.Sprintf("Unknown (%d)", cause.Value)
	}
}
