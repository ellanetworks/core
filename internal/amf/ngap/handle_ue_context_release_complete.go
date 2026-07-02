// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"context"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/amf/ngap/decode"
	"github.com/ellanetworks/core/internal/logger"
	"go.uber.org/zap"
)

func HandleUEContextReleaseComplete(ctx context.Context, amfInstance *amf.AMF, ran *amf.Radio, msg decode.UEContextReleaseComplete) {
	if msg.AMFUENGAPID == nil {
		logger.WithTrace(ctx, ran.Log).Error("AMFUENGAPID IE (mandatory) is missing in UEContextReleaseComplete")
		return
	}

	if msg.RANUENGAPID == nil {
		logger.WithTrace(ctx, ran.Log).Error("RANUENGAPID IE (mandatory) is missing in UEContextReleaseComplete")
		return
	}

	ranUe, ok := resolveUE(ctx, ran, msg.RANUENGAPID, msg.AMFUENGAPID)
	if !ok {
		return
	}

	if msg.UserLocationInformation != nil {
		ranUe.UpdateLocation(ctx, amfInstance, msg.UserLocationInformation)
	}

	ranUe.TouchLastSeen()

	amfUe := ranUe.UeContext()
	if amfUe == nil {
		logger.WithTrace(ctx, ranUe.Log).Info("Release UE Context", zap.Int64("AmfUeNgapID", ranUe.AmfUeNgapID), zap.Int64("RanUeNgapID", *msg.RANUENGAPID))

		err := ranUe.Remove(ctx)
		if err != nil {
			logger.WithTrace(ctx, ranUe.Log).Error(err.Error())
		}

		return
	}

	if amfUe.State() == amf.Registered {
		logger.WithTrace(ctx, ranUe.Log).Debug("Release UE Context in GMM-Registered", logger.SUPI(amfUe.Supi().String()))

		if msg.PDUSessionResourceList != nil {
			for _, pduSessionReourceItem := range msg.PDUSessionResourceList.List {
				pduSessionID, ok := validPDUSessionID(pduSessionReourceItem.PDUSessionID.Value)
				if !ok {
					logger.WithTrace(ctx, ranUe.Log).Error("invalid PDU session ID from gNB, skipping", zap.Int64("pduSessionID", pduSessionReourceItem.PDUSessionID.Value))
					continue
				}

				smContext, ok := amfUe.SmContextFindByPDUSessionID(pduSessionID)
				if !ok {
					logger.WithTrace(ctx, ranUe.Log).Error("SmContext not found", zap.Uint8("PduSessionID", pduSessionID))
					continue
				}

				err := amfInstance.Smf.DeactivateSmContext(ctx, smContext.Ref)
				if err != nil {
					logger.WithTrace(ctx, ranUe.Log).Warn("Send Update SmContextDeactivate UpCnxState Error", zap.Error(err), zap.Uint8("PduSessionID", pduSessionID))
				}
			}
		} else {
			logger.WithTrace(ctx, ranUe.Log).Info("Pdu Session IDs not received from gNB, Releasing the UE Context with SMF using local context")

			for _, sr := range amfUe.SmContextRefs() {
				err := amfInstance.Smf.DeactivateSmContext(ctx, sr.Ref)
				if err != nil {
					logger.WithTrace(ctx, ranUe.Log).Warn("Send Update SmContextDeactivate UpCnxState Error", zap.Error(err), zap.Uint8("PduSessionID", sr.PduSessionID))
				}
			}
		}
	}

	if amfUe.State() == amf.Registered {
		amfUe.ResetMobileReachableTimer()
	}

	switch ranUe.ReleaseAction {
	case amf.UeContextN2NormalRelease:
		logger.WithTrace(ctx, ranUe.Log).Info("Release UE Context: N2 Connection Release", logger.SUPI(amfUe.Supi().String()))

		err := ranUe.Remove(ctx)
		if err != nil {
			logger.WithTrace(ctx, ranUe.Log).Error(err.Error())
		}
	case amf.UeContextReleaseUeContext:
		logger.WithTrace(ctx, ranUe.Log).Info("Release UE Context: Release Ue Context", logger.SUPI(amfUe.Supi().String()))

		err := ranUe.Remove(ctx)
		if err != nil {
			logger.WithTrace(ctx, ranUe.Log).Error(err.Error())
		}

		if !amfUe.Secured() {
			logger.WithTrace(ctx, ranUe.Log).Info("No valid security context for UE, deleting AMF UE context", logger.SUPI(amfUe.Supi().String()))
			amfInstance.DeregisterAndRemoveUeContext(ctx, amfUe)
		}
	case amf.UeContextReleaseDueToNwInitiatedDeregistraion:
		logger.WithTrace(ctx, ranUe.Log).Info("Release UE Context Due to Nw Initiated: Release Ue Context", logger.SUPI(amfUe.Supi().String()))

		err := ranUe.Remove(ctx)
		if err != nil {
			logger.WithTrace(ctx, ranUe.Log).Error(err.Error())
		}

		amfInstance.DeregisterAndRemoveUeContext(ctx, amfUe)
	case amf.UeContextReleaseHandover:
		// ranUe is the TARGET being released after a failed or cancelled handover;
		// the source remains the active RAN UE. A successful handover already moved
		// the UE to the target at HANDOVER NOTIFY and detached the source, so the
		// source's release takes the amfUe==nil early-return above and never reaches
		// here.
		logger.WithTrace(ctx, ranUe.Log).Info("Release target UE context after handover failure/cancel", logger.SUPI(amfUe.Supi().String()))

		amfInstance.ClearHandover(amfUe)

		err := ranUe.Remove(ctx)
		if err != nil {
			logger.WithTrace(ctx, ranUe.Log).Error(err.Error())
		}
	default:
		logger.WithTrace(ctx, ranUe.Log).Error("Invalid Release Action", zap.Any("ReleaseAction", ranUe.ReleaseAction))
	}
}
