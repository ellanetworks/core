// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package smf

import (
	"context"
	"fmt"

	"github.com/ellanetworks/core/etsi"
)

// HandlePagingFailure runs when the AMF abandons paging for an unreachable 5GS UE
// (TS 23.502 §4.2.3.3): the SMF keeps the session's downlink data notification
// suppressed so subsequent downlink packets do not re-page the UE. Paging resumes
// only when the UE returns and the user plane is reactivated.
func (s *SMF) HandlePagingFailure(ctx context.Context, supi etsi.SUPI, pduSessionID uint8) error {
	smContext := s.currentSession(supi, pduSessionID)
	if smContext == nil {
		return fmt.Errorf("no session for %s pdu %d", supi.String(), pduSessionID)
	}

	s.suppressDownlinkDataNotification(ctx, smContext)

	return nil
}

// HandleEPSPagingFailure runs when the MME abandons paging for an unreachable EPS
// UE (TS 23.401 §5.3.4.3): the anchor keeps the session's downlink data
// notification suppressed so subsequent downlink packets do not re-page the UE.
// Paging resumes only when the UE returns and the bearer is reactivated.
func (s *SMF) HandleEPSPagingFailure(ctx context.Context, imsi string, ebi uint8) error {
	supi, err := etsi.NewSUPIFromIMSI(imsi)
	if err != nil {
		return fmt.Errorf("invalid imsi %q: %w", imsi, err)
	}

	smContext := s.currentSession(supi, ebi)
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

	s.upf.SuppressDownlinkDataNotification(ctx, pfcp.RemoteSEID)
}
