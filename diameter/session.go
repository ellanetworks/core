// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package diameter

import "fmt"

type SessionBinding uint32

const (
	SessionBindingReAuth     SessionBinding = 1
	SessionBindingSTR        SessionBinding = 2
	SessionBindingAccounting SessionBinding = 4
)

func (b SessionBinding) Has(bit SessionBinding) bool {
	return b&bit != 0
}

type SessionServerFailover uint32

const (
	RefuseService        SessionServerFailover = 0
	TryAgain             SessionServerFailover = 1
	AllowService         SessionServerFailover = 2
	TryAgainAllowService SessionServerFailover = 3
)

func (f SessionServerFailover) Valid() bool {
	return f <= TryAgainAllowService
}

func (f SessionServerFailover) TriesAgain() bool {
	return f == TryAgain || f == TryAgainAllowService
}

func (f SessionServerFailover) AllowsService() bool {
	return f == AllowService || f == TryAgainAllowService
}

func (f SessionServerFailover) String() string {
	switch f {
	case RefuseService:
		return "REFUSE_SERVICE"
	case TryAgain:
		return "TRY_AGAIN"
	case AllowService:
		return "ALLOW_SERVICE"
	case TryAgainAllowService:
		return "TRY_AGAIN_ALLOW_SERVICE"
	default:
		return fmt.Sprintf("SessionServerFailover(%d)", uint32(f))
	}
}
