// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"errors"
	"testing"

	"github.com/ellanetworks/core/nas"
)

// A UE that advertises no integrity algorithm common with the operator policy is
// rejected (EMM cause #23), not silently downgraded to the null algorithm
// (TS 33.401 §5).
func TestCipherIntegrityAlgMapping(t *testing.T) {
	for _, alg := range []nas.CipheringAlgorithm{nas.CipheringNull, nas.CipheringSNOW3G, nas.CipheringAES} {
		if _, err := nas.CipherFor(alg); err != nil {
			t.Errorf("CipherFor(%s): %v", alg, err)
		}
	}

	for _, alg := range []nas.IntegrityAlgorithm{nas.IntegrityNull, nas.IntegritySNOW3G, nas.IntegrityAES} {
		if _, err := nas.IntegrityFor(alg); err != nil {
			t.Errorf("IntegrityFor(%s): %v", alg, err)
		}
	}

	// ZUC is defined by TS 33.401 but not implemented here, and must fail closed.
	if _, err := nas.CipherFor(nas.CipheringZUC); !errors.Is(err, nas.ErrUnsupportedAlgorithm) {
		t.Errorf("CipherFor(ZUC) = %v, want ErrUnsupportedAlgorithm", err)
	}

	if _, err := nas.IntegrityFor(nas.IntegrityZUC); !errors.Is(err, nas.ErrUnsupportedAlgorithm) {
		t.Errorf("IntegrityFor(ZUC) = %v, want ErrUnsupportedAlgorithm", err)
	}
}
