// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package client_test

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/ellanetworks/core/client"
)

type scriptedRequester struct {
	statuses []string
	calls    int
}

func (s *scriptedRequester) Do(_ context.Context, _ *client.RequestOptions) (*client.RequestResponse, error) {
	i := s.calls
	s.calls++

	if i >= len(s.statuses) {
		i = len(s.statuses) - 1
	}

	return &client.RequestResponse{
		StatusCode: 200,
		Headers:    http.Header{},
		Result:     []byte(s.statuses[i]),
	}, nil
}

func datapathStatus(policyIndex uint64) string {
	return fmt.Sprintf(`{"datapath":{"appliedPolicyIndex":%d,"appliedSettingsIndex":0}}`, policyIndex)
}

func TestWaitForDatapathPolicy_ReturnsOnceTheIndexIsApplied(t *testing.T) {
	fake := &scriptedRequester{statuses: []string{
		datapathStatus(4),
		datapathStatus(6),
		datapathStatus(9),
	}}

	c := &client.Client{Requester: fake}

	if err := c.WaitForDatapathPolicy(context.Background(), 9); err != nil {
		t.Fatalf("wait: %v", err)
	}

	if fake.calls != 3 {
		t.Errorf("polled %d times, want 3", fake.calls)
	}
}

func TestWaitForDatapathPolicy_ReturnsWhenTheNodeReportsNoDatapath(t *testing.T) {
	fake := &scriptedRequester{statuses: []string{`{"version":"v1"}`}}

	c := &client.Client{Requester: fake}

	if err := c.WaitForDatapathPolicy(context.Background(), 9); err != nil {
		t.Fatalf("wait: %v", err)
	}
}

func TestWaitForDatapathPolicy_ZeroIndexDoesNotPoll(t *testing.T) {
	fake := &scriptedRequester{statuses: []string{datapathStatus(0)}}

	c := &client.Client{Requester: fake}

	if err := c.WaitForDatapathPolicy(context.Background(), 0); err != nil {
		t.Fatalf("wait: %v", err)
	}

	if fake.calls != 0 {
		t.Errorf("polled %d times, want 0", fake.calls)
	}
}

func TestWaitForDatapathPolicy_StopsWhenTheIndexGoesBackwards(t *testing.T) {
	fake := &scriptedRequester{statuses: []string{
		datapathStatus(7),
		datapathStatus(0),
	}}

	c := &client.Client{Requester: fake}

	if err := c.WaitForDatapathPolicy(context.Background(), 9); err != nil {
		t.Fatalf("wait: %v", err)
	}

	if fake.calls != 2 {
		t.Errorf("polled %d times, want 2", fake.calls)
	}
}

func TestWaitForDatapathPolicy_TimesOutWhileTheIndexLags(t *testing.T) {
	fake := &scriptedRequester{statuses: []string{datapathStatus(1)}}

	c := &client.Client{Requester: fake}

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	err := c.WaitForDatapathPolicy(ctx, 9)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("wait error = %v, want context.DeadlineExceeded", err)
	}
}
