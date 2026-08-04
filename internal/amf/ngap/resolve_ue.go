// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"context"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/ngap"
	"go.uber.org/zap"
)

// resolveUE looks up a UE context for a message whose two UE NGAP IDs are both
// mandatory with reject criticality, so the decoder guarantees both. Mirrors
// the MME's resolveUE.
func resolveUE(ctx context.Context, amfInstance *amf.AMF, ran *amf.Radio, amfID ngap.AMFUENGAPID, ranID ngap.RANUENGAPID) (*amf.UeConn, bool) {
	a, r := int64(amfID), int64(ranID)

	return resolveDecodedUE(ctx, amfInstance, ran, &r, &a)
}

// causeMissingUENGAPIDs answers a UE-associated message that omitted a UE NGAP
// ID: without it the AMF cannot address a UE context, so the procedure is
// rejected (TS 38.413 §10.3.5).
var causeMissingUENGAPIDs = ngap.Cause{Group: ngap.CauseGroupProtocol, Value: ngap.CauseProtocolAbstractSyntaxErrorReject}

// resolveUEIDs is resolveUE for a message whose UE NGAP IDs carry ignore
// criticality and may therefore be absent.
func resolveUEIDs(ctx context.Context, amfInstance *amf.AMF, ran *amf.Radio, amfID *ngap.AMFUENGAPID, ranID *ngap.RANUENGAPID) (*amf.UeConn, bool) {
	if amfID == nil || ranID == nil {
		logger.WithTrace(ctx, ran.Log).Warn("UE-associated NGAP message without both UE NGAP IDs")
		sendErrorIndication(ctx, ran, amfID, ranID, causeMissingUENGAPIDs)

		return nil, false
	}

	return resolveUE(ctx, amfInstance, ran, *amfID, *ranID)
}

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
func resolveDecodedUE(ctx context.Context, amfInstance *amf.AMF, ran *amf.Radio, ranID *int64, amfID *int64) (*amf.UeConn, bool) {
	if amfID != nil {
		ueConn := amfInstance.FindUEByAmfUeNgapID(ran, models.AmfUeNgapID(*amfID))
		if ueConn == nil {
			logger.WithTrace(ctx, ran.Log).Warn("Unknown local AMF-UE-NGAP-ID on this radio",
				zap.Uint64("amf-ue-id", uint64(*amfID)))
			sendUnknownLocalUEError(ctx, ran, amfID, ranID)

			return nil, false
		}

		if ranID != nil && ueConn.RanUeNgapID != models.RanUeNgapID(*ranID) {
			logger.WithTrace(ctx, ran.Log).Warn("Inconsistent remote RAN-UE-NGAP-ID",
				zap.Uint64("amf-ue-id", uint64(*amfID)),
				zap.Uint32("stored-ran-ue-id", uint32(ueConn.RanUeNgapID)),
				zap.Uint32("received-ran-ue-id", uint32(*ranID)))
			sendInconsistentRemoteUEError(ctx, ran, amfID, ranID)

			return nil, false
		}

		return ueConn, true
	}

	if ranID != nil {
		ueConn := amfInstance.FindUEByRanUeNgapID(ran, models.RanUeNgapID(*ranID))
		if ueConn == nil {
			logger.WithTrace(ctx, ran.Log).Warn("Unknown remote RAN-UE-NGAP-ID on this radio",
				zap.Uint32("ran-ue-id", uint32(*ranID)))
			sendInconsistentRemoteUEError(ctx, ran, amfID, ranID)

			return nil, false
		}

		return ueConn, true
	}

	return nil, false
}

// causeUnknownLocalUEID is Cause Radio Network "unknown-local-UE-NGAP-ID"
// (TS 38.413): an AMF UE NGAP ID this AMF does not hold.
var causeUnknownLocalUEID = ngap.Cause{
	Group: ngap.CauseGroupRadioNetwork, Value: ngap.CauseRadioNetworkUnknownLocalUENGAPID,
}

// causeInconsistentRemoteUEID is Cause Radio Network
// "inconsistent-remote-UE-NGAP-ID" (TS 38.413): a RAN UE NGAP ID that does not
// match the one stored against its AMF UE NGAP ID.
var causeInconsistentRemoteUEID = ngap.Cause{
	Group: ngap.CauseGroupRadioNetwork, Value: ngap.CauseRadioNetworkInconsistentRemoteUEID,
}

func sendUnknownLocalUEError(ctx context.Context, ran *amf.Radio, amfID, ranID *int64) {
	a, r := decodedUEIDs(amfID, ranID)
	sendErrorIndication(ctx, ran, a, r, causeUnknownLocalUEID)
}

func sendInconsistentRemoteUEError(ctx context.Context, ran *amf.Radio, amfID, ranID *int64) {
	a, r := decodedUEIDs(amfID, ranID)
	sendErrorIndication(ctx, ran, a, r, causeInconsistentRemoteUEID)
}

// decodedUEIDs converts the dispatcher's raw AP IDs into the library's identifier
// types, leaving an absent id absent so the Error Indication does not claim
// one the sender never gave (TS 38.413 §8.7.5.2).
func decodedUEIDs(amfID, ranID *int64) (*ngap.AMFUENGAPID, *ngap.RANUENGAPID) {
	var (
		a *ngap.AMFUENGAPID
		r *ngap.RANUENGAPID
	)

	if amfID != nil {
		a = ngap.Ptr(ngap.AMFUENGAPID(*amfID))
	}

	if ranID != nil {
		r = ngap.Ptr(ngap.RANUENGAPID(*ranID))
	}

	return a, r
}
