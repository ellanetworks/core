// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package wire

import (
	"regexp"
	"strings"
)

var supportedFeaturesPattern = regexp.MustCompile(`^[A-Fa-f0-9]*$`)

type SupportedFeatures string

func Features(numbers ...int) SupportedFeatures {
	var nibbles []byte

	for _, n := range numbers {
		if n < 1 {
			continue
		}

		i := (n - 1) / 4
		for len(nibbles) <= i {
			nibbles = append(nibbles, 0)
		}

		nibbles[i] |= 1 << ((n - 1) % 4)
	}

	return fromNibbles(nibbles)
}

func (f SupportedFeatures) Check(Policy) error {
	if !supportedFeaturesPattern.MatchString(string(f)) {
		return Incorrect("", true, "%q is not hexadecimal", string(f))
	}

	return nil
}

func (f SupportedFeatures) Has(n int) bool {
	if n < 1 {
		return false
	}

	nibbles := f.nibbles()
	i := (n - 1) / 4

	return i < len(nibbles) && nibbles[i]&(1<<((n-1)%4)) != 0
}

func (f SupportedFeatures) Intersect(o SupportedFeatures) SupportedFeatures {
	a, b := f.nibbles(), o.nibbles()
	out := make([]byte, min(len(a), len(b)))

	for i := range out {
		out[i] = a[i] & b[i]
	}

	return fromNibbles(out)
}

func (f SupportedFeatures) Empty() bool {
	return len(fromNibbles(f.nibbles())) == 0
}

func (f SupportedFeatures) nibbles() []byte {
	s := string(f)
	out := make([]byte, 0, len(s))

	for i := len(s) - 1; i >= 0; i-- {
		out = append(out, hexNibble(s[i]))
	}

	return out
}

func hexNibble(c byte) byte {
	switch {
	case c >= '0' && c <= '9':
		return c - '0'
	case c >= 'a' && c <= 'f':
		return c - 'a' + 10
	case c >= 'A' && c <= 'F':
		return c - 'A' + 10
	default:
		return 0
	}
}

func fromNibbles(nibbles []byte) SupportedFeatures {
	end := len(nibbles)
	for end > 0 && nibbles[end-1] == 0 {
		end--
	}

	var b strings.Builder

	for i := end - 1; i >= 0; i-- {
		b.WriteByte("0123456789abcdef"[nibbles[i]])
	}

	return SupportedFeatures(b.String())
}
