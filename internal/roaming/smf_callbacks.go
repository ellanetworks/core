// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package roaming

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"path"

	"github.com/ellanetworks/core/etsi"
	"github.com/ellanetworks/core/internal/roaming/wire"
	"github.com/ellanetworks/core/internal/sepp"
	"github.com/ellanetworks/core/internal/smf"
	"go.uber.org/zap"
)

const maxSMFCallbackBody = 64 << 10

func writeSMFProblem(w http.ResponseWriter, pd wire.ProblemDetails) {
	w.Header().Set("Content-Type", wire.ContentTypeProblem)
	w.WriteHeader(pd.Status)
	_ = json.NewEncoder(w).Encode(pd)
}

func (v *VSMF) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	peer, ok := v.peer(r.Context())
	if !ok || peer.Callback.NF != sepp.CallbackSMF {
		writeSMFProblem(w, wire.NewProblem(http.StatusNotFound, wire.CauseResourceURIStructureNotFound, "not an SMF callback"))
		return
	}

	kind := peer.Callback.Kind

	imsi, ok := wire.IMSIFromSupi(peer.Callback.Supi)
	if !ok {
		writeSMFProblem(w, kind.NotFound("unknown SUPI"))
		return
	}

	supi, err := etsi.NewSUPIFromIMSI(imsi)
	if err != nil {
		writeSMFProblem(w, kind.NotFound("unknown SUPI"))
		return
	}

	body, err := io.ReadAll(io.LimitReader(r.Body, maxSMFCallbackBody+1))
	if err != nil || len(body) > maxSMFCallbackBody {
		writeSMFProblem(w, wire.NewProblem(http.StatusRequestEntityTooLarge, wire.CauseMaxJSONSizeExceeded, "request body too large"))
		return
	}

	switch kind {
	case sepp.CallbackSDMChange:
		v.serveDataChange(r.Context(), w, peer, supi, body)
	case sepp.CallbackUECMDereg:
		v.serveDeregistration(r.Context(), w, peer, supi, body)
	default:
		writeSMFProblem(w, wire.NewProblem(http.StatusNotFound, wire.CauseResourceURIStructureNotFound, "not an SMF callback"))
	}
}

func (v *VSMF) serveDataChange(ctx context.Context, w http.ResponseWriter, peer sepp.Peer, supi etsi.SUPI, body []byte) {
	n, err := wire.Decode[wire.ModificationNotification](body)
	if err != nil {
		writeSMFProblem(w, wire.ProblemFromError(err))
		return
	}

	if !v.subscribedTo(peer.PartnerID, supi, n.SubscriptionID) {
		writeSMFProblem(w, sepp.CallbackSDMChange.NotFound("no SDM subscription for this UE"))
		return
	}

	w.WriteHeader(http.StatusNoContent)

	changed := false

	for _, item := range n.NotifyItems {
		changed = changed || item.About(supi.String(), vsmfSMDataResource)
	}

	if !changed {
		return
	}

	ctx = context.WithoutCancel(ctx)

	v.queue.Go(supi.IMSI(), func() {
		v.refresh(ctx, supi)

		if sessions := v.deps.Load().sessions; sessions != nil {
			go sessions.ReconcileHPLMNBoundSessions(ctx, supi)
		}
	})
}

func (v *VSMF) subscribedTo(partnerID string, supi etsi.SUPI, subscriptionID string) bool {
	v.mu.Lock()
	defer v.mu.Unlock()

	ue := v.ues[supi.IMSI()]
	if ue == nil || ue.target.PartnerID != partnerID {
		return false
	}

	for _, d := range ue.data {
		if d.subscription == "" {
			continue
		}

		if subscriptionID == "" || path.Base(d.subscription) == subscriptionID {
			return true
		}
	}

	return false
}

func (v *VSMF) serveDeregistration(ctx context.Context, w http.ResponseWriter, peer sepp.Peer, supi etsi.SUPI, body []byte) {
	d, err := wire.Decode[wire.DeregistrationData](body)
	if err != nil {
		writeSMFProblem(w, wire.ProblemFromError(err))
		return
	}

	if d.PduSessionID == nil {
		writeSMFProblem(w, wire.ProblemFromError(wire.Missing("/pduSessionId")))
		return
	}

	how := deregistrationRelease(d.DeregReason)

	v.mu.Lock()

	var s *vsmfSession

	if ue := v.ues[supi.IMSI()]; ue != nil && ue.target.PartnerID == peer.PartnerID && *d.PduSessionID >= 1 && *d.PduSessionID <= 15 {
		s = ue.sessions[uint8(*d.PduSessionID)]
	}

	if s == nil || s.closed || !s.registered {
		v.mu.Unlock()
		writeSMFProblem(w, sepp.CallbackUECMDereg.NotFound("no SMF registration for this PDU session"))

		return
	}

	s.released = how != smf.HPLMNReleaseReactivate
	v.mu.Unlock()

	w.WriteHeader(http.StatusNoContent)

	v.logger.Info("The home UDM deregistered the visitor's PDU session", zap.String("supi", supi.String()), zap.Uint8("pduSessionID", s.psi), zap.String("reason", string(d.DeregReason)))

	sessions := v.deps.Load().sessions
	if sessions == nil {
		s.Close()
		return
	}

	ctx = context.WithoutCancel(ctx)

	go func() {
		if !sessions.ReleaseHPLMNBoundSession(ctx, supi, s.psi, s, how) {
			s.Close()
		}
	}()
}

func vsmfSMDataResource(supi string) string {
	return supi + "/sm-data"
}

func deregistrationRelease(reason wire.DeregistrationReason) smf.HPLMNRelease {
	switch reason {
	case wire.DeregPDUSessionReactivationRequired:
		return smf.HPLMNReleaseReactivate
	case wire.DeregDuplicatePDUSession, wire.DeregSMFContextTransferred:
		return smf.HPLMNReleaseDuplicate
	case wire.DeregDuplicatePDUSessionEPDG:
		return smf.HPLMNReleaseDuplicateNotify
	default:
		return smf.HPLMNReleaseDeactivate
	}
}
