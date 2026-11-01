// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package usim

import (
	"bytes"
	"encoding/hex"
	"testing"

	"github.com/ellanetworks/core/internal/tester/tpdu"
)

// spiOTA is the SPI of the commands of an OTA platform such as Ella SIM OTA: ciphered, with a cryptographic checksum,
// of a counter that must be higher, asking for a PoR always, ciphered with a cryptographic checksum, in the
// SMS-DELIVER-REPORT.
const spiOTA uint16 = 0x1619

var rfmTAR = [3]byte{0xb0, 0x00, 0x01}

// seal is the user data of a Command Packet of the Sending Entity, the CPI included (TS 102 225 clause 5.1, TS 31.115
// clause 4.2).
func seal(t *testing.T, k Keyset, spi uint16, tar [3]byte, counter uint64, data []byte) []byte {
	t.Helper()

	ccLen := 0
	if spi&spiIntegrity == spiIntegrityCC {
		ccLen = k.ccLength()
	}

	padding := 0

	if spi&spiCiphering != 0 {
		b, err := k.block(k.KIc)
		if err != nil {
			t.Fatal(err)
		}

		if r := (counterLength + 1 + ccLen + len(data)) % b.BlockSize(); r != 0 {
			padding = b.BlockSize() - r
		}
	}

	data = append(append([]byte{}, data...), make([]byte, padding)...)
	chl := 13 + ccLen
	cpl := 1 + chl + len(data)

	keyID := func(used bool) byte {
		if !used {
			return 0
		}

		return k.Version<<4 | byte(k.Algorithm)
	}

	header := []byte{
		byte(cpl >> 8), byte(cpl), byte(chl), byte(spi >> 8), byte(spi), keyID(spi&spiCiphering != 0),
		keyID(ccLen > 0), tar[0], tar[1], tar[2],
	}

	cntr := []byte{byte(counter >> 32), byte(counter >> 24), byte(counter >> 16), byte(counter >> 8), byte(counter), byte(padding)}

	var cc []byte

	if ccLen > 0 {
		var err error

		cc, err = k.checksum(concat(header, cntr, data))
		if err != nil {
			t.Fatal(err)
		}
	}

	secured := concat(cntr, cc, data)

	if spi&spiCiphering != 0 {
		var err error

		secured, err = k.crypt(secured, true)
		if err != nil {
			t.Fatal(err)
		}
	}

	return concat(commandUDH, header, secured)
}

// TestSeal checks the tests' Sending Entity against a command of a real card's exchanges.
func TestSeal(t *testing.T) {
	got := seal(t, sja5(t), spiOTA, [3]byte{0xb0, 0x00, 0x10}, 100, mustHex(t, "a0a40000027f20a0c0000016"))
	if want := "027000" + "00281516191515b000108fb60fad7125e0ee4811b11427719bef3eb8b1ef6ecb1b4f326272f85256243e"; hex.EncodeToString(got) != want {
		t.Fatalf("command %x, want %s", got, want)
	}
}

func newCard(t *testing.T, entries int) *Card {
	t.Helper()

	c, err := New(Config{TAR: rfmTAR, Keysets: []Keyset{sja5(t), pysimAES(t)}, IMSI: "001019000001200", OPLMNwAcTEntries: entries})
	if err != nil {
		t.Fatal(err)
	}

	return c
}

// download is the SMS-DELIVER of a (U)SIM data download of user data ud.
func download(ud []byte) tpdu.Deliver {
	return tpdu.Deliver{UDHI: true, OriginatingAddress: "+15550001000", PID: tpdu.PIDSIMDataDownload, DCS: 0xf6, UDL: len(ud), UD: ud}
}

// por is the PoR the card answers a command of SPI spiOTA with.
func por(t *testing.T, k Keyset, counter uint64, status Status, data []byte) []byte {
	t.Helper()

	p, err := response{tar: rfmTAR, counter: counter, status: status, data: data}.pack(command{spi: spiOTA, keyset: k})
	if err != nil {
		t.Fatal(err)
	}

	return p
}

// TestRemoteFileManagement runs the command strings of Ella SIM OTA on EF.OPLMNwAcT (TS 102 226 clause 5.1).
func TestRemoteFileManagement(t *testing.T) {
	c := newCard(t, 40)
	k := sja5(t)

	steps := []struct {
		name     string
		commands string
		compact  string
	}{
		{
			name:     "select for the FCP, get response",
			commands: "00a40004026f6100c0000000",
			compact:  "029000" + "6217" + "82024121" + "83026f61" + "8a0105" + "8b036f0601" + "800200c8" + "880188",
		},
		{
			name:     "update slot 2, read it back",
			commands: "00a4000c026f6100d600050500f1104000" + "00b0000505",
			compact:  "039000" + "00f1104000",
		},
		{
			name:     "read past the end of the file",
			commands: "00a4000c026f6100b000c80a",
			compact:  "026b00",
		},
		{
			name:     "read by SFI to the end, from the last entry",
			commands: "00b091c300",
			compact:  "019000" + "ffffff0000",
		},
		{
			name:     "an unknown file stops the command string",
			commands: "00a4000c026f62" + "00b0000005",
			compact:  "016a82",
		},
		{
			name:     "a read without a selected EF",
			commands: "00b0000005",
			compact:  "016986",
		},
		{
			name:     "an unsupported class",
			commands: "a0a4000c026f61",
			compact:  "016e00",
		},
	}

	for i, s := range steps {
		counter := uint64(i + 1)

		got, dl := c.DataDownload(download(seal(t, k, spiOTA, rfmTAR, counter, mustHex(t, s.commands))))
		if want := por(t, k, counter, StatusPoROK, mustHex(t, s.compact)); !bytes.Equal(got, want) || dl.Err != nil {
			t.Fatalf("%s: PoR %x (%+v), want compact response %s", s.name, got, dl, s.compact)
		}
	}

	if got := c.File(FileOPLMNwAcT)[5:10]; hex.EncodeToString(got) != "00f1104000" {
		t.Fatalf("slot 2 = %x", got)
	}
}

// TestSecurityErrors checks the card's answers to commands it cannot process (TS 102 225 clause 4.1).
func TestSecurityErrors(t *testing.T) {
	k := sja5(t)

	t.Run("unauthenticated: no PoR", func(t *testing.T) {
		c := newCard(t, 0)
		ud := seal(t, k, spiOTA, rfmTAR, 1, mustHex(t, "00a4000c026f61"))
		ud[len(ud)-1] ^= 0x01

		if got, dl := c.DataDownload(download(ud)); got != nil || dl.PoR || dl.Err == nil {
			t.Fatalf("PoR %x (%+v), want none", got, dl)
		}
	})

	t.Run("counter low, then accepted", func(t *testing.T) {
		c := newCard(t, 0)
		if _, dl := c.DataDownload(download(seal(t, k, spiOTA, rfmTAR, 5, mustHex(t, "00a4000c026f61")))); dl.Status != StatusPoROK {
			t.Fatalf("first command: %+v", dl)
		}

		got, dl := c.DataDownload(download(seal(t, k, spiOTA, rfmTAR, 5, mustHex(t, "00a4000c026f61"))))
		if want := por(t, k, 5, StatusCNTRLow, nil); !bytes.Equal(got, want) || dl.Executed != 0 {
			t.Fatalf("replayed command: PoR %x (%+v), want CNTR low", got, dl)
		}
	})

	t.Run("TAR unknown", func(t *testing.T) {
		c := newCard(t, 0)
		other := [3]byte{0xb0, 0x00, 0x10}

		got, dl := c.DataDownload(download(seal(t, k, spiOTA, other, 1, mustHex(t, "00a4000c026f61"))))

		want, err := response{tar: other, counter: 1, status: StatusTARUnknown}.pack(command{spi: spiOTA, keyset: k})
		if err != nil {
			t.Fatal(err)
		}

		if !bytes.Equal(got, want) || dl.Executed != 0 {
			t.Fatalf("PoR %x (%+v), want TAR unknown", got, dl)
		}
	})

	t.Run("PoR on error only", func(t *testing.T) {
		c := newCard(t, 0)

		if got, dl := c.DataDownload(download(seal(t, k, 0x161a, rfmTAR, 1, mustHex(t, "00a4000c026f61")))); got != nil || dl.Status != StatusPoROK {
			t.Fatalf("PoR %x (%+v), want none for a success", got, dl)
		}
	})

	t.Run("PoR too large for the SMS-DELIVER-REPORT", func(t *testing.T) {
		c := newCard(t, 60)

		// A read of 150 octets, whose PoR exceeds the 159 octets of TP-UD.
		got, dl := c.DataDownload(download(seal(t, k, spiOTA, rfmTAR, 1, mustHex(t, "00a4000c026f6100b0000096"))))
		if want := por(t, k, 1, StatusResponseInSubmit, nil); !bytes.Equal(got, want) || dl.Executed != 2 {
			t.Fatalf("PoR %x (%+v), want a response by SMS-SUBMIT", got, dl)
		}
	})
}

// TestEnvelope checks the ENVELOPE (SMS-PP DOWNLOAD) of an AES command (TS 31.111 clause 7.1.1.2).
func TestEnvelope(t *testing.T) {
	c := newCard(t, 0)
	k := pysimAES(t)

	ud := seal(t, k, spiOTA, rfmTAR, 1, mustHex(t, "00a4000c026f07"+"00b0000009"))

	// An SMS-DELIVER from +15550001000, of TP-PID '7F', TP-DCS 'F6', with UDHI.
	deliver := concat(mustHex(t, "440b915155001000f07ff6"+"62909251341100"), []byte{byte(len(ud))}, ud)

	envelope := tlv(0xd1, concat(mustHex(t, "82028381"), mustHex(t, "0607915155000000f0"), tlv(0x8b, deliver)))

	got, sw := c.Envelope(envelope)

	// EF.IMSI of 001019000001200: length 8, then '0' with odd parity and type IMSI, the digits, no unused byte.
	if want := por(t, k, 1, StatusPoROK, mustHex(t, "029000"+"08"+"0910100900002100")); sw != swOK || !bytes.Equal(got, want) {
		t.Fatalf("ENVELOPE = %x %04x, want %x 9000", got, sw, want)
	}

	if _, sw := c.Envelope(mustHex(t, "d30282028381")); sw != swInstructionNotFound {
		t.Fatalf("another ENVELOPE = %04x", sw)
	}
}

// tlv is a data object of a one-octet tag, its length coded as TS 101 220 clause 7.1.2 does.
func tlv(tag byte, value []byte) []byte {
	if len(value) > 0x7f {
		return concat([]byte{tag, 0x81, byte(len(value))}, value)
	}

	return concat([]byte{tag, byte(len(value))}, value)
}

func TestNewRejects(t *testing.T) {
	for name, cfg := range map[string]Config{
		"a TAR of the expanded format": {TAR: [3]byte{0xb0, 0x01, 0x40}, Keysets: []Keyset{sja5(t)}, IMSI: "001019000001200"},
		"fewer than 8 entries":         {TAR: rfmTAR, Keysets: []Keyset{sja5(t)}, IMSI: "001019000001200", OPLMNwAcTEntries: 7},
		"no key set":                   {TAR: rfmTAR, IMSI: "001019000001200"},
		"a short IMSI":                 {TAR: rfmTAR, Keysets: []Keyset{sja5(t)}, IMSI: "00101"},
	} {
		if _, err := New(cfg); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}
