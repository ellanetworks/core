// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package smf

import (
	"context"
	"fmt"

	"github.com/ellanetworks/core/internal/logger"
	"go.uber.org/zap"
)

// epsBearerIdentityAvailable reports whether ebi names a default bearer this
// session may take, without claiming it. A move is validated when the UE asks
// and claimed when the target access binds, because an identity claimed for a
// move the target abandons would be held by a session reachable under neither
// access. Caller holds sc.mu; must not hold s.mu.
func (s *SMF) epsBearerIdentityAvailable(sc *SMContext, ebi uint8) error {
	if !(SessionIdentity{PDUSessionID: sc.PDUSessionID, EBI: ebi}).valid() {
		return fmt.Errorf("%w: EPS bearer identity %d names no default bearer", ErrSessionNotMovable, ebi)
	}

	// The key is what the UE address is leased under, so a session that has no
	// PDU session identity to stand on cannot re-key onto another EPS bearer.
	if sc.PDUSessionID == 0 {
		return fmt.Errorf("%w: session %q is keyed on its EPS bearer identity", ErrSessionNotMovable, sc.Ref)
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	if held := s.byKey[canonicalName(sc.Supi, epsBearerKey(ebi))]; held != nil && held != sc {
		return fmt.Errorf("%w: EPS bearer identity %d is held by session %q", ErrSessionNotMovable, ebi, held.Ref)
	}

	return nil
}

// setEPSBearerIdentity gives the session a new EPS bearer identity, re-indexing
// it so a lookup in either namespace still resolves it. ebi of 0 drops the
// identity, which is what a move to 5GS does: the EPS bearer the UE left no
// longer exists, and leaving the key indexed would refuse the next attach that
// legitimately allocates it. The session key itself never changes. Caller holds
// sc.mu.
func (s *SMF) setEPSBearerIdentity(sc *SMContext, ebi uint8) error {
	if sc.EBI == ebi {
		return nil
	}

	if ebi != 0 {
		if err := s.epsBearerIdentityAvailable(sc, ebi); err != nil {
			return err
		}
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// A session already dropped from the pool must not re-insert a key: its
	// release has run, and the entry would outlive it and refuse the slot forever.
	if s.pool[sc.Ref] != sc {
		return fmt.Errorf("%w: session %q is no longer in the pool", ErrSessionNotMovable, sc.Ref)
	}

	if sc.EBI != 0 {
		key := canonicalName(sc.Supi, epsBearerKey(sc.EBI))
		if s.byKey[key] == sc {
			delete(s.byKey, key)
		}
	}

	sc.EBI = ebi

	if ebi != 0 {
		s.byKey[canonicalName(sc.Supi, epsBearerKey(ebi))] = sc
	}

	return nil
}

// assignEPSBearerIdentity is setEPSBearerIdentity where the caller has no answer
// to a failure: it is unwinding a move whose bind the UPF refused, and the
// identity is then out of step with the access the session is on. Caller holds
// sc.mu.
func (s *SMF) assignEPSBearerIdentity(ctx context.Context, sc *SMContext, ebi uint8) {
	if err := s.setEPSBearerIdentity(sc, ebi); err != nil {
		logger.WithTrace(ctx, logger.SmfLog).Error("failed to restore the EPS bearer identity of a session whose move was refused",
			zap.Error(err), logger.SUPI(sc.Supi.String()),
			logger.PDUSessionID(sc.PDUSessionID), zap.Uint8("ebi", ebi))
	}
}
