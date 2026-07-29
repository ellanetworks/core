// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

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
	return fmt.Sprintf("nas: %s at octet %d: %v", e.Op, e.Offset, e.Err)
}

func (e *Error) Unwrap() error { return e.Err }

// ErrUnknownMessageType reports a message type the library does not model. It
// accompanies a decoded header, not a nil message: TS 24.501 §7.6 and
// TS 24.301 §7.6 require the receiver to answer with a STATUS naming the type,
// which it can only do if the header survived.
var ErrUnknownMessageType = errors.New("unknown message type")

// ErrSecurityHeaderTypeNotPermitted reports a protected message whose security
// header type the caller did not permit at this point in its procedure.
var ErrSecurityHeaderTypeNotPermitted = errors.New("security header type not permitted")

// ErrSequenceNumberMismatch reports a protected message whose sequence number is
// not the one carried by the NAS COUNT it was verified under.
var ErrSequenceNumberMismatch = errors.New("sequence number does not match the NAS COUNT")

// ErrNotPlain reports a security-protected message where a plain one was
// expected. Unwrap it first.
var ErrNotPlain = errors.New("message is security protected")

// ErrNotProtected reports a plain message where a security-protected one was
// expected.
var ErrNotProtected = errors.New("message is not security protected")

// ErrWrongMessageType reports a message type other than the one a parser
// expected.
var ErrWrongMessageType = errors.New("unexpected message type")

// ErrUnknownProtocolDiscriminator reports a protocol discriminator that names
// neither of a generation's two NAS protocols. Unlike an unknown message type it
// is a hard failure: without the discriminator the message type cannot be
// located, so there is nothing to answer with.
var ErrUnknownProtocolDiscriminator = errors.New("unknown protocol discriminator")

// IEError reports an optional information element that could not be decoded.
// The containing message is still usable: TS 24.501 §7.7.1 and TS 24.301 §7.7.1
// direct a receiver to treat a syntactically incorrect optional element as not
// present, so the field is left absent and the failure is reported alongside the
// message rather than in place of it.
//
// Raw is the element's undecodable value, kept so an observability decoder or a
// conformance tester can show what arrived. The element is also preserved in the
// message, so it re-encodes with everything that arrived.
type IEError struct {
	IEI uint8
	// Name is the element's name in its message, where the generation package
	// knows one. An IEI alone is the least useful thing a log line can carry.
	Name   string
	Format IEFormat
	Raw    []byte
	Err    error
}

func (e *IEError) Error() string {
	if e.Name == "" {
		return fmt.Sprintf("nas: optional IE %#02x: %v", e.IEI, e.Err)
	}

	return fmt.Sprintf("nas: %s (IEI %#02x): %v", e.Name, e.IEI, e.Err)
}

func (e *IEError) Unwrap() error { return e.Err }

// SoftOnly reports whether err leaves the message returned alongside it usable:
// optional elements that failed to decode, and a message type this library does
// not model. A nil error is soft.
//
// A parse returns a non-nil message exactly when its error is soft; a hard
// failure — bad framing, a malformed mandatory element, or any element in the
// exempt set — returns a nil message instead. Callers that do not need the
// distinction can ignore the error and still behave as the spec requires.
func SoftOnly(err error) bool {
	if err == nil {
		return true
	}

	if errors.Is(err, ErrUnknownMessageType) {
		return true
	}

	var joined interface{ Unwrap() []error }
	if errors.As(err, &joined) {
		for _, e := range joined.Unwrap() {
			if !SoftOnly(e) {
				return false
			}
		}

		return true
	}

	var ieErr *IEError

	return errors.As(err, &ieErr)
}

// IEErrors returns every optional-element failure joined into err, in walk
// order. errors.As finds only the first, so a caller that wants to report each
// dropped element needs this.
func IEErrors(err error) []*IEError {
	if err == nil {
		return nil
	}

	var joined interface{ Unwrap() []error }
	if errors.As(err, &joined) {
		var out []*IEError
		for _, e := range joined.Unwrap() {
			out = append(out, IEErrors(e)...)
		}

		return out
	}

	var ieErr *IEError
	if errors.As(err, &ieErr) {
		return []*IEError{ieErr}
	}

	return nil
}
