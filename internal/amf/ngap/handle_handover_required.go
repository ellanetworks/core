// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"context"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/amf/util"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/ngap"
	"go.uber.org/zap"
)

func HandleHandoverRequired(ctx context.Context, amfInstance *amf.AMF, ran *amf.Radio, msg *ngap.HandoverRequired) {
	sourceUe, ok := resolveUE(ctx, amfInstance, ran, msg.AMFUENGAPID, msg.RANUENGAPID)
	if !ok {
		return
	}

	amfUe := sourceUe.UeContext()
	if amfUe == nil {
		logger.WithTrace(ctx, sourceUe.Log).Error("Cannot find amfUE from sourceUE")
		return
	}

	sourceUe.TouchLastSeen()

	conn := amfUe.Conn()
	if conn == nil {
		logger.WithTrace(ctx, sourceUe.Log).Error("no active NAS connection")
		return
	}

	if !amfUe.SecurityContextIsValid() {
		logger.WithTrace(ctx, sourceUe.Log).Info("handle Handover Preparation Failure [Authentication Failure]")

		sourceUe.SendHandoverPreparationFailure(ctx, causeHandoverNoSecurity, nil, nil)

		return
	}

	if msg.HandoverType != ngap.HandoverTypeIntra5GS {
		logger.WithTrace(ctx, sourceUe.Log).Info("handle Handover Preparation Failure [unsupported Handover Type]",
			zap.Uint8("handoverType", uint8(msg.HandoverType)))

		sourceUe.SendHandoverPreparationFailure(ctx, causeHOTargetNotAllowed, nil, nil)

		return
	}

	// TS 38.413 §9.2.3.1 pairs the Target ID alternative with the Handover Type,
	// but the ASN.1 does not, so a targeteNB-ID can arrive under intra5gs. It
	// names a target this AMF cannot reach either way.
	if msg.TargetID.TargetRANNodeID == nil {
		logger.WithTrace(ctx, sourceUe.Log).Info("handle Handover Preparation Failure [Target ID is not an NG-RAN node]")

		sourceUe.SendHandoverPreparationFailure(ctx, causeUnknownTargetID, nil, nil)

		return
	}

	targetRanNodeID := util.RANNodeIDToModels(msg.TargetID.TargetRANNodeID.GlobalRANNodeID)

	targetRan, ok := amfInstance.FindRadioByRanID(targetRanNodeID)
	if !ok {
		logger.WithTrace(ctx, sourceUe.Log).Info("handle Handover Preparation Failure [Unknown Target ID]", zap.Any("targetRanNodeID", targetRanNodeID))

		sourceUe.SendHandoverPreparationFailure(ctx, causeUnknownTargetID, nil, nil)

		return
	}

	if targetRan.Conn == ran.Conn {
		logger.WithTrace(ctx, sourceUe.Log).Info("handle Handover Preparation Failure [target gNB is the source]")

		sourceUe.SendHandoverPreparationFailure(ctx, causeHOTargetNotAllowed, nil, nil)

		return
	}

	sourceUe.HandOverType = msg.HandoverType

	var (
		sessions   ngap.PDUSessionResourceSetupListHOReq
		candidates []amf.HandoverCandidate
	)

	notOffered := func(pduSessionID ngap.PDUSessionID, cause ngap.Cause) {
		candidates = append(candidates, amf.HandoverCandidate{PDUSessionID: pduSessionID, Cause: &cause})
	}

	for _, item := range msg.PDUSessionResourceListHORqd {
		pduSessionID, ok := validPDUSessionID(int64(item.PDUSessionID))
		if !ok {
			logger.WithTrace(ctx, sourceUe.Log).Error("invalid PDU session ID from gNB, reporting it as not handed over", zap.Int64("pduSessionID", int64(item.PDUSessionID)))
			notOffered(item.PDUSessionID, causeUnknownPDUSessionID)

			continue
		}

		smContext, exist := amfUe.SmContextFindByPDUSessionID(pduSessionID)
		if !exist {
			logger.WithTrace(ctx, sourceUe.Log).Error("no SM context for a PDU session the gNB asked to hand over", zap.Uint8("pduSessionID", pduSessionID))
			notOffered(item.PDUSessionID, causeUnknownPDUSessionID)

			continue
		}

		n2Rsp, err := amfInstance.Session.UpdateSmContextN2HandoverPreparing(ctx, smContext.Ref, item.Transfer)
		if err != nil {
			logger.WithTrace(ctx, sourceUe.Log).Error("SendUpdateSmContextN2HandoverPreparing Error", zap.Error(err), zap.Uint8("PduSessionID", pduSessionID))
			notOffered(item.PDUSessionID, causeHandoverCNReason)

			continue
		}

		setupItem, err := amf.PDUSessionSetupItemHOReq(pduSessionID, smContext.Snssai, n2Rsp)
		if err != nil {
			logger.WithTrace(ctx, sourceUe.Log).Error("could not build the handover request item", zap.Error(err), zap.Uint8("PduSessionID", pduSessionID))
			notOffered(item.PDUSessionID, causeHandoverCNReason)

			continue
		}

		sessions = append(sessions, setupItem)
		candidates = append(candidates, amf.HandoverCandidate{PDUSessionID: item.PDUSessionID})
	}

	if len(sessions) == 0 {
		logger.WithTrace(ctx, sourceUe.Log).Info("handle Handover Preparation Failure [HoFailure In Target5GC NgranNode Or TargetSystem]")

		sourceUe.SendHandoverPreparationFailure(ctx, causeHOFailureInTarget, nil, nil)

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

	targetUe, nh, ncc, ok := amfInstance.PrepareHandover(ctx, amfUe, sourceUe, targetRan, candidates)
	if !ok {
		sourceUe.SendHandoverPreparationFailure(ctx, causeHOFailureInTarget, nil, nil)

		return
	}

	cause := causeHandoverPrepUnspecific
	if msg.Cause != nil {
		cause = *msg.Cause
	}

	err = targetUe.SendHandoverRequest(
		ctx,
		sourceUe.HandOverType,
		amfUe.Ambr.Uplink,
		amfUe.Ambr.Downlink,
		amfUe.UESecCap(),
		ncc,
		nh[:],
		cause,
		sessions,
		msg.SourceToTargetTransparentContainer,
		snssaiList,
		operatorInfo.Guami,
	)
	if err != nil {
		logger.WithTrace(ctx, sourceUe.Log).Error("error sending handover request to target UE", zap.Error(err))
		amfInstance.ClearHandover(amfUe)

		if rerr := amfInstance.RemoveUeConn(ctx, targetUe); rerr != nil {
			logger.WithTrace(ctx, sourceUe.Log).Error("error removing target ue after failed handover request", zap.Error(rerr))
		}

		sourceUe.SendHandoverPreparationFailure(ctx, causeHOFailureInTarget, nil, nil)

		return
	}

	amfInstance.SuperviseHandover(amfUe, sourceUe, targetUe)
}

func sendHandoverPreparationProtocolFailure(ctx context.Context, ran *amf.Radio, amfID ngap.AMFUENGAPID, ranID ngap.RANUENGAPID, ase *ngap.AbstractSyntaxError) {
	diagnostics := ase.OutcomeDiagnostics()

	b, err := (&ngap.HandoverPreparationFailure{
		AMFUENGAPID:            &amfID,
		RANUENGAPID:            &ranID,
		Cause:                  &ase.Cause,
		CriticalityDiagnostics: &diagnostics,
	}).Marshal()
	if err != nil {
		logger.WithTrace(ctx, ran.Log).Error("failed to marshal Handover Preparation Failure", zap.Error(err))
		return
	}

	ran.SendToRadio(ctx, amf.NGAPProcedureHandoverPreparationFailure, b)

	logger.WithTrace(ctx, ran.Log).Warn("Handover Preparation rejected", zap.Error(ase))
}
