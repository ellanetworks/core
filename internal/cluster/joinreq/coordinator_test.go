// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package joinreq_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/ellanetworks/core/internal/cluster/joinreq"
)

func TestSubmitRefusedWhenClosed(t *testing.T) {
	t.Parallel()

	c := joinreq.New()

	if err := c.Submit(joinreq.Request{Mode: joinreq.ModeJoin, Token: "t"}); !errors.Is(err, joinreq.ErrNotAccepting) {
		t.Fatalf("a closed coordinator must refuse submissions, got %v", err)
	}

	if got := c.Status().State; got != joinreq.StateUnavailable {
		t.Errorf("state should be unavailable before Open, got %q", got)
	}

	c.Open()
	c.Close()

	if err := c.Submit(joinreq.Request{Mode: joinreq.ModeJoin, Token: "t"}); !errors.Is(err, joinreq.ErrNotAccepting) {
		t.Fatalf("a coordinator closed after opening must refuse submissions, got %v", err)
	}
}

func TestSubmitIsAcceptedOnceAtATime(t *testing.T) {
	t.Parallel()

	c := joinreq.New()
	c.Open()

	if err := c.Submit(joinreq.Request{Mode: joinreq.ModeJoin, Token: "one"}); err != nil {
		t.Fatalf("first submission: %v", err)
	}

	if got := c.Status().State; got != joinreq.StateJoining {
		t.Errorf("state should be joining after a submission, got %q", got)
	}

	if err := c.Submit(joinreq.Request{Mode: joinreq.ModeJoin, Token: "two"}); !errors.Is(err, joinreq.ErrJoinInProgress) {
		t.Fatalf("a second submission must be refused while one runs, got %v", err)
	}
}

func TestFailedJoinReturnsToWaitingWithReason(t *testing.T) {
	t.Parallel()

	c := joinreq.New()
	c.Open()

	if err := c.Submit(joinreq.Request{Mode: joinreq.ModeJoin, Token: "one"}); err != nil {
		t.Fatalf("submit: %v", err)
	}

	req, err := c.Await(context.Background())
	if err != nil {
		t.Fatalf("await: %v", err)
	}

	if req.Token != "one" {
		t.Fatalf("wrong request delivered: %q", req.Token)
	}

	c.Report(errors.New("seed refused the token"))

	st := c.Status()
	if st.State != joinreq.StateWaiting {
		t.Errorf("a failed join must leave the node waiting for a retry, got %q", st.State)
	}

	if st.Error != "seed refused the token" {
		t.Errorf("the failure reason must survive for the operator, got %q", st.Error)
	}

	if err := c.Submit(joinreq.Request{Mode: joinreq.ModeJoin, Token: "two"}); err != nil {
		t.Fatalf("a retry after a failure must be accepted, got %v", err)
	}

	if c.Status().Error != "" {
		t.Error("a new attempt must clear the previous failure")
	}
}

func TestSuccessfulJoinSticksAcrossClose(t *testing.T) {
	t.Parallel()

	c := joinreq.New()
	c.Open()
	c.Report(nil)
	c.Close()

	if got := c.Status().State; got != joinreq.StateJoined {
		t.Fatalf("a joined node must keep reporting joined, got %q", got)
	}
}

func TestAwaitUnblocksOnShutdown(t *testing.T) {
	t.Parallel()

	c := joinreq.New()
	c.Open()

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	if _, err := c.Await(ctx); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Await must return when the process is shutting down, got %v", err)
	}
}

func TestSubmitRejectsUnknownMode(t *testing.T) {
	t.Parallel()

	c := joinreq.New()
	c.Open()

	if err := c.Submit(joinreq.Request{Token: "t"}); !errors.Is(err, joinreq.ErrUnknownMode) {
		t.Fatalf("a request with no mode must be refused, got %v", err)
	}
}

func TestBootstrapIsSubmittable(t *testing.T) {
	t.Parallel()

	c := joinreq.New()
	c.Open()

	if err := c.Submit(joinreq.Request{Mode: joinreq.ModeBootstrap}); err != nil {
		t.Fatalf("founding a cluster must be submittable without a token: %v", err)
	}

	req, err := c.Await(context.Background())
	if err != nil {
		t.Fatalf("await: %v", err)
	}

	if req.Mode != joinreq.ModeBootstrap {
		t.Fatalf("mode should survive delivery, got %q", req.Mode)
	}
}
