// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-FileCopyrightText: 2022-present Intel Corporation
// SPDX-FileCopyrightText: 2021 Open Networking Foundation <info@opennetworking.org>
// Copyright 2019 free5GC.org
// Modified by Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	gocontext "context"
	"encoding/hex"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/amf/ngap/decode"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/models"
	"go.uber.org/zap"
)

func HandleUERadioCapabilityInfoIndication(ctx gocontext.Context, ran *amf.Radio, msg decode.UERadioCapabilityInfoIndication) {
	ranUe, ok := resolveUE(ctx, ran, &msg.RANUENGAPID, &msg.AMFUENGAPID)
	if !ok {
		return
	}

	logger.WithTrace(ctx, ranUe.Log).Debug("Handle UE Radio Capability Info Indication", zap.Int64("RanUeNgapID", ranUe.RanUeNgapID), zap.Int64("AmfUeNgapID", ranUe.AmfUeNgapID))
	ranUe.TouchLastSeen()

	amfUe := ranUe.UeContext()
	if amfUe == nil {
		logger.WithTrace(ctx, ranUe.Log).Error("amfUe is nil")
		return
	}

	if msg.UERadioCapability != nil {
		amfUe.UeRadioCapability = hex.EncodeToString(msg.UERadioCapability)
	}

	if msg.UERadioCapabilityForPaging != nil {
		amfUe.UeRadioCapabilityForPaging = &models.UERadioCapabilityForPaging{}
		if msg.UERadioCapabilityForPaging.UERadioCapabilityForPagingOfNR != nil {
			amfUe.UeRadioCapabilityForPaging.NR = hex.EncodeToString(
				msg.UERadioCapabilityForPaging.UERadioCapabilityForPagingOfNR.Value)
		}

		if msg.UERadioCapabilityForPaging.UERadioCapabilityForPagingOfEUTRA != nil {
			amfUe.UeRadioCapabilityForPaging.EUTRA = hex.EncodeToString(
				msg.UERadioCapabilityForPaging.UERadioCapabilityForPagingOfEUTRA.Value)
		}
	}
}
