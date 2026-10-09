// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package amf

import (
	"errors"
	"slices"

	"github.com/ellanetworks/core/etsi"
)

const (
	firstEPSBearerIdentity = 5
	lastEPSBearerIdentity  = 15
)

var (
	ErrNoEPSBearerIdentity    = errors.New("amf: no EPS bearer identity available for this UE")
	ErrSessionNotInterworking = errors.New("amf: the PDU session has no EPS bearer identity for interworking")
)

func (ue *UeContext) SetAllow4G(v bool) {
	ue.mu.Lock()
	defer ue.mu.Unlock()

	ue.allow4G = v
}

func (ue *UeContext) EPSInterworkingAllowed() bool {
	if !ue.SupportsS1Mode() {
		return false
	}

	ue.mu.Lock()
	defer ue.mu.Unlock()

	return ue.allow4G
}

func (ue *UeContext) NextEPSBearerIdentity(pduSessionID uint8) (uint8, error) {
	ue.mu.Lock()
	defer ue.mu.Unlock()

	if sc, ok := ue.SmContextList[pduSessionID]; ok && sc.EBI != 0 {
		return sc.EBI, nil
	}

	if ebi, ok := ue.reservedEBIs[pduSessionID]; ok {
		return ebi, nil
	}

	ebi, err := ue.freeEPSBearerIdentityLocked()
	if err != nil {
		return 0, err
	}

	if ue.reservedEBIs == nil {
		ue.reservedEBIs = make(map[uint8]uint8)
	}

	ue.reservedEBIs[pduSessionID] = ebi

	return ebi, nil
}

func (ue *UeContext) ReleaseEPSBearerReservation(pduSessionID uint8) {
	ue.mu.Lock()
	defer ue.mu.Unlock()

	delete(ue.reservedEBIs, pduSessionID)
}

func (ue *UeContext) freeEPSBearerIdentityLocked() (uint8, error) {
	taken := make(map[uint8]struct{}, len(ue.SmContextList)+len(ue.reservedEBIs))
	for _, ebi := range ue.reservedEBIs {
		taken[ebi] = struct{}{}
	}

	for _, sc := range ue.SmContextList {
		taken[sc.EBI] = struct{}{}

		for _, ebi := range sc.FlowEBIs {
			taken[ebi] = struct{}{}
		}
	}

	for ebi := uint8(firstEPSBearerIdentity); ebi <= lastEPSBearerIdentity; ebi++ {
		if _, used := taken[ebi]; used {
			continue
		}

		return ebi, nil
	}

	return 0, ErrNoEPSBearerIdentity
}

func (ue *UeContext) AssignFlowEPSBearerIdentity(pduSessionID uint8, ref string) (uint8, error) {
	ue.mu.Lock()
	defer ue.mu.Unlock()

	sc, ok := ue.SmContextList[pduSessionID]
	if !ok || sc.Ref != ref || sc.EBI == 0 {
		return 0, ErrSessionNotInterworking
	}

	ebi, err := ue.freeEPSBearerIdentityLocked()
	if err != nil {
		return 0, err
	}

	sc.FlowEBIs = append(sc.FlowEBIs, ebi)

	return ebi, nil
}

func (ue *UeContext) ReleaseFlowEPSBearerIdentities(pduSessionID uint8, ref string, ebis []uint8) {
	ue.mu.Lock()
	defer ue.mu.Unlock()

	if sc, ok := ue.SmContextList[pduSessionID]; ok && sc.Ref == ref {
		sc.FlowEBIs = slices.DeleteFunc(sc.FlowEBIs, func(ebi uint8) bool { return slices.Contains(ebis, ebi) })
	}
}

func (ue *UeContext) SetEPSBearerIdentity(pduSessionID, ebi uint8) {
	ue.mu.Lock()
	defer ue.mu.Unlock()

	delete(ue.reservedEBIs, pduSessionID)

	if sc, ok := ue.SmContextList[pduSessionID]; ok {
		sc.EBI = ebi
	}
}

func (ue *UeContext) SetFlowEPSBearerIdentities(pduSessionID uint8, ebis []uint8) {
	ue.mu.Lock()
	defer ue.mu.Unlock()

	if sc, ok := ue.SmContextList[pduSessionID]; ok {
		sc.FlowEBIs = slices.Clone(ebis)
	}
}

func (ue *UeContext) FlowEPSBearerIdentities() map[uint8][]uint8 {
	ue.mu.Lock()
	defer ue.mu.Unlock()

	out := make(map[uint8][]uint8, len(ue.SmContextList))

	for pduSessionID, sc := range ue.SmContextList {
		if len(sc.FlowEBIs) > 0 {
			out[pduSessionID] = slices.Clone(sc.FlowEBIs)
		}
	}

	return out
}

func (ue *UeContext) EPSBearerIdentity(pduSessionID uint8) (uint8, bool) {
	ue.mu.Lock()
	defer ue.mu.Unlock()

	sc, ok := ue.SmContextList[pduSessionID]
	if !ok || sc.EBI == 0 {
		return 0, false
	}

	return sc.EBI, true
}

func (ue *UeContext) EPSBearerIdentities() map[uint8]uint8 {
	ue.mu.Lock()
	defer ue.mu.Unlock()

	out := make(map[uint8]uint8, len(ue.SmContextList))

	for pduSessionID, sc := range ue.SmContextList {
		if sc.EBI != 0 {
			out[pduSessionID] = sc.EBI
		}
	}

	return out
}

func (amf *AMF) AssignEPSBearerIdentity(supi etsi.SUPI, pduSessionID uint8, ref string) (uint8, error) {
	ue, ok := amf.LookupUeBySupi(supi)
	if !ok {
		return 0, ErrSessionNotInterworking
	}

	return ue.AssignFlowEPSBearerIdentity(pduSessionID, ref)
}

func (amf *AMF) ReleaseEPSBearerIdentities(supi etsi.SUPI, pduSessionID uint8, ref string, ebis []uint8) {
	if ue, ok := amf.LookupUeBySupi(supi); ok {
		ue.ReleaseFlowEPSBearerIdentities(pduSessionID, ref, ebis)
	}
}
