// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package amf

import "errors"

const (
	firstEPSBearerIdentity = 5
	lastEPSBearerIdentity  = 15
)

var ErrNoEPSBearerIdentity = errors.New("amf: no EPS bearer identity available for this UE")

func (ue *UeContext) SetAllow4G(v bool) {
	ue.mu.Lock()
	defer ue.mu.Unlock()

	ue.allow4G = v
}

func (ue *UeContext) TransfersToEPS() bool {
	ue.mu.Lock()
	defer ue.mu.Unlock()

	return ue.allow4G && ue.gmmCapability != nil && ue.gmmCapability.S1Mode
}

func (ue *UeContext) NextEPSBearerIdentity(pduSessionID uint8) (uint8, error) {
	ue.mu.Lock()
	defer ue.mu.Unlock()

	if sc, ok := ue.SmContextList[pduSessionID]; ok && sc.EBI != 0 {
		return sc.EBI, nil
	}

	taken := make(map[uint8]struct{}, len(ue.SmContextList))
	for _, sc := range ue.SmContextList {
		taken[sc.EBI] = struct{}{}
	}

	for ebi := uint8(firstEPSBearerIdentity); ebi <= lastEPSBearerIdentity; ebi++ {
		if _, used := taken[ebi]; used {
			continue
		}

		return ebi, nil
	}

	return 0, ErrNoEPSBearerIdentity
}

func (ue *UeContext) SetEPSBearerIdentity(pduSessionID, ebi uint8) {
	ue.mu.Lock()
	defer ue.mu.Unlock()

	if sc, ok := ue.SmContextList[pduSessionID]; ok {
		sc.EBI = ebi
	}
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
