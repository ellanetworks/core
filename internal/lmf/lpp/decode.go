// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package lpp

import (
	"bytes"
	"encoding/binary"
	"fmt"

	"github.com/ellanetworks/core/internal/lmf/lpp/models"
)

// DecodeProvideLocationCapabilities decodes a ProvideLocationCapabilities message.
func DecodeProvideLocationCapabilities(data []byte) (*models.ProvideLocationCapabilities, error) {
	if len(data) < 2 {
		return nil, fmt.Errorf("insufficient data for ProvideLocationCapabilities: %d bytes", len(data))
	}

	if data[0] != MsgTypeProvideLocationCapabilities {
		return nil, fmt.Errorf("expected message type 0x%02x, got 0x%02x", MsgTypeProvideLocationCapabilities, data[0])
	}

	msg := &models.ProvideLocationCapabilities{
		TransactionID: data[1],
	}

	// Parse GNSS capability (simplified — single byte bitmask)
	if len(data) > 2 {
		capVal := data[2]
		msg.GNSSCapability.GPS = capVal&0x01 != 0
		msg.GNSSCapability.GLO = capVal&0x02 != 0
		msg.GNSSCapability.BDT = capVal&0x04 != 0
		msg.GNSSCapability.QZS = capVal&0x08 != 0
		msg.GNSSCapability.SBS = capVal&0x10 != 0
		msg.GNSSCapability.IRN = capVal&0x20 != 0
		msg.GNSSCapability.ESAT = capVal&0x40 != 0
	}

	return msg, nil
}

// EncodeProvideAssistanceData encodes a ProvideAssistanceData message.
func EncodeProvideAssistanceData(msg *models.ProvideAssistanceData) ([]byte, error) {
	var buf bytes.Buffer

	if err := buf.WriteByte(MsgTypeProvideAssistanceData); err != nil {
		return nil, fmt.Errorf("write type: %w", err)
	}

	if err := buf.WriteByte(msg.TransactionID); err != nil {
		return nil, fmt.Errorf("write transaction ID: %w", err)
	}

	// GNSS assistance data (raw bytes)
	dataLen := uint16(len(msg.GNSSAssistanceData))
	if err := binary.Write(&buf, binary.BigEndian, dataLen); err != nil {
		return nil, fmt.Errorf("write data length: %w", err)
	}

	if _, err := buf.Write(msg.GNSSAssistanceData); err != nil {
		return nil, fmt.Errorf("write assistance data: %w", err)
	}

	return buf.Bytes(), nil
}

// DecodeProvideAssistanceData decodes a ProvideAssistanceData message.
func DecodeProvideAssistanceData(data []byte) (*models.ProvideAssistanceData, error) {
	if len(data) < 3 {
		return nil, fmt.Errorf("insufficient data for ProvideAssistanceData: %d bytes", len(data))
	}

	if data[0] != MsgTypeProvideAssistanceData {
		return nil, fmt.Errorf("expected message type 0x%02x, got 0x%02x", MsgTypeProvideAssistanceData, data[0])
	}

	msg := &models.ProvideAssistanceData{
		TransactionID: data[1],
	}

	// Read assistance data length
	dataLen := binary.BigEndian.Uint16(data[2:4])
	if len(data) < 4+int(dataLen) {
		return nil, fmt.Errorf("insufficient data for assistance payload: need %d, got %d", 4+dataLen, len(data))
	}

	msg.GNSSAssistanceData = data[4 : 4+dataLen]

	return msg, nil
}

// EncodeProvideLocationInformation encodes a ProvideLocationInformation message.
func EncodeProvideLocationInformation(msg *models.ProvideLocationInformation) ([]byte, error) {
	var buf bytes.Buffer

	if err := buf.WriteByte(MsgTypeProvideLocationInformation); err != nil {
		return nil, fmt.Errorf("write type: %w", err)
	}

	if err := buf.WriteByte(msg.TransactionID); err != nil {
		return nil, fmt.Errorf("write transaction ID: %w", err)
	}

	// GNSS position result
	result := msg.GNSSPositionResult

	// Latitude (4 bytes, signed, 1e-7 degrees)
	if err := binary.Write(&buf, binary.BigEndian, result.Latitude); err != nil {
		return nil, fmt.Errorf("write latitude: %w", err)
	}

	// Longitude (4 bytes, signed, 1e-7 degrees)
	if err := binary.Write(&buf, binary.BigEndian, result.Longitude); err != nil {
		return nil, fmt.Errorf("write longitude: %w", err)
	}

	// Altitude (4 bytes, signed, cm)
	if err := binary.Write(&buf, binary.BigEndian, result.Altitude); err != nil {
		return nil, fmt.Errorf("write altitude: %w", err)
	}

	// Horizontal accuracy (2 bytes, unsigned, meters)
	if err := binary.Write(&buf, binary.BigEndian, uint16(result.HorizontalAccuracy)); err != nil {
		return nil, fmt.Errorf("write horizontal accuracy: %w", err)
	}

	// Vertical accuracy (2 bytes, unsigned, meters)
	if err := binary.Write(&buf, binary.BigEndian, uint16(result.VerticalAccuracy)); err != nil {
		return nil, fmt.Errorf("write vertical accuracy: %w", err)
	}

	// Timestamp (8 bytes, Unix ms)
	if err := binary.Write(&buf, binary.BigEndian, result.Timestamp); err != nil {
		return nil, fmt.Errorf("write timestamp: %w", err)
	}

	return buf.Bytes(), nil
}

// DecodeProvideLocationInformation decodes a ProvideLocationInformation message.
func DecodeProvideLocationInformation(data []byte) (*models.ProvideLocationInformation, error) {
	if len(data) < 26 {
		return nil, fmt.Errorf("insufficient data for ProvideLocationInformation: %d bytes", len(data))
	}

	if data[0] != MsgTypeProvideLocationInformation {
		return nil, fmt.Errorf("expected message type 0x%02x, got 0x%02x", MsgTypeProvideLocationInformation, data[0])
	}

	msg := &models.ProvideLocationInformation{
		TransactionID: data[1],
	}

	// Parse GNSS position result
	msg.GNSSPositionResult.Latitude = int32(binary.BigEndian.Uint32(data[2:6]))
	msg.GNSSPositionResult.Longitude = int32(binary.BigEndian.Uint32(data[6:10]))
	msg.GNSSPositionResult.Altitude = int32(binary.BigEndian.Uint32(data[10:14]))
	msg.GNSSPositionResult.HorizontalAccuracy = uint32(binary.BigEndian.Uint16(data[14:16]))
	msg.GNSSPositionResult.VerticalAccuracy = uint32(binary.BigEndian.Uint16(data[16:18]))
	msg.GNSSPositionResult.Timestamp = int64(binary.BigEndian.Uint64(data[18:26]))

	return msg, nil
}
