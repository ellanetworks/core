// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"context"
	"crypto/sha256"
	"fmt"

	"github.com/ellanetworks/core/nas"
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
