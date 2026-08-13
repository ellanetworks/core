// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package eps

import (
	"errors"
	"fmt"
	"slices"

	"github.com/ellanetworks/core/nas"
)

// VerifyWith5GContext verifies an EPS-framed, integrity-protected NAS message
// against a 5G NAS security context.
//
// A UE changing system from 5GS to EPS in idle mode protects its TRACKING AREA
// UPDATE REQUEST with the 5G context named by the mapped EPS GUTI, computing the
// NAS-MAC "as it is done for a 5G NAS message over a 3GPP access" over a message
// framed by TS 24.301 §9.1 (TS 33.501 §8.5.2 steps 1 and 4). The frame is
// therefore this package's, while BEARER is the 3GPP access connection
// identifier of the 5G system rather than the EPS constant [Unprotect] uses, and
// the count is the 5G uplink NAS COUNT.
//
// The caller owns the count: estimate it from the message's sequence number and
// commit it only once this returns without error, or the same message verifies
// again (TS 33.501 §8.5.2 step 1 increments the UE's stored count).
//
// The message arrives unciphered — the MME reads its Old GUTI and UE status to
// find the AMF (TS 24.301 §4.4.2.3) — so no ciphered security header type is
// admitted and the payload is returned as it was framed.
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
