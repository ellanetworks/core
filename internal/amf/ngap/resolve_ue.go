package ngap

import (
	"context"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/free5gc/ngap/ngapType"
	"go.uber.org/zap"
)

// resolveUE looks up a UE context scoped to the sending radio, following
// TS 38.413 clause 10.6 (Handling of AP ID).
//
// When both IDs are present it looks up by RAN-UE-NGAP-ID (scoped to the
// sender) and cross-checks AMF-UE-NGAP-ID. When only AMF-UE-NGAP-ID is
// present it looks up by that ID, still scoped to the sender.
//
// On any mismatch an Error Indication is sent to the sender and the
// function returns (nil, false).
func resolveUE(ctx context.Context, ran *amf.Radio, ranID *int64, amfID *int64) (*amf.RanUe, bool) {
	if ranID != nil {
		ranUe := ran.FindUEByRanUeNgapID(*ranID)
		if ranUe == nil {
			logger.WithTrace(ctx, ran.Log).Warn("No UE context for RAN-UE-NGAP-ID on this radio",
				zap.Int64("RanUeNgapID", *ranID))
			sendUnknownLocalUEError(ctx, ran)

			return nil, false
		}

		if amfID != nil && ranUe.AmfUeNgapID != *amfID {
			logger.WithTrace(ctx, ran.Log).Warn("Inconsistent AMF-UE-NGAP-ID",
				zap.Int64("RanUeNgapID", *ranID),
				zap.Int64("storedAmfUeNgapID", ranUe.AmfUeNgapID),
				zap.Int64("receivedAmfUeNgapID", *amfID))
			sendInconsistentRemoteUEError(ctx, ran)

			return nil, false
		}

		return ranUe, true
	}

	if amfID != nil {
		ranUe := ran.FindUEByAmfUeNgapID(*amfID)
		if ranUe == nil {
			logger.WithTrace(ctx, ran.Log).Warn("No UE context for AMF-UE-NGAP-ID on this radio",
				zap.Int64("AmfUeNgapID", *amfID))
			sendUnknownLocalUEError(ctx, ran)

			return nil, false
		}

		return ranUe, true
	}

	return nil, false
}

func sendUnknownLocalUEError(ctx context.Context, ran *amf.Radio) {
	cause := ngapType.Cause{
		Present: ngapType.CausePresentRadioNetwork,
		RadioNetwork: &ngapType.CauseRadioNetwork{
			Value: ngapType.CauseRadioNetworkPresentUnknownLocalUENGAPID,
		},
	}

	err := ran.NGAPSender.SendErrorIndication(ctx, &cause, nil)
	if err != nil {
		logger.WithTrace(ctx, ran.Log).Error("error sending error indication", zap.Error(err))
	}
}

func sendInconsistentRemoteUEError(ctx context.Context, ran *amf.Radio) {
	cause := ngapType.Cause{
		Present: ngapType.CausePresentRadioNetwork,
		RadioNetwork: &ngapType.CauseRadioNetwork{
			Value: ngapType.CauseRadioNetworkPresentInconsistentRemoteUENGAPID,
		},
	}

	err := ran.NGAPSender.SendErrorIndication(ctx, &cause, nil)
	if err != nil {
		logger.WithTrace(ctx, ran.Log).Error("error sending error indication", zap.Error(err))
	}
}
