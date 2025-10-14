package server

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/omec-project/ngap"
	"github.com/omec-project/ngap/aper"
	"github.com/omec-project/ngap/ngapType"
	"go.uber.org/zap"
)

const (
	UpdateNetworkLogRetentionPolicyAction = "update_network_log_retention_policy"
)

type GetNetworkLogsRetentionPolicyResponse struct {
	Days int `json:"days"`
}

type UpdateNetworkLogsRetentionPolicyParams struct {
	Days int `json:"days"`
}

type NetworkLog struct {
	ID            int    `json:"id"`
	Timestamp     string `json:"timestamp"`
	Protocol      string `json:"protocol"`
	MessageType   string `json:"message_type"`
	Direction     string `json:"direction"`
	LocalAddress  string `json:"local_address"`
	RemoteAddress string `json:"remote_address"`
	Raw           []byte `json:"raw"`
	Details       string `json:"details"`
}

type ListNetworkLogsResponse struct {
	Items      []NetworkLog `json:"items"`
	Page       int          `json:"page"`
	PerPage    int          `json:"per_page"`
	TotalCount int          `json:"total_count"`
}

type GetDecodedNetworkLogResponse struct {
	Content any `json:"content"`
}

func isRFC3339(s string) bool {
	if _, err := time.Parse(time.RFC3339, s); err != nil {
		return false
	}

	return true
}

func parseNetworkLogFilters(r *http.Request) (*db.NetworkLogFilters, error) {
	q := r.URL.Query()
	f := &db.NetworkLogFilters{}

	if v := strings.TrimSpace(q.Get("protocol")); v != "" {
		f.Protocol = &v
	}

	if v := strings.TrimSpace(q.Get("direction")); v != "" {
		v = strings.ToLower(v)
		if v != "inbound" && v != "outbound" {
			return f, fmt.Errorf("invalid direction")
		}
		f.Direction = &v
	}

	if v := strings.TrimSpace(q.Get("local_address")); v != "" {
		f.LocalAddress = &v
	}

	if v := strings.TrimSpace(q.Get("remote_address")); v != "" {
		f.RemoteAddress = &v
	}

	if v := strings.TrimSpace(q.Get("message_type")); v != "" {
		f.MessageType = &v
	}

	if v := strings.TrimSpace(q.Get("timestamp_from")); v != "" {
		if !isRFC3339(v) {
			return f, fmt.Errorf("invalid from timestamp")
		}
		f.TimestampFrom = &v
	}

	if v := strings.TrimSpace(q.Get("timestamp_to")); v != "" {
		if !isRFC3339(v) {
			return f, fmt.Errorf("invalid to timestamp")
		}
		f.TimestampTo = &v
	}

	return f, nil
}

func GetNetworkLogRetentionPolicy(dbInstance *db.Database) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		policyDays, err := dbInstance.GetLogRetentionPolicy(ctx, db.CategoryNetworkLogs)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "Failed to retrieve network log retention policy", err, logger.APILog)
			return
		}

		response := GetNetworkLogsRetentionPolicyResponse{Days: policyDays}
		writeResponse(w, response, http.StatusOK, logger.APILog)
	})
}

func UpdateNetworkLogRetentionPolicy(dbInstance *db.Database) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		email, ok := r.Context().Value(contextKeyEmail).(string)
		if !ok {
			writeError(w, http.StatusInternalServerError, "Failed to get email", errors.New("missing email in context"), logger.APILog)
			return
		}

		var params UpdateNetworkLogsRetentionPolicyParams
		if err := json.NewDecoder(r.Body).Decode(&params); err != nil {
			writeError(w, http.StatusBadRequest, "Invalid request body", err, logger.APILog)
			return
		}

		if params.Days < 1 {
			writeError(w, http.StatusBadRequest, "retention days must be greater than 0", nil, logger.APILog)
			return
		}

		updatedPolicy := &db.LogRetentionPolicy{
			Category: db.CategoryNetworkLogs,
			Days:     params.Days,
		}

		if err := dbInstance.SetLogRetentionPolicy(r.Context(), updatedPolicy); err != nil {
			writeError(w, http.StatusInternalServerError, "Failed to update network log retention policy", err, logger.APILog)
			return
		}

		writeResponse(w, SuccessResponse{Message: "Network log retention policy updated successfully"}, http.StatusOK, logger.APILog)
		logger.LogAuditEvent(UpdateNetworkLogRetentionPolicyAction, email, getClientIP(r), fmt.Sprintf("User updated network log retention policy to %d days", params.Days))
	})
}

func ListNetworkLogs(dbInstance *db.Database) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		page := atoiDefault(q.Get("page"), 1)
		perPage := atoiDefault(q.Get("per_page"), 25)

		if page < 1 {
			writeError(w, http.StatusBadRequest, "page must be >= 1", nil, logger.APILog)
			return
		}

		if perPage < 1 || perPage > 100 {
			writeError(w, http.StatusBadRequest, "per_page must be between 1 and 100", nil, logger.APILog)
			return
		}

		ctx := r.Context()

		filters, err := parseNetworkLogFilters(r)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error(), nil, logger.APILog)
			return
		}

		logs, total, err := dbInstance.ListNetworkLogs(ctx, page, perPage, filters)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "Failed to retrieve network logs", err, logger.APILog)
			return
		}

		items := make([]NetworkLog, len(logs))
		for i, log := range logs {
			items[i] = NetworkLog{
				ID:            log.ID,
				Timestamp:     log.Timestamp,
				Protocol:      log.Protocol,
				MessageType:   log.MessageType,
				Direction:     log.Direction,
				LocalAddress:  log.LocalAddress,
				RemoteAddress: log.RemoteAddress,
				Raw:           log.Raw,
				Details:       log.Details,
			}
		}

		response := ListNetworkLogsResponse{
			Items:      items,
			Page:       page,
			PerPage:    perPage,
			TotalCount: total,
		}

		writeResponse(w, response, http.StatusOK, logger.APILog)
	})
}

func DecodeNetworkLog(dbInstance *db.Database) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		networkIDStr := strings.TrimPrefix(r.URL.Path, "/api/v1/logs/network/")
		networkID, err := strconv.Atoi(networkIDStr)
		if err != nil || networkID < 1 {
			writeError(w, http.StatusBadRequest, "Invalid network log ID", err, logger.APILog)
			return
		}

		networkLog, err := dbInstance.GetNetworkLogByID(ctx, networkID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "Failed to retrieve network logs", err, logger.APILog)
			return
		}

		decodedContent, err := decodeNetworkLog(networkLog.Raw)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "Failed to decode network log", err, logger.APILog)
			return
		}

		response := GetDecodedNetworkLogResponse{
			Content: decodedContent,
		}

		writeResponse(w, response, http.StatusOK, logger.APILog)
	})
}

func ClearNetworkLogs(dbInstance *db.Database) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		email, ok := r.Context().Value(contextKeyEmail).(string)
		if !ok {
			writeError(w, http.StatusInternalServerError, "Failed to get email", errors.New("missing email in context"), logger.APILog)
			return
		}

		if err := dbInstance.ClearNetworkLogs(r.Context()); err != nil {
			writeError(w, http.StatusInternalServerError, "Failed to clear network logs", err, logger.APILog)
			return
		}

		writeResponse(w, SuccessResponse{Message: "All network logs cleared successfully"}, http.StatusOK, logger.APILog)
		logger.LogAuditEvent("clear_network_logs", email, getClientIP(r), "User cleared all network logs")
	})
}

type GlobalRANNodeIDIE struct {
	GlobalGNBID   string `json:"global_gnb_id,omitempty"`
	GlobalNgENBID string `json:"global_ng_enb_id,omitempty"`
	GlobalN3IWFID string `json:"global_n3iwf_id,omitempty"`
}

type SupportedTA struct {
	TAC string `json:"tac"`
}

type IE struct {
	Id                     string             `json:"id"`
	Criticality            string             `json:"criticality"`
	GlobalRANNodeID        *GlobalRANNodeIDIE `json:"global_ran_node_id,omitempty"`
	RANNodeName            *string            `json:"ran_node_name,omitempty"`
	SupportedTAList        []SupportedTA      `json:"supported_ta_list,omitempty"`
	DefaultPagingDRX       *int               `json:"default_paging_drx,omitempty"`
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

func decodeNetworkLog(raw []byte) (*NGAPMessage, error) {
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
				Id:              protocolIEIDToString(ie.Id.Value),
				Criticality:     criticalityToString(ie.Criticality.Value),
				GlobalRANNodeID: buildGlobalRANNodeIDIE(ie.Value.GlobalRANNodeID),
			})
		case ngapType.ProtocolIEIDSupportedTAList:
			ngSetup.IEs = append(ngSetup.IEs, IE{
				Id:              protocolIEIDToString(ie.Id.Value),
				Criticality:     criticalityToString(ie.Criticality.Value),
				SupportedTAList: buildSupportedTAListIE(ie.Value.SupportedTAList),
			})

		case ngapType.ProtocolIEIDRANNodeName:
			ngSetup.IEs = append(ngSetup.IEs, IE{
				Id:          protocolIEIDToString(ie.Id.Value),
				Criticality: criticalityToString(ie.Criticality.Value),
				RANNodeName: buildRanNodeNameIE(ie.Value.RANNodeName),
			})

		// case ngapType.ProtocolIEIDDefaultPagingDRX:
		// 	pagingDRX = ie.Value.DefaultPagingDRX
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
			TAC: hex.EncodeToString(stal.List[i].TAC.Value),
		}
	}

	return supportedTAs
}

func buildRanNodeNameIE(rnn *ngapType.RANNodeName) *string {
	if rnn == nil || rnn.Value == "" {
		return nil
	}

	s := rnn.Value

	return &s
}

func criticalityToString(c aper.Enumerated) string {
	switch c {
	case ngapType.CriticalityPresentReject:
		return "Reject"
	case ngapType.CriticalityPresentIgnore:
		return "Ignore"
	case ngapType.CriticalityPresentNotify:
		return "Notify"
	default:
		return "Unknown"
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
		return "AllowedNSSAI"
	case ngapType.ProtocolIEIDAMFName:
		return "AMFName"
	case ngapType.ProtocolIEIDAMFOverloadResponse:
		return "AMFOverloadResponse"
	case ngapType.ProtocolIEIDAMFSetID:
		return "AMFSetID"
	case ngapType.ProtocolIEIDAMFTNLAssociationFailedToSetupList:
		return "AMFTNLAssociationFailedToSetupList"
	case ngapType.ProtocolIEIDAMFTNLAssociationSetupList:
		return "AMFTNLAssociationSetupList"
	case ngapType.ProtocolIEIDAMFTNLAssociationToAddList:
		return "AMFTNLAssociationToAddList"
	case ngapType.ProtocolIEIDAMFTNLAssociationToRemoveList:
		return "AMFTNLAssociationToRemoveList"
	case ngapType.ProtocolIEIDAMFTNLAssociationToUpdateList:
		return "AMFTNLAssociationToUpdateList"
	case ngapType.ProtocolIEIDAMFTrafficLoadReductionIndication:
		return "AMFTrafficLoadReductionIndication"
	case ngapType.ProtocolIEIDAMFUENGAPID:
		return "AMFUENGAPID"
	case ngapType.ProtocolIEIDAssistanceDataForPaging:
		return "AssistanceDataForPaging"
	case ngapType.ProtocolIEIDBroadcastCancelledAreaList:
		return "BroadcastCancelledAreaList"
	case ngapType.ProtocolIEIDBroadcastCompletedAreaList:
		return "BroadcastCompletedAreaList"
	case ngapType.ProtocolIEIDCancelAllWarningMessages:
		return "CancelAllWarningMessages"
	case ngapType.ProtocolIEIDCause:
		return "Cause"
	case ngapType.ProtocolIEIDCellIDListForRestart:
		return "CellIDListForRestart"
	case ngapType.ProtocolIEIDConcurrentWarningMessageInd:
		return "ConcurrentWarningMessageInd"
	case ngapType.ProtocolIEIDCoreNetworkAssistanceInformation:
		return "CoreNetworkAssistanceInformation"
	case ngapType.ProtocolIEIDCriticalityDiagnostics:
		return "CriticalityDiagnostics"
	case ngapType.ProtocolIEIDDataCodingScheme:
		return "DataCodingScheme"
	case ngapType.ProtocolIEIDDefaultPagingDRX:
		return "DefaultPagingDRX"
	case ngapType.ProtocolIEIDDirectForwardingPathAvailability:
		return "DirectForwardingPathAvailability"
	case ngapType.ProtocolIEIDEmergencyAreaIDListForRestart:
		return "EmergencyAreaIDListForRestart"
	case ngapType.ProtocolIEIDEmergencyFallbackIndicator:
		return "EmergencyFallbackIndicator"
	case ngapType.ProtocolIEIDEUTRACGI:
		return "EUTRACGI"
	case ngapType.ProtocolIEIDFiveGSTMSI:
		return "FiveGSTMSI"
	case ngapType.ProtocolIEIDGlobalRANNodeID:
		return "GlobalRANNodeID"
	case ngapType.ProtocolIEIDGUAMI:
		return "GUAMI"
	case ngapType.ProtocolIEIDHandoverType:
		return "HandoverType"
	case ngapType.ProtocolIEIDIMSVoiceSupportIndicator:
		return "IMSVoiceSupportIndicator"
	case ngapType.ProtocolIEIDIndexToRFSP:
		return "IndexToRFSP"
	case ngapType.ProtocolIEIDInfoOnRecommendedCellsAndRANNodesForPaging:
		return "InfoOnRecommendedCellsAndRANNodesForPaging"
	case ngapType.ProtocolIEIDLocationReportingRequestType:
		return "LocationReportingRequestType"
	case ngapType.ProtocolIEIDMaskedIMEISV:
		return "MaskedIMEISV"
	case ngapType.ProtocolIEIDMessageIdentifier:
		return "MessageIdentifier"
	case ngapType.ProtocolIEIDMobilityRestrictionList:
		return "MobilityRestrictionList"
	case ngapType.ProtocolIEIDNASC:
		return "NASC"
	case ngapType.ProtocolIEIDNASPDU:
		return "NASPDU"
	case ngapType.ProtocolIEIDNASSecurityParametersFromNGRAN:
		return "NASSecurityParametersFromNGRAN"
	case ngapType.ProtocolIEIDNewAMFUENGAPID:
		return "NewAMFUENGAPID"
	case ngapType.ProtocolIEIDNewSecurityContextInd:
		return "NewSecurityContextInd"
	case ngapType.ProtocolIEIDNGAPMessage:
		return "NGAPMessage"
	case ngapType.ProtocolIEIDNGRANCGI:
		return "NGRANCGI"
	case ngapType.ProtocolIEIDNGRANTraceID:
		return "NGRANTraceID"
	case ngapType.ProtocolIEIDNRCGI:
		return "NRCGI"
	case ngapType.ProtocolIEIDNRPPaPDU:
		return "NRPPaPDU"
	case ngapType.ProtocolIEIDNumberOfBroadcastsRequested:
		return "NumberOfBroadcastsRequested"
	case ngapType.ProtocolIEIDOldAMF:
		return "OldAMF"
	case ngapType.ProtocolIEIDOverloadStartNSSAIList:
		return "OverloadStartNSSAIList"
	case ngapType.ProtocolIEIDPagingDRX:
		return "PagingDRX"
	case ngapType.ProtocolIEIDPagingOrigin:
		return "PagingOrigin"
	case ngapType.ProtocolIEIDPagingPriority:
		return "PagingPriority"
	case ngapType.ProtocolIEIDPDUSessionResourceAdmittedList:
		return "PDUSessionResourceAdmittedList"
	case ngapType.ProtocolIEIDPDUSessionResourceFailedToModifyListModRes:
		return "PDUSessionResourceFailedToModifyListModRes"
	case ngapType.ProtocolIEIDPDUSessionResourceFailedToSetupListCxtRes:
		return "PDUSessionResourceFailedToSetupListCxtRes"
	case ngapType.ProtocolIEIDPDUSessionResourceFailedToSetupListHOAck:
		return "PDUSessionResourceFailedToSetupListHOAck"
	case ngapType.ProtocolIEIDPDUSessionResourceFailedToSetupListPSReq:
		return "PDUSessionResourceFailedToSetupListPSReq"
	case ngapType.ProtocolIEIDPDUSessionResourceFailedToSetupListSURes:
		return "PDUSessionResourceFailedToSetupListSURes"
	case ngapType.ProtocolIEIDPDUSessionResourceHandoverList:
		return "PDUSessionResourceHandoverList"
	case ngapType.ProtocolIEIDPDUSessionResourceListCxtRelCpl:
		return "PDUSessionResourceListCxtRelCpl"
	case ngapType.ProtocolIEIDPDUSessionResourceListHORqd:
		return "PDUSessionResourceListHORqd"
	case ngapType.ProtocolIEIDPDUSessionResourceModifyListModCfm:
		return "PDUSessionResourceModifyListModCfm"
	case ngapType.ProtocolIEIDPDUSessionResourceModifyListModInd:
		return "PDUSessionResourceModifyListModInd"
	case ngapType.ProtocolIEIDPDUSessionResourceModifyListModReq:
		return "PDUSessionResourceModifyListModReq"
	case ngapType.ProtocolIEIDPDUSessionResourceModifyListModRes:
		return "PDUSessionResourceModifyListModRes"
	case ngapType.ProtocolIEIDPDUSessionResourceNotifyList:
		return "PDUSessionResourceNotifyList"
	case ngapType.ProtocolIEIDPDUSessionResourceReleasedListNot:
		return "PDUSessionResourceReleasedListNot"
	case ngapType.ProtocolIEIDPDUSessionResourceReleasedListPSAck:
		return "PDUSessionResourceReleasedListPSAck"
	case ngapType.ProtocolIEIDPDUSessionResourceReleasedListPSFail:
		return "PDUSessionResourceReleasedListPSFail"
	case ngapType.ProtocolIEIDPDUSessionResourceReleasedListRelRes:
		return "PDUSessionResourceReleasedListRelRes"
	case ngapType.ProtocolIEIDPDUSessionResourceSetupListCxtReq:
		return "PDUSessionResourceSetupListCxtReq"
	case ngapType.ProtocolIEIDPDUSessionResourceSetupListCxtRes:
		return "PDUSessionResourceSetupListCxtRes"
	case ngapType.ProtocolIEIDPDUSessionResourceSetupListHOReq:
		return "PDUSessionResourceSetupListHOReq"
	case ngapType.ProtocolIEIDPDUSessionResourceSetupListSUReq:
		return "PDUSessionResourceSetupListSUReq"
	case ngapType.ProtocolIEIDPDUSessionResourceSetupListSURes:
		return "PDUSessionResourceSetupListSURes"
	case ngapType.ProtocolIEIDPDUSessionResourceToBeSwitchedDLList:
		return "PDUSessionResourceToBeSwitchedDLList"
	case ngapType.ProtocolIEIDPDUSessionResourceSwitchedList:
		return "PDUSessionResourceSwitchedList"
	case ngapType.ProtocolIEIDPDUSessionResourceToReleaseListHOCmd:
		return "PDUSessionResourceToReleaseListHOCmd"
	case ngapType.ProtocolIEIDPDUSessionResourceToReleaseListRelCmd:
		return "PDUSessionResourceToReleaseListRelCmd"
	case ngapType.ProtocolIEIDPLMNSupportList:
		return "PLMNSupportList"
	case ngapType.ProtocolIEIDPWSFailedCellIDList:
		return "PWSFailedCellIDList"
	case ngapType.ProtocolIEIDRANNodeName:
		return "RANNodeName"
	case ngapType.ProtocolIEIDRANPagingPriority:
		return "RANPagingPriority"
	case ngapType.ProtocolIEIDRANStatusTransferTransparentContainer:
		return "RANStatusTransferTransparentContainer"
	case ngapType.ProtocolIEIDRANUENGAPID:
		return "RANUENGAPID"
	case ngapType.ProtocolIEIDRelativeAMFCapacity:
		return "RelativeAMFCapacity"
	case ngapType.ProtocolIEIDRepetitionPeriod:
		return "RepetitionPeriod"
	case ngapType.ProtocolIEIDResetType:
		return "ResetType"
	case ngapType.ProtocolIEIDRoutingID:
		return "RoutingID"
	case ngapType.ProtocolIEIDRRCEstablishmentCause:
		return "RRCEstablishmentCause"
	case ngapType.ProtocolIEIDRRCInactiveTransitionReportRequest:
		return "RRCInactiveTransitionReportRequest"
	case ngapType.ProtocolIEIDRRCState:
		return "RRCState"
	case ngapType.ProtocolIEIDSecurityContext:
		return "SecurityContext"
	case ngapType.ProtocolIEIDSecurityKey:
		return "SecurityKey"
	case ngapType.ProtocolIEIDSerialNumber:
		return "SerialNumber"
	case ngapType.ProtocolIEIDServedGUAMIList:
		return "ServedGUAMIList"
	case ngapType.ProtocolIEIDSliceSupportList:
		return "SliceSupportList"
	case ngapType.ProtocolIEIDSONConfigurationTransferDL:
		return "SONConfigurationTransferDL"
	case ngapType.ProtocolIEIDSONConfigurationTransferUL:
		return "SONConfigurationTransferUL"
	case ngapType.ProtocolIEIDSourceAMFUENGAPID:
		return "SourceAMFUENGAPID"
	case ngapType.ProtocolIEIDSourceToTargetTransparentContainer:
		return "SourceToTargetTransparentContainer"
	case ngapType.ProtocolIEIDSupportedTAList:
		return "SupportedTAList"
	case ngapType.ProtocolIEIDTAIListForPaging:
		return "TAIListForPaging"
	case ngapType.ProtocolIEIDTAIListForRestart:
		return "TAIListForRestart"
	case ngapType.ProtocolIEIDTargetID:
		return "TargetID"
	case ngapType.ProtocolIEIDTargetToSourceTransparentContainer:
		return "TargetToSourceTransparentContainer"
	case ngapType.ProtocolIEIDTimeToWait:
		return "TimeToWait"
	case ngapType.ProtocolIEIDTraceActivation:
		return "TraceActivation"
	case ngapType.ProtocolIEIDTraceCollectionEntityIPAddress:
		return "TraceCollectionEntityIPAddress"
	case ngapType.ProtocolIEIDUEAggregateMaximumBitRate:
		return "UEAggregateMaximumBitRate"
	case ngapType.ProtocolIEIDUEAssociatedLogicalNGConnectionList:
		return "UEAssociatedLogicalNGConnectionList"
	case ngapType.ProtocolIEIDUEContextRequest:
		return "UEContextRequest"
	case ngapType.ProtocolIEIDUENGAPIDs:
		return "UENGAPIDs"
	case ngapType.ProtocolIEIDUEPagingIdentity:
		return "UEPagingIdentity"
	case ngapType.ProtocolIEIDUEPresenceInAreaOfInterestList:
		return "UEPresenceInAreaOfInterestList"
	case ngapType.ProtocolIEIDUERadioCapability:
		return "UERadioCapability"
	case ngapType.ProtocolIEIDUERadioCapabilityForPaging:
		return "UERadioCapabilityForPaging"
	case ngapType.ProtocolIEIDUESecurityCapabilities:
		return "UESecurityCapabilities"
	case ngapType.ProtocolIEIDUnavailableGUAMIList:
		return "UnavailableGUAMIList"
	case ngapType.ProtocolIEIDUserLocationInformation:
		return "UserLocationInformation"
	case ngapType.ProtocolIEIDWarningAreaList:
		return "WarningAreaList"
	case ngapType.ProtocolIEIDWarningMessageContents:
		return "WarningMessageContents"
	case ngapType.ProtocolIEIDWarningSecurityInfo:
		return "WarningSecurityInfo"
	case ngapType.ProtocolIEIDWarningType:
		return "WarningType"
	case ngapType.ProtocolIEIDAdditionalULNGUUPTNLInformation:
		return "AdditionalULNGUUPTNLInformation"
	case ngapType.ProtocolIEIDDataForwardingNotPossible:
		return "DataForwardingNotPossible"
	case ngapType.ProtocolIEIDDLNGUUPTNLInformation:
		return "DLNGUUPTNLInformation"
	case ngapType.ProtocolIEIDNetworkInstance:
		return "NetworkInstance"
	case ngapType.ProtocolIEIDPDUSessionAggregateMaximumBitRate:
		return "PDUSessionAggregateMaximumBitRate"
	case ngapType.ProtocolIEIDPDUSessionResourceFailedToModifyListModCfm:
		return "PDUSessionResourceFailedToModifyListModCfm"
	case ngapType.ProtocolIEIDPDUSessionResourceFailedToSetupListCxtFail:
		return "PDUSessionResourceFailedToSetupListCxtFail"
	case ngapType.ProtocolIEIDPDUSessionResourceListCxtRelReq:
		return "PDUSessionResourceListCxtRelReq"
	case ngapType.ProtocolIEIDPDUSessionType:
		return "PDUSessionType"
	case ngapType.ProtocolIEIDQosFlowAddOrModifyRequestList:
		return "QosFlowAddOrModifyRequestList"
	case ngapType.ProtocolIEIDQosFlowSetupRequestList:
		return "QosFlowSetupRequestList"
	case ngapType.ProtocolIEIDQosFlowToReleaseList:
		return "QosFlowToReleaseList"
	case ngapType.ProtocolIEIDSecurityIndication:
		return "SecurityIndication"
	case ngapType.ProtocolIEIDULNGUUPTNLInformation:
		return "ULNGUUPTNLInformation"
	case ngapType.ProtocolIEIDULNGUUPTNLModifyList:
		return "ULNGUUPTNLModifyList"
	case ngapType.ProtocolIEIDWarningAreaCoordinates:
		return "WarningAreaCoordinates"
	case ngapType.ProtocolIEIDPDUSessionResourceSecondaryRATUsageList:
		return "PDUSessionResourceSecondaryRATUsageList"
	case ngapType.ProtocolIEIDHandoverFlag:
		return "HandoverFlag"
	case ngapType.ProtocolIEIDSecondaryRATUsageInformation:
		return "SecondaryRATUsageInformation"
	case ngapType.ProtocolIEIDPDUSessionResourceReleaseResponseTransfer:
		return "PDUSessionResourceReleaseResponseTransfer"
	case ngapType.ProtocolIEIDRedirectionVoiceFallback:
		return "RedirectionVoiceFallback"
	case ngapType.ProtocolIEIDUERetentionInformation:
		return "UERetentionInformation"
	case ngapType.ProtocolIEIDSNSSAI:
		return "SNSSAI"
	case ngapType.ProtocolIEIDPSCellInformation:
		return "PSCellInformation"
	case ngapType.ProtocolIEIDLastEUTRANPLMNIdentity:
		return "LastEUTRANPLMNIdentity"
	case ngapType.ProtocolIEIDMaximumIntegrityProtectedDataRateDL:
		return "MaximumIntegrityProtectedDataRateDL"
	case ngapType.ProtocolIEIDAdditionalDLForwardingUPTNLInformation:
		return "AdditionalDLForwardingUPTNLInformation"
	case ngapType.ProtocolIEIDAdditionalDLUPTNLInformationForHOList:
		return "AdditionalDLUPTNLInformationForHOList"
	case ngapType.ProtocolIEIDAdditionalNGUUPTNLInformation:
		return "AdditionalNGUUPTNLInformation"
	case ngapType.ProtocolIEIDAdditionalDLQosFlowPerTNLInformation:
		return "AdditionalDLQosFlowPerTNLInformation"
	case ngapType.ProtocolIEIDSecurityResult:
		return "SecurityResult"
	case ngapType.ProtocolIEIDENDCSONConfigurationTransferDL:
		return "ENDCSONConfigurationTransferDL"
	case ngapType.ProtocolIEIDENDCSONConfigurationTransferUL:
		return "ENDCSONConfigurationTransferUL"
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
