// Copyright 2024 Canonical Ltd.
//
// SPDX-License-Identifier: Apache-2.0
//
// The User Plane IP Resource Information has been deprecated in 3GPP Release 16.
// This file should be removed from the project as we get rid of the User Plane IP Resource Information
// IE in the PFCP Association Setup Response Message

package ies

import (
	"fmt"
	"net"

	"github.com/yeastengine/ella/internal/smf/context"
)

const (
	Mask8 = 1<<8 - 1
	Mask7 = 1<<7 - 1
	Mask6 = 1<<6 - 1
	Mask5 = 1<<5 - 1
	Mask4 = 1<<4 - 1
	Mask3 = 1<<3 - 1
	Mask2 = 1<<2 - 1
	Mask1 = 1<<1 - 1
)

const (
	BitMask8 = 1 << 7
	BitMask7 = 1 << 6
	BitMask6 = 1 << 5
	BitMask5 = 1 << 4
	BitMask4 = 1 << 3
	BitMask3 = 1 << 2
	BitMask2 = 1 << 1
	BitMask1 = 1
)

func utob(u uint8) bool {
	return u != 0
}

func UnmarshalUEIPInformationBinary(data []byte) (*context.UserPlaneIPResourceInformation, error) {
	u := &context.UserPlaneIPResourceInformation{}
	length := uint16(len(data))

	var idx uint16 = 0
	// Octet 5
	if length < idx+1 {
		return nil, fmt.Errorf("inadequate TLV length: %d", length)
	}
	u.Assosi = utob(data[idx] & BitMask7)
	u.Assoni = utob(data[idx] & BitMask6)
	u.Teidri = data[idx] >> 2 & Mask3
	u.V6 = utob(data[idx] & BitMask2)
	u.V4 = utob(data[idx] & BitMask1)
	idx = idx + 1

	// Octet 6
	if u.Teidri != 0 {
		if length < idx+1 {
			return nil, fmt.Errorf("inadequate TLV length: %d", length)
		}
		u.TeidRange = data[idx]
		idx = idx + 1
	}

	// Octet m to (m+3)
	if u.V4 {
		if length < idx+net.IPv4len {
			return nil, fmt.Errorf("inadequate TLV length: %d", length)
		}
		u.Ipv4Address = data[idx : idx+net.IPv4len]
		idx = idx + net.IPv4len
	}

	// Octet p to (p+15)
	if u.V6 {
		if length < idx+net.IPv6len {
			return nil, fmt.Errorf("inadequate TLV length: %d", length)
		}
		u.Ipv6Address = data[idx : idx+net.IPv6len]
		idx = idx + net.IPv6len
	}

	if !u.V4 && !u.V6 {
		return nil, fmt.Errorf("none of V4 and V6 flags is set")
	}

	// Octet r
	if u.Assosi {
		if length < idx+1 {
			return nil, fmt.Errorf("inadequate TLV length: %d", length)
		}
		u.SourceInterface = data[length-1] & Mask4
		data = data[:length-1]
	}

	// Octet k to l
	if u.Assoni {
		if length < idx+1 {
			return nil, fmt.Errorf("inadequate TLV length: %d", length)
		}
		err := u.NetworkInstance.UnmarshalBinary(data[idx:])
		if err != nil {
			return nil, err
		}
	}

	return u, nil
}
