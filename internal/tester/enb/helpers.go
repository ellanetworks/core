// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package enb

import (
	"fmt"

	"github.com/ellanetworks/core/internal/tester/gnb"
	"github.com/ellanetworks/core/ngap"
	"github.com/free5gc/aper"
	"github.com/free5gc/ngap/ngapType"
)

func GetEUTRACellIdentity(enbID string) (ngapType.EUTRACellIdentity, error) {
	enbIDBytes, err := gnb.GetGnbIdInBytes(enbID)
	if err != nil {
		return ngapType.EUTRACellIdentity{}, fmt.Errorf("could not get EUTRACellIdentity: %v", err)
	}

	// Derive the cell identity from the same node id BuildInitialUEMessage uses,
	// so both messages report the same cell for the same UE and the cell belongs
	// to the node NG Setup announced (TS 38.413 §9.3.1.8).
	var v uint64
	for _, o := range enbIDBytes {
		v = v<<8 | uint64(o)
	}

	node := ngap.GlobalRANNodeID{
		Kind:  ngap.RANNodeIDMacroNgENB,
		Value: uint32(v >> uint(8*len(enbIDBytes)-ngENBIDBits)), Bits: ngENBIDBits,
	}

	eci, err := node.EUTRACellIdentity(0)
	if err != nil {
		return ngapType.EUTRACellIdentity{}, fmt.Errorf("could not get EUTRACellIdentity: %w", err)
	}

	return ngapType.EUTRACellIdentity{
		Value: aper.BitString{
			Bytes:     uintToLeftAlignedBytes(eci, ngap.EUTRACellIdentityBits),
			BitLength: ngap.EUTRACellIdentityBits,
		},
	}, nil
}

// uintToLeftAlignedBytes packs the low nbits of v most-significant-bit first,
// the layout an aper.BitString of that length expects.
func uintToLeftAlignedBytes(v uint64, nbits int) []byte {
	b := make([]byte, (nbits+7)/8)

	for i := range nbits {
		if v&(1<<uint(nbits-1-i)) != 0 {
			b[i/8] |= 1 << uint(7-i%8)
		}
	}

	return b
}
