// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package smf

import (
	"context"
	"fmt"

	"github.com/ellanetworks/core/internal/logger"
	"go.uber.org/zap"
)

func (s *SMF) epsBearerIdentityAvailable(sc *SMContext, ebi uint8) error {
	if !(SessionIdentity{PDUSessionID: sc.PDUSessionID, EBI: ebi}).valid() {
		return fmt.Errorf("%w: EPS bearer identity %d names no default bearer", ErrSessionNotMovable, ebi)
	}

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

func (s *SMF) assignEPSBearerIdentity(ctx context.Context, sc *SMContext, ebi uint8) {
	if err := s.setEPSBearerIdentity(sc, ebi); err != nil {
		logger.WithTrace(ctx, logger.SmfLog).Error("failed to restore the EPS bearer identity of a session whose move was refused",
			zap.Error(err), logger.SUPI(sc.Supi.String()),
			logger.PDUSessionID(sc.PDUSessionID), zap.Uint8("ebi", ebi))
	}
}
