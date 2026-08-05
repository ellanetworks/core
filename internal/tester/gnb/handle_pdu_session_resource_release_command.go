// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package gnb

import (
	"fmt"

	"github.com/ellanetworks/core/internal/tester/logger"
	"github.com/ellanetworks/core/ngap"
	"go.uber.org/zap"
)

func handlePDUSessionResourceReleaseCommand(gnb *GnodeB, value []byte) error {
	cmd, err := ngap.ParsePDUSessionResourceReleaseCommand(value)
	if err != nil {
		return fmt.Errorf("undecodable PDUSessionResourceReleaseCommand: %w", err)
	}

	amfUeNgapID, ranUeNgapID := int64(cmd.AMFUENGAPID), int64(cmd.RANUENGAPID)

	logger.GnbLogger.Debug(
		"Received PDU Session Resource Release Command",
		zap.String("GNB ID", gnb.GnbID),
		zap.Int64("RAN UE NGAP ID", ranUeNgapID),
		zap.Int64("AMF UE NGAP ID", amfUeNgapID),
		zap.Int("PDU Sessions to release", len(cmd.PDUSessionResourceRelease)),
	)

	if cmd.NASPDU != nil {
		ue, err := gnb.LoadUE(ranUeNgapID)
		if err != nil {
			return fmt.Errorf("could not load UE with RAN UE NGAP ID %d: %w", ranUeNgapID, err)
		}

		if err := ue.SendDownlinkNAS(*cmd.NASPDU, amfUeNgapID, ranUeNgapID); err != nil {
			return fmt.Errorf("forward NAS PDU for release command: %w", err)
		}
	}

	ids := make([]int64, 0, len(cmd.PDUSessionResourceRelease))

	for _, item := range cmd.PDUSessionResourceRelease {
		pduSessionID := int64(item.PDUSessionID)
		ids = append(ids, pduSessionID)
		gnb.RemovePDUSession(ranUeNgapID, pduSessionID)

		logger.GnbLogger.Debug(
			"Released PDU session",
			zap.Int64("PDU Session ID", pduSessionID),
			zap.Int64("RAN UE NGAP ID", ranUeNgapID),
		)
	}

	if err := gnb.SendPDUSessionResourceReleaseResponse(&PDUSessionResourceReleaseResponseOpts{
		AMFUENGAPID:   amfUeNgapID,
		RANUENGAPID:   ranUeNgapID,
		PDUSessionIDs: ids,
	}); err != nil {
		return fmt.Errorf("failed to send PDUSessionResourceReleaseResponse: %w", err)
	}

	logger.GnbLogger.Debug(
		"Sent PDU Session Resource Release Response",
		zap.String("GNB ID", gnb.GnbID),
		zap.Int64("RAN UE NGAP ID", ranUeNgapID),
		zap.Int64("AMF UE NGAP ID", amfUeNgapID),
	)

	return nil
}
