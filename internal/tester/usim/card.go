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

	"github.com/ellanetworks/core/internal/tester/cat"
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

// The status words of the ENVELOPE (TS 102 221 clause 10.2.1).
const (
	swOK                  = cat.SWOK
	swIncorrectData       = 0x6a80
	swInstructionNotFound = 0x6d00
)

// Config is a card's.
type Config struct {
	// TAR is that of the card's ADF RFM application linked to the USIM, of the compact data format: 'B0 00 01' or
	// 'B0 00 20' to 'B0 01 1F' (TS 101 220 annex D).
	TAR                  [3]byte
	MinimumSecurityLevel byte
	Keysets              []Keyset
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
	msl        byte
	keysets    map[uint8]Keyset
	onDownload func(Download)

	mu sync.Mutex
	// counters are the counters of the key sets, by their versions: the last one each accepted.
	counters map[uint8]uint64
	files    *fileSystem

	proactive   []proactiveCommand
	outstanding *proactiveCommand
	number      byte
	series      int
	concatRef   byte
}

type proactiveCommand struct {
	details cat.CommandDetails
	series  int
	data    []byte
}

// New makes a card.
func New(cfg Config) (*Card, error) {
	if !adfRFMCompact(cfg.TAR) {
		return nil, fmt.Errorf("usim: TAR %x is not of an ADF RFM application of the compact data format", cfg.TAR)
	}

	if cfg.MinimumSecurityLevel&^0x1f != 0 || cfg.MinimumSecurityLevel&0x03 == 0x03 {
		return nil, fmt.Errorf("usim: minimum SPI1 %#02x", cfg.MinimumSecurityLevel)
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
		tar: cfg.TAR, msl: cfg.MinimumSecurityLevel, keysets: keysets, onDownload: cfg.OnDownload, counters: map[uint8]uint64{},
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
	tag, value, _, err := cat.ReadTLV(data)
	if err != nil {
		return nil, swIncorrectData
	}

	if tag != cat.TagSMSPPDownload {
		return nil, swInstructionNotFound
	}

	deliver, serviceCentre, err := smsPPDownload(value)
	if err != nil {
		return nil, swIncorrectData
	}

	por, _ := c.download(deliver, serviceCentre)

	return por, c.status()
}

// smsPPDownload is the SMS-DELIVER of the value of an ENVELOPE (SMS-PP DOWNLOAD), from the network to the UICC (TS
// 31.111 clause 7.1.1.2).
func smsPPDownload(value []byte) (tpdu.Deliver, []byte, error) {
	objects, err := cat.ReadObjects(value)
	if err != nil {
		return tpdu.Deliver{}, nil, err
	}

	if identities, _ := objects.Get(cat.TagDeviceIdentities); !bytes.Equal(identities, []byte{cat.DeviceNetwork, cat.DeviceUICC}) {
		return tpdu.Deliver{}, nil, fmt.Errorf("device identities %x, want from the network to the UICC", identities)
	}

	smsTPDU, ok := objects.Get(cat.TagSMSTPDU)
	if !ok {
		return tpdu.Deliver{}, nil, errors.New("no SMS TPDU")
	}

	deliver, err := tpdu.DecodeDeliver(smsTPDU)
	if err != nil {
		return tpdu.Deliver{}, nil, err
	}

	serviceCentre, _ := objects.Get(cat.TagAddress)

	return deliver, serviceCentre, nil
}

func (c *Card) Fetch() ([]byte, uint16) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.outstanding != nil || len(c.proactive) == 0 {
		return nil, swTechnicalProblem
	}

	next := c.proactive[0]
	c.proactive, c.outstanding = c.proactive[1:], &next

	return next.data, swOK
}

func (c *Card) TerminalResponse(data []byte) uint16 {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.outstanding == nil {
		return swTechnicalProblem
	}

	objects, err := cat.ReadObjects(data)
	if err != nil {
		return swIncorrectData
	}

	raw, _ := objects.Get(cat.TagCommandDetails)
	result, _ := objects.Get(cat.TagResult)

	details, err := cat.ParseCommandDetails(raw)
	if err != nil || details != c.outstanding.details || len(result) == 0 {
		return swIncorrectData
	}

	if result[0] > 0x1f {
		kept := c.proactive[:0]

		for _, p := range c.proactive {
			if p.series != c.outstanding.series {
				kept = append(kept, p)
			}
		}

		c.proactive = kept
	}

	c.outstanding = nil

	return c.statusLocked()
}

func (c *Card) status() uint16 {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.statusLocked()
}

func (c *Card) statusLocked() uint16 {
	if c.outstanding != nil || len(c.proactive) == 0 {
		return swOK
	}

	return uint16(cat.SWProactive)<<8 | uint16(len(c.proactive[0].data))
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
	Submits    int
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
// SMS-SUBMIT, without additional data (TS 31.115 clause 4.1).
func (c *Card) DataDownload(d tpdu.Deliver) ([]byte, Download) {
	return c.download(d, nil)
}

func (c *Card) download(d tpdu.Deliver, serviceCentre []byte) ([]byte, Download) {
	por, dl := c.dataDownload(d, serviceCentre)

	if c.onDownload != nil {
		c.onDownload(dl)
	}

	return por, dl
}

func (c *Card) dataDownload(d tpdu.Deliver, serviceCentre []byte) ([]byte, Download) {
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
		if err != nil {
			dl.Err = err
			return nil, dl
		}

		dl.PoR = true

		if cmd.spi&spiPoRSubmit != 0 {
			dl.Submits, dl.Err = c.submit(por, d, serviceCentre)
			return nil, dl
		}

		if len(por) <= tpdu.MaxDeliverReportUserData {
			return por, dl
		}

		r.status, r.data = StatusResponseInSubmit, nil
		dl.Status = r.status

		report, err := r.pack(cmd)
		if err != nil {
			dl.Err = err
			return nil, dl
		}

		dl.Submits, dl.Err = c.submit(por, d, serviceCentre)

		return report, dl
	}

	if cmd.keyVersionsDiffer() {
		if !cmd.authenticated() {
			dl.Err = fmt.Errorf("%w: KIc %#02x and KID %#02x of different key versions", errUnauthenticated, cmd.kic, cmd.kid)
			return nil, dl
		}

		cmd.spi &^= spiPoRIntegrity | spiPoRCiphering

		return answer(StatusUnidentified, nil)
	}

	if !cmd.sufficient(c.msl) {
		if !cmd.authenticated() {
			dl.Err = fmt.Errorf("%w: SPI %04x below the minimum security level", errUnauthenticated, cmd.spi)
			return nil, dl
		}

		return answer(StatusInsufficient, nil)
	}

	// The counter is updated once the command is authenticated and its counter checked (TS 102 225 clause 5.1.4).
	counter, status, ok := checkCounter(cmd, c.counters[cmd.keyVersion()])
	if !ok {
		return answer(status, nil)
	}

	c.counters[cmd.keyVersion()] = counter

	if cmd.tar != c.tar {
		return answer(StatusTARUnknown, nil)
	}

	result := c.files.run(cmd.data)
	dl.Executed, dl.StatusWord = result.executed, result.statusWord

	return answer(StatusPoROK, result.compact())
}

func (c *Card) submit(por []byte, d tpdu.Deliver, serviceCentre []byte) (int, error) {
	parts := [][]byte{por}

	if len(por) > tpdu.MaxSubmitUserData {
		parts = concatenate(por[len(responseUDH):], c.concatRef)
		c.concatRef++
	}

	submits := make([][]byte, 0, len(parts))

	for _, ud := range parts {
		submit, err := tpdu.Submit{UDHI: true, Destination: d.RawOriginatingAddress, PID: d.PID, DCS: d.DCS, UD: ud}.AppendBinary(nil)
		if err != nil {
			return 0, fmt.Errorf("usim: SMS-SUBMIT of the PoR: %w", err)
		}

		submits = append(submits, submit)
	}

	c.series++

	for _, submit := range submits {
		c.number = c.number%0xfe + 1
		details := cat.CommandDetails{Number: c.number, Type: cat.TypeSendShortMessage}

		objects := cat.AppendTLV(nil, cat.TagCommandDetails|cat.ComprehensionRequired, details.Value())
		objects = cat.AppendTLV(objects, cat.TagDeviceIdentities|cat.ComprehensionRequired, []byte{cat.DeviceUICC, cat.DeviceNetwork})

		if serviceCentre != nil {
			objects = cat.AppendTLV(objects, cat.TagAddress, serviceCentre)
		}

		objects = cat.AppendTLV(objects, cat.TagSMSTPDU|cat.ComprehensionRequired, submit)

		c.proactive = append(c.proactive, proactiveCommand{
			details: details, series: c.series, data: cat.AppendTLV(nil, cat.TagProactiveCommand, objects),
		})
	}

	return len(submits), nil
}

func concatenate(packet []byte, ref byte) [][]byte {
	const (
		first = tpdu.MaxSubmitUserData - 8
		next  = tpdu.MaxSubmitUserData - 6
	)

	total := 1
	if len(packet) > first {
		total += (len(packet) - first + next - 1) / next
	}

	parts := make([][]byte, 0, total)

	for i := range total {
		size, udh := next, []byte{0x05, 0x00, 0x03, ref, byte(total), byte(i + 1)}
		if i == 0 {
			size, udh = first, []byte{0x07, 0x00, 0x03, ref, byte(total), 0x01, 0x71, 0x00}
		}

		size = min(size, len(packet))
		parts = append(parts, concat(udh, packet[:size]))
		packet = packet[size:]
	}

	return parts
}
