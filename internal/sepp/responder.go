// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package sepp

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/ellanetworks/core/internal/roaming/wire"
	"go.uber.org/zap"
)

func problemBody(pd wire.ProblemDetails) (int, []byte) {
	body, err := json.Marshal(pd)
	if err != nil {
		return http.StatusInternalServerError, nil
	}

	return pd.Status, body
}

func (m *Manager) otherPeerLocked(c *n32cContext, sender string) {
	if _, known := c.others[sender]; known {
		go m.closeInboundFrom(c.partner.id, sender)
	} else {
		m.logger.Info("N32-c context established with another SEPP of the partner", zap.String("partner", c.partner.name), zap.String("sepp", sender))
	}

	if c.others == nil {
		c.others = map[string]time.Time{}
	}

	c.others[sender] = m.now()
}

func (m *Manager) respond(partnerID string, dnsNames []string, body []byte) (int, []byte) {
	snap := m.snap.Load()

	p, ok := snap.partners[partnerID]
	if !ok {
		return problemBody(wire.NewProblem(http.StatusForbidden, CauseNegotiationNotAllowed, "the partner has no 5G agreement"))
	}

	reject := func(pd wire.ProblemDetails) (int, []byte) {
		m.logger.Warn("Rejected an N32-c exchange-capability", zap.String("partner", p.name), zap.Int("status", pd.Status), zap.String("cause", string(pd.Cause)), zap.String("detail", pd.Detail))

		return problemBody(pd)
	}

	decoded, err := wire.Decode[SecNegotiateReqData](body)
	if err != nil {
		return reject(wire.ProblemFromError(err))
	}

	if !snap.usable() {
		return reject(wire.NewProblem(http.StatusForbidden, CauseNegotiationNotAllowed, snap.problem))
	}

	neg := m.negotiation(p, snap.identity, snap.op, dnsNames)

	rsp, err := neg.Respond(decoded)
	if err != nil {
		return reject(wire.ProblemFromError(err))
	}

	m.mu.Lock()

	if m.root == nil {
		m.mu.Unlock()
		return problemBody(wire.NewProblem(http.StatusServiceUnavailable, wire.CauseNFCongestion, ErrNotReady.Error()))
	}

	c, ok := m.contexts[p.id]
	if !ok {
		c = &n32cContext{status: ContextStatus{State: StateIdle, Since: m.now()}, partner: p}
		m.contexts[p.id] = c
	}

	sender := strings.ToLower(strings.TrimSuffix(decoded.Sender, "."))

	switch {
	case sameFQDN(sender, p.fqdn) || (c.peer != nil && sameFQDN(sender, c.peer.fqdn)):
		var peer *seppPeer
		if !sameFQDN(sender, p.fqdn) {
			peer = c.peer
		}

		if c.inflight && LocalPrecedes(snap.identity.fqdn, sender) {
			m.mu.Unlock()
			return reject(wire.NewProblem(http.StatusConflict, CauseN32cExchangeCapabilityOngoing, "this SEPP's own exchange-capability is in progress"))
		}

		c.gen++
		m.establishedLocked(c, InitiatorPeer, peer)
	default:
		m.otherPeerLocked(c, sender)
	}

	m.mu.Unlock()

	out, err := wire.Encode(rsp)
	if err != nil {
		return problemBody(wire.NewProblem(http.StatusInternalServerError, wire.CauseSystemFailure, ""))
	}

	return http.StatusOK, out
}
