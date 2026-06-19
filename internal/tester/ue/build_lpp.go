// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ue

import (
	"bytes"
	"encoding/binary"

	"github.com/free5gc/nas"
	"github.com/free5gc/nas/nasMessage"
)

// LPP message types per 3GPP TS 24.030 / 36.355.
const (
	lppMsgTypeRequestLocationInformation  = 0x01
	lppMsgTypeProvideLocationCapabilities = 0x02
	lppMsgTypeProvideAssistanceData       = 0x03
	lppMsgTypeProvideLocationInformation  = 0x04
)

// GNSS capability bitmask values.
const (
	lppGNSSCapabilityGPS  = 0x01
	lppGNSSCapabilityGLO  = 0x02
	lppGNSSCapabilityBDT  = 0x04
	lppGNSSCapabilityQZS  = 0x08
	lppGNSSCapabilitySBS  = 0x10
	lppGNSSCapabilityIRN  = 0x20
	lppGNSSCapabilityESAT = 0x40
)

// LPPCapabilitiesResponseOpts contains the parameters for building the UE's
// ProvideLocationCapabilities LPP response.
type LPPCapabilitiesResponseOpts struct {
	TransactionID byte
	GNSSGPS       bool
	GNSSGLO       bool
	GNSSBDT       bool
}

// BuildLPPCapabilitiesResponse creates an LPP ProvideLocationCapabilities
// message encoded as raw LPP bytes (without NAS wrapper).
func BuildLPPCapabilitiesResponse(opts *LPPCapabilitiesResponseOpts) ([]byte, error) {
	if opts == nil {
		return nil, nil
	}

	var buf bytes.Buffer

	// Message type
	if err := buf.WriteByte(lppMsgTypeProvideLocationCapabilities); err != nil {
		return nil, err
	}

	// Transaction ID
	if err := buf.WriteByte(opts.TransactionID); err != nil {
		return nil, err
	}

	// GNSS capability bitmask
	var capByte byte
	if opts.GNSSGPS {
		capByte |= lppGNSSCapabilityGPS
	}

	if opts.GNSSGLO {
		capByte |= lppGNSSCapabilityGLO
	}

	if opts.GNSSBDT {
		capByte |= lppGNSSCapabilityBDT
	}

	if err := buf.WriteByte(capByte); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

// LPPLocationResponseOpts contains the parameters for building the UE's
// ProvideLocationInformation LPP response (UE-assisted GNSS fix).
type LPPLocationResponseOpts struct {
	TransactionID      byte
	Latitude           int32  // 1e-7 degrees
	Longitude          int32  // 1e-7 degrees
	Altitude           int32  // cm
	HorizontalAccuracy uint16 // meters
	VerticalAccuracy   uint16 // meters
	Timestamp          int64  // Unix milliseconds
}

// BuildLPPLocationResponse creates an LPP ProvideLocationInformation message
// encoded as raw LPP bytes (without NAS wrapper).
func BuildLPPLocationResponse(opts *LPPLocationResponseOpts) ([]byte, error) {
	if opts == nil {
		return nil, nil
	}

	var buf bytes.Buffer

	// Message type
	if err := buf.WriteByte(lppMsgTypeProvideLocationInformation); err != nil {
		return nil, err
	}

	// Transaction ID
	if err := buf.WriteByte(opts.TransactionID); err != nil {
		return nil, err
	}

	// Latitude (4 bytes, signed, 1e-7 degrees)
	if err := binary.Write(&buf, binary.BigEndian, opts.Latitude); err != nil {
		return nil, err
	}

	// Longitude (4 bytes, signed, 1e-7 degrees)
	if err := binary.Write(&buf, binary.BigEndian, opts.Longitude); err != nil {
		return nil, err
	}

	// Altitude (4 bytes, signed, cm)
	if err := binary.Write(&buf, binary.BigEndian, opts.Altitude); err != nil {
		return nil, err
	}

	// Horizontal accuracy (2 bytes, unsigned, meters)
	if err := binary.Write(&buf, binary.BigEndian, opts.HorizontalAccuracy); err != nil {
		return nil, err
	}

	// Vertical accuracy (2 bytes, unsigned, meters)
	if err := binary.Write(&buf, binary.BigEndian, opts.VerticalAccuracy); err != nil {
		return nil, err
	}

	// Timestamp (8 bytes, Unix ms)
	if err := binary.Write(&buf, binary.BigEndian, opts.Timestamp); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

// BuildDLNASTransportLPP wraps LPP payload bytes in a DL NAS Transport message
// with payload container type set to LPP.
func BuildDLNASTransportLPP(lppPayload []byte) ([]byte, error) {
	m := nas.NewMessage()
	m.GmmMessage = nas.NewGmmMessage()
	m.GmmHeader.SetMessageType(nas.MsgTypeDLNASTransport)

	dlNasTransport := nasMessage.NewDLNASTransport(0)
	dlNasTransport.SetSecurityHeaderType(nas.SecurityHeaderTypePlainNas)
	dlNasTransport.SetMessageType(nas.MsgTypeDLNASTransport)
	dlNasTransport.SetExtendedProtocolDiscriminator(nasMessage.Epd5GSMobilityManagementMessage)
	dlNasTransport.SetPayloadContainerType(nasMessage.PayloadContainerTypeLPP)
	dlNasTransport.PayloadContainer.SetLen(uint16(len(lppPayload)))
	dlNasTransport.SetPayloadContainerContents(lppPayload)

	m.DLNASTransport = dlNasTransport

	data := new(bytes.Buffer)
	if err := m.GmmMessageEncode(data); err != nil {
		return nil, err
	}

	return data.Bytes(), nil
}
