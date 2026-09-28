// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package diameter

import (
	"encoding/binary"
	"errors"
	"fmt"
)

const (
	FlagRequest    uint8 = 0x80
	FlagProxiable  uint8 = 0x40
	FlagError      uint8 = 0x20
	FlagRetransmit uint8 = 0x10
)

const (
	version   = 1
	headerLen = 20
)

var (
	ErrUnsupportedVersion   = errors.New("diameter: unsupported version")
	ErrInvalidMessageLength = errors.New("diameter: invalid message length")
)

type Message struct {
	Flags         uint8
	CommandCode   uint32
	ApplicationID uint32
	HopByHopID    uint32
	EndToEndID    uint32
	AVPs          []AVP
}

func (m *Message) IsRequest() bool {
	return m.Flags&FlagRequest != 0
}

func (m *Message) Find(code, vendorID uint32) (AVP, bool) {
	return Find(m.AVPs, code, vendorID)
}

func (m *Message) Marshal() ([]byte, error) {
	if m.CommandCode > 0xffffff {
		return nil, fmt.Errorf("diameter: command code %d exceeds 24 bits", m.CommandCode)
	}

	b := make([]byte, headerLen, headerLen+64*len(m.AVPs))
	for _, a := range m.AVPs {
		if a.Flags&AVPFlagVendor == 0 && a.VendorID != 0 {
			return nil, fmt.Errorf("diameter: AVP %d has a Vendor-ID without the V flag", a.Code)
		}

		if a.headerLen()+len(a.Data) > 0xffffff {
			return nil, fmt.Errorf("diameter: AVP %d exceeds 24-bit length", a.Code)
		}

		b = a.appendTo(b)
	}

	if len(b) > 0xffffff {
		return nil, fmt.Errorf("diameter: message of %d octets exceeds 24-bit length", len(b))
	}

	b[0] = version
	b[1], b[2], b[3] = byte(len(b)>>16), byte(len(b)>>8), byte(len(b))
	b[4] = m.Flags
	b[5], b[6], b[7] = byte(m.CommandCode>>16), byte(m.CommandCode>>8), byte(m.CommandCode)
	binary.BigEndian.PutUint32(b[8:12], m.ApplicationID)
	binary.BigEndian.PutUint32(b[12:16], m.HopByHopID)
	binary.BigEndian.PutUint32(b[16:20], m.EndToEndID)

	return b, nil
}

func Unmarshal(b []byte) (*Message, error) {
	m, err := unmarshalHeader(b)
	if err != nil {
		return nil, err
	}

	avps, err := decodeAVPs(b[headerLen:])
	if err != nil {
		return m, err
	}

	m.AVPs = avps

	return m, nil
}

func unmarshalHeader(b []byte) (*Message, error) {
	if len(b) < headerLen {
		return nil, ErrInvalidMessageLength
	}

	m := &Message{
		Flags:         b[4],
		CommandCode:   uint32(b[5])<<16 | uint32(b[6])<<8 | uint32(b[7]),
		ApplicationID: binary.BigEndian.Uint32(b[8:12]),
		HopByHopID:    binary.BigEndian.Uint32(b[12:16]),
		EndToEndID:    binary.BigEndian.Uint32(b[16:20]),
	}

	if b[0] != version {
		return m, ErrUnsupportedVersion
	}

	length := int(b[1])<<16 | int(b[2])<<8 | int(b[3])
	if length != len(b) || length%4 != 0 {
		return m, ErrInvalidMessageLength
	}

	return m, nil
}
