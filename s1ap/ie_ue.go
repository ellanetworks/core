// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"fmt"

	"github.com/ellanetworks/core/s1ap/aper"
)

// UE S1AP ID ranges (TS 36.413 §9.2.3).
const (
	enbUES1APIDMax = 16777215   // INTEGER (0..2^24-1)
	mmeUES1APIDMax = 4294967295 // INTEGER (0..2^32-1)
)

// ENBUES1APID ::= INTEGER (0..16777215).
type ENBUES1APID uint32

func (id ENBUES1APID) encode(w *aper.Writer) error {
	return w.WriteConstrainedInt(int64(id), 0, enbUES1APIDMax)
}

func decodeENBUES1APID(r *aper.Reader) (ENBUES1APID, error) {
	v, err := r.ReadConstrainedInt(0, enbUES1APIDMax)
	return ENBUES1APID(v), err
}

// MMEUES1APID ::= INTEGER (0..4294967295).
type MMEUES1APID uint32

func (id MMEUES1APID) encode(w *aper.Writer) error {
	return w.WriteConstrainedInt(int64(id), 0, mmeUES1APIDMax)
}

func decodeMMEUES1APID(r *aper.Reader) (MMEUES1APID, error) {
	v, err := r.ReadConstrainedInt(0, mmeUES1APIDMax)
	return MMEUES1APID(v), err
}

// NASPDU ::= OCTET STRING (unbounded). The S1AP layer carries NAS opaquely; the
// bytes are decoded by the EPS NAS codec (TS 24.301), not here.
type NASPDU []byte

func (n NASPDU) encode(w *aper.Writer) error {
	return w.WriteOctetString(n, 0, aper.Unbounded, false)
}

func decodeNASPDU(r *aper.Reader) (NASPDU, error) {
	b, err := r.ReadOctetString(0, aper.Unbounded, false)
	return NASPDU(b), err
}

// TAI ::= SEQUENCE { pLMNidentity, tAC, iE-Extensions OPTIONAL } (extensible).
type TAI struct {
	PLMNIdentity PLMNIdentity
	TAC          TAC
}

func (t TAI) encode(w *aper.Writer) error {
	w.WriteSequencePreamble(true, false, []bool{false})

	if err := t.PLMNIdentity.encode(w); err != nil {
		return err
	}

	return t.TAC.encode(w)
}

func decodeTAI(r *aper.Reader) (TAI, error) {
	extPresent, opt, err := r.ReadSequencePreamble(true, 1)
	if err != nil {
		return TAI{}, err
	}

	plmn, err := decodePLMNIdentity(r)
	if err != nil {
		return TAI{}, err
	}

	tac, err := decodeTAC(r)
	if err != nil {
		return TAI{}, err
	}

	if err := skipSequenceExtensions(r, opt[0], extPresent); err != nil {
		return TAI{}, err
	}

	return TAI{PLMNIdentity: plmn, TAC: tac}, nil
}

// EUTRANCGI ::= SEQUENCE { pLMNidentity, cell-ID CellIdentity, iE-Extensions
// OPTIONAL } (extensible). CellIdentity ::= BIT STRING (SIZE(28)).
type EUTRANCGI struct {
	PLMNIdentity PLMNIdentity
	CellID       uint32 // 28-bit cell identity
}

const cellIDBits = 28

func (c EUTRANCGI) encode(w *aper.Writer) error {
	w.WriteSequencePreamble(true, false, []bool{false})

	if err := c.PLMNIdentity.encode(w); err != nil {
		return err
	}

	return w.WriteBitString(uintToBits(uint64(c.CellID), cellIDBits), cellIDBits, cellIDBits, cellIDBits, false)
}

func decodeEUTRANCGI(r *aper.Reader) (EUTRANCGI, error) {
	extPresent, opt, err := r.ReadSequencePreamble(true, 1)
	if err != nil {
		return EUTRANCGI{}, err
	}

	plmn, err := decodePLMNIdentity(r)
	if err != nil {
		return EUTRANCGI{}, err
	}

	b, _, err := r.ReadBitString(cellIDBits, cellIDBits, false)
	if err != nil {
		return EUTRANCGI{}, err
	}

	if err := skipSequenceExtensions(r, opt[0], extPresent); err != nil {
		return EUTRANCGI{}, err
	}

	return EUTRANCGI{PLMNIdentity: plmn, CellID: uint32(bitsToUint(b, cellIDBits))}, nil
}

// RRCEstablishmentCause ::= ENUMERATED { emergency, highPriorityAccess,
// mt-Access, mo-Signalling, mo-Data, ... } (extensible).
type RRCEstablishmentCause uint8

const (
	RRCCauseEmergency RRCEstablishmentCause = iota
	RRCCauseHighPriorityAccess
	RRCCauseMTAccess
	RRCCauseMOSignalling
	RRCCauseMOData

	rrcEstablishmentCauseRootCount = 5
)

func (c RRCEstablishmentCause) encode(w *aper.Writer) error {
	return w.WriteEnum(int(c), rrcEstablishmentCauseRootCount, true, false)
}

func decodeRRCEstablishmentCause(r *aper.Reader) (RRCEstablishmentCause, error) {
	idx, _, err := r.ReadEnum(rrcEstablishmentCauseRootCount, true)
	if err != nil {
		return 0, fmt.Errorf("s1ap: rrc establishment cause: %w", err)
	}

	return RRCEstablishmentCause(idx), nil
}
