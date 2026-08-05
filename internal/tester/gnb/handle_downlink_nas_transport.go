// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package gnb

import (
	"fmt"

	"github.com/ellanetworks/core/internal/tester/logger"
	"github.com/ellanetworks/core/ngap"
	"go.uber.org/zap"
)

func handleDownlinkNASTransport(gnb *GnodeB, value []byte) error {
	msg, err := ngap.ParseDownlinkNASTransport(value)
	if err != nil {
		return fmt.Errorf("undecodable DownlinkNASTransport: %w", err)
	}

	amfUENGAPID, ranUENGAPID := int64(msg.AMFUENGAPID), int64(msg.RANUENGAPID)

	logger.GnbLogger.Debug("Received DownlinkNASTransport",
		zap.Int64("AMFUENGAPID", amfUENGAPID),
		zap.Int64("RANUENGAPID", ranUENGAPID),
	)

	gnb.UpdateNGAPIDs(ranUENGAPID, amfUENGAPID)

	ue, err := gnb.LoadUE(ranUENGAPID)
	if err != nil {
		return fmt.Errorf("cannot find UE for DownlinkNASTransport message: %w", err)
	}

	if err := ue.SendDownlinkNAS(msg.NASPDU, amfUENGAPID, ranUENGAPID); err != nil {
		return fmt.Errorf("HandleDownlinkNASTransport failed: %w", err)
	}

	return nil
}
