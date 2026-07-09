// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"context"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/amf/ngap/send"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/models"
	"github.com/free5gc/ngap/ngapType"
	"go.uber.org/zap"
)

// resolveUE looks up a UE context on the sending radio per TS 38.413 (Handling
// of AP ID). At the AMF the local AP ID is the AMF UE NGAP ID and
// the remote AP ID is the RAN UE NGAP ID, so the connection is identified by the
// AMF UE NGAP ID and the RAN UE NGAP ID is then cross-checked.
//
//   - An AMF UE NGAP ID the AMF does not hold is an unknown local AP ID.
//   - A RAN UE NGAP ID different from the stored one is an inconsistent remote
//     AP ID.
//
// On either error an Error Indication carrying the received AP IDs is sent to
// the sender (TS 38.413) and the function returns (nil, false).
func resolveUE(ctx context.Context, amfInstance *amf.AMF, ran *amf.Radio, ranID *int64, amfID *int64) (*amf.UeConn, bool) {
	if amfID != nil {
		ueConn := amfInstance.FindUEByAmfUeNgapID(ran, models.AmfUeNgapID(*amfID))
		if ueConn == nil {
			logger.WithTrace(ctx, ran.Log).Warn("Unknown local AMF-UE-NGAP-ID on this radio",
				zap.Int64("AmfUeNgapID", *amfID))
			sendUnknownLocalUEError(ctx, ran, amfID, ranID)

			return nil, false
		}

		if ranID != nil && ueConn.RanUeNgapID != models.RanUeNgapID(*ranID) {
			logger.WithTrace(ctx, ran.Log).Warn("Inconsistent remote RAN-UE-NGAP-ID",
				zap.Int64("AmfUeNgapID", *amfID),
				zap.Int64("storedRanUeNgapID", int64(ueConn.RanUeNgapID)),
				zap.Int64("receivedRanUeNgapID", *ranID))
			sendInconsistentRemoteUEError(ctx, ran, amfID, ranID)

			return nil, false
		}

		return ueConn, true
	}

	if ranID != nil {
		ueConn := amfInstance.FindUEByRanUeNgapID(ran, models.RanUeNgapID(*ranID))
		if ueConn == nil {
			logger.WithTrace(ctx, ran.Log).Warn("Unknown remote RAN-UE-NGAP-ID on this radio",
				zap.Int64("RanUeNgapID", *ranID))
			sendInconsistentRemoteUEError(ctx, ran, amfID, ranID)

			return nil, false
		}

		return ueConn, true
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

	sendErrorIndication(ctx, ran, amfID, ranID, &cause)
}

func sendInconsistentRemoteUEError(ctx context.Context, ran *amf.Radio, amfID, ranID *int64) {
	cause := ngapType.Cause{
		Present: ngapType.CausePresentRadioNetwork,
		RadioNetwork: &ngapType.CauseRadioNetwork{
			Value: ngapType.CauseRadioNetworkPresentInconsistentRemoteUENGAPID,
		},
	}

	sendErrorIndication(ctx, ran, amfID, ranID, &cause)
}

func sendErrorIndication(ctx context.Context, ran *amf.Radio, amfID, ranID *int64, cause *ngapType.Cause) {
	pkt, err := send.BuildErrorIndication(amfID, ranID, cause, nil)
	if err != nil {
		logger.WithTrace(ctx, ran.Log).Error("error building error indication", zap.Error(err))
		return
	}

	ran.SendToRadio(ctx, send.NGAPProcedureErrorIndication, pkt)
}
