package ngap

import (
	"encoding/hex"
	"fmt"

	"github.com/ellanetworks/core/internal/decoder/nas"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/omec-project/ngap/aper"
	"github.com/omec-project/ngap/ngapType"
	"go.uber.org/zap"
)

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
	UESpecificDRX                   *EnumField           `json:"ue_specific_drx,omitempty"`
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

type UESecurityCapabilities struct {
	NRencryptionAlgorithms             []string `json:"nr_encryption_algorithms"`
	NRintegrityProtectionAlgorithms    []string `json:"nr_integrity_protection_algorithms"`
	EUTRAencryptionAlgorithms          string   `json:"eutra_encryption_algorithms"`
	EUTRAintegrityProtectionAlgorithms string   `json:"eutra_integrity_protection_algorithms"`
}

func buildInitialContextSetupRequest(initialContextSetupRequest ngapType.InitialContextSetupRequest) NGAPMessageValue {
	ies := make([]IE, 0)

	AMFUENGAPID := int64(0)

	for i := 0; i < len(initialContextSetupRequest.ProtocolIEs.List); i++ {
		ie := initialContextSetupRequest.ProtocolIEs.List[i]
		switch ie.Id.Value {
		case ngapType.ProtocolIEIDAMFUENGAPID:
			AMFUENGAPID = ie.Value.AMFUENGAPID.Value
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
			ies = append(ies, IE{
				ID:          protocolIEIDToEnum(ie.Id.Value),
				Criticality: criticalityToEnum(ie.Criticality.Value),
				Value:       buildCoreNetworkAssistanceInformation(*ie.Value.CoreNetworkAssistanceInformation),
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
			nasContextInfo := &nas.NasContextInfo{
				Direction:   nas.DirUplink,
				AMFUENGAPID: AMFUENGAPID,
			}
			decodednNasPdu, err := nas.DecodeNASMessage(ie.Value.NASPDU.Value, nasContextInfo)
			if err != nil {
				logger.EllaLog.Warn("Failed to decode NAS PDU", zap.Error(err))
			}

			nasPdu := NASPDU{
				Raw:     ie.Value.NASPDU.Value,
				Decoded: decodednNasPdu,
			}

			ies = append(ies, IE{
				ID:          protocolIEIDToEnum(ie.Id.Value),
				Criticality: criticalityToEnum(ie.Criticality.Value),
				Value:       nasPdu,
			})
		default:
			ies = append(ies, IE{
				ID:          protocolIEIDToEnum(ie.Id.Value),
				Criticality: criticalityToEnum(ie.Criticality.Value),
				Value: UnknownIE{
					Reason: fmt.Sprintf("unsupported ie type %d", ie.Id.Value),
				},
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

		pduSessionResourceSetupList = append(pduSessionResourceSetupList, PDUSessionResourceSetupCxtReq{
			PDUSessionID:                           item.PDUSessionID.Value,
			SNSSAI:                                 *buildSNSSAI(&item.SNSSAI),
			PDUSessionResourceSetupRequestTransfer: item.PDUSessionResourceSetupRequestTransfer,
		})

		if item.NASPDU != nil {
			decodednNasPdu, err := nas.DecodeNASMessage(item.NASPDU.Value, nil)
			if err != nil {
				logger.EllaLog.Warn("Failed to decode NAS PDU", zap.Error(err))
			}
			pduSessionResourceSetupList[i].NASPDU = &NASPDU{
				Raw:     item.NASPDU.Value,
				Decoded: decodednNasPdu,
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

func buildCoreNetworkAssistanceInformation(cnai ngapType.CoreNetworkAssistanceInformation) CoreNetworkAssistanceInformation {
	returnedCNAI := CoreNetworkAssistanceInformation{}

	switch cnai.UEIdentityIndexValue.Present {
	case ngapType.UEIdentityIndexValuePresentIndexLength10:
		returnedCNAI.UEIdentityIndexValue = bitStringToHex(cnai.UEIdentityIndexValue.IndexLength10)
	default:
		logger.EllaLog.Warn("Unsupported UEIdentityIndexValue", zap.Int("present", cnai.UEIdentityIndexValue.Present))
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
