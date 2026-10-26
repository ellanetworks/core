// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package runtime

import (
	"context"
	"fmt"
	"sync"

	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/logger"
	"go.uber.org/zap"
)

type imsReconciler interface {
	Reconcile(ctx context.Context, imsi string) error
}

type imsProfileNotifier struct {
	db  *db.Database
	hss imsReconciler

	mu      sync.Mutex
	jobs    []func(context.Context) ([]string, error)
	pending []string
	queued  map[string]bool
	wake    chan struct{}
}

func newIMSProfileNotifier(database *db.Database, hss imsReconciler) *imsProfileNotifier {
	return &imsProfileNotifier{db: database, hss: hss, queued: make(map[string]bool), wake: make(chan struct{}, 1)}
}

func (n *imsProfileNotifier) ApplicationServerChanged(_ context.Context, applicationServerID string) {
	n.enqueue(func(ctx context.Context) ([]string, error) {
		profileIDs, err := n.db.ProfilesUsingIMSApplicationServer(ctx, applicationServerID)
		if err != nil {
			return nil, fmt.Errorf("list the profiles using application server %s: %w", applicationServerID, err)
		}

		return n.db.IMSRegisteredSubscribersOfProfiles(ctx, profileIDs)
	})
}

func (n *imsProfileNotifier) ProfileChanged(_ context.Context, profileID string) {
	n.enqueue(func(ctx context.Context) ([]string, error) {
		return n.db.IMSRegisteredSubscribersOfProfiles(ctx, []string{profileID})
	})
}

func (n *imsProfileNotifier) SubscriberChanged(_ context.Context, imsi string) {
	n.enqueue(func(context.Context) ([]string, error) {
		return []string{imsi}, nil
	})
}

func (n *imsProfileNotifier) AllSubscribersChanged(_ context.Context) {
	n.enqueue(func(ctx context.Context) ([]string, error) {
		regs, err := n.db.ListAllIMSRegistrations(ctx)
		if err != nil {
			return nil, fmt.Errorf("list the IMS registrations: %w", err)
		}

		imsis := make([]string, 0, len(regs))

		for _, r := range regs {
			if r.State != db.IMSNotRegistered {
				imsis = append(imsis, r.IMSI)
			}
		}

		return imsis, nil
	})
}

func (n *imsProfileNotifier) enqueue(job func(context.Context) ([]string, error)) {
	n.mu.Lock()
	n.jobs = append(n.jobs, job)
	n.mu.Unlock()

	select {
	case n.wake <- struct{}{}:
	default:
	}
}

func (n *imsProfileNotifier) run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-n.wake:
		}

		for n.step(ctx) {
			if ctx.Err() != nil {
				return
			}
		}
	}
}

func (n *imsProfileNotifier) step(ctx context.Context) bool {
	n.mu.Lock()
	jobs := n.jobs
	n.jobs = nil
	n.mu.Unlock()

	for _, job := range jobs {
		imsis, err := job(ctx)
		if err != nil {
			logger.HssLog.Warn("IMS subscription change: failed to find the affected subscribers", zap.Error(err))
			continue
		}

		n.mu.Lock()
		for _, imsi := range imsis {
			if !n.queued[imsi] {
				n.queued[imsi] = true
				n.pending = append(n.pending, imsi)
			}
		}
		n.mu.Unlock()
	}

	n.mu.Lock()
	if len(n.pending) == 0 {
		more := len(n.jobs) > 0
		n.mu.Unlock()

		return more
	}

	imsi := n.pending[0]
	n.pending = n.pending[1:]
	delete(n.queued, imsi)
	n.mu.Unlock()

	if err := n.hss.Reconcile(ctx, imsi); err != nil {
		logger.HssLog.Debug("IMS subscription change not applied", zap.String("imsi", imsi), zap.Error(err))
	}

	return true
}
