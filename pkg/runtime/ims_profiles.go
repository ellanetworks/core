// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package runtime

import (
	"context"
	"fmt"
	"math"
	"slices"
	"sync"

	"github.com/ellanetworks/core/diameter/cx"
	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/logger"
	"go.uber.org/zap"
)

func profileFilterCriteria(ctx context.Context, database *db.Database, profileID string) ([]cx.InitialFilterCriteria, error) {
	ifcs, err := database.ListProfileIMSFilterCriteria(ctx, profileID)
	if err != nil {
		return nil, fmt.Errorf("list the initial filter criteria of profile %s: %w", profileID, err)
	}

	out := make([]cx.InitialFilterCriteria, 0, len(ifcs))

	serverNames := make(map[string]string)

	for _, ifc := range ifcs {
		serverName, ok := serverNames[ifc.ApplicationServerID]
		if !ok {
			as, err := database.GetIMSApplicationServerByID(ctx, ifc.ApplicationServerID)
			if err != nil {
				return nil, fmt.Errorf("get application server %s: %w", ifc.ApplicationServerID, err)
			}

			serverName = as.ServerName
			serverNames[ifc.ApplicationServerID] = serverName
		}

		if len(out) >= math.MaxInt32 {
			return nil, fmt.Errorf("profile %s has too many initial filter criteria", profileID)
		}

		out = append(out, ifc.CX(int32(len(out)), serverName))
	}

	return out, nil
}

type profilePusher interface {
	PushProfile(ctx context.Context, imsi string) error
	IdentitiesChanged(ctx context.Context, imsi, previousMSISDN string) error
}

type imsProfileNotifier struct {
	db     *db.Database
	pusher profilePusher

	mu      sync.Mutex
	jobs    []func(context.Context) ([]string, error)
	pending []string
	wake    chan struct{}
}

func newIMSProfileNotifier(database *db.Database, pusher profilePusher) *imsProfileNotifier {
	return &imsProfileNotifier{db: database, pusher: pusher, wake: make(chan struct{}, 1)}
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

func (n *imsProfileNotifier) SubscriberIdentitiesChanged(_ context.Context, imsi, previousMSISDN string) {
	n.enqueue(func(ctx context.Context) ([]string, error) {
		_ = n.pusher.IdentitiesChanged(ctx, imsi, previousMSISDN)

		return nil, nil
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
			logger.HssLog.Warn("IMS profile push: failed to find the affected subscribers", zap.Error(err))
			continue
		}

		n.mu.Lock()
		for _, imsi := range imsis {
			if !slices.Contains(n.pending, imsi) {
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
	n.mu.Unlock()

	_ = n.pusher.PushProfile(ctx, imsi)

	return true
}
