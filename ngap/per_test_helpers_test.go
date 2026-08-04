// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"testing"

	"github.com/ellanetworks/core/per"
)

func unmarshalPERValue[T any](b []byte) (T, error) {
	var v T

	err := any(&v).(per.Unmarshaler).UnmarshalPER(per.NewReader(b), per.Aligned)

	return v, err
}

// ieRaw encodes one IE value, as it appears inside a ProtocolIE-Field open type.
func ieRaw(t *testing.T, m per.Marshaler) []byte {
	t.Helper()

	w := per.NewWriter()
	if err := m.MarshalPER(w, per.Aligned); err != nil {
		t.Fatal(err)
	}

	return perBytes(w)
}

// perBytes applies the padding implicit in a complete PER encoding.
func perBytes(w *per.Writer) []byte {
	w.AlignToByte()

	return w.Bytes()
}

func deref[T any](p *T) T {
	var zero T

	if p == nil {
		return zero
	}

	return *p
}
