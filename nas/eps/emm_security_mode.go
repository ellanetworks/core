// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package eps

import "github.com/ellanetworks/core/nas/common"

// imeisvRequestIEI is the IEI of the IMEISV request information element (type 1)
// in SECURITY MODE COMMAND; imeisvRequested is its "IMEISV requested" value
// (TS 24.301). imeisvIEI is the IEI of the IMEISV mobile-identity TLV
// returned in SECURITY MODE COMPLETE (TS 24.301).
const (
	imeisvRequestIEI uint8 = 0xC
	imeisvRequested  uint8 = 0x1
	imeisvIEI        uint8 = 0x23
	hashMMEIEI       uint8 = 0x4F
	// replayedNASMessageIEI is the IEI of the Replayed NAS message container IE
	// (type 6, TLV-E) the UE returns in SECURITY MODE COMPLETE when its HASHMME
	// check fails (TS 24.301 §8.2.21.1, §9.9.3.51). It shares the value 0x4F with
	// hashMMEIEI, in the opposite message direction.
	replayedNASMessageIEI uint8 = 0x4F
)

// SecurityModeCommand is the SECURITY MODE COMMAND message (TS 24.301),
// sent by the MME to select the NAS security algorithms. The optional part is
// preserved verbatim.
type SecurityModeCommand struct {
	CipheringAlgorithm             uint8
	IntegrityAlgorithm             uint8
	NASKeySetIdentifier            uint8
	ReplayedUESecurityCapabilities []byte
	IMEISVRequested                bool   // request the UE's IMEISV in SECURITY MODE COMPLETE
	HASHMME                        []byte // 8-octet hash of the triggering plain Attach/TAU (TS 24.301), nil when absent
}

// Marshal encodes the plain SECURITY MODE COMMAND message.
func (m *SecurityModeCommand) Marshal() ([]byte, error) {
	var w common.Writer

	writeEMMHeader(&w, MsgSecurityModeCommand)
	w.U8(m.CipheringAlgorithm&0x07<<4 | m.IntegrityAlgorithm&0x07) // selected NAS security algorithms
	w.U8(m.NASKeySetIdentifier & 0x0F)                             // NAS KSI | spare half octet

	if err := w.LV(m.ReplayedUESecurityCapabilities); err != nil {
		return nil, err
	}

	if m.IMEISVRequested {
		w.U8(imeisvRequestIEI<<4 | imeisvRequested)
	}

	if len(m.HASHMME) > 0 {
		w.U8(hashMMEIEI)

		if err := w.LV(m.HASHMME); err != nil {
			return nil, err
		}
	}

	return w.Bytes(), nil
}

// ParseSecurityModeCommand decodes a plain SECURITY MODE COMMAND message.
func ParseSecurityModeCommand(b []byte) (*SecurityModeCommand, error) {
	r := common.NewReader(b)

	if err := readEMMHeader(r, MsgSecurityModeCommand); err != nil {
		return nil, err
	}

	alg, err := r.U8()
	if err != nil {
		return nil, err
	}

	ksi, err := r.U8()
	if err != nil {
		return nil, err
	}

	m := &SecurityModeCommand{
		CipheringAlgorithm:  alg >> 4 & 0x07,
		IntegrityAlgorithm:  alg & 0x07,
		NASKeySetIdentifier: ksi & 0x0F,
	}

	if m.ReplayedUESecurityCapabilities, err = r.LV(); err != nil {
		return nil, err
	}

	// The IMEISV request is a type-1 IE the walker delimits inherently (IEI >= 0x80
	// after the half-octet shift); HashMME is a type-4 TLV and needs a table entry.
	if _, err := common.WalkOptionalIEs(r, securityModeCommandIEs, func(iei uint8, value []byte) error {
		switch {
		case iei == imeisvRequestIEI<<4 && len(value) == 1:
			m.IMEISVRequested = value[0] == imeisvRequested
		case iei == hashMMEIEI:
			m.HASHMME = value
		}

		return nil
	}); err != nil {
		return nil, err
	}

	return m, nil
}

// securityModeCommandIEs are the optional type-4 IEs Ella Core round-trips in a
// SECURITY MODE COMMAND (TS 24.301): the HashMME.
var securityModeCommandIEs = []common.OptionalIE{
	{IEI: hashMMEIEI, Format: common.IETLV},
}

// SecurityModeComplete is the SECURITY MODE COMPLETE message (TS 24.301).
// It has no mandatory information elements; the UE includes its IMEISV
// (a mobile identity, IEI 0x23) when the MME requested it, and — when its HASHMME
// check fails — the complete plain ATTACH/TAU REQUEST it originally sent, in the
// Replayed NAS message container (IEI 0x4F), so the network can recover the
// genuine triggering message (TS 24.301 §5.4.3.4).
type SecurityModeComplete struct {
	IMEISV             []byte // IMEISV mobile-identity value (IEI 0x23), when present
	ReplayedNASMessage []byte // complete triggering NAS message (IEI 0x4F), when present
}

// securityModeCompleteIEs are the optional IEs Ella Core consumes from a
// SECURITY MODE COMPLETE (TS 24.301): the UE's IMEISV mobile identity and the
// Replayed NAS message container.
var securityModeCompleteIEs = []common.OptionalIE{
	{IEI: imeisvIEI, Format: common.IETLV},
	{IEI: replayedNASMessageIEI, Format: common.IETLVE},
}

// Marshal encodes the plain SECURITY MODE COMPLETE message.
func (m *SecurityModeComplete) Marshal() ([]byte, error) {
	var w common.Writer

	writeEMMHeader(&w, MsgSecurityModeComplete)

	if len(m.IMEISV) > 0 {
		w.U8(imeisvIEI)

		if err := w.LV(m.IMEISV); err != nil {
			return nil, err
		}
	}

	if len(m.ReplayedNASMessage) > 0 {
		w.U8(replayedNASMessageIEI)

		if err := w.LVE(m.ReplayedNASMessage); err != nil {
			return nil, err
		}
	}

	return w.Bytes(), nil
}

// ParseSecurityModeComplete decodes a plain SECURITY MODE COMPLETE message.
func ParseSecurityModeComplete(b []byte) (*SecurityModeComplete, error) {
	r := common.NewReader(b)

	if err := readEMMHeader(r, MsgSecurityModeComplete); err != nil {
		return nil, err
	}

	m := &SecurityModeComplete{}

	if _, err := common.WalkOptionalIEs(r, securityModeCompleteIEs, func(iei uint8, value []byte) error {
		switch iei {
		case imeisvIEI:
			m.IMEISV = value
		case replayedNASMessageIEI:
			m.ReplayedNASMessage = value
		}

		return nil
	}); err != nil {
		return nil, err
	}

	return m, nil
}

// SecurityModeReject is the SECURITY MODE REJECT message (TS 24.301).
type SecurityModeReject struct {
	Cause uint8
}

// Marshal encodes the plain SECURITY MODE REJECT message.
func (m *SecurityModeReject) Marshal() ([]byte, error) {
	var w common.Writer

	writeEMMHeader(&w, MsgSecurityModeReject)
	w.U8(m.Cause)

	return w.Bytes(), nil
}

// ParseSecurityModeReject decodes a plain SECURITY MODE REJECT message.
func ParseSecurityModeReject(b []byte) (*SecurityModeReject, error) {
	r := common.NewReader(b)

	if err := readEMMHeader(r, MsgSecurityModeReject); err != nil {
		return nil, err
	}

	cause, err := r.U8()
	if err != nil {
		return nil, err
	}

	return &SecurityModeReject{Cause: cause}, nil
}
