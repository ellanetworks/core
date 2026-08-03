// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import "github.com/ellanetworks/core/per"

func unmarshalPERValue[T any](b []byte) (T, error) {
	var v T

	err := any(&v).(per.Unmarshaler).UnmarshalPER(per.NewReader(b), per.Aligned)

	return v, err
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
