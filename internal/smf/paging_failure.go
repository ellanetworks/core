// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package smf

import (
	"context"
	"fmt"

	"github.com/ellanetworks/core/etsi"
)

// HandlePagingFailure re-arms downlink-data paging for a 5G PDU session after the
// AMF abandons paging, so continued downlink data raises a fresh notification and
// pages the UE again instead of staying buffered (TS 23.502 §4.2.3.3).
func (s *SMF) HandlePagingFailure(ctx context.Context, supi etsi.SUPI, pduSessionID uint8) error {
	smContext := s.currentSession(supi, pduSessionID)
	if smContext == nil {
		return fmt.Errorf("no session for %s pdu %d", supi.String(), pduSessionID)
	}

	s.resetDownlinkDataNotification(ctx, smContext)

	return nil
}

// HandleEPSPagingFailure re-arms downlink-data paging for a 4G PDN connection after
// the MME abandons paging, so continued downlink data pages the UE again instead of
// staying buffered (TS 23.401 §5.3.4.3).
func (s *SMF) HandleEPSPagingFailure(ctx context.Context, imsi string, ebi uint8) error {
	supi, err := etsi.NewSUPIFromIMSI(imsi)
	if err != nil {
		return fmt.Errorf("invalid imsi %q: %w", imsi, err)
	}

	smContext := s.currentSession(supi, ebi)
	if smContext == nil {
		return fmt.Errorf("no EPS session for %s", imsi)
	}

	s.resetDownlinkDataNotification(ctx, smContext)

	return nil
}

// resetDownlinkDataNotification clears the UPF's buffered-data notification state
// for the session so the next downlink packet raises a fresh Downlink Data
// Notification. The 3GPP anchor sends this to the user plane on paging failure.
func (s *SMF) resetDownlinkDataNotification(ctx context.Context, smContext *SMContext) {
	smContext.Mutex.Lock()
	pfcp := smContext.PFCPContext
	smContext.Mutex.Unlock()

	if pfcp == nil {
		return
	}

	s.upf.ResetDownlinkDataNotification(ctx, pfcp.RemoteSEID)
}
