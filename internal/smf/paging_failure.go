// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package smf

import (
	"context"
	"fmt"

	"github.com/ellanetworks/core/etsi"
)

// pagedOn refuses a session that has since moved off the access the paging key
// names. The PDU session identity survives a move to EPS, so it can still
// resolve a session the other access now serves.
func pagedOn(sc *SMContext, access AccessType) error {
	sc.Mutex.Lock()
	defer sc.Mutex.Unlock()

	if sc.Access != access {
		return fmt.Errorf("session %q is on %s", sc.Ref, sc.Access)
	}

	return nil
}

func (s *SMF) HandlePagingFailure(ctx context.Context, supi etsi.SUPI, pduSessionID uint8) error {
	smContext := s.currentPDUSession(supi, pduSessionID)
	if smContext == nil {
		return fmt.Errorf("no session for %s pdu %d", supi.String(), pduSessionID)
	}

	if err := pagedOn(smContext, Access5G); err != nil {
		return err
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

	if err := pagedOn(smContext, Access4G); err != nil {
		return err
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

	s.upf.SuppressDownlinkDataNotification(ctx, pfcp.RemoteSEID)
}
