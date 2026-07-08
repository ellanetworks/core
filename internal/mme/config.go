// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"context"
	"fmt"
	"math"
	"strconv"

	"github.com/ellanetworks/core/internal/models"
)

// OperatorPLMN returns the operator's serving PLMN (TS 23.003), the network's
// identity advertised in S1 Setup and used for K_ASME derivation and the TAI.
func (m *MME) OperatorPLMN(ctx context.Context) (models.PlmnID, error) {
	ctx, span := Tracer.Start(ctx, "mme/get_operator_plmn")
	defer span.End()

	op, err := m.Bearer.GetOperator(ctx)
	if err != nil {
		return models.PlmnID{}, fmt.Errorf("get operator: %w", err)
	}

	return models.PlmnID{Mcc: op.Mcc, Mnc: op.Mnc}, nil
}

// defaultMMEGroupID is the fixed MME Group ID of the GUMMEI (TS 23.003).
// Ella Core is a single MME pool, so the group is constant; per-node identity
// comes from the MME Code.
const defaultMMEGroupID uint16 = 1

// MmeIdentity returns the GUMMEI components (TS 23.003): a fixed MME Group
// ID, and an MME Code derived from the cluster node ID so each HA node advertises
// a distinct GUMMEI and a UE's GUTI routes back to its owning node.
// The MME Code is 8 bits, so distinct codes hold for clusters up to 256 nodes;
// beyond that the low 8 bits could collide.
func (m *MME) MmeIdentity() (uint16, uint8) {
	return defaultMMEGroupID, uint8(m.Bearer.NodeID() & 0xFF)
}

// OperatorTACs returns the operator's E-UTRAN-valid Tracking Area Codes. A TAC is
// an OCTET STRING configured as hex. The E-UTRAN TAC is 2 octets and the 5GS TAC 3
// (TS 23.003); a configured value above 16 bits is a 5GS-only TAC, excluded here so
// it cannot match a 16-bit eNB TAC.
func (m *MME) OperatorTACs(ctx context.Context) ([]uint16, error) {
	ctx, span := Tracer.Start(ctx, "mme/get_operator_tacs")
	defer span.End()

	op, err := m.Bearer.GetOperator(ctx)
	if err != nil {
		return nil, fmt.Errorf("get operator: %w", err)
	}

	tacs, err := op.GetSupportedTacs()
	if err != nil {
		return nil, fmt.Errorf("get supported TACs: %w", err)
	}

	out := make([]uint16, 0, len(tacs))

	for _, t := range tacs {
		n, err := strconv.ParseUint(t, 16, 32)
		if err != nil {
			return nil, fmt.Errorf("invalid TAC %q: %w", t, err)
		}

		if n > math.MaxUint16 {
			continue
		}

		out = append(out, uint16(n))
	}

	return out, nil
}

// OperatorTAC returns the operator's first supported Tracking Area Code.
func (m *MME) OperatorTAC(ctx context.Context) (uint16, error) {
	tacs, err := m.OperatorTACs(ctx)
	if err != nil {
		return 0, err
	}

	if len(tacs) == 0 {
		return 0, fmt.Errorf("operator has no supported TAC")
	}

	return tacs[0], nil
}
