// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package lpp

import (
	"bytes"
	"encoding/binary"
	"fmt"

	"github.com/ellanetworks/core/internal/lmf/lpp/models"
)

// EncodeRequestLocationInformation encodes a RequestLocationInformation message.
func EncodeRequestLocationInformation(msg *models.RequestLocationInformation) ([]byte, error) {
	var buf bytes.Buffer

	// Message type byte
	if err := buf.WriteByte(MsgTypeRequestLocationInformation); err != nil {
		return nil, fmt.Errorf("write type: %w", err)
	}

	// Transaction ID
	if err := buf.WriteByte(msg.TransactionID); err != nil {
		return nil, fmt.Errorf("write transaction ID: %w", err)
	}

	// Positioning method
	if err := buf.WriteByte(msg.PositioningMethod); err != nil {
		return nil, fmt.Errorf("write positioning method: %w", err)
	}

	// Number of SVs (optional, encoded as presence bit + value)
	if msg.NumberOfSVs > 0 {
		if err := buf.WriteByte(0x80); err != nil { // presence bit set
			return nil, fmt.Errorf("write presence: %w", err)
		}

		if err := binary.Write(&buf, binary.BigEndian, msg.NumberOfSVs); err != nil {
			return nil, fmt.Errorf("write num SVs: %w", err)
		}
	}

	return buf.Bytes(), nil
}

// DecodeRequestLocationInformation decodes a RequestLocationInformation message.
func DecodeRequestLocationInformation(data []byte) (*models.RequestLocationInformation, error) {
	if len(data) < 3 {
		return nil, fmt.Errorf("insufficient data for RequestLocationInformation: %d bytes", len(data))
	}

	if data[0] != MsgTypeRequestLocationInformation {
		return nil, fmt.Errorf("expected message type 0x%02x, got 0x%02x", MsgTypeRequestLocationInformation, data[0])
	}

	msg := &models.RequestLocationInformation{
		TransactionID:     data[1],
		PositioningMethod: data[2],
	}

	// Check for optional numberOfSVs
	if len(data) > 4 && data[3]&0x80 != 0 {
		msg.NumberOfSVs = data[4]
	}

	return msg, nil
}
