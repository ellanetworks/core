package ngap

import (
	"github.com/ellanetworks/core/internal/amf/context"
	"github.com/free5gc/aper"
	"github.com/free5gc/ngap/ngapType"
	"go.uber.org/zap"
)

func printCriticalityDiagnostics(ran *context.AmfRan, criticalityDiagnostics *ngapType.CriticalityDiagnostics) {
	ran.Log.Debug("Criticality Diagnostics")

	if criticalityDiagnostics.ProcedureCriticality != nil {
		switch criticalityDiagnostics.ProcedureCriticality.Value {
		case ngapType.CriticalityPresentReject:
			ran.Log.Debug("Procedure Criticality: Reject")
		case ngapType.CriticalityPresentIgnore:
			ran.Log.Debug("Procedure Criticality: Ignore")
		case ngapType.CriticalityPresentNotify:
			ran.Log.Debug("Procedure Criticality: Notify")
		}
	}

	if criticalityDiagnostics.IEsCriticalityDiagnostics != nil {
		for _, ieCriticalityDiagnostics := range criticalityDiagnostics.IEsCriticalityDiagnostics.List {
			ran.Log.Debug("IE ID", zap.Int64("IE ID", ieCriticalityDiagnostics.IEID.Value))

			switch ieCriticalityDiagnostics.IECriticality.Value {
			case ngapType.CriticalityPresentReject:
				ran.Log.Debug("Criticality Reject")
			case ngapType.CriticalityPresentNotify:
				ran.Log.Debug("Criticality Notify")
			}

			switch ieCriticalityDiagnostics.TypeOfError.Value {
			case ngapType.TypeOfErrorPresentNotUnderstood:
				ran.Log.Debug("Type of error: Not understood")
			case ngapType.TypeOfErrorPresentMissing:
				ran.Log.Debug("Type of error: Missing")
			}
		}
	}
}

func buildCriticalityDiagnostics(
	procedureCode *int64,
	triggeringMessage *aper.Enumerated,
	procedureCriticality *aper.Enumerated,
	iesCriticalityDiagnostics *ngapType.CriticalityDiagnosticsIEList) (
	criticalityDiagnostics ngapType.CriticalityDiagnostics,
) {
	if procedureCode != nil {
		criticalityDiagnostics.ProcedureCode = new(ngapType.ProcedureCode)
		criticalityDiagnostics.ProcedureCode.Value = *procedureCode
	}

	if triggeringMessage != nil {
		criticalityDiagnostics.TriggeringMessage = new(ngapType.TriggeringMessage)
		criticalityDiagnostics.TriggeringMessage.Value = *triggeringMessage
	}

	if procedureCriticality != nil {
		criticalityDiagnostics.ProcedureCriticality = new(ngapType.Criticality)
		criticalityDiagnostics.ProcedureCriticality.Value = *procedureCriticality
	}

	if iesCriticalityDiagnostics != nil {
		criticalityDiagnostics.IEsCriticalityDiagnostics = iesCriticalityDiagnostics
	}

	return criticalityDiagnostics
}

func buildCriticalityDiagnosticsIEItem(ieID int64) (
	item ngapType.CriticalityDiagnosticsIEItem,
) {
	ieCriticality := ngapType.CriticalityPresentReject
	typeOfErr := ngapType.TypeOfErrorPresentMissing
	item = ngapType.CriticalityDiagnosticsIEItem{
		IECriticality: ngapType.Criticality{
			Value: ieCriticality,
		},
		IEID: ngapType.ProtocolIEID{
			Value: ieID,
		},
		TypeOfError: ngapType.TypeOfError{
			Value: typeOfErr,
		},
	}

	return item
}
