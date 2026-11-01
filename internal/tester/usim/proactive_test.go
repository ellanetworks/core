// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package usim

import (
	"bytes"
	"encoding/hex"
	"testing"

	"github.com/ellanetworks/core/internal/tester/cat"
)

const serviceCentreTLV = "0607915155000000f0"

func envelopeOf(t *testing.T, ud []byte) []byte {
	t.Helper()

	deliver := concat(mustHex(t, "440b915155001000f07ff6"+"62909251341100"), []byte{byte(len(ud))}, ud)

	return tlv(0xd1, concat(mustHex(t, "82028381"), mustHex(t, serviceCentreTLV), tlv(0x8b, deliver)))
}

func fetchSubmit(t *testing.T, c *Card, sw uint16) (cat.CommandDetails, []byte) {
	t.Helper()

	command, fetchSW := c.Fetch()
	if fetchSW != swOK || sw != uint16(cat.SWProactive)<<8|uint16(len(command)) {
		t.Fatalf("FETCH %x %04x after %04x", command, fetchSW, sw)
	}

	tag, value, _, err := cat.ReadTLV(command)
	if err != nil || tag != cat.TagProactiveCommand {
		t.Fatalf("proactive command %x: %v", command, err)
	}

	objects, err := cat.ReadObjects(value)
	if err != nil {
		t.Fatal(err)
	}

	raw, _ := objects.Get(cat.TagCommandDetails)

	details, err := cat.ParseCommandDetails(raw)
	if err != nil || details.Type != cat.TypeSendShortMessage || details.Qualifier != 0 {
		t.Fatalf("command details %x", raw)
	}

	if identities, _ := objects.Get(cat.TagDeviceIdentities); hex.EncodeToString(identities) != "8183" {
		t.Fatalf("device identities %x", identities)
	}

	if address, _ := objects.Get(cat.TagAddress); hex.EncodeToString(address) != serviceCentreTLV[4:] {
		t.Fatalf("address %x", address)
	}

	submit, _ := objects.Get(cat.TagSMSTPDU)
	header := "41" + "00" + "0b915155001000f0" + "7ff6"

	if len(submit) < 13 || hex.EncodeToString(submit[:12]) != header || int(submit[12]) != len(submit)-13 {
		t.Fatalf("SMS-SUBMIT %x", submit)
	}

	return details, submit[13:]
}

func terminalResponse(details cat.CommandDetails, result ...byte) []byte {
	b := cat.AppendTLV(nil, 0x81, details.Value())
	b = cat.AppendTLV(b, 0x82, []byte{cat.DeviceTerminal, cat.DeviceUICC})

	return cat.AppendTLV(b, 0x83, result)
}

func TestPoRBySubmit(t *testing.T) {
	c := newCard(t, 0)
	k := sja5(t)

	const spi = spiOTA | spiPoRSubmit

	report, sw := c.Envelope(envelopeOf(t, seal(t, k, spi, rfmTAR, 1, mustHex(t, "00a4000c026f07"+"00b0000009"))))
	if report != nil {
		t.Fatalf("SMS-DELIVER-REPORT data %x, want none", report)
	}

	details, ud := fetchSubmit(t, c, sw)

	want := porOf(t, command{spi: spi, cipher: &k, verified: &k}, StatusPoROK, mustHex(t, "029000"+"08"+"0910100900002100"))
	if !bytes.Equal(ud, want) {
		t.Fatalf("PoR %x, want %x", ud, want)
	}

	if sw := c.TerminalResponse(terminalResponse(details, cat.ResultOK)); sw != swOK {
		t.Fatalf("TERMINAL RESPONSE = %04x", sw)
	}

	if _, sw := c.Fetch(); sw == swOK {
		t.Fatal("a second proactive command")
	}
}

func TestPoRTooLargeBySubmit(t *testing.T) {
	k := sja5(t)
	read := mustHex(t, "00a4000c026f6100b0000096")

	c := newCard(t, 60)

	report, sw := c.Envelope(envelopeOf(t, seal(t, k, spiOTA, rfmTAR, 1, read)))
	if want := por(t, k, 1, StatusResponseInSubmit, nil); !bytes.Equal(report, want) {
		t.Fatalf("SMS-DELIVER-REPORT data %x, want %x", report, want)
	}

	first, ud1 := fetchSubmit(t, c, sw)
	if ud1[0] != 0x07 || !bytes.Equal(ud1[1:3], []byte{0x00, 0x03}) || ud1[4] != 2 || ud1[5] != 1 || !bytes.Equal(ud1[6:8], []byte{0x71, 0x00}) {
		t.Fatalf("first part %x", ud1)
	}

	sw = c.TerminalResponse(terminalResponse(first, cat.ResultOK))

	second, ud2 := fetchSubmit(t, c, sw)
	if !bytes.Equal(ud2[:6], []byte{0x05, 0x00, 0x03, ud1[3], 2, 2}) {
		t.Fatalf("second part %x", ud2)
	}

	if sw := c.TerminalResponse(terminalResponse(second, cat.ResultOK)); sw != swOK {
		t.Fatalf("TERMINAL RESPONSE = %04x", sw)
	}

	compact := concat(mustHex(t, "029000"), c.File(FileOPLMNwAcT)[:150])
	if got, want := concat(responseUDH, ud1[8:], ud2[6:]), por(t, k, 1, StatusPoROK, compact); !bytes.Equal(got, want) {
		t.Fatalf("PoR %x, want %x", got, want)
	}

	t.Run("a failed part drops the rest", func(t *testing.T) {
		c := newCard(t, 60)

		_, sw := c.Envelope(envelopeOf(t, seal(t, k, spiOTA, rfmTAR, 1, read)))
		first, _ := fetchSubmit(t, c, sw)

		if sw := c.TerminalResponse(terminalResponse(first, cat.ResultSMSRPError, 0x26)); sw != swOK {
			t.Fatalf("TERMINAL RESPONSE = %04x", sw)
		}
	})

	t.Run("a terminal response of another command", func(t *testing.T) {
		c := newCard(t, 60)

		_, sw := c.Envelope(envelopeOf(t, seal(t, k, spiOTA, rfmTAR, 1, read)))
		first, _ := fetchSubmit(t, c, sw)
		first.Number++

		if sw := c.TerminalResponse(terminalResponse(first, cat.ResultOK)); sw != swIncorrectData {
			t.Fatalf("TERMINAL RESPONSE = %04x", sw)
		}
	})
}
