// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package sepp

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"slices"
	"strconv"

	"github.com/ellanetworks/core/internal/roaming/wire"
	"go.uber.org/zap"
)

const (
	RejectNoContext       = "no_context"
	RejectRateLimited     = "rate_limited"
	RejectTargetAPIRoot   = "target_apiroot"
	RejectOriginatingPLMN = "originating_network_id"
	RejectNotAllowed      = "not_allowed"
	RejectForeignSupi     = "foreign_supi"
	RejectOtherNode       = "other_node"
	RejectBodyTooLarge    = "body_too_large"
)

type Peer struct {
	PartnerID string
	PLMNs     []wire.PlmnID
	Callback  Callback
}

type peerKey struct{}

func PeerFrom(ctx context.Context) (Peer, bool) {
	p, ok := ctx.Value(peerKey{}).(Peer)
	return p, ok
}

func (m *Manager) reject(w http.ResponseWriter, p *partner, reason string, pd wire.ProblemDetails) {
	n32fRejected.WithLabelValues(p.name, reason).Inc()
	n32fRequests.WithLabelValues(p.name, directionInbound, "", strconv.Itoa(pd.Status)).Inc()
	m.logger.Warn("Rejected an inbound N32-f request", zap.String("partner", p.name), zap.String("reason", reason), zap.Int("status", pd.Status), zap.String("detail", pd.Detail))
	m.writeProblem(w, pd)
}

func (m *Manager) serveN32f(w http.ResponseWriter, r *http.Request, partnerID string, dnsNames []string) {
	snap, p, err := m.partner(partnerID)
	if err != nil {
		m.writeProblem(w, wire.NewProblem(http.StatusForbidden, wire.CauseContextNotFound, "no N32-c context with this SEPP"))
		return
	}

	if !snap.usable() || !m.correlated(partnerID, dnsNames) {
		m.reject(w, p, RejectNoContext, wire.NewProblem(http.StatusForbidden, wire.CauseContextNotFound, "no N32-c context with this SEPP"))
		return
	}

	if !m.n32fLimit.allow(partnerID, m.now(), m.tuning.N32fRequestsPerSecond) {
		w.Header().Set("Retry-After", strconv.Itoa(n32fRateRetryAfterS))
		m.reject(w, p, RejectRateLimited, wire.NewProblem(http.StatusTooManyRequests, wire.CauseNFCongestionRisk, "too many N32-f requests"))

		return
	}

	root, err := ParseTargetAPIRoot(r.Header.Values(HeaderTargetAPIRoot))
	if err != nil {
		m.reject(w, p, RejectTargetAPIRoot, wire.ProblemFromError(err))
		return
	}

	if root.Scheme != "https" || root.Prefix != "" {
		m.reject(w, p, RejectTargetAPIRoot, wire.ProblemFromError(headerError(HeaderTargetAPIRoot, "%s is not an Ella Core callback apiRoot", root)))
		return
	}

	if _, _, err := ParseCallbackHost(root.Authority, snap.op); err != nil {
		m.reject(w, p, RejectTargetAPIRoot, wire.ProblemFromError(headerError(HeaderTargetAPIRoot, "%s is not an Ella Core callback host", root.Authority)))
		return
	}

	if values := r.Header.Values(HeaderOriginatingNetworkID); len(values) > 0 {
		oni, err := ParseOriginatingNetworkID(values)
		if err != nil {
			m.reject(w, p, RejectOriginatingPLMN, wire.ProblemFromError(err))
			return
		}

		if !slices.ContainsFunc(p.plmns, oni.Matches) {
			m.reject(w, p, RejectOriginatingPLMN, wire.NewProblem(http.StatusForbidden, wire.CauseOriginatingNetworkIDMismatch, "3gpp-Sbi-Originating-Network-Id "+oni.String()+" is not a PLMN of this partner"))
			return
		}
	}

	cb, err := ParseCallback(r.Method, root.Authority, r.URL.EscapedPath(), snap.op)
	if err != nil {
		m.reject(w, p, RejectNotAllowed, wire.NewProblem(http.StatusNotFound, wire.CauseResourceURIStructureNotFound, "not an Ella Core callback URI"))
		return
	}

	if _, ok := wire.HomePLMN(cb.Supi, p.plmns); !ok {
		m.reject(w, p, RejectForeignSupi, wire.NewProblem(http.StatusNotFound, wire.CauseContextNotFound, "the SUPI is not a subscriber of this partner"))
		return
	}

	if cb.SEPPID != snap.identity.seppID {
		m.serveOtherNode(w, snap, p, cb)
		return
	}

	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxBodyBytes))
	if err != nil {
		m.reject(w, p, RejectBodyTooLarge, wire.NewProblem(http.StatusRequestEntityTooLarge, wire.CauseMaxJSONSizeExceeded, "request body too large"))
		return
	}

	req := r.Clone(context.WithValue(r.Context(), peerKey{}, Peer{PartnerID: p.id, PLMNs: slices.Clone(p.plmns), Callback: cb}))
	req.Header.Del(HeaderTargetAPIRoot)
	req.Body = io.NopCloser(bytes.NewReader(body))
	req.ContentLength = int64(len(body))
	req.Host = root.Authority

	rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}

	if h := m.callbacks.Load(); h != nil {
		(*h).ServeHTTP(rec, req)
	} else {
		m.writeProblem(rec, cb.Kind.NotFound("no UE context for this callback"))
	}

	n32fRequests.WithLabelValues(p.name, directionInbound, string(cb.NF), strconv.Itoa(rec.status)).Inc()
}

func (m *Manager) correlated(partnerID string, dnsNames []string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	c, ok := m.contexts[partnerID]

	return ok && c.correlates(dnsNames)
}

func (m *Manager) serveOtherNode(w http.ResponseWriter, snap *snapshot, p *partner, cb Callback) {
	if _, ok := snap.clusterSEPPIDs[cb.SEPPID]; !ok {
		m.reject(w, p, RejectOtherNode, cb.Kind.NotFound("the target NF is not on any Ella Core node"))
		return
	}

	w.Header().Set("Retry-After", strconv.Itoa(otherNodeRetryAfterS))
	m.reject(w, p, RejectOtherNode, wire.NewProblem(http.StatusServiceUnavailable, wire.CauseNFCongestion, "the target NF is on another Ella Core SEPP node: retry so that it reaches that node"))
}

type statusRecorder struct {
	http.ResponseWriter
	status int
	wrote  bool
}

func (s *statusRecorder) WriteHeader(code int) {
	if !s.wrote {
		s.status, s.wrote = code, true
	}

	s.ResponseWriter.WriteHeader(code)
}

func (s *statusRecorder) Write(b []byte) (int, error) {
	s.wrote = true
	return s.ResponseWriter.Write(b)
}
