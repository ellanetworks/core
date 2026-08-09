// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/s1ap"
	"go.uber.org/zap"
)

// HandoverBearers snapshots the UE's PDN connections into the E-RABs To Be Setup
// list of a HANDOVER REQUEST (TS 36.413 §9.1.5.4), reporting false when the UE has
// no usable bearer.
func HandoverBearers(ue *UeContext) ([]s1ap.ERABToBeSetupItemHOReq, bool) {
	ue.mu.Lock()
	defer ue.mu.Unlock()

	bearers := make([]s1ap.ERABToBeSetupItemHOReq, 0, len(ue.Pdns))

	for _, p := range ue.Pdns {
		sgwTLA, err := models.EncodeTransportLayerAddress(p.SgwFTEID.Addr, p.SgwN3IPv6)
		if err != nil {
			// imsiOrEmpty, not IMSI(): ue.mu is held and is not reentrant.
			logger.MmeLog.Error("failed to encode S-GW transport layer address for handover",
				zap.String("imsi", ue.imsiOrEmpty()), zap.Uint8("e-rab-id", p.Ebi), zap.Error(err))

			continue
		}

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

	return bearers, len(bearers) > 0
}
