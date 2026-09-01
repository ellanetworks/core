// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package utils_test

import (
	"testing"

	"github.com/ellanetworks/core/internal/decoder/utils"
	"github.com/ellanetworks/core/nas"
)

func TestRawIEs(t *testing.T) {
	raw := []nas.RawIE{
		{IEI: 0x52, Value: []byte{0x99, 0xf9}},
		{IEI: 0x5d, Value: []byte{0x06}},
	}

	got := utils.RawIEs(raw)
	if len(got) != 2 {
		t.Fatalf("got %d elements, want 2", len(got))
	}

	if got[0].IEI != 0x52 || got[0].Hex != "99f9" {
		t.Errorf("got %+v, want IEI 0x52 hex 99f9", got[0])
	}
}

// An element a builder already renders itself must not also appear as a raw one.
func TestRawIEsExceptSkipsHandled(t *testing.T) {
	raw := []nas.RawIE{
		{IEI: 0x52, Value: []byte{0x01}},
		{IEI: 0x5d, Value: []byte{0x02}},
	}

	got := utils.RawIEsExcept(raw, 0x52)
	if len(got) != 1 || got[0].IEI != 0x5d {
		t.Fatalf("got %+v, want only IEI 0x5d", got)
	}
}

func TestRawIEsEmptyStaysNil(t *testing.T) {
	if got := utils.RawIEs(nil); got != nil {
		t.Fatalf("got %+v, want nil so the field stays out of the JSON", got)
	}
}
