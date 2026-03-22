package ngap

import (
	gocontext "context"

	amfContext "github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/free5gc/ngap/ngapType"
)

func HandleErrorIndication(ctx gocontext.Context, ran *amfContext.Radio, msg *ngapType.ErrorIndication) {
	if msg == nil {
		logger.WithTrace(ctx, ran.Log).Error("ErrorIndication is nil")
		return
	}

	var (
		aMFUENGAPID            *ngapType.AMFUENGAPID
		rANUENGAPID            *ngapType.RANUENGAPID
		cause                  *ngapType.Cause
		criticalityDiagnostics *ngapType.CriticalityDiagnostics
	)

	for _, ie := range msg.ProtocolIEs.List {
		switch ie.Id.Value {
		case ngapType.ProtocolIEIDAMFUENGAPID:
			aMFUENGAPID = ie.Value.AMFUENGAPID
			if aMFUENGAPID == nil {
				logger.WithTrace(ctx, ran.Log).Error("AmfUeNgapID is nil")
			}
		case ngapType.ProtocolIEIDRANUENGAPID:
			rANUENGAPID = ie.Value.RANUENGAPID
			if rANUENGAPID == nil {
				logger.WithTrace(ctx, ran.Log).Error("RanUeNgapID is nil")
			}
		case ngapType.ProtocolIEIDCause:
			cause = ie.Value.Cause
		case ngapType.ProtocolIEIDCriticalityDiagnostics:
			criticalityDiagnostics = ie.Value.CriticalityDiagnostics
		}
	}

	if cause == nil && criticalityDiagnostics == nil {
		logger.WithTrace(ctx, ran.Log).Error("[ErrorIndication] both Cause IE and CriticalityDiagnostics IE are nil, should have at least one")
		return
	}

	if cause != nil {
		logger.WithTrace(ctx, logger.AmfLog).Debug("Error Indication Cause", logger.Cause(causeToString(*cause)))
	}
}
