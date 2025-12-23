package ngap

import (
	ctxt "context"

	"github.com/ellanetworks/core/internal/amf/context"
	"github.com/ellanetworks/core/internal/amf/ngap/message"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/free5gc/ngap/ngapType"
	"go.uber.org/zap"
)

func HandlePDUSessionResourceModifyIndication(ctx ctxt.Context, ran *context.AmfRan, msg *ngapType.NGAPPDU) {
	if ran == nil {
		logger.AmfLog.Error("ran is nil")
		return
	}

	if msg == nil {
		ran.Log.Error("NGAP Message is nil")
		return
	}

	initiatingMessage := msg.InitiatingMessage // reject
	if initiatingMessage == nil {
		ran.Log.Error("InitiatingMessage is nil")
		cause := ngapType.Cause{
			Present: ngapType.CausePresentProtocol,
			Protocol: &ngapType.CauseProtocol{
				Value: ngapType.CauseProtocolPresentAbstractSyntaxErrorReject,
			},
		}
		err := ran.NGAPSender.SendErrorIndication(ctx, nil, nil, &cause, nil)
		if err != nil {
			ran.Log.Error("error sending error indication", zap.Error(err))
		}
		ran.Log.Info("sent error indication")
		return
	}
	pDUSessionResourceModifyIndication := initiatingMessage.Value.PDUSessionResourceModifyIndication
	if pDUSessionResourceModifyIndication == nil {
		ran.Log.Error("PDUSessionResourceModifyIndication is nil")
		cause := ngapType.Cause{
			Present: ngapType.CausePresentProtocol,
			Protocol: &ngapType.CauseProtocol{
				Value: ngapType.CauseProtocolPresentAbstractSyntaxErrorReject,
			},
		}
		err := ran.NGAPSender.SendErrorIndication(ctx, nil, nil, &cause, nil)
		if err != nil {
			ran.Log.Error("error sending error indication", zap.Error(err))
		}
		ran.Log.Info("sent error indication")
		return
	}

	ran.Log.Info("handle PDU Session Resource Modify Indication")

	var aMFUENGAPID *ngapType.AMFUENGAPID
	var rANUENGAPID *ngapType.RANUENGAPID
	var pduSessionResourceModifyIndicationList *ngapType.PDUSessionResourceModifyListModInd
	var iesCriticalityDiagnostics ngapType.CriticalityDiagnosticsIEList

	for _, ie := range pDUSessionResourceModifyIndication.ProtocolIEs.List {
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
		procedureCode := ngapType.ProcedureCodePDUSessionResourceModifyIndication
		triggeringMessage := ngapType.TriggeringMessagePresentInitiatingMessage
		procedureCriticality := ngapType.CriticalityPresentReject
		criticalityDiagnostics := buildCriticalityDiagnostics(&procedureCode, &triggeringMessage, &procedureCriticality,
			&iesCriticalityDiagnostics)
		err := ran.NGAPSender.SendErrorIndication(ctx, nil, nil, nil, &criticalityDiagnostics)
		if err != nil {
			ran.Log.Error("error sending error indication", zap.Error(err))
			return
		}
		ran.Log.Info("sent error indication")
		return
	}

	ranUe := ran.RanUeFindByRanUeNgapID(rANUENGAPID.Value)
	if ranUe == nil {
		ran.Log.Error("No UE Context", zap.Int64("RanUeNgapID", rANUENGAPID.Value))
		cause := ngapType.Cause{
			Present: ngapType.CausePresentRadioNetwork,
			RadioNetwork: &ngapType.CauseRadioNetwork{
				Value: ngapType.CauseRadioNetworkPresentUnknownLocalUENGAPID,
			},
		}
		err := ran.NGAPSender.SendErrorIndication(ctx, nil, nil, &cause, nil)
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

	err := message.SendPDUSessionResourceModifyConfirm(ctx, ranUe, pduSessionResourceModifyListModCfm, pduSessionResourceFailedToModifyListModCfm, nil)
	if err != nil {
		ranUe.Log.Error("error sending pdu session resource modify confirm", zap.Error(err))
		return
	}
	ran.Log.Info("sent pdu session resource modify confirm")
}
