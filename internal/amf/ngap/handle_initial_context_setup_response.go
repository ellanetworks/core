package ngap

import (
	"context"

	amfContext "github.com/ellanetworks/core/internal/amf/context"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/smf/pdusession"
	"github.com/free5gc/ngap/ngapType"
	"go.uber.org/zap"
)

func HandleInitialContextSetupResponse(ctx context.Context, ran *amfContext.Radio, msg *ngapType.InitialContextSetupResponse) {
	if msg == nil {
		logger.WithTrace(ctx, ran.Log).Error("NGAP Message is nil")
		return
	}

	var (
		aMFUENGAPID                         *ngapType.AMFUENGAPID
		rANUENGAPID                         *ngapType.RANUENGAPID
		pDUSessionResourceSetupResponseList *ngapType.PDUSessionResourceSetupListCxtRes
		pDUSessionResourceFailedToSetupList *ngapType.PDUSessionResourceFailedToSetupListCxtRes
		criticalityDiagnostics              *ngapType.CriticalityDiagnostics
	)

	for _, ie := range msg.ProtocolIEs.List {
		switch ie.Id.Value {
		case ngapType.ProtocolIEIDAMFUENGAPID:
			aMFUENGAPID = ie.Value.AMFUENGAPID
			if aMFUENGAPID == nil {
				logger.WithTrace(ctx, ran.Log).Warn("AmfUeNgapID is nil")
			}
		case ngapType.ProtocolIEIDRANUENGAPID:
			rANUENGAPID = ie.Value.RANUENGAPID
			if rANUENGAPID == nil {
				logger.WithTrace(ctx, ran.Log).Warn("RanUeNgapID is nil")
			}
		case ngapType.ProtocolIEIDPDUSessionResourceSetupListCxtRes:
			pDUSessionResourceSetupResponseList = ie.Value.PDUSessionResourceSetupListCxtRes
			if pDUSessionResourceSetupResponseList == nil {
				logger.WithTrace(ctx, ran.Log).Warn("PDUSessionResourceSetupResponseList is nil")
			}
		case ngapType.ProtocolIEIDPDUSessionResourceFailedToSetupListCxtRes:
			pDUSessionResourceFailedToSetupList = ie.Value.PDUSessionResourceFailedToSetupListCxtRes
			if pDUSessionResourceFailedToSetupList == nil {
				logger.WithTrace(ctx, ran.Log).Warn("PDUSessionResourceFailedToSetupList is nil")
			}
		case ngapType.ProtocolIEIDCriticalityDiagnostics:
			criticalityDiagnostics = ie.Value.CriticalityDiagnostics
			if criticalityDiagnostics == nil {
				logger.WithTrace(ctx, ran.Log).Warn("Criticality Diagnostics is nil")
			}
		}
	}

	if rANUENGAPID == nil {
		logger.WithTrace(ctx, ran.Log).Error("initial context setup response is missing RANUENGAPID")
		return
	}

	if aMFUENGAPID == nil {
		logger.WithTrace(ctx, ran.Log).Error("initial context setup response is missing AMFUENGAPID")
		return
	}

	ranUe := ran.FindUEByRanUeNgapID(rANUENGAPID.Value)
	if ranUe == nil {
		logger.WithTrace(ctx, ran.Log).Error("No UE Context", zap.Int64("RanUeNgapID", rANUENGAPID.Value), zap.Int64("AmfUeNgapID", aMFUENGAPID.Value))
		return
	}

	ranUe.TouchLastSeen()

	amfUe := ranUe.AmfUe
	if amfUe == nil {
		logger.WithTrace(ctx, ran.Log).Error("amfUe is nil")
		return
	}

	if pDUSessionResourceSetupResponseList != nil {
		logger.WithTrace(ctx, ranUe.Log).Debug("Send PDUSessionResourceSetupResponseTransfer to SMF")

		for _, item := range pDUSessionResourceSetupResponseList.List {
			if item.PDUSessionID.Value < 1 || item.PDUSessionID.Value > 15 {
				logger.WithTrace(ctx, ranUe.Log).Error("invalid PDU session ID from gNB, skipping", zap.Int64("pduSessionID", item.PDUSessionID.Value))
				continue
			}

			pduSessionID := uint8(item.PDUSessionID.Value)
			transfer := item.PDUSessionResourceSetupResponseTransfer

			smContext, ok := amfUe.SmContextFindByPDUSessionID(pduSessionID)
			if !ok {
				logger.WithTrace(ctx, ranUe.Log).Error("SmContext not found", zap.Uint8("PduSessionID", pduSessionID))
				return
			}

			err := pdusession.UpdateSmContextN2InfoPduResSetupRsp(ctx, smContext.Ref, transfer)
			if err != nil {
				logger.WithTrace(ctx, ranUe.Log).Error("SendUpdateSmContextN2Info[PDUSessionResourceSetupResponseTransfer] Error", zap.Error(err))
			}
		}
	}

	if pDUSessionResourceFailedToSetupList != nil {
		logger.WithTrace(ctx, ranUe.Log).Debug("Send PDUSessionResourceSetupUnsuccessfulTransfer to SMF")

		for _, item := range pDUSessionResourceFailedToSetupList.List {
			if item.PDUSessionID.Value < 1 || item.PDUSessionID.Value > 15 {
				logger.WithTrace(ctx, ranUe.Log).Error("invalid PDU session ID from gNB, skipping", zap.Int64("pduSessionID", item.PDUSessionID.Value))
				continue
			}

			pduSessionID := uint8(item.PDUSessionID.Value)
			transfer := item.PDUSessionResourceSetupUnsuccessfulTransfer

			smContext, ok := amfUe.SmContextFindByPDUSessionID(pduSessionID)
			if !ok {
				logger.WithTrace(ctx, ranUe.Log).Error("SmContext not found", zap.Uint8("PduSessionID", pduSessionID))
				return
			}

			err := pdusession.UpdateSmContextN2InfoPduResSetupFail(smContext.Ref, transfer)
			if err != nil {
				logger.WithTrace(ctx, ranUe.Log).Error("SendUpdateSmContextN2Info[PDUSessionResourceSetupUnsuccessfulTransfer] Error", zap.Error(err))
			}
		}
	}

	ranUe.RecvdInitialContextSetupResponse = true
}
