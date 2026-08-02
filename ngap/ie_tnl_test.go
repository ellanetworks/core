// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"bytes"
	"strings"
	"testing"

	"github.com/ellanetworks/core/per"
)

// free5gc/ngap v1.1.3 does not model NGRAN-TNLAssociationToRemoveList, so this
// IE has no second implementation to pin bytes against. Round-tripping is what
// is available: it still catches a codec that reads back something other than
// what it wrote.
func TestNGRANTNLAssociationToRemoveListRoundTrip(t *testing.T) {
	in := NGRANTNLAssociationToRemoveList{
		{
			TNLAssociationTransportLayerAddress:    CPTransportLayerInformation{EndpointIPAddress: TransportLayerAddress{10, 3, 0, 3}},
			TNLAssociationTransportLayerAddressAMF: &CPTransportLayerInformation{EndpointIPAddress: TransportLayerAddress{10, 3, 0, 2}},
		},
		// The AMF-side address is optional; an item may name only its own.
		{TNLAssociationTransportLayerAddress: CPTransportLayerInformation{
			EndpointIPAddress: TransportLayerAddress{
				0x20, 0x01, 0x0d, 0xb8, 0, 3, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0x11,
			},
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
		if !bytes.Equal(out[i].TNLAssociationTransportLayerAddress.EndpointIPAddress,
			in[i].TNLAssociationTransportLayerAddress.EndpointIPAddress) {
			t.Errorf("item %d address = %v, want %v",
				i, out[i].TNLAssociationTransportLayerAddress.EndpointIPAddress,
				in[i].TNLAssociationTransportLayerAddress.EndpointIPAddress)
		}

		gotAMF, wantAMF := out[i].TNLAssociationTransportLayerAddressAMF, in[i].TNLAssociationTransportLayerAddressAMF
		if (gotAMF == nil) != (wantAMF == nil) {
			t.Fatalf("item %d AMF address presence = %v, want %v", i, gotAMF, wantAMF)
		}

		if wantAMF != nil && !bytes.Equal(gotAMF.EndpointIPAddress, wantAMF.EndpointIPAddress) {
			t.Errorf("item %d AMF address = %v, want %v", i, gotAMF.EndpointIPAddress, wantAMF.EndpointIPAddress)
		}
	}
}

// TS 38.413 §9.3.2.6 closes CPTransportLayerInformation with a choice-Extensions
// alternative rather than an extension marker, so selecting it must be an
// explicit error and not a zero address that reads as one the peer chose.
func TestCPTransportLayerInformationChoiceExtensionIsRejected(t *testing.T) {
	w := per.NewWriter()
	if err := per.EncodeConstrainedWholeNumber(w, per.Aligned, 0,
		cpTransportLayerInformationAlternatives-1, cpTransportLayerInformationChoiceExtensions); err != nil {
		t.Fatal(err)
	}

	got, err := unmarshalPERValue[CPTransportLayerInformation](perBytes(w))
	if err == nil {
		t.Fatalf("decoded choice-Extensions as %+v, want an error", got)
	}

	if !strings.Contains(err.Error(), "unsupported CPTransportLayerInformation alternative") {
		t.Errorf("error = %q, want it to name the unsupported alternative", err)
	}

	if got.EndpointIPAddress != nil {
		t.Errorf("EndpointIPAddress = %v, want it left untouched", got.EndpointIPAddress)
	}
}

// The list is SIZE(1..maxnoofTNLAssociations), so an empty one cannot be encoded.
func TestNGRANTNLAssociationToRemoveListRejectsEmpty(t *testing.T) {
	w := per.NewWriter()
	if err := (NGRANTNLAssociationToRemoveList{}).MarshalPER(w, per.Aligned); err == nil {
		t.Fatal("encode accepted an empty NGRAN-TNLAssociationToRemoveList")
	}
}
