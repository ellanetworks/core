package ngap

import (
	"fmt"

	"github.com/ellanetworks/core/internal/decoder/nas"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/omec-project/ngap"
	"github.com/omec-project/ngap/aper"
	"github.com/omec-project/ngap/ngapType"
	"go.uber.org/zap"
)

const ntpToUnixOffset = 2208988800 // seconds between 1900-01-01 and 1970-01-01

type GlobalRANNodeIDIE struct {
	GlobalGNBID   string `json:"global_gnb_id,omitempty"`
	GlobalNgENBID string `json:"global_ng_enb_id,omitempty"`
	GlobalN3IWFID string `json:"global_n3iwf_id,omitempty"`
}

type SNSSAI struct {
	SST int32   `json:"sst"`
	SD  *string `json:"sd,omitempty"`
}

type PLMNID struct {
	Mcc string `json:"mcc"`
	Mnc string `json:"mnc"`
}

type PLMN struct {
	PLMNID           PLMNID   `json:"plmn_id"`
	SliceSupportList []SNSSAI `json:"slice_support_list,omitempty"`
}

type SupportedTA struct {
	TAC               string `json:"tac"`
	BroadcastPLMNList []PLMN `json:"broadcast_plmn_list,omitempty"`
}

type Guami struct {
	PLMNID PLMNID `json:"plmn_id"`
	AMFID  string `json:"amf_id"`
}

type IEsCriticalityDiagnostics struct {
	IECriticality string `json:"ie_criticality"`
	IEID          string `json:"ie_id"`
	TypeOfError   string `json:"type_of_error"`
}

type CriticalityDiagnostics struct {
	ProcedureCode             *string                     `json:"procedure_code,omitempty"`
	TriggeringMessage         *string                     `json:"triggering_message,omitempty"`
	ProcedureCriticality      *string                     `json:"procedure_criticality,omitempty"`
	IEsCriticalityDiagnostics []IEsCriticalityDiagnostics `json:"ie_criticality_diagnostics,omitempty"`
}

type EUTRACGI struct {
	PLMNID            PLMNID `json:"plmn_id"`
	EUTRACellIdentity string `json:"eutra_cell_identity"`
}

type TAI struct {
	PLMNID PLMNID `json:"plmn_id"`
	TAC    string `json:"tac"`
}

type UserLocationInformationEUTRA struct {
	EUTRACGI  EUTRACGI `json:"eutra_cgi"`
	TAI       TAI      `json:"tai"`
	TimeStamp *string  `json:"timestamp,omitempty"`
}

type NRCGI struct {
	PLMNID         PLMNID `json:"plmn_id"`
	NRCellIdentity string `json:"nr_cell_identity"`
}

type UserLocationInformationNR struct {
	NRCGI     NRCGI   `json:"nr_cgi"`
	TAI       TAI     `json:"tai"`
	TimeStamp *string `json:"timestamp,omitempty"`
}

type UserLocationInformationN3IWF struct {
	IPAddress  string `json:"ip_address"`
	PortNumber int32  `json:"port_number"`
}

type UserLocationInformation struct {
	EUTRA *UserLocationInformationEUTRA `json:"eutra,omitempty"`
	NR    *UserLocationInformationNR    `json:"nr,omitempty"`
	N3IWF *UserLocationInformationN3IWF `json:"n3iwf,omitempty"`
}

type FiveGSTMSI struct {
	AMFSetID   string `json:"amf_set_id"`
	AMFPointer string `json:"amf_pointer"`
	FiveGTMSI  string `json:"fiveg_tmsi"`
}

type RATRestriction struct {
	PLMNID                    PLMNID `json:"plmn_id"`
	RATRestrictionInformation string `json:"rat_restriction_information"`
}

type ForbiddenAreaInformation struct {
	PLMNID        PLMNID   `json:"plmn_id"`
	ForbiddenTACs []string `json:"forbidden_tacs"`
}

type ServiceAreaInformation struct {
	PLMNID         PLMNID   `json:"plmn_id"`
	AllowedTACs    []string `json:"allowed_tacs,omitempty"`
	NotAllowedTACs []string `json:"not_allowed_tacs,omitempty"`
}

type MobilityRestrictionList struct {
	ServingPLMN              PLMNID                     `json:"serving_plmn"`
	EquivalentPLMNs          []PLMNID                   `json:"equivalent_plmns,omitempty"`
	RATRestrictions          []RATRestriction           `json:"rat_restrictions,omitempty"`
	ForbiddenAreaInformation []ForbiddenAreaInformation `json:"forbidden_area_information,omitempty"`
	ServiceAreaInformation   []ServiceAreaInformation   `json:"service_area_information,omitempty"`
}

type UEAggregateMaximumBitRate struct {
	Downlink int64 `json:"downlink"`
	Uplink   int64 `json:"uplink"`
}

type ExpectedUEActivityBehaviour struct {
	ExpectedActivityPeriod                 *int64  `json:"expected_activity_period,omitempty"`
	ExpectedIdlePeriod                     *int64  `json:"expected_idle_period,omitempty"`
	SourceOfUEActivityBehaviourInformation *string `json:"source_of_ue_activity_behaviour_information,omitempty"`
}

type NGRANCGI struct {
	NRCGI    *NRCGI    `json:"nr_ran_cgi,omitempty"`
	EUTRACGI *EUTRACGI `json:"eutra_cgi,omitempty"`
}

type ExpectedUEMovingTrajectoryItem struct {
	NGRANCGI         NGRANCGI `json:"ng_ran_cgi"`
	TimeStayedInCell *int64   `json:"time_stayed_in_cell,omitempty"`
}

type ExpectedUEBehaviour struct {
	ExpectedUEActivityBehaviour *ExpectedUEActivityBehaviour     `json:"expected_ue_activity_behaviour,omitempty"`
	ExpectedHOInterval          *string                          `json:"expected_ho_interval,omitempty"`
	ExpectedUEMobility          *string                          `json:"expected_ue_mobility,omitempty"`
	ExpectedUEMovingTrajectory  []ExpectedUEMovingTrajectoryItem `json:"expected_ue_moving_trajectory,omitempty"`
}

type CoreNetworkAssistanceInformation struct {
	UEIdentityIndexValue            string               `json:"ue_identity_index_value"`
	UESpecificDRX                   *string              `json:"ue_specific_drx,omitempty"`
	PeriodicRegistrationUpdateTimer string               `json:"periodic_registration_update_timer"`
	MICOModeIndication              *string              `json:"mico_mode_indication,omitempty"`
	TAIListForInactive              []TAI                `json:"tai_list_for_inactive,omitempty"`
	ExpectedUEBehaviour             *ExpectedUEBehaviour `json:"expected_ue_behaviour,omitempty"`
}

type PDUSessionResourceSetupCxtReq struct {
	PDUSessionID                           int64   `json:"pdu_session_id"`
	NASPDU                                 *NASPDU `json:"nas_pdu,omitempty"`
	SNSSAI                                 SNSSAI  `json:"snssai"`
	PDUSessionResourceSetupRequestTransfer []byte  `json:"pdu_session_resource_setup_request_transfer"`
}

type PDUSessionResourceSetupCxtRes struct {
	PDUSessionID                            int64  `json:"pdu_session_id"`
	PDUSessionResourceSetupResponseTransfer []byte `json:"pdu_session_resource_setup_response_transfer"`
}

type PDUSessionResourceFailedToSetupCxtRes struct {
	PDUSessionID                                int64  `json:"pdu_session_id"`
	PDUSessionResourceSetupUnsuccessfulTransfer []byte `json:"pdu_session_resource_setup_unsuccessful_transfer"`
}

type PDUSessionResourceSetupSUReq struct {
	PDUSessionID                           int64   `json:"pdu_session_id"`
	PDUSessionNASPDU                       *NASPDU `json:"pdu_session_nas_pdu,omitempty"`
	SNSSAI                                 SNSSAI  `json:"snssai"`
	PDUSessionResourceSetupRequestTransfer []byte  `json:"pdu_session_resource_setup_request_transfer"`
}

type PDUSessionResourceSetupSURes struct {
	PDUSessionID                            int64  `json:"pdu_session_id"`
	PDUSessionResourceSetupResponseTransfer []byte `json:"pdu_session_resource_setup_response_transfer"`
}

type PDUSessionResourceFailedToSetupSURes struct {
	PDUSessionID                                int64  `json:"pdu_session_id"`
	PDUSessionResourceSetupUnsuccessfulTransfer []byte `json:"pdu_session_resource_setup_unsuccessful_transfer"`
}

type UESecurityCapabilities struct {
	NRencryptionAlgorithms             []string `json:"nr_encryption_algorithms"`
	NRintegrityProtectionAlgorithms    []string `json:"nr_integrity_protection_algorithms"`
	EUTRAencryptionAlgorithms          string   `json:"eutra_encryption_algorithms"`
	EUTRAintegrityProtectionAlgorithms string   `json:"eutra_integrity_protection_algorithms"`
}

type NASPDU struct {
	Raw     []byte          `json:"raw"`
	Decoded *nas.NASMessage `json:"decoded"`
}

type IE struct {
	ID                                        string                                  `json:"id"`
	Criticality                               string                                  `json:"criticality"`
	GlobalRANNodeID                           *GlobalRANNodeIDIE                      `json:"global_ran_node_id,omitempty"`
	RANNodeName                               *string                                 `json:"ran_node_name,omitempty"`
	SupportedTAList                           []SupportedTA                           `json:"supported_ta_list,omitempty"`
	DefaultPagingDRX                          *string                                 `json:"default_paging_drx,omitempty"`
	UERetentionInformation                    *string                                 `json:"ue_retention_information,omitempty"`
	AMFName                                   *string                                 `json:"amf_name,omitempty"`
	ServedGUAMIList                           []Guami                                 `json:"served_guami_list,omitempty"`
	RelativeAMFCapacity                       *int64                                  `json:"relative_amf_capacity,omitempty"`
	PLMNSupportList                           []PLMN                                  `json:"plmn_support_list,omitempty"`
	CriticalityDiagnostics                    *CriticalityDiagnostics                 `json:"criticality_diagnostics,omitempty"`
	Cause                                     *string                                 `json:"cause,omitempty"`
	TimeToWait                                *string                                 `json:"time_to_wait,omitempty"`
	RANUENGAPID                               *int64                                  `json:"ran_ue_ngap_id,omitempty"`
	NASPDU                                    *NASPDU                                 `json:"nas_pdu,omitempty"`
	UserLocationInformation                   *UserLocationInformation                `json:"user_location_information,omitempty"`
	RRCEstablishmentCause                     *string                                 `json:"rrc_establishment_cause,omitempty"`
	FiveGSTMSI                                *FiveGSTMSI                             `json:"fiveg_stmsi,omitempty"`
	AMFSetID                                  *string                                 `json:"amf_set_id,omitempty"`
	UEContextRequest                          *string                                 `json:"ue_context_request,omitempty"`
	AllowedNSSAI                              []SNSSAI                                `json:"allowed_nssai,omitempty"`
	AMFUENGAPID                               *int64                                  `json:"amf_ue_ngap_id,omitempty"`
	OldAMF                                    *string                                 `json:"old_amf,omitempty"`
	RANPagingPriority                         *int64                                  `json:"ran_paging_priority,omitempty"`
	MobilityRestrictionList                   *MobilityRestrictionList                `json:"mobility_restriction_list,omitempty"`
	IndexToRFSP                               *int64                                  `json:"index_to_rfsp,omitempty"`
	UEAggregateMaximumBitRate                 *UEAggregateMaximumBitRate              `json:"ue_aggregate_maximum_bit_rate,omitempty"`
	CoreNetworkAssistanceInformation          *CoreNetworkAssistanceInformation       `json:"core_network_assistance_information,omitempty"`
	GUAMI                                     *Guami                                  `json:"guami,omitempty"`
	PDUSessionResourceSetupListCxtReq         []PDUSessionResourceSetupCxtReq         `json:"pdu_session_resource_setup_list_cxt_req,omitempty"`
	PDUSessionResourceSetupListCxtRes         []PDUSessionResourceSetupCxtRes         `json:"pdu_session_resource_setup_list_cxt_res,omitempty"`
	PDUSessionResourceFailedToSetupListCxtRes []PDUSessionResourceFailedToSetupCxtRes `json:"pdu_session_resource_failed_to_setup_list_cxt_res,omitempty"`
	PDUSessionResourceSetupListSUReq          []PDUSessionResourceSetupSUReq          `json:"pdu_session_resource_setup_list_su_req,omitempty"`
	PDUSessionResourceSetupListSURes          []PDUSessionResourceSetupSURes          `json:"pdu_session_resource_setup_list_su_res,omitempty"`
	PDUSessionResourceFailedToSetupListSURes  []PDUSessionResourceFailedToSetupSURes  `json:"pdu_session_resource_failed_to_setup_list_su_res,omitempty"`
	UESecurityCapabilities                    *UESecurityCapabilities                 `json:"ue_security_capabilities,omitempty"`
	SecurityKey                               *string                                 `json:"security_key,omitempty"`
}

type InitialUEMessage struct {
	IEs []IE `json:"ies"`
}

type PDUSessionResourceSetupRequest struct {
	IEs []IE `json:"ies"`
}

type InitiatingMessageValue struct {
	NGSetupRequest                 *NGSetupRequest                 `json:"ng_setup_request,omitempty"`
	InitialUEMessage               *InitialUEMessage               `json:"initial_ue_message,omitempty"`
	DownlinkNASTransport           *DownlinkNASTransport           `json:"downlink_nas_transport,omitempty"`
	UplinkNASTransport             *UplinkNASTransport             `json:"uplink_nas_transport,omitempty"`
	InitialContextSetupRequest     *InitialContextSetupRequest     `json:"initial_context_setup_request,omitempty"`
	PDUSessionResourceSetupRequest *PDUSessionResourceSetupRequest `json:"pdu_session_resource_setup_request,omitempty"`
}

type InitiatingMessage struct {
	ProcedureCode string                 `json:"procedure_code"`
	Criticality   string                 `json:"criticality"`
	Value         InitiatingMessageValue `json:"value"`
}

type SuccessfulOutcomeValue struct {
	NGSetupResponse                 *NGSetupResponse                 `json:"ng_setup_response,omitempty"`
	InitialContextSetupResponse     *InitialContextSetupResponse     `json:"initial_context_setup_response,omitempty"`
	PDUSessionResourceSetupResponse *PDUSessionResourceSetupResponse `json:"pdu_session_resource_setup_response,omitempty"`
}

type SuccessfulOutcome struct {
	ProcedureCode string                 `json:"procedure_code"`
	Criticality   string                 `json:"criticality"`
	Value         SuccessfulOutcomeValue `json:"value"`
}

type UnsuccessfulOutcomeValue struct {
	NGSetupFailure *NGSetupFailure `json:"ng_setup_failure,omitempty"`
}

type UnsuccessfulOutcome struct {
	ProcedureCode string                   `json:"procedure_code"`
	Criticality   string                   `json:"criticality"`
	Value         UnsuccessfulOutcomeValue `json:"value"`
}

type NGAPMessage struct {
	InitiatingMessage   *InitiatingMessage   `json:"initiating_message,omitempty"`
	SuccessfulOutcome   *SuccessfulOutcome   `json:"successful_outcome,omitempty"`
	UnsuccessfulOutcome *UnsuccessfulOutcome `json:"unsuccessful_outcome,omitempty"`
}

func DecodeNGAPMessage(raw []byte) (*NGAPMessage, error) {
	pdu, err := ngap.Decoder(raw)
	if err != nil {
		return nil, fmt.Errorf("failed to decode NGAP message: %w", err)
	}

	ngapMsg := &NGAPMessage{}

	switch pdu.Present {
	case ngapType.NGAPPDUPresentInitiatingMessage:
		ngapMsg.InitiatingMessage = buildInitiatingMessage(pdu.InitiatingMessage)
		return ngapMsg, nil
	case ngapType.NGAPPDUPresentSuccessfulOutcome:
		ngapMsg.SuccessfulOutcome = buildSuccessfulOutcome(pdu.SuccessfulOutcome)
		return ngapMsg, nil
	case ngapType.NGAPPDUPresentUnsuccessfulOutcome:
		ngapMsg.UnsuccessfulOutcome = buildUnsuccessfulOutcome(pdu.UnsuccessfulOutcome)
		return ngapMsg, nil
	default:
		return nil, fmt.Errorf("unknown NGAP PDU type: %d", pdu.Present)
	}
}

func buildInitiatingMessage(initMsg *ngapType.InitiatingMessage) *InitiatingMessage {
	if initMsg == nil {
		return nil
	}

	initiatingMsg := &InitiatingMessage{
		ProcedureCode: procedureCodeToString(initMsg.ProcedureCode.Value),
		Criticality:   criticalityToString(initMsg.Criticality.Value),
		Value:         InitiatingMessageValue{},
	}

	switch initMsg.Value.Present {
	case ngapType.InitiatingMessagePresentNGSetupRequest:
		initiatingMsg.Value.NGSetupRequest = buildNGSetupRequest(initMsg.Value.NGSetupRequest)
		return initiatingMsg
	case ngapType.InitiatingMessagePresentInitialUEMessage:
		initiatingMsg.Value.InitialUEMessage = buildInitialUEMessage(initMsg.Value.InitialUEMessage)
		return initiatingMsg
	case ngapType.InitiatingMessagePresentDownlinkNASTransport:
		initiatingMsg.Value.DownlinkNASTransport = buildDownlinkNASTransport(initMsg.Value.DownlinkNASTransport)
		return initiatingMsg
	case ngapType.InitiatingMessagePresentUplinkNASTransport:
		initiatingMsg.Value.UplinkNASTransport = buildUplinkNASTransport(initMsg.Value.UplinkNASTransport)
		return initiatingMsg
	case ngapType.InitiatingMessagePresentInitialContextSetupRequest:
		initiatingMsg.Value.InitialContextSetupRequest = buildInitialContextSetupRequest(initMsg.Value.InitialContextSetupRequest)
		return initiatingMsg
	case ngapType.InitiatingMessagePresentPDUSessionResourceSetupRequest:
		initiatingMsg.Value.PDUSessionResourceSetupRequest = buildPDUSessionResourceSetupRequest(initMsg.Value.PDUSessionResourceSetupRequest)
		return initiatingMsg
	default:
		logger.EllaLog.Warn("Unsupported procedure code", zap.Int("present", initMsg.Value.Present))
		return initiatingMsg
	}
}

func buildSuccessfulOutcome(sucMsg *ngapType.SuccessfulOutcome) *SuccessfulOutcome {
	if sucMsg == nil {
		return nil
	}
	successfulOutcome := &SuccessfulOutcome{
		ProcedureCode: procedureCodeToString(sucMsg.ProcedureCode.Value),
		Criticality:   criticalityToString(sucMsg.Criticality.Value),
		Value:         SuccessfulOutcomeValue{},
	}

	switch sucMsg.Value.Present {
	case ngapType.SuccessfulOutcomePresentNGSetupResponse:
		successfulOutcome.Value.NGSetupResponse = buildNGSetupResponse(sucMsg.Value.NGSetupResponse)
		return successfulOutcome
	case ngapType.SuccessfulOutcomePresentInitialContextSetupResponse:
		successfulOutcome.Value.InitialContextSetupResponse = buildInitialContextSetupResponse(sucMsg.Value.InitialContextSetupResponse)
		return successfulOutcome
	case ngapType.SuccessfulOutcomePresentPDUSessionResourceSetupResponse:
		successfulOutcome.Value.PDUSessionResourceSetupResponse = buildPDUSessionResourceSetupResponse(sucMsg.Value.PDUSessionResourceSetupResponse)
		return successfulOutcome
	default:
		logger.EllaLog.Warn("Unsupported message", zap.Int("present", sucMsg.Value.Present))
		return successfulOutcome
	}
}

func buildUnsuccessfulOutcome(unsucMsg *ngapType.UnsuccessfulOutcome) *UnsuccessfulOutcome {
	if unsucMsg == nil {
		return nil
	}

	unsuccessfulOutcome := &UnsuccessfulOutcome{
		ProcedureCode: procedureCodeToString(unsucMsg.ProcedureCode.Value),
		Criticality:   criticalityToString(unsucMsg.Criticality.Value),
		Value:         UnsuccessfulOutcomeValue{},
	}

	switch unsucMsg.Value.Present {
	case ngapType.UnsuccessfulOutcomePresentNGSetupFailure:
		unsuccessfulOutcome.Value.NGSetupFailure = buildNGSetupFailure(unsucMsg.Value.NGSetupFailure)
		return unsuccessfulOutcome
	default:
		logger.EllaLog.Warn("Unsupported message", zap.Int("present", unsucMsg.Value.Present))
		return unsuccessfulOutcome
	}
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
