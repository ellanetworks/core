package ngap

import (
	"github.com/ellanetworks/core/internal/amf/context"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/free5gc/ngap/ngapType"
	"go.uber.org/zap"
)

func HandleNGResetAcknowledge(ran *context.AmfRan, message *ngapType.NGAPPDU) {
	var uEAssociatedLogicalNGConnectionList *ngapType.UEAssociatedLogicalNGConnectionList
	var criticalityDiagnostics *ngapType.CriticalityDiagnostics

	if ran == nil {
		logger.AmfLog.Error("ran is nil")
		return
	}

	if message == nil {
		ran.Log.Error("NGAP Message is nil")
		return
	}
	successfulOutcome := message.SuccessfulOutcome
	if successfulOutcome == nil {
		ran.Log.Error("SuccessfulOutcome is nil")
		return
	}
	nGResetAcknowledge := successfulOutcome.Value.NGResetAcknowledge
	if nGResetAcknowledge == nil {
		ran.Log.Error("NGResetAcknowledge is nil")
		return
	}

	for _, ie := range nGResetAcknowledge.ProtocolIEs.List {
		switch ie.Id.Value {
		case ngapType.ProtocolIEIDUEAssociatedLogicalNGConnectionList:
			uEAssociatedLogicalNGConnectionList = ie.Value.UEAssociatedLogicalNGConnectionList
		case ngapType.ProtocolIEIDCriticalityDiagnostics:
			criticalityDiagnostics = ie.Value.CriticalityDiagnostics
		}
	}

	if uEAssociatedLogicalNGConnectionList != nil {
		ran.Log.Debug("UE association(s) has been reset", zap.Int("len", len(uEAssociatedLogicalNGConnectionList.List)))
		for i, item := range uEAssociatedLogicalNGConnectionList.List {
			if item.AMFUENGAPID != nil && item.RANUENGAPID != nil {
				ran.Log.Debug("", zap.Int("index", i+1), zap.Int64("AmfUeNgapID", item.AMFUENGAPID.Value), zap.Int64("RanUeNgapID", item.RANUENGAPID.Value))
			} else if item.AMFUENGAPID != nil {
				ran.Log.Debug("", zap.Int("index", i+1), zap.Int64("AmfUeNgapID", item.AMFUENGAPID.Value), zap.String("RanUeNgapID", "-1"))
			} else if item.RANUENGAPID != nil {
				ran.Log.Debug("", zap.Int("index", i+1), zap.String("AmfUeNgapID", "-1"), zap.Int64("RanUeNgapID", item.RANUENGAPID.Value))
			}
		}
	}

	if criticalityDiagnostics != nil {
		printCriticalityDiagnostics(ran, criticalityDiagnostics)
	}
}
