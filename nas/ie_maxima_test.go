// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"strings"
	"testing"
)

// TestPCOMaximum checks that protocol configuration options longer than
// TS 24.008 §10.5.6.3 allows are rejected in both directions.
func TestPCOMaximum(t *testing.T) {
	if _, err := ParseProtocolConfigurationOptions(make([]byte, maxPCOLen+1), PCOMSToNetwork); err == nil {
		t.Error("parse: want an error, got none")
	}

	p := ProtocolConfigurationOptions{
		Direction:  PCOMSToNetwork,
		Containers: []PCOContainer{{ID: 0x000D, Content: make([]byte, maxPCOLen)}},
	}
	if _, err := p.MarshalBinary(); err == nil {
		t.Error("encode: want an error, got none")
	}
}

// TestLabelledNameMaxima checks the label and total lengths TS 23.003 §9.1 and
// TS 24.008 §10.5.6.1 fix for an APN or DNN.
func TestLabelledNameMaxima(t *testing.T) {
	if _, err := AppendLabelledName(nil, strings.Repeat("a", maxLabelLen+1)); err == nil {
		t.Error("over-long label: want an error, got none")
	}

	// Six labels of 63 characters encode to 384 octets, past the element's 100.
	long := strings.Join([]string{
		strings.Repeat("a", maxLabelLen), strings.Repeat("b", maxLabelLen), strings.Repeat("c", maxLabelLen),
		strings.Repeat("d", maxLabelLen), strings.Repeat("e", maxLabelLen), strings.Repeat("f", maxLabelLen),
	}, ".")
	if _, err := AppendLabelledName(nil, long); err == nil {
		t.Error("over-long name: want an error, got none")
	}

	if _, err := ParseLabelledName(make([]byte, maxLabelledNameLen+1)); err == nil {
		t.Error("parse over-long name: want an error, got none")
	}

	// A single label longer than 63 octets, inside a value short enough overall.
	over := append([]byte{maxLabelLen + 1}, make([]byte, maxLabelLen+1)...)
	if _, err := ParseLabelledName(over); err == nil {
		t.Error("parse over-long label: want an error, got none")
	}
}
