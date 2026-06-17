// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: Apache-2.0

package ngap

import (
	"context"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/amf/ngap/decode"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/free5gc/ngap/ngapType"
	"go.uber.org/zap"
)

func HandlePDUSessionResourceModifyIndication(ctx context.Context, ran *amf.Radio, msg decode.PDUSessionResourceModifyIndication) {
	ranUe, ok := resolveUE(ctx, ran, &msg.RANUENGAPID, &msg.AMFUENGAPID)
	if !ok {
		return
	}

	logger.WithTrace(ctx, ranUe.Log).Debug("UE Context", zap.Int64("AmfUeNgapID", ranUe.AmfUeNgapID), zap.Int64("RanUeNgapID", ranUe.RanUeNgapID))
	ranUe.TouchLastSeen()

	pduSessionResourceModifyListModCfm := ngapType.PDUSessionResourceModifyListModCfm{}
	pduSessionResourceFailedToModifyListModCfm := ngapType.PDUSessionResourceFailedToModifyListModCfm{}

	err := ranUe.SendPDUSessionResourceModifyConfirm(ctx, pduSessionResourceModifyListModCfm, pduSessionResourceFailedToModifyListModCfm)
	if err != nil {
		logger.WithTrace(ctx, ranUe.Log).Error("error sending pdu session resource modify confirm", zap.Error(err))
		return
	}
}
