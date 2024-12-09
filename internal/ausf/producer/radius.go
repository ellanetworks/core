package producer

import (
	"encoding/binary"
	"fmt"
)

type EapCode uint8

const (
	EapCodeRequest  EapCode = 1
	EapCodeResponse EapCode = 2
	EapCodeSuccess  EapCode = 3
	EapCodeFailure  EapCode = 4
)

type EapType uint8

const (
	EapTypeIdentity         EapType = 1
	EapTypeNotification     EapType = 2
	EapTypeNak              EapType = 3 // Response only
	EapTypeMd5Challenge     EapType = 4
	EapTypeOneTimePassword  EapType = 5 // otp
	EapTypeGenericTokenCard EapType = 6 // gtc
	EapTypeMSCHAPV2         EapType = 26
	EapTypeExpandedTypes    EapType = 254
	EapTypeExperimentalUse  EapType = 255
)

type EapPacket struct {
	Code       EapCode
	Identifier uint8
	Type       EapType
	Data       []byte
}

func (a *EapPacket) Encode() (b []byte) {
	b = make([]byte, len(a.Data)+5)
	b[0] = byte(a.Code)
	b[1] = byte(a.Identifier)
	binary.BigEndian.PutUint16(b[2:4], uint16(len(a.Data)+5))
	b[4] = byte(a.Type)
	copy(b[5:], a.Data)
	return b
}

func EapDecode(b []byte) (eap *EapPacket, err error) {
	if len(b) < 5 {
		return nil, fmt.Errorf("[EapDecode] protocol error input too small 1")
	}
	length := binary.BigEndian.Uint16(b[2:4])
	if len(b) < int(length) {
		return nil, fmt.Errorf("[EapDecode] protocol error input too small 2")
	}
	eap = &EapPacket{
		Code:       EapCode(b[0]),
		Identifier: uint8(b[1]),
		Type:       EapType(b[4]),
		Data:       b[5:length],
	}
	return eap, nil
}
