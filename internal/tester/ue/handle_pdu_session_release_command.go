// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ue

import (
	"fmt"

	"github.com/ellanetworks/core/internal/tester/logger"
	"github.com/ellanetworks/core/nas/fgs"
	"go.uber.org/zap"
)

// handlePDUSessionReleaseCommand answers a PDU SESSION RELEASE COMMAND with a
// PDU SESSION RELEASE COMPLETE (TS 24.501 §6.3.3.3). Until the completion
// arrives the SMF keeps the procedure outstanding and retransmits the command
// on T3592, so a UE that stays silent leaves the session half-released.
func handlePDUSessionReleaseCommand(ue *UE, payload []byte, amfUENGAPID int64, ranUENGAPID int64) error {
	cmd, err := fgs.ParsePDUSessionReleaseCommand(payload)
	if err != nil {
		return fmt.Errorf("could not parse PDU Session Release Command: %v", err)
	}

	pduSessionID := uint8(cmd.PDUSessionID)

	complete, err := BuildPDUSessionReleaseComplete(&PDUSessionReleaseCompleteOpts{
		PDUSessionID: pduSessionID,
		PTI:          uint8(cmd.PTI),
	})
	if err != nil {
		return fmt.Errorf("could not build PDU Session Release Complete: %v", err)
	}

	uplink, err := BuildUplinkNasTransportSM(pduSessionID, complete)
	if err != nil {
		return fmt.Errorf("could not build Uplink NAS Transport for PDU Session Release Complete: %v", err)
	}

	encodedPdu, err := ue.EncodeNasPduWithSecurity(uplink, uint8(fgs.SHTIntegrityProtectedCiphered))
	if err != nil {
		return fmt.Errorf("error encoding %s IMSI UE NAS PDU Session Release Complete Msg", ue.UeSecurity.Supi)
	}

	ue.DropPDUSession(pduSessionID)

	if err := ue.Gnb.SendUplinkNAS(encodedPdu, amfUENGAPID, ranUENGAPID); err != nil {
		return fmt.Errorf("could not send UplinkNASTransport for PDU Session Release Complete: %v", err)
	}

	logger.UeLogger.Debug(
		"Sent PDU Session Release Complete",
		zap.String("IMSI", ue.UeSecurity.Supi),
		zap.Uint8("PDU Session ID", pduSessionID),
		zap.Uint8("PTI", uint8(cmd.PTI)),
		zap.Uint8("Cause", uint8(cmd.Cause)),
	)

	return nil
}
