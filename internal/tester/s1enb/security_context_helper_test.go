// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1enb

import (
	"testing"

	"github.com/ellanetworks/core/nas"
)

// mustSecurityContext builds a NAS security context for a test.
func mustSecurityContext(t *testing.T, ia nas.IntegrityAlgorithm, ca nas.CipheringAlgorithm, kInt nas.IntegrityKey, kEnc nas.CipherKey) *nas.SecurityContext {
	t.Helper()

	sc, err := nas.NewSecurityContext(nas.SecurityContextOptions{
		Integrity:          ia,
		Ciphering:          ca,
		IntegrityKey:       kInt,
		CipherKey:          kEnc,
		AllowNullIntegrity: ia == nas.IntegrityNull,
	})
	if err != nil {
		t.Fatalf("NewSecurityContext: %v", err)
	}

	return sc
}

// unprotected drops the security header type Unprotect reports, for a test that
// only wants the plain message.
func unprotected(plain []byte, _ interface{ String() string }, err error) ([]byte, error) {
	return plain, err
}
