// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package gnb

import (
	"fmt"
	"time"

	"github.com/ellanetworks/core/ngap"
)

func handleTraceStart(_ *GnodeB, value []byte) error {
	msg, err := ngap.ParseTraceStart(value)
	if err != nil {
		return fmt.Errorf("undecodable TraceStart: %w", err)
	}

	if msg.TraceActivation == nil {
		return fmt.Errorf("TraceStart without Trace Activation")
	}

	return nil
}

func (g *GnodeB) WaitForTraceStart(timeout time.Duration) (*ngap.TraceStart, error) {
	frame, err := g.WaitForMessage(Initiating, ngap.ProcTraceStart, timeout)
	if err != nil {
		return nil, fmt.Errorf("gnb: await Trace Start: %w", err)
	}

	return ngap.ParseTraceStart(frame.Value)
}
