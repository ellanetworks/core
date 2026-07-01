// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"context"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/amf/ngap/decode"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/free5gc/ngap/ngapType"
	"go.uber.org/zap"
)

func HandleUEContextReleaseRequest(ctx context.Context, amfInstance *amf.AMF, ran *amf.Radio, msg decode.UEContextReleaseRequest) {
	ranUe, ok := resolveUE(ctx, ran, &msg.RANUENGAPID, &msg.AMFUENGAPID)
	if !ok {
		return
	}

	ranUe.TouchLastSeen()
	logger.WithTrace(ctx, ranUe.Log).Debug("Handle UE Context Release Request", zap.Int64("AmfUeNgapID", ranUe.AmfUeNgapID), zap.Int64("RanUeNgapID", ranUe.RanUeNgapID))

	causeGroup := ngapType.CausePresentRadioNetwork
	causeValue := ngapType.CauseRadioNetworkPresentUnspecified

	var err error

	if msg.Cause != nil {
		fields := []zap.Field{logger.Cause(causeToString(*msg.Cause))}
		if ranUe.UeContext() != nil {
			fields = append(fields, logger.SUPI(ranUe.UeContext().Supi().String()))
		}

		logger.WithTrace(ctx, ranUe.Log).Info("UE Context Release Cause", fields...)

		causeGroup, causeValue, err = getCause(msg.Cause)
		if err != nil {
			logger.WithTrace(ctx, ranUe.Log).Error("could not get cause group and value", zap.Error(err))
		}
	}

	amfUe := ranUe.UeContext()
	if amfUe != nil {
		if amfUe.State() == amf.Registered {
			logger.WithTrace(ctx, ranUe.Log).Info("Ue Context in GMM-Registered")

			if msg.PDUSessionResourceList != nil {
				for _, pduSessionReourceItem := range msg.PDUSessionResourceList {
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
						logger.WithTrace(ctx, ranUe.Log).Error("Send Update SmContextDeactivate UpCnxState Error", zap.Error(err), zap.Uint8("PduSessionID", pduSessionID))
					}
				}
			} else {
				logger.WithTrace(ctx, ranUe.Log).Info("Pdu Session IDs not received from gNB, Releasing the UE Context with SMF using local context")

				for _, sr := range amfUe.SmContextRefs() {
					if sr.Inactive {
						logger.WithTrace(ctx, ranUe.Log).Info("Pdu Session is inactive so not sending deactivate to SMF", logger.PDUSessionID(sr.PduSessionID))
						continue
					}

					err := amfInstance.Smf.DeactivateSmContext(ctx, sr.Ref)
					if err != nil {
						logger.WithTrace(ctx, ranUe.Log).Warn("Send Update SmContextDeactivate UpCnxState Error", zap.Error(err), zap.Uint8("PduSessionID", sr.PduSessionID))
					}
				}
			}
		} else {
			logger.WithTrace(ctx, ranUe.Log).Info("Ue Context in Non GMM-Registered")
			ranUe.ReleaseAction = amf.UeContextReleaseUeContext

			err := ranUe.SendUEContextReleaseCommand(ctx, causeGroup, causeValue)
			if err != nil {
				logger.WithTrace(ctx, ranUe.Log).Error("error sending ue context release command", zap.Error(err))
				return
			}

			for _, sr := range amfUe.SmContextRefs() {
				err := amfInstance.Smf.ReleaseSmContext(ctx, sr.Ref)
				if err != nil {
					logger.WithTrace(ctx, ranUe.Log).Error("error sending release sm context request", zap.Error(err), zap.Uint8("PduSessionID", sr.PduSessionID))
				}
			}

			return
		}
	}

	ranUe.ReleaseAction = amf.UeContextN2NormalRelease

	err = ranUe.SendUEContextReleaseCommand(ctx, causeGroup, causeValue)
	if err != nil {
		logger.WithTrace(ctx, ranUe.Log).Error("error sending ue context release command", zap.Error(err))
		return
	}
}
