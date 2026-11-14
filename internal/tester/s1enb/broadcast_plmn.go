// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1enb

import (
	"fmt"
	"slices"

	"github.com/ellanetworks/core/nas"
	"github.com/ellanetworks/core/s1ap"
)

func broadcastPLMNs(primary s1ap.PLMNIdentity, additional []nas.PLMN) (s1ap.BPLMNs, error) {
	out := s1ap.BPLMNs{primary}

	for _, p := range additional {
		plmn, err := plmnOctets(p.MCC, p.MNC)
		if err != nil {
			return nil, err
		}

		if slices.Contains(out, plmn) {
			return nil, fmt.Errorf("s1enb: PLMN %s/%s is broadcast twice", p.MCC, p.MNC)
		}

		out = append(out, plmn)
	}

	return out, nil
}

func (e *ENB) BroadcastPLMNs() s1ap.BPLMNs {
	return slices.Clone(e.bplmns)
}

func (ue *UE) SelectPLMN(mcc, mnc string) error {
	plmn, err := plmnOctets(mcc, mnc)
	if err != nil {
		return err
	}

	if !slices.Contains(ue.enb.bplmns, plmn) {
		return fmt.Errorf("s1enb: PLMN %s/%s is not broadcast by eNB %#x", mcc, mnc, ue.enb.enbID)
	}

	ue.selectedPLMN = &plmn
	ue.plmn = append([]byte(nil), plmn[:]...)

	return nil
}

func (e *ENB) AllocateENBUEIDFor(ue *UE) int64 {
	id := e.AllocateENBUEID()

	if ue != nil && ue.selectedPLMN != nil {
		e.mu.Lock()
		e.selected[id] = *ue.selectedPLMN
		e.mu.Unlock()
	}

	return id
}

func (e *ENB) taiFor(enbUEID int64) s1ap.TAI {
	e.mu.Lock()
	plmn, ok := e.selected[enbUEID]
	e.mu.Unlock()

	if !ok {
		plmn = e.plmn
	}

	return s1ap.TAI{PLMNIdentity: plmn, TAC: s1ap.TAC(e.tac)}
}

func (e *ENB) dropSelectedPLMN(enbUEID int64) {
	e.mu.Lock()
	defer e.mu.Unlock()

	delete(e.selected, enbUEID)
}
