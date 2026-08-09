// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/s1ap"
	"go.uber.org/zap"
)

func HandoverBearers(ue *UeContext) (bearers []s1ap.ERABToBeSetupItemHOReq, candidates []HandoverCandidate, ok bool) {
	ue.mu.Lock()
	defer ue.mu.Unlock()

	bearers = make([]s1ap.ERABToBeSetupItemHOReq, 0, len(ue.Pdns))
	candidates = make([]HandoverCandidate, 0, len(ue.Pdns))

	for _, p := range ue.Pdns {
		sgwTLA, err := models.EncodeTransportLayerAddress(p.SgwFTEID.Addr, p.SgwN3IPv6)
		if err != nil {
			logger.MmeLog.Error("failed to encode S-GW transport layer address for handover",
				zap.String("imsi", ue.imsiOrEmpty()), zap.Uint8("e-rab-id", p.Ebi), zap.Error(err))

			candidates = append(candidates, HandoverCandidate{Ebi: p.Ebi, Cause: &causeHandoverCNReason})

			continue
		}

		candidates = append(candidates, HandoverCandidate{Ebi: p.Ebi})

		bearers = append(bearers, s1ap.ERABToBeSetupItemHOReq{
			ERABID:                s1ap.ERABID(p.Ebi),
			TransportLayerAddress: s1ap.TransportLayerAddress(sgwTLA),
			GTPTEID:               s1ap.GTPTEID(p.SgwFTEID.TEID),
			QoS: s1ap.ERABLevelQoSParameters{
				QCI: s1ap.QCI(p.Qci),
				ARP: BearerARP(p.Arp),
			},
		})
	}

	return bearers, candidates, len(bearers) > 0
}
