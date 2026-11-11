// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package runtime

import (
	"encoding/json"
	"net/http"

	"github.com/ellanetworks/core/internal/roaming/wire"
	"github.com/ellanetworks/core/internal/sepp"
)

type seppCallbacks struct {
	amf http.Handler
	smf http.Handler
}

func (c seppCallbacks) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	peer, ok := sepp.PeerFrom(r.Context())
	if !ok {
		writeCallbackProblem(w, wire.NewProblem(http.StatusNotFound, wire.CauseResourceURIStructureNotFound, "not a roaming callback"))
		return
	}

	var h http.Handler

	switch peer.Callback.NF {
	case sepp.CallbackAMF:
		h = c.amf
	case sepp.CallbackSMF:
		h = c.smf
	}

	if h == nil {
		writeCallbackProblem(w, peer.Callback.Kind.NotFound("no UE context for this callback"))
		return
	}

	h.ServeHTTP(w, r)
}

func writeCallbackProblem(w http.ResponseWriter, pd wire.ProblemDetails) {
	w.Header().Set("Content-Type", wire.ContentTypeProblem)
	w.WriteHeader(pd.Status)
	_ = json.NewEncoder(w).Encode(pd)
}
