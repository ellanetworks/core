// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package usim

import (
	"bytes"
	"errors"
	"testing"
)

func newCardMSL(t *testing.T, msl byte) *Card {
	t.Helper()

	c, err := New(Config{TAR: rfmTAR, MinimumSecurityLevel: msl, Keysets: []Keyset{sja5(t), pysimAES(t)}, IMSI: "001019000001200"})
	if err != nil {
		t.Fatal(err)
	}

	return c
}

func porOf(t *testing.T, cmd command, status Status, data []byte) []byte {
	t.Helper()

	p, err := response{tar: rfmTAR, counter: 1, status: status, data: data}.pack(cmd)
	if err != nil {
		t.Fatal(err)
	}

	return p
}

func TestMinimumSecurityLevel(t *testing.T) {
	k := sja5(t)
	update := mustHex(t, "00a4000c026f07"+"00d6000009"+"089910100900002100")

	t.Run("unsecured command is discarded", func(t *testing.T) {
		c := newCardMSL(t, 0x16)
		imsi := c.File(FileIMSI)

		got, dl := c.DataDownload(download(seal(t, k, 0x0001, rfmTAR, 0, update)))
		if got != nil || dl.PoR || !errors.Is(dl.Err, errUnauthenticated) || dl.Executed != 0 {
			t.Fatalf("PoR %x (%+v), want the command discarded", got, dl)
		}

		if !bytes.Equal(c.File(FileIMSI), imsi) {
			t.Fatalf("EF.IMSI rewritten to %x", c.File(FileIMSI))
		}
	})

	t.Run("authenticated but unciphered command", func(t *testing.T) {
		c := newCardMSL(t, 0x16)

		const spi uint16 = 0x1209

		got, dl := c.DataDownload(download(seal(t, k, spi, rfmTAR, 1, update)))
		if want := porOf(t, command{spi: spi, verified: &k}, StatusInsufficient, nil); !bytes.Equal(got, want) || dl.Executed != 0 {
			t.Fatalf("PoR %x (%+v), want insufficient security level", got, dl)
		}

		if _, dl := c.DataDownload(download(seal(t, k, spiOTA, rfmTAR, 1, mustHex(t, "00a4000c026f07")))); dl.Status != StatusPoROK {
			t.Fatalf("the rejected command updated the counter: %+v", dl)
		}
	})

	t.Run("sufficient", func(t *testing.T) {
		c := newCardMSL(t, 0x16)

		if _, dl := c.DataDownload(download(seal(t, k, spiOTA, rfmTAR, 1, update))); dl.Status != StatusPoROK || dl.Executed != 2 {
			t.Fatalf("%+v", dl)
		}
	})

	t.Run("invalid", func(t *testing.T) {
		for _, msl := range []byte{0x03, 0x20} {
			if _, err := New(Config{TAR: rfmTAR, MinimumSecurityLevel: msl, Keysets: []Keyset{k}, IMSI: "001019000001200"}); err == nil {
				t.Errorf("minimum SPI1 %#02x accepted", msl)
			}
		}
	})
}

func TestAESCounter(t *testing.T) {
	k := pysimAES(t)
	c := newCardMSL(t, 0)

	const spi uint16 = 0x0e19

	got, dl := c.DataDownload(download(seal(t, k, spi, rfmTAR, 1, mustHex(t, "00a4000c026f07"))))
	if want := porOf(t, command{spi: spi, cipher: &k, verified: &k}, StatusInsufficient, nil); !bytes.Equal(got, want) || dl.Executed != 0 {
		t.Fatalf("PoR %x (%+v), want insufficient security level", got, dl)
	}

	if _, dl := c.DataDownload(download(seal(t, sja5(t), 0x0619, rfmTAR, 0, mustHex(t, "00a4000c026f07")))); dl.Status != StatusPoROK {
		t.Fatalf("triple DES without a counter: %+v", dl)
	}
}

func TestKeyVersionsDiffer(t *testing.T) {
	kc, k := sja5(t), pysimAES(t)

	got, dl := newCardMSL(t, 0x16).DataDownload(download(sealWith(t, kc, k, spiOTA, rfmTAR, 1, mustHex(t, "00a4000c026f07"))))
	if want := porOf(t, command{spi: spiOTA &^ (spiPoRIntegrity | spiPoRCiphering)}, StatusUnidentified, nil); !bytes.Equal(got, want) || dl.Executed != 0 {
		t.Fatalf("PoR %x (%+v), want an unsecured unidentified security error", got, dl)
	}

	ud := sealWith(t, kc, k, spiOTA, rfmTAR, 1, mustHex(t, "00a4000c026f07"))
	ud[len(ud)-1] ^= 0x01

	if got, dl := newCardMSL(t, 0x16).DataDownload(download(ud)); got != nil || dl.Err == nil {
		t.Fatalf("PoR %x (%+v), want none", got, dl)
	}
}

func TestEmptyCommandString(t *testing.T) {
	k := sja5(t)

	got, dl := newCard(t, 0).DataDownload(download(seal(t, k, spiOTA, rfmTAR, 1, nil)))
	if want := por(t, k, 1, StatusPoROK, mustHex(t, "009000")); !bytes.Equal(got, want) {
		t.Fatalf("PoR %x (%+v), want no command executed", got, dl)
	}
}
