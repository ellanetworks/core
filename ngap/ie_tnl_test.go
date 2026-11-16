// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"bytes"
	"testing"

	"github.com/ellanetworks/core/per"
)

func TestNGRANTNLAssociationToRemoveListRoundTrip(t *testing.T) {
	in := NGRANTNLAssociationToRemoveList{
		{
			TNLAssociationTransportLayerAddress:    CPTransportLayerInformation{EndpointIPAddress: Ptr(tlaOf(10, 3, 0, 3))},
			TNLAssociationTransportLayerAddressAMF: &CPTransportLayerInformation{EndpointIPAddress: Ptr(tlaOf(10, 3, 0, 2))},
		},
		// The AMF-side address is optional; an item may name only its own.
		{TNLAssociationTransportLayerAddress: CPTransportLayerInformation{
			EndpointIPAddress: Ptr(tlaOf(
				0x20, 0x01, 0x0d, 0xb8, 0, 3, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0x11,
			)),
		}},
	}

	w := per.NewWriter()
	if err := in.MarshalPER(w, per.Aligned); err != nil {
		t.Fatalf("marshal: %v", err)
	}

	out, err := unmarshalPERValue[NGRANTNLAssociationToRemoveList](perBytes(w))
	if err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if len(out) != len(in) {
		t.Fatalf("%d items, want %d", len(out), len(in))
	}

	for i := range in {
		if *out[i].TNLAssociationTransportLayerAddress.EndpointIPAddress !=
			*in[i].TNLAssociationTransportLayerAddress.EndpointIPAddress {
			t.Errorf("item %d address = %v, want %v",
				i, out[i].TNLAssociationTransportLayerAddress.EndpointIPAddress,
				in[i].TNLAssociationTransportLayerAddress.EndpointIPAddress)
		}

		gotAMF, wantAMF := out[i].TNLAssociationTransportLayerAddressAMF, in[i].TNLAssociationTransportLayerAddressAMF
		if (gotAMF == nil) != (wantAMF == nil) {
			t.Fatalf("item %d AMF address presence = %v, want %v", i, gotAMF, wantAMF)
		}

		if wantAMF != nil && *gotAMF.EndpointIPAddress != *wantAMF.EndpointIPAddress {
			t.Errorf("item %d AMF address = %v, want %v", i, gotAMF.EndpointIPAddress, wantAMF.EndpointIPAddress)
		}
	}
}

// The one §9.3 item SEQUENCE with no extension marker: its preamble is the two
// OPTIONAL bits. A leading extension bit would shift every field by one.
func TestNGRANTNLAssociationToRemoveItemPreambleHasNoExtensionBit(t *testing.T) {
	encode := func(t *testing.T, item NGRANTNLAssociationToRemoveItem) []byte {
		t.Helper()

		w := per.NewWriter()
		if err := item.MarshalPER(w, per.Aligned); err != nil {
			t.Fatalf("marshal: %v", err)
		}

		return perBytes(w)
	}

	addr := CPTransportLayerInformation{EndpointIPAddress: Ptr(tlaOf(10, 3, 0, 3))}

	without := encode(t, NGRANTNLAssociationToRemoveItem{TNLAssociationTransportLayerAddress: addr})
	with := encode(t, NGRANTNLAssociationToRemoveItem{
		TNLAssociationTransportLayerAddress:    addr,
		TNLAssociationTransportLayerAddressAMF: &addr,
	})

	// Bit 0 is the AMF-address presence bit, not an extension bit.
	if got := without[0] & 0x80; got != 0x00 {
		t.Errorf("first bit with AMF address absent = %#02x, want 0 (presence bit, not an extension bit)", got)
	}

	if got := with[0] & 0x80; got != 0x80 {
		t.Errorf("first bit with AMF address present = %#02x, want the high bit set", got)
	}
}

func TestCPTransportLayerInformationChoiceExtensions(t *testing.T) {
	port := EndpointIPAddressAndPort{EndpointIPAddress: tlaOf(192, 0, 2, 1), PortNumber: 38470}

	for _, c := range []struct {
		name string
		raw  []byte
		want func(CPTransportLayerInformation) bool
	}{
		{"typed", ieRaw(t, &CPTransportLayerInformation{EndpointIPAddressAndPort: &port}), func(v CPTransportLayerInformation) bool {
			return v.EndpointIPAddressAndPort != nil && v.ChoiceExtension == nil
		}},
		{"unknown", choiceExtensionValue(t, 2, 1, IDAMFUENGAPID), func(v CPTransportLayerInformation) bool {
			return v.ChoiceExtension != nil && v.ChoiceExtension.ID == IDAMFUENGAPID
		}},
	} {
		t.Run(c.name, func(t *testing.T) {
			got, err := unmarshalPERValue[CPTransportLayerInformation](c.raw)
			if err != nil {
				t.Fatalf("decode is lenient, got %v", err)
			}

			if !c.want(got) {
				t.Fatalf("decoded %+v", got)
			}

			if !bytes.Equal(ieRaw(t, &got), c.raw) {
				t.Fatal("did not re-encode byte-exact")
			}
		})
	}
}

func TestCPTransportLayerInformationKeepsWireCriticality(t *testing.T) {
	port := EndpointIPAddressAndPort{EndpointIPAddress: tlaOf(192, 0, 2, 1), PortNumber: 38470}

	w := per.NewWriter()
	if err := per.EncodeConstrainedWholeNumber(w, per.Aligned, 0, 1, 1); err != nil {
		t.Fatal(err)
	}

	if err := encodeContainerField(w, ieField{ID: IDEndpointIPAddressAndPort, Criticality: CriticalityIgnore, Value: &port}); err != nil {
		t.Fatal(err)
	}

	raw := perBytes(w)

	got, err := unmarshalPERValue[CPTransportLayerInformation](raw)
	if err != nil || got.EndpointIPAddressAndPort == nil {
		t.Fatalf("decoded %+v, %v; want the typed alternative", got, err)
	}

	if !bytes.Equal(ieRaw(t, &got), raw) {
		t.Fatal("the peer's criticality was not kept")
	}
}

// The list is SIZE(1..maxnoofTNLAssociations), so an empty one cannot be encoded.
func TestNGRANTNLAssociationToRemoveListRejectsEmpty(t *testing.T) {
	w := per.NewWriter()
	if err := (NGRANTNLAssociationToRemoveList{}).MarshalPER(w, per.Aligned); err == nil {
		t.Fatal("encode accepted an empty NGRAN-TNLAssociationToRemoveList")
	}
}
