// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1enb

import (
	"fmt"
	"time"

	"github.com/ellanetworks/core/s1ap"
)

func (e *ENB) WaitForTraceStart(enbUEID int64, timeout time.Duration) (*s1ap.TraceStart, error) {
	f, err := e.WaitForMessage(enbUEID, Initiating, s1ap.ProcTraceStart, timeout)
	if err != nil {
		return nil, fmt.Errorf("s1enb: await Trace Start: %w", err)
	}

	return s1ap.ParseTraceStart(f.Value)
}
