package models

import (
	"fmt"
	"strconv"
)

type PlmnID struct {
	Mcc string
	Mnc string
}

func (p PlmnID) Equal(other PlmnID) bool {
	return p.Mcc == other.Mcc && p.Mnc == other.Mnc
}

// ServingNetworkName returns the serving network name per TS 24.501 9.12.1.
// Both MCC and MNC are zero-padded to 3 digits as required by the spec.
func (p PlmnID) ServingNetworkName() (string, error) {
	mcc, err := strconv.Atoi(p.Mcc)
	if err != nil {
		return "", fmt.Errorf("invalid MCC %q: %w", p.Mcc, err)
	}

	mnc, err := strconv.Atoi(p.Mnc)
	if err != nil {
		return "", fmt.Errorf("invalid MNC %q: %w", p.Mnc, err)
	}

	return fmt.Sprintf("5G:mnc%03d.mcc%03d.3gppnetwork.org", mnc, mcc), nil
}
