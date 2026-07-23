// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package smf

import (
	"context"
	"fmt"

	"github.com/ellanetworks/core/etsi"
)

func (s *SMF) ClearPagingSuppression(ctx context.Context, supi etsi.SUPI, pduSessionID uint8) error {
	smContext := s.currentSession(supi, pduSessionID)
	if smContext == nil {
		return nil
	}

	s.clearDownlinkDataNotification(ctx, smContext)

	return nil
}

func (s *SMF) ClearEPSPagingSuppression(ctx context.Context, imsi string, ebi uint8) error {
	supi, err := etsi.NewSUPIFromIMSI(imsi)
	if err != nil {
		return fmt.Errorf("invalid imsi %q: %w", imsi, err)
	}

	smContext := s.currentSession(supi, ebi)
	if smContext == nil {
		return nil
	}

	s.clearDownlinkDataNotification(ctx, smContext)

	return nil
}

func (s *SMF) clearDownlinkDataNotification(ctx context.Context, smContext *SMContext) {
	smContext.Mutex.Lock()
	pfcp := smContext.PFCPContext
	smContext.Mutex.Unlock()

	if pfcp == nil {
		return
	}

	s.upf.ClearDownlinkDataNotification(ctx, pfcp.RemoteSEID)
}
