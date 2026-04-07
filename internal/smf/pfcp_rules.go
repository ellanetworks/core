// Copyright 2024 Ella Networks
// SPDX-FileCopyrightText: 2021 Open Networking Foundation <info@opennetworking.org>
// Copyright 2019 free5GC.org
// SPDX-License-Identifier: Apache-2.0

package smf

import (
	"github.com/ellanetworks/core/internal/models"
)

// ToPDR flattens a PDR into the message struct the UPF expects.
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

// ToFAR flattens a FAR into the message struct the UPF expects.
func (f *FAR) ToFAR() models.FAR {
	return models.FAR{
		FARID:                f.FARID,
		ApplyAction:          f.ApplyAction,
		ForwardingParameters: f.ForwardingParameters,
	}
}

// ToQER flattens a QER into the message struct the UPF expects.
func (q *QER) ToQER() models.QER {
	return models.QER{
		QERID:      q.QERID,
		QFI:        q.QFI,
		GateStatus: q.GateStatus,
		MBR:        q.MBR,
	}
}

// ToURR flattens a URR into the message struct the UPF expects.
func (u *URR) ToURR() models.URR {
	return models.URR{
		URRID: u.URRID,
	}
}

// BuildEstablishRequest converts live SMF rule slices into the flat message
// struct the UPF expects for session establishment, then marks all rules as
// created so subsequent modifications use the correct state.
func BuildEstablishRequest(
	localSEID uint64,
	imsi string,
	pdrs []*PDR,
	fars []*FAR,
	qers []*QER,
	urrs []*URR,
	filterIndexByPDRID map[uint16]uint32,
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
		LocalSEID:          localSEID,
		IMSI:               imsi,
		PDRs:               mpdrs,
		FARs:               mfars,
		QERs:               mqers,
		URRs:               murrs,
		FilterIndexByPDRID: filterIndexByPDRID,
	}
}

// BuildModifyRequest converts live SMF rule slices into the flat message
// struct the UPF expects for session modification. Rules are dispatched
// into Create/Update/Remove buckets based on their RuleState, then all
// states are advanced to RuleCreate.
func BuildModifyRequest(
	remoteSEID uint64,
	pdrs []*PDR,
	fars []*FAR,
	qers []*QER,
	filterIndexByPDRID map[uint16]uint32,
) *models.ModifyRequest {
	req := &models.ModifyRequest{
		SEID:               remoteSEID,
		FilterIndexByPDRID: filterIndexByPDRID,
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
		if qer.State == RuleInitial {
			req.CreateQERs = append(req.CreateQERs, qer.ToQER())
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

const (
	OuterHeaderCreationGtpUUdpIpv4 uint16 = 256
	OuterHeaderRemovalGtpUUdpIpv4  uint8  = 0
)

const (
	GateOpen uint8 = iota
	GateClose
)

// Packet Detection Rule. Table 7.5.2.2-1
type PDR struct {
	OuterHeaderRemoval *uint8

	FAR *FAR
	URR *URR
	QER *QER

	PDI            models.PDI
	State          RuleState
	PDRID          uint16
	FilterMapIndex uint32 // BPF sdf_filters map index; 0 = no filter
}

// Forwarding Action Rule. 7.5.2.3-1
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
