// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"context"
	"fmt"
	"math"
	"slices"
	"strconv"

	"github.com/ellanetworks/core/etsi"
	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/nas/fgs"
	"github.com/ellanetworks/core/s1ap"
)

// OperatorConfig is a point-in-time view of the operator row: a handler needing
// several derived values reads the row once and derives from this snapshot.
type OperatorConfig struct {
	op     *db.Operator
	nodeID int
}

func (m *MME) Operator(ctx context.Context) (OperatorConfig, error) {
	op, err := m.Bearer.GetOperator(ctx)
	if err != nil {
		return OperatorConfig{}, fmt.Errorf("get operator: %w", err)
	}

	return OperatorConfig{op: op, nodeID: m.Bearer.NodeID()}, nil
}

// PLMN returns the operator's serving PLMN (TS 23.003), the network's
// identity advertised in S1 Setup and used for K_ASME derivation and the TAI.
func (o OperatorConfig) PLMN() models.PlmnID {
	return models.PlmnID{Mcc: o.op.Mcc, Mnc: o.op.Mnc}
}

// TACs returns the operator's E-UTRAN-valid Tracking Area Codes. A TAC is
// an OCTET STRING configured as hex. The E-UTRAN TAC is 2 octets and the 5GS TAC 3
// (TS 23.003); a configured value above 16 bits is a 5GS-only TAC, excluded here so
// it cannot match a 16-bit eNB TAC.
func (o OperatorConfig) TACs() ([]uint16, error) {
	tacs, err := o.op.GetSupportedTacs()
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

// ServedTAIs is the network's served tracking areas: the operator PLMN paired with
// each served TAC. Every UE is registered in this area (TS 23.401 §5.3.4).
func (o OperatorConfig) ServedTAIs() ([]models.Tai, error) {
	plmn := o.PLMN()

	tacs, err := o.TACs()
	if err != nil {
		return nil, fmt.Errorf("operator TACs: %w", err)
	}

	out := make([]models.Tai, 0, len(tacs))

	for _, tac := range tacs {
		p := plmn
		out = append(out, models.Tai{PlmnID: &p, Tac: fmt.Sprintf("%06x", tac)})
	}

	return out, nil
}

// ServesTAI reports whether tai — a UE's serving-cell TAI — is served: operator PLMN
// and an operator E-UTRAN TAC. This per-UE gate (EMM cause #12, TS 24.301 §5.5.1.2.5)
// is finer than the node-level S1 Setup gate, which admits an eNB broadcasting any
// served TAI even when it also broadcasts an unserved one.
func (o OperatorConfig) ServesTAI(tai s1ap.TAI) (bool, error) {
	served, err := EncodePLMN(o.PLMN())
	if err != nil {
		return false, err
	}

	if tai.PLMNIdentity != served {
		return false, nil
	}

	tacs, err := o.TACs()
	if err != nil {
		return false, err
	}

	return slices.Contains(tacs, uint16(tai.TAC)), nil
}

// OperatorPLMN returns the operator's serving PLMN (TS 23.003).
func (m *MME) OperatorPLMN(ctx context.Context) (models.PlmnID, error) {
	ctx, span := Tracer.Start(ctx, "mme/get_operator_plmn")
	defer span.End()

	o, err := m.Operator(ctx)
	if err != nil {
		return models.PlmnID{}, err
	}

	return o.PLMN(), nil
}

// OperatorTACs returns the operator's E-UTRAN-valid Tracking Area Codes.
func (m *MME) OperatorTACs(ctx context.Context) ([]uint16, error) {
	ctx, span := Tracer.Start(ctx, "mme/get_operator_tacs")
	defer span.End()

	o, err := m.Operator(ctx)
	if err != nil {
		return nil, err
	}

	return o.TACs()
}

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

func (o OperatorConfig) GUMMEI() (uint16, uint8) {
	mapped := etsi.MapGUTI5GToEPS(fgs.GUTI{
		AMFRegionID: o.op.AMFRegionID(),
		AMFSetID:    uint16(o.op.AmfSetID),
		AMFPointer:  uint8(o.nodeID),
	})

	return mapped.MMEGroupID, mapped.MMECode
}

func (m *MME) MmeIdentity(ctx context.Context) (uint16, uint8, error) {
	o, err := m.Operator(ctx)
	if err != nil {
		return 0, 0, err
	}

	group, code := o.GUMMEI()

	return group, code, nil
}
