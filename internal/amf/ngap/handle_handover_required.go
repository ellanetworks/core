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

		sourceUe.SendHandoverPreparationFailure(ctx, ngap.Cause{Group: ngap.CauseGroupNAS, Value: ngap.CauseNASAuthenticationFailure}, nil, nil)

		return
	}

	// Only intra5gs reaches an NG-RAN target. fivegs-to-eps needs the NAS
	// security parameters HANDOVER COMMAND makes conditional on it (§9.2.3.2
	// iftoEPSUTRA) and which this AMF does not derive, and eps-to-5gs describes
	// an eNB source. Either would run preparation to completion and then fail to
	// encode the command, leaving the source waiting on TNGRELOCprep.
	if msg.HandoverType != ngap.HandoverTypeIntra5GS {
		logger.WithTrace(ctx, sourceUe.Log).Info("handle Handover Preparation Failure [unsupported Handover Type]",
			zap.Uint8("handoverType", uint8(msg.HandoverType)))

		sourceUe.SendHandoverPreparationFailure(ctx, ngap.Cause{Group: ngap.CauseGroupRadioNetwork, Value: ngap.CauseRadioNetworkHoTargetNotAllowed}, nil, nil)

		return
	}

	targetRanNodeID := util.RANNodeIDToModels(msg.TargetID.TargetRANNodeID.GlobalRANNodeID)

	targetRan, ok := amfInstance.FindRadioByRanID(targetRanNodeID)
	if !ok {
		// The target gNB is not served by this AMF, so fail preparation explicitly and
		// leave the source not waiting (TS 38.413).
		logger.WithTrace(ctx, sourceUe.Log).Info("handle Handover Preparation Failure [Unknown Target ID]", zap.Any("targetRanNodeID", targetRanNodeID))

		sourceUe.SendHandoverPreparationFailure(ctx, ngap.Cause{Group: ngap.CauseGroupRadioNetwork, Value: ngap.CauseRadioNetworkUnknownTargetID}, nil, nil)

		return
	}

	if targetRan.Conn == ran.Conn {
		// A HANDOVER REQUIRED targeting the source gNB itself: intra-node mobility is
		// handled in the RAN and never reaches the core, so reject it (TS 38.413).
		logger.WithTrace(ctx, sourceUe.Log).Info("handle Handover Preparation Failure [target gNB is the source]")

		sourceUe.SendHandoverPreparationFailure(ctx, ngap.Cause{Group: ngap.CauseGroupRadioNetwork, Value: ngap.CauseRadioNetworkHoTargetNotAllowed}, nil, nil)

		return
	}

	sourceUe.HandOverType = msg.HandoverType

	// Every PDU session the source listed is a candidate, whether or not the 5GC can
	// offer it to the target: a candidate the target does not admit is reported to
	// the source in the HANDOVER COMMAND's to-release list, and one the 5GC could
	// not even offer is reported with the reason it failed here (TS 38.413 §8.4.1.2,
	// TS 23.502 §4.9.1.3.2 step 12).
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
			notOffered(item.PDUSessionID, causeHoFailureInTarget)

			continue
		}

		setupItem, err := amf.PDUSessionSetupItemHOReq(pduSessionID, smContext.Snssai, n2Rsp)
		if err != nil {
			logger.WithTrace(ctx, sourceUe.Log).Error("could not build the handover request item", zap.Error(err), zap.Uint8("PduSessionID", pduSessionID))
			notOffered(item.PDUSessionID, causeHoFailureInTarget)

			continue
		}

		sessions = append(sessions, setupItem)
		// No Cause: the session reaches the target, which answers for it.
		candidates = append(candidates, amf.HandoverCandidate{PDUSessionID: item.PDUSessionID})
	}

	if len(sessions) == 0 {
		logger.WithTrace(ctx, sourceUe.Log).Info("handle Handover Preparation Failure [HoFailure In Target5GC NgranNode Or TargetSystem]")

		sourceUe.SendHandoverPreparationFailure(ctx, causeHoFailureInTarget, nil, nil)

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
		sourceUe.SendHandoverPreparationFailure(ctx, causeHoFailureInTarget, nil, nil)

		return
	}

	// A HANDOVER REQUIRED with no Cause is delivered anyway: the IE is ignore
	// criticality, so §10.3.5 has the AMF carry on without it. The relayed Cause is
	// mandatory in the HANDOVER REQUEST, so an absent one becomes unspecified.
	cause := ngap.Cause{Group: ngap.CauseGroupRadioNetwork, Value: ngap.CauseRadioNetworkUnspecified}
	if msg.Cause != nil {
		cause = *msg.Cause
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
		cause,
		sessions,
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

		sourceUe.SendHandoverPreparationFailure(ctx, causeHoFailureInTarget, nil, nil)

		return
	}

	// Arm the guard only now the HANDOVER REQUEST is sent, so the timer can never race
	// the outbound request (TS 38.413 §8.4).
	amfInstance.SuperviseHandover(amfUe, sourceUe, targetUe)
}

// causeHoFailureInTarget names the target side as the reason preparation could
// not complete: no session survived core-side preparation, or the AMF could not
// stage the target context (TS 38.413 §9.3.1.2).
var causeHoFailureInTarget = ngap.Cause{Group: ngap.CauseGroupRadioNetwork, Value: ngap.CauseRadioNetworkHOFailureInTarget}

// causeUnknownPDUSessionID reports a PDU session the source asked to hand over
// that the 5GC does not hold — an identity outside the assignable range, or one
// with no SM context (TS 38.413 §9.3.1.2). Path Switch answers the same condition
// with the same cause.
var causeUnknownPDUSessionID = ngap.Cause{Group: ngap.CauseGroupRadioNetwork, Value: ngap.CauseRadioNetworkUnknownPDUSessionID}

// sendHandoverPreparationProtocolFailure reports a HANDOVER REQUIRED the AMF
// rejected on criticality grounds, using the procedure's own unsuccessful
// outcome as §10.3.4.2 requires. It is sent on the radio rather than through a
// UeConn: the message is rejected before any UE is resolved, and the UE NGAP
// IDs the reply needs come from the rejected message itself.
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
