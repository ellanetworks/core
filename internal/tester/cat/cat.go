// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package cat

import (
	"errors"
	"fmt"
)

const ComprehensionRequired = 0x80

const (
	TagProactiveCommand = 0xd0
	TagSMSPPDownload    = 0xd1
)

const (
	TagCommandDetails   = 0x01
	TagDeviceIdentities = 0x02
	TagResult           = 0x03
	TagAddress          = 0x06
	TagSMSTPDU          = 0x0b
)

const (
	DeviceUICC     = 0x81
	DeviceTerminal = 0x82
	DeviceNetwork  = 0x83
)

const TypeSendShortMessage = 0x13

const QualifierSMSPacking = 0x01

const (
	ResultOK                    = 0x00
	ResultTerminalUnable        = 0x20
	ResultNetworkUnable         = 0x21
	ResultBeyondCapabilities    = 0x30
	ResultDataNotUnderstood     = 0x32
	ResultRequiredValuesMissing = 0x36
	ResultSMSRPError            = 0x35
)

const NoSpecificCause = 0x00

const (
	SWOK          uint16 = 0x9000
	SWToolkitBusy uint16 = 0x9300
	SWProactive   byte   = 0x91
)

func ProactivePending(sw uint16) bool {
	return byte(sw>>8) == SWProactive
}

func AppendTLV(b []byte, tag byte, value []byte) []byte {
	b = append(b, tag)

	if len(value) > 0x7f {
		b = append(b, 0x81)
	}

	b = append(b, byte(len(value)))

	return append(b, value...)
}

func ReadTLV(b []byte) (tag byte, value, rest []byte, err error) {
	if len(b) < 2 {
		return 0, nil, nil, errors.New("cat: truncated data object")
	}

	tag, n, i := b[0], int(b[1]), 2

	switch {
	case n == 0x81:
		if len(b) < 3 || b[2] < 0x80 {
			return 0, nil, nil, errors.New("cat: invalid length")
		}

		n, i = int(b[2]), 3
	case n > 0x7f:
		return 0, nil, nil, fmt.Errorf("cat: length coding %#02x", n)
	}

	if len(b) < i+n {
		return 0, nil, nil, errors.New("cat: truncated data object")
	}

	return tag, b[i : i+n], b[i+n:], nil
}

type Objects map[byte][]byte

func ReadObjects(b []byte) (Objects, error) {
	objects := Objects{}

	for len(b) > 0 {
		tag, value, rest, err := ReadTLV(b)
		if err != nil {
			return nil, err
		}

		b = rest

		if _, seen := objects[tag&^ComprehensionRequired]; !seen {
			objects[tag&^ComprehensionRequired] = value
		}
	}

	return objects, nil
}

func (o Objects) Get(tag byte) ([]byte, bool) {
	v, ok := o[tag&^ComprehensionRequired]
	return v, ok
}

type CommandDetails struct {
	Number, Type, Qualifier byte
}

func (d CommandDetails) Value() []byte {
	return []byte{d.Number, d.Type, d.Qualifier}
}

func ParseCommandDetails(v []byte) (CommandDetails, error) {
	if len(v) != 3 {
		return CommandDetails{}, fmt.Errorf("cat: command details of %d octets", len(v))
	}

	return CommandDetails{Number: v[0], Type: v[1], Qualifier: v[2]}, nil
}
