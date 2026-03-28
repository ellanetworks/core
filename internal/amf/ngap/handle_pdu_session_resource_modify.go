package ngap

import (
	"context"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/free5gc/ngap/ngapType"
	"go.uber.org/zap"
)

func HandlePDUSessionResourceNotify(ctx context.Context, amfInstance *amf.AMF, ran *amf.Radio, msg *ngapType.PDUSessionResourceNotify) {
	if msg == nil {
		logger.WithTrace(ctx, ran.Log).Error("NGAP Message is nil")
		return
	}

	var (
		aMFUENGAPID                       *ngapType.AMFUENGAPID
		rANUENGAPID                       *ngapType.RANUENGAPID
		pDUSessionResourceNotifyList      *ngapType.PDUSessionResourceNotifyList
		pDUSessionResourceReleasedListNot *ngapType.PDUSessionResourceReleasedListNot
		userLocationInformation           *ngapType.UserLocationInformation
	)

	for _, ie := range msg.ProtocolIEs.List {
		switch ie.Id.Value {
		case ngapType.ProtocolIEIDAMFUENGAPID:
			aMFUENGAPID = ie.Value.AMFUENGAPID // reject
		case ngapType.ProtocolIEIDRANUENGAPID:
			rANUENGAPID = ie.Value.RANUENGAPID // reject
		case ngapType.ProtocolIEIDPDUSessionResourceNotifyList: // reject
			pDUSessionResourceNotifyList = ie.Value.PDUSessionResourceNotifyList
			if pDUSessionResourceNotifyList == nil {
				logger.WithTrace(ctx, ran.Log).Error("pDUSessionResourceNotifyList is nil")
			}
		case ngapType.ProtocolIEIDPDUSessionResourceReleasedListNot: // ignore
			pDUSessionResourceReleasedListNot = ie.Value.PDUSessionResourceReleasedListNot
			if pDUSessionResourceReleasedListNot == nil {
				logger.WithTrace(ctx, ran.Log).Error("PDUSessionResourceReleasedListNot is nil")
			}
		case ngapType.ProtocolIEIDUserLocationInformation: // optional, ignore
			userLocationInformation = ie.Value.UserLocationInformation
			if userLocationInformation == nil {
				logger.WithTrace(ctx, ran.Log).Warn("userLocationInformation is nil [optional]")
			}
		}
	}

	if rANUENGAPID == nil {
		logger.WithTrace(ctx, ran.Log).Error("RANUENGAPID IE (mandatory) is missing in PDUSessionResourceNotify")
		return
	}

	if aMFUENGAPID == nil {
		logger.WithTrace(ctx, ran.Log).Error("AMFUENGAPID IE (mandatory) is missing in PDUSessionResourceNotify")
		return
	}

	ranUe := ran.FindUEByRanUeNgapID(rANUENGAPID.Value)
	if ranUe == nil {
		logger.WithTrace(ctx, ran.Log).Error("No UE Context", zap.Int64("RanUeNgapID", rANUENGAPID.Value), zap.Int64("AmfUeNgapID", aMFUENGAPID.Value))
		return
	}

	ranUe.Radio = ran
	ranUe.TouchLastSeen()
	logger.WithTrace(ctx, ranUe.Log).Debug("Handle PDUSessionResourceNotify", zap.Int64("AmfUeNgapID", ranUe.AmfUeNgapID))

	amfUe := ranUe.AmfUe()
	if amfUe == nil {
		logger.WithTrace(ctx, ranUe.Log).Error("amfUe is nil")
		return
	}

	if userLocationInformation != nil {
		ranUe.UpdateLocation(ctx, amfInstance, userLocationInformation)
	}

	if pDUSessionResourceNotifyList != nil {
		// QoS flow-level notifications — forwarding to SMF is not yet implemented.
		logger.WithTrace(ctx, ranUe.Log).Warn("PDUSessionResourceNotifyList received but QoS flow notification forwarding is not implemented",
			zap.Int("sessions", len(pDUSessionResourceNotifyList.List)))
	}

	if pDUSessionResourceReleasedListNot != nil {
		for _, item := range pDUSessionResourceReleasedListNot.List {
			if item.PDUSessionID.Value < 1 || item.PDUSessionID.Value > 15 {
				logger.WithTrace(ctx, ranUe.Log).Error("invalid PDU session ID from gNB, skipping", zap.Int64("pduSessionID", item.PDUSessionID.Value))
				continue
			}

			pduSessionID := uint8(item.PDUSessionID.Value)

			smContext, ok := amfUe.SmContextFindByPDUSessionID(pduSessionID)
			if !ok {
				logger.WithTrace(ctx, ranUe.Log).Error("SmContext not found", zap.Uint8("PduSessionID", pduSessionID))
				continue
			}

			err := amfInstance.Smf.DeactivateSmContext(ctx, smContext.Ref)
			if err != nil {
				logger.WithTrace(ctx, ranUe.Log).Error("DeactivateSmContext failed", zap.Error(err), zap.Uint8("PduSessionID", pduSessionID))
				continue
			}

			amfUe.SetSmContextInactive(pduSessionID)

			logger.WithTrace(ctx, ranUe.Log).Info("deactivated PDU session released by gNB", zap.Uint8("PduSessionID", pduSessionID))
		}
	}
}
