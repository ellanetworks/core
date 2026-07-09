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

func HandleUERadioCapabilityInfoIndication(ctx gocontext.Context, amfInstance *amf.AMF, ran *amf.Radio, msg decode.UERadioCapabilityInfoIndication) {
	ueConn, ok := resolveUE(ctx, amfInstance, ran, &msg.RANUENGAPID, &msg.AMFUENGAPID)
	if !ok {
		return
	}

	logger.WithTrace(ctx, ueConn.Log).Debug("Handle UE Radio Capability Info Indication", zap.Int64("RanUeNgapID", int64(ueConn.RanUeNgapID)), zap.Int64("AmfUeNgapID", int64(ueConn.AmfUeNgapID)))
	ueConn.TouchLastSeen()

	amfUe := ueConn.UeContext()
	if amfUe == nil {
		logger.WithTrace(ctx, ueConn.Log).Error("amfUe is nil")
		return
	}

	if msg.UERadioCapability != nil {
		amfUe.RadioCapability = msg.UERadioCapability
	}

	if msg.UERadioCapabilityForPaging != nil {
		amfUe.RadioCapabilityForPaging = &models.UERadioCapabilityForPaging{}
		if msg.UERadioCapabilityForPaging.UERadioCapabilityForPagingOfNR != nil {
			amfUe.RadioCapabilityForPaging.NR = hex.EncodeToString(
				msg.UERadioCapabilityForPaging.UERadioCapabilityForPagingOfNR.Value)
		}

		if msg.UERadioCapabilityForPaging.UERadioCapabilityForPagingOfEUTRA != nil {
			amfUe.RadioCapabilityForPaging.EUTRA = hex.EncodeToString(
				msg.UERadioCapabilityForPaging.UERadioCapabilityForPagingOfEUTRA.Value)
		}
	}
}
