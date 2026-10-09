// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"maps"
	"slices"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/s1ap"
	"go.uber.org/zap"
)

func HandoverBearers(ue *UeContext, forwarding bool) (bearers []s1ap.ERABToBeSetupItemHOReq, candidates []HandoverCandidate, ok bool) {
	ue.mu.Lock()
	defer ue.mu.Unlock()

	bearers = make([]s1ap.ERABToBeSetupItemHOReq, 0, len(ue.Pdns))
	candidates = make([]HandoverCandidate, 0, len(ue.Pdns))

	for _, p := range ue.Pdns {
		sgwTLA, err := models.EncodeTransportLayerAddress(p.SgwFTEID.Addr, p.SgwN3IPv6)
		if err != nil {
			logger.MmeLog.Error("failed to encode S-GW transport layer address for handover",
				logger.SUPI(ue.supiOrEmpty()), logger.ERABID(p.Ebi), zap.Error(err))

			candidates = append(candidates, HandoverCandidate{Ebi: p.Ebi, Cause: &causeHandoverUnspecified})

			continue
		}

		candidates = append(candidates, HandoverCandidate{Ebi: p.Ebi})

		bearer := s1ap.ERABToBeSetupItemHOReq{
			ERABID:                s1ap.ERABID(p.Ebi),
			TransportLayerAddress: s1ap.TransportLayerAddress(sgwTLA),
			GTPTEID:               s1ap.GTPTEID(p.SgwFTEID.TEID),
			QoS: s1ap.ERABLevelQoSParameters{
				QCI: s1ap.QCI(p.Qci),
				ARP: BearerARP(p.Arp),
			},
		}

		if !forwarding {
			bearer.Extensions = &s1ap.ERABToBeSetupItemHOReqExtIEs{
				DataForwardingNotPossible: s1ap.Ptr(s1ap.DataForwardingNotPossibleTrue),
			}
		}

		bearers = append(bearers, bearer)

		for _, ebi := range slices.Sorted(maps.Keys(p.Dedicated)) {
			b := p.Dedicated[ebi]
			if b.Activating || b.Deactivating {
				continue
			}

			item, err := dedicatedHandoverItem(b, forwarding)
			if err != nil {
				logger.MmeLog.Error("failed to encode S-GW transport layer address for handover",
					logger.SUPI(ue.supiOrEmpty()), logger.ERABID(ebi), zap.Error(err))

				candidates = append(candidates, HandoverCandidate{Ebi: ebi, Cause: &causeHandoverUnspecified})

				continue
			}

			candidates = append(candidates, HandoverCandidate{Ebi: ebi})
			bearers = append(bearers, item)
		}
	}

	return bearers, candidates, len(bearers) > 0
}

func dedicatedHandoverItem(b *DedicatedBearer, forwarding bool) (s1ap.ERABToBeSetupItemHOReq, error) {
	sgwTLA, err := models.EncodeTransportLayerAddress(b.SgwFTEID.Addr, b.SgwN3IPv6)
	if err != nil {
		return s1ap.ERABToBeSetupItemHOReq{}, err
	}

	item := s1ap.ERABToBeSetupItemHOReq{
		ERABID:                s1ap.ERABID(b.Ebi),
		TransportLayerAddress: s1ap.TransportLayerAddress(sgwTLA),
		GTPTEID:               s1ap.GTPTEID(b.SgwFTEID.TEID),
		QoS:                   DedicatedERABQoS(&b.DedicatedBearerInfo),
	}

	if !forwarding {
		item.Extensions = &s1ap.ERABToBeSetupItemHOReqExtIEs{
			DataForwardingNotPossible: s1ap.Ptr(s1ap.DataForwardingNotPossibleTrue),
		}
	}

	return item, nil
}
