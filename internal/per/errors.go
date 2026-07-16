package per

import "errors"

// Errors returned by the codec. Callers may test with errors.Is.
var (
	// ErrTruncated is returned when the input ends before a complete value
	// could be decoded.
	ErrTruncated = errors.New("per: truncated input")
	// ErrOverflow is returned when a value exceeds its declared constraint
	// (range, size, length) during encoding or decoding.
	ErrOverflow = errors.New("per: value out of range")
	// ErrUnaligned is returned when a byte-oriented operation is attempted
	// on a reader or writer that is not octet-aligned.
	ErrUnaligned = errors.New("per: not octet-aligned")
	// ErrEmpty is returned when a CHOICE or mandatory field has no value set.
	ErrEmpty = errors.New("per: no value set")
)
