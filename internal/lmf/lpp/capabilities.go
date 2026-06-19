// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package lpp

import (
	"github.com/ellanetworks/core/internal/lmf/lpp/models"
)

// BuildRequestCapabilities constructs a RequestLocationInformation message
// asking the UE to provide its capabilities.
func BuildRequestCapabilities(transactionID byte) ([]byte, error) {
	msg := &models.RequestLocationInformation{
		TransactionID:     transactionID,
		PositioningMethod: PosMethodGNSS,
	}

	return EncodeRequestLocationInformation(msg)
}

// BuildProvideCapabilities constructs a ProvideLocationCapabilities response
// indicating the UE supports GPS.
func BuildProvideCapabilities(transactionID byte) ([]byte, error) {
	msg := &models.ProvideLocationCapabilities{
		TransactionID: transactionID,
		GNSSCapability: models.GNSSCapability{
			GPS: true,
			GLO: true,
		},
	}

	// Encode as raw LPP message
	var buf []byte

	buf = append(buf, MsgTypeProvideLocationCapabilities)
	buf = append(buf, transactionID)

	capVal := byte(0)
	if msg.GNSSCapability.GPS {
		capVal |= 0x01
	}

	if msg.GNSSCapability.GLO {
		capVal |= 0x02
	}

	if msg.GNSSCapability.BDT {
		capVal |= 0x04
	}

	if msg.GNSSCapability.QZS {
		capVal |= 0x08
	}

	if msg.GNSSCapability.SBS {
		capVal |= 0x10
	}

	if msg.GNSSCapability.IRN {
		capVal |= 0x20
	}

	if msg.GNSSCapability.ESAT {
		capVal |= 0x40
	}

	buf = append(buf, capVal)

	return buf, nil
}
