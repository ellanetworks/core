// Copyright 2024 Canonical Ltd.
// SPDX-License-Identifier: Apache-2.0

// This file exists because the way SupportedFeatures are represented here is different from the go-pfcp library.

package ies

import (
	"encoding/binary"
	"fmt"

	"github.com/ellanetworks/core/internal/smf/context"
)

func UnmarshallUserPlaneFunctionFeatures(data []byte) (*context.UPFunctionFeatures, error) {
	length := uint16(len(data))

	u := &context.UPFunctionFeatures{}

	var idx uint16 = 0
	// Octet 5 to 6
	if length < idx+2 {
		return nil, fmt.Errorf("inadequate TLV length: %d", length)
	}

	// Additional Supported-Features
	if length >= 2 {
		u.SupportedFeatures = binary.LittleEndian.Uint16(data[idx : idx+2])
	}

	if length >= 4 {
		// Additional Supported-Features 1
		idx += 2
		u.SupportedFeatures1 = binary.LittleEndian.Uint16(data[idx : idx+2])
	}

	if length == 6 {
		// Additional Supported-Features 2
		idx += 2
		u.SupportedFeatures2 = binary.LittleEndian.Uint16(data[idx : idx+2])
	}

	return u, nil
}
