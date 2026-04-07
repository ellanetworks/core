// SPDX-FileCopyrightText: 2025-present Ella Networks
// SPDX-License-Identifier: Apache-2.0

package engine

import (
	"context"
	"fmt"

	"github.com/ellanetworks/core/internal/models"
)

// SMFReportHandler is the callback interface the UPF uses to deliver
// reports (downlink data notifications, usage measurements, flow stats)
// back to the SMF.
type SMFReportHandler interface {
	HandleDownlinkDataReport(context.Context, *models.DownlinkDataReport) error
	HandleUsageReport(context.Context, *models.UsageReport) error
	SendFlowReports(context.Context, []*models.FlowReportRequest) error
}

func (conn *SessionEngine) SendDownlinkDataReport(ctx context.Context, smf SMFReportHandler, localSeid uint64, pdrid uint16, qfi uint8) error {
	session := conn.GetSession(localSeid)
	if session == nil {
		return fmt.Errorf("failed to find session with localSeid: %d", localSeid)
	}

	return smf.HandleDownlinkDataReport(ctx, &models.DownlinkDataReport{
		SEID:  session.SEID,
		PDRID: pdrid,
		QFI:   qfi,
	})
}

func (conn *SessionEngine) SendUsageReport(ctx context.Context, smf SMFReportHandler, localSeid uint64, uvol uint64, dvol uint64) error {
	session := conn.GetSession(localSeid)
	if session == nil {
		return fmt.Errorf("failed to find session with localSeid: %d", localSeid)
	}

	return smf.HandleUsageReport(ctx, &models.UsageReport{
		SEID:           session.SEID,
		UplinkVolume:   uvol,
		DownlinkVolume: dvol,
	})
}
