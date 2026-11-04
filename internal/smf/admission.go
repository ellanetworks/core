// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package smf

import (
	"context"
	"errors"
	"time"

	"github.com/ellanetworks/core/nas"
	"github.com/ellanetworks/core/nas/fgs"
	"golang.org/x/sys/unix"
)

type datapathCapacityError struct{}

func (datapathCapacityError) Error() string { return "datapath session capacity reached" }

func (datapathCapacityError) DatapathCapacity() bool { return true }

var ErrDatapathCapacity error = datapathCapacityError{}

const CapacityBackoff = 60 * time.Second

func WithMaxSessions(n uint32) Option { return func(s *SMF) { s.maxSessions = n } }

func (s *SMF) atCapacityLocked() bool {
	return s.maxSessions > 0 && len(s.pool) >= int(s.maxSessions)
}

func datapathFull(err error) bool {
	return errors.Is(err, unix.E2BIG) || errors.Is(err, unix.ENOMEM) || errors.Is(err, unix.ENOSPC)
}

type highPriorityAccessKey struct{}

func WithHighPriorityAccess(ctx context.Context, high bool) context.Context {
	return context.WithValue(ctx, highPriorityAccessKey{}, high)
}

func highPriorityAccess(ctx context.Context) bool {
	high, ok := ctx.Value(highPriorityAccessKey{}).(bool)

	return !ok || high
}

func emergencyRequest(t fgs.RequestType) bool {
	return t == fgs.RequestTypeInitialEmergencyRequest || t == fgs.RequestTypeExistingEmergencyPDUSession
}

func capacityBackoff(ctx context.Context, err error, requestType fgs.RequestType) *nas.GPRSTimer3 {
	if !errors.Is(err, ErrDatapathCapacity) || emergencyRequest(requestType) || highPriorityAccess(ctx) {
		return nil
	}

	timer, tErr := nas.GPRSTimer3FromDuration(CapacityBackoff)
	if tErr != nil {
		return nil
	}

	return &timer
}
