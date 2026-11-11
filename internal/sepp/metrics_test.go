// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package sepp

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"testing"

	"github.com/ellanetworks/core/internal/metrics"
)

type timeoutError struct{}

func (timeoutError) Error() string   { return "i/o timeout" }
func (timeoutError) Timeout() bool   { return true }
func (timeoutError) Temporary() bool { return true }

func TestN32Result(t *testing.T) {
	cases := map[string]struct {
		resp *http.Response
		err  error
		want string
	}{
		"ok":                 {&http.Response{StatusCode: http.StatusOK}, nil, metrics.PeerSuccess},
		"redirect":           {&http.Response{StatusCode: http.StatusTemporaryRedirect}, nil, metrics.PeerSuccess},
		"not found":          {&http.Response{StatusCode: http.StatusNotFound}, nil, metrics.PeerFailure},
		"server error":       {&http.Response{StatusCode: http.StatusGatewayTimeout}, nil, metrics.PeerFailure},
		"deadline":           {nil, fmt.Errorf("%w x: %w", ErrUnreachable, context.DeadlineExceeded), metrics.PeerTimeout},
		"net timeout":        {nil, fmt.Errorf("%w x: %w", ErrUnreachable, timeoutError{}), metrics.PeerTimeout},
		"no context":         {nil, ErrNoContext, metrics.PeerError},
		"connection refused": {nil, errors.New("connection refused"), metrics.PeerError},
	}

	for name, tc := range cases {
		if got := n32Result(tc.resp, tc.err); got != tc.want {
			t.Errorf("%s: n32Result = %q, want %q", name, got, tc.want)
		}
	}
}
