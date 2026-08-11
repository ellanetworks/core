// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package amf

import "errors"

const (
	firstEPSBearerIdentity = 5
	lastEPSBearerIdentity  = 15
)

var ErrNoEPSBearerIdentity = errors.New("amf: no EPS bearer identity available for this UE")

func (ue *UeContext) AllocateEPSBearerIdentity(pduSessionID uint8) (uint8, error) {
	ue.mu.Lock()
	defer ue.mu.Unlock()

	if existing, ok := ue.epsBearerIdentities[pduSessionID]; ok {
		return existing, nil
	}

	taken := make(map[uint8]struct{}, len(ue.epsBearerIdentities))
	for _, ebi := range ue.epsBearerIdentities {
		taken[ebi] = struct{}{}
	}

	for ebi := uint8(firstEPSBearerIdentity); ebi <= lastEPSBearerIdentity; ebi++ {
		if _, used := taken[ebi]; used {
			continue
		}

		if ue.epsBearerIdentities == nil {
			ue.epsBearerIdentities = make(map[uint8]uint8)
		}

		ue.epsBearerIdentities[pduSessionID] = ebi

		return ebi, nil
	}

	return 0, ErrNoEPSBearerIdentity
}

func (ue *UeContext) ReleaseEPSBearerIdentity(pduSessionID uint8) {
	ue.mu.Lock()
	defer ue.mu.Unlock()

	delete(ue.epsBearerIdentities, pduSessionID)
}

func (ue *UeContext) ReleaseAllEPSBearerIdentities() {
	ue.mu.Lock()
	defer ue.mu.Unlock()

	ue.epsBearerIdentities = nil
}

func (ue *UeContext) EPSBearerIdentity(pduSessionID uint8) (uint8, bool) {
	ue.mu.Lock()
	defer ue.mu.Unlock()

	ebi, ok := ue.epsBearerIdentities[pduSessionID]

	return ebi, ok
}

func (ue *UeContext) EPSBearerIdentities() map[uint8]uint8 {
	ue.mu.Lock()
	defer ue.mu.Unlock()

	out := make(map[uint8]uint8, len(ue.epsBearerIdentities))
	for pduSessionID, ebi := range ue.epsBearerIdentities {
		out[pduSessionID] = ebi
	}

	return out
}
