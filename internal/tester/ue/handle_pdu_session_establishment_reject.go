// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ue

import (
	"fmt"

	"github.com/ellanetworks/core/internal/tester/logger"
	"github.com/ellanetworks/core/nas/fgs"
	"go.uber.org/zap"
)

func handlePDUSessionEstablishmentReject(ue *UE, payload []byte) error {
	rej, err := fgs.ParsePDUSessionEstablishmentReject(payload)
	if err != nil {
		return fmt.Errorf("could not parse PDU Session Establishment Reject: %v", err)
	}

	logger.UeLogger.Debug(
		"Received PDU Session Establishment Reject NAS message",
		zap.String("IMSI", ue.UeSecurity.Supi),
		zap.Uint8("PDU Session ID", uint8(rej.PDUSessionID)),
		zap.String("Cause", cause5GSMToString(rej.Cause)),
	)

	return nil
}
