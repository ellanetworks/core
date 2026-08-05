// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package enb

import (
	"fmt"

	"github.com/ellanetworks/core/internal/tester/gnb"
	"github.com/ellanetworks/core/ngap"
)

// ngENBNodeID builds this simulator's Global RAN Node ID. An ng-eNB announces a
// macro ng-eNB ID (TS 38.413 §9.3.1.5), where internal/tester/gnb announces a
// gNB ID.
func ngENBNodeID(mcc, mnc, enbID string) (ngap.GlobalRANNodeID, error) {
	plmn, err := gnb.PLMNIdentity(mcc, mnc)
	if err != nil {
		return ngap.GlobalRANNodeID{}, err
	}

	b, err := gnb.GetGnbIdInBytes(enbID)
	if err != nil {
		return ngap.GlobalRANNodeID{}, fmt.Errorf("could not decode ng-eNB id: %w", err)
	}

	var v uint64
	for _, o := range b {
		v = v<<8 | uint64(o)
	}

	return ngap.GlobalRANNodeID{
		Kind:         ngap.RANNodeIDMacroNgENB,
		PLMNIdentity: plmn,
		Value:        uint32(v >> uint(8*len(b)-ngENBIDBits)),
		Bits:         ngENBIDBits,
	}, nil
}

// userLocation builds the User Location Information this simulator reports: the
// E-UTRA cell it serves and the TAI that cell is in (TS 38.413 §9.3.1.16).
// internal/tester/gnb reports an NR cell the same way.
func userLocation(mcc, mnc, enbID, tac string) (ngap.UserLocationInformation, error) {
	plmn, err := gnb.PLMNIdentity(mcc, mnc)
	if err != nil {
		return ngap.UserLocationInformation{}, err
	}

	tacValue, err := gnb.TACValue(tac)
	if err != nil {
		return ngap.UserLocationInformation{}, err
	}

	node, err := ngENBNodeID(mcc, mnc, enbID)
	if err != nil {
		return ngap.UserLocationInformation{}, err
	}

	// Derive the cell identity from the same node id NG Setup announced, so both
	// messages report the same cell for the same UE (TS 38.413 §9.3.1.9).
	cellID, err := node.EUTRACellIdentity(0)
	if err != nil {
		return ngap.UserLocationInformation{}, fmt.Errorf("could not build E-UTRA cell identity: %w", err)
	}

	return ngap.UserLocationInformation{
		Kind:         ngap.UserLocationEUTRA,
		PLMNIdentity: plmn,
		CellIdentity: cellID,
		TAI:          ngap.TAI{PLMNIdentity: plmn, TAC: tacValue},
	}, nil
}
