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

// HandlePDUSessionResourceModifyIndication forwards each Modify Indication Transfer
// to its SMF and returns the Modify Confirm; sessions the SMF cannot modify go in
// the Failed to Modify list with a cause (TS 38.413 §8.2.5.2).
func HandlePDUSessionResourceModifyIndication(ctx context.Context, amfInstance *amf.AMF, ran *amf.Radio, msg decode.PDUSessionResourceModifyIndication) {
	ueConn, ok := resolveUE(ctx, amfInstance, ran, &msg.RANUENGAPID, &msg.AMFUENGAPID)
	if !ok {
		return
	}

	logger.WithTrace(ctx, ueConn.Log).Debug("UE Context", zap.Int64("AmfUeNgapID", int64(ueConn.AmfUeNgapID)), zap.Int64("RanUeNgapID", int64(ueConn.RanUeNgapID)))
	ueConn.TouchLastSeen()

	amfUe := ueConn.UeContext()
	if amfUe == nil {
		logger.WithTrace(ctx, ueConn.Log).Error("UeContext is nil")
		return
	}

	var (
		modifyList ngapType.PDUSessionResourceModifyListModCfm
		failedList ngapType.PDUSessionResourceFailedToModifyListModCfm
	)

	for _, item := range msg.PDUSessionResourceItems {
		pduSessionID, ok := validPDUSessionID(item.PDUSessionID.Value)
		if !ok {
			logger.WithTrace(ctx, ueConn.Log).Error("invalid PDU session ID from gNB, skipping", zap.Int64("pduSessionID", item.PDUSessionID.Value))
			continue
		}

		smContext, ok := amfUe.SmContextFindByPDUSessionID(pduSessionID)
		if !ok {
			logger.WithTrace(ctx, ueConn.Log).Error("SmContext not found", zap.Uint8("PduSessionID", pduSessionID))
			appendFailedToModify(ctx, ueConn, &failedList, item.PDUSessionID, ngapType.CauseRadioNetworkPresentUnknownPDUSessionID)

			continue
		}

		confirmTransfer, err := amfInstance.Session.UpdateSmContextN2ModifyIndication(ctx, smContext.Ref, item.PDUSessionResourceModifyIndicationTransfer)
		if err != nil {
			logger.WithTrace(ctx, ueConn.Log).Error("UpdateSmContextN2ModifyIndication error", zap.Error(err), zap.Uint8("PduSessionID", pduSessionID))
			appendFailedToModify(ctx, ueConn, &failedList, item.PDUSessionID, ngapType.CauseRadioNetworkPresentUnspecified)

			continue
		}

		modifyList.List = append(modifyList.List, ngapType.PDUSessionResourceModifyItemModCfm{
			PDUSessionID:                            item.PDUSessionID,
			PDUSessionResourceModifyConfirmTransfer: confirmTransfer,
		})
	}

	pkt, err := send.BuildPDUSessionResourceModifyConfirm(int64(ueConn.AmfUeNgapID), int64(ueConn.RanUeNgapID), modifyList, failedList)
	if err != nil {
		logger.WithTrace(ctx, ueConn.Log).Error("error building pdu session resource modify confirm", zap.Error(err))
		return
	}

	ran.SendToRadio(ctx, send.NGAPProcedurePDUSessionResourceModifyConfirm, pkt)
}

func appendFailedToModify(ctx context.Context, ueConn *amf.UeConn, list *ngapType.PDUSessionResourceFailedToModifyListModCfm, pduSessionID ngapType.PDUSessionID, causeValue aper.Enumerated) {
	transfer, err := aper.MarshalWithParams(ngapType.PDUSessionResourceModifyIndicationUnsuccessfulTransfer{
		Cause: ngapType.Cause{
			Present:      ngapType.CausePresentRadioNetwork,
			RadioNetwork: &ngapType.CauseRadioNetwork{Value: causeValue},
		},
	}, "valueExt")
	if err != nil {
		logger.WithTrace(ctx, ueConn.Log).Error("encode modify indication unsuccessful transfer", zap.Error(err))
		return
	}

	list.List = append(list.List, ngapType.PDUSessionResourceFailedToModifyItemModCfm{
		PDUSessionID: pduSessionID,
		PDUSessionResourceModifyIndicationUnsuccessfulTransfer: transfer,
	})
}
