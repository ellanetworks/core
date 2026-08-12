// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"bytes"
	"encoding/hex"
	"errors"
	"testing"

	"github.com/ellanetworks/core/per"
)

// Golden UPTransportLayerInformation carrying a GTP tunnel to 192.168.1.1
// TEID 1. choice index
// (1 bit) + SEQUENCE extension bit + absent iE-Extensions + BIT STRING
// extension bit + 8-bit length (32-1) pads to 0x01 0xf0, then the four address
// octets and four TEID octets.
const goldenUPTransportLayerInformation = "01f0c0a8010100000001"

func TestUPTransportLayerInformationGolden(t *testing.T) {
	in := UPTransportLayerInformation{
		GTPTunnel: GTPTunnel{
			TransportLayerAddress: TransportLayerAddress{192, 168, 1, 1},
			GTPTEID:               1,
		},
	}

	w := per.NewWriter()
	if err := in.MarshalPER(w, per.Aligned); err != nil {
		t.Fatal(err)
	}

	if got := hex.EncodeToString(perBytes(w)); got != goldenUPTransportLayerInformation {
		t.Fatalf("encoded %s, want %s", got, goldenUPTransportLayerInformation)
	}

	out, err := unmarshalPERValue[UPTransportLayerInformation](perBytes(w))
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(out.GTPTunnel.TransportLayerAddress, in.GTPTunnel.TransportLayerAddress) ||
		out.GTPTunnel.GTPTEID != in.GTPTunnel.GTPTEID {
		t.Fatalf("round trip: %+v, want %+v", out.GTPTunnel, in.GTPTunnel)
	}
}

// The CHOICE is closed by choice-Extensions rather than an extension marker, so
// an unmodeled alternative is §10.3.1 case 6, handled on the carrying IE's
// criticality.
func TestUPTransportLayerInformationChoiceExtension(t *testing.T) {
	raw := choiceExtensionValue(t, upTransportLayerInformationAlternatives,
		upTransportLayerInformationChoiceExtensions, IDAMFUENGAPID)

	_, err := unmarshalPERValue[UPTransportLayerInformation](raw)
	if err == nil {
		t.Fatal("decoded an unmodeled choice-Extensions alternative")
	}

	// A transfer-syntax error here would abort the whole message instead of
	// leaving criticality to decide, so the kind matters, not just that it
	// failed.
	if !errors.Is(err, errNotComprehended) {
		t.Fatalf("err = %v, want errNotComprehended so criticality decides (§10.3.1 case 6)", err)
	}
}
