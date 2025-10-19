package ngap

import (
	"encoding/hex"
	"fmt"

	"github.com/ellanetworks/core/internal/decoder/nas"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/omec-project/ngap/ngapConvert"
	"github.com/omec-project/ngap/ngapType"
	"go.uber.org/zap"
)

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
			decodednNasPdu, err := nas.DecodeNASMessage(ie.Value.NASPDU.Value, nil)
			if err != nil {
				logger.EllaLog.Warn("Failed to decode NAS PDU", zap.Error(err))
			}
			nasPdu := &NASPDU{
				Raw:     ie.Value.NASPDU.Value,
				Decoded: decodednNasPdu,
			}
			ieList.IEs = append(ieList.IEs, IE{
				ID:          protocolIEIDToString(ie.Id.Value),
				Criticality: criticalityToString(ie.Criticality.Value),
				NASPDU:      nasPdu,
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
