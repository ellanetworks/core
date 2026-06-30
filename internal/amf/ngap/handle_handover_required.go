// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"context"
	"time"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/amf/ngap/decode"
	"github.com/ellanetworks/core/internal/amf/ngap/send"
	"github.com/ellanetworks/core/internal/amf/procedure"
	"github.com/ellanetworks/core/internal/amf/util"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/models"
	"github.com/free5gc/ngap/ngapType"
	"go.uber.org/zap"
)

func HandleHandoverRequired(ctx context.Context, amfInstance *amf.AMF, ran *amf.Radio, msg decode.HandoverRequired) {
	sourceUe, ok := resolveUE(ctx, ran, &msg.RANUENGAPID, &msg.AMFUENGAPID)
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
		logger.WithTrace(ctx, sourceUe.Log).Error("targetID type is not supported", zap.Int("targetID", msg.TargetID.Present))
		return
	}

	conn := amfUe.NasConn()
	if conn == nil {
		logger.WithTrace(ctx, sourceUe.Log).Error("no active NAS connection")
		return
	}

	_, beginErr := conn.Procedures.Begin(conn.Ctx(), procedure.Procedure{Type: procedure.N2Handover})
	if beginErr != nil {
		logger.WithTrace(ctx, sourceUe.Log).Info("N2Handover rejected by procedure registry", zap.Error(beginErr))
		return
	}

	// Supervision is armed only once the target is engaged (see end of function).
	// Until then — and on every preparation-failure path — the procedure must not
	// be left active, or it would pin the UE with no deadline to clear it.
	armed := false

	defer func() {
		if !armed {
			conn.Procedures.End(procedure.N2Handover)
			amfUe.ClearHandover()
		}
	}()

	if !amfUe.SecurityContextIsValid() {
		logger.WithTrace(ctx, sourceUe.Log).Info("handle Handover Preparation Failure [Authentication Failure]")

		failureCause := ngapType.Cause{
			Present: ngapType.CausePresentNas,
			Nas: &ngapType.CauseNas{
				Value: ngapType.CauseNasPresentAuthenticationFailure,
			},
		}

		conn.Procedures.End(procedure.N2Handover)

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
		// The target gNB is not served by this AMF, so preparation cannot
		// proceed; fail it explicitly so the source is not left waiting
		// (TS 38.413 §8.4.1.3).
		logger.WithTrace(ctx, sourceUe.Log).Info("handle Handover Preparation Failure [Unknown Target ID]", zap.Any("targetRanNodeID", targetRanNodeID))

		failureCause := ngapType.Cause{
			Present: ngapType.CausePresentRadioNetwork,
			RadioNetwork: &ngapType.CauseRadioNetwork{
				Value: ngapType.CauseRadioNetworkPresentUnknownTargetID,
			},
		}

		conn.Procedures.End(procedure.N2Handover)

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
			n2Rsp, err := amfInstance.Smf.UpdateSmContextN2HandoverPreparing(ctx, smContext.Ref, pDUSessionResourceHoItem.HandoverRequiredTransfer)
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

		conn.Procedures.End(procedure.N2Handover)

		err := sourceUe.SendHandoverPreparationFailure(ctx, failureCause, nil)
		if err != nil {
			logger.WithTrace(ctx, sourceUe.Log).Error("error sending handover preparation failure", zap.Error(err))
			return
		}

		return
	}

	err := amfUe.UpdateNH()
	if err != nil {
		logger.WithTrace(ctx, sourceUe.Log).Error("error updating NH", zap.Error(err))
		return
	}

	operatorInfo, err := amfInstance.GetOperatorInfo(ctx)
	if err != nil {
		logger.WithTrace(ctx, sourceUe.Log).Error("Could not get operator info", zap.Error(err))
		return
	}

	snssaiList, err := amfInstance.ListOperatorSnssai(ctx)
	if err != nil {
		logger.WithTrace(ctx, sourceUe.Log).Error("Could not list operator SNSSAI", zap.Error(err))
		return
	}

	targetUe, err := amfInstance.NewRanUe(targetRan, models.RanUeNgapIDUnspecified)
	if err != nil {
		logger.WithTrace(ctx, sourceUe.Log).Error("error creating target ue", zap.Error(err))
		return
	}

	err = amf.AttachSourceUeTargetUe(sourceUe, targetUe)
	if err != nil {
		logger.WithTrace(ctx, sourceUe.Log).Error("attach source ue target ue error", zap.Error(err))
		return
	}

	nh, ncc := amfUe.NextHopNCC()

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
		logger.WithTrace(ctx, sourceUe.Log).Error("error sending handover request to target UE", zap.Error(err))
		return
	}

	// Bound the handover (HANDOVER REQUIRED → NOTIFY): if the target gNB never
	// completes it, the guard abandons the handover and releases the target.
	// Armed here, after the target is engaged, so its cleanup captures the target
	// directly and the timer goroutine has a happens-before edge to this setup.
	if supErr := conn.Procedures.Supervise(conn.Ctx(), procedure.N2Handover,
		time.Now().Add(amfInstance.HandoverGuardTimeout()),
		handoverGuardExpiry(sourceUe, targetUe)); supErr != nil {
		logger.WithTrace(ctx, sourceUe.Log).Warn("could not arm N2 handover guard", zap.Error(supErr))
	} else {
		armed = true
	}
}

// handoverGuardExpiry abandons a stalled N2 handover. The procedure registry runs
// it when the supervision deadline elapses (or the source NAS connection is
// cancelled) before HANDOVER NOTIFY arrives — the target gNB never completed the
// handover. The half-prepared target UE context is released; the source is left in
// place (its own TNGRELOCprep/Overall timers abort the handover on the radio),
// mirroring the MME's onHandoverGuardExpiry (TS 38.413 §8.4). A normal completion
// (HANDOVER NOTIFY/FAILURE/CANCEL) ends the procedure, which stops this timer
// before it can fire, so the captured target is touched by at most one goroutine.
func handoverGuardExpiry(sourceUe, targetUe *amf.RanUe) func(context.Context) error {
	return func(cctx context.Context) error {
		logger.WithTrace(cctx, sourceUe.Log).Warn("N2 handover abandoned: target gNB did not complete it in time, releasing target")

		sourceUe.UeContext().ClearHandover()

		targetUe.ReleaseAction = amf.UeContextReleaseHandover

		return targetUe.SendUEContextReleaseCommand(cctx,
			ngapType.CausePresentRadioNetwork,
			ngapType.CauseRadioNetworkPresentTngrelocoverallExpiry)
	}
}
