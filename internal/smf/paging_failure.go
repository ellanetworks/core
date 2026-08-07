// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package smf

import (
	"context"
	"fmt"

	"github.com/ellanetworks/core/etsi"
)

func (s *SMF) HandlePagingFailure(ctx context.Context, supi etsi.SUPI, pduSessionID uint8) error {
	smContext := s.currentPDUSession(supi, pduSessionID)
	if smContext == nil {
		return fmt.Errorf("no session for %s pdu %d", supi.String(), pduSessionID)
	}

	return s.suppressDownlinkDataNotification(ctx, smContext.Ref, Access5G)
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

	return s.suppressDownlinkDataNotification(ctx, smContext.Ref, Access4G)
}

func (s *SMF) suppressDownlinkDataNotification(ctx context.Context, ref string, access AccessType) error {
	smContext, unlock, err := s.sessionFor(ref, access)
	if err != nil {
		return err
	}

	defer unlock()

	if smContext.UPFSession == nil {
		return nil
	}

	s.upf.SuppressDownlinkDataNotification(ctx, smContext.UPFSession.SEID)

	return nil
}
