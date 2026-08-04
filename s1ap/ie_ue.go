// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"fmt"

	"github.com/ellanetworks/core/per"
)

// UE S1AP ID ranges (TS 36.413).
const (
	enbUES1APIDMax = 16777215   // INTEGER (0..2^24-1)
	mmeUES1APIDMax = 4294967295 // INTEGER (0..2^32-1)
)

// ENBUES1APID ::= INTEGER (0..16777215).
type ENBUES1APID uint32

func (id ENBUES1APID) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return per.EncodeInteger(w, enc, per.Bounds{LB: 0, HasLB: true, UB: enbUES1APIDMax, HasUB: true}, int64(id))
}

func (id *ENBUES1APID) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	v, err := per.DecodeInteger(r, enc, per.Bounds{LB: 0, HasLB: true, UB: enbUES1APIDMax, HasUB: true})
	if err != nil {
		return err
	}

	*id = ENBUES1APID(v)

	return nil
}

// MMEUES1APID ::= INTEGER (0..4294967295).
type MMEUES1APID uint32

func (id MMEUES1APID) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return per.EncodeInteger(w, enc, per.Bounds{LB: 0, HasLB: true, UB: mmeUES1APIDMax, HasUB: true}, int64(id))
}

func (id *MMEUES1APID) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	v, err := per.DecodeInteger(r, enc, per.Bounds{LB: 0, HasLB: true, UB: mmeUES1APIDMax, HasUB: true})
	if err != nil {
		return err
	}

	*id = MMEUES1APID(v)

	return nil
}

// NASPDU ::= OCTET STRING (unbounded), carried opaquely (TS 24.301).
type NASPDU []byte

func (n NASPDU) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return per.EncodeOctetString(w, enc, 0, 0, true, false, false, n)
}

func (n *NASPDU) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	b, err := per.DecodeOctetString(r, enc, 0, 0, true, false, false)
	if err != nil {
		return err
	}

	*n = NASPDU(b)

	return nil
}

// TAI ::= SEQUENCE { pLMNidentity, tAC, iE-Extensions OPTIONAL } (extensible).
type TAI struct {
	_            [0]struct{} `per:"extseq"`
	PLMNIdentity PLMNIdentity
	TAC          TAC
	_            ieExtensions `per:",skip"`
}

// EUTRANCGI ::= SEQUENCE { pLMNidentity, cell-ID CellIdentity, iE-Extensions
// OPTIONAL } (extensible). CellIdentity ::= BIT STRING (SIZE(28)).
type EUTRANCGI struct {
	PLMNIdentity PLMNIdentity
	CellID       uint32 // 28-bit cell identity
}

const cellIDBits = 28

func (c EUTRANCGI) MarshalPER(w *per.Writer, enc per.Encoding) error {
	w.WriteBit(false)
	w.WriteBit(false)

	if err := c.PLMNIdentity.MarshalPER(w, enc); err != nil {
		return err
	}

	return per.EncodeBitString(w, enc, cellIDBits, cellIDBits, true, true, false, uintToBits(uint64(c.CellID), cellIDBits), cellIDBits)
}

func (c *EUTRANCGI) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	extBit, err := r.ReadBit()
	if err != nil {
		return err
	}

	extContainer, err := r.ReadBit()
	if err != nil {
		return err
	}

	var plmn PLMNIdentity
	if err := plmn.UnmarshalPER(r, enc); err != nil {
		return err
	}

	b, _, err := per.DecodeBitString(r, enc, cellIDBits, cellIDBits, true, true, false)
	if err != nil {
		return err
	}

	if err := skipSequenceExtensionsPER(r, enc, extContainer, extBit); err != nil {
		return err
	}

	*c = EUTRANCGI{PLMNIdentity: plmn, CellID: uint32(bitsToUint(b, cellIDBits))}

	return nil
}

// UserLocationInformation ::= SEQUENCE { eutran-CGI EUTRAN-CGI, tai TAI,
// iE-Extensions OPTIONAL, ... } (extensible) — TS 36.413 §9.2.1.86.
type UserLocationInformation struct {
	_         [0]struct{} `per:"extseq"`
	EUTRANCGI EUTRANCGI
	TAI       TAI
	_         ieExtensions `per:",skip"`
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

func (c RRCEstablishmentCause) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return encodeRootEnumerated(w, enc, rrcEstablishmentCauseRootCount, int64(c), "RRCEstablishmentCause")
}

func (c *RRCEstablishmentCause) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	idx, err := decodeRootEnumerated(r, enc, rrcEstablishmentCauseRootCount, "RRCEstablishmentCause")
	if err != nil {
		return fmt.Errorf("s1ap: rrc establishment cause: %w", err)
	}

	*c = RRCEstablishmentCause(idx)

	return nil
}
