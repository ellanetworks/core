// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package fgs

import "github.com/ellanetworks/core/nas/common"

// UESecurityCapability is the UE security capability IE (TS 24.501 §9.11.3.54).
// EA and IA are the 5G encryption and integrity algorithm bitmaps (octets 1-2);
// Rest holds the optional EEA/EIA (E-UTRA) octets and any beyond.
type UESecurityCapability struct {
	EA   uint8
	IA   uint8
	Rest []byte
}

// ParseUESecurityCapability decodes a UE security capability IE value.
func ParseUESecurityCapability(b []byte) (UESecurityCapability, error) {
	r := common.NewReader(b)

	ea, err := r.U8()
	if err != nil {
		return UESecurityCapability{}, err
	}

	ia, err := r.U8()
	if err != nil {
		return UESecurityCapability{}, err
	}

	rest, err := r.Bytes(r.Remaining())
	if err != nil {
		return UESecurityCapability{}, err
	}

	return UESecurityCapability{EA: ea, IA: ia, Rest: rest}, nil
}

// SupportsEA reports whether the UE supports 128-5G-EAn (n = 0..7).
func (c UESecurityCapability) SupportsEA(n uint8) bool {
	return n <= 7 && c.EA&(1<<(7-n)) != 0
}

// SupportsIA reports whether the UE supports 128-5G-IAn (n = 0..7).
func (c UESecurityCapability) SupportsIA(n uint8) bool {
	return n <= 7 && c.IA&(1<<(7-n)) != 0
}

// IMEISV request values (TS 24.501 §9.11.3.28).
const (
	IMEISVNotRequested uint8 = 0x00
	IMEISVRequested    uint8 = 0x01
)

// SecurityModeCommand is the SECURITY MODE COMMAND message (TS 24.501 §8.2.25):
// the selected NAS security algorithms, the ngKSI, and the replayed UE security
// capabilities, with optional IMEISV request and additional 5G security
// information.
type SecurityModeCommand struct {
	CipheringAlgorithm  uint8 // selected NEA (bits 5-8)
	IntegrityAlgorithm  uint8 // selected NIA (bits 1-4)
	NgKSI               uint8 // ngKSI half octet (bits 1-4)
	ReplayedUESecCap    []byte
	IMEISVRequest       *uint8 // optional (IEI 0xE)
	Additional5GSecInfo *uint8 // optional (IEI 0x36): RINMR (bit 2), HDP (bit 1)
}

// Marshal encodes the plain SECURITY MODE COMMAND message.
func (m *SecurityModeCommand) Marshal() ([]byte, error) {
	var w common.Writer

	writeGMMHeader(&w, MsgSecurityModeCommand)
	w.U8((m.CipheringAlgorithm&0x0F)<<4 | (m.IntegrityAlgorithm & 0x0F))
	w.U8(m.NgKSI & 0x0F) // spare half octet in bits 5-8

	if err := w.LV(m.ReplayedUESecCap); err != nil {
		return nil, err
	}

	if m.IMEISVRequest != nil {
		w.U8(ieiIMEISVRequest | (*m.IMEISVRequest & 0x07))
	}

	if m.Additional5GSecInfo != nil {
		writeTLV(&w, ieiAdditional5GSec, []byte{*m.Additional5GSecInfo})
	}

	return w.Bytes(), nil
}

// SecurityModeComplete is the SECURITY MODE COMPLETE message (TS 24.501 §8.2.26).
// It has no mandatory IEs; the UE includes its IMEISV (a 5GS mobile identity,
// IEI 0x77) when the network requested it, and — when it rejected the replayed
// UE security capabilities — the complete plain REGISTRATION REQUEST it originally
// sent, in the NAS message container (IEI 0x71), so the network can recover the
// genuine triggering message (TS 24.501 §5.4.2.3).
type SecurityModeComplete struct {
	IMEISV              []byte // IMEISV mobile-identity value (IEI 0x77), when present
	NASMessageContainer []byte // complete triggering NAS message (IEI 0x71), when present
}

var securityModeCompleteIEs = []common.OptionalIE{
	{IEI: 0x77, Format: common.IETLVE},
	{IEI: 0x71, Format: common.IETLVE},
}

// ParseSecurityModeComplete decodes a plain SECURITY MODE COMPLETE message.
func ParseSecurityModeComplete(b []byte) (*SecurityModeComplete, error) {
	r := common.NewReader(b)

	if err := readGMMHeader(r, MsgSecurityModeComplete); err != nil {
		return nil, err
	}

	m := &SecurityModeComplete{}

	if _, err := common.WalkOptionalIEs(r, securityModeCompleteIEs, func(iei uint8, value []byte) error {
		switch iei {
		case 0x77:
			m.IMEISV = value
		case 0x71:
			m.NASMessageContainer = value
		}

		return nil
	}); err != nil {
		return nil, err
	}

	return m, nil
}
