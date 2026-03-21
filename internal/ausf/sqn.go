// Copyright 2026 Ella Networks
// SPDX-License-Identifier: Apache-2.0

package ausf

import (
	"crypto/hmac"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
)

// aucSQN recovers the UE's SQN from AUTS using Milenage and returns
// the recovered SQN bytes and MAC-S for verification.
func aucSQN(opc, k, auts, rand []byte) ([]byte, []byte, error) {
	AK, SQNms := make([]byte, 6), make([]byte, 6)
	macS := make([]byte, 8)
	ConcSQNms := auts[:6]

	AMF, err := hex.DecodeString("0000")
	if err != nil {
		return nil, nil, fmt.Errorf("failed to decode AMF: %w", err)
	}

	err = F2345(opc, k, rand, nil, nil, nil, nil, AK)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to generate AK: %w", err)
	}

	for i := range 6 {
		SQNms[i] = AK[i] ^ ConcSQNms[i]
	}

	err = F1(opc, k, rand, SQNms, AMF, nil, macS)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to generate macS: %w", err)
	}

	return SQNms, macS, nil
}

const sqnMax uint64 = 0x7FFFFFFFFFF // 2^43 - 1 (48-bit counter)

// incrementSQN adds 1 to sqn (mod sqnMax) and returns the new value as a
// zero-padded 12-character hex string.
func incrementSQN(sqnHex string) (string, error) {
	n, err := strconv.ParseUint(sqnHex, 16, 64)
	if err != nil {
		return "", fmt.Errorf("invalid SQN hex %q: %w", sqnHex, err)
	}

	n = (n + 1) % sqnMax

	return fmt.Sprintf("%012x", n), nil
}

// resyncSQN recovers the UE's SQN from an AUTS, verifies MAC-S, and increments.
// It returns the recovered SQN bytes (for Milenage re-run) and the next SQN hex to persist.
func resyncSQN(opc, k, auts, rand []byte) (recovered []byte, next string, err error) {
	sqnMs, macS, err := aucSQN(opc, k, auts, rand)
	if err != nil {
		return nil, "", err
	}

	if !hmac.Equal(macS, auts[6:]) {
		return nil, "", fmt.Errorf("AUTS MAC verification failed")
	}

	nextHex, err := incrementSQN(hex.EncodeToString(sqnMs))
	if err != nil {
		return nil, "", err
	}

	return sqnMs, nextHex, nil
}

// strictHex pads or truncates a hex string to exactly n characters.
func strictHex(s string, n int) string {
	l := len(s)
	if l < n {
		return strings.Repeat("0", n-l) + s
	}

	return s[l-n : l]
}
