package ngap

import (
	"fmt"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/omec-project/ngap/aper"
	"github.com/omec-project/ngap/ngapConvert"
	"github.com/omec-project/ngap/ngapType"
	"go.uber.org/zap"
)

type NGSetupResponse struct {
	IEs []IE `json:"ies"`
}

func buildAMFNameIE(an *ngapType.AMFName) *string {
	if an == nil || an.Value == "" {
		return nil
	}

	s := an.Value

	return &s
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
				Criticality: criticalityToEnum(ie.Criticality.Value),
				AMFName:     buildAMFNameIE(ie.Value.AMFName),
			})
		case ngapType.ProtocolIEIDServedGUAMIList:
			ngSetup.IEs = append(ngSetup.IEs, IE{
				ID:              protocolIEIDToString(ie.Id.Value),
				Criticality:     criticalityToEnum(ie.Criticality.Value),
				ServedGUAMIList: buildServedGUAMIListIE(ie.Value.ServedGUAMIList),
			})
		case ngapType.ProtocolIEIDRelativeAMFCapacity:
			ngSetup.IEs = append(ngSetup.IEs, IE{
				ID:                  protocolIEIDToString(ie.Id.Value),
				Criticality:         criticalityToEnum(ie.Criticality.Value),
				RelativeAMFCapacity: &ie.Value.RelativeAMFCapacity.Value,
			})
		case ngapType.ProtocolIEIDPLMNSupportList:
			ngSetup.IEs = append(ngSetup.IEs, IE{
				ID:              protocolIEIDToString(ie.Id.Value),
				Criticality:     criticalityToEnum(ie.Criticality.Value),
				PLMNSupportList: buildPLMNSupportListIE(ie.Value.PLMNSupportList),
			})
		case ngapType.ProtocolIEIDCriticalityDiagnostics:
			ngSetup.IEs = append(ngSetup.IEs, IE{
				ID:                     protocolIEIDToString(ie.Id.Value),
				Criticality:            criticalityToEnum(ie.Criticality.Value),
				CriticalityDiagnostics: buildCriticalityDiagnosticsIE(ie.Value.CriticalityDiagnostics),
			})
		case ngapType.ProtocolIEIDUERetentionInformation:
			ngSetup.IEs = append(ngSetup.IEs, IE{
				ID:                     protocolIEIDToString(ie.Id.Value),
				Criticality:            criticalityToEnum(ie.Criticality.Value),
				UERetentionInformation: buildUERetentionInformationIE(ie.Value.UERetentionInformation),
			})
		default:
			ngSetup.IEs = append(ngSetup.IEs, IE{
				ID:          protocolIEIDToString(ie.Id.Value),
				Criticality: criticalityToEnum(ie.Criticality.Value),
			})
			logger.EllaLog.Warn("Unsupported ie type", zap.Int64("type", ie.Id.Value))
		}
	}

	return ngSetup
}

func buildCriticalityDiagnosticsIE(cd *ngapType.CriticalityDiagnostics) *CriticalityDiagnostics {
	if cd == nil {
		return nil
	}

	critDiag := &CriticalityDiagnostics{}

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
