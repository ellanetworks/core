// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1enb

import (
	"fmt"
	"time"

	"github.com/ellanetworks/core/nas/eps"
)

func (e *ENB) answerGUTIReallocation(ue *UE, mmeUEID, enbUEID int64, timeout time.Duration) (*eps.EPSMobileIdentity, error) {
	deadline := time.Now().Add(timeout)

	for {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			return nil, fmt.Errorf("s1enb: await GUTI Reallocation Command: timed out")
		}

		wire, _, err := e.WaitForDownlinkNAS(enbUEID, remaining)
		if err != nil {
			return nil, fmt.Errorf("s1enb: await GUTI Reallocation Command: %w", err)
		}

		plain, err := ue.unprotectDownlink(wire)
		if err != nil {
			return nil, fmt.Errorf("s1enb: unprotect downlink NAS: %w", err)
		}

		mt, err := eps.PeekMessageType(plain)
		if err != nil {
			return nil, fmt.Errorf("s1enb: peek downlink NAS: %w", err)
		}

		if mt != eps.MsgGUTIReallocationCommand {
			continue
		}

		cmd, err := eps.ParseGUTIReallocationCommand(plain)
		if err != nil {
			return nil, fmt.Errorf("s1enb: parse GUTI Reallocation Command: %w", err)
		}

		complete, err := (&eps.GUTIReallocationComplete{}).MarshalBinary()
		if err != nil {
			return nil, fmt.Errorf("s1enb: build GUTI Reallocation Complete: %w", err)
		}

		protected, err := ue.protectUplink(complete)
		if err != nil {
			return nil, fmt.Errorf("s1enb: protect GUTI Reallocation Complete: %w", err)
		}

		if err := e.SendUplinkNASTransport(mmeUEID, enbUEID, protected); err != nil {
			return nil, fmt.Errorf("s1enb: send GUTI Reallocation Complete: %w", err)
		}

		guti := cmd.GUTI

		return &guti, nil
	}
}
