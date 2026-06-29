// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/s1ap"
	"go.uber.org/zap"
)

// ensureDefaultPDN promotes the lowest surviving admitted PDN to the UE's default
// when the attach-default PDN was released during a partial-admission handover, so
// a registered UE always retains a default PDN connection (its EPS last-resort
// connectivity, TS 23.401). A no-op when a default still exists or no admitted PDN
// survives.
func EnsureDefaultPDN(ue *UeContext, admitted []AdmittedERAB) {
	ue.mu.Lock()
	defer ue.mu.Unlock()

	if ue.DefaultEBI != 0 {
		return
	}

	lowest := uint8(0)

	for _, a := range admitted {
		if _, ok := ue.Pdns[a.Ebi]; !ok {
			continue
		}

		if lowest == 0 || a.Ebi < lowest {
			lowest = a.Ebi
		}
	}

	if lowest != 0 {
		ue.DefaultEBI = lowest
	}
}

// handoverBearers snapshots the UE's PDN connections into the E-RABs To Be Setup
// list of a HANDOVER REQUEST (TS 36.413 §9.1.5.4): each bearer's serving GW S1-U
// uplink endpoint and QoS. It returns false when the UE has no usable bearer.
func HandoverBearers(ue *UeContext) ([]s1ap.ERABToBeSetupItemHOReq, bool) {
	ue.mu.Lock()
	defer ue.mu.Unlock()

	bearers := make([]s1ap.ERABToBeSetupItemHOReq, 0, len(ue.Pdns))

	for _, p := range ue.Pdns {
		sgwTLA, err := models.EncodeTransportLayerAddress(p.SgwFTEID.Addr, p.SgwN3IPv6)
		if err != nil {
			logger.MmeLog.Error("failed to encode S-GW transport layer address for handover",
				zap.String("imsi", ue.IMSI()), zap.Uint8("e-rab-id", p.Ebi), zap.Error(err))

			continue
		}

		bearers = append(bearers, s1ap.ERABToBeSetupItemHOReq{
			ERABID:                s1ap.ERABID(p.Ebi),
			TransportLayerAddress: s1ap.TransportLayerAddress(sgwTLA),
			GTPTEID:               s1ap.GTPTEID(p.SgwFTEID.TEID),
			QoS: s1ap.ERABLevelQoSParameters{
				QCI: s1ap.QCI(p.Qci),
				ARP: s1ap.AllocationAndRetentionPriority{
					PriorityLevel:           p.Arp,
					PreemptionCapability:    s1ap.PreemptionShallNotTrigger,
					PreemptionVulnerability: s1ap.PreemptionNotPreemptable,
				},
			},
		})
	}

	return bearers, len(bearers) > 0
}
