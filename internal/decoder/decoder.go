package decoder

import (
	"encoding/hex"
	"fmt"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/omec-project/ngap"
	"github.com/omec-project/ngap/aper"
	"github.com/omec-project/ngap/ngapType"
	"go.uber.org/zap"
)

type GlobalRANNodeIDIE struct {
	GlobalGNBID   string `json:"global_gnb_id,omitempty"`
	GlobalNgENBID string `json:"global_ng_enb_id,omitempty"`
	GlobalN3IWFID string `json:"global_n3iwf_id,omitempty"`
}

type SNSSAI struct {
	SST int32   `json:"sst"`
	SD  *string `json:"sd,omitempty"`
}

type PLMN struct {
	PLMNIdentity        string   `json:"plmn_identity"`
	TAISliceSupportList []SNSSAI `json:"slice_support_list,omitempty"`
}

type SupportedTA struct {
	TAC               string `json:"tac"`
	BroadcastPLMNList []PLMN `json:"broadcast_plmn_list,omitempty"`
}

type IE struct {
	ID                     string             `json:"id"`
	Criticality            string             `json:"criticality"`
	GlobalRANNodeID        *GlobalRANNodeIDIE `json:"global_ran_node_id,omitempty"`
	RANNodeName            *string            `json:"ran_node_name,omitempty"`
	SupportedTAList        []SupportedTA      `json:"supported_ta_list,omitempty"`
	DefaultPagingDRX       *string            `json:"default_paging_drx,omitempty"`
	UERetentionInformation *string            `json:"ue_retention_information,omitempty"`
}

type NGSetupRequest struct {
	IEs []IE `json:"ies"`
}

type InitiatingMessage struct {
	NGSetupRequest *NGSetupRequest `json:"ng_setup_request,omitempty"`
}

type NGAPMessage struct {
	ProcedureCode     string             `json:"procedure_code"`
	Criticality       string             `json:"criticality"`
	InitiatingMessage *InitiatingMessage `json:"initiating_message,omitempty"`
}

func DecodeNetworkLog(raw []byte) (*NGAPMessage, error) {
	pdu, err := ngap.Decoder(raw)
	if err != nil {
		return nil, fmt.Errorf("failed to decode NGAP message: %w", err)
	}

	ngapMsg := &NGAPMessage{}

	// Extract message type
	switch pdu.Present {
	case ngapType.NGAPPDUPresentInitiatingMessage:
		im := pdu.InitiatingMessage
		if im == nil {
			return nil, fmt.Errorf("initiating message is nil")
		}
		ngapMsg.ProcedureCode = procedureCodeToString(im.ProcedureCode.Value)
		ngapMsg.Criticality = criticalityToString(im.Criticality.Value)
		ngapMsg.InitiatingMessage = buildInitiatingMessage(im.ProcedureCode.Value, im.Value)
		return ngapMsg, nil

	default:
		return nil, fmt.Errorf("unknown NGAP PDU type")
	}
}

func buildInitiatingMessage(procedureCode int64, initMsg ngapType.InitiatingMessageValue) *InitiatingMessage {
	initiatingMsg := &InitiatingMessage{}

	switch procedureCode {
	case ngapType.ProcedureCodeNGSetup:
		ngSetupRequest := initMsg.NGSetupRequest
		if ngSetupRequest == nil {
			return nil
		}
		initiatingMsg.NGSetupRequest = buildNGSetupRequest(initMsg.NGSetupRequest)
		return initiatingMsg
	default:
		logger.EllaLog.Warn("Unsupported procedure code", zap.Int64("procedure_code", procedureCode))
	}
	return nil
}

func buildNGSetupRequest(ngSetupRequest *ngapType.NGSetupRequest) *NGSetupRequest {
	ngSetup := &NGSetupRequest{}

	for i := 0; i < len(ngSetupRequest.ProtocolIEs.List); i++ {
		ie := ngSetupRequest.ProtocolIEs.List[i]

		switch ie.Id.Value {
		case ngapType.ProtocolIEIDGlobalRANNodeID:
			ngSetup.IEs = append(ngSetup.IEs, IE{
				ID:              protocolIEIDToString(ie.Id.Value),
				Criticality:     criticalityToString(ie.Criticality.Value),
				GlobalRANNodeID: buildGlobalRANNodeIDIE(ie.Value.GlobalRANNodeID),
			})
		case ngapType.ProtocolIEIDSupportedTAList:
			ngSetup.IEs = append(ngSetup.IEs, IE{
				ID:              protocolIEIDToString(ie.Id.Value),
				Criticality:     criticalityToString(ie.Criticality.Value),
				SupportedTAList: buildSupportedTAListIE(ie.Value.SupportedTAList),
			})
		case ngapType.ProtocolIEIDRANNodeName:
			ngSetup.IEs = append(ngSetup.IEs, IE{
				ID:          protocolIEIDToString(ie.Id.Value),
				Criticality: criticalityToString(ie.Criticality.Value),
				RANNodeName: buildRanNodeNameIE(ie.Value.RANNodeName),
			})
		case ngapType.ProtocolIEIDDefaultPagingDRX:
			ngSetup.IEs = append(ngSetup.IEs, IE{
				ID:               protocolIEIDToString(ie.Id.Value),
				Criticality:      criticalityToString(ie.Criticality.Value),
				DefaultPagingDRX: buildDefaultPagingDRXIE(ie.Value.DefaultPagingDRX),
			})
		case ngapType.ProtocolIEIDUERetentionInformation:
			ngSetup.IEs = append(ngSetup.IEs, IE{
				ID:                     protocolIEIDToString(ie.Id.Value),
				Criticality:            criticalityToString(ie.Criticality.Value),
				UERetentionInformation: buildUERetentionInformationIE(ie.Value.UERetentionInformation),
			})
		default:
			logger.EllaLog.Warn("Unsupported ie type", zap.Int64("type", ie.Id.Value))
		}
	}

	return ngSetup
}

func buildGlobalRANNodeIDIE(grn *ngapType.GlobalRANNodeID) *GlobalRANNodeIDIE {
	if grn == nil {
		return nil
	}

	ie := &GlobalRANNodeIDIE{}

	if grn.GlobalGNBID != nil && grn.GlobalGNBID.GNBID.GNBID != nil {
		ie.GlobalGNBID = bitStringToHex(grn.GlobalGNBID.GNBID.GNBID)
	}

	if grn.GlobalNgENBID != nil && grn.GlobalNgENBID.NgENBID.MacroNgENBID != nil {
		ie.GlobalNgENBID = bitStringToHex(grn.GlobalNgENBID.NgENBID.MacroNgENBID)
	}

	if grn.GlobalN3IWFID != nil && grn.GlobalN3IWFID.N3IWFID.N3IWFID != nil {
		ie.GlobalN3IWFID = bitStringToHex(grn.GlobalN3IWFID.N3IWFID.N3IWFID)
	}

	return ie
}

func buildSupportedTAListIE(stal *ngapType.SupportedTAList) []SupportedTA {
	if stal == nil {
		return nil
	}

	supportedTAs := make([]SupportedTA, len(stal.List))
	for i := 0; i < len(stal.List); i++ {
		supportedTAs[i] = SupportedTA{
			TAC:               hex.EncodeToString(stal.List[i].TAC.Value),
			BroadcastPLMNList: buildPLMNList(stal.List[i].BroadcastPLMNList),
		}
	}

	return supportedTAs
}

func buildPLMNList(bpl ngapType.BroadcastPLMNList) []PLMN {
	plmns := make([]PLMN, len(bpl.List))
	for i := 0; i < len(bpl.List); i++ {
		plmns[i] = PLMN{
			PLMNIdentity:        hex.EncodeToString(bpl.List[i].PLMNIdentity.Value),
			TAISliceSupportList: buildSNSSAIList(bpl.List[i].TAISliceSupportList),
		}
	}

	return plmns
}

func buildSNSSAIList(sssl ngapType.SliceSupportList) []SNSSAI {
	snssais := make([]SNSSAI, len(sssl.List))
	for i := 0; i < len(sssl.List); i++ {
		snssai := SNSSAI{
			SST: int32(sssl.List[i].SNSSAI.SST.Value[0]),
		}
		if sssl.List[i].SNSSAI.SD != nil {
			sd := hex.EncodeToString(sssl.List[i].SNSSAI.SD.Value)
			snssai.SD = &sd
		}
		snssais[i] = snssai
	}

	return snssais
}

func buildRanNodeNameIE(rnn *ngapType.RANNodeName) *string {
	if rnn == nil || rnn.Value == "" {
		return nil
	}

	s := rnn.Value

	return &s
}

func buildDefaultPagingDRXIE(dpd *ngapType.PagingDRX) *string {
	if dpd == nil {
		return nil
	}

	switch dpd.Value {
	case ngapType.PagingDRXPresentV32:
		return strPtr("v32")
	case ngapType.PagingDRXPresentV64:
		return strPtr("v64")
	case ngapType.PagingDRXPresentV128:
		return strPtr("v128")
	case ngapType.PagingDRXPresentV256:
		return strPtr("v256")
	default:
		return strPtr(fmt.Sprintf("Unknown (%d)", dpd.Value))
	}
}

func buildUERetentionInformationIE(uri *ngapType.UERetentionInformation) *string {
	if uri == nil {
		return nil
	}

	switch uri.Value {
	case ngapType.UERetentionInformationPresentUesRetained:
		return strPtr("present")
	default:
		return strPtr(fmt.Sprintf("unknown (%d)", uri.Value))
	}
}

func strPtr(s string) *string {
	return &s
}

func criticalityToString(c aper.Enumerated) string {
	switch c {
	case ngapType.CriticalityPresentReject:
		return "Reject (0)"
	case ngapType.CriticalityPresentIgnore:
		return "Ignore (1)"
	case ngapType.CriticalityPresentNotify:
		return "Notify (2)"
	default:
		return fmt.Sprintf("Unknown (%d)", c)
	}
}

func procedureCodeToString(code int64) string {
	name := ngapType.ProcedureName(code)
	if name == "" {
		return fmt.Sprintf("Unknown (%d)", code)
	}
	return name
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

func bitStringToHex(bitString *aper.BitString) string {
	hexString := hex.EncodeToString(bitString.Bytes)
	hexLen := (bitString.BitLength + 3) / 4
	hexString = hexString[:hexLen]
	return hexString
}
