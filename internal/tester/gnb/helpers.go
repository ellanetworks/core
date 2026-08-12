// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package gnb

import (
	"encoding/hex"
	"fmt"
	"net/netip"

	"github.com/ellanetworks/core/nas"
	"github.com/ellanetworks/core/ngap"
)

func GetTacInBytes(tacStr string) ([]byte, error) {
	resu, err := hex.DecodeString(tacStr)
	if err != nil {
		return nil, fmt.Errorf("could not decode tac to bytes: %v", err)
	}

	return resu, nil
}

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

// transportLayerAddress renders an IP as the bit string a GTP tunnel endpoint
// carries: 32 bits for IPv4, 128 for IPv6 (TS 38.413 §9.3.2.4).
func transportLayerAddress(ip netip.Addr) (ngap.TransportLayerAddress, error) {
	switch {
	case ip.Is4():
		v4 := ip.As4()

		return ngap.TransportLayerAddress(v4[:]), nil
	case ip.Is6():
		v6 := ip.As16()

		return ngap.TransportLayerAddress(v6[:]), nil
	default:
		return nil, fmt.Errorf("transport layer address %q is neither IPv4 nor IPv6", ip)
	}
}

// userLocation builds the User Location Information this simulator reports:
// the NR cell it serves and the TAI that cell is in (TS 38.413 §9.3.1.16).
// internal/tester/s1enb reports its E-UTRAN CGI and TAI the same way.
func userLocation(mcc, mnc, gnbID, tac string) (ngap.UserLocationInformation, error) {
	plmn, err := PLMNIdentity(mcc, mnc)
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

	return ngap.UserLocationInformation{
		Kind:         ngap.UserLocationNR,
		PLMNIdentity: plmn,
		CellIdentity: cellID,
		TAI:          ngap.TAI{PLMNIdentity: plmn, TAC: tacValue},
	}, nil
}
