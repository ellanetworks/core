package ngap

import (
	"context"

	amfContext "github.com/ellanetworks/core/internal/amf/context"
	"github.com/free5gc/ngap/ngapType"
	"go.uber.org/zap"
)

func HandlePDUSessionResourceModifyIndication(ctx context.Context, ran *amfContext.Radio, msg *ngapType.PDUSessionResourceModifyIndication) {
	if msg == nil {
		ran.Log.Error("NGAP Message is nil")
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
				ran.Log.Error("AmfUeNgapID is nil")

				item := buildCriticalityDiagnosticsIEItem(ngapType.ProtocolIEIDAMFUENGAPID)
				iesCriticalityDiagnostics.List = append(iesCriticalityDiagnostics.List, item)
			}
		case ngapType.ProtocolIEIDRANUENGAPID: // reject
			rANUENGAPID = ie.Value.RANUENGAPID
			if rANUENGAPID == nil {
				ran.Log.Error("RanUeNgapID is nil")

				item := buildCriticalityDiagnosticsIEItem(ngapType.ProtocolIEIDRANUENGAPID)
				iesCriticalityDiagnostics.List = append(iesCriticalityDiagnostics.List, item)
			}
		case ngapType.ProtocolIEIDPDUSessionResourceModifyListModInd: // reject
			pduSessionResourceModifyIndicationList = ie.Value.PDUSessionResourceModifyListModInd
			if pduSessionResourceModifyIndicationList == nil {
				ran.Log.Error("PDUSessionResourceModifyListModInd is nil")

				item := buildCriticalityDiagnosticsIEItem(ngapType.ProtocolIEIDPDUSessionResourceModifyListModInd)
				iesCriticalityDiagnostics.List = append(iesCriticalityDiagnostics.List, item)
			}
		}
	}

	if len(iesCriticalityDiagnostics.List) > 0 {
		ran.Log.Error("Has missing reject IE(s)")

		criticalityDiagnostics := buildCriticalityDiagnostics(ngapType.ProcedureCodePDUSessionResourceModifyIndication, ngapType.TriggeringMessagePresentInitiatingMessage, ngapType.CriticalityPresentReject, &iesCriticalityDiagnostics)

		err := ran.NGAPSender.SendErrorIndication(ctx, nil, &criticalityDiagnostics)
		if err != nil {
			ran.Log.Error("error sending error indication", zap.Error(err))
			return
		}

		ran.Log.Info("sent error indication")

		return
	}

	ranUe := ran.FindUEByRanUeNgapID(rANUENGAPID.Value)
	if ranUe == nil {
		ran.Log.Error("No UE Context", zap.Int64("RanUeNgapID", rANUENGAPID.Value))
		cause := ngapType.Cause{
			Present: ngapType.CausePresentRadioNetwork,
			RadioNetwork: &ngapType.CauseRadioNetwork{
				Value: ngapType.CauseRadioNetworkPresentUnknownLocalUENGAPID,
			},
		}

		err := ran.NGAPSender.SendErrorIndication(ctx, &cause, nil)
		if err != nil {
			ran.Log.Error("error sending error indication", zap.Error(err))
			return
		}

		ran.Log.Info("sent error indication")

		return
	}

	ran.Log.Debug("UE Context", zap.Int64("AmfUeNgapID", ranUe.AmfUeNgapID), zap.Int64("RanUeNgapID", ranUe.RanUeNgapID))

	pduSessionResourceModifyListModCfm := ngapType.PDUSessionResourceModifyListModCfm{}
	pduSessionResourceFailedToModifyListModCfm := ngapType.PDUSessionResourceFailedToModifyListModCfm{}

	err := ranUe.Radio.NGAPSender.SendPDUSessionResourceModifyConfirm(ctx, ranUe.AmfUeNgapID, ranUe.RanUeNgapID, pduSessionResourceModifyListModCfm, pduSessionResourceFailedToModifyListModCfm)
	if err != nil {
		ranUe.Log.Error("error sending pdu session resource modify confirm", zap.Error(err))
		return
	}

	ran.Log.Info("sent pdu session resource modify confirm")
}
