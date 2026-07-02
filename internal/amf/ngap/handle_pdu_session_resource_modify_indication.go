// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"context"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/amf/ngap/decode"
	"github.com/ellanetworks/core/internal/amf/ngap/send"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/free5gc/aper"
	"github.com/free5gc/ngap/ngapType"
	"go.uber.org/zap"
)

// HandlePDUSessionResourceModifyIndication forwards each indicated PDU session's
// Modify Indication Transfer to its SMF and returns the resulting Modify Confirm
// to the NG-RAN (TS 38.413 §8.2.5.2). Sessions the SMF cannot modify go in the
// Failed to Modify list with a cause.
func HandlePDUSessionResourceModifyIndication(ctx context.Context, amfInstance *amf.AMF, ran *amf.Radio, msg decode.PDUSessionResourceModifyIndication) {
	ranUe, ok := resolveUE(ctx, ran, &msg.RANUENGAPID, &msg.AMFUENGAPID)
	if !ok {
		return
	}

	logger.WithTrace(ctx, ranUe.Log).Debug("UE Context", zap.Int64("AmfUeNgapID", ranUe.AmfUeNgapID), zap.Int64("RanUeNgapID", ranUe.RanUeNgapID))
	ranUe.TouchLastSeen()

	amfUe := ranUe.UeContext()
	if amfUe == nil {
		logger.WithTrace(ctx, ranUe.Log).Error("UeContext is nil")
		return
	}

	var (
		modifyList ngapType.PDUSessionResourceModifyListModCfm
		failedList ngapType.PDUSessionResourceFailedToModifyListModCfm
	)

	for _, item := range msg.PDUSessionResourceItems {
		pduSessionID, ok := validPDUSessionID(item.PDUSessionID.Value)
		if !ok {
			logger.WithTrace(ctx, ranUe.Log).Error("invalid PDU session ID from gNB, skipping", zap.Int64("pduSessionID", item.PDUSessionID.Value))
			continue
		}

		smContext, ok := amfUe.SmContextFindByPDUSessionID(pduSessionID)
		if !ok {
			logger.WithTrace(ctx, ranUe.Log).Error("SmContext not found", zap.Uint8("PduSessionID", pduSessionID))
			appendFailedToModify(ctx, ranUe, &failedList, item.PDUSessionID, ngapType.CauseRadioNetworkPresentUnknownPDUSessionID)

			continue
		}

		confirmTransfer, err := amfInstance.Smf.UpdateSmContextN2ModifyIndication(ctx, smContext.Ref, item.PDUSessionResourceModifyIndicationTransfer)
		if err != nil {
			logger.WithTrace(ctx, ranUe.Log).Error("UpdateSmContextN2ModifyIndication error", zap.Error(err), zap.Uint8("PduSessionID", pduSessionID))
			appendFailedToModify(ctx, ranUe, &failedList, item.PDUSessionID, ngapType.CauseRadioNetworkPresentUnspecified)

			continue
		}

		modifyList.List = append(modifyList.List, ngapType.PDUSessionResourceModifyItemModCfm{
			PDUSessionID:                            item.PDUSessionID,
			PDUSessionResourceModifyConfirmTransfer: confirmTransfer,
		})
	}

	pkt, err := send.BuildPDUSessionResourceModifyConfirm(ranUe.AmfUeNgapID, ranUe.RanUeNgapID, modifyList, failedList)
	if err != nil {
		logger.WithTrace(ctx, ranUe.Log).Error("error building pdu session resource modify confirm", zap.Error(err))
		return
	}

	if err := ran.SendToRan(ctx, send.NGAPProcedurePDUSessionResourceModifyConfirm, pkt); err != nil {
		logger.WithTrace(ctx, ranUe.Log).Error("error sending pdu session resource modify confirm", zap.Error(err))
	}
}

// appendFailedToModify records a PDU session the AMF could not modify, carrying
// a Modify Indication Unsuccessful Transfer with the cause (TS 38.413 §8.2.5.2).
func appendFailedToModify(ctx context.Context, ranUe *amf.RanUe, list *ngapType.PDUSessionResourceFailedToModifyListModCfm, pduSessionID ngapType.PDUSessionID, causeValue aper.Enumerated) {
	transfer, err := aper.MarshalWithParams(ngapType.PDUSessionResourceModifyIndicationUnsuccessfulTransfer{
		Cause: ngapType.Cause{
			Present:      ngapType.CausePresentRadioNetwork,
			RadioNetwork: &ngapType.CauseRadioNetwork{Value: causeValue},
		},
	}, "valueExt")
	if err != nil {
		logger.WithTrace(ctx, ranUe.Log).Error("encode modify indication unsuccessful transfer", zap.Error(err))
		return
	}

	list.List = append(list.List, ngapType.PDUSessionResourceFailedToModifyItemModCfm{
		PDUSessionID: pduSessionID,
		PDUSessionResourceModifyIndicationUnsuccessfulTransfer: transfer,
	})
}
