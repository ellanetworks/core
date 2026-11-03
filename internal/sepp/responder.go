// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package sepp

import (
	"encoding/json"
	"net/http"
	"strings"

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

func (m *Manager) otherPeerLocked(c *n32cContext, sender string, outcome Outcome, nftlst bool) string {
	_, known := c.others[sender]

	if outcome == OutcomeTeardown {
		if known {
			delete(c.others, sender)

			go m.closeInboundFrom(c.partner.id, sender)

			m.logger.Info("N32-c context with another SEPP of the partner terminated", zap.String("partner", c.partner.name), zap.String("sepp", sender))
		}

		return resultTeardown
	}

	if known {
		go m.closeInboundFrom(c.partner.id, sender)
	} else {
		m.logger.Info("N32-c context established with another SEPP of the partner", zap.String("partner", c.partner.name), zap.String("sepp", sender))
	}

	if c.others == nil {
		c.others = map[string]PeerContext{}
	}

	c.others[sender] = PeerContext{Peer: sender, Since: m.now(), NFTLST: nftlst}

	return resultEstablished
}

func (m *Manager) respond(partnerID string, dnsNames []string, body []byte) (int, []byte) {
	snap := m.snap.Load()

	p, ok := snap.partners[partnerID]
	if !ok {
		return problemBody(wire.NewProblem(http.StatusForbidden, CauseNegotiationNotAllowed, "the partner has no 5G agreement"))
	}

	reject := func(result string, pd wire.ProblemDetails) (int, []byte) {
		handshakes.WithLabelValues(p.name, roleResponder, result).Inc()
		m.logger.Warn("Rejected an N32-c exchange-capability", zap.String("partner", p.name), zap.Int("status", pd.Status), zap.String("cause", string(pd.Cause)), zap.String("detail", pd.Detail))

		return problemBody(pd)
	}

	decoded, err := wire.Decode[SecNegotiateReqData](body)
	if err != nil {
		return reject(resultRejected, wire.ProblemFromError(err))
	}

	if !snap.usable() || !p.usable() {
		reason := snap.problem
		if reason == "" {
			reason = p.problem
		}

		return reject(resultRejected, wire.NewProblem(http.StatusForbidden, CauseNegotiationNotAllowed, reason))
	}

	neg := m.negotiation(p, snap.identity, snap.op, dnsNames)

	rsp, outcome, err := neg.Respond(decoded)
	if err != nil {
		return reject(resultRejected, wire.ProblemFromError(err))
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

	result := resultEstablished
	sender := strings.ToLower(strings.TrimSuffix(decoded.Sender, "."))
	nftlst := rsp.SupportedFeatures.Has(FeatureNFTLST)

	switch {
	case sameFQDN(sender, p.fqdn) || (c.peer != nil && sameFQDN(sender, c.peer.fqdn)):
		var peer *seppPeer
		if !sameFQDN(sender, p.fqdn) {
			peer = c.peer
		}

		switch outcome {
		case OutcomeTeardown:
			result = resultTeardown

			if c.status.State == StateEstablished {
				c.gen++
				m.setState(p, c, StateTerminated, "terminated by the partner SEPP")
				m.retryLocked(p.id, c)
			}
		case OutcomeEstablish:
			if c.inflight && LocalPrecedes(snap.identity.fqdn, sender) {
				m.mu.Unlock()
				return reject(resultConflict, wire.NewProblem(http.StatusConflict, CauseN32cExchangeCapabilityOngoing, "this SEPP's own exchange-capability is in progress"))
			}

			c.gen++
			m.establishedLocked(c, InitiatorPeer, nftlst, snap.identity, peer)
		}
	default:
		result = m.otherPeerLocked(c, sender, outcome, nftlst)
	}

	m.mu.Unlock()

	handshakes.WithLabelValues(p.name, roleResponder, result).Inc()

	out, err := wire.Encode(rsp)
	if err != nil {
		return problemBody(wire.NewProblem(http.StatusInternalServerError, wire.CauseSystemFailure, ""))
	}

	return http.StatusOK, out
}
