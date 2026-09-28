// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1enb

import (
	"fmt"
	"time"

	"github.com/ellanetworks/core/s1ap"
)

func (e *ENB) AcceptUEContextModification(enbUEID int64, timeout time.Duration) (*s1ap.UEContextModificationRequest, error) {
	frame, err := e.WaitForMessage(enbUEID, Initiating, s1ap.ProcUEContextModification, timeout)
	if err != nil {
		return nil, fmt.Errorf("await UE Context Modification Request: %w", err)
	}

	req, err := s1ap.ParseUEContextModificationRequest(frame.Value)
	if err != nil {
		return nil, fmt.Errorf("parse UE Context Modification Request: %w", err)
	}

	b, err := (&s1ap.UEContextModificationResponse{
		MMEUES1APID: s1ap.Ptr(req.MMEUES1APID),
		ENBUES1APID: s1ap.Ptr(req.ENBUES1APID),
	}).Marshal()
	if err != nil {
		return nil, fmt.Errorf("build UE Context Modification Response: %w", err)
	}

	if err := e.SendMessage(b, true); err != nil {
		return nil, fmt.Errorf("send UE Context Modification Response: %w", err)
	}

	return req, nil
}
