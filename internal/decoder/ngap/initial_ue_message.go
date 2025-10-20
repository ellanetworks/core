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

func buildInitialUEMessage(initialUEMessage ngapType.InitialUEMessage) NGAPMessageValue {
	ies := make([]IE, 0)

	for i := 0; i < len(initialUEMessage.ProtocolIEs.List); i++ {
		ie := initialUEMessage.ProtocolIEs.List[i]

		switch ie.Id.Value {
		case ngapType.ProtocolIEIDRANUENGAPID:
			ies = append(ies, IE{
				ID:          protocolIEIDToEnum(ie.Id.Value),
				Criticality: criticalityToEnum(ie.Criticality.Value),
				Value:       ie.Value.RANUENGAPID.Value,
			})
		case ngapType.ProtocolIEIDNASPDU:
			decodednNasPdu, err := nas.DecodeNASMessage(ie.Value.NASPDU.Value, nil)
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
		case ngapType.ProtocolIEIDUserLocationInformation:
			ies = append(ies, IE{
				ID:          protocolIEIDToEnum(ie.Id.Value),
				Criticality: criticalityToEnum(ie.Criticality.Value),
				Value:       buildUserLocationInformationIE(*ie.Value.UserLocationInformation),
			})
		case ngapType.ProtocolIEIDRRCEstablishmentCause:
			ies = append(ies, IE{
				ID:          protocolIEIDToEnum(ie.Id.Value),
				Criticality: criticalityToEnum(ie.Criticality.Value),
				Value:       buildRRCEstablishmentCauseIE(*ie.Value.RRCEstablishmentCause),
			})
		case ngapType.ProtocolIEIDFiveGSTMSI:
			ies = append(ies, IE{
				ID:          protocolIEIDToEnum(ie.Id.Value),
				Criticality: criticalityToEnum(ie.Criticality.Value),
				Value:       buildFiveGSTMSIIE(*ie.Value.FiveGSTMSI),
			})
		case ngapType.ProtocolIEIDAMFSetID:
			ies = append(ies, IE{
				ID:          protocolIEIDToEnum(ie.Id.Value),
				Criticality: criticalityToEnum(ie.Criticality.Value),
				Value:       bitStringToHex(&ie.Value.AMFSetID.Value),
			})
		case ngapType.ProtocolIEIDUEContextRequest:
			ies = append(ies, IE{
				ID:          protocolIEIDToEnum(ie.Id.Value),
				Criticality: criticalityToEnum(ie.Criticality.Value),
				Value:       buildUEContextRequestIE(*ie.Value.UEContextRequest),
			})
		case ngapType.ProtocolIEIDAllowedNSSAI:
			ies = append(ies, IE{
				ID:          protocolIEIDToEnum(ie.Id.Value),
				Criticality: criticalityToEnum(ie.Criticality.Value),
				Value:       buildAllowedNSSAI(*ie.Value.AllowedNSSAI),
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

func buildFiveGSTMSIIE(fivegStmsi ngapType.FiveGSTMSI) FiveGSTMSI {
	fiveg := FiveGSTMSI{}

	fiveg.AMFSetID = bitStringToHex(&fivegStmsi.AMFSetID.Value)
	fiveg.AMFPointer = bitStringToHex(&fivegStmsi.AMFPointer.Value)
	fiveg.FiveGTMSI = hex.EncodeToString(fivegStmsi.FiveGTMSI.Value)

	return fiveg
}

func buildRRCEstablishmentCauseIE(rrc ngapType.RRCEstablishmentCause) EnumField {
	switch rrc.Value {
	case ngapType.RRCEstablishmentCausePresentEmergency:
		return EnumField{Label: "Emergency", Value: int(ngapType.RRCEstablishmentCausePresentEmergency)}
	case ngapType.RRCEstablishmentCausePresentHighPriorityAccess:
		return EnumField{Label: "HighPriorityAccess", Value: int(ngapType.RRCEstablishmentCausePresentHighPriorityAccess)}
	case ngapType.RRCEstablishmentCausePresentMtAccess:
		return EnumField{Label: "MtAccess", Value: int(ngapType.RRCEstablishmentCausePresentMtAccess)}
	case ngapType.RRCEstablishmentCausePresentMoSignalling:
		return EnumField{Label: "MoSignalling", Value: int(ngapType.RRCEstablishmentCausePresentMoSignalling)}
	case ngapType.RRCEstablishmentCausePresentMoData:
		return EnumField{Label: "MoData", Value: int(ngapType.RRCEstablishmentCausePresentMoData)}
	case ngapType.RRCEstablishmentCausePresentMoVoiceCall:
		return EnumField{Label: "MoVoiceCall", Value: int(ngapType.RRCEstablishmentCausePresentMoVoiceCall)}
	case ngapType.RRCEstablishmentCausePresentMoVideoCall:
		return EnumField{Label: "MoVideoCall", Value: int(ngapType.RRCEstablishmentCausePresentMoVideoCall)}
	case ngapType.RRCEstablishmentCausePresentMoSMS:
		return EnumField{Label: "MoSMS", Value: int(ngapType.RRCEstablishmentCausePresentMoSMS)}
	case ngapType.RRCEstablishmentCausePresentMpsPriorityAccess:
		return EnumField{Label: "MpsPriorityAccess", Value: int(ngapType.RRCEstablishmentCausePresentMpsPriorityAccess)}
	case ngapType.RRCEstablishmentCausePresentMcsPriorityAccess:
		return EnumField{Label: "McsPriorityAccess", Value: int(ngapType.RRCEstablishmentCausePresentMcsPriorityAccess)}
	case ngapType.RRCEstablishmentCausePresentNotAvailable:
		return EnumField{Label: "NotAvailable", Value: int(ngapType.RRCEstablishmentCausePresentNotAvailable)}
	default:
		return EnumField{Label: "Unknown", Value: int(rrc.Value)}
	}
}

func buildUEContextRequestIE(ueCtxReq ngapType.UEContextRequest) EnumField {
	switch ueCtxReq.Value {
	case ngapType.UEContextRequestPresentRequested:
		return EnumField{Label: "Requested", Value: int(ngapType.UEContextRequestPresentRequested)}
	default:
		return EnumField{Label: "Unknown", Value: int(ueCtxReq.Value)}
	}
}

func buildAllowedNSSAI(allowedNSSAI ngapType.AllowedNSSAI) []SNSSAI {
	snssaiList := make([]SNSSAI, 0)

	for i := 0; i < len(allowedNSSAI.List); i++ {
		ngapSnssai := allowedNSSAI.List[i].SNSSAI
		snssai := buildSNSSAI(&ngapSnssai)
		snssaiList = append(snssaiList, *snssai)
	}

	return snssaiList
}

func buildUserLocationInformationIE(uli ngapType.UserLocationInformation) UserLocationInformation {
	userLocationInfo := UserLocationInformation{}

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
