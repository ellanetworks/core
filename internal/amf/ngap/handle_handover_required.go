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
		sourceUe.Log(ctx).Error("Cannot find amfUE from sourceUE")
		return
	}

	sourceUe.TouchLastSeen()

	conn := amfUe.Conn()
	if conn == nil {
		sourceUe.Log(ctx).Error("no active NAS connection")
		return
	}

	if !amfUe.SecurityContextIsValid() {
		sourceUe.Log(ctx).Info("handle Handover Preparation Failure [Authentication Failure]")

		sourceUe.SendHandoverPreparationFailure(ctx, causeHandoverNoSecurity, nil, nil)

		return
	}

	if msg.HandoverType == ngap.HandoverTypeFiveGSToEPS {
		handoverRequiredToEPS(ctx, amfInstance, sourceUe, amfUe, msg)

		return
	}

	if msg.HandoverType != ngap.HandoverTypeIntra5GS {
		sourceUe.Log(ctx).Info("handle Handover Preparation Failure [unsupported Handover Type]",
			zap.Uint8("handover_type", uint8(msg.HandoverType)))

		sourceUe.SendHandoverPreparationFailure(ctx, causeHOTargetNotAllowed, nil, nil)

		return
	}

	if msg.TargetID.TargetRANNodeID == nil {
		sourceUe.Log(ctx).Info("handle Handover Preparation Failure [Target ID is not an NG-RAN node]")

		sourceUe.SendHandoverPreparationFailure(ctx, causeUnknownTargetID, nil, nil)

		return
	}

	targetRanNodeID, err := util.RANNodeIDToModels(msg.TargetID.TargetRANNodeID.GlobalRANNodeID)
	if err != nil {
		sourceUe.Log(ctx).Info("handle Handover Preparation Failure [Target ID cannot be decoded]", zap.Error(err))

		sourceUe.SendHandoverPreparationFailure(ctx, causeUnknownTargetID, nil, nil)

		return
	}

	targetRan, ok := amfInstance.FindConnectedRadioByRanID(targetRanNodeID)
	if !ok {
		sourceUe.Log(ctx).Info("handle Handover Preparation Failure [Unknown Target ID]", zap.Stringer("target_ran_node_id", targetRanNodeID))

		sourceUe.SendHandoverPreparationFailure(ctx, causeUnknownTargetID, nil, nil)

		return
	}

	if targetRan.Conn == ran.Conn {
		sourceUe.Log(ctx).Info("handle Handover Preparation Failure [target gNB is the source]")

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
			sourceUe.Log(ctx).Error("invalid PDU session ID from gNB, reporting it as not handed over", logger.PDUSessionID(uint8(item.PDUSessionID)))
			notOffered(item.PDUSessionID, causeUnknownPDUSessionID)

			continue
		}

		smContext, exist := amfUe.SmContextFindByPDUSessionID(pduSessionID)
		if !exist {
			sourceUe.Log(ctx).Error("no SM context for a PDU session the gNB asked to hand over", logger.PDUSessionID(pduSessionID))
			notOffered(item.PDUSessionID, causeUnknownPDUSessionID)

			continue
		}

		n2Rsp, err := amfInstance.Session.UpdateSmContextN2HandoverPreparing(ctx, smContext.Ref, item.Transfer)
		if err != nil {
			sourceUe.Log(ctx).Error("SendUpdateSmContextN2HandoverPreparing Error", zap.Error(err), logger.PDUSessionID(pduSessionID))
			notOffered(item.PDUSessionID, causeHandoverCNReason)

			continue
		}

		setupItem, err := amf.PDUSessionSetupItemHOReq(pduSessionID, smContext.Snssai, n2Rsp)
		if err != nil {
			sourceUe.Log(ctx).Error("could not build the handover request item", zap.Error(err), logger.PDUSessionID(pduSessionID))
			notOffered(item.PDUSessionID, causeHandoverCNReason)

			continue
		}

		sessions = append(sessions, setupItem)
		candidates = append(candidates, amf.HandoverCandidate{PDUSessionID: item.PDUSessionID})
	}

	if len(sessions) == 0 {
		sourceUe.Log(ctx).Info("handle Handover Preparation Failure [HoFailure In Target5GC NgranNode Or TargetSystem]")

		sourceUe.SendHandoverPreparationFailure(ctx, causeHOFailureInTarget, nil, nil)

		return
	}

	operatorInfo, err := amfInstance.OperatorInfo(ctx)
	if err != nil {
		sourceUe.Log(ctx).Error("Could not get operator info", zap.Error(err))
		return
	}

	snssaiList, err := amfInstance.ListOperatorSnssai(ctx)
	if err != nil {
		sourceUe.Log(ctx).Error("Could not list operator SNSSAI", zap.Error(err))
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

	err = targetUe.SendHandoverRequest(ctx, amf.HandoverRequestOpts{
		HandoverType:         sourceUe.HandOverType,
		UplinkAmbr:           amfUe.Ambr.Uplink,
		DownlinkAmbr:         amfUe.Ambr.Downlink,
		UESecurityCapability: amfUe.UESecCap(),
		NCC:                  ncc,
		NH:                   nh[:],
		Cause:                cause,
		Sessions:             sessions,
		SourceToTarget:       msg.SourceToTargetTransparentContainer,
		SnssaiList:           snssaiList,
		GUAMI:                operatorInfo.Guami,
		ServingPLMN:          operatorInfo.Guami.PlmnID,
	})
	if err != nil {
		sourceUe.Log(ctx).Error("error sending handover request to target UE", zap.Error(err))
		amfInstance.ClearHandover(amfUe)

		if rerr := amfInstance.RemoveUeConn(ctx, targetUe); rerr != nil {
			sourceUe.Log(ctx).Error("error removing target ue after failed handover request", zap.Error(rerr))
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
		ran.Log(ctx).Error("failed to marshal Handover Preparation Failure", zap.Error(err))
		return
	}

	ran.SendToRadio(ctx, amf.NGAPProcedureHandoverPreparationFailure, b)

	ran.Log(ctx).Warn("Handover Preparation rejected", zap.Error(ase))
}
