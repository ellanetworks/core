// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"bytes"
	"encoding/hex"
	"testing"

	"github.com/ellanetworks/core/per"
	"github.com/ellanetworks/core/per/ap"
)

// Golden UPTransportLayerInformation carrying a GTP tunnel to 192.168.1.1
// TEID 1. choice index
// (1 bit) + SEQUENCE extension bit + absent iE-Extensions + BIT STRING
// extension bit + 8-bit length (32-1) pads to 0x01 0xf0, then the four address
// octets and four TEID octets.
const goldenUPTransportLayerInformation = "01f0c0a8010100000001"

func TestUPTransportLayerInformationGolden(t *testing.T) {
	in := UPTransportLayerInformation{
		GTPTunnel: &GTPTunnel{
			TransportLayerAddress: tlaOf(192, 168, 1, 1),
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

	if out.GTPTunnel.TransportLayerAddress != in.GTPTunnel.TransportLayerAddress ||
		out.GTPTunnel.GTPTEID != in.GTPTunnel.GTPTEID {
		t.Fatalf("round trip: %+v, want %+v", out.GTPTunnel, in.GTPTunnel)
	}
}

func TestUPTransportLayerInformationChoiceExtension(t *testing.T) {
	raw := choiceExtensionValue(t, 2, 1, IDAMFUENGAPID)

	got, err := unmarshalPERValue[UPTransportLayerInformation](raw)
	if err != nil {
		t.Fatalf("decode is lenient, got %v", err)
	}

	if got.GTPTunnel != nil || got.ChoiceExtension == nil || got.ChoiceExtension.ID != IDAMFUENGAPID {
		t.Fatalf("decoded %+v, want the unknown alternative kept", got)
	}

	if !bytes.Equal(ieRaw(t, &got), raw) {
		t.Fatal("unknown alternative did not re-encode byte-exact")
	}

	d := ap.Diagnose[ProtocolIEID](&got)
	if len(d.IEs) != 1 || d.IEs[0].ID != IDAMFUENGAPID || d.IEs[0].Criticality != CriticalityReject {
		t.Fatalf("diagnostics = %+v, want the alternative not comprehended on its own criticality", d.IEs)
	}
}
