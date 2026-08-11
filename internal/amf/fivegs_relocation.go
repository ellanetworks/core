// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package amf

import (
	"context"
	"fmt"

	"github.com/ellanetworks/core/etsi"
	"github.com/ellanetworks/core/internal/amf/util"
	"github.com/ellanetworks/core/internal/interworking"
	"github.com/ellanetworks/core/ngap"
)

var _ interworking.FiveGSPeer = (*AMF)(nil)

var ErrRelocationFromEPSUnwired = fmt.Errorf("%w: amf: the 5GS target half of an EPS to 5GS handover is not implemented", interworking.ErrTargetRefused)

func NGRANIdentityToNGAP(target interworking.NGRANIdentity) (ngap.GlobalRANNodeID, error) {
	var kind ngap.RANNodeIDKind

	switch target.Kind {
	case interworking.NGRANNodeGNB:
		if target.Bits < 22 || target.Bits > 32 {
			return ngap.GlobalRANNodeID{}, fmt.Errorf("amf: no gNB identity is %d bits wide", target.Bits)
		}

		kind = ngap.RANNodeIDGNB
	case interworking.NGRANNodeNgENB:
		k, ok := map[uint8]ngap.RANNodeIDKind{
			18: ngap.RANNodeIDShortMacroNgENB,
			20: ngap.RANNodeIDMacroNgENB,
			21: ngap.RANNodeIDLongMacroNgENB,
		}[target.Bits]
		if !ok {
			return ngap.GlobalRANNodeID{}, fmt.Errorf("amf: no ng-eNB identity is %d bits wide", target.Bits)
		}

		kind = k
	default:
		return ngap.GlobalRANNodeID{}, fmt.Errorf("amf: unknown NG-RAN node kind %d", target.Kind)
	}

	plmn, err := util.PLMNToNGAP(target.PlmnID)
	if err != nil {
		return ngap.GlobalRANNodeID{}, err
	}

	return ngap.GlobalRANNodeID{
		Kind:         kind,
		PLMNIdentity: plmn,
		Value:        target.ID,
		Bits:         int(target.Bits),
	}, nil
}

func (a *AMF) ForwardRelocation(ctx context.Context, req interworking.FiveGSRelocationRequest) (interworking.FiveGSRelocationResponse, error) {
	return interworking.FiveGSRelocationResponse{}, ErrRelocationFromEPSUnwired
}

func (a *AMF) RelocationCancel(ctx context.Context, supi etsi.SUPI, id interworking.RelocationID) error {
	return ErrRelocationFromEPSUnwired
}
