// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package per

import (
	"time"
)

// EncodeString encodes a non-known-multiplier character string — UTF8String,
// TeletexString, VideotexString, GraphicString, GeneralString,
// ObjectDescriptor — as its BER octets preceded by an unconstrained length
// determinant (§30.6).
func EncodeString(w *Writer, enc Encoding, data []byte) error {
	off := 0

	return EncodeLength(w, enc, 0, 0, false, int64(len(data)), func(count int64) error {
		end := off + int(count)
		writeOctetAligned(w, enc, data[off:end])
		off = end

		return nil
	})
}

// DecodeString decodes a non-known-multiplier character string (§30.6).
func DecodeString(r *Reader, enc Encoding) ([]byte, error) {
	var buf []byte

	err := DecodeLength(r, enc, 0, 0, false, func(count int64) error {
		p, err := readOctetAligned(r, enc, int(count))
		if err != nil {
			return err
		}

		buf = append(buf, p...)

		return nil
	})

	return buf, err
}

// EncodeGeneralizedTime encodes a GeneralizedTime as BER content octets
// preceded by an unconstrained length, the unoptimized path of §32.11.
func EncodeGeneralizedTime(w *Writer, enc Encoding, t time.Time) error {
	s := t.UTC().Format("20060102150405Z")
	return EncodeString(w, enc, []byte(s))
}

// DecodeGeneralizedTime decodes a GeneralizedTime (§32).
func DecodeGeneralizedTime(r *Reader, enc Encoding) (time.Time, error) {
	buf, err := DecodeString(r, enc)
	if err != nil {
		return time.Time{}, err
	}

	layouts := []string{
		"20060102150405Z",
		"20060102150405-0700",
		"20060102150405+0700",
		"20060102150405.0Z",
	}
	for _, layout := range layouts {
		if t, err := time.Parse(layout, string(buf)); err == nil {
			return t, nil
		}
	}

	return time.Parse("20060102150405Z", string(buf))
}

// EncodeUTCTime encodes a UTCTime as BER content octets preceded by an
// unconstrained length, the unoptimized path of §32.11.
func EncodeUTCTime(w *Writer, enc Encoding, t time.Time) error {
	s := t.UTC().Format("060102150405Z")
	return EncodeString(w, enc, []byte(s))
}

// DecodeUTCTime decodes a UTCTime (§32).
func DecodeUTCTime(r *Reader, enc Encoding) (time.Time, error) {
	buf, err := DecodeString(r, enc)
	if err != nil {
		return time.Time{}, err
	}

	layouts := []string{
		"060102150405Z",
		"060102150405-0700",
		"060102150405+0700",
	}
	for _, layout := range layouts {
		if t, err := time.Parse(layout, string(buf)); err == nil {
			return t, nil
		}
	}

	return time.Parse("060102150405Z", string(buf))
}
