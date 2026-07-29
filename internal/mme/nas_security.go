// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"context"
	"crypto/sha256"
	"fmt"

	"github.com/ellanetworks/core/nas"
	"github.com/ellanetworks/core/nas/eps"
)

// HashMME returns the 8-octet HashMME for the SECURITY MODE COMMAND — the 64 most
// significant bits of the SHA-256 of the triggering plain Attach/TAU — or nil when
// there is nothing to hash (TS 24.301 §5.4.3.2, TS 33.401 §6.4.2.1).
func HashMME(input []byte) []byte {
	if len(input) == 0 {
		return nil
	}

	sum := sha256.Sum256(input)

	return sum[:8]
}

// SelectAlgorithms picks the EPS NAS algorithms allowed by both the UE network
// capability and the operator policy (TS 33.401), in the operator's order of
// preference (EPS algorithm codes as returned by SecurityAlgorithms). It reports
// false when the UE and operator share no algorithm, so the caller rejects the
// attach without falling back to the null algorithm.
func SelectAlgorithms(uecap eps.UENetworkCapability, integrity []nas.IntegrityAlgorithm, ciphering []nas.CipheringAlgorithm) (eea nas.CipheringAlgorithm, eia nas.IntegrityAlgorithm, ok bool) {
	eea, eok := selectEPSAlgorithm(ciphering, uecap.SupportsEEA)
	eia, iok := selectEPSAlgorithm(integrity, uecap.SupportsEIA)

	return eea, eia, eok && iok
}

// epsAlgorithmValue maps an operator algorithm identity to its EPS algorithm
// number (TS 33.401): NULL=0, SNOW3G=1, AES=2. The two generations number them
// alike, so one mapping serves both.
func epsAlgorithmValue[T ~uint8](name string) (T, bool) {
	switch name {
	case "NULL":
		return 0, true
	case "SNOW3G":
		return 1, true
	case "AES":
		return 2, true
	default:
		return 0, false
	}
}

// SecurityAlgorithms returns the operator's configured NAS integrity and ciphering
// algorithm orders as EPS algorithm codes (TS 33.401), mapping the operator's
// RAT-neutral names (NULL/SNOW3G/AES) and dropping any it does not recognise.
func (m *MME) SecurityAlgorithms(ctx context.Context) ([]nas.IntegrityAlgorithm, []nas.CipheringAlgorithm, error) {
	ctx, span := Tracer.Start(ctx, "mme/get_security_algorithms")
	defer span.End()

	op, err := m.Bearer.GetOperator(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get operator: %w", err)
	}

	cipherNames, err := op.GetCiphering()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read ciphering policy: %w", err)
	}

	integrityNames, err := op.GetIntegrity()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read integrity policy: %w", err)
	}

	encOrder := make([]nas.CipheringAlgorithm, 0, len(cipherNames))
	for _, name := range cipherNames {
		alg, ok := epsAlgorithmValue[nas.CipheringAlgorithm](name)
		if !ok {
			return nil, nil, fmt.Errorf("unknown ciphering algorithm: %s", name)
		}

		encOrder = append(encOrder, alg)
	}

	intOrder := make([]nas.IntegrityAlgorithm, 0, len(integrityNames))
	for _, name := range integrityNames {
		alg, ok := epsAlgorithmValue[nas.IntegrityAlgorithm](name)
		if !ok {
			return nil, nil, fmt.Errorf("unknown integrity algorithm: %s", name)
		}

		intOrder = append(intOrder, alg)
	}

	return intOrder, encOrder, nil
}

// selectEPSAlgorithm returns the first operator-preferred algorithm the UE
// advertises support for, reporting false when none is nas. The null
// algorithm is selected only when the operator lists it and the UE advertises it
// (TS 33.401 §5: EIA0 is not an implicit fallback for a non-emergency UE).
func selectEPSAlgorithm[T ~uint8](preference []T, supported func(uint8) bool) (T, bool) {
	for _, v := range preference {
		if supported(uint8(v)) {
			return v, true
		}
	}

	return 0, false
}
