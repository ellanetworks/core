// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package eps

import (
	"testing"

	"github.com/ellanetworks/core/nas"
)

// The EPS algorithms are read out of the UE network capability's own EEA and EIA
// octets, never out of the UMTS or GERAN ones sitting beside them.
func TestSelectNASAlgorithms(t *testing.T) {
	// EEA: EEA0 and 128-EEA2. EIA: 128-EIA2 only.
	uecap := UENetworkCapability{EEA: 0b1010_0000, EIA: 0b0010_0000}

	order := []nas.CipheringAlgorithm{nas.CipheringSNOW3G, nas.CipheringAES, nas.CipheringNull}
	intOrder := []nas.IntegrityAlgorithm{nas.IntegritySNOW3G, nas.IntegrityAES}

	eea, eia, ok := SelectNASAlgorithms(uecap, intOrder, order)
	if !ok {
		t.Fatal("a UE and operator sharing 128-EEA2 and 128-EIA2 must negotiate")
	}

	if eea != nas.CipheringAES || eia != nas.IntegrityAES {
		t.Fatalf("SelectNASAlgorithms = (%s, %s), want (128-EEA2, 128-EIA2)", eea, eia)
	}

	// No integrity algorithm in common: the pair is rejected rather than falling
	// back to EIA0.
	if _, _, ok := SelectNASAlgorithms(uecap, []nas.IntegrityAlgorithm{nas.IntegritySNOW3G}, order); ok {
		t.Fatal("a UE with no operator-preferred integrity algorithm must not negotiate")
	}
}
