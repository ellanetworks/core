// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ue

import (
	"fmt"

	"github.com/ellanetworks/core/internal/tester/logger"
	"github.com/ellanetworks/core/nas/fgs"
	"go.uber.org/zap"
)

func handlePDUSessionModificationCommand(ue *UE, payload []byte, amfUENGAPID int64, ranUENGAPID int64) error {
	cmd, err := fgs.ParsePDUSessionModificationCommand(payload)
	if err != nil {
		return fmt.Errorf("could not parse PDU Session Modification Command: %v", err)
	}

	pduSessionID := uint8(cmd.PDUSessionID)

	complete, err := BuildPDUSessionModificationComplete(&PDUSessionModificationCompleteOpts{
		PDUSessionID: pduSessionID,
		PTI:          uint8(cmd.PTI),
	})
	if err != nil {
		return fmt.Errorf("could not build PDU Session Modification Complete: %v", err)
	}

	uplink, err := BuildUplinkNasTransportSM(pduSessionID, complete)
	if err != nil {
		return fmt.Errorf("could not build Uplink NAS Transport for PDU Session Modification Complete: %v", err)
	}

	encodedPdu, err := ue.EncodeNasPduWithSecurity(uplink, uint8(fgs.SHTIntegrityProtectedCiphered))
	if err != nil {
		return fmt.Errorf("error encoding %s IMSI UE NAS PDU Session Modification Complete Msg", ue.UeSecurity.Supi)
	}

	if err := ue.Gnb.SendUplinkNAS(encodedPdu, amfUENGAPID, ranUENGAPID); err != nil {
		return fmt.Errorf("could not send UplinkNASTransport for PDU Session Modification Complete: %v", err)
	}

	logger.UeLogger.Debug(
		"Sent PDU Session Modification Complete",
		zap.String("IMSI", ue.UeSecurity.Supi),
		zap.Uint8("PDU Session ID", pduSessionID),
		zap.Uint8("PTI", uint8(cmd.PTI)),
		zap.Bool("Always-on indication", cmd.AlwaysOn != nil),
	)

	return nil
}
