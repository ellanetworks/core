// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"context"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/free5gc/ngap/ngapType"
	"go.uber.org/zap"
)

// resolveUE looks up a UE context on the sending radio per TS 38.413 clause
// 10.6 (Handling of AP ID). At the AMF the local AP ID is the AMF UE NGAP ID and
// the remote AP ID is the RAN UE NGAP ID, so the connection is identified by the
// AMF UE NGAP ID and the RAN UE NGAP ID is then cross-checked.
//
//   - An AMF UE NGAP ID the AMF does not hold is an unknown local AP ID.
//   - A RAN UE NGAP ID different from the stored one is an inconsistent remote
//     AP ID.
//
// On either error an Error Indication carrying the received AP IDs is sent to
// the sender (clause 10.6, clause 8.7.5.2) and the function returns (nil, false).
func resolveUE(ctx context.Context, ran *amf.Radio, ranID *int64, amfID *int64) (*amf.RanUe, bool) {
	if amfID != nil {
		ranUe := ran.FindUEByAmfUeNgapID(*amfID)
		if ranUe == nil {
			logger.WithTrace(ctx, ran.Log).Warn("Unknown local AMF-UE-NGAP-ID on this radio",
				zap.Int64("AmfUeNgapID", *amfID))
			sendUnknownLocalUEError(ctx, ran, amfID, ranID)

			return nil, false
		}

		if ranID != nil && ranUe.RanUeNgapID != *ranID {
			logger.WithTrace(ctx, ran.Log).Warn("Inconsistent remote RAN-UE-NGAP-ID",
				zap.Int64("AmfUeNgapID", *amfID),
				zap.Int64("storedRanUeNgapID", ranUe.RanUeNgapID),
				zap.Int64("receivedRanUeNgapID", *ranID))
			sendInconsistentRemoteUEError(ctx, ran, amfID, ranID)

			return nil, false
		}

		return ranUe, true
	}

	if ranID != nil {
		ranUe := ran.FindUEByRanUeNgapID(*ranID)
		if ranUe == nil {
			logger.WithTrace(ctx, ran.Log).Warn("Unknown remote RAN-UE-NGAP-ID on this radio",
				zap.Int64("RanUeNgapID", *ranID))
			sendInconsistentRemoteUEError(ctx, ran, amfID, ranID)

			return nil, false
		}

		return ranUe, true
	}

	return nil, false
}

func sendUnknownLocalUEError(ctx context.Context, ran *amf.Radio, amfID, ranID *int64) {
	cause := ngapType.Cause{
		Present: ngapType.CausePresentRadioNetwork,
		RadioNetwork: &ngapType.CauseRadioNetwork{
			Value: ngapType.CauseRadioNetworkPresentUnknownLocalUENGAPID,
		},
	}

	err := ran.NGAPSender.SendErrorIndication(ctx, amfID, ranID, &cause, nil)
	if err != nil {
		logger.WithTrace(ctx, ran.Log).Error("error sending error indication", zap.Error(err))
	}
}

func sendInconsistentRemoteUEError(ctx context.Context, ran *amf.Radio, amfID, ranID *int64) {
	cause := ngapType.Cause{
		Present: ngapType.CausePresentRadioNetwork,
		RadioNetwork: &ngapType.CauseRadioNetwork{
			Value: ngapType.CauseRadioNetworkPresentInconsistentRemoteUENGAPID,
		},
	}

	err := ran.NGAPSender.SendErrorIndication(ctx, amfID, ranID, &cause, nil)
	if err != nil {
		logger.WithTrace(ctx, ran.Log).Error("error sending error indication", zap.Error(err))
	}
}
