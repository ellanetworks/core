// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package lpp

import (
	"time"

	"github.com/ellanetworks/core/internal/lmf/lpp/models"
)

// BuildLocationInformation constructs a ProvideLocationInformation message
// from a GNSS fix result.
func BuildLocationInformation(transactionID byte, lat int32, lon int32, alt int32, hAcc, vAcc uint32) ([]byte, error) {
	msg := &models.ProvideLocationInformation{
		TransactionID: transactionID,
		GNSSPositionResult: models.GNSSPositionResult{
			Latitude:           lat,
			Longitude:          lon,
			Altitude:           alt,
			HorizontalAccuracy: hAcc,
			VerticalAccuracy:   vAcc,
			Timestamp:          time.Now().UnixMilli(),
		},
	}

	return EncodeProvideLocationInformation(msg)
}
