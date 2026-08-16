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

// EPSInterworkingAllowed reports whether a PDU session established now may be
// given a mapped EPS bearer context. TS 23.502 §4.11.5.3 step 3: the AMF decides
// EPS interworking support for a PDU session from the 5GMM capability, the
// subscription and network configuration — once, when the session is created.
// It is not re-asked at inter-system mobility; see transferableEPSSessions.
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
