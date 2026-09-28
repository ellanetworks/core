// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package lpptype

import (
	"fmt"

	"github.com/ellanetworks/core/per"
)

func encodeRootEnumerated(w *per.Writer, enc per.Encoding, nRoot, v int64, _ string) error {
	if v < 0 {
		return fmt.Errorf("lpp: negative ENUMERATED index %d", v)
	}

	return per.EncodeEnumerated(w, enc, nRoot, true, v)
}

func decodeRootEnumerated(r *per.Reader, enc per.Encoding, nRoot int64, _ string) (int64, error) {
	return per.DecodeEnumerated(r, enc, nRoot, true)
}
