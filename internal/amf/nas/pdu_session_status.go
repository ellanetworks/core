// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"context"
	"fmt"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/nas/fgs"
)

func syncPDUSessionStatus(ctx context.Context, amfInstance *amf.AMF, ue *amf.UeContext, req *fgs.RegistrationRequest) (*[16]bool, error) {
	if req == nil || req.PDUSessionStatus == nil {
		return nil, nil
	}

	reported := req.PDUSessionStatus.PSI
	held := new([16]bool)

	for psi := 1; psi <= 15; psi++ {
		smContext, ok := ue.SmContextFindByPDUSessionID(uint8(psi))
		if !ok {
			continue
		}

		if reported[psi] {
			held[psi] = true
			continue
		}

		if err := amfInstance.Session.ReleaseSmContext(ctx, smContext.Ref); err != nil {
			return nil, fmt.Errorf("release PDU session %d the UE reports inactive: %w", psi, err)
		}
	}

	return held, nil
}
