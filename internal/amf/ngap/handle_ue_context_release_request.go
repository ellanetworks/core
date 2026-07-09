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
	ueConn, ok := resolveUE(ctx, amfInstance, ran, &msg.RANUENGAPID, &msg.AMFUENGAPID)
	if !ok {
		return
	}

	ueConn.TouchLastSeen()
	logger.WithTrace(ctx, ueConn.Log).Debug("Handle UE Context Release Request", zap.Int64("AmfUeNgapID", int64(ueConn.AmfUeNgapID)), zap.Int64("RanUeNgapID", int64(ueConn.RanUeNgapID)))

	causeGroup := ngapType.CausePresentRadioNetwork
	causeValue := ngapType.CauseRadioNetworkPresentUnspecified

	var err error

	if msg.Cause != nil {
		fields := []zap.Field{logger.Cause(causeToString(*msg.Cause))}
		if ueConn.UeContext() != nil {
			fields = append(fields, logger.SUPI(ueConn.UeContext().Supi().String()))
		}

		logger.WithTrace(ctx, ueConn.Log).Info("UE Context Release Cause", fields...)

		causeGroup, causeValue, err = getCause(msg.Cause)
		if err != nil {
			logger.WithTrace(ctx, ueConn.Log).Error("could not get cause group and value", zap.Error(err))
		}
	}

	amfUe := ueConn.UeContext()
	if amfUe != nil {
		if amfUe.State() == amf.Registered {
			logger.WithTrace(ctx, ueConn.Log).Info("Ue Context in GMM-Registered")

			if msg.PDUSessionResourceList != nil {
				for _, pduSessionReourceItem := range msg.PDUSessionResourceList {
					pduSessionID, ok := validPDUSessionID(pduSessionReourceItem.PDUSessionID.Value)
					if !ok {
						logger.WithTrace(ctx, ueConn.Log).Error("invalid PDU session ID from gNB, skipping", zap.Int64("pduSessionID", pduSessionReourceItem.PDUSessionID.Value))
						continue
					}

					smContext, ok := amfUe.SmContextFindByPDUSessionID(pduSessionID)
					if !ok {
						logger.WithTrace(ctx, ueConn.Log).Error("SmContext not found", zap.Uint8("PduSessionID", pduSessionID))
						continue
					}

					err := amfInstance.Session.DeactivateSmContext(ctx, smContext.Ref)
					if err != nil {
						logger.WithTrace(ctx, ueConn.Log).Error("Send Update SmContextDeactivate UpCnxState Error", zap.Error(err), zap.Uint8("PduSessionID", pduSessionID))
					}
				}
			} else {
				logger.WithTrace(ctx, ueConn.Log).Info("Pdu Session IDs not received from gNB, Releasing the UE Context with SMF using local context")

				for _, sr := range amfUe.SmContextRefs() {
					if sr.Inactive {
						logger.WithTrace(ctx, ueConn.Log).Info("Pdu Session is inactive so not sending deactivate to SMF", logger.PDUSessionID(sr.PduSessionID))
						continue
					}

					err := amfInstance.Session.DeactivateSmContext(ctx, sr.Ref)
					if err != nil {
						logger.WithTrace(ctx, ueConn.Log).Warn("Send Update SmContextDeactivate UpCnxState Error", zap.Error(err), zap.Uint8("PduSessionID", sr.PduSessionID))
					}
				}
			}
		} else {
			logger.WithTrace(ctx, ueConn.Log).Info("Ue Context in Non GMM-Registered")
			ueConn.ReleaseAction = amf.UeContextReleaseUeContext

			ueConn.SendUEContextReleaseCommand(ctx, causeGroup, causeValue)

			for _, sr := range amfUe.SmContextRefs() {
				err := amfInstance.Session.ReleaseSmContext(ctx, sr.Ref)
				if err != nil {
					logger.WithTrace(ctx, ueConn.Log).Error("error sending release sm context request", zap.Error(err), zap.Uint8("PduSessionID", sr.PduSessionID))
				}
			}

			return
		}
	}

	ueConn.ReleaseAction = amf.UeContextN2NormalRelease

	ueConn.SendUEContextReleaseCommand(ctx, causeGroup, causeValue)
}
