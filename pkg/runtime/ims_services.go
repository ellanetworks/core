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
	services, err := database.ListProfileIMSServices(ctx, profileID)
	if err != nil {
		return nil, fmt.Errorf("list the IMS services of profile %s: %w", profileID, err)
	}

	var out []cx.InitialFilterCriteria

	serverNames := make(map[string]string)

	for _, svc := range services {
		serverName, ok := serverNames[svc.ApplicationServerID]
		if !ok {
			as, err := database.GetIMSApplicationServerByID(ctx, svc.ApplicationServerID)
			if err != nil {
				return nil, fmt.Errorf("get the application server of IMS service %s: %w", svc.Name, err)
			}

			serverName = as.ServerName
			serverNames[svc.ApplicationServerID] = serverName
		}

		for _, ifc := range svc.FilterCriteria() {
			if len(out) >= math.MaxInt32 {
				return nil, fmt.Errorf("profile %s has too many initial filter criteria", profileID)
			}

			out = append(out, ifc.CX(int32(len(out)), serverName))
		}
	}

	return out, nil
}

type profilePusher interface {
	PushProfile(ctx context.Context, imsi string) error
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
		services, err := n.db.ListIMSServices(ctx)
		if err != nil {
			return nil, fmt.Errorf("list IMS services: %w", err)
		}

		var serviceIDs []string

		for _, svc := range services {
			if svc.ApplicationServerID == applicationServerID {
				serviceIDs = append(serviceIDs, svc.ID)
			}
		}

		return n.subscribersOfServices(ctx, serviceIDs)
	})
}

func (n *imsProfileNotifier) ServiceChanged(_ context.Context, serviceID string) {
	n.enqueue(func(ctx context.Context) ([]string, error) {
		return n.subscribersOfServices(ctx, []string{serviceID})
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

func (n *imsProfileNotifier) subscribersOfServices(ctx context.Context, serviceIDs []string) ([]string, error) {
	profileIDs, err := n.db.ProfilesWithIMSServices(ctx, serviceIDs)
	if err != nil {
		return nil, fmt.Errorf("list the profiles of IMS services: %w", err)
	}

	return n.db.IMSRegisteredSubscribersOfProfiles(ctx, profileIDs)
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
