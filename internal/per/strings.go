package per

import (
	"time"
)

// ---- Non-known-multiplier restricted character strings (§30.6) -------------
//
// These types (UTF8String, TeletexString, VideotexString, GraphicString,
// GeneralString, ObjectDescriptor) are encoded as BER base encoding (the raw
// octets of the character string) preceded by an unconstrained length
// determinant in octets.

// EncodeString encodes a non-known-multiplier character string (e.g. UTF8String)
// per §30.6: raw octets preceded by an unconstrained length determinant.
func EncodeString(w *Writer, enc Encoding, data []byte) error {
	off := 0

	return EncodeLength(w, enc, 0, 0, false, int64(len(data)), func(count int64) error {
		end := off + int(count)
		writeOctetAligned(w, enc, data[off:end])
		off = end

		return nil
	})
}

// DecodeString decodes a non-known-multiplier character string per §30.6.
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

// ---- Time types (§32) -------------------------------------------------------
//
// GeneralizedTime, UTCTime, Date, TimeOfDay, DateTime, Duration are encoded
// as the BER content octets of the corresponding time string, preceded by an
// unconstrained length determinant. This is the simplest (non-optimized) path
// for time types per §32.11.

// EncodeGeneralizedTime encodes a GeneralizedTime per §32 (via BER content +
// unconstrained length). The time is formatted as "20060102150405Z" (or with
// fractional seconds if non-zero).
func EncodeGeneralizedTime(w *Writer, enc Encoding, t time.Time) error {
	s := t.UTC().Format("20060102150405Z")
	return EncodeString(w, enc, []byte(s))
}

// DecodeGeneralizedTime decodes a GeneralizedTime per §32.
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

// EncodeUTCTime encodes a UTCTime per §32 (via BER content + unconstrained
// length). The time is formatted as "060102150405Z".
func EncodeUTCTime(w *Writer, enc Encoding, t time.Time) error {
	s := t.UTC().Format("060102150405Z")
	return EncodeString(w, enc, []byte(s))
}

// DecodeUTCTime decodes a UTCTime per §32.
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
