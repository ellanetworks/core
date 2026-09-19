// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package runtime

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/ellanetworks/core/internal/db"
)

type fakePublisher struct {
	mu      sync.Mutex
	calls   int
	results []error
}

func (f *fakePublisher) PublishClusterMemberAttributes(_ context.Context, _ string) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.calls++

	if len(f.results) == 0 {
		return nil
	}

	err := f.results[0]
	f.results = f.results[1:]

	return err
}

func (f *fakePublisher) callCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()

	return f.calls
}

func shortPublishBackoff(t *testing.T) {
	t.Helper()

	initial, maxBackoff := attributePublishInitialBackoff, attributePublishMaxBackoff
	attributePublishInitialBackoff = time.Millisecond
	attributePublishMaxBackoff = 5 * time.Millisecond

	t.Cleanup(func() {
		attributePublishInitialBackoff, attributePublishMaxBackoff = initial, maxBackoff
	})
}

func TestPublishClusterMemberAttributesRetriesUntilItLands(t *testing.T) {
	shortPublishBackoff(t)

	publisher := &fakePublisher{results: []error{
		fmt.Errorf("leader unreachable"),
		fmt.Errorf("leader unreachable"),
		nil,
	}}

	publishClusterMemberAttributes(context.Background(), publisher, "1.18.0")

	if got := publisher.callCount(); got != 3 {
		t.Fatalf("publish attempts = %d, want 3", got)
	}
}

func TestPublishClusterMemberAttributesRetriesWhenTheLeaderPredatesTheOperation(t *testing.T) {
	shortPublishBackoff(t)

	publisher := &fakePublisher{results: []error{
		fmt.Errorf("unknown operation: %w", db.ErrForwardRejected),
		nil,
	}}

	publishClusterMemberAttributes(context.Background(), publisher, "1.18.0")

	if got := publisher.callCount(); got != 2 {
		t.Fatalf("publish attempts = %d, want 2: a leader that predates the operation must not end the loop", got)
	}
}

func TestPublishClusterMemberAttributesStopsWhenTheNodeWasRemoved(t *testing.T) {
	shortPublishBackoff(t)

	publisher := &fakePublisher{results: []error{
		fmt.Errorf("node-id 2 is gone: %w", db.ErrRemovedFromCluster),
		nil,
	}}

	publishClusterMemberAttributes(context.Background(), publisher, "1.18.0")

	if got := publisher.callCount(); got != 1 {
		t.Fatalf("publish attempts = %d, want 1: a removed node must stop publishing", got)
	}
}

func TestPublishClusterMemberAttributesStopsOnContextCancellation(t *testing.T) {
	shortPublishBackoff(t)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	publisher := &fakePublisher{results: []error{errors.New("leader unreachable")}}

	done := make(chan struct{})

	go func() {
		publishClusterMemberAttributes(ctx, publisher, "1.18.0")
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("publish loop did not stop on context cancellation")
	}
}
