// SPDX-FileCopyrightText: 2025-present Ella Networks
// SPDX-License-Identifier: Apache-2.0

package core

import (
	"github.com/wmnsk/go-pfcp/ie"
	"github.com/wmnsk/go-pfcp/message"
)

func BuildPfcpSessionReportRequestForDownlinkData(remoteSEID uint64, sequenceNumber uint32, pdrid uint16, qfi uint8) (*message.SessionReportRequest, error) {
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

func BuildPfcpSessionReportRequestForUsage(remoteSEID uint64, sequenceNumber uint32, urrid uint32, urSeqn uint32, uvol uint64, dvol uint64) (*message.SessionReportRequest, error) {
	var volMeasurement uint8 = 0

	volMeasurement |= 0x01 // total volume
	volMeasurement |= 0x02 // uplink volume
	volMeasurement |= 0x04 // downlink volume

	ies := make([]*ie.IE, 0)
	ies = append(ies, ie.NewUsageReportWithinSessionReportRequest(
		ie.NewURRID(urrid),
		ie.NewURSEQN(urSeqn),
		ie.NewUsageReportTrigger(),
		ie.NewVolumeMeasurement(volMeasurement, uvol+dvol, uvol, dvol, 0, 0, 0),
	))
	ies = append(ies, ie.NewReportType(0, 0, 1, 0))

	return message.NewSessionReportRequest(
		1,
		0,
		remoteSEID,
		sequenceNumber,
		0,
		ies...,
	), nil
}
