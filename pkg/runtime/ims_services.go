// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package runtime

import (
	"context"
	"fmt"
	"math"
	"slices"

	"github.com/ellanetworks/core/diameter/cx"
	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/hss"
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

type imsProfileNotifier struct {
	db  *db.Database
	hss *hss.HSS
}

func (n *imsProfileNotifier) ApplicationServerChanged(ctx context.Context, applicationServerID string) {
	go func() {
		services, err := n.db.ListIMSServices(ctx)
		if err != nil {
			n.warn("list IMS services", err)
			return
		}

		var serviceIDs []string

		for _, svc := range services {
			if svc.ApplicationServerID == applicationServerID {
				serviceIDs = append(serviceIDs, svc.ID)
			}
		}

		n.pushServices(ctx, serviceIDs)
	}()
}

func (n *imsProfileNotifier) ServiceChanged(ctx context.Context, serviceID string) {
	go n.pushServices(ctx, []string{serviceID})
}

func (n *imsProfileNotifier) ProfileChanged(ctx context.Context, profileID string) {
	go n.pushProfiles(ctx, []string{profileID})
}

func (n *imsProfileNotifier) SubscriberChanged(ctx context.Context, imsi string) {
	go n.push(ctx, []string{imsi})
}

func (n *imsProfileNotifier) pushServices(ctx context.Context, serviceIDs []string) {
	profileIDs, err := n.db.ProfilesWithIMSServices(ctx, serviceIDs)
	if err != nil {
		n.warn("list the profiles of IMS services", err)
		return
	}

	n.pushProfiles(ctx, profileIDs)
}

func (n *imsProfileNotifier) pushProfiles(ctx context.Context, profileIDs []string) {
	imsis, err := n.db.IMSRegisteredSubscribersOfProfiles(ctx, profileIDs)
	if err != nil {
		n.warn("list the IMS registered subscribers of profiles", err)
		return
	}

	n.push(ctx, imsis)
}

func (n *imsProfileNotifier) push(ctx context.Context, imsis []string) {
	for _, imsi := range slices.Compact(slices.Sorted(slices.Values(imsis))) {
		_ = n.hss.PushProfile(ctx, imsi)
	}
}

func (n *imsProfileNotifier) warn(what string, err error) {
	logger.HssLog.Warn("IMS profile push: failed to "+what, zap.Error(err))
}
