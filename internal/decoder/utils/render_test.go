// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package utils

import (
	"encoding/json"
	"slices"
	"testing"

	"github.com/ellanetworks/core/per"
	"github.com/ellanetworks/core/per/ap"
)

type testEnum uint8

func (e testEnum) Name() string {
	if e == 0 {
		return "first"
	}

	return ""
}

type testBits struct{}

func (testBits) Bits() int     { return 12 }
func (testBits) Bytes() []byte { return []byte{0xab, 0xc0} }

type testChoice struct {
	First  *testEnum
	Second *per.Null
	Ext    *per.ExtensionAlternative
}

type testSequence struct {
	TransportLayerAddress ap.TransportLayerAddress
	GTPTEID               uint32
	Absent                *testEnum
	Empty                 []testEnum
	Octets                []byte
	Bits                  testBits
	Choice                testChoice
	List                  []testEnum
	hidden                int
}

func TestRender(t *testing.T) {
	v := testSequence{
		TransportLayerAddress: ap.TransportLayerAddressFromOctets([]byte{10, 0, 0, 1}),
		GTPTEID:               7,
		Octets:                []byte{0x01, 0x02},
		Choice:                testChoice{Second: &per.Null{}, Ext: &per.ExtensionAlternative{Index: 3, Value: "\x0f"}},
		List:                  []testEnum{0, 5},
		hidden:                1,
	}

	b, err := json.Marshal(Render(&v, nil))
	if err != nil {
		t.Fatal(err)
	}

	want := `{"transport_layer_address":"10.0.0.1","gtp_teid":7,"octets":{"hex":"0102"},"bits":{"hex":"abc0","bits":12},` +
		`"choice":{"second":true,"ext":{"index":3,"hex":"0f"}},` +
		`"list":[{"type":"enum","value":0,"label":"first","unknown":false},{"type":"enum","value":5,"label":"","unknown":true}]}`

	if string(b) != want {
		t.Errorf("Render =\n%s\nwant\n%s", b, want)
	}
}

func TestRenderLeafOverrides(t *testing.T) {
	leaf := func(v any) (any, bool) {
		if _, ok := v.(testEnum); ok {
			return "overridden", true
		}

		return nil, false
	}

	b, err := json.Marshal(Render([]testEnum{1}, leaf))
	if err != nil {
		t.Fatal(err)
	}

	if string(b) != `["overridden"]` {
		t.Errorf("Render = %s", b)
	}
}

func TestBitStringUint(t *testing.T) {
	got := BitStringUint(0x155, 10)
	if got.Hex != "5540" || got.Bits != 10 {
		t.Errorf("BitStringUint = %+v, want left-aligned 5540/10", got)
	}
}

func TestPresent(t *testing.T) {
	var nilPtr *testEnum

	for _, v := range []any{nil, nilPtr, []byte{}, []testEnum(nil)} {
		if Present(v) {
			t.Errorf("Present(%#v) = true", v)
		}
	}

	for _, v := range []any{testEnum(0), new(testEnum), []byte{0}} {
		if !Present(v) {
			t.Errorf("Present(%#v) = false", v)
		}
	}
}

func TestWireOrder(t *testing.T) {
	got := WireOrder([]int64{3, 1, 3, 2}, []int64{1, 2, 3, 9, 3}, func(v int64) int64 { return v })

	if want := []int64{3, 1, 3, 2, 9}; !slices.Equal(got, want) {
		t.Errorf("WireOrder = %v, want %v", got, want)
	}
}
