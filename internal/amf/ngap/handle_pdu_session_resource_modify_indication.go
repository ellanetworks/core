package ngap

import (
	"context"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/free5gc/ngap/ngapType"
	"go.uber.org/zap"
)

func HandlePDUSessionResourceModifyIndication(ctx context.Context, ran *amf.Radio, msg *ngapType.PDUSessionResourceModifyIndication) {
	if msg == nil {
		logger.WithTrace(ctx, ran.Log).Error("NGAP Message is nil")
		return
	}

	var (
		aMFUENGAPID                            *ngapType.AMFUENGAPID
		rANUENGAPID                            *ngapType.RANUENGAPID
		pduSessionResourceModifyIndicationList *ngapType.PDUSessionResourceModifyListModInd
		iesCriticalityDiagnostics              ngapType.CriticalityDiagnosticsIEList
	)

	for _, ie := range msg.ProtocolIEs.List {
		switch ie.Id.Value {
		case ngapType.ProtocolIEIDAMFUENGAPID: // reject
			aMFUENGAPID = ie.Value.AMFUENGAPID
			if aMFUENGAPID == nil {
				logger.WithTrace(ctx, ran.Log).Error("AmfUeNgapID is nil")

				item := buildCriticalityDiagnosticsIEItem(ngapType.ProtocolIEIDAMFUENGAPID)
				iesCriticalityDiagnostics.List = append(iesCriticalityDiagnostics.List, item)
			}
		case ngapType.ProtocolIEIDRANUENGAPID: // reject
			rANUENGAPID = ie.Value.RANUENGAPID
			if rANUENGAPID == nil {
				logger.WithTrace(ctx, ran.Log).Error("RanUeNgapID is nil")

				item := buildCriticalityDiagnosticsIEItem(ngapType.ProtocolIEIDRANUENGAPID)
				iesCriticalityDiagnostics.List = append(iesCriticalityDiagnostics.List, item)
			}
		case ngapType.ProtocolIEIDPDUSessionResourceModifyListModInd: // reject
			pduSessionResourceModifyIndicationList = ie.Value.PDUSessionResourceModifyListModInd
			if pduSessionResourceModifyIndicationList == nil {
				logger.WithTrace(ctx, ran.Log).Error("PDUSessionResourceModifyListModInd is nil")

				item := buildCriticalityDiagnosticsIEItem(ngapType.ProtocolIEIDPDUSessionResourceModifyListModInd)
				iesCriticalityDiagnostics.List = append(iesCriticalityDiagnostics.List, item)
			}
		}
	}

	if len(iesCriticalityDiagnostics.List) > 0 {
		logger.WithTrace(ctx, ran.Log).Error("Has missing reject IE(s)")

		criticalityDiagnostics := buildCriticalityDiagnostics(ngapType.ProcedureCodePDUSessionResourceModifyIndication, ngapType.TriggeringMessagePresentInitiatingMessage, ngapType.CriticalityPresentReject, &iesCriticalityDiagnostics)

		err := ran.NGAPSender.SendErrorIndication(ctx, nil, &criticalityDiagnostics)
		if err != nil {
			logger.WithTrace(ctx, ran.Log).Error("error sending error indication", zap.Error(err))
			return
		}

		logger.WithTrace(ctx, ran.Log).Info("sent error indication")

		return
	}

	if aMFUENGAPID == nil {
		logger.WithTrace(ctx, ran.Log).Error("AMFUENGAPID IE (mandatory) is missing in PDUSessionResourceModifyIndication")
		return
	}

	if rANUENGAPID == nil {
		logger.WithTrace(ctx, ran.Log).Error("RANUENGAPID IE (mandatory) is missing in PDUSessionResourceModifyIndication")
		return
	}

	ranUe := ran.FindUEByRanUeNgapID(rANUENGAPID.Value)
	if ranUe == nil {
		logger.WithTrace(ctx, ran.Log).Error("No UE Context", zap.Int64("RanUeNgapID", rANUENGAPID.Value))
		cause := ngapType.Cause{
			Present: ngapType.CausePresentRadioNetwork,
			RadioNetwork: &ngapType.CauseRadioNetwork{
				Value: ngapType.CauseRadioNetworkPresentUnknownLocalUENGAPID,
			},
		}

		err := ran.NGAPSender.SendErrorIndication(ctx, &cause, nil)
		if err != nil {
			logger.WithTrace(ctx, ran.Log).Error("error sending error indication", zap.Error(err))
			return
		}

		logger.WithTrace(ctx, ran.Log).Info("sent error indication")

		return
	}

	logger.WithTrace(ctx, ran.Log).Debug("UE Context", zap.Int64("AmfUeNgapID", ranUe.AmfUeNgapID), zap.Int64("RanUeNgapID", ranUe.RanUeNgapID))
	ranUe.TouchLastSeen()

	pduSessionResourceModifyListModCfm := ngapType.PDUSessionResourceModifyListModCfm{}
	pduSessionResourceFailedToModifyListModCfm := ngapType.PDUSessionResourceFailedToModifyListModCfm{}

	err := ranUe.Radio.NGAPSender.SendPDUSessionResourceModifyConfirm(ctx, ranUe.AmfUeNgapID, ranUe.RanUeNgapID, pduSessionResourceModifyListModCfm, pduSessionResourceFailedToModifyListModCfm)
	if err != nil {
		logger.WithTrace(ctx, ranUe.Log).Error("error sending pdu session resource modify confirm", zap.Error(err))
		return
	}

	logger.WithTrace(ctx, ran.Log).Info("sent pdu session resource modify confirm")
}
