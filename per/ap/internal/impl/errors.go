// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package impl

import (
	"errors"
	"fmt"
)

var (
	ErrProfile                = errors.New("ap: invalid profile")
	ErrExtensionAlternative   = errors.New("ap: PDU extension alternative")
	ErrNotProtocolIEContainer = errors.New("ap: message body is not a ProtocolIE-Container")
	ErrIndex                  = errors.New("ap: field index out of range")
	ErrTooManyFields          = errors.New("ap: more than maxProtocolIEs fields")
	ErrCriticality            = errors.New("ap: criticality out of range")
	ErrDuplicateIE            = errors.New("ap: IE present more than once")
	ErrIENotPresent           = errors.New("ap: IE not present")
)

type TransferSyntaxError[P ~uint8] struct {
	Protocol       string
	Procedure      P
	ProcedureKnown bool
	Err            error
}

func (e *TransferSyntaxError[P]) Error() string {
	if !e.ProcedureKnown {
		return fmt.Sprintf("%s: transfer syntax error: %v", e.Protocol, e.Err)
	}

	return fmt.Sprintf("%s: %v: transfer syntax error: %v", e.Protocol, e.Procedure, e.Err)
}

func (e *TransferSyntaxError[P]) Unwrap() error { return e.Err }
