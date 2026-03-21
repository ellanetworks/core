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
	if len(auts) < 14 {
		return nil, nil, fmt.Errorf("AUTS too short: need 14 bytes, got %d", len(auts))
	}

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

const sqnMax uint64 = 0x7FFFFFFFFFF // 2^43 - 1; bitwise AND mask

// indStep is 2^IND_LEN (IND_LEN = 5 per TS 33.102 Annex C) — the number
// of IND slots. Incrementing SQN by this value advances SEQ by 1 while
// keeping the same IND, which is the standard scheme for HE-side
// sequence-number management.
const indStep uint64 = 32

// advanceSQN adds delta to sqn, masked to 43 bits, and returns the
// new value as a zero-padded 12-character hex string.
func advanceSQN(sqnHex string, delta uint64) (string, error) {
	n, err := strconv.ParseUint(sqnHex, 16, 64)
	if err != nil {
		return "", fmt.Errorf("invalid SQN hex %q: %w", sqnHex, err)
	}

	n = (n + delta) & sqnMax

	return fmt.Sprintf("%012x", n), nil
}

// resyncSQN recovers the UE's SQN from an AUTS and verifies MAC-S.
// It returns the recovered SQN_MS as a hex string. The caller is
// responsible for incrementing before use.
func resyncSQN(opc, k, auts, rand []byte) (string, error) {
	sqnMs, macS, err := aucSQN(opc, k, auts, rand)
	if err != nil {
		return "", err
	}

	if !hmac.Equal(macS, auts[6:]) {
		return "", fmt.Errorf("AUTS MAC verification failed")
	}

	return hex.EncodeToString(sqnMs), nil
}

// strictHex pads or truncates a hex string to exactly n characters.
func strictHex(s string, n int) string {
	l := len(s)
	if l < n {
		return strings.Repeat("0", n-l) + s
	}

	return s[l-n : l]
}
