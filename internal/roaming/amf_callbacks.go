// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package roaming

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/roaming/wire"
	"github.com/ellanetworks/core/internal/sepp"
	"go.uber.org/zap"
)

const maxAMFCallbackBody = 64 << 10

type AMFHomeEvents interface {
	HomeDeregistration(ctx context.Context, partnerID, imsi string, data wire.DeregistrationData) error
	HomeSubscriptionChanged(ctx context.Context, partnerID, imsi string, n wire.ModificationNotification) error
}

type AMFCallbacks struct {
	events AMFHomeEvents
	logger *zap.Logger
	peer   func(context.Context) (sepp.Peer, bool)
}

func NewAMFCallbacks(events AMFHomeEvents, logger *zap.Logger) *AMFCallbacks {
	return &AMFCallbacks{events: events, logger: logger, peer: sepp.PeerFrom}
}

func (c *AMFCallbacks) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	peer, ok := c.peer(r.Context())
	if !ok || peer.Callback.NF != sepp.CallbackAMF {
		writeAMFProblem(w, wire.NewProblem(http.StatusNotFound, wire.CauseResourceURIStructureNotFound, "not an AMF callback"))
		return
	}

	kind := peer.Callback.Kind

	imsi, ok := wire.IMSIFromSupi(peer.Callback.Supi)
	if !ok {
		writeAMFProblem(w, kind.NotFound("unknown SUPI"))
		return
	}

	body, err := io.ReadAll(io.LimitReader(r.Body, maxAMFCallbackBody+1))
	if err != nil || len(body) > maxAMFCallbackBody {
		writeAMFProblem(w, wire.NewProblem(http.StatusRequestEntityTooLarge, wire.CauseMaxJSONSizeExceeded, "request body too large"))
		return
	}

	switch kind {
	case sepp.CallbackUECMDereg:
		data, err := wire.Decode[wire.DeregistrationData](body)
		if err != nil {
			writeAMFProblem(w, wire.ProblemFromError(err))
			return
		}

		err = c.events.HomeDeregistration(r.Context(), peer.PartnerID, imsi, data)
		c.answer(w, kind, imsi, err)
	case sepp.CallbackSDMChange:
		n, err := wire.Decode[wire.ModificationNotification](body)
		if err != nil {
			writeAMFProblem(w, wire.ProblemFromError(err))
			return
		}

		err = c.events.HomeSubscriptionChanged(r.Context(), peer.PartnerID, imsi, n)
		c.answer(w, kind, imsi, err)
	default:
		writeAMFProblem(w, wire.NewProblem(http.StatusNotFound, wire.CauseResourceURIStructureNotFound, "not an AMF callback"))
	}
}

func (c *AMFCallbacks) answer(w http.ResponseWriter, kind sepp.CallbackKind, imsi string, err error) {
	switch {
	case err == nil:
		w.WriteHeader(http.StatusNoContent)
	case errors.Is(err, ErrNoVisitorContext):
		writeAMFProblem(w, kind.NotFound("no UE context for this callback"))
	default:
		c.logger.Warn("failed to handle a home network notification", zap.String("kind", string(kind)), logger.SUPIFromIMSI(imsi), zap.Error(err))
		writeAMFProblem(w, wire.NewProblem(http.StatusInternalServerError, wire.CauseSystemFailure, ""))
	}
}

func writeAMFProblem(w http.ResponseWriter, pd wire.ProblemDetails) {
	w.Header().Set("Content-Type", wire.ContentTypeProblem)
	w.WriteHeader(pd.Status)
	_ = json.NewEncoder(w).Encode(pd)
}
