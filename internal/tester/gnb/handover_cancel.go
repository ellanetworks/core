// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package gnb

import (
	"fmt"
	"time"

	"github.com/ellanetworks/core/internal/tester/logger"
	"github.com/ellanetworks/core/ngap"
	"go.uber.org/zap"
)

func (g *GnodeB) SendHandoverCancel(amfUENGAPID, ranUENGAPID int64, cause ngap.Cause) error {
	msg := &ngap.HandoverCancel{
		AMFUENGAPID: ngap.AMFUENGAPID(amfUENGAPID),
		RANUENGAPID: ngap.RANUENGAPID(ranUENGAPID),
		Cause:       &cause,
	}

	pdu, err := msg.Marshal()
	if err != nil {
		return fmt.Errorf("couldn't build HandoverCancel: %w", err)
	}

	if err := g.SendMessage(pdu, NGAPProcedureHandoverCancel); err != nil {
		return fmt.Errorf("couldn't send HandoverCancel: %w", err)
	}

	logger.GnbLogger.Debug("Sent Handover Cancel",
		zap.Int64("AMF UE NGAP ID", amfUENGAPID),
		zap.Int64("RAN UE NGAP ID", ranUENGAPID),
	)

	return nil
}

func (g *GnodeB) WaitForHandoverCancelAcknowledge(timeout time.Duration) (*ngap.HandoverCancelAcknowledge, error) {
	frame, err := g.WaitForMessage(Successful, ngap.ProcHandoverCancel, timeout)
	if err != nil {
		return nil, fmt.Errorf("gnb: await Handover Cancel Acknowledge: %w", err)
	}

	ack, err := ngap.ParseHandoverCancelAcknowledge(frame.Value)
	if err != nil {
		return nil, fmt.Errorf("gnb: parse Handover Cancel Acknowledge: %w", err)
	}

	return ack, nil
}
