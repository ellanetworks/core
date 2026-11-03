// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package roaming

import (
	"context"

	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/smf"
	"github.com/ellanetworks/core/internal/udm"
)

type SessionSubscriptions struct {
	smf.SubscriptionData
	v *VSMF
}

func (v *VSMF) Subscriptions(fallback smf.SubscriptionData) *SessionSubscriptions {
	return &SessionSubscriptions{SubscriptionData: fallback, v: v}
}

type vsmfHomeEntry struct {
	snssai      models.Snssai
	dnn         string
	sessionAMBR *models.Ambr
}

func (v *VSMF) homeEntries(imsi string) ([]vsmfHomeEntry, models.PlmnID, bool) {
	v.mu.Lock()
	defer v.mu.Unlock()

	ue := v.ues[imsi]
	if ue == nil || len(ue.data) == 0 {
		return nil, models.PlmnID{}, false
	}

	entries := make([]vsmfHomeEntry, 0, len(ue.data))

	for _, d := range ue.data {
		if d.subscribed {
			entries = append(entries, vsmfHomeEntry{snssai: d.snssai, dnn: d.dnn, sessionAMBR: d.sessionAMBR})
		}
	}

	return entries, ue.visitor.HomePLMN.Model(), true
}

func (s *SessionSubscriptions) SessionManagement(ctx context.Context, imsi string) (*udm.SessionManagementSubscription, error) {
	entries, homePLMN, ok := s.v.homeEntries(imsi)
	if !ok {
		return s.SubscriptionData.SessionManagement(ctx, imsi)
	}

	local, err := s.v.local.SessionManagement(ctx, imsi)
	if err != nil || !local.Visitor {
		return local, err
	}

	merged := vsmfMerge(local, entries, homePLMN)

	if eps, err := s.SubscriptionData.SessionManagement(ctx, imsi); err == nil {
		for _, c := range eps.DNNs {
			if _, ok := merged.ForDNN(c.Snssai, c.DNN); !ok {
				merged.DNNs = append(merged.DNNs, c)
			}
		}
	}

	return merged, nil
}

func vsmfMerge(local *udm.SessionManagementSubscription, entries []vsmfHomeEntry, homePLMN models.PlmnID) *udm.SessionManagementSubscription {
	out := &udm.SessionManagementSubscription{Visitor: true, HomePLMN: homePLMN}

	for _, c := range local.DNNs {
		if c.IMS {
			continue
		}

		key := smKeyOf(HomeSnssai(c.Snssai), c.DNN)

		for _, e := range entries {
			if smKeyOf(e.snssai, e.dnn) != key {
				continue
			}

			if e.sessionAMBR != nil {
				c.SessionAMBR = c.SessionAMBR.Min(*e.sessionAMBR)
			}

			out.DNNs = append(out.DNNs, c)

			break
		}
	}

	return out
}
