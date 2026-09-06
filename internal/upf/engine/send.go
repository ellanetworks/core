// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

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
	HandleUsageReports(context.Context, []*models.UsageReport) error
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

func (conn *SessionEngine) SendUsageReports(ctx context.Context, smf SMFReportHandler, reports []*models.UsageReport) error {
	if len(reports) == 0 {
		return nil
	}

	return smf.HandleUsageReports(ctx, reports)
}
