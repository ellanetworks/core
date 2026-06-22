// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/s1ap"
)

const (
	relativeMMECapacity uint8 = 255
	mmeName                   = "ella"
)

// encodePLMN encodes an MCC/MNC pair into the 3-octet TBCD PLMN identity
// (TS 23.003).
func encodePLMN(plmn models.PlmnID) (s1ap.PLMNIdentity, error) {
	mcc := strings.Split(plmn.Mcc, "")
	mnc := strings.Split(plmn.Mnc, "")

	if len(mcc) != 3 || (len(mnc) != 2 && len(mnc) != 3) {
		return s1ap.PLMNIdentity{}, fmt.Errorf("mme: invalid PLMN mcc=%q mnc=%q", plmn.Mcc, plmn.Mnc)
	}

	var hexString string
	if len(mnc) == 2 {
		hexString = mcc[1] + mcc[0] + "f" + mcc[2] + mnc[1] + mnc[0]
	} else {
		hexString = mcc[1] + mcc[0] + mnc[0] + mcc[2] + mnc[2] + mnc[1]
	}

	b, err := hex.DecodeString(hexString)
	if err != nil {
		return s1ap.PLMNIdentity{}, fmt.Errorf("mme: encode PLMN: %w", err)
	}

	var out s1ap.PLMNIdentity

	copy(out[:], b)

	return out, nil
}

// servedGUMMEIs builds the Served GUMMEIs advertised in the S1 Setup Response:
// the operator PLMN combined with the configured MME group ID and code.
func servedGUMMEIs(plmn models.PlmnID, mmeGroupID uint16, mmeCode uint8) (s1ap.ServedGUMMEIs, error) {
	p, err := encodePLMN(plmn)
	if err != nil {
		return nil, err
	}

	return s1ap.ServedGUMMEIs{{
		ServedPLMNs:    []s1ap.PLMNIdentity{p},
		ServedGroupIDs: []s1ap.MMEGroupID{{byte(mmeGroupID >> 8), byte(mmeGroupID)}},
		ServedMMECs:    []s1ap.MMECode{s1ap.MMECode(mmeCode)},
	}}, nil
}

// buildS1SetupResponse assembles this MME's S1 Setup Response identity for the
// given operator PLMN and configured MME identity.
func buildS1SetupResponse(plmn models.PlmnID, mmeGroupID uint16, mmeCode uint8) (*s1ap.S1SetupResponse, error) {
	gummeis, err := servedGUMMEIs(plmn, mmeGroupID, mmeCode)
	if err != nil {
		return nil, err
	}

	return &s1ap.S1SetupResponse{
		MMEName:             mmeName,
		ServedGUMMEIs:       gummeis,
		RelativeMMECapacity: relativeMMECapacity,
	}, nil
}
