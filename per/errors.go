// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package per

import "errors"

var (
	// ErrTruncated: the input ended before a complete value could be decoded.
	ErrTruncated = errors.New("per: truncated input")
	// ErrOverflow: a value exceeds its declared range, size or length constraint.
	ErrOverflow = errors.New("per: value out of range")
	// ErrUnaligned: an octet-oriented operation on a non-octet-aligned position.
	ErrUnaligned = errors.New("per: not octet-aligned")
	// ErrEmpty: a CHOICE or mandatory field has no value set.
	ErrEmpty = errors.New("per: no value set")
)
