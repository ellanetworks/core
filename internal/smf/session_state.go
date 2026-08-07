// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-FileCopyrightText: 2021 Open Networking Foundation <info@opennetworking.org>
// Copyright 2019 free5GC.org
// Modified by Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package smf

import (
	"github.com/ellanetworks/core/internal/models"
)

func (p *PDR) ToPDR() models.PDR {
	mp := models.PDR{
		PDRID:              p.PDRID,
		PDI:                p.PDI,
		OuterHeaderRemoval: p.OuterHeaderRemoval,
	}

	if p.FAR != nil {
		mp.FARID = p.FAR.FARID
	}

	if p.QER != nil {
		mp.QERID = p.QER.QERID
	}

	if p.URR != nil {
		mp.URRID = p.URR.URRID
	}

	return mp
}

func (f *FAR) ToFAR() models.FAR {
	return models.FAR{
		FARID:                f.FARID,
		ApplyAction:          f.ApplyAction,
		ForwardingParameters: f.ForwardingParameters,
	}
}

func (q *QER) ToQER() models.QER {
	return models.QER{
		QERID:      q.QERID,
		QFI:        q.QFI,
		GateStatus: q.GateStatus,
		MBR:        q.MBR,
	}
}

func (u *URR) ToURR() models.URR {
	return models.URR{
		URRID: u.URRID,
	}
}

// BuildSessionState states the session's whole user plane: every rule it is to
// have, named once. The UPF converges to exactly this, so a rule the data path
// is to stop carrying is one this omits. policy names the policy whose SDF
// filters the session's PDRs use, which during a move is the target access's.
// Caller holds smContext.mu.
func BuildSessionState(smContext *SMContext, policy *Policy) *models.SessionState {
	state := &models.SessionState{
		SEID:         smContext.UPFSession.SEID,
		IMSI:         smContext.Supi.IMSI(),
		FramedRoutes: smContext.FramedRoutes,
	}

	if policy != nil {
		state.PolicyID = policy.PolicyID
	}

	dataPath := smContext.Tunnel.DataPath

	pdrs := make([]*PDR, 0, 3)

	for _, t := range []*GTPTunnel{dataPath.UpLinkTunnel, dataPath.DownLinkTunnel} {
		if t != nil && t.PDR != nil {
			pdrs = append(pdrs, t.PDR)
		}
	}

	if dataPath.SecondPDR != nil {
		pdrs = append(pdrs, dataPath.SecondPDR)
	}

	// Rule IDs are scoped to the session and several PDRs share a forwarding,
	// QoS or usage rule, so each of those is stated once.
	seenFAR := make(map[uint32]struct{}, len(pdrs))
	seenQER := make(map[uint32]struct{}, len(pdrs))
	seenURR := make(map[uint32]struct{}, len(pdrs))

	for _, pdr := range pdrs {
		state.PDRs = append(state.PDRs, pdr.ToPDR())

		if pdr.FAR != nil {
			if _, dup := seenFAR[pdr.FAR.FARID]; !dup {
				seenFAR[pdr.FAR.FARID] = struct{}{}
				state.FARs = append(state.FARs, pdr.FAR.ToFAR())
			}
		}

		if pdr.QER != nil {
			if _, dup := seenQER[pdr.QER.QERID]; !dup {
				seenQER[pdr.QER.QERID] = struct{}{}
				state.QERs = append(state.QERs, pdr.QER.ToQER())
			}
		}

		if pdr.URR != nil {
			if _, dup := seenURR[pdr.URR.URRID]; !dup {
				seenURR[pdr.URR.URRID] = struct{}{}
				state.URRs = append(state.URRs, pdr.URR.ToURR())
			}
		}
	}

	return state
}

// Packet Detection Rule.
type PDR struct {
	OuterHeaderRemoval *uint8

	FAR *FAR
	URR *URR
	QER *QER

	PDI   models.PDI
	PDRID uint16
}

// Forwarding Action Rule.
type FAR struct {
	ForwardingParameters *models.ForwardingParameters

	FARID uint32

	ApplyAction models.ApplyAction
}

// QoS Enhancement Rule
type QER struct {
	GateStatus *models.GateStatus
	MBR        *models.MBR

	QFI   uint8
	QERID uint32
}

// Usage Report Rule
type URR struct {
	URRID uint32
}

// PFCP rule IDs are scoped to their PFCP session (TS 29.244 §5.2), and the UPF
// datapath keys every rule by SEID, so each session reuses the same fixed set.
// Fixed IDs need no cross-session allocator, which is why no rule ID can leak or
// be double-freed.
const (
	pdrIDUplink   uint16 = 1
	pdrIDDownlink uint16 = 2
	pdrIDSecond   uint16 = 3

	farIDUplink   uint32 = 1
	farIDDownlink uint32 = 2

	qerIDDefault uint32 = 1

	urrIDUplink   uint32 = 1
	urrIDDownlink uint32 = 2
)

// NewPDR builds a PDR with its associated FAR.
func NewPDR(pdrID uint16, farID uint32) *PDR {
	return &PDR{
		PDRID: pdrID,
		FAR:   newFAR(farID),
	}
}

// newFAR builds a FAR defaulting to drop.
func newFAR(farID uint32) *FAR {
	return &FAR{
		FARID:       farID,
		ApplyAction: models.ApplyAction{Drop: true},
	}
}

func NewQER(policy *Policy, qerID uint32) *QER {
	return &QER{
		QERID: qerID,
		QFI:   policy.QosData.QFI,
		GateStatus: &models.GateStatus{
			ULGate: models.GateOpen,
			DLGate: models.GateOpen,
		},
		MBR: &models.MBR{
			ULMBR: policy.Ambr.Uplink.Kbps(),
			DLMBR: policy.Ambr.Downlink.Kbps(),
		},
	}
}

func newURR(urrID uint32) *URR {
	return &URR{
		URRID: urrID,
	}
}
