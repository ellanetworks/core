// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"context"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/amf/ngap/decode"
	"github.com/ellanetworks/core/internal/amf/ngap/send"
	"github.com/ellanetworks/core/internal/amf/util"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/free5gc/ngap/ngapType"
	"go.uber.org/zap"
)

func HandleHandoverRequired(ctx context.Context, amfInstance *amf.AMF, ran *amf.Radio, msg decode.HandoverRequired) {
	sourceUe, ok := resolveUE(ctx, amfInstance, ran, &msg.RANUENGAPID, &msg.AMFUENGAPID)
	if !ok {
		return
	}

	amfUe := sourceUe.UeContext()
	if amfUe == nil {
		logger.WithTrace(ctx, sourceUe.Log).Error("Cannot find amfUE from sourceUE")
		return
	}

	sourceUe.TouchLastSeen()

	if msg.TargetID.Present != ngapType.TargetIDPresentTargetRANNodeID {
		// A validly-decoded but unservable TargetID: fail preparation explicitly so the
		// source gNB does not wait out its own TNGRELOCprep timer (TS 38.413 §8.4.1.3).
		logger.WithTrace(ctx, sourceUe.Log).Info("handle Handover Preparation Failure [unsupported TargetID type]", zap.Int("targetID", msg.TargetID.Present))

		failureCause := ngapType.Cause{
			Present: ngapType.CausePresentRadioNetwork,
			RadioNetwork: &ngapType.CauseRadioNetwork{
				Value: ngapType.CauseRadioNetworkPresentHoTargetNotAllowed,
			},
		}

		if err := sourceUe.SendHandoverPreparationFailure(ctx, failureCause, nil); err != nil {
			logger.WithTrace(ctx, sourceUe.Log).Error("error sending handover preparation failure", zap.Error(err))
		}

		return
	}

	conn := amfUe.Conn()
	if conn == nil {
		logger.WithTrace(ctx, sourceUe.Log).Error("no active NAS connection")
		return
	}

	if !amfUe.SecurityContextIsValid() {
		logger.WithTrace(ctx, sourceUe.Log).Info("handle Handover Preparation Failure [Authentication Failure]")

		failureCause := ngapType.Cause{
			Present: ngapType.CausePresentNas,
			Nas: &ngapType.CauseNas{
				Value: ngapType.CauseNasPresentAuthenticationFailure,
			},
		}

		err := sourceUe.SendHandoverPreparationFailure(ctx, failureCause, nil)
		if err != nil {
			logger.WithTrace(ctx, sourceUe.Log).Error("error sending handover preparation failure", zap.Error(err))
			return
		}

		return
	}

	targetRanNodeID := util.RanIDToModels(msg.TargetID.TargetRANNodeID.GlobalRANNodeID)

	targetRan, ok := amfInstance.FindRadioByRanID(targetRanNodeID)
	if !ok {
		// The target gNB is not served by this AMF, so fail preparation explicitly and
		// leave the source not waiting (TS 38.413).
		logger.WithTrace(ctx, sourceUe.Log).Info("handle Handover Preparation Failure [Unknown Target ID]", zap.Any("targetRanNodeID", targetRanNodeID))

		failureCause := ngapType.Cause{
			Present: ngapType.CausePresentRadioNetwork,
			RadioNetwork: &ngapType.CauseRadioNetwork{
				Value: ngapType.CauseRadioNetworkPresentUnknownTargetID,
			},
		}

		if err := sourceUe.SendHandoverPreparationFailure(ctx, failureCause, nil); err != nil {
			logger.WithTrace(ctx, sourceUe.Log).Error("error sending handover preparation failure", zap.Error(err))
		}

		return
	}

	if targetRan.Conn == ran.Conn {
		// A HANDOVER REQUIRED targeting the source gNB itself: intra-node mobility is
		// handled in the RAN and never reaches the core, so reject it (TS 38.413).
		logger.WithTrace(ctx, sourceUe.Log).Info("handle Handover Preparation Failure [target gNB is the source]")

		failureCause := ngapType.Cause{
			Present: ngapType.CausePresentRadioNetwork,
			RadioNetwork: &ngapType.CauseRadioNetwork{
				Value: ngapType.CauseRadioNetworkPresentHoTargetNotAllowed,
			},
		}

		if err := sourceUe.SendHandoverPreparationFailure(ctx, failureCause, nil); err != nil {
			logger.WithTrace(ctx, sourceUe.Log).Error("error sending handover preparation failure", zap.Error(err))
		}

		return
	}

	sourceUe.HandOverType.Value = msg.HandoverType.Value

	var pduSessionReqList ngapType.PDUSessionResourceSetupListHOReq

	for _, pDUSessionResourceHoItem := range msg.PDUSessionResourceItems {
		pduSessionIDUint8, ok := validPDUSessionID(pDUSessionResourceHoItem.PDUSessionID.Value)
		if !ok {
			logger.WithTrace(ctx, sourceUe.Log).Error("invalid PDU session ID from gNB, skipping", zap.Int64("pduSessionID", pDUSessionResourceHoItem.PDUSessionID.Value))
			continue
		}

		if smContext, exist := amfUe.SmContextFindByPDUSessionID(pduSessionIDUint8); exist {
			n2Rsp, err := amfInstance.Session.UpdateSmContextN2HandoverPreparing(ctx, smContext.Ref, pDUSessionResourceHoItem.HandoverRequiredTransfer)
			if err != nil {
				logger.WithTrace(ctx, sourceUe.Log).Error("SendUpdateSmContextN2HandoverPreparing Error", zap.Error(err), zap.Uint8("PduSessionID", pduSessionIDUint8))
				continue
			}

			send.AppendPDUSessionResourceSetupListHOReq(&pduSessionReqList, pduSessionIDUint8, smContext.Snssai, n2Rsp)
		}
	}

	if len(pduSessionReqList.List) == 0 {
		logger.WithTrace(ctx, sourceUe.Log).Info("handle Handover Preparation Failure [HoFailure In Target5GC NgranNode Or TargetSystem]")

		failureCause := ngapType.Cause{
			Present: ngapType.CausePresentRadioNetwork,
			RadioNetwork: &ngapType.CauseRadioNetwork{
				Value: ngapType.CauseRadioNetworkPresentHoFailureInTarget5GCNgranNodeOrTargetSystem,
			},
		}

		err := sourceUe.SendHandoverPreparationFailure(ctx, failureCause, nil)
		if err != nil {
			logger.WithTrace(ctx, sourceUe.Log).Error("error sending handover preparation failure", zap.Error(err))
			return
		}

		return
	}

	operatorInfo, err := amfInstance.OperatorInfo(ctx)
	if err != nil {
		logger.WithTrace(ctx, sourceUe.Log).Error("Could not get operator info", zap.Error(err))
		return
	}

	snssaiList, err := amfInstance.ListOperatorSnssai(ctx)
	if err != nil {
		logger.WithTrace(ctx, sourceUe.Log).Error("Could not list operator SNSSAI", zap.Error(err))
		return
	}

	targetUe, nh, ncc, ok := amfInstance.PrepareHandover(ctx, amfUe, sourceUe, targetRan)
	if !ok {
		failureCause := ngapType.Cause{
			Present: ngapType.CausePresentRadioNetwork,
			RadioNetwork: &ngapType.CauseRadioNetwork{
				Value: ngapType.CauseRadioNetworkPresentHoFailureInTarget5GCNgranNodeOrTargetSystem,
			},
		}

		if err := sourceUe.SendHandoverPreparationFailure(ctx, failureCause, nil); err != nil {
			logger.WithTrace(ctx, sourceUe.Log).Error("error sending handover preparation failure", zap.Error(err))
		}

		return
	}

	// The HANDOVER REQUEST carries the AS key chain {NH, NCC} staged at preparation; it
	// is committed to the UE only when the UE reaches the target (NOTIFY).
	err = targetUe.SendHandoverRequest(
		ctx,
		sourceUe.HandOverType,
		amfUe.Ambr.Uplink,
		amfUe.Ambr.Downlink,
		amfUe.UESecCap(),
		ncc,
		nh[:],
		msg.Cause,
		pduSessionReqList,
		msg.SourceToTargetTransparentContainer,
		snssaiList,
		operatorInfo.Guami,
	)
	if err != nil {
		// The target never received the request, so it holds no context and is freed
		// locally; the source is failed so it does not wait out its own TNGRELOCprep
		// timer (TS 38.413 §8.4.1.3).
		logger.WithTrace(ctx, sourceUe.Log).Error("error sending handover request to target UE", zap.Error(err))
		amfInstance.ClearHandover(amfUe)

		if rerr := amfInstance.RemoveUeConn(ctx, targetUe); rerr != nil {
			logger.WithTrace(ctx, sourceUe.Log).Error("error removing target ue after failed handover request", zap.Error(rerr))
		}

		failureCause := ngapType.Cause{
			Present: ngapType.CausePresentRadioNetwork,
			RadioNetwork: &ngapType.CauseRadioNetwork{
				Value: ngapType.CauseRadioNetworkPresentHoFailureInTarget5GCNgranNodeOrTargetSystem,
			},
		}

		if ferr := sourceUe.SendHandoverPreparationFailure(ctx, failureCause, nil); ferr != nil {
			logger.WithTrace(ctx, sourceUe.Log).Error("error sending handover preparation failure", zap.Error(ferr))
		}

		return
	}

	// Arm the guard only now the HANDOVER REQUEST is sent, so the timer can never race
	// the outbound request (TS 38.413 §8.4).
	amfInstance.SuperviseHandover(amfUe, sourceUe, targetUe)
}
