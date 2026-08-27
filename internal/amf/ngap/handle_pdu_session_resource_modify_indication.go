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

// HandlePDUSessionResourceModifyIndication forwards each Modify Indication Transfer
// to its SMF and returns the Modify Confirm; sessions the SMF cannot modify go in
// the Failed to Modify list with a cause (TS 38.413 §8.2.5.2).
func HandlePDUSessionResourceModifyIndication(ctx context.Context, amfInstance *amf.AMF, ran *amf.Radio, msg *ngap.PDUSessionResourceModifyIndication) {
	ueConn, ok := resolveUE(ctx, amfInstance, ran, msg.AMFUENGAPID, msg.RANUENGAPID)
	if !ok {
		return
	}

	if msg.UserLocationInformation != nil {
		ueConn.UpdateLocation(ctx, *msg.UserLocationInformation)
	}

	logger.WithTrace(ctx, ueConn.Log()).Debug("UE Context", zap.Uint64("amf-ue-id", uint64(ueConn.AmfUeNgapID)), zap.Uint32("ran-ue-id", uint32(ueConn.RanUeNgapID)))
	ueConn.TouchLastSeen()

	amfUe := ueConn.UeContext()
	if amfUe == nil {
		logger.WithTrace(ctx, ueConn.Log()).Error("UeContext is nil")
		return
	}

	var (
		modifyList ngap.PDUSessionResourceModifyListModCfm
		failedList ngap.PDUSessionResourceFailedToModifyListModCfm
	)

	for _, item := range msg.PDUSessionResourceModify {
		pduSessionID := uint8(item.PDUSessionID)

		smContext, ok := amfUe.SmContextFindByPDUSessionID(pduSessionID)
		if !ok {
			logger.WithTrace(ctx, ueConn.Log()).Error("SmContext not found", zap.Uint8("PduSessionID", pduSessionID))
			failedList = appendFailedToModify(ctx, ueConn, failedList, item.PDUSessionID, ngap.CauseRadioNetworkUnknownPDUSessionID)

			continue
		}

		confirmTransfer, err := amfInstance.Session.UpdateSmContextN2ModifyIndication(ctx, smContext.Ref, []byte(item.Transfer))
		if err != nil {
			logger.WithTrace(ctx, ueConn.Log()).Error("UpdateSmContextN2ModifyIndication error", zap.Error(err), zap.Uint8("PduSessionID", pduSessionID))
			failedList = appendFailedToModify(ctx, ueConn, failedList, item.PDUSessionID, ngap.CauseRadioNetworkUnspecified)

			continue
		}

		modifyList = append(modifyList, ngap.PDUSessionResourceModifyItemModCfm{
			PDUSessionID: item.PDUSessionID,
			Transfer:     ngap.TransferContainer(confirmTransfer),
		})
	}

	confirm := &ngap.PDUSessionResourceModifyConfirm{
		AMFUENGAPID:              ngap.Ptr(ngap.AMFUENGAPID(ueConn.AmfUeNgapID)),
		RANUENGAPID:              ngap.Ptr(ngap.RANUENGAPID(ueConn.RanUeNgapID)),
		PDUSessionResourceModify: modifyList,
		PDUSessionResourceFailed: failedList,
	}

	// §10.3.4.2 reports a not-comprehended notify-criticality IE in the response
	// message of the procedure where it defines one, which this procedure does
	// (§9.2.1.9 gives the confirm a Criticality Diagnostics IE).
	if diag := msg.Diagnostics(); diag.ReportRequired() {
		confirm.CriticalityDiagnostics = &ngap.CriticalityDiagnostics{
			ProcedureCriticality:      ngap.Ptr(ngap.ProcedureCriticality(ngap.ProcPDUSessionResourceModifyIndication)),
			IEsCriticalityDiagnostics: diag.Report(),
		}
	}

	pkt, err := confirm.Marshal()
	if err != nil {
		logger.WithTrace(ctx, ueConn.Log()).Error("error building pdu session resource modify confirm", zap.Error(err))
		return
	}

	ran.SendToRadio(ctx, amf.NGAPProcedurePDUSessionResourceModifyConfirm, pkt)
}

// appendFailedToModify records a session the AMF could not hand to the SMF,
// carrying the reason in a Modify Indication Unsuccessful Transfer
// (TS 38.413 §9.3.4.22).
func appendFailedToModify(ctx context.Context, ueConn *amf.UeConn, list ngap.PDUSessionResourceFailedToModifyListModCfm, pduSessionID ngap.PDUSessionID, causeValue int) ngap.PDUSessionResourceFailedToModifyListModCfm {
	t := &ngap.PDUSessionResourceModifyIndicationUnsuccessfulTransfer{
		Cause: ngap.Cause{Group: ngap.CauseGroupRadioNetwork, Value: causeValue},
	}

	transfer, err := t.Marshal()
	if err != nil {
		logger.WithTrace(ctx, ueConn.Log()).Error("encode modify indication unsuccessful transfer", zap.Error(err))
		return list
	}

	return append(list, ngap.PDUSessionResourceFailedToModifyItemModCfm{
		PDUSessionID: pduSessionID,
		Transfer:     transfer,
	})
}
