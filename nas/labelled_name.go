// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"bytes"
	"fmt"
	"strings"
)

// Label and name maxima (TS 23.003 §9.1, TS 24.008 §10.5.6.1,
// TS 24.501 §9.11.2.1A): each label is a domain-name label, and the element
// holding the labels is at most 102 octets, two of which are its IEI and length.
const (
	maxLabelLen        = 63
	maxLabelledNameLen = 100
)

// ParseLabelledName decodes a sequence of RFC 1035 labels — each a 1-octet
// length followed by its characters — into its dotted form (TS 23.003). It is
// the encoding shared by the EPS Access Point Name and the 5GS Data Network
// Name.
func ParseLabelledName(b []byte) (string, error) {
	// Both elements are a type-4 IE of at least three octets — an IEI, a length,
	// and one octet of content (TS 24.501 §9.11.2.1B, TS 24.008 §10.5.6.1), so an
	// empty value names no network and is syntactically incorrect.
	if len(b) == 0 {
		return "", fmt.Errorf("nas: labelled name is empty, want at least 1 octet")
	}

	if len(b) > maxLabelledNameLen {
		return "", fmt.Errorf("nas: labelled name is %d octets, want at most %d", len(b), maxLabelledNameLen)
	}

	r := NewReader(b)

	var labels []string

	for r.Remaining() > 0 {
		label, err := r.LV()
		if err != nil {
			return "", err
		}

		if len(label) == 0 {
			return "", fmt.Errorf("nas: label is empty, want at least 1 octet")
		}

		if len(label) > maxLabelLen {
			return "", fmt.Errorf("nas: label is %d octets, want at most %d", len(label), maxLabelLen)
		}

		// The dotted form separates labels with a dot, so a label carrying one
		// has no dotted form to decode to: re-encoding would split it in two and
		// emit a different element (RFC 1035 §2.3.1 excludes it from a label).
		if bytes.ContainsRune(label, '.') {
			return "", fmt.Errorf("nas: label %q contains a dot, which separates labels", label)
		}

		labels = append(labels, string(label))
	}

	return strings.Join(labels, "."), nil
}

// AppendLabelledName encodes the dotted form of name as RFC 1035 labels onto b
// (TS 23.003).
func AppendLabelledName(b []byte, name string) ([]byte, error) {
	if name == "" {
		return b, fmt.Errorf("nas: labelled name is empty, which no peer accepts")
	}

	w := NewWriter(b)

	for _, label := range strings.Split(name, ".") {
		// A label carries its own length, so an empty one is a zero-length label
		// on the wire — which no receiver can tell from the end of the element.
		if len(label) == 0 {
			return b, fmt.Errorf("nas: %q has an empty label, which no peer accepts", name)
		}

		if len(label) > maxLabelLen {
			return b, fmt.Errorf("nas: label is %d octets, want at most %d", len(label), maxLabelLen)
		}

		w.U8(uint8(len(label)))
		w.Raw([]byte(label))
	}

	out, err := w.Result(b)
	if err != nil {
		return b, err
	}

	if len(out)-len(b) > maxLabelledNameLen {
		return b, fmt.Errorf("nas: labelled name is %d octets, want at most %d", len(out)-len(b), maxLabelledNameLen)
	}

	return out, nil
}
