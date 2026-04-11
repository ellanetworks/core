package ngap

import (
	"context"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/amf/ngap/decode"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/free5gc/ngap/ngapType"
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

	ranUe := amfInstance.FindRanUeByAmfUeNgapID(*msg.AMFUENGAPID)
	if ranUe == nil {
		logger.WithTrace(ctx, ran.Log).Error("No RanUe Context", zap.Int64("AmfUeNgapID", *msg.AMFUENGAPID), zap.Int64("RanUeNgapID", *msg.RANUENGAPID))

		cause := ngapType.Cause{
			Present: ngapType.CausePresentRadioNetwork,
			RadioNetwork: &ngapType.CauseRadioNetwork{
				Value: ngapType.CauseRadioNetworkPresentUnknownLocalUENGAPID,
			},
		}

		err := ran.NGAPSender.SendErrorIndication(ctx, &cause, nil)
		if err != nil {
			logger.WithTrace(ctx, ran.Log).Error("error sending error indication", zap.Error(err))
			return
		}

		return
	}

	if msg.UserLocationInformation != nil {
		ranUe.UpdateLocation(ctx, amfInstance, msg.UserLocationInformation)
	}

	ranUe.Radio = ran
	ranUe.TouchLastSeen()

	amfUe := ranUe.AmfUe()
	if amfUe == nil {
		logger.WithTrace(ctx, ranUe.Log).Info("Release UE Context", zap.Int64("AmfUeNgapID", ranUe.AmfUeNgapID), zap.Int64("RanUeNgapID", *msg.RANUENGAPID))

		err := ranUe.Remove()
		if err != nil {
			logger.WithTrace(ctx, ranUe.Log).Error(err.Error())
		}

		return
	}

	if amfUe.GetState() == amf.Registered {
		logger.WithTrace(ctx, ranUe.Log).Debug("Release UE Context in GMM-Registered", logger.SUPI(amfUe.Supi.String()))

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

			amfUe.Mutex.Lock()

			type smCtxRef struct {
				ref string
				id  uint8
			}

			smContextRefs := make([]smCtxRef, 0, len(amfUe.SmContextList))
			for pduSessionID, smContext := range amfUe.SmContextList {
				smContextRefs = append(smContextRefs, smCtxRef{ref: smContext.Ref, id: pduSessionID})
			}

			amfUe.Mutex.Unlock()

			for _, sr := range smContextRefs {
				err := amfInstance.Smf.DeactivateSmContext(ctx, sr.ref)
				if err != nil {
					logger.WithTrace(ctx, ranUe.Log).Warn("Send Update SmContextDeactivate UpCnxState Error", zap.Error(err), zap.Uint8("PduSessionID", sr.id))
				}
			}
		}
	}

	if amfUe.GetState() == amf.Registered {
		amfUe.ResetMobileReachableTimer()
	}

	switch ranUe.ReleaseAction {
	case amf.UeContextN2NormalRelease:
		logger.WithTrace(ctx, ranUe.Log).Info("Release UE Context: N2 Connection Release", logger.SUPI(amfUe.Supi.String()))

		err := ranUe.Remove()
		if err != nil {
			logger.WithTrace(ctx, ranUe.Log).Error(err.Error())
		}
	case amf.UeContextReleaseUeContext:
		logger.WithTrace(ctx, ranUe.Log).Info("Release UE Context: Release Ue Context", logger.SUPI(amfUe.Supi.String()))

		err := ranUe.Remove()
		if err != nil {
			logger.WithTrace(ctx, ranUe.Log).Error(err.Error())
		}

		// No valid security context exists for this UE, so delete the AMF UE context
		if !amfUe.SecurityContextAvailable {
			logger.WithTrace(ctx, ranUe.Log).Info("No valid security context for UE, deleting AMF UE context", logger.SUPI(amfUe.Supi.String()))
			amfInstance.DeregisterAndRemoveAMFUE(ctx, amfUe)
		}
	case amf.UeContextReleaseDueToNwInitiatedDeregistraion:
		logger.WithTrace(ctx, ranUe.Log).Info("Release UE Context Due to Nw Initiated: Release Ue Context", logger.SUPI(amfUe.Supi.String()))

		err := ranUe.Remove()
		if err != nil {
			logger.WithTrace(ctx, ranUe.Log).Error(err.Error())
		}

		amfInstance.DeregisterAndRemoveAMFUE(ctx, amfUe)
	case amf.UeContextReleaseHandover:
		logger.WithTrace(ctx, ranUe.Log).Info("Release UE Context : Release for Handover", logger.SUPI(amfUe.Supi.String()))

		if ranUe.TargetUe != nil {
			// Success path: ranUe is the SOURCE being released after a
			// completed handover (HandoverNotify). Transfer the AMF UE
			// association to the target.
			targetRanUe := amfInstance.FindRanUeByAmfUeNgapID(ranUe.TargetUe.AmfUeNgapID)
			if targetRanUe == nil {
				logger.WithTrace(ctx, ranUe.Log).Error("target RAN UE not found during handover release",
					zap.Int64("targetAmfUeNgapID", ranUe.TargetUe.AmfUeNgapID))
			} else {
				targetRanUe.Radio = ran
				amfUe.AttachRanUe(targetRanUe)
			}
		} else {
			// Failure/cancel path: ranUe is the TARGET being released
			// after a failed or cancelled handover. The source UE
			// remains the active RAN UE — just clean up the target.
			logger.WithTrace(ctx, ranUe.Log).Info("Release target UE context after handover failure/cancel", logger.SUPI(amfUe.Supi.String()))
		}

		amf.DetachSourceUeTargetUe(ranUe)

		err := ranUe.Remove()
		if err != nil {
			logger.WithTrace(ctx, ranUe.Log).Error(err.Error())
		}
	default:
		logger.WithTrace(ctx, ranUe.Log).Error("Invalid Release Action", zap.Any("ReleaseAction", ranUe.ReleaseAction))
	}
}
