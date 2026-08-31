// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"errors"
	"testing"

	"github.com/ellanetworks/core/nas"
)

// TS 33.401 §5
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

	if _, err := nas.CipherFor(nas.CipheringZUC); !errors.Is(err, nas.ErrUnsupportedAlgorithm) {
		t.Errorf("CipherFor(ZUC) = %v, want ErrUnsupportedAlgorithm", err)
	}

	if _, err := nas.IntegrityFor(nas.IntegrityZUC); !errors.Is(err, nas.ErrUnsupportedAlgorithm) {
		t.Errorf("IntegrityFor(ZUC) = %v, want ErrUnsupportedAlgorithm", err)
	}
}
