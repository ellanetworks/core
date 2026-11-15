// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package gnb

import (
	"encoding/hex"
	"fmt"

	"github.com/ellanetworks/core/nas"
	"github.com/ellanetworks/core/ngap"
)

func GetSliceInBytes(sst int32, sd string) ([]byte, []byte, error) {
	sstBytes := []byte{byte(sst)}

	if sd != "" {
		sdBytes, err := hex.DecodeString(sd)
		if err != nil {
			return sstBytes, nil, fmt.Errorf("could not decode sd to bytes: %v", err)
		}

		return sstBytes, sdBytes, nil
	}

	return sstBytes, nil, nil
}

func GetMccAndMncInOctets(mccStr string, mncStr string) ([]byte, error) {
	octets, err := nas.PLMN{MCC: mccStr, MNC: mncStr}.Octets()
	if err != nil {
		return nil, fmt.Errorf("could not encode mcc/mnc to octets: %w", err)
	}

	return octets[:], nil
}

func GetGnbIdInBytes(gnbId string) ([]byte, error) {
	resu, err := hex.DecodeString(gnbId)
	if err != nil {
		return nil, fmt.Errorf("could not decode gnbId to bytes: %v", err)
	}

	return resu, nil
}

// userLocation builds the User Location Information this simulator reports:
// the NR cell it serves and the TAI that cell is in (TS 38.413 §9.3.1.16).
// internal/tester/s1enb reports its E-UTRAN CGI and TAI the same way.
func userLocation(mcc, mnc, selMcc, selMnc, gnbID, tac string) (ngap.UserLocationInformation, error) {
	plmn, err := PLMNIdentity(mcc, mnc)
	if err != nil {
		return ngap.UserLocationInformation{}, err
	}

	selected, err := PLMNIdentity(selectedOrServing(mcc, mnc, selMcc, selMnc))
	if err != nil {
		return ngap.UserLocationInformation{}, err
	}

	tacValue, err := TACValue(tac)
	if err != nil {
		return ngap.UserLocationInformation{}, err
	}

	node, err := GNBNodeID(mcc, mnc, gnbID)
	if err != nil {
		return ngap.UserLocationInformation{}, err
	}

	cellID, err := node.NRCellIdentity(0)
	if err != nil {
		return ngap.UserLocationInformation{}, fmt.Errorf("could not build NR cell identity: %w", err)
	}

	return ngap.UserLocationInformation{NR: &ngap.UserLocationInformationNR{
		NRCGI: ngap.NRCGI{PLMNIdentity: plmn, NRCellIdentity: ngap.NRCellIdentity(cellID)},
		TAI:   ngap.TAI{PLMNIdentity: selected, TAC: tacValue},
	}}, nil
}
