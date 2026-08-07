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

func BuildEstablishRequest(
	seid uint64,
	imsi string,
	policyID string,
	pdrs []*PDR,
	fars []*FAR,
	qers []*QER,
	urrs []*URR,
) *models.EstablishRequest {
	mpdrs := make([]models.PDR, 0, len(pdrs))
	for _, pdr := range pdrs {
		mpdrs = append(mpdrs, pdr.ToPDR())
	}

	mfars := make([]models.FAR, 0, len(fars))
	for _, far := range fars {
		mfars = append(mfars, far.ToFAR())
	}

	mqers := make([]models.QER, 0, len(qers))
	seen := make(map[uint32]struct{})

	for _, qer := range qers {
		if _, dup := seen[qer.QERID]; dup {
			continue
		}

		seen[qer.QERID] = struct{}{}
		mqers = append(mqers, qer.ToQER())
	}

	murrs := make([]models.URR, 0, len(urrs))
	for _, urr := range urrs {
		murrs = append(murrs, urr.ToURR())
	}

	return &models.EstablishRequest{
		SEID:     seid,
		IMSI:     imsi,
		PolicyID: policyID,
		PDRs:     mpdrs,
		FARs:     mfars,
		QERs:     mqers,
		URRs:     murrs,
	}
}

// Every rule passed is an update: rules are created once at establish and
// removed only by deleting the session.
func BuildModifyRequest(
	seid uint64,
	policyID string,
	pdrs []*PDR,
	fars []*FAR,
	qers []*QER,
) *models.ModifyRequest {
	req := &models.ModifyRequest{
		SEID:     seid,
		PolicyID: policyID,
	}

	for _, pdr := range pdrs {
		req.UpdatePDRs = append(req.UpdatePDRs, pdr.ToPDR())
	}

	for _, far := range fars {
		req.UpdateFARs = append(req.UpdateFARs, far.ToFAR())
	}

	for _, qer := range qers {
		req.UpdateQERs = append(req.UpdateQERs, qer.ToQER())
	}

	return req
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
