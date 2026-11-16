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

		if id.GNBID.GNBID == nil {
			return interworking.NGRANIdentity{}, fmt.Errorf("%w: unknown gNB identity alternative", ErrUnusableTargetNGRAN)
		}

		gnbID := *id.GNBID.GNBID

		if gnbID.Bits < 22 || gnbID.Bits > 32 {
			return interworking.NGRANIdentity{}, fmt.Errorf("%w: no gNB identity is %d bits wide", ErrUnusableTargetNGRAN, gnbID.Bits)
		}

		plmn, err := decodePLMN(id.PLMNIdentity)
		if err != nil {
			return interworking.NGRANIdentity{}, fmt.Errorf("mme: target gNB: %w", err)
		}

		out.Kind = interworking.NGRANNodeGNB
		out.PlmnID = plmn
		out.ID = gnbID.Value
		out.Bits = uint8(gnbID.Bits)
	case node.NgENB != nil:
		id := node.NgENB.GlobalNgENBID

		value, ok := id.ENBID.Value()
		if !ok {
			return interworking.NGRANIdentity{}, fmt.Errorf("%w: unknown ng-eNB identity alternative", ErrUnusableTargetNGRAN)
		}

		plmn, err := decodePLMN(id.PLMNIdentity)
		if err != nil {
			return interworking.NGRANIdentity{}, fmt.Errorf("mme: target ng-eNB: %w", err)
		}

		out.Kind = interworking.NGRANNodeNgENB
		out.PlmnID = plmn
		out.ID = value
		out.Bits = uint8(id.ENBID.Bits())
	default:
		return interworking.NGRANIdentity{}, fmt.Errorf("%w: it is neither a gNB nor an ng-eNB", ErrUnusableTargetNGRAN)
	}

	return out, nil
}
