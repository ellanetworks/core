// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import "github.com/ellanetworks/core/per"

// unmarshalPERValue decodes a complete aligned-PER value of type T from b.
func unmarshalPERValue[T any](b []byte) (T, error) {
	var v T

	err := any(&v).(per.Unmarshaler).UnmarshalPER(per.NewReader(b), per.Aligned)

	return v, err
}

// perBytes pads w to an octet boundary and returns its bytes, mirroring the
// implicit padding of a complete PER encoding.
func perBytes(w *per.Writer) []byte {
	w.AlignToByte()

	return w.Bytes()
}
