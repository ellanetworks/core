// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package diameter

import (
	"encoding/binary"
	"errors"
	"fmt"
	"net/netip"
	"time"
)

const (
	AVPFlagVendor    uint8 = 0x80
	AVPFlagMandatory uint8 = 0x40
	AVPFlagProtected uint8 = 0x20
)

const (
	addressFamilyIPv4 uint16 = 1
	addressFamilyIPv6 uint16 = 2
)

var ErrInvalidAVPLength = errors.New("diameter: invalid AVP length")

type AVP struct {
	Code     uint32
	Flags    uint8
	VendorID uint32
	Data     []byte
}

func (a AVP) headerLen() int {
	if a.Flags&AVPFlagVendor != 0 {
		return 12
	}

	return 8
}

func (a AVP) paddedLen() int {
	return (a.headerLen() + len(a.Data) + 3) &^ 3
}

func (a AVP) appendTo(b []byte) []byte {
	length := a.headerLen() + len(a.Data)

	b = binary.BigEndian.AppendUint32(b, a.Code)
	b = append(b, a.Flags, byte(length>>16), byte(length>>8), byte(length))

	if a.Flags&AVPFlagVendor != 0 {
		b = binary.BigEndian.AppendUint32(b, a.VendorID)
	}

	b = append(b, a.Data...)

	for range a.paddedLen() - length {
		b = append(b, 0)
	}

	return b
}

func decodeAVPs(b []byte) ([]AVP, error) {
	var avps []AVP

	for len(b) > 0 {
		if len(b) < 8 {
			return nil, ErrInvalidAVPLength
		}

		a := AVP{
			Code:  binary.BigEndian.Uint32(b[0:4]),
			Flags: b[4],
		}

		length := int(b[5])<<16 | int(b[6])<<8 | int(b[7])
		if length < a.headerLen() || length > len(b) {
			return nil, ErrInvalidAVPLength
		}

		if a.Flags&AVPFlagVendor != 0 {
			a.VendorID = binary.BigEndian.Uint32(b[8:12])
		}

		a.Data = b[a.headerLen():length]

		padded := (length + 3) &^ 3
		if padded > len(b) {
			padded = len(b)
		}

		avps = append(avps, a)
		b = b[padded:]
	}

	return avps, nil
}

func newAVP(code uint32, flags uint8, vendorID uint32, data []byte) AVP {
	if vendorID != 0 {
		flags |= AVPFlagVendor
	}

	return AVP{Code: code, Flags: flags, VendorID: vendorID, Data: data}
}

func Unsigned32(code uint32, flags uint8, vendorID uint32, v uint32) AVP {
	return newAVP(code, flags, vendorID, binary.BigEndian.AppendUint32(nil, v))
}

func OctetString(code uint32, flags uint8, vendorID uint32, v []byte) AVP {
	return newAVP(code, flags, vendorID, append([]byte(nil), v...))
}

func UTF8String(code uint32, flags uint8, vendorID uint32, v string) AVP {
	return newAVP(code, flags, vendorID, []byte(v))
}

func Address(code uint32, flags uint8, vendorID uint32, v netip.Addr) AVP {
	var data []byte

	if v.Is4() {
		data = binary.BigEndian.AppendUint16(nil, addressFamilyIPv4)
	} else {
		data = binary.BigEndian.AppendUint16(nil, addressFamilyIPv6)
	}

	data = append(data, v.AsSlice()...)

	return newAVP(code, flags, vendorID, data)
}

const ntpUnixOffset = 2208988800

func Time(code uint32, flags uint8, vendorID uint32, t time.Time) AVP {
	return newAVP(code, flags, vendorID, binary.BigEndian.AppendUint32(nil, uint32(t.Unix()+ntpUnixOffset)))
}

func (a AVP) Time() (time.Time, error) {
	if len(a.Data) != 4 {
		return time.Time{}, fmt.Errorf("diameter: AVP %d: Time of %d octets", a.Code, len(a.Data))
	}

	seconds := int64(binary.BigEndian.Uint32(a.Data))
	if seconds < 1<<31 {
		seconds += 1 << 32
	}

	return time.Unix(seconds-ntpUnixOffset, 0).UTC(), nil
}

func Grouped(code uint32, flags uint8, vendorID uint32, avps ...AVP) AVP {
	var data []byte
	for _, a := range avps {
		data = a.appendTo(data)
	}

	return newAVP(code, flags, vendorID, data)
}

func (a AVP) Unsigned32() (uint32, error) {
	if len(a.Data) != 4 {
		return 0, fmt.Errorf("diameter: AVP %d: Unsigned32 of %d octets", a.Code, len(a.Data))
	}

	return binary.BigEndian.Uint32(a.Data), nil
}

func (a AVP) UTF8String() string {
	return string(a.Data)
}

func (a AVP) String() string {
	return fmt.Sprintf("AVP{code=%d vendor=%d flags=%#02x len=%d}", a.Code, a.VendorID, a.Flags, len(a.Data))
}

func (a AVP) Address() (netip.Addr, error) {
	if len(a.Data) < 2 {
		return netip.Addr{}, fmt.Errorf("diameter: AVP %d: Address of %d octets", a.Code, len(a.Data))
	}

	family := binary.BigEndian.Uint16(a.Data)
	value := a.Data[2:]

	switch {
	case family == addressFamilyIPv4 && len(value) == 4:
		return netip.AddrFrom4([4]byte(value)), nil
	case family == addressFamilyIPv6 && len(value) == 16:
		return netip.AddrFrom16([16]byte(value)), nil
	default:
		return netip.Addr{}, fmt.Errorf("diameter: AVP %d: unsupported address family %d with %d octets", a.Code, family, len(value))
	}
}

func (a AVP) Grouped() ([]AVP, error) {
	return decodeAVPs(a.Data)
}

func Find(avps []AVP, code, vendorID uint32) (AVP, bool) {
	for _, a := range avps {
		if a.Code == code && a.VendorID == vendorID {
			return a, true
		}
	}

	return AVP{}, false
}

func FindAll(avps []AVP, code, vendorID uint32) []AVP {
	var out []AVP

	for _, a := range avps {
		if a.Code == code && a.VendorID == vendorID {
			out = append(out, a)
		}
	}

	return out
}
