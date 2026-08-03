// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"testing"

	"github.com/ellanetworks/core/per"
)

func TestPagingDRXRoundTrip(t *testing.T) {
	for _, p := range []PagingDRX{PagingDRXv32, PagingDRXv64, PagingDRXv128, PagingDRXv256} {
		w := per.NewWriter()

		if err := p.MarshalPER(w, per.Aligned); err != nil {
			t.Fatal(err)
		}

		got, err := unmarshalPERValue[PagingDRX](perBytes(w))
		if err != nil || got != p {
			t.Fatalf("p=%d: decoded %d err=%v", p, got, err)
		}
	}
}
