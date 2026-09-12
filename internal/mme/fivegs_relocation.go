// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"errors"
	"fmt"

	"github.com/ellanetworks/core/internal/interworking"
	"github.com/ellanetworks/core/s1ap"
)

var _ interworking.EPSPeer = (*MME)(nil)

var (
	ErrUnusableTargetNGRAN  = errors.New("mme: the target NG-RAN node cannot be named")
	ErrNoRelocationToFiveGS = errors.New("mme: no handover to 5GS is in progress for this subscriber")
)

func NGRANIdentityFromS1AP(target s1ap.TargetNgRanNodeID) (interworking.NGRANIdentity, error) {
	selectedPLMN, err := decodePLMN(target.SelectedTAI.PLMNIdentity)
	if err != nil {
		return interworking.NGRANIdentity{}, fmt.Errorf("mme: selected 5GS TAI: %w", err)
	}

	out := interworking.NGRANIdentity{
		SelectedTAI: interworking.FiveGSTAI{
			PlmnID: selectedPLMN,
			TAC:    uint32(target.SelectedTAI.TAC),
		},
	}

	switch node := target.GlobalRANNodeID; {
	case node.GNB != nil:
		id := node.GNB.GlobalGNBID

		if id.GNBID.Bits < 22 || id.GNBID.Bits > 32 {
			return interworking.NGRANIdentity{}, fmt.Errorf("%w: no gNB identity is %d bits wide", ErrUnusableTargetNGRAN, id.GNBID.Bits)
		}

		plmn, err := decodePLMN(id.PLMNIdentity)
		if err != nil {
			return interworking.NGRANIdentity{}, fmt.Errorf("mme: target gNB: %w", err)
		}

		out.Kind = interworking.NGRANNodeGNB
		out.PlmnID = plmn
		out.ID = id.GNBID.Value
		out.Bits = uint8(id.GNBID.Bits)
	case node.NgENB != nil:
		id := node.NgENB.GlobalNgENBID

		bits, ok := map[s1ap.ENBIDKind]uint8{
			s1ap.ENBIDShortMacro: 18,
			s1ap.ENBIDMacro:      20,
			s1ap.ENBIDLongMacro:  21,
			s1ap.ENBIDHome:       28,
		}[id.ENBID.Kind]
		if !ok {
			return interworking.NGRANIdentity{}, fmt.Errorf("%w: unknown ng-eNB identity kind %d", ErrUnusableTargetNGRAN, id.ENBID.Kind)
		}

		plmn, err := decodePLMN(id.PLMNIdentity)
		if err != nil {
			return interworking.NGRANIdentity{}, fmt.Errorf("mme: target ng-eNB: %w", err)
		}

		out.Kind = interworking.NGRANNodeNgENB
		out.PlmnID = plmn
		out.ID = id.ENBID.Value
		out.Bits = bits
	default:
		return interworking.NGRANIdentity{}, fmt.Errorf("%w: it is neither a gNB nor an ng-eNB", ErrUnusableTargetNGRAN)
	}

	return out, nil
}
