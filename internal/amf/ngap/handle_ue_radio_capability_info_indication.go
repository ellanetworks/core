// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-FileCopyrightText: 2022-present Intel Corporation
// SPDX-FileCopyrightText: 2021 Open Networking Foundation <info@opennetworking.org>
// Copyright 2019 free5GC.org
// Modified by Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"context"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/ngap"
	"go.uber.org/zap"
)

// HandleUERadioCapabilityInfoIndication stores the UE Radio Capability reported
// by the NG-RAN node (TS 38.413 §8.5.1), replayed in later INITIAL CONTEXT SETUP
// REQUEST messages so the node need not re-fetch it from the UE (TS 23.502).
func HandleUERadioCapabilityInfoIndication(ctx context.Context, amfInstance *amf.AMF, ran *amf.Radio, msg *ngap.UERadioCapabilityInfoIndication) {
	ueConn, ok := resolveUE(ctx, amfInstance, ran, msg.AMFUENGAPID, msg.RANUENGAPID)
	if !ok {
		return
	}

	reportDiagnostics(ctx, ran, ngap.ProcUERadioCapabilityInfoIndication, ngap.TriggeringInitiatingMessage, ueAssociated(msg.AMFUENGAPID, msg.RANUENGAPID), msg.Diagnostics())

	ueConn.TouchLastSeen()

	amfUe := ueConn.UeContext()
	if amfUe == nil {
		logger.WithTrace(ctx, ueConn.Log).Error("amfUe is nil")
		return
	}

	// §10.3.5: an absent IE leaves the stored capability standing.
	if msg.UERadioCapability != nil {
		amfUe.RadioCapability = msg.UERadioCapability
	}

	if p := msg.UERadioCapabilityForPaging; p != nil {
		stored := &models.UERadioCapabilityForPaging{}

		if p.NR != nil {
			stored.NR = *p.NR
		}

		if p.EUTRA != nil {
			stored.EUTRA = *p.EUTRA
		}

		amfUe.RadioCapabilityForPaging = stored
	}

	logger.WithTrace(ctx, ueConn.Log).Info("stored UE Radio Capability",
		zap.Int("bytes", len(amfUe.RadioCapability)))
}
