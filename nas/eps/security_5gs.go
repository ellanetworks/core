// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package eps

import (
	"errors"
	"fmt"
	"slices"

	"github.com/ellanetworks/core/nas"
)

func VerifyWith5GContext(b []byte, count nas.Count, dir nas.Direction, sc *nas.SecurityContext) ([]byte, error) {
	if sc == nil {
		return nil, nas.ErrNoSecurityContext
	}

	m, err := ParseSecurityProtectedMessage(b)
	if err != nil {
		return nil, err
	}

	permitted := []SecurityHeaderType{SHTIntegrityProtected, SHTIntegrityProtectedNewContext}
	if !slices.Contains(permitted, m.SecurityHeaderType) {
		return nil, fmt.Errorf("%w: %s", ErrSecurityHeaderTypeNotPermitted, m.SecurityHeaderType)
	}

	if m.SequenceNumber != count.SQN() {
		return nil, fmt.Errorf("%w: message carries %#02x, NAS COUNT carries %#02x",
			ErrSequenceNumberMismatch, m.SequenceNumber, count.SQN())
	}

	if err := sc.VerifyMAC(macInput(m.SequenceNumber, m.UnverifiedPayload), m.MAC, count, nas.Bearer3GPP, dir); err != nil {
		if errors.Is(err, nas.ErrMACMismatch) {
			return nil, ErrMACMismatch
		}

		return nil, err
	}

	return m.UnverifiedPayload, nil
}
