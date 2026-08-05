// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"context"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/ngap"
	"go.uber.org/zap"
)

// causeReleaseUnspecified stands in for an omitted Cause IE (TS 38.413 §9.3.1.2,
// CauseRadioNetwork "unspecified").
var causeReleaseUnspecified = ngap.Cause{Group: ngap.CauseGroupRadioNetwork, Value: ngap.CauseRadioNetworkUnspecified}

// HandleUEContextReleaseRequest handles an NG-RAN-initiated UE Context Release
// Request (inactivity or radio-link failure), starting the release procedure
// (TS 38.413 §8.3.2).
func HandleUEContextReleaseRequest(ctx context.Context, amfInstance *amf.AMF, ran *amf.Radio, msg *ngap.UEContextReleaseRequest) {
	ueConn, ok := resolveUE(ctx, amfInstance, ran, msg.AMFUENGAPID, msg.RANUENGAPID)
	if !ok {
		return
	}

	reportDiagnostics(ctx, ran, ngap.ProcUEContextReleaseRequest, ngap.TriggeringInitiatingMessage, ueAssociated(msg.AMFUENGAPID, msg.RANUENGAPID), msg.Diagnostics())

	ueConn.TouchLastSeen()
	logger.WithTrace(ctx, ueConn.Log).Debug("Handle UE Context Release Request", zap.Uint64("amf-ue-id", uint64(ueConn.AmfUeNgapID)), zap.Uint32("ran-ue-id", uint32(ueConn.RanUeNgapID)))

	// An omitted Cause is an ignore-criticality absence: the NG-RAN node has
	// dropped the radio connection either way, so the release proceeds under a
	// generic cause (§10.3.5).
	cause := causeReleaseUnspecified

	if msg.Cause != nil {
		cause = *msg.Cause

		fields := []zap.Field{logger.Cause(cause.String())}
		if ueConn.UeContext() != nil {
			fields = append(fields, logger.SUPI(ueConn.UeContext().Supi().String()))
		}

		logger.WithTrace(ctx, ueConn.Log).Info("UE Context Release Cause", fields...)
	}

	amfUe := ueConn.UeContext()
	if amfUe != nil {
		if amfUe.State() == amf.Registered {
			logger.WithTrace(ctx, ueConn.Log).Info("Ue Context in GMM-Registered")

			if msg.PDUSessionResourceList != nil {
				for _, item := range msg.PDUSessionResourceList {
					pduSessionID := uint8(item.PDUSessionID)

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

			ueConn.SendUEContextReleaseCommand(ctx, cause)

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

	ueConn.SendUEContextReleaseCommand(ctx, cause)
}
