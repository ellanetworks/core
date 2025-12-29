package ngap

import (
	"github.com/free5gc/aper"
	"github.com/free5gc/ngap/ngapType"
)

func buildCriticalityDiagnostics(
	procedureCode int64,
	triggeringMessage aper.Enumerated,
	procedureCriticality aper.Enumerated,
	iesCriticalityDiagnostics *ngapType.CriticalityDiagnosticsIEList) ngapType.CriticalityDiagnostics {
	return ngapType.CriticalityDiagnostics{
		ProcedureCode: &ngapType.ProcedureCode{
			Value: procedureCode,
		},
		TriggeringMessage: &ngapType.TriggeringMessage{
			Value: triggeringMessage,
		},
		ProcedureCriticality: &ngapType.Criticality{
			Value: procedureCriticality,
		},
		IEsCriticalityDiagnostics: iesCriticalityDiagnostics,
	}
}

func buildCriticalityDiagnosticsIEItem(ieID int64) ngapType.CriticalityDiagnosticsIEItem {
	return ngapType.CriticalityDiagnosticsIEItem{
		IECriticality: ngapType.Criticality{
			Value: ngapType.CriticalityPresentReject,
		},
		IEID: ngapType.ProtocolIEID{
			Value: ieID,
		},
		TypeOfError: ngapType.TypeOfError{
			Value: ngapType.TypeOfErrorPresentMissing,
		},
	}
}
