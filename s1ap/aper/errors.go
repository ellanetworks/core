// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package aper

import (
	"errors"
	"fmt"
)

// DecodeError reports malformed input, located by the bit offset at which the
// failure was detected. It is distinct from encode-side errors (which signal a
// caller bug, such as an out-of-range value) so callers can tell a bad packet
// from a programming mistake.
type DecodeError struct {
	Offset int
	Msg    string
}

func (e *DecodeError) Error() string {
	return fmt.Sprintf("aper: %s at bit %d", e.Msg, e.Offset)
}

// ErrFragmented is returned for the 16K-fragmented form of a length
// determinant (X.691 §11.9.3.8), which S1AP needs only for payloads larger
// than 16383 octets.
var ErrFragmented = errors.New("aper: 16K-fragmented length determinant not supported")
