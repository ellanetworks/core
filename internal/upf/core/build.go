// SPDX-FileCopyrightText: 2025-present Ella Networks
// SPDX-License-Identifier: Apache-2.0

package core

import (
	"github.com/wmnsk/go-pfcp/ie"
	"github.com/wmnsk/go-pfcp/message"
)

func BuildPfcpSessionReportRequest(remoteSEID uint64, sequenceNumber uint32, pdrid uint16, qfi uint8) (*message.SessionReportRequest, error) {
	ies := make([]*ie.IE, 0)
	ies = append(ies, ie.NewDownlinkDataReport(ie.NewPDRID(pdrid), ie.NewDownlinkDataServiceInformation(false, true, 0, qfi)))
	ies = append(ies, ie.NewReportType(0, 0, 0, 1))
	return message.NewSessionReportRequest(
		1,
		0,
		remoteSEID,
		sequenceNumber,
		0,
		ies...,
	), nil
}
