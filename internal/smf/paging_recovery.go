// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package smf

import (
	"context"
	"fmt"

	"github.com/ellanetworks/core/etsi"
)

func (s *SMF) ClearPagingSuppression(ctx context.Context, supi etsi.SUPI, pduSessionID uint8) error {
	smContext := s.currentPDUSession(supi, pduSessionID)
	if smContext == nil {
		return nil
	}

	s.clearDownlinkDataNotification(ctx, smContext.Ref, Access5G)

	return nil
}

func (s *SMF) ClearEPSPagingSuppression(ctx context.Context, imsi string, ebi uint8) error {
	supi, err := etsi.NewSUPIFromIMSI(imsi)
	if err != nil {
		return fmt.Errorf("invalid imsi %q: %w", imsi, err)
	}

	smContext := s.currentEPSSession(supi, ebi)
	if smContext == nil {
		return nil
	}

	s.clearDownlinkDataNotification(ctx, smContext.Ref, Access4G)

	return nil
}

// A session that has moved holds no suppression on the access it left.
func (s *SMF) clearDownlinkDataNotification(ctx context.Context, ref string, access AccessType) {
	smContext, unlock, err := s.sessionFor(ref, access)
	if err != nil {
		return
	}

	defer unlock()

	if smContext.UPFSession == nil {
		return
	}

	s.upf.ClearDownlinkDataNotification(ctx, smContext.UPFSession.SEID)
}
