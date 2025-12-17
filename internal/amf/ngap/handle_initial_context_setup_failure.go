package ngap

import (
	ctxt "context"

	"github.com/ellanetworks/core/internal/amf/context"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/smf/pdusession"
	"github.com/free5gc/ngap/ngapType"
	"go.uber.org/zap"
)

func HandleInitialContextSetupFailure(ctx ctxt.Context, ran *context.AmfRan, message *ngapType.NGAPPDU) {
	if ran == nil {
		logger.AmfLog.Error("ran is nil")
		return
	}

	if message == nil {
		ran.Log.Error("NGAP Message is nil")
		return
	}

	unsuccessfulOutcome := message.UnsuccessfulOutcome
	if unsuccessfulOutcome == nil {
		ran.Log.Error("UnsuccessfulOutcome is nil")
		return
	}

	initialContextSetupFailure := unsuccessfulOutcome.Value.InitialContextSetupFailure
	if initialContextSetupFailure == nil {
		ran.Log.Error("InitialContextSetupFailure is nil")
		return
	}

	var aMFUENGAPID *ngapType.AMFUENGAPID
	var rANUENGAPID *ngapType.RANUENGAPID
	var pDUSessionResourceFailedToSetupList *ngapType.PDUSessionResourceFailedToSetupListCxtFail
	var cause *ngapType.Cause

	for _, ie := range initialContextSetupFailure.ProtocolIEs.List {
		switch ie.Id.Value {
		case ngapType.ProtocolIEIDAMFUENGAPID:
			aMFUENGAPID = ie.Value.AMFUENGAPID
			if aMFUENGAPID == nil {
				ran.Log.Warn("AmfUeNgapID is nil")
			}
		case ngapType.ProtocolIEIDRANUENGAPID:
			rANUENGAPID = ie.Value.RANUENGAPID
			if rANUENGAPID == nil {
				ran.Log.Warn("RanUeNgapID is nil")
			}
		case ngapType.ProtocolIEIDPDUSessionResourceFailedToSetupListCxtFail:
			pDUSessionResourceFailedToSetupList = ie.Value.PDUSessionResourceFailedToSetupListCxtFail
			if pDUSessionResourceFailedToSetupList == nil {
				ran.Log.Warn("PDUSessionResourceFailedToSetupList is nil")
			}
		case ngapType.ProtocolIEIDCause:
			cause = ie.Value.Cause
			if cause == nil {
				ran.Log.Warn("Cause is nil")
			}
		}
	}

	ran.Log.Warn("Initial Context Setup Failure received", zap.String("Cause", causeToString(*cause)))

	ranUe := ran.RanUeFindByRanUeNgapID(rANUENGAPID.Value)
	if ranUe == nil {
		ran.Log.Error("No UE Context", zap.Int64("RanUeNgapID", rANUENGAPID.Value))
		return
	}

	amfUe := ranUe.AmfUe
	if amfUe == nil {
		ran.Log.Error("amfUe is nil")
		return
	}

	if amfUe.T3550 != nil {
		amfUe.T3550.Stop()
		amfUe.T3550 = nil
		amfUe.State.Set(context.Deregistered)
		amfUe.ClearRegistrationRequestData()
	}

	if pDUSessionResourceFailedToSetupList != nil {
		ranUe.Log.Debug("Send PDUSessionResourceSetupUnsuccessfulTransfer to SMF")

		for _, item := range pDUSessionResourceFailedToSetupList.List {
			pduSessionID := uint8(item.PDUSessionID.Value)
			transfer := item.PDUSessionResourceSetupUnsuccessfulTransfer
			smContext, ok := amfUe.SmContextFindByPDUSessionID(pduSessionID)
			if !ok {
				ranUe.Log.Error("SmContext not found", zap.Uint8("PduSessionID", pduSessionID))
				continue
			}
			err := pdusession.UpdateSmContextN2InfoPduResSetupFail(smContext.SmContextRef(), transfer)
			if err != nil {
				ranUe.Log.Error("SendUpdateSmContextN2Info[PDUSessionResourceSetupUnsuccessfulTransfer] Error", zap.Error(err))
			}
		}
	}
}
