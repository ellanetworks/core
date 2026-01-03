package ngap

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"

	"github.com/ellanetworks/core/internal/decoder/nas"
	"github.com/ellanetworks/core/internal/decoder/utils"
	"github.com/free5gc/aper"
	"github.com/free5gc/ngap/ngapType"
)

type ExpectedUEActivityBehaviour struct {
	ExpectedActivityPeriod                 *int64                   `json:"expected_activity_period,omitempty"`
	ExpectedIdlePeriod                     *int64                   `json:"expected_idle_period,omitempty"`
	SourceOfUEActivityBehaviourInformation *utils.EnumField[uint64] `json:"source_of_ue_activity_behaviour_information,omitempty"`
}

type NGRANCGI struct {
	NRCGI    *NRCGI    `json:"nr_ran_cgi,omitempty"`
	EUTRACGI *EUTRACGI `json:"eutra_cgi,omitempty"`

	Error string `json:"error,omitempty"` // Reserved field for decoding errors
}

type ExpectedUEMovingTrajectoryItem struct {
	NGRANCGI         NGRANCGI `json:"ng_ran_cgi"`
	TimeStayedInCell *int64   `json:"time_stayed_in_cell,omitempty"`
}

type ExpectedUEBehaviour struct {
	ExpectedUEActivityBehaviour *ExpectedUEActivityBehaviour     `json:"expected_ue_activity_behaviour,omitempty"`
	ExpectedHOInterval          *utils.EnumField[uint64]         `json:"expected_ho_interval,omitempty"`
	ExpectedUEMobility          *utils.EnumField[uint64]         `json:"expected_ue_mobility,omitempty"`
	ExpectedUEMovingTrajectory  []ExpectedUEMovingTrajectoryItem `json:"expected_ue_moving_trajectory,omitempty"`
}

type CoreNetworkAssistanceInformation struct {
	UEIdentityIndexValue            string                   `json:"ue_identity_index_value"`
	UESpecificDRX                   *utils.EnumField[uint64] `json:"ue_specific_drx,omitempty"`
	PeriodicRegistrationUpdateTimer string                   `json:"periodic_registration_update_timer"`
	MICOModeIndication              *string                  `json:"mico_mode_indication,omitempty"`
	TAIListForInactive              []TAI                    `json:"tai_list_for_inactive,omitempty"`
	ExpectedUEBehaviour             *ExpectedUEBehaviour     `json:"expected_ue_behaviour,omitempty"`
}

type MaximumBitRate struct {
	DownlinkNAggregateMaximumBitRate uint64 `json:"downlink_n_aggregate_maximum_bit_rate"`
	UplinkNAggregateMaximumBitRate   uint64 `json:"uplink_n_aggregate_maximum_bit_rate"`
}

type GTPTunnel struct {
	GTPTEID               uint32 `json:"gtp_teid"`
	TransportLayerAddress string `json:"transport_layer_address"`
}

type ULNGUUPTNLInformation struct {
	GTPTunnel GTPTunnel `json:"gtp_tunnel"`
}

type QosFlowSetupRequest struct {
	QosId  int64 `json:"qos_id"`
	FiveQi int64 `json:"five_qi"`
	PriArp int64 `json:"pri_arp"`
}

type PDUSessionResourceSetupRequestTransfer struct {
	ULNGUUPTNLInformation   *ULNGUUPTNLInformation  `json:"ul_ng_u_up_tnl_information,omitempty"`
	QosFlowSetupRequestList []QosFlowSetupRequest   `json:"qos_flow_setup_request_list,omitempty"`
	PduSType                *utils.EnumField[int64] `json:"pdu_s_type,omitempty"`
	MaximumBitRate          *MaximumBitRate         `json:"maximum_bit_rate,omitempty"`
	SecurityIndication      *UnsupportedIE          `json:"security_indication,omitempty"`
}

type PDUSessionResourceSetupCxtReq struct {
	PDUSessionID                           int64                                  `json:"pdu_session_id"`
	NASPDU                                 *NASPDU                                `json:"nas_pdu,omitempty"`
	SNSSAI                                 SNSSAI                                 `json:"snssai"`
	PDUSessionResourceSetupRequestTransfer PDUSessionResourceSetupRequestTransfer `json:"pdu_session_resource_setup_request_transfer"`
}

type UESecurityCapabilities struct {
	NRencryptionAlgorithms             []string `json:"nr_encryption_algorithms"`
	NRintegrityProtectionAlgorithms    []string `json:"nr_integrity_protection_algorithms"`
	EUTRAencryptionAlgorithms          string   `json:"eutra_encryption_algorithms"`
	EUTRAintegrityProtectionAlgorithms string   `json:"eutra_integrity_protection_algorithms"`
}

func buildInitialContextSetupRequest(initialContextSetupRequest ngapType.InitialContextSetupRequest) NGAPMessageValue {
	ies := make([]IE, 0)

	for i := 0; i < len(initialContextSetupRequest.ProtocolIEs.List); i++ {
		ie := initialContextSetupRequest.ProtocolIEs.List[i]
		switch ie.Id.Value {
		case ngapType.ProtocolIEIDAMFUENGAPID:
			ies = append(ies, IE{
				ID:          protocolIEIDToEnum(ie.Id.Value),
				Criticality: criticalityToEnum(ie.Criticality.Value),
				Value:       ie.Value.AMFUENGAPID.Value,
			})
		case ngapType.ProtocolIEIDRANUENGAPID:
			ies = append(ies, IE{
				ID:          protocolIEIDToEnum(ie.Id.Value),
				Criticality: criticalityToEnum(ie.Criticality.Value),
				Value:       ie.Value.RANUENGAPID.Value,
			})
		case ngapType.ProtocolIEIDOldAMF:
			ies = append(ies, IE{
				ID:          protocolIEIDToEnum(ie.Id.Value),
				Criticality: criticalityToEnum(ie.Criticality.Value),
				Value:       ie.Value.OldAMF.Value,
			})
		case ngapType.ProtocolIEIDUEAggregateMaximumBitRate:
			ies = append(ies, IE{
				ID:          protocolIEIDToEnum(ie.Id.Value),
				Criticality: criticalityToEnum(ie.Criticality.Value),
				Value:       buildUEAggregateMaximumBitRateIE(*ie.Value.UEAggregateMaximumBitRate),
			})
		case ngapType.ProtocolIEIDCoreNetworkAssistanceInformation:
			value, err := buildCoreNetworkAssistanceInformation(*ie.Value.CoreNetworkAssistanceInformation)

			ieErr := ""
			if err != nil {
				ieErr = fmt.Sprintf("failed to build CoreNetworkAssistanceInformation: %v", err)
			}

			ies = append(ies, IE{
				ID:          protocolIEIDToEnum(ie.Id.Value),
				Criticality: criticalityToEnum(ie.Criticality.Value),
				Value:       value,
				Error:       ieErr,
			})
		case ngapType.ProtocolIEIDGUAMI:
			ies = append(ies, IE{
				ID:          protocolIEIDToEnum(ie.Id.Value),
				Criticality: criticalityToEnum(ie.Criticality.Value),
				Value:       buildGUAMI(*ie.Value.GUAMI),
			})
		case ngapType.ProtocolIEIDPDUSessionResourceSetupListCxtReq:
			ies = append(ies, IE{
				ID:          protocolIEIDToEnum(ie.Id.Value),
				Criticality: criticalityToEnum(ie.Criticality.Value),
				Value:       buildPDUSessionResourceSetupListCxtReq(*ie.Value.PDUSessionResourceSetupListCxtReq),
			})
		case ngapType.ProtocolIEIDAllowedNSSAI:
			ies = append(ies, IE{
				ID:          protocolIEIDToEnum(ie.Id.Value),
				Criticality: criticalityToEnum(ie.Criticality.Value),
				Value:       buildAllowedNSSAI(*ie.Value.AllowedNSSAI),
			})
		case ngapType.ProtocolIEIDUESecurityCapabilities:
			ies = append(ies, IE{
				ID:          protocolIEIDToEnum(ie.Id.Value),
				Criticality: criticalityToEnum(ie.Criticality.Value),
				Value:       buildUESecurityCapabilities(*ie.Value.UESecurityCapabilities),
			})
		case ngapType.ProtocolIEIDSecurityKey:
			ies = append(ies, IE{
				ID:          protocolIEIDToEnum(ie.Id.Value),
				Criticality: criticalityToEnum(ie.Criticality.Value),
				Value:       bitStringToHex(&ie.Value.SecurityKey.Value),
			})
		case ngapType.ProtocolIEIDMobilityRestrictionList:
			ies = append(ies, IE{
				ID:          protocolIEIDToEnum(ie.Id.Value),
				Criticality: criticalityToEnum(ie.Criticality.Value),
				Value:       buildMobilityRestrictionListIE(*ie.Value.MobilityRestrictionList),
			})
		case ngapType.ProtocolIEIDIndexToRFSP:
			ies = append(ies, IE{
				ID:          protocolIEIDToEnum(ie.Id.Value),
				Criticality: criticalityToEnum(ie.Criticality.Value),
				Value:       ie.Value.IndexToRFSP.Value,
			})
		case ngapType.ProtocolIEIDNASPDU:
			ies = append(ies, IE{
				ID:          protocolIEIDToEnum(ie.Id.Value),
				Criticality: criticalityToEnum(ie.Criticality.Value),
				Value: NASPDU{
					Raw:     ie.Value.NASPDU.Value,
					Decoded: nas.DecodeNASMessage(ie.Value.NASPDU.Value),
				},
			})
		case ngapType.ProtocolIEIDUERadioCapability:
			ies = append(ies, IE{
				ID:          protocolIEIDToEnum(ie.Id.Value),
				Criticality: criticalityToEnum(ie.Criticality.Value),
				Value:       []byte(ie.Value.UERadioCapability.Value),
			})
		default:
			ies = append(ies, IE{
				ID:          protocolIEIDToEnum(ie.Id.Value),
				Criticality: criticalityToEnum(ie.Criticality.Value),
				Error:       fmt.Sprintf("unsupported ie type %d", ie.Id.Value),
			})
		}
	}

	return NGAPMessageValue{
		IEs: ies,
	}
}

func buildPDUSessionResourceSetupListCxtReq(pduSessionResourceSetupListCxtReq ngapType.PDUSessionResourceSetupListCxtReq) []PDUSessionResourceSetupCxtReq {
	var pduSessionResourceSetupList []PDUSessionResourceSetupCxtReq

	for i := 0; i < len(pduSessionResourceSetupListCxtReq.List); i++ {
		item := pduSessionResourceSetupListCxtReq.List[i]

		setupRequestTransfer, err := buildPDUSessionInfoFromSetupRequestTransfer(item.PDUSessionResourceSetupRequestTransfer)
		if err != nil {
			continue
		}

		pduSessionResourceSetupList = append(pduSessionResourceSetupList, PDUSessionResourceSetupCxtReq{
			PDUSessionID:                           item.PDUSessionID.Value,
			SNSSAI:                                 *buildSNSSAI(&item.SNSSAI),
			PDUSessionResourceSetupRequestTransfer: *setupRequestTransfer,
		})

		if item.NASPDU != nil {
			pduSessionResourceSetupList[i].NASPDU = &NASPDU{
				Raw:     item.NASPDU.Value,
				Decoded: nas.DecodeNASMessage(item.NASPDU.Value),
			}
		}
	}

	return pduSessionResourceSetupList
}

func buildUESecurityCapabilities(uesec ngapType.UESecurityCapabilities) UESecurityCapabilities {
	return UESecurityCapabilities{
		NRencryptionAlgorithms:             decodeNRencryptionAlgorithms(uesec.NRencryptionAlgorithms.Value),
		NRintegrityProtectionAlgorithms:    decodeNRintegrityAlgorithms(uesec.NRintegrityProtectionAlgorithms.Value),
		EUTRAencryptionAlgorithms:          bitStringToHex(&uesec.EUTRAencryptionAlgorithms.Value),
		EUTRAintegrityProtectionAlgorithms: bitStringToHex(&uesec.EUTRAintegrityProtectionAlgorithms.Value),
	}
}

func decodeNRintegrityAlgorithms(bs aper.BitString) []string {
	if bs.Bytes == nil {
		return nil
	}

	// Ensure we can safely read bs.Bytes[0]
	if bs.BitLength < 8 {
		for bs.BitLength < 8 {
			bs.Bytes = append([]byte{0}, bs.Bytes...)
			bs.BitLength += 8
		}
	}

	var algos []string

	b := bs.Bytes[0]

	if (b>>7)&1 == 1 {
		algos = append(algos, "NIA1")
	}

	if (b>>6)&1 == 1 {
		algos = append(algos, "NIA2")
	}

	if (b>>5)&1 == 1 {
		algos = append(algos, "NIA3")
	}

	if len(algos) == 0 {
		return []string{"None or NIA0 (null integrity)"}
	}

	return algos
}

func decodeNRencryptionAlgorithms(bs aper.BitString) []string {
	if bs.Bytes == nil {
		return nil
	}

	if bs.BitLength < 8 {
		for bs.BitLength < 8 {
			bs.Bytes = append([]byte{0}, bs.Bytes...)
			bs.BitLength += 8
		}
	}

	var algos []string

	b := bs.Bytes[0]

	if (b>>7)&1 == 1 {
		algos = append(algos, "NEA1")
	}

	if (b>>6)&1 == 1 {
		algos = append(algos, "NEA2")
	}

	if (b>>5)&1 == 1 {
		algos = append(algos, "NEA3")
	}

	if len(algos) == 0 {
		return []string{"None or NEA0 (null ciphering)"}
	}

	return algos
}

func buildCoreNetworkAssistanceInformation(cnai ngapType.CoreNetworkAssistanceInformation) (CoreNetworkAssistanceInformation, error) {
	returnedCNAI := CoreNetworkAssistanceInformation{}

	switch cnai.UEIdentityIndexValue.Present {
	case ngapType.UEIdentityIndexValuePresentIndexLength10:
		returnedCNAI.UEIdentityIndexValue = bitStringToHex(cnai.UEIdentityIndexValue.IndexLength10)
	default:
		return returnedCNAI, fmt.Errorf("unsupported UEIdentityIndexValue present: %d", cnai.UEIdentityIndexValue.Present)
	}

	if cnai.UESpecificDRX != nil {
		pagingDRX := buildDefaultPagingDRXIE(*cnai.UESpecificDRX)
		returnedCNAI.UESpecificDRX = &pagingDRX
	}

	returnedCNAI.PeriodicRegistrationUpdateTimer = bitStringToHex(&cnai.PeriodicRegistrationUpdateTimer.Value)

	if cnai.MICOModeIndication != nil {
		switch cnai.MICOModeIndication.Value {
		case ngapType.MICOModeIndicationPresentTrue:
			returnedCNAI.MICOModeIndication = new(string)
			*returnedCNAI.MICOModeIndication = "true"
		default:
			return returnedCNAI, fmt.Errorf("unsupported MICOModeIndication present: %d", cnai.MICOModeIndication.Value)
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
		expectedUEBehaviour := buildExpectedUEBehaviour(*cnai.ExpectedUEBehaviour)
		returnedCNAI.ExpectedUEBehaviour = &expectedUEBehaviour
	}

	return returnedCNAI, nil
}

func buildExpectedUEBehaviour(eub ngapType.ExpectedUEBehaviour) ExpectedUEBehaviour {
	returnedEUB := ExpectedUEBehaviour{}

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
				*returnedEUB.ExpectedUEActivityBehaviour.SourceOfUEActivityBehaviourInformation = utils.MakeEnum(uint64(eub.ExpectedUEActivityBehaviour.SourceOfUEActivityBehaviourInformation.Value), "subscription_information", false)
			case ngapType.SourceOfUEActivityBehaviourInformationPresentStatistics:
				*returnedEUB.ExpectedUEActivityBehaviour.SourceOfUEActivityBehaviourInformation = utils.MakeEnum(uint64(eub.ExpectedUEActivityBehaviour.SourceOfUEActivityBehaviourInformation.Value), "statistics", false)
			default:
				*returnedEUB.ExpectedUEActivityBehaviour.SourceOfUEActivityBehaviourInformation = utils.MakeEnum(uint64(eub.ExpectedUEActivityBehaviour.SourceOfUEActivityBehaviourInformation.Value), "", true)
			}
		}
	}

	if eub.ExpectedHOInterval != nil {
		switch eub.ExpectedHOInterval.Value {
		case ngapType.ExpectedHOIntervalPresentSec15:
			*returnedEUB.ExpectedHOInterval = utils.MakeEnum(uint64(eub.ExpectedHOInterval.Value), "sec15", false)
		case ngapType.ExpectedHOIntervalPresentSec30:
			*returnedEUB.ExpectedHOInterval = utils.MakeEnum(uint64(eub.ExpectedHOInterval.Value), "sec30", false)
		case ngapType.ExpectedHOIntervalPresentSec60:
			*returnedEUB.ExpectedHOInterval = utils.MakeEnum(uint64(eub.ExpectedHOInterval.Value), "sec60", false)
		case ngapType.ExpectedHOIntervalPresentSec120:
			*returnedEUB.ExpectedHOInterval = utils.MakeEnum(uint64(eub.ExpectedHOInterval.Value), "sec120", false)
		case ngapType.ExpectedHOIntervalPresentSec180:
			*returnedEUB.ExpectedHOInterval = utils.MakeEnum(uint64(eub.ExpectedHOInterval.Value), "sec180", false)
		case ngapType.ExpectedHOIntervalPresentLongTime:
			*returnedEUB.ExpectedHOInterval = utils.MakeEnum(uint64(eub.ExpectedHOInterval.Value), "long_time", false)
		default:
			*returnedEUB.ExpectedHOInterval = utils.MakeEnum(uint64(eub.ExpectedHOInterval.Value), "", true)
		}
	}

	if eub.ExpectedUEMobility != nil {
		switch eub.ExpectedUEMobility.Value {
		case ngapType.ExpectedUEMobilityPresentStationary:
			*returnedEUB.ExpectedUEMobility = utils.MakeEnum(uint64(eub.ExpectedUEMobility.Value), "stationary", false)
		case ngapType.ExpectedUEMobilityPresentMobile:
			*returnedEUB.ExpectedUEMobility = utils.MakeEnum(uint64(eub.ExpectedUEMobility.Value), "mobile", false)
		default:
			*returnedEUB.ExpectedUEMobility = utils.MakeEnum(uint64(eub.ExpectedUEMobility.Value), "", true)
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
		ngRANCGI.Error = fmt.Sprintf("unsupported NGRANCGI present: %d", ngRanCgi.Present)
	}

	return ngRANCGI
}

func buildPDUSessionInfoFromSetupRequestTransfer(transfer aper.OctetString) (*PDUSessionResourceSetupRequestTransfer, error) {
	if transfer == nil {
		return nil, fmt.Errorf("PDU Session Resource Setup Request Transfer is missing")
	}

	pdu := &ngapType.PDUSessionResourceSetupRequestTransfer{}

	err := aper.UnmarshalWithParams(transfer, pdu, "valueExt")
	if err != nil {
		return nil, fmt.Errorf("could not unmarshal Pdu Session Resource Setup Request Transfer: %v", err)
	}

	pduTransfer := &PDUSessionResourceSetupRequestTransfer{}

	for _, ies := range pdu.ProtocolIEs.List {
		switch ies.Id.Value {
		case ngapType.ProtocolIEIDULNGUUPTNLInformation:
			ulTeid := binary.BigEndian.Uint32(ies.Value.ULNGUUPTNLInformation.GTPTunnel.GTPTEID.Value)
			upfAddress := ies.Value.ULNGUUPTNLInformation.GTPTunnel.TransportLayerAddress.Value.Bytes

			upfIp := fmt.Sprintf("%d.%d.%d.%d", upfAddress[0], upfAddress[1], upfAddress[2], upfAddress[3])

			pduTransfer.ULNGUUPTNLInformation = &ULNGUUPTNLInformation{
				GTPTunnel: GTPTunnel{
					GTPTEID:               ulTeid,
					TransportLayerAddress: upfIp,
				},
			}

		case ngapType.ProtocolIEIDQosFlowSetupRequestList:
			qosFlowList := []QosFlowSetupRequest{}

			for _, itemsQos := range ies.Value.QosFlowSetupRequestList.List {
				qosId := itemsQos.QosFlowIdentifier.Value
				fiveQi := itemsQos.QosFlowLevelQosParameters.QosCharacteristics.NonDynamic5QI.FiveQI.Value
				priArp := itemsQos.QosFlowLevelQosParameters.AllocationAndRetentionPriority.PriorityLevelARP.Value

				qosFlowList = append(qosFlowList, QosFlowSetupRequest{
					QosId:  qosId,
					FiveQi: fiveQi,
					PriArp: priArp,
				})
			}

			pduTransfer.QosFlowSetupRequestList = qosFlowList

		case ngapType.ProtocolIEIDPDUSessionAggregateMaximumBitRate:
			maxBitRateUL := uint64(ies.Value.PDUSessionAggregateMaximumBitRate.PDUSessionAggregateMaximumBitRateUL.Value)
			maxBitRateDL := uint64(ies.Value.PDUSessionAggregateMaximumBitRate.PDUSessionAggregateMaximumBitRateDL.Value)

			pduTransfer.MaximumBitRate = &MaximumBitRate{
				UplinkNAggregateMaximumBitRate:   maxBitRateUL,
				DownlinkNAggregateMaximumBitRate: maxBitRateDL,
			}
		case ngapType.ProtocolIEIDPDUSessionType:
			enum := pduSessionTypeToEnum(ies.Value.PDUSessionType.Value)
			pduTransfer.PduSType = &enum

		case ngapType.ProtocolIEIDSecurityIndication:
			securityIndication := makeUnsupportedIE()
			pduTransfer.SecurityIndication = securityIndication
		}
	}

	return pduTransfer, nil
}

func pduSessionTypeToEnum(pduType aper.Enumerated) utils.EnumField[int64] {
	switch pduType {
	case ngapType.PDUSessionTypePresentIpv4:
		return utils.MakeEnum(int64(pduType), "ipv4", false)
	case ngapType.PDUSessionTypePresentIpv6:
		return utils.MakeEnum(int64(pduType), "ipv6", false)
	case ngapType.PDUSessionTypePresentIpv4v6:
		return utils.MakeEnum(int64(pduType), "ipv4v6", false)
	case ngapType.PDUSessionTypePresentEthernet:
		return utils.MakeEnum(int64(pduType), "ethernet", false)
	case ngapType.PDUSessionTypePresentUnstructured:
		return utils.MakeEnum(int64(pduType), "unstructured", false)
	default:
		return utils.MakeEnum(int64(pduType), "", true)
	}
}
