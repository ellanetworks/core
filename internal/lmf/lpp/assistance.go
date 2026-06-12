// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package lpp

import (
	"fmt"

	"github.com/ellanetworks/core/internal/lmf/lpp/models"
)

// BuildAssistanceData constructs a mock ProvideAssistanceData message with
// test ephemeris data. Real ephemeris from u-blox/RTCM is Phase 3+.
func BuildAssistanceData(transactionID byte) ([]byte, error) {
	// Mock GNSS assistance data: 8-byte test ephemeris payload
	// Format: [constellation(1), prn(1), reserved(6)]
	mockEphemeris := []byte{
		0x01,                               // GPS
		0x01,                               // PRN 1
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, // Reserved
	}

	msg := &models.ProvideAssistanceData{
		TransactionID:      transactionID,
		GNSSAssistanceData: mockEphemeris,
	}

	return EncodeProvideAssistanceData(msg)
}

// BuildRequestLocationInfo constructs a RequestLocationInformation message
// requesting a GNSS fix.
func BuildRequestLocationInfo(transactionID byte, method uint8) ([]byte, error) {
	msg := &models.RequestLocationInformation{
		TransactionID:     transactionID,
		PositioningMethod: method,
		NumberOfSVs:       4, // Request at least 4 SVs
	}

	return EncodeRequestLocationInformation(msg)
}

// ParseLPPMessage dispatches an LPP payload to the appropriate decoder.
func ParseLPPMessage(data []byte) (any, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("empty LPP payload")
	}

	switch data[0] {
	case MsgTypeProvideLocationCapabilities:
		return DecodeProvideLocationCapabilities(data)
	case MsgTypeProvideAssistanceData:
		return DecodeProvideAssistanceData(data)
	case MsgTypeProvideLocationInformation:
		return DecodeProvideLocationInformation(data)
	case MsgTypeRequestLocationInformation:
		return DecodeRequestLocationInformation(data)
	default:
		return nil, fmt.Errorf("unknown LPP message type: 0x%02x", data[0])
	}
}
