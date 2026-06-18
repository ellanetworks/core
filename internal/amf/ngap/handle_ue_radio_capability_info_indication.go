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

	amfUe := ranUe.AmfUe()
	if amfUe == nil {
		logger.WithTrace(ctx, ranUe.Log).Error("amfUe is nil")
		return
	}

	if msg.UERadioCapability != nil {
		amfUe.Current().UeRadioCapability = hex.EncodeToString(msg.UERadioCapability)
	}

	if msg.UERadioCapabilityForPaging != nil {
		amfUe.Current().UeRadioCapabilityForPaging = &models.UERadioCapabilityForPaging{}
		if msg.UERadioCapabilityForPaging.UERadioCapabilityForPagingOfNR != nil {
			amfUe.Current().UeRadioCapabilityForPaging.NR = hex.EncodeToString(
				msg.UERadioCapabilityForPaging.UERadioCapabilityForPagingOfNR.Value)
		}

		if msg.UERadioCapabilityForPaging.UERadioCapabilityForPagingOfEUTRA != nil {
			amfUe.Current().UeRadioCapabilityForPaging.EUTRA = hex.EncodeToString(
				msg.UERadioCapabilityForPaging.UERadioCapabilityForPagingOfEUTRA.Value)
		}
	}

	// TS 38.413 8.14.1.2/TS 23.502 4.2.8a step5/TS 23.501, clause 5.4.4.1.
	// send its most up to date UE Radio Capability information to the RAN in the N2 REQUEST message.
}
