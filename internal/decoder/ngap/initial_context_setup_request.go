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

type InitialContextSetupRequest struct {
	IEs []IE `json:"ies"`
}

func buildInitialContextSetupRequest(initialContextSetupRequest *ngapType.InitialContextSetupRequest) *InitialContextSetupRequest {
	if initialContextSetupRequest == nil {
		return nil
	}

	ieList := &InitialContextSetupRequest{}

	AMFUENGAPID := int64(0)

	for i := 0; i < len(initialContextSetupRequest.ProtocolIEs.List); i++ {
		ie := initialContextSetupRequest.ProtocolIEs.List[i]
		switch ie.Id.Value {
		case ngapType.ProtocolIEIDAMFUENGAPID:
			AMFUENGAPID = ie.Value.AMFUENGAPID.Value
			ieList.IEs = append(ieList.IEs, IE{
				ID:          protocolIEIDToEnum(ie.Id.Value),
				Criticality: criticalityToEnum(ie.Criticality.Value),
				AMFUENGAPID: &ie.Value.AMFUENGAPID.Value,
			})
		case ngapType.ProtocolIEIDRANUENGAPID:
			ieList.IEs = append(ieList.IEs, IE{
				ID:          protocolIEIDToEnum(ie.Id.Value),
				Criticality: criticalityToEnum(ie.Criticality.Value),
				Value:       ie.Value.RANUENGAPID.Value,
			})
		case ngapType.ProtocolIEIDOldAMF:
			ieList.IEs = append(ieList.IEs, IE{
				ID:          protocolIEIDToEnum(ie.Id.Value),
				Criticality: criticalityToEnum(ie.Criticality.Value),
				Value:       ie.Value.OldAMF.Value,
			})
		case ngapType.ProtocolIEIDUEAggregateMaximumBitRate:
			ieList.IEs = append(ieList.IEs, IE{
				ID:          protocolIEIDToEnum(ie.Id.Value),
				Criticality: criticalityToEnum(ie.Criticality.Value),
				UEAggregateMaximumBitRate: &UEAggregateMaximumBitRate{
					Downlink: ie.Value.UEAggregateMaximumBitRate.UEAggregateMaximumBitRateDL.Value,
					Uplink:   ie.Value.UEAggregateMaximumBitRate.UEAggregateMaximumBitRateUL.Value,
				},
			})
		case ngapType.ProtocolIEIDCoreNetworkAssistanceInformation:
			ieList.IEs = append(ieList.IEs, IE{
				ID:                               protocolIEIDToEnum(ie.Id.Value),
				Criticality:                      criticalityToEnum(ie.Criticality.Value),
				CoreNetworkAssistanceInformation: buildCoreNetworkAssistanceInformation(ie.Value.CoreNetworkAssistanceInformation),
			})
		case ngapType.ProtocolIEIDGUAMI:
			ieList.IEs = append(ieList.IEs, IE{
				ID:          protocolIEIDToEnum(ie.Id.Value),
				Criticality: criticalityToEnum(ie.Criticality.Value),
				GUAMI:       buildGUAMI(ie.Value.GUAMI),
			})
		case ngapType.ProtocolIEIDPDUSessionResourceSetupListCxtReq:
			ieList.IEs = append(ieList.IEs, IE{
				ID:                                protocolIEIDToEnum(ie.Id.Value),
				Criticality:                       criticalityToEnum(ie.Criticality.Value),
				PDUSessionResourceSetupListCxtReq: buildPDUSessionResourceSetupListCxtReq(ie.Value.PDUSessionResourceSetupListCxtReq),
			})
		case ngapType.ProtocolIEIDAllowedNSSAI:
			ieList.IEs = append(ieList.IEs, IE{
				ID:           protocolIEIDToEnum(ie.Id.Value),
				Criticality:  criticalityToEnum(ie.Criticality.Value),
				AllowedNSSAI: buildAllowedNSSAI(ie.Value.AllowedNSSAI),
			})
		case ngapType.ProtocolIEIDUESecurityCapabilities:
			ieList.IEs = append(ieList.IEs, IE{
				ID:                     protocolIEIDToEnum(ie.Id.Value),
				Criticality:            criticalityToEnum(ie.Criticality.Value),
				UESecurityCapabilities: buildUESecurityCapabilities(ie.Value.UESecurityCapabilities),
			})
		case ngapType.ProtocolIEIDSecurityKey:
			securityKey := bitStringToHex(&ie.Value.SecurityKey.Value)
			ieList.IEs = append(ieList.IEs, IE{
				ID:          protocolIEIDToEnum(ie.Id.Value),
				Criticality: criticalityToEnum(ie.Criticality.Value),
				SecurityKey: &securityKey,
			})
		case ngapType.ProtocolIEIDMobilityRestrictionList:
			ieList.IEs = append(ieList.IEs, IE{
				ID:                      protocolIEIDToEnum(ie.Id.Value),
				Criticality:             criticalityToEnum(ie.Criticality.Value),
				MobilityRestrictionList: buildMobilityRestrictionListIE(ie.Value.MobilityRestrictionList),
			})
		case ngapType.ProtocolIEIDIndexToRFSP:
			ieList.IEs = append(ieList.IEs, IE{
				ID:          protocolIEIDToEnum(ie.Id.Value),
				Criticality: criticalityToEnum(ie.Criticality.Value),
				IndexToRFSP: &ie.Value.IndexToRFSP.Value,
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

			ieList.IEs = append(ieList.IEs, IE{
				ID:          protocolIEIDToEnum(ie.Id.Value),
				Criticality: criticalityToEnum(ie.Criticality.Value),
				Value:       nasPdu,
			})
		default:
			ieList.IEs = append(ieList.IEs, IE{
				ID:          protocolIEIDToEnum(ie.Id.Value),
				Criticality: criticalityToEnum(ie.Criticality.Value),
				Value: UnknownIE{
					Reason: fmt.Sprintf("unsupported ie type %d", ie.Id.Value),
				},
			})
		}
	}

	return ieList
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

func buildUESecurityCapabilities(uesec *ngapType.UESecurityCapabilities) *UESecurityCapabilities {
	if uesec == nil {
		return nil
	}

	return &UESecurityCapabilities{
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
