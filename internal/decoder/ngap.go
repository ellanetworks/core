package decoder

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/omec-project/ngap"
	"github.com/omec-project/ngap/aper"
	"github.com/omec-project/ngap/ngapConvert"
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
	UEAggregateMaximumBitRateDL int64 `json:"ue_aggregate_maximum_bit_rate_dl"`
	UEAggregateMaximumBitRateUL int64 `json:"ue_aggregate_maximum_bit_rate_ul"`
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
	PDUSessionID                           int64  `json:"pdu_session_id"`
	NASPDU                                 []byte `json:"nas_pdu,omitempty"`
	SNSSAI                                 SNSSAI `json:"snssai"`
	PDUSessionResourceSetupRequestTransfer []byte `json:"pdu_session_resource_setup_request_transfer"`
}

type UESecurityCapabilities struct {
	NRencryptionAlgorithms             string `json:"nr_encryption_algorithms"`
	NRintegrityProtectionAlgorithms    string `json:"nr_integrity_protection_algorithms"`
	EUTRAencryptionAlgorithms          string `json:"eutra_encryption_algorithms"`
	EUTRAintegrityProtectionAlgorithms string `json:"eutra_integrity_protection_algorithms"`
}

type IE struct {
	ID                                string                            `json:"id"`
	Criticality                       string                            `json:"criticality"`
	GlobalRANNodeID                   *GlobalRANNodeIDIE                `json:"global_ran_node_id,omitempty"`
	RANNodeName                       *string                           `json:"ran_node_name,omitempty"`
	SupportedTAList                   []SupportedTA                     `json:"supported_ta_list,omitempty"`
	DefaultPagingDRX                  *string                           `json:"default_paging_drx,omitempty"`
	UERetentionInformation            *string                           `json:"ue_retention_information,omitempty"`
	AMFName                           *string                           `json:"amf_name,omitempty"`
	ServedGUAMIList                   []Guami                           `json:"served_guami_list,omitempty"`
	RelativeAMFCapacity               *int64                            `json:"relative_amf_capacity,omitempty"`
	PLMNSupportList                   []PLMN                            `json:"plmn_support_list,omitempty"`
	CriticalityDiagnostics            *CriticalityDiagnostics           `json:"criticality_diagnostics,omitempty"`
	Cause                             *string                           `json:"cause,omitempty"`
	TimeToWait                        *string                           `json:"time_to_wait,omitempty"`
	RANUENGAPID                       *int64                            `json:"ran_ue_ngap_id,omitempty"`
	NASPDU                            []byte                            `json:"nas_pdu,omitempty"`
	UserLocationInformation           *UserLocationInformation          `json:"user_location_information,omitempty"`
	RRCEstablishmentCause             *string                           `json:"rrc_establishment_cause,omitempty"`
	FiveGSTMSI                        *FiveGSTMSI                       `json:"fiveg_stmsi,omitempty"`
	AMFSetID                          *string                           `json:"amf_set_id,omitempty"`
	UEContextRequest                  *string                           `json:"ue_context_request,omitempty"`
	AllowedNSSAI                      []SNSSAI                          `json:"allowed_nssai,omitempty"`
	AMFUENGAPID                       *int64                            `json:"amf_ue_ngap_id,omitempty"`
	OldAMF                            *string                           `json:"old_amf,omitempty"`
	RANPagingPriority                 *int64                            `json:"ran_paging_priority,omitempty"`
	MobilityRestrictionList           *MobilityRestrictionList          `json:"mobility_restriction_list,omitempty"`
	IndexToRFSP                       *int64                            `json:"index_to_rfsp,omitempty"`
	UEAggregateMaximumBitRate         *UEAggregateMaximumBitRate        `json:"ue_aggregate_maximum_bit_rate,omitempty"`
	CoreNetworkAssistanceInformation  *CoreNetworkAssistanceInformation `json:"core_network_assistance_information,omitempty"`
	GUAMI                             *Guami                            `json:"guami,omitempty"`
	PDUSessionResourceSetupListCxtReq []PDUSessionResourceSetupCxtReq   `json:"pdu_session_resource_setup_list_cxt_req,omitempty"`
	UESecurityCapabilities            *UESecurityCapabilities           `json:"ue_security_capabilities,omitempty"`
	SecurityKey                       *string                           `json:"security_key,omitempty"`
}

type NGSetupRequest struct {
	IEs []IE `json:"ies"`
}

type InitialUEMessage struct {
	IEs []IE `json:"ies"`
}

type DownlinkNASTransport struct {
	IEs []IE `json:"ies"`
}

type UplinkNASTransport struct {
	IEs []IE `json:"ies"`
}

type InitialContextSetupRequest struct {
	IEs []IE `json:"ies"`
}

type InitiatingMessageValue struct {
	NGSetupRequest             *NGSetupRequest             `json:"ng_setup_request,omitempty"`
	InitialUEMessage           *InitialUEMessage           `json:"initial_ue_message,omitempty"`
	DownlinkNASTransport       *DownlinkNASTransport       `json:"downlink_nas_transport,omitempty"`
	UplinkNASTransport         *UplinkNASTransport         `json:"uplink_nas_transport,omitempty"`
	InitialContextSetupRequest *InitialContextSetupRequest `json:"initial_context_setup_request,omitempty"`
}

type InitiatingMessage struct {
	ProcedureCode string                 `json:"procedure_code"`
	Criticality   string                 `json:"criticality"`
	Value         InitiatingMessageValue `json:"value"`
}

type NGSetupResponse struct {
	IEs []IE `json:"ies"`
}

type SuccessfulOutcomeValue struct {
	NGSetupResponse *NGSetupResponse `json:"ng_setup_response,omitempty"`
}

type SuccessfulOutcome struct {
	ProcedureCode string                 `json:"procedure_code"`
	Criticality   string                 `json:"criticality"`
	Value         SuccessfulOutcomeValue `json:"value"`
}

type NGSetupFailure struct {
	IEs []IE `json:"ies"`
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

func DecodeNetworkLog(raw []byte) (*NGAPMessage, error) {
	pdu, err := ngap.Decoder(raw)
	if err != nil {
		return nil, fmt.Errorf("failed to decode NGAP message: %w", err)
	}

	ngapMsg := &NGAPMessage{}

	// Extract message type
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

func buildInitialContextSetupRequest(initialContextSetupRequest *ngapType.InitialContextSetupRequest) *InitialContextSetupRequest {
	if initialContextSetupRequest == nil {
		return nil
	}

	ieList := &InitialContextSetupRequest{}

	for i := 0; i < len(initialContextSetupRequest.ProtocolIEs.List); i++ {
		ie := initialContextSetupRequest.ProtocolIEs.List[i]
		switch ie.Id.Value {
		case ngapType.ProtocolIEIDAMFUENGAPID:
			ieList.IEs = append(ieList.IEs, IE{
				ID:          protocolIEIDToString(ie.Id.Value),
				Criticality: criticalityToString(ie.Criticality.Value),
				AMFUENGAPID: &ie.Value.AMFUENGAPID.Value,
			})
		case ngapType.ProtocolIEIDRANUENGAPID:
			ieList.IEs = append(ieList.IEs, IE{
				ID:          protocolIEIDToString(ie.Id.Value),
				Criticality: criticalityToString(ie.Criticality.Value),
				RANUENGAPID: &ie.Value.RANUENGAPID.Value,
			})
		case ngapType.ProtocolIEIDOldAMF:
			ieList.IEs = append(ieList.IEs, IE{
				ID:          protocolIEIDToString(ie.Id.Value),
				Criticality: criticalityToString(ie.Criticality.Value),
				OldAMF:      &ie.Value.OldAMF.Value,
			})
		case ngapType.ProtocolIEIDUEAggregateMaximumBitRate:
			ieList.IEs = append(ieList.IEs, IE{
				ID:          protocolIEIDToString(ie.Id.Value),
				Criticality: criticalityToString(ie.Criticality.Value),
				UEAggregateMaximumBitRate: &UEAggregateMaximumBitRate{
					UEAggregateMaximumBitRateDL: ie.Value.UEAggregateMaximumBitRate.UEAggregateMaximumBitRateDL.Value,
					UEAggregateMaximumBitRateUL: ie.Value.UEAggregateMaximumBitRate.UEAggregateMaximumBitRateUL.Value,
				},
			})
		case ngapType.ProtocolIEIDCoreNetworkAssistanceInformation:
			ieList.IEs = append(ieList.IEs, IE{
				ID:                               protocolIEIDToString(ie.Id.Value),
				Criticality:                      criticalityToString(ie.Criticality.Value),
				CoreNetworkAssistanceInformation: buildCoreNetworkAssistanceInformation(ie.Value.CoreNetworkAssistanceInformation),
			})
		case ngapType.ProtocolIEIDGUAMI:
			ieList.IEs = append(ieList.IEs, IE{
				ID:          protocolIEIDToString(ie.Id.Value),
				Criticality: criticalityToString(ie.Criticality.Value),
				GUAMI:       buildGUAMI(ie.Value.GUAMI),
			})
		case ngapType.ProtocolIEIDPDUSessionResourceSetupListCxtReq:
			ieList.IEs = append(ieList.IEs, IE{
				ID:                                protocolIEIDToString(ie.Id.Value),
				Criticality:                       criticalityToString(ie.Criticality.Value),
				PDUSessionResourceSetupListCxtReq: buildPDUSessionResourceSetupListCxtReq(ie.Value.PDUSessionResourceSetupListCxtReq),
			})
		case ngapType.ProtocolIEIDAllowedNSSAI:
			ieList.IEs = append(ieList.IEs, IE{
				ID:           protocolIEIDToString(ie.Id.Value),
				Criticality:  criticalityToString(ie.Criticality.Value),
				AllowedNSSAI: buildAllowedNSSAI(ie.Value.AllowedNSSAI),
			})
		case ngapType.ProtocolIEIDUESecurityCapabilities:
			ieList.IEs = append(ieList.IEs, IE{
				ID:                     protocolIEIDToString(ie.Id.Value),
				Criticality:            criticalityToString(ie.Criticality.Value),
				UESecurityCapabilities: buildUESecurityCapabilities(ie.Value.UESecurityCapabilities),
			})
		case ngapType.ProtocolIEIDSecurityKey:
			securityKey := bitStringToHex(&ie.Value.SecurityKey.Value)
			ieList.IEs = append(ieList.IEs, IE{
				ID:          protocolIEIDToString(ie.Id.Value),
				Criticality: criticalityToString(ie.Criticality.Value),
				SecurityKey: &securityKey,
			})
		case ngapType.ProtocolIEIDMobilityRestrictionList:
			ieList.IEs = append(ieList.IEs, IE{
				ID:                      protocolIEIDToString(ie.Id.Value),
				Criticality:             criticalityToString(ie.Criticality.Value),
				MobilityRestrictionList: buildMobilityRestrictionListIE(ie.Value.MobilityRestrictionList),
			})
		case ngapType.ProtocolIEIDIndexToRFSP:
			ieList.IEs = append(ieList.IEs, IE{
				ID:          protocolIEIDToString(ie.Id.Value),
				Criticality: criticalityToString(ie.Criticality.Value),
				IndexToRFSP: &ie.Value.IndexToRFSP.Value,
			})
		case ngapType.ProtocolIEIDNASPDU:
			ieList.IEs = append(ieList.IEs, IE{
				ID:          protocolIEIDToString(ie.Id.Value),
				Criticality: criticalityToString(ie.Criticality.Value),
				NASPDU:      ie.Value.NASPDU.Value,
			})
		default:
			ieList.IEs = append(ieList.IEs, IE{
				ID:          protocolIEIDToString(ie.Id.Value),
				Criticality: criticalityToString(ie.Criticality.Value),
			})
			logger.EllaLog.Warn("Unsupported ie type", zap.Int64("type", ie.Id.Value))
		}
	}

	// missing IEs:
	// - TraceActivation
	// - UERadioCapability
	// - MaskedIMEISV
	// - EmergencyFallbackIndicator
	// - RRCInactiveTransitionReportRequest
	// - UERadioCapabilityForPaging
	// - RedirectionVoiceFallback

	return ieList
}

func buildUESecurityCapabilities(uesec *ngapType.UESecurityCapabilities) *UESecurityCapabilities {
	if uesec == nil {
		return nil
	}

	return &UESecurityCapabilities{
		NRencryptionAlgorithms:             bitStringToHex(&uesec.NRencryptionAlgorithms.Value),
		NRintegrityProtectionAlgorithms:    bitStringToHex(&uesec.NRintegrityProtectionAlgorithms.Value),
		EUTRAencryptionAlgorithms:          bitStringToHex(&uesec.EUTRAencryptionAlgorithms.Value),
		EUTRAintegrityProtectionAlgorithms: bitStringToHex(&uesec.EUTRAintegrityProtectionAlgorithms.Value),
	}
}

func buildPDUSessionResourceSetupListCxtReq(pduSessionResourceSetupListCxtReq *ngapType.PDUSessionResourceSetupListCxtReq) []PDUSessionResourceSetupCxtReq {
	if pduSessionResourceSetupListCxtReq == nil {
		return nil
	}

	var pduSessionResourceSetupList []PDUSessionResourceSetupCxtReq

	for i := 0; i < len(pduSessionResourceSetupListCxtReq.List); i++ {
		item := pduSessionResourceSetupListCxtReq.List[i]
		pduSessionResourceSetupList = append(pduSessionResourceSetupList, PDUSessionResourceSetupCxtReq{
			PDUSessionID:                           item.PDUSessionID.Value,
			NASPDU:                                 item.NASPDU.Value,
			SNSSAI:                                 *buildSNSSAI(&item.SNSSAI),
			PDUSessionResourceSetupRequestTransfer: item.PDUSessionResourceSetupRequestTransfer,
		})
	}

	return pduSessionResourceSetupList
}

func buildCoreNetworkAssistanceInformation(cnai *ngapType.CoreNetworkAssistanceInformation) *CoreNetworkAssistanceInformation {
	if cnai == nil {
		return nil
	}

	returnedCNAI := &CoreNetworkAssistanceInformation{}

	switch cnai.UEIdentityIndexValue.Present {
	case ngapType.UEIdentityIndexValuePresentIndexLength10:
		returnedCNAI.UEIdentityIndexValue = bitStringToHex(cnai.UEIdentityIndexValue.IndexLength10)
	default:
		logger.EllaLog.Warn("Unsupported UEIdentityIndexValue", zap.Int("present", cnai.UEIdentityIndexValue.Present))
	}

	if cnai.UESpecificDRX != nil {
		returnedCNAI.UESpecificDRX = buildDefaultPagingDRXIE(cnai.UESpecificDRX)
	}

	returnedCNAI.PeriodicRegistrationUpdateTimer = bitStringToHex(&cnai.PeriodicRegistrationUpdateTimer.Value)

	if cnai.MICOModeIndication != nil {
		switch cnai.MICOModeIndication.Value {
		case ngapType.MICOModeIndicationPresentTrue:
			returnedCNAI.MICOModeIndication = new(string)
			*returnedCNAI.MICOModeIndication = "true"
		default:
			logger.EllaLog.Warn("Unsupported MICOModeIndication", zap.Int64("present", int64(cnai.MICOModeIndication.Value)))
		}
	}

	for i := 0; i < len(cnai.TAIListForInactive.List); i++ {
		tai := cnai.TAIListForInactive.List[i]
		returnedCNAI.TAIListForInactive = append(returnedCNAI.TAIListForInactive, TAI{
			PLMNID: plmnIDToModels(tai.TAI.PLMNIdentity),
			TAC:    hex.EncodeToString(tai.TAI.TAC.Value),
		})
	}

	if cnai.ExpectedUEBehaviour != nil {
		returnedCNAI.ExpectedUEBehaviour = buildExpectedUEBehaviour(cnai.ExpectedUEBehaviour)
	}

	return returnedCNAI
}

func buildExpectedUEBehaviour(eub *ngapType.ExpectedUEBehaviour) *ExpectedUEBehaviour {
	if eub == nil {
		return nil
	}

	returnedEUB := &ExpectedUEBehaviour{}

	if eub.ExpectedUEActivityBehaviour != nil {
		returnedEUB.ExpectedUEActivityBehaviour = &ExpectedUEActivityBehaviour{}

		if eub.ExpectedUEActivityBehaviour.ExpectedActivityPeriod != nil {
			returnedEUB.ExpectedUEActivityBehaviour.ExpectedActivityPeriod = &eub.ExpectedUEActivityBehaviour.ExpectedActivityPeriod.Value
		}

		if eub.ExpectedUEActivityBehaviour.ExpectedIdlePeriod != nil {
			returnedEUB.ExpectedUEActivityBehaviour.ExpectedIdlePeriod = &eub.ExpectedUEActivityBehaviour.ExpectedIdlePeriod.Value
		}

		if eub.ExpectedUEActivityBehaviour.SourceOfUEActivityBehaviourInformation != nil {
			switch eub.ExpectedUEActivityBehaviour.SourceOfUEActivityBehaviourInformation.Value {
			case ngapType.SourceOfUEActivityBehaviourInformationPresentSubscriptionInformation:
				returnedEUB.ExpectedUEActivityBehaviour.SourceOfUEActivityBehaviourInformation = new(string)
				*returnedEUB.ExpectedUEActivityBehaviour.SourceOfUEActivityBehaviourInformation = "subscription"
			case ngapType.SourceOfUEActivityBehaviourInformationPresentStatistics:
				returnedEUB.ExpectedUEActivityBehaviour.SourceOfUEActivityBehaviourInformation = new(string)
				*returnedEUB.ExpectedUEActivityBehaviour.SourceOfUEActivityBehaviourInformation = "statistics"
			default:
				logger.EllaLog.Warn("Unsupported SourceOfUEActivityBehaviourInformation", zap.Int64("present", int64(eub.ExpectedUEActivityBehaviour.SourceOfUEActivityBehaviourInformation.Value)))
			}
		}
	}

	if eub.ExpectedHOInterval != nil {
		switch eub.ExpectedHOInterval.Value {
		case ngapType.ExpectedHOIntervalPresentSec15:
			returnedEUB.ExpectedHOInterval = new(string)
			*returnedEUB.ExpectedHOInterval = "sec15"
		case ngapType.ExpectedHOIntervalPresentSec30:
			returnedEUB.ExpectedHOInterval = new(string)
			*returnedEUB.ExpectedHOInterval = "sec30"
		case ngapType.ExpectedHOIntervalPresentSec60:
			returnedEUB.ExpectedHOInterval = new(string)
			*returnedEUB.ExpectedHOInterval = "sec60"
		case ngapType.ExpectedHOIntervalPresentSec120:
			returnedEUB.ExpectedHOInterval = new(string)
			*returnedEUB.ExpectedHOInterval = "sec120"
		case ngapType.ExpectedHOIntervalPresentSec180:
			returnedEUB.ExpectedHOInterval = new(string)
			*returnedEUB.ExpectedHOInterval = "sec180"
		case ngapType.ExpectedHOIntervalPresentLongTime:
			returnedEUB.ExpectedHOInterval = new(string)
			*returnedEUB.ExpectedHOInterval = "long"
		default:
			logger.EllaLog.Warn("Unsupported ExpectedHOInterval", zap.Int64("present", int64(eub.ExpectedHOInterval.Value)))
		}
	}

	if eub.ExpectedUEMobility != nil {
		switch eub.ExpectedUEMobility.Value {
		case ngapType.ExpectedUEMobilityPresentStationary:
			returnedEUB.ExpectedUEMobility = new(string)
			*returnedEUB.ExpectedUEMobility = "stationary"
		case ngapType.ExpectedUEMobilityPresentMobile:
			returnedEUB.ExpectedUEMobility = new(string)
			*returnedEUB.ExpectedUEMobility = "mobile"
		default:
			logger.EllaLog.Warn("Unsupported ExpectedUEMobility", zap.Int64("present", int64(eub.ExpectedUEMobility.Value)))
		}
	}

	for i := 0; i < len(eub.ExpectedUEMovingTrajectory.List); i++ {
		item := eub.ExpectedUEMovingTrajectory.List[i]
		ngRanCgi := buildNGRANCGI(item.NGRANCGI)
		expectedUEMovingTrajectoryItem := ExpectedUEMovingTrajectoryItem{
			NGRANCGI: ngRanCgi,
		}
		if item.TimeStayedInCell != nil {
			expectedUEMovingTrajectoryItem.TimeStayedInCell = item.TimeStayedInCell
		}
		returnedEUB.ExpectedUEMovingTrajectory = append(returnedEUB.ExpectedUEMovingTrajectory, expectedUEMovingTrajectoryItem)
	}

	return returnedEUB
}

func buildNGRANCGI(ngRanCgi ngapType.NGRANCGI) NGRANCGI {
	ngRANCGI := NGRANCGI{}

	switch ngRanCgi.Present {
	case ngapType.NGRANCGIPresentNRCGI:
		ngRANCGI.NRCGI = &NRCGI{
			PLMNID:         plmnIDToModels(ngRanCgi.NRCGI.PLMNIdentity),
			NRCellIdentity: bitStringToHex(&ngRanCgi.NRCGI.NRCellIdentity.Value),
		}
	case ngapType.NGRANCGIPresentEUTRACGI:
		ngRANCGI.EUTRACGI = &EUTRACGI{
			PLMNID:            plmnIDToModels(ngRanCgi.EUTRACGI.PLMNIdentity),
			EUTRACellIdentity: bitStringToHex(&ngRanCgi.EUTRACGI.EUTRACellIdentity.Value),
		}
	default:
		logger.EllaLog.Warn("Unsupported NGRANCGI", zap.Int("present", ngRanCgi.Present))
	}

	return ngRANCGI
}

func buildUplinkNASTransport(uplinkNASTransport *ngapType.UplinkNASTransport) *UplinkNASTransport {
	if uplinkNASTransport == nil {
		return nil
	}

	ieList := &UplinkNASTransport{}

	for i := 0; i < len(uplinkNASTransport.ProtocolIEs.List); i++ {
		ie := uplinkNASTransport.ProtocolIEs.List[i]
		switch ie.Id.Value {
		case ngapType.ProtocolIEIDAMFUENGAPID:
			ieList.IEs = append(ieList.IEs, IE{
				ID:          protocolIEIDToString(ie.Id.Value),
				Criticality: criticalityToString(ie.Criticality.Value),
				AMFUENGAPID: &ie.Value.AMFUENGAPID.Value,
			})
		case ngapType.ProtocolIEIDRANUENGAPID:
			ieList.IEs = append(ieList.IEs, IE{
				ID:          protocolIEIDToString(ie.Id.Value),
				Criticality: criticalityToString(ie.Criticality.Value),
				RANUENGAPID: &ie.Value.RANUENGAPID.Value,
			})
		case ngapType.ProtocolIEIDNASPDU:
			ieList.IEs = append(ieList.IEs, IE{
				ID:          protocolIEIDToString(ie.Id.Value),
				Criticality: criticalityToString(ie.Criticality.Value),
				NASPDU:      ie.Value.NASPDU.Value,
			})
		case ngapType.ProtocolIEIDUserLocationInformation:
			ieList.IEs = append(ieList.IEs, IE{
				ID:                      protocolIEIDToString(ie.Id.Value),
				Criticality:             criticalityToString(ie.Criticality.Value),
				UserLocationInformation: buildUserLocationInformationIE(ie.Value.UserLocationInformation),
			})
		default:
			ieList.IEs = append(ieList.IEs, IE{
				ID:          protocolIEIDToString(ie.Id.Value),
				Criticality: criticalityToString(ie.Criticality.Value),
			})
			logger.EllaLog.Warn("Unsupported ie type", zap.Int64("type", ie.Id.Value))
		}
	}

	return ieList
}

func buildDownlinkNASTransport(downlinkNASTransport *ngapType.DownlinkNASTransport) *DownlinkNASTransport {
	if downlinkNASTransport == nil {
		return nil
	}

	ieList := &DownlinkNASTransport{}

	for i := 0; i < len(downlinkNASTransport.ProtocolIEs.List); i++ {
		ie := downlinkNASTransport.ProtocolIEs.List[i]
		switch ie.Id.Value {
		case ngapType.ProtocolIEIDAMFUENGAPID:
			ieList.IEs = append(ieList.IEs, IE{
				ID:          protocolIEIDToString(ie.Id.Value),
				Criticality: criticalityToString(ie.Criticality.Value),
				AMFUENGAPID: &ie.Value.AMFUENGAPID.Value,
			})
		case ngapType.ProtocolIEIDRANUENGAPID:
			ieList.IEs = append(ieList.IEs, IE{
				ID:          protocolIEIDToString(ie.Id.Value),
				Criticality: criticalityToString(ie.Criticality.Value),
				RANUENGAPID: &ie.Value.RANUENGAPID.Value,
			})
		case ngapType.ProtocolIEIDOldAMF:
			ieList.IEs = append(ieList.IEs, IE{
				ID:          protocolIEIDToString(ie.Id.Value),
				Criticality: criticalityToString(ie.Criticality.Value),
				OldAMF:      buildAMFNameIE(ie.Value.OldAMF),
			})
		case ngapType.ProtocolIEIDRANPagingPriority:
			ieList.IEs = append(ieList.IEs, IE{
				ID:                protocolIEIDToString(ie.Id.Value),
				Criticality:       criticalityToString(ie.Criticality.Value),
				RANPagingPriority: &ie.Value.RANPagingPriority.Value,
			})
		case ngapType.ProtocolIEIDNASPDU:
			ieList.IEs = append(ieList.IEs, IE{
				ID:          protocolIEIDToString(ie.Id.Value),
				Criticality: criticalityToString(ie.Criticality.Value),
				NASPDU:      ie.Value.NASPDU.Value,
			})
		case ngapType.ProtocolIEIDMobilityRestrictionList:
			ieList.IEs = append(ieList.IEs, IE{
				ID:                      protocolIEIDToString(ie.Id.Value),
				Criticality:             criticalityToString(ie.Criticality.Value),
				MobilityRestrictionList: buildMobilityRestrictionListIE(ie.Value.MobilityRestrictionList),
			})
		case ngapType.ProtocolIEIDIndexToRFSP:
			ieList.IEs = append(ieList.IEs, IE{
				ID:          protocolIEIDToString(ie.Id.Value),
				Criticality: criticalityToString(ie.Criticality.Value),
				IndexToRFSP: &ie.Value.IndexToRFSP.Value,
			})
		case ngapType.ProtocolIEIDUEAggregateMaximumBitRate:
			ieList.IEs = append(ieList.IEs, IE{
				ID:                        protocolIEIDToString(ie.Id.Value),
				Criticality:               criticalityToString(ie.Criticality.Value),
				UEAggregateMaximumBitRate: buildUEAggregateMaximumBitRateIE(ie.Value.UEAggregateMaximumBitRate),
			})
		case ngapType.ProtocolIEIDAllowedNSSAI:
			ieList.IEs = append(ieList.IEs, IE{
				ID:           protocolIEIDToString(ie.Id.Value),
				Criticality:  criticalityToString(ie.Criticality.Value),
				AllowedNSSAI: buildAllowedNSSAI(ie.Value.AllowedNSSAI),
			})
		default:
			ieList.IEs = append(ieList.IEs, IE{
				ID:          protocolIEIDToString(ie.Id.Value),
				Criticality: criticalityToString(ie.Criticality.Value),
			})
			logger.EllaLog.Warn("Unsupported ie type", zap.Int64("type", ie.Id.Value))
		}
	}
	return ieList
}

func buildUEAggregateMaximumBitRateIE(ueambr *ngapType.UEAggregateMaximumBitRate) *UEAggregateMaximumBitRate {
	if ueambr == nil {
		return nil
	}

	return &UEAggregateMaximumBitRate{
		UEAggregateMaximumBitRateDL: ueambr.UEAggregateMaximumBitRateDL.Value,
		UEAggregateMaximumBitRateUL: ueambr.UEAggregateMaximumBitRateUL.Value,
	}
}

func ratRestrictionInfoToString(ratType ngapType.RATRestrictionInformation) string {
	if bytes.Equal(ratType.Value.Bytes, []byte{0x40}) {
		return "NR"
	} else if bytes.Equal(ratType.Value.Bytes, []byte{0x80}) {
		return "EUTRA"
	} else {
		return fmt.Sprintf("Unknown (%v)", ratType.Value)
	}
}

func buildMobilityRestrictionListIE(mrl *ngapType.MobilityRestrictionList) *MobilityRestrictionList {
	if mrl == nil {
		return nil
	}

	mobilityRestrictionList := &MobilityRestrictionList{}

	mobilityRestrictionList.ServingPLMN = plmnIDToModels(mrl.ServingPLMN)

	if mrl.EquivalentPLMNs != nil {
		eqPlmns := make([]PLMNID, 0)
		for i := 0; i < len(mrl.EquivalentPLMNs.List); i++ {
			eqPlmns = append(eqPlmns, plmnIDToModels(mrl.EquivalentPLMNs.List[i]))
		}
		mobilityRestrictionList.EquivalentPLMNs = eqPlmns
	}

	if mrl.RATRestrictions != nil {
		ratRestrictions := make([]RATRestriction, 0)
		for i := 0; i < len(mrl.RATRestrictions.List); i++ {
			ratRestrictions = append(ratRestrictions, RATRestriction{
				PLMNID:                    plmnIDToModels(mrl.RATRestrictions.List[i].PLMNIdentity),
				RATRestrictionInformation: ratRestrictionInfoToString(mrl.RATRestrictions.List[i].RATRestrictionInformation),
			})
		}
		mobilityRestrictionList.RATRestrictions = ratRestrictions
	}

	if mrl.ForbiddenAreaInformation != nil {
		faiList := make([]ForbiddenAreaInformation, 0)
		for i := 0; i < len(mrl.ForbiddenAreaInformation.List); i++ {
			tacList := make([]string, 0)
			for j := 0; j < len(mrl.ForbiddenAreaInformation.List[i].ForbiddenTACs.List); j++ {
				tacList = append(tacList, hex.EncodeToString(mrl.ForbiddenAreaInformation.List[i].ForbiddenTACs.List[j].Value))
			}
			faiList = append(faiList, ForbiddenAreaInformation{
				PLMNID:        plmnIDToModels(mrl.ForbiddenAreaInformation.List[i].PLMNIdentity),
				ForbiddenTACs: tacList,
			})
		}
		mobilityRestrictionList.ForbiddenAreaInformation = faiList
	}

	if mrl.ServiceAreaInformation != nil {
		saiList := make([]ServiceAreaInformation, 0)
		for i := 0; i < len(mrl.ServiceAreaInformation.List); i++ {
			allowedTACs := make([]string, 0)
			for j := 0; j < len(mrl.ServiceAreaInformation.List[i].AllowedTACs.List); j++ {
				allowedTACs = append(allowedTACs, hex.EncodeToString(mrl.ServiceAreaInformation.List[i].AllowedTACs.List[j].Value))
			}
			notAllowedTACs := make([]string, 0)
			for j := 0; j < len(mrl.ServiceAreaInformation.List[i].NotAllowedTACs.List); j++ {
				notAllowedTACs = append(notAllowedTACs, hex.EncodeToString(mrl.ServiceAreaInformation.List[i].NotAllowedTACs.List[j].Value))
			}
			saiList = append(saiList, ServiceAreaInformation{
				PLMNID:         plmnIDToModels(mrl.ServiceAreaInformation.List[i].PLMNIdentity),
				AllowedTACs:    allowedTACs,
				NotAllowedTACs: notAllowedTACs,
			})
		}
		mobilityRestrictionList.ServiceAreaInformation = saiList
	}
	return mobilityRestrictionList
}

func buildInitialUEMessage(initialUEMessage *ngapType.InitialUEMessage) *InitialUEMessage {
	if initialUEMessage == nil {
		return nil
	}

	ieList := &InitialUEMessage{}

	for i := 0; i < len(initialUEMessage.ProtocolIEs.List); i++ {
		ie := initialUEMessage.ProtocolIEs.List[i]

		switch ie.Id.Value {
		case ngapType.ProtocolIEIDRANUENGAPID:
			ieList.IEs = append(ieList.IEs, IE{
				ID:          protocolIEIDToString(ie.Id.Value),
				Criticality: criticalityToString(ie.Criticality.Value),
				RANUENGAPID: &ie.Value.RANUENGAPID.Value,
			})
		case ngapType.ProtocolIEIDNASPDU:
			ieList.IEs = append(ieList.IEs, IE{
				ID:          protocolIEIDToString(ie.Id.Value),
				Criticality: criticalityToString(ie.Criticality.Value),
				NASPDU:      ie.Value.NASPDU.Value,
			})
		case ngapType.ProtocolIEIDUserLocationInformation:
			ieList.IEs = append(ieList.IEs, IE{
				ID:                      protocolIEIDToString(ie.Id.Value),
				Criticality:             criticalityToString(ie.Criticality.Value),
				UserLocationInformation: buildUserLocationInformationIE(ie.Value.UserLocationInformation),
			})
		case ngapType.ProtocolIEIDRRCEstablishmentCause:
			ieList.IEs = append(ieList.IEs, IE{
				ID:                    protocolIEIDToString(ie.Id.Value),
				Criticality:           criticalityToString(ie.Criticality.Value),
				RRCEstablishmentCause: buildRRCEstablishmentCauseIE(ie.Value.RRCEstablishmentCause),
			})
		case ngapType.ProtocolIEIDFiveGSTMSI:
			ieList.IEs = append(ieList.IEs, IE{
				ID:          protocolIEIDToString(ie.Id.Value),
				Criticality: criticalityToString(ie.Criticality.Value),
				FiveGSTMSI:  buildFiveGSTMSIIE(ie.Value.FiveGSTMSI),
			})
		case ngapType.ProtocolIEIDAMFSetID:
			amfSetID := bitStringToHex(&ie.Value.AMFSetID.Value)
			ieList.IEs = append(ieList.IEs, IE{
				ID:          protocolIEIDToString(ie.Id.Value),
				Criticality: criticalityToString(ie.Criticality.Value),
				AMFSetID:    &amfSetID,
			})
		case ngapType.ProtocolIEIDUEContextRequest:
			ieList.IEs = append(ieList.IEs, IE{
				ID:               protocolIEIDToString(ie.Id.Value),
				Criticality:      criticalityToString(ie.Criticality.Value),
				UEContextRequest: buildUEContextRequestIE(ie.Value.UEContextRequest),
			})
		case ngapType.ProtocolIEIDAllowedNSSAI:
			ieList.IEs = append(ieList.IEs, IE{
				ID:           protocolIEIDToString(ie.Id.Value),
				Criticality:  criticalityToString(ie.Criticality.Value),
				AllowedNSSAI: buildAllowedNSSAI(ie.Value.AllowedNSSAI),
			})
		default:
			ieList.IEs = append(ieList.IEs, IE{
				ID:          protocolIEIDToString(ie.Id.Value),
				Criticality: criticalityToString(ie.Criticality.Value),
			})
			logger.EllaLog.Warn("Unsupported ie type", zap.Int64("type", ie.Id.Value))
		}
	}

	return ieList
}

func buildAllowedNSSAI(allowedNSSAI *ngapType.AllowedNSSAI) []SNSSAI {
	if allowedNSSAI == nil {
		return nil
	}

	snssaiList := make([]SNSSAI, 0)

	for i := 0; i < len(allowedNSSAI.List); i++ {
		ngapSnssai := allowedNSSAI.List[i].SNSSAI
		snssai := buildSNSSAI(&ngapSnssai)
		snssaiList = append(snssaiList, *snssai)
	}

	return snssaiList
}

func buildSNSSAI(ngapSnssai *ngapType.SNSSAI) *SNSSAI {
	if ngapSnssai == nil {
		return nil
	}

	snssai := &SNSSAI{
		SST: int32(ngapSnssai.SST.Value[0]),
	}

	if ngapSnssai.SD != nil {
		sd := hex.EncodeToString(ngapSnssai.SD.Value)
		snssai.SD = &sd
	}

	return snssai
}

func buildUEContextRequestIE(ueCtxReq *ngapType.UEContextRequest) *string {
	if ueCtxReq == nil {
		return nil
	}

	var req string

	switch ueCtxReq.Value {
	case ngapType.UEContextRequestPresentRequested:
		req = "Requested"
	default:
		req = fmt.Sprintf("Unknown(%d)", ueCtxReq.Value)
	}

	return &req
}

func buildFiveGSTMSIIE(fivegStmsi *ngapType.FiveGSTMSI) *FiveGSTMSI {
	if fivegStmsi == nil {
		return nil
	}

	fiveg := &FiveGSTMSI{}

	fiveg.AMFSetID = bitStringToHex(&fivegStmsi.AMFSetID.Value)
	fiveg.AMFPointer = bitStringToHex(&fivegStmsi.AMFPointer.Value)
	fiveg.FiveGTMSI = hex.EncodeToString(fivegStmsi.FiveGTMSI.Value)

	return fiveg
}

func buildRRCEstablishmentCauseIE(rrc *ngapType.RRCEstablishmentCause) *string {
	if rrc == nil {
		return nil
	}

	var cause string

	switch rrc.Value {
	case ngapType.RRCEstablishmentCausePresentEmergency:
		cause = "Emergency"
	case ngapType.RRCEstablishmentCausePresentHighPriorityAccess:
		cause = "HighPriorityAccess"
	case ngapType.RRCEstablishmentCausePresentMtAccess:
		cause = "MtAccess"
	case ngapType.RRCEstablishmentCausePresentMoSignalling:
		cause = "MoSignalling"
	case ngapType.RRCEstablishmentCausePresentMoData:
		cause = "MoData"
	case ngapType.RRCEstablishmentCausePresentMoVoiceCall:
		cause = "MoVoiceCall"
	case ngapType.RRCEstablishmentCausePresentMoVideoCall:
		cause = "MoVideoCall"
	case ngapType.RRCEstablishmentCausePresentMoSMS:
		cause = "MoSMS"
	case ngapType.RRCEstablishmentCausePresentMpsPriorityAccess:
		cause = "MpsPriorityAccess"
	case ngapType.RRCEstablishmentCausePresentMcsPriorityAccess:
		cause = "McsPriorityAccess"
	case ngapType.RRCEstablishmentCausePresentNotAvailable:
		cause = "NotAvailable"
	default:
		cause = fmt.Sprintf("Unknown(%d)", rrc.Value)
	}

	return &cause
}

func buildUserLocationInformationIE(uli *ngapType.UserLocationInformation) *UserLocationInformation {
	if uli == nil {
		return nil
	}

	userLocationInfo := &UserLocationInformation{}

	switch uli.Present {
	case ngapType.UserLocationInformationPresentUserLocationInformationEUTRA:
		userLocationInfo.EUTRA = buildUserLocationInformationEUTRA(uli.UserLocationInformationEUTRA)
	case ngapType.UserLocationInformationPresentUserLocationInformationNR:
		userLocationInfo.NR = buildUserLocationInformationNR(uli.UserLocationInformationNR)
	case ngapType.UserLocationInformationPresentUserLocationInformationN3IWF:
		userLocationInfo.N3IWF = buildUserLocationInformationN3IWF(uli.UserLocationInformationN3IWF)
	default:
		logger.EllaLog.Warn("Unsupported UserLocationInformation type", zap.Int("present", uli.Present))
	}

	return userLocationInfo
}

func buildUserLocationInformationEUTRA(uliEUTRA *ngapType.UserLocationInformationEUTRA) *UserLocationInformationEUTRA {
	if uliEUTRA == nil {
		return nil
	}

	eutra := &UserLocationInformationEUTRA{}

	eutra.EUTRACGI = EUTRACGI{
		PLMNID:            plmnIDToModels(uliEUTRA.EUTRACGI.PLMNIdentity),
		EUTRACellIdentity: bitStringToHex(&uliEUTRA.EUTRACGI.EUTRACellIdentity.Value),
	}

	eutra.TAI = TAI{
		PLMNID: plmnIDToModels(uliEUTRA.TAI.PLMNIdentity),
		TAC:    hex.EncodeToString(uliEUTRA.TAI.TAC.Value),
	}

	if uliEUTRA.TimeStamp != nil {
		tsStr, err := timeStampToRFC3339(uliEUTRA.TimeStamp.Value)
		if err != nil {
			logger.EllaLog.Warn("failed to convert NGAP timestamp to RFC3339", zap.Error(err))
		} else {
			eutra.TimeStamp = &tsStr
		}
	}

	return eutra
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

func buildUserLocationInformationNR(uliNR *ngapType.UserLocationInformationNR) *UserLocationInformationNR {
	if uliNR == nil {
		return nil
	}

	nr := &UserLocationInformationNR{}

	nr.NRCGI = NRCGI{
		PLMNID:         plmnIDToModels(uliNR.NRCGI.PLMNIdentity),
		NRCellIdentity: bitStringToHex(&uliNR.NRCGI.NRCellIdentity.Value),
	}

	nr.TAI = TAI{
		PLMNID: plmnIDToModels(uliNR.TAI.PLMNIdentity),
		TAC:    hex.EncodeToString(uliNR.TAI.TAC.Value),
	}

	if uliNR.TimeStamp != nil {
		tsStr, err := timeStampToRFC3339(uliNR.TimeStamp.Value)
		if err != nil {
			logger.EllaLog.Warn("failed to convert NGAP timestamp to RFC3339", zap.Error(err))
		} else {
			nr.TimeStamp = &tsStr
		}
	}

	return nr
}

func buildUserLocationInformationN3IWF(uliN3IWF *ngapType.UserLocationInformationN3IWF) *UserLocationInformationN3IWF {
	if uliN3IWF == nil {
		return nil
	}

	n3iwf := &UserLocationInformationN3IWF{}

	ipv4Addr, ipv6Addr := ngapConvert.IPAddressToString(uliN3IWF.IPAddress)
	if ipv4Addr != "" {
		n3iwf.IPAddress = ipv4Addr
	} else {
		n3iwf.IPAddress = ipv6Addr
	}

	n3iwf.PortNumber = ngapConvert.PortNumberToInt(uliN3IWF.PortNumber)

	return n3iwf
}

func buildNGSetupRequest(ngSetupRequest *ngapType.NGSetupRequest) *NGSetupRequest {
	if ngSetupRequest == nil {
		return nil
	}

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
			ngSetup.IEs = append(ngSetup.IEs, IE{
				ID:          protocolIEIDToString(ie.Id.Value),
				Criticality: criticalityToString(ie.Criticality.Value),
			})
			logger.EllaLog.Warn("Unsupported ie type", zap.Int64("type", ie.Id.Value))
		}
	}

	return ngSetup
}

func buildNGSetupResponse(ngSetupResponse *ngapType.NGSetupResponse) *NGSetupResponse {
	if ngSetupResponse == nil {
		return nil
	}

	ngSetup := &NGSetupResponse{}

	for i := 0; i < len(ngSetupResponse.ProtocolIEs.List); i++ {
		ie := ngSetupResponse.ProtocolIEs.List[i]

		switch ie.Id.Value {
		case ngapType.ProtocolIEIDAMFName:
			ngSetup.IEs = append(ngSetup.IEs, IE{
				ID:          protocolIEIDToString(ie.Id.Value),
				Criticality: criticalityToString(ie.Criticality.Value),
				AMFName:     buildAMFNameIE(ie.Value.AMFName),
			})
		case ngapType.ProtocolIEIDServedGUAMIList:
			ngSetup.IEs = append(ngSetup.IEs, IE{
				ID:              protocolIEIDToString(ie.Id.Value),
				Criticality:     criticalityToString(ie.Criticality.Value),
				ServedGUAMIList: buildServedGUAMIListIE(ie.Value.ServedGUAMIList),
			})
		case ngapType.ProtocolIEIDRelativeAMFCapacity:
			ngSetup.IEs = append(ngSetup.IEs, IE{
				ID:                  protocolIEIDToString(ie.Id.Value),
				Criticality:         criticalityToString(ie.Criticality.Value),
				RelativeAMFCapacity: &ie.Value.RelativeAMFCapacity.Value,
			})
		case ngapType.ProtocolIEIDPLMNSupportList:
			ngSetup.IEs = append(ngSetup.IEs, IE{
				ID:              protocolIEIDToString(ie.Id.Value),
				Criticality:     criticalityToString(ie.Criticality.Value),
				PLMNSupportList: buildPLMNSupportListIE(ie.Value.PLMNSupportList),
			})
		case ngapType.ProtocolIEIDCriticalityDiagnostics:
			ngSetup.IEs = append(ngSetup.IEs, IE{
				ID:                     protocolIEIDToString(ie.Id.Value),
				Criticality:            criticalityToString(ie.Criticality.Value),
				CriticalityDiagnostics: buildCriticalityDiagnosticsIE(ie.Value.CriticalityDiagnostics),
			})
		case ngapType.ProtocolIEIDUERetentionInformation:
			ngSetup.IEs = append(ngSetup.IEs, IE{
				ID:                     protocolIEIDToString(ie.Id.Value),
				Criticality:            criticalityToString(ie.Criticality.Value),
				UERetentionInformation: buildUERetentionInformationIE(ie.Value.UERetentionInformation),
			})
		default:
			ngSetup.IEs = append(ngSetup.IEs, IE{
				ID:          protocolIEIDToString(ie.Id.Value),
				Criticality: criticalityToString(ie.Criticality.Value),
			})
			logger.EllaLog.Warn("Unsupported ie type", zap.Int64("type", ie.Id.Value))
		}
	}

	return ngSetup
}

func buildNGSetupFailure(ngSetupFailure *ngapType.NGSetupFailure) *NGSetupFailure {
	if ngSetupFailure == nil {
		return nil
	}

	ngFail := &NGSetupFailure{}

	for i := 0; i < len(ngSetupFailure.ProtocolIEs.List); i++ {
		ie := ngSetupFailure.ProtocolIEs.List[i]

		switch ie.Id.Value {
		case ngapType.ProtocolIEIDCause:
			ngFail.IEs = append(ngFail.IEs, IE{
				ID:          protocolIEIDToString(ie.Id.Value),
				Criticality: criticalityToString(ie.Criticality.Value),
				Cause:       strPtr(causeToString(ie.Value.Cause)),
			})
		case ngapType.ProtocolIEIDTimeToWait:
			ngFail.IEs = append(ngFail.IEs, IE{
				ID:          protocolIEIDToString(ie.Id.Value),
				Criticality: criticalityToString(ie.Criticality.Value),
				TimeToWait:  buildTimeToWaitIE(ie.Value.TimeToWait),
			})
		case ngapType.ProtocolIEIDCriticalityDiagnostics:
			ngFail.IEs = append(ngFail.IEs, IE{
				ID:                     protocolIEIDToString(ie.Id.Value),
				Criticality:            criticalityToString(ie.Criticality.Value),
				CriticalityDiagnostics: buildCriticalityDiagnosticsIE(ie.Value.CriticalityDiagnostics),
			})
		default:
			ngFail.IEs = append(ngFail.IEs, IE{
				ID:          protocolIEIDToString(ie.Id.Value),
				Criticality: criticalityToString(ie.Criticality.Value),
			})
			logger.EllaLog.Warn("Unsupported ie type", zap.Int64("type", ie.Id.Value))
		}
	}

	return ngFail
}

func buildTimeToWaitIE(timeToWait *ngapType.TimeToWait) *string {
	if timeToWait == nil {
		return nil
	}

	var str string

	switch timeToWait.Value {
	case ngapType.TimeToWaitPresentV1s:
		str = "V1s (0)"
	case ngapType.TimeToWaitPresentV2s:
		str = "V2s (1)"
	case ngapType.TimeToWaitPresentV5s:
		str = "V5s (2)"
	case ngapType.TimeToWaitPresentV10s:
		str = "V10s (3)"
	case ngapType.TimeToWaitPresentV20s:
		str = "V20s (4)"
	case ngapType.TimeToWaitPresentV60s:
		str = "V60s (5)"
	default:
		str = fmt.Sprintf("Unknown (%d)", timeToWait.Value)
	}

	return &str
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

func buildAMFNameIE(an *ngapType.AMFName) *string {
	if an == nil || an.Value == "" {
		return nil
	}

	s := an.Value

	return &s
}

func buildServedGUAMIListIE(sgl *ngapType.ServedGUAMIList) []Guami {
	if sgl == nil {
		return nil
	}

	guamiList := make([]Guami, len(sgl.List))
	for i := 0; i < len(sgl.List); i++ {
		guamiList[i] = *buildGUAMI(&sgl.List[i].GUAMI)
	}

	return guamiList
}

func buildGUAMI(guami *ngapType.GUAMI) *Guami {
	if guami == nil {
		return nil
	}

	amfID := ngapConvert.AmfIdToModels(guami.AMFRegionID.Value, guami.AMFSetID.Value, guami.AMFPointer.Value)
	return &Guami{
		PLMNID: plmnIDToModels(guami.PLMNIdentity),
		AMFID:  amfID,
	}
}

func buildPLMNSupportListIE(psl *ngapType.PLMNSupportList) []PLMN {
	if psl == nil {
		return nil
	}

	plmnList := make([]PLMN, len(psl.List))
	for i := 0; i < len(psl.List); i++ {
		plmnList[i] = PLMN{
			PLMNID:           plmnIDToModels(psl.List[i].PLMNIdentity),
			SliceSupportList: buildSNSSAIList(psl.List[i].SliceSupportList),
		}
	}

	return plmnList
}

func buildCriticalityDiagnosticsIE(cd *ngapType.CriticalityDiagnostics) *CriticalityDiagnostics {
	if cd == nil {
		return nil
	}

	critDiag := &CriticalityDiagnostics{}

	if cd.ProcedureCode != nil {
		procCode := procedureCodeToString(cd.ProcedureCode.Value)
		critDiag.ProcedureCode = &procCode
	}

	if cd.TriggeringMessage != nil {
		trigMsg := triggeringMessageToString(cd.TriggeringMessage.Value)
		critDiag.TriggeringMessage = &trigMsg
	}

	if cd.ProcedureCriticality != nil {
		procCrit := criticalityToString(cd.ProcedureCriticality.Value)
		critDiag.ProcedureCriticality = &procCrit
	}

	if cd.IEsCriticalityDiagnostics != nil {
		critDiag.IEsCriticalityDiagnostics = buildIEsCriticalityDiagnisticsList(cd.IEsCriticalityDiagnostics)
	}

	return critDiag
}

func buildIEsCriticalityDiagnisticsList(ieList *ngapType.CriticalityDiagnosticsIEList) []IEsCriticalityDiagnostics {
	if ieList == nil {
		return nil
	}

	ies := make([]IEsCriticalityDiagnostics, len(ieList.List))
	for i := 0; i < len(ieList.List); i++ {
		ie := ieList.List[i]
		ies[i] = IEsCriticalityDiagnostics{
			IECriticality: criticalityToString(ie.IECriticality.Value),
			IEID:          protocolIEIDToString(ie.IEID.Value),
			TypeOfError:   typeOfErrorToString(ie.TypeOfError.Value),
		}
	}

	return ies
}

func typeOfErrorToString(toe aper.Enumerated) string {
	switch toe {
	case ngapType.TypeOfErrorPresentNotUnderstood:
		return "NotUnderstood (0)"
	case ngapType.TypeOfErrorPresentMissing:
		return "Missing (1)"
	default:
		return fmt.Sprintf("Unknown (%d)", toe)
	}
}

func triggeringMessageToString(tm aper.Enumerated) string {
	switch tm {
	case ngapType.TriggeringMessagePresentInitiatingMessage:
		return "InitiatingMessage (0)"
	case ngapType.TriggeringMessagePresentSuccessfulOutcome:
		return "SuccessfulOutcome (1)"
	case ngapType.TriggeringMessagePresentUnsuccessfullOutcome:
		return "UnsuccessfulOutcome (2)"
	default:
		return fmt.Sprintf("Unknown (%d)", tm)
	}
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
			PLMNID:           plmnIDToModels(bpl.List[i].PLMNIdentity),
			SliceSupportList: buildSNSSAIList(bpl.List[i].TAISliceSupportList),
		}
	}

	return plmns
}

func buildSNSSAIList(sssl ngapType.SliceSupportList) []SNSSAI {
	snssais := make([]SNSSAI, len(sssl.List))
	for i := 0; i < len(sssl.List); i++ {
		snssai := buildSNSSAI(&sssl.List[i].SNSSAI)
		snssais[i] = *snssai
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

func bitStringToHex(bitString *aper.BitString) string {
	hexString := hex.EncodeToString(bitString.Bytes)
	hexLen := (bitString.BitLength + 3) / 4
	hexString = hexString[:hexLen]
	return hexString
}
