// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"encoding/hex"
	"testing"

	naslib "github.com/ellanetworks/core/nas"
)

func decoded(t *testing.T, dir naslib.PCODirection, id uint16, content string) PCOContainer {
	t.Helper()

	b, err := hex.DecodeString(content)
	if err != nil {
		t.Fatal(err)
	}

	out := ExtendedPCO(&naslib.ProtocolConfigurationOptions{
		Direction:  dir,
		Containers: []naslib.PCOContainer{{ID: id, Content: b}},
	})

	if out == nil || len(out.Containers) != 1 {
		t.Fatalf("got %+v", out)
	}

	return out.Containers[0]
}

func TestExtendedPCORendersTheDownlinkIPCPAnswer(t *testing.T) {
	c := decoded(t, naslib.PCONetworkToMS, naslib.PCOProtocolIPCP,
		"03000010"+"810608080808"+"830608080808")

	if c.Name != "IPCP" {
		t.Fatalf("name = %q", c.Name)
	}

	want := "Configure-Nak: Primary DNS Server Address 8.8.8.8, Secondary DNS Server Address 8.8.8.8"
	if c.Value != want {
		t.Fatalf("value = %q, want %q", c.Value, want)
	}

	if c.Hex != "" {
		t.Fatalf("a decoded unit carries no hex fallback, got %q", c.Hex)
	}
}

func TestExtendedPCORendersTheUplinkIPCPRequest(t *testing.T) {
	c := decoded(t, naslib.PCOMSToNetwork, naslib.PCOProtocolIPCP,
		"01000010"+"810600000000"+"830600000000")

	want := "Configure-Request: Primary DNS Server Address 0.0.0.0, Secondary DNS Server Address 0.0.0.0"
	if c.Value != want {
		t.Fatalf("value = %q, want %q", c.Value, want)
	}
}

func TestExtendedPCODoesNotRenderPAPCredentials(t *testing.T) {
	c := decoded(t, naslib.PCOMSToNetwork, naslib.PCOProtocolPAP,
		"0101000c"+"02"+"6162"+"04"+"70617373")

	if c.Value != "Authenticate-Request" {
		t.Fatalf("value = %q", c.Value)
	}

	if c.Hex != "" {
		t.Fatalf("the peer-ID and password must not reach the event log, got %q", c.Hex)
	}
}
