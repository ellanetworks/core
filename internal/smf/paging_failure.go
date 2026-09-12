// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package smf

import (
	"context"
	"fmt"

	"github.com/ellanetworks/core/etsi"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/models"
	"go.uber.org/zap"
)

func (s *SMF) HandleN1N2TransferFailure(ctx context.Context, supi etsi.SUPI, pduSessionID uint8, cause models.N1N2MessageTransferCause) error {
	smContext := s.currentPDUSession(supi, pduSessionID)
	if smContext == nil {
		return fmt.Errorf("no session for %s pdu %d", supi.String(), pduSessionID)
	}

	logger.SmfLog.Info("N1N2 message transfer failed",
		zap.String("supi", supi.String()),
		zap.Uint8("pdu_session_id", pduSessionID),
		zap.String("cause", cause.String()))

	if cause != models.N1N2UENotResponding {
		return nil
	}

	s.suppressDownlinkDataNotification(ctx, smContext)

	return nil
}

func (s *SMF) HandleEPSPagingFailure(ctx context.Context, imsi string, ebi uint8) error {
	supi, err := etsi.NewSUPIFromIMSI(imsi)
	if err != nil {
		return fmt.Errorf("invalid imsi %q: %w", imsi, err)
	}

	smContext := s.currentEPSSession(supi, ebi)
	if smContext == nil {
		return fmt.Errorf("no EPS session for %s", imsi)
	}

	s.suppressDownlinkDataNotification(ctx, smContext)

	return nil
}

func (s *SMF) suppressDownlinkDataNotification(ctx context.Context, smContext *SMContext) {
	smContext.Mutex.Lock()
	pfcp := smContext.PFCPContext
	smContext.Mutex.Unlock()

	if pfcp == nil {
		return
	}

	s.upf.SuppressDownlinkDataNotification(ctx, pfcp.SEID)
}
