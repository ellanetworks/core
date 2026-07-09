// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"context"
	"fmt"
	"slices"

	"github.com/ellanetworks/core/internal/models"
	nascommon "github.com/ellanetworks/core/nas/common"
	"github.com/ellanetworks/core/s1ap"
)

// ServesTAI reports whether tai — a UE's serving-cell TAI — is served: operator PLMN
// and an operator E-UTRAN TAC. This per-UE gate (EMM cause #12, TS 24.301 §5.5.1.2.5)
// is finer than the node-level S1 Setup gate, which admits an eNB broadcasting any
// served TAI even when it also broadcasts an unserved one.
func (m *MME) ServesTAI(ctx context.Context, tai s1ap.TAI) (bool, error) {
	plmn, err := m.OperatorPLMN(ctx)
	if err != nil {
		return false, err
	}

	served, err := EncodePLMN(plmn)
	if err != nil {
		return false, err
	}

	if tai.PLMNIdentity != served {
		return false, nil
	}

	tacs, err := m.OperatorTACs(ctx)
	if err != nil {
		return false, err
	}

	return slices.Contains(tacs, uint16(tai.TAC)), nil
}

// EncodePLMN encodes an MCC/MNC pair into the 3-octet TBCD PLMN identity
// (TS 23.003).
func EncodePLMN(plmn models.PlmnID) (s1ap.PLMNIdentity, error) {
	b, err := nascommon.EncodePLMN(plmn.Mcc, plmn.Mnc)
	if err != nil {
		return s1ap.PLMNIdentity{}, fmt.Errorf("mme: encode PLMN mcc=%q mnc=%q: %w", plmn.Mcc, plmn.Mnc, err)
	}

	return s1ap.PLMNIdentity(b), nil
}

// decodePLMN decodes a 3-octet TBCD PLMN identity into its MCC/MNC pair
// (TS 23.003).
func decodePLMN(p s1ap.PLMNIdentity) models.PlmnID {
	mcc, mnc := nascommon.DecodePLMN([3]byte(p))

	return models.PlmnID{Mcc: mcc, Mnc: mnc}
}
