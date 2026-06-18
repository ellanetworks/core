// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package common

import (
	"errors"
	"fmt"
)

var (
	// ErrTruncated is returned when a read would run past the end of the buffer.
	ErrTruncated = errors.New("buffer truncated")
	// ErrOverflow is returned when a value is too large for its length field.
	ErrOverflow = errors.New("value exceeds length field")
	// ErrDigit is returned by the TBCD/PLMN encoders on a non-decimal digit.
	ErrDigit = errors.New("non-decimal digit")
)

// Error locates a framing failure at the octet offset where it occurred.
type Error struct {
	Op     string
	Offset int
	Err    error
}

func (e *Error) Error() string {
	return fmt.Sprintf("nas/common: %s at octet %d: %v", e.Op, e.Offset, e.Err)
}

func (e *Error) Unwrap() error { return e.Err }
