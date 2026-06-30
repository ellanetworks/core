// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-FileCopyrightText: 2021 Open Networking Foundation <info@opennetworking.org>
// Copyright 2019 free5GC.org
// Modified by Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package smf

import (
	"fmt"

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

// BuildEstablishRequest advances each rule to RuleCreate so later modifications
// dispatch as updates.
func BuildEstablishRequest(
	localSEID uint64,
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
		far.State = RuleCreate
	}

	mqers := make([]models.QER, 0, len(qers))
	seen := make(map[uint32]struct{})

	for _, qer := range qers {
		if _, dup := seen[qer.QERID]; dup {
			continue
		}

		seen[qer.QERID] = struct{}{}
		mqers = append(mqers, qer.ToQER())
		qer.State = RuleCreate
	}

	murrs := make([]models.URR, 0, len(urrs))
	for _, urr := range urrs {
		murrs = append(murrs, urr.ToURR())
	}

	return &models.EstablishRequest{
		LocalSEID: localSEID,
		IMSI:      imsi,
		PolicyID:  policyID,
		PDRs:      mpdrs,
		FARs:      mfars,
		QERs:      mqers,
		URRs:      murrs,
	}
}

// BuildModifyRequest buckets each rule by its RuleState and advances it to
// RuleCreate.
func BuildModifyRequest(
	remoteSEID uint64,
	policyID string,
	pdrs []*PDR,
	fars []*FAR,
	qers []*QER,
) *models.ModifyRequest {
	req := &models.ModifyRequest{
		SEID:     remoteSEID,
		PolicyID: policyID,
	}

	for _, pdr := range pdrs {
		switch pdr.State {
		case RuleInitial:
			req.CreatePDRs = append(req.CreatePDRs, pdr.ToPDR())
		case RuleUpdate:
			req.UpdatePDRs = append(req.UpdatePDRs, pdr.ToPDR())
		case RuleRemove:
			req.RemovePDRIDs = append(req.RemovePDRIDs, pdr.PDRID)
		}

		pdr.State = RuleCreate
	}

	for _, far := range fars {
		switch far.State {
		case RuleInitial:
			req.CreateFARs = append(req.CreateFARs, far.ToFAR())
		case RuleUpdate:
			req.UpdateFARs = append(req.UpdateFARs, far.ToFAR())
		case RuleRemove:
			req.RemoveFARIDs = append(req.RemoveFARIDs, far.FARID)
		}

		far.State = RuleCreate
	}

	for _, qer := range qers {
		switch qer.State {
		case RuleInitial:
			req.CreateQERs = append(req.CreateQERs, qer.ToQER())
		case RuleUpdate:
			req.UpdateQERs = append(req.UpdateQERs, qer.ToQER())
		case RuleRemove:
			req.RemoveQERIDs = append(req.RemoveQERIDs, qer.QERID)
		}

		qer.State = RuleCreate
	}

	return req
}

const (
	RuleInitial RuleState = 0
	RuleCreate  RuleState = 1
	RuleUpdate  RuleState = 2
	RuleRemove  RuleState = 3
)

type RuleState uint8

// Packet Detection Rule.
type PDR struct {
	OuterHeaderRemoval *uint8

	FAR *FAR
	URR *URR
	QER *QER

	PDI   models.PDI
	State RuleState
	PDRID uint16
}

// Forwarding Action Rule.
type FAR struct {
	ForwardingParameters *models.ForwardingParameters

	State RuleState
	FARID uint32

	ApplyAction models.ApplyAction
}

// QoS Enhancement Rule
type QER struct {
	GateStatus *models.GateStatus
	MBR        *models.MBR

	State RuleState
	QFI   uint8
	QERID uint32
}

// Usage Report Rule
type URR struct {
	URRID uint32
}

// NewPDR allocates a PDR with an associated FAR.
func (s *SMF) NewPDR() (*PDR, error) {
	pdrID, err := s.pdrIDs.Allocate()
	if err != nil {
		return nil, fmt.Errorf("could not allocate PDR ID: %v", err)
	}

	far, err := s.NewFAR()
	if err != nil {
		return nil, err
	}

	return &PDR{
		PDRID: uint16(pdrID),
		FAR:   far,
	}, nil
}

// NewFAR allocates a FAR defaulting to drop.
func (s *SMF) NewFAR() (*FAR, error) {
	farID, err := s.farIDs.Allocate()
	if err != nil {
		return nil, fmt.Errorf("could not allocate FAR ID: %v", err)
	}

	return &FAR{
		FARID:       uint32(farID),
		ApplyAction: models.ApplyAction{Drop: true},
	}, nil
}

func (s *SMF) NewQER(policy *Policy) (*QER, error) {
	qerID, err := s.qerIDs.Allocate()
	if err != nil {
		return nil, fmt.Errorf("could not allocate QER ID: %v", err)
	}

	return &QER{
		QERID: uint32(qerID),
		QFI:   policy.QosData.QFI,
		GateStatus: &models.GateStatus{
			ULGate: models.GateOpen,
			DLGate: models.GateOpen,
		},
		MBR: &models.MBR{
			ULMBR: bitRateTokbps(policy.Ambr.Uplink),
			DLMBR: bitRateTokbps(policy.Ambr.Downlink),
		},
	}, nil
}

func (s *SMF) NewURR() (*URR, error) {
	urrID, err := s.urrIDs.Allocate()
	if err != nil {
		return nil, fmt.Errorf("could not allocate URR ID: %v", err)
	}

	return &URR{
		URRID: uint32(urrID),
	}, nil
}

func (s *SMF) RemovePDR(pdr *PDR) {
	s.pdrIDs.FreeID(int64(pdr.PDRID))
}

func (s *SMF) RemoveFAR(far *FAR) {
	s.farIDs.FreeID(int64(far.FARID))
}

func (s *SMF) RemoveQER(qer *QER) {
	s.qerIDs.FreeID(int64(qer.QERID))
}

func (s *SMF) RemoveURR(urr *URR) {
	s.urrIDs.FreeID(int64(urr.URRID))
}
