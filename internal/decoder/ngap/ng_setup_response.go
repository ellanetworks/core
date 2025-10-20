package ngap

import (
	"fmt"

	"github.com/omec-project/ngap/aper"
	"github.com/omec-project/ngap/ngapConvert"
	"github.com/omec-project/ngap/ngapType"
)

type Guami struct {
	PLMNID PLMNID `json:"plmn_id"`
	AMFID  string `json:"amf_id"`
}

type IEsCriticalityDiagnostics struct {
	IECriticality EnumField `json:"ie_criticality"`
	IEID          EnumField `json:"ie_id"`
	TypeOfError   string    `json:"type_of_error"`
}

type CriticalityDiagnostics struct {
	ProcedureCode             *EnumField                  `json:"procedure_code,omitempty"`
	TriggeringMessage         *string                     `json:"triggering_message,omitempty"`
	ProcedureCriticality      *EnumField                  `json:"procedure_criticality,omitempty"`
	IEsCriticalityDiagnostics []IEsCriticalityDiagnostics `json:"ie_criticality_diagnostics,omitempty"`
}

func buildAMFNameIE(an ngapType.AMFName) string {
	return an.Value
}

func buildGUAMI(guami ngapType.GUAMI) Guami {
	amfID := ngapConvert.AmfIdToModels(guami.AMFRegionID.Value, guami.AMFSetID.Value, guami.AMFPointer.Value)
	return Guami{
		PLMNID: plmnIDToModels(guami.PLMNIdentity),
		AMFID:  amfID,
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
