// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package lmf

// RequestType identifies the type of location request.
type RequestType string

const (
	RequestImmediate RequestType = "immediate"
	RequestPeriodic  RequestType = "periodic"
	RequestTriggered RequestType = "triggered"
	RequestCancel    RequestType = "cancel"
)

// SessionType identifies the kind of positioning session.
type SessionType int

const (
	SessionTypeImmediate SessionType = 0 // One-shot location request
	SessionTypePeriodic  SessionType = 1 // Repeated at interval (scheduler deferred)
	SessionTypeTriggered SessionType = 2 // Area/event trigger (scheduler deferred)
)

// SessionStatus tracks the lifecycle of a positioning session.
type SessionStatus int

const (
	SessionStatusActive    SessionStatus = 0
	SessionStatusCompleted SessionStatus = 1
	SessionStatusFailed    SessionStatus = 2
	SessionStatusCancelled SessionStatus = 3
)

// PositioningMethod identifies the positioning algorithm to use.
type PositioningMethod string

const (
	MethodCellID        PositioningMethod = "cell_id"
	MethodECID          PositioningMethod = "ecid"
	MethodAGNSSAssisted PositioningMethod = "agnss_ue_assisted"
	MethodAGNSSBased    PositioningMethod = "agnss_ue_based"
)

// DefaultMethodForRequest returns the default positioning method for a request type.
func DefaultMethodForRequest(rt RequestType) PositioningMethod {
	return MethodCellID
}

// SessionTypeFromRequest maps RequestType to the internal SessionType.
func SessionTypeFromRequest(rt RequestType) SessionType {
	switch rt {
	case RequestPeriodic:
		return SessionTypePeriodic
	case RequestTriggered:
		return SessionTypeTriggered
	case RequestImmediate, RequestCancel:
		return SessionTypeImmediate
	default:
		return SessionTypeImmediate
	}
}
