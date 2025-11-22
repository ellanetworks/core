package ngap

import (
	"fmt"

	"github.com/ellanetworks/core/internal/decoder/utils"
	"github.com/free5gc/aper"
	"github.com/free5gc/ngap/ngapType"
)

type Guami struct {
	PLMNID      PLMNID `json:"plmn_id"`
	AMFRegionID string `json:"amf_region_id"`
	AMFSetID    string `json:"amf_set_id"`
	AMFPointer  string `json:"amf_pointer"`
}

type IEsCriticalityDiagnostics struct {
	IECriticality utils.EnumField[uint64] `json:"ie_criticality"`
	IEID          utils.EnumField[int64]  `json:"ie_id"`
	TypeOfError   utils.EnumField[uint64] `json:"type_of_error"`
}

type CriticalityDiagnostics struct {
	ProcedureCode             *utils.EnumField[int64]     `json:"procedure_code,omitempty"`
	TriggeringMessage         *utils.EnumField[uint64]    `json:"triggering_message,omitempty"`
	ProcedureCriticality      *utils.EnumField[uint64]    `json:"procedure_criticality,omitempty"`
	IEsCriticalityDiagnostics []IEsCriticalityDiagnostics `json:"ie_criticality_diagnostics,omitempty"`
}

func buildAMFNameIE(an ngapType.AMFName) string {
	return an.Value
}

func buildGUAMI(guami ngapType.GUAMI) Guami {
	return Guami{
		PLMNID:      plmnIDToModels(guami.PLMNIdentity),
		AMFRegionID: bitStringToHex(&guami.AMFRegionID.Value),
		AMFSetID:    bitStringToHex(&guami.AMFSetID.Value),
		AMFPointer:  bitStringToHex(&guami.AMFPointer.Value),
	}
}

func buildServedGUAMIListIE(sgl ngapType.ServedGUAMIList) []Guami {
	guamiList := make([]Guami, len(sgl.List))
	for i := 0; i < len(sgl.List); i++ {
		guamiList[i] = buildGUAMI(sgl.List[i].GUAMI)
	}

	return guamiList
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

func buildNGSetupResponse(ngSetupResponse ngapType.NGSetupResponse) NGAPMessageValue {
	ies := make([]IE, 0)

	for i := 0; i < len(ngSetupResponse.ProtocolIEs.List); i++ {
		ie := ngSetupResponse.ProtocolIEs.List[i]

		switch ie.Id.Value {
		case ngapType.ProtocolIEIDAMFName:
			ies = append(ies, IE{
				ID:          protocolIEIDToEnum(ie.Id.Value),
				Criticality: criticalityToEnum(ie.Criticality.Value),
				Value:       buildAMFNameIE(*ie.Value.AMFName),
			})
		case ngapType.ProtocolIEIDServedGUAMIList:
			ies = append(ies, IE{
				ID:          protocolIEIDToEnum(ie.Id.Value),
				Criticality: criticalityToEnum(ie.Criticality.Value),
				Value:       buildServedGUAMIListIE(*ie.Value.ServedGUAMIList),
			})
		case ngapType.ProtocolIEIDRelativeAMFCapacity:
			ies = append(ies, IE{
				ID:          protocolIEIDToEnum(ie.Id.Value),
				Criticality: criticalityToEnum(ie.Criticality.Value),
				Value:       ie.Value.RelativeAMFCapacity.Value,
			})
		case ngapType.ProtocolIEIDPLMNSupportList:
			ies = append(ies, IE{
				ID:          protocolIEIDToEnum(ie.Id.Value),
				Criticality: criticalityToEnum(ie.Criticality.Value),
				Value:       buildPLMNSupportListIE(ie.Value.PLMNSupportList),
			})
		case ngapType.ProtocolIEIDCriticalityDiagnostics:
			ies = append(ies, IE{
				ID:          protocolIEIDToEnum(ie.Id.Value),
				Criticality: criticalityToEnum(ie.Criticality.Value),
				Value:       buildCriticalityDiagnosticsIE(ie.Value.CriticalityDiagnostics),
			})
		case ngapType.ProtocolIEIDUERetentionInformation:
			ies = append(ies, IE{
				ID:          protocolIEIDToEnum(ie.Id.Value),
				Criticality: criticalityToEnum(ie.Criticality.Value),
				Value:       buildUERetentionInformationIE(*ie.Value.UERetentionInformation),
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

func buildCriticalityDiagnosticsIE(cd *ngapType.CriticalityDiagnostics) CriticalityDiagnostics {
	critDiag := CriticalityDiagnostics{}

	if cd.ProcedureCode != nil {
		procCode := procedureCodeToEnum(cd.ProcedureCode.Value)
		critDiag.ProcedureCode = &procCode
	}

	if cd.TriggeringMessage != nil {
		trigMsg := triggeringMessageToString(cd.TriggeringMessage.Value)
		critDiag.TriggeringMessage = &trigMsg
	}

	if cd.ProcedureCriticality != nil {
		procCrit := criticalityToEnum(cd.ProcedureCriticality.Value)
		critDiag.ProcedureCriticality = &procCrit
	}

	if cd.IEsCriticalityDiagnostics != nil {
		critDiag.IEsCriticalityDiagnostics = buildIEsCriticalityDiagnisticsList(cd.IEsCriticalityDiagnostics)
	}

	return critDiag
}

func triggeringMessageToString(tm aper.Enumerated) utils.EnumField[uint64] {
	switch tm {
	case ngapType.TriggeringMessagePresentInitiatingMessage:
		return utils.MakeEnum(uint64(tm), "InitiatingMessage", false)
	case ngapType.TriggeringMessagePresentSuccessfulOutcome:
		return utils.MakeEnum(uint64(tm), "SuccessfulOutcome", false)
	case ngapType.TriggeringMessagePresentUnsuccessfullOutcome:
		return utils.MakeEnum(uint64(tm), "UnsuccessfulOutcome", false)
	default:
		return utils.MakeEnum(uint64(tm), "", true)
	}
}

func buildIEsCriticalityDiagnisticsList(ieList *ngapType.CriticalityDiagnosticsIEList) []IEsCriticalityDiagnostics {
	if ieList == nil {
		return nil
	}

	ies := make([]IEsCriticalityDiagnostics, len(ieList.List))
	for i := 0; i < len(ieList.List); i++ {
		ie := ieList.List[i]
		ies[i] = IEsCriticalityDiagnostics{
			IECriticality: criticalityToEnum(ie.IECriticality.Value),
			IEID:          protocolIEIDToEnum(ie.IEID.Value),
			TypeOfError:   typeOfErrorToString(ie.TypeOfError.Value),
		}
	}

	return ies
}

func typeOfErrorToString(toe aper.Enumerated) utils.EnumField[uint64] {
	switch toe {
	case ngapType.TypeOfErrorPresentNotUnderstood:
		return utils.MakeEnum(uint64(toe), "NotUnderstood", false)
	case ngapType.TypeOfErrorPresentMissing:
		return utils.MakeEnum(uint64(toe), "Missing", false)
	default:
		return utils.MakeEnum(uint64(toe), "", true)
	}
}
