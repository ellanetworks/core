// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package eps

import (
	"encoding/binary"
	"fmt"

	"github.com/ellanetworks/core/nas/common"
)

// EPS attach type values (TS 24.301).
const (
	AttachTypeEPS          uint8 = 1
	AttachTypeCombined     uint8 = 2
	AttachTypeEPSEmergency uint8 = 6
)

// MobileIdentityType is the type-of-identity field of an EPS mobile identity
// (TS 24.301).
type MobileIdentityType uint8

const (
	IdentityIMSI MobileIdentityType = 1
	IdentityIMEI MobileIdentityType = 3
	IdentityGUTI MobileIdentityType = 6
)

// EPSMobileIdentity is an EPS mobile identity (TS 24.301): a GUTI, or
// an IMSI/IMEI carried as packed BCD digits.
type EPSMobileIdentity struct {
	Type MobileIdentityType

	// GUTI
	MCC, MNC   string
	MMEGroupID uint16
	MMECode    uint8
	MTMSI      uint32

	// IMSI / IMEI
	Digits string
}

func decodeEPSMobileIdentity(b []byte) (EPSMobileIdentity, error) {
	if len(b) == 0 {
		return EPSMobileIdentity{}, fmt.Errorf("nas/eps: empty EPS mobile identity")
	}

	typ := MobileIdentityType(b[0] & 0x07)

	switch typ {
	case IdentityGUTI:
		if len(b) < 11 {
			return EPSMobileIdentity{}, fmt.Errorf("nas/eps: GUTI is %d octets, want 11", len(b))
		}

		mcc, mnc := common.DecodePLMN([3]byte{b[1], b[2], b[3]})

		return EPSMobileIdentity{
			Type:       IdentityGUTI,
			MCC:        mcc,
			MNC:        mnc,
			MMEGroupID: binary.BigEndian.Uint16(b[4:6]),
			MMECode:    b[6],
			MTMSI:      binary.BigEndian.Uint32(b[7:11]),
		}, nil
	case IdentityIMSI, IdentityIMEI:
		return EPSMobileIdentity{Type: typ, Digits: string([]byte{'0' + (b[0] >> 4)}) + common.DecodeTBCD(b[1:])}, nil
	default:
		return EPSMobileIdentity{}, fmt.Errorf("nas/eps: unsupported EPS mobile identity type %d", typ)
	}
}

func (id EPSMobileIdentity) encode() ([]byte, error) {
	switch id.Type {
	case IdentityGUTI:
		plmn, err := common.EncodePLMN(id.MCC, id.MNC)
		if err != nil {
			return nil, err
		}

		out := make([]byte, 11)
		out[0] = 0xF0 | uint8(IdentityGUTI) // spare 1111, even, type-of-identity 110
		copy(out[1:4], plmn[:])
		binary.BigEndian.PutUint16(out[4:6], id.MMEGroupID)
		out[6] = id.MMECode
		binary.BigEndian.PutUint32(out[7:11], id.MTMSI)

		return out, nil
	case IdentityIMSI, IdentityIMEI:
		if len(id.Digits) == 0 || id.Digits[0] < '0' || id.Digits[0] > '9' {
			return nil, fmt.Errorf("nas/eps: invalid identity digits %q", id.Digits)
		}

		rest, err := common.EncodeTBCD(id.Digits[1:])
		if err != nil {
			return nil, err
		}

		oddEven := byte(len(id.Digits) & 1)
		out := append([]byte{(id.Digits[0]-'0')<<4 | oddEven<<3 | uint8(id.Type)}, rest...)

		return out, nil
	default:
		return nil, fmt.Errorf("nas/eps: cannot encode EPS mobile identity type %d", id.Type)
	}
}
