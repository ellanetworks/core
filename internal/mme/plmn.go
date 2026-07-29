// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"context"
	"fmt"

	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/nas"
	"github.com/ellanetworks/core/s1ap"
)

// ServesTAI reports whether tai — a UE's serving-cell TAI — is served, per
// OperatorConfig.ServesTAI.
func (m *MME) ServesTAI(ctx context.Context, tai s1ap.TAI) (bool, error) {
	o, err := m.Operator(ctx)
	if err != nil {
		return false, err
	}

	return o.ServesTAI(tai)
}

// EncodePLMN encodes an MCC/MNC pair into the 3-octet TBCD PLMN identity
// (TS 23.003).
func EncodePLMN(plmn models.PlmnID) (s1ap.PLMNIdentity, error) {
	b, err := nas.PLMN{MCC: plmn.Mcc, MNC: plmn.Mnc}.Octets()
	if err != nil {
		return s1ap.PLMNIdentity{}, fmt.Errorf("mme: encode PLMN mcc=%q mnc=%q: %w", plmn.Mcc, plmn.Mnc, err)
	}

	return s1ap.PLMNIdentity(b), nil
}

// decodePLMN decodes a 3-octet TBCD PLMN identity into its MCC/MNC pair
// (TS 23.003).
func decodePLMN(p s1ap.PLMNIdentity) models.PlmnID {
	plmn, err := nas.ParsePLMN([3]byte(p))
	if err != nil {
		return models.PlmnID{}
	}

	return models.PlmnID{Mcc: plmn.MCC, Mnc: plmn.MNC}
}
