// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas_test

import (
	"encoding/hex"
	"net/netip"
	"testing"

	"github.com/ellanetworks/core/nas"
)

func mustHex(t *testing.T, s string) []byte {
	t.Helper()

	b, err := hex.DecodeString(s)
	if err != nil {
		t.Fatalf("decode %q: %v", s, err)
	}

	return b
}

func answer(t *testing.T, id uint16, content []byte, dns, ue netip.Addr) []byte {
	t.Helper()

	got := nas.AnswerProtocolOptions([]nas.PCOContainer{{ID: id, Content: content}}, dns, ue)
	if len(got) == 0 {
		return nil
	}

	if len(got) != 1 || got[0].ID != id {
		t.Fatalf("answer: got %+v", got)
	}

	return got[0].Content
}

func TestIPCPConfigureNakSuppliesDNS(t *testing.T) {
	dns := netip.MustParseAddr("8.8.8.8")

	req := mustHex(t, "01000010"+"810600000000"+"830600000000")

	got := answer(t, nas.PCOProtocolIPCP, req, dns, netip.MustParseAddr("10.45.0.2"))

	want := mustHex(t, "03000010"+"810608080808"+"830608080808")
	if hex.EncodeToString(got) != hex.EncodeToString(want) {
		t.Fatalf("got %x, want %x", got, want)
	}
}

func TestIPCPEchoesIdentifier(t *testing.T) {
	req := mustHex(t, "017b000a"+"810600000000")

	got := answer(t, nas.PCOProtocolIPCP, req, netip.MustParseAddr("1.1.1.1"), netip.Addr{})
	if len(got) < 2 || got[1] != 0x7b {
		t.Fatalf("identifier not echoed: %x", got)
	}
}

func TestIPCPAcksAcceptableValues(t *testing.T) {
	dns := netip.MustParseAddr("8.8.8.8")
	req := mustHex(t, "0100000a"+"810608080808")

	got := answer(t, nas.PCOProtocolIPCP, req, dns, netip.Addr{})
	if got[0] != 2 {
		t.Fatalf("want Configure-Ack, got code %d (%x)", got[0], got)
	}

	if hex.EncodeToString(got[4:]) != "810608080808" {
		t.Fatalf("Ack must echo the requested options, got %x", got)
	}
}

func TestIPCPRejectDisplacesNak(t *testing.T) {
	req := mustHex(t, "01000010"+"810600000000"+"820600000000")

	got := answer(t, nas.PCOProtocolIPCP, req, netip.MustParseAddr("8.8.8.8"), netip.Addr{})
	if got[0] != 4 {
		t.Fatalf("want Configure-Reject, got code %d (%x)", got[0], got)
	}

	if hex.EncodeToString(got[4:]) != "820600000000" {
		t.Fatalf("reject must carry only the rejected option, got %x", got)
	}
}

func TestIPCPAddressOptionUsesAllocatedAddress(t *testing.T) {
	req := mustHex(t, "0100000a"+"030600000000")

	got := answer(t, nas.PCOProtocolIPCP, req, netip.Addr{}, netip.MustParseAddr("10.45.0.2"))
	if got[0] != 3 || hex.EncodeToString(got[4:]) != "03060a2d0002" {
		t.Fatalf("want Nak with the allocated address, got %x", got)
	}
}

func TestIPCPWithoutDNSConfiguredRejects(t *testing.T) {
	req := mustHex(t, "0100000a"+"810600000000")

	got := answer(t, nas.PCOProtocolIPCP, req, netip.Addr{}, netip.Addr{})
	if got[0] != 4 {
		t.Fatalf("want Configure-Reject, got code %d (%x)", got[0], got)
	}
}

func TestProtocolAnswersForLCPPAPCHAP(t *testing.T) {
	for _, tc := range []struct {
		name    string
		id      uint16
		request string
		want    string
	}{
		{"LCP rejects options", nas.PCOProtocolLCP, "0100000e" + "010405dc" + "020600000000", "0400000e" + "010405dc" + "020600000000"},
		{"LCP acks an empty request", nas.PCOProtocolLCP, "01000004", "02000004"},
		{"PAP acks", nas.PCOProtocolPAP, "01010008" + "02616200", "02010005" + "00"},
		{"CHAP succeeds", nas.PCOProtocolCHAP, "02050006" + "0000", "03050004"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := answer(t, tc.id, mustHex(t, tc.request), netip.Addr{}, netip.Addr{})
			if hex.EncodeToString(got) != tc.want {
				t.Fatalf("got %x, want %s", got, tc.want)
			}
		})
	}
}

func TestProtocolAnswersIgnoreUnanswerableUnits(t *testing.T) {
	for _, tc := range []struct {
		name    string
		id      uint16
		content string
	}{
		{"truncated packet", nas.PCOProtocolIPCP, "0100"},
		{"length field past the end", nas.PCOProtocolIPCP, "010000ff"},
		{"length field below the header", nas.PCOProtocolIPCP, "01000002"},
		{"option runs past the end", nas.PCOProtocolIPCP, "0100000a" + "81ff00000000"},
		{"not a Configure-Request", nas.PCOProtocolIPCP, "02000004"},
		{"CHAP challenge, not a response", nas.PCOProtocolCHAP, "01000004"},
		{"unsupported protocol identifier", 0x8281, "01000004"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := answer(t, tc.id, mustHex(t, tc.content), netip.MustParseAddr("8.8.8.8"), netip.Addr{}); got != nil {
				t.Fatalf("want no answer, got %x", got)
			}
		})
	}
}

func TestAnswerProtocolOptionsPreservesRequestOrder(t *testing.T) {
	got := nas.AnswerProtocolOptions([]nas.PCOContainer{
		{ID: nas.PCOProtocolPAP, Content: mustHex(t, "01010008"+"02616200")},
		{ID: nas.PCOProtocolIPCP, Content: mustHex(t, "0100000a"+"810600000000")},
	}, netip.MustParseAddr("8.8.8.8"), netip.Addr{})

	if len(got) != 2 || got[0].ID != nas.PCOProtocolPAP || got[1].ID != nas.PCOProtocolIPCP {
		t.Fatalf("got %+v", got)
	}
}

func TestProtocolOptionsSelectsUplinkProtocolUnits(t *testing.T) {
	opts := nas.ProtocolConfigurationOptions{
		Direction: nas.PCOMSToNetwork,
		Containers: []nas.PCOContainer{
			{ID: nas.PCOProtocolIPCP, Content: []byte{1, 0, 0, 4}},
			{ID: nas.PCOContainerDNSServerIPv4Address},
			{ID: nas.PCOProtocolCHAP, Content: []byte{2, 0, 0, 4}},
		},
	}

	got := opts.ProtocolOptions()
	if len(got) != 2 || got[0].ID != nas.PCOProtocolIPCP || got[1].ID != nas.PCOProtocolCHAP {
		t.Fatalf("got %+v", got)
	}

	downlink := opts
	downlink.Direction = nas.PCONetworkToMS

	if downlink.ProtocolOptions() != nil {
		t.Fatal("a downlink element carries no request to answer")
	}
}

func TestPrependProtocolOptionsPutsProtocolUnitsFirst(t *testing.T) {
	opts := nas.NewProtocolConfigurationOptions([][]byte{{8, 8, 8, 8}}, 1400)
	opts.PrependProtocolOptions([]nas.PCOContainer{{ID: nas.PCOProtocolIPCP, Content: []byte{3, 0, 0, 4}}})

	if opts.Containers[0].ID != nas.PCOProtocolIPCP {
		t.Fatalf("got %+v", opts.Containers)
	}

	b, err := opts.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}

	if hex.EncodeToString(b[1:4]) != "802104" {
		t.Fatalf("IPCP is not the first unit on the wire: %x", b)
	}
}
