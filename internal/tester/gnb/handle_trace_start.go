// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package gnb

import (
	"fmt"
	"time"

	"github.com/ellanetworks/core/ngap"
)

func handleTraceStart(gnb *GnodeB, value []byte) error {
	msg, err := ngap.ParseTraceStart(value)
	if err != nil {
		return fmt.Errorf("undecodable TraceStart: %w", err)
	}

	if msg.TraceActivation == nil {
		return fmt.Errorf("TraceStart without Trace Activation")
	}

	gnb.mu.Lock()
	defer gnb.mu.Unlock()

	if gnb.traceIDs == nil {
		gnb.traceIDs = make(map[int64]ngap.NGRANTraceID)
	}

	if _, held := gnb.traceIDs[int64(msg.RANUENGAPID)]; !held {
		gnb.traceIDs[int64(msg.RANUENGAPID)] = msg.TraceActivation.NGRANTraceID
	}

	return nil
}

func (g *GnodeB) recordHandoverTraceID(amfUENGAPID int64, id ngap.NGRANTraceID) {
	g.mu.Lock()
	defer g.mu.Unlock()

	if g.handoverTraceIDs == nil {
		g.handoverTraceIDs = make(map[int64]ngap.NGRANTraceID)
	}

	g.handoverTraceIDs[amfUENGAPID] = id
}

func (g *GnodeB) forgetTraceID(ranUENGAPID int64) {
	g.mu.Lock()
	defer g.mu.Unlock()

	delete(g.traceIDs, ranUENGAPID)
}

func (g *GnodeB) TraceID(ranUENGAPID int64) (ngap.NGRANTraceID, bool) {
	g.mu.Lock()
	defer g.mu.Unlock()

	id, ok := g.traceIDs[ranUENGAPID]

	return id, ok
}

func (g *GnodeB) HandoverTraceID(amfUENGAPID int64) (ngap.NGRANTraceID, bool) {
	g.mu.Lock()
	defer g.mu.Unlock()

	id, ok := g.handoverTraceIDs[amfUENGAPID]

	return id, ok
}

func (g *GnodeB) WaitForTraceStart(timeout time.Duration) (*ngap.TraceStart, error) {
	frame, err := g.WaitForMessage(Initiating, ngap.ProcTraceStart, timeout)
	if err != nil {
		return nil, fmt.Errorf("gnb: await Trace Start: %w", err)
	}

	return ngap.ParseTraceStart(frame.Value)
}
