// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

// Package usim is a USIM for the tester's UE that runs SIM OTA over SMS: it takes the ENVELOPE (SMS-PP DOWNLOAD) of a
// (U)SIM data download (3GPP TS 31.111 clause 7.1.1), unpacks its Command Packet, secured with SCP80 (ETSI TS 102 225,
// 3GPP TS 31.115), for its ADF Remote File Management application, which runs the command string on the files of
// ADF.USIM (ETSI TS 102 226, 3GPP TS 31.116, ETSI TS 102 221), and answers with the Response Packet, the PoR, that the
// ME puts in the SMS-DELIVER-REPORT.
package usim

import (
	"bytes"
	"errors"
	"fmt"
	"sync"

	"github.com/ellanetworks/core/internal/tester/tpdu"
	"github.com/ellanetworks/core/nas"
)

// The files of ADF.USIM that the card has (TS 31.102 clauses 4.2.2, 4.2.53 and 4.2.55).
const (
	FileIMSI      uint16 = 0x6f07
	FileOPLMNwAcT uint16 = 0x6f61
	fileARR       uint16 = 0x6f06
)

// minOPLMNwAcTEntries is the fewest entries of EF.OPLMNwAcT, of 5 bytes each (TS 31.102 clause 4.2.53).
const minOPLMNwAcTEntries = 8

// unusedPLMNEntry is an entry of a PLMN selector that is not used: no PLMN (TS 31.102 clause 4.2.53, TS 24.008) and no
// access technology selected (TS 31.102 clause 4.2.5).
var unusedPLMNEntry = []byte{0xff, 0xff, 0xff, 0x00, 0x00}

// The tags of the ENVELOPE (SMS-PP DOWNLOAD) (TS 31.111 clause 7.1.1.2, TS 101 220 clause 7.2). The card reads the
// COMPREHENSION-TLV tags whatever their comprehension required flag, bit 8 (TS 102 223 clause 6.10).
const (
	tagSMSPPDownload    = 0xd1
	tagDeviceIdentities = 0x02
	tagSMSTPDU          = 0x0b
)

// The device identities of the ENVELOPE (SMS-PP DOWNLOAD): from the network to the UICC (TS 102 223 clause 8.7).
const (
	deviceUICC    = 0x81
	deviceNetwork = 0x83
)

// The status words of the ENVELOPE (TS 102 221 clause 10.2.1).
const (
	swOK                  = 0x9000
	swIncorrectData       = 0x6a80
	swInstructionNotFound = 0x6d00
)

// Config is a card's.
type Config struct {
	// TAR is that of the card's ADF RFM application linked to the USIM, of the compact data format: 'B0 00 01' or
	// 'B0 00 20' to 'B0 01 1F' (TS 101 220 annex D).
	TAR     [3]byte
	Keysets []Keyset
	// IMSI is the IMSI of EF.IMSI.
	IMSI string
	// OPLMNwAcTEntries is the number of entries of EF.OPLMNwAcT, all unused at first; 0 is the fewest, 8.
	OPLMNwAcTEntries int
	// OnDownload, when set, is told what the card did with each (U)SIM data download.
	OnDownload func(Download)
}

// Card is a USIM. It is safe for concurrent use.
type Card struct {
	tar        [3]byte
	keysets    map[uint8]Keyset
	onDownload func(Download)

	mu sync.Mutex
	// counters are the counters of the key sets, by their versions: the last one each accepted.
	counters map[uint8]uint64
	files    *fileSystem
}

// New makes a card.
func New(cfg Config) (*Card, error) {
	if !adfRFMCompact(cfg.TAR) {
		return nil, fmt.Errorf("usim: TAR %x is not of an ADF RFM application of the compact data format", cfg.TAR)
	}

	if len(cfg.Keysets) == 0 {
		return nil, errors.New("usim: no key set")
	}

	keysets := map[uint8]Keyset{}

	for _, k := range cfg.Keysets {
		if err := k.validate(); err != nil {
			return nil, fmt.Errorf("usim: key set %d: %w", k.Version, err)
		}

		keysets[k.Version] = k
	}

	imsi, err := efIMSI(cfg.IMSI)
	if err != nil {
		return nil, fmt.Errorf("usim: %w", err)
	}

	entries := cfg.OPLMNwAcTEntries
	if entries == 0 {
		entries = minOPLMNwAcTEntries
	}

	// The file's size is coded on two bytes, of which READ BINARY and UPDATE BINARY address the first 0x7FFF.
	if entries < minOPLMNwAcTEntries || 5*entries > 0x7fff {
		return nil, fmt.Errorf("usim: EF.OPLMNwAcT of %d entries, want %d to %d", entries, minOPLMNwAcTEntries, 0x7fff/5)
	}

	oplmn := bytes.Repeat(unusedPLMNEntry, entries)

	return &Card{
		tar: cfg.TAR, keysets: keysets, onDownload: cfg.OnDownload, counters: map[uint8]uint64{},
		files: newFileSystem(
			// SFIs of TS 31.102 clauses 4.2.2 and 4.2.53.
			&ef{fid: FileIMSI, sfi: 0x07, data: imsi},
			&ef{fid: FileOPLMNwAcT, sfi: 0x11, data: oplmn},
		),
	}, nil
}

// adfRFMCompact reports whether tar is that of an ADF RFM application of the compact data format (TS 101 220 annex D).
func adfRFMCompact(tar [3]byte) bool {
	t := uint32(tar[0])<<16 | uint32(tar[1])<<8 | uint32(tar[2])
	return t == 0xb00001 || t >= 0xb00020 && t <= 0xb0011f
}

// efIMSI is the contents of EF.IMSI: the length of the IMSI, then the IMSI as a mobile identity of TS 24.008, unused
// bytes 'FF' (TS 31.102 clause 4.2.2).
func efIMSI(imsi string) ([]byte, error) {
	const size, typeIMSI = 9, 0x01

	if n := len(imsi); n < 6 || n > 15 {
		return nil, fmt.Errorf("IMSI of %d digits", n)
	}

	identity, err := nas.EncodeBCDIdentity(imsi, typeIMSI)
	if err != nil {
		return nil, fmt.Errorf("IMSI: %w", err)
	}

	ef := append([]byte{byte(len(identity))}, identity...)

	return append(ef, bytes.Repeat([]byte{0xff}, size-len(ef))...), nil
}

// File is the contents of a file of ADF.USIM.
func (c *Card) File(fid uint16) []byte {
	c.mu.Lock()
	defer c.mu.Unlock()

	if f := c.files.byFID(fid); f != nil {
		return append([]byte{}, f.data...)
	}

	return nil
}

// Envelope runs the ENVELOPE command with data and returns its response data and status word (TS 102 221 clause 11.2,
// TS 31.111 clause 7). The card takes the SMS-PP DOWNLOAD only.
func (c *Card) Envelope(data []byte) ([]byte, uint16) {
	tag, value, _, err := readTLV(data)
	if err != nil {
		return nil, swIncorrectData
	}

	if tag != tagSMSPPDownload {
		return nil, swInstructionNotFound
	}

	deliver, err := smsPPDownload(value)
	if err != nil {
		return nil, swIncorrectData
	}

	por, _ := c.DataDownload(deliver)

	return por, swOK
}

// smsPPDownload is the SMS-DELIVER of the value of an ENVELOPE (SMS-PP DOWNLOAD), from the network to the UICC (TS
// 31.111 clause 7.1.1.2).
func smsPPDownload(value []byte) (tpdu.Deliver, error) {
	var (
		identities, smsTPDU []byte
		seen                = map[byte]bool{}
	)

	for len(value) > 0 {
		tag, v, rest, err := readTLV(value)
		if err != nil {
			return tpdu.Deliver{}, err
		}

		value = rest

		// The first instance of a tag is the one read (TS 102 223 clause 6.10.5).
		if seen[tag&0x7f] {
			continue
		}

		seen[tag&0x7f] = true

		switch tag & 0x7f {
		case tagDeviceIdentities:
			identities = v
		case tagSMSTPDU:
			smsTPDU = v
		}
	}

	if !bytes.Equal(identities, []byte{deviceNetwork, deviceUICC}) {
		return tpdu.Deliver{}, fmt.Errorf("device identities %x, want from the network to the UICC", identities)
	}

	if smsTPDU == nil {
		return tpdu.Deliver{}, errors.New("no SMS TPDU")
	}

	return tpdu.DecodeDeliver(smsTPDU)
}

// readTLV reads the data object that starts b, of a one-octet tag and a length coded as TS 101 220 clause 7.1.2 does,
// and returns its tag, its value and what follows it.
func readTLV(b []byte) (tag byte, value, rest []byte, err error) {
	if len(b) < 2 {
		return 0, nil, nil, errors.New("truncated data object")
	}

	tag, n, i := b[0], int(b[1]), 2

	switch {
	case n == 0x81:
		if len(b) < 3 || b[2] < 0x80 {
			return 0, nil, nil, errors.New("invalid length")
		}

		n, i = int(b[2]), 3
	case n > 0x7f:
		return 0, nil, nil, fmt.Errorf("length coding %#02x", n)
	}

	if len(b) < i+n {
		return 0, nil, nil, errors.New("truncated data object")
	}

	return tag, b[i : i+n], b[i+n:], nil
}

// Download is what the card did with a (U)SIM data download.
type Download struct {
	TAR     [3]byte
	Counter uint64
	// PoR is whether the card answered with a Response Packet, and Status its status.
	PoR    bool
	Status Status
	// Executed is the number of commands of the command string that ran, and StatusWord the status word of the last.
	Executed   int
	StatusWord uint16
	// Err is why the card discarded the message, if it did.
	Err error
}

// DataDownload takes a (U)SIM data download, the SMS-DELIVER that an ENVELOPE (SMS-PP DOWNLOAD) carries, and returns
// the Response Packet that the card answers with, the RPI included, or nil when it sends none.
//
// The card follows TS 102 225 clause 4.1: it discards a Command Packet of a header it cannot interpret, or of a Sending
// Entity it cannot authenticate, without answering, as TS 31.111 clause 7.1.1.1 has a UICC do over SMS-PP; it answers
// a security error after authenticating the Sending Entity with a Response Packet of its status, secured as the
// command asks. A Response Packet too large for the SMS-DELIVER-REPORT has the status of a response sent by
// SMS-SUBMIT, without additional data (TS 31.115 clause 4.1); the card does not send that SMS-SUBMIT.
func (c *Card) DataDownload(d tpdu.Deliver) ([]byte, Download) {
	por, dl := c.dataDownload(d)

	if c.onDownload != nil {
		c.onDownload(dl)
	}

	return por, dl
}

func (c *Card) dataDownload(d tpdu.Deliver) ([]byte, Download) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// The Command Packet is in the user data header: the UDHI is set (TS 31.115 clause 4.1).
	if !d.UDHI {
		return nil, Download{Err: fmt.Errorf("%w: no user data header", errUnrecognized)}
	}

	cmd, err := unpack(d.UD, c.keysets)
	if err != nil {
		return nil, Download{Err: err}
	}

	dl := Download{TAR: cmd.tar, Counter: cmd.counter}

	answer := func(status Status, data []byte) ([]byte, Download) {
		dl.Status = status

		switch cmd.spi & spiPoR {
		case spiPoRAlways:
		case spiPoROnError:
			if status == StatusPoROK {
				return nil, dl
			}
		default:
			return nil, dl
		}

		r := response{tar: cmd.tar, counter: cmd.counter, status: status, data: data}

		por, err := r.pack(cmd)
		if err == nil && len(por) > tpdu.MaxDeliverReportUserData {
			r.status, r.data = StatusResponseInSubmit, nil
			dl.Status = r.status
			por, err = r.pack(cmd)
		}

		if err != nil {
			dl.Err = err
			return nil, dl
		}

		dl.PoR = true

		return por, dl
	}

	// The counter is updated once the command is authenticated and its counter checked (TS 102 225 clause 5.1.4).
	counter, status, ok := checkCounter(cmd, c.counters[cmd.keyset.Version])
	if !ok {
		return answer(status, nil)
	}

	c.counters[cmd.keyset.Version] = counter

	if cmd.tar != c.tar {
		return answer(StatusTARUnknown, nil)
	}

	result := c.files.run(cmd.data)
	dl.Executed, dl.StatusWord = result.executed, result.statusWord

	return answer(StatusPoROK, result.compact())
}
