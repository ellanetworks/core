// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package roaming

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/ellanetworks/core/diameter"
	"github.com/ellanetworks/core/diameter/s6a"
	"github.com/ellanetworks/core/diameter/tgpp"
	"github.com/ellanetworks/core/internal/metrics"
)

func TestS6aResult(t *testing.T) {
	req := &diameter.Message{Flags: diameter.FlagRequest, CommandCode: s6a.CommandUpdateLocation, ApplicationID: s6a.ApplicationID}
	id := diameter.Identity{OriginHost: "mme.example.org", OriginRealm: "example.org"}

	cases := map[string]struct {
		ans  *diameter.Message
		err  error
		want string
	}{
		"success":      {tgpp.NewResultAnswer(req, id, tgpp.Result{Code: diameter.ResultSuccess}), nil, metrics.PeerSuccess},
		"user unknown": {tgpp.NewResultAnswer(req, id, tgpp.Experimental(tgpp.ResultErrorUserUnknown)), nil, metrics.PeerFailure},
		"timeout":      {nil, fmt.Errorf("send: %w", context.DeadlineExceeded), metrics.PeerTimeout},
		"no peer":      {nil, errors.New("no route"), metrics.PeerError},
	}

	for name, tc := range cases {
		if got := s6aResult(tc.ans, tc.err); got != tc.want {
			t.Errorf("%s: s6aResult = %q, want %q", name, got, tc.want)
		}
	}
}
