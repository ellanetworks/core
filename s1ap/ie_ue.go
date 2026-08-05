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

// BitRate ::= INTEGER (0..10000000000).
const bitRateMax = 10000000000

type BitRate uint64

func (b BitRate) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return per.EncodeInteger(w, enc, per.Bounds{LB: 0, HasLB: true, UB: bitRateMax, HasUB: true}, int64(b))
}

func (b *BitRate) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	v, err := per.DecodeInteger(r, enc, per.Bounds{LB: 0, HasLB: true, UB: bitRateMax, HasUB: true})
	if err != nil {
		return err
	}

	*b = BitRate(v)

	return nil
}

// UEAggregateMaximumBitRate ::= SEQUENCE { ...DL, ...UL, iE-Extensions OPTIONAL }
// (extensible).
type UEAggregateMaximumBitRate struct {
	_  [0]struct{} `per:"extseq"`
	DL BitRate
	UL BitRate
	_  ieExtensions `per:",skip"`
}

// SecurityKey ::= BIT STRING (SIZE(256)).
type SecurityKey [32]byte

func (k SecurityKey) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return per.EncodeBitString(w, enc, 256, 256, true, true, false, k[:], 256)
}

func (k *SecurityKey) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	b, _, err := per.DecodeBitString(r, enc, 256, 256, true, true, false)
	if err != nil {
		return err
	}

	copy(k[:], b)

	return nil
}

// UESecurityCapabilities ::= SEQUENCE { encryptionAlgorithms,
// integrityProtectionAlgorithms, iE-Extensions OPTIONAL } (extensible). Each
// algorithm field is BIT STRING (SIZE(16, ...)).
type UESecurityCapabilities struct {
	EncryptionAlgorithms          uint16
	IntegrityProtectionAlgorithms uint16
}

func (c UESecurityCapabilities) MarshalPER(w *per.Writer, enc per.Encoding) error {
	w.WriteBit(false)
	w.WriteBit(false)

	if err := per.EncodeBitString(w, enc, 16, 16, true, true, true, uintToBits(uint64(c.EncryptionAlgorithms), 16), 16); err != nil {
		return err
	}

	return per.EncodeBitString(w, enc, 16, 16, true, true, true, uintToBits(uint64(c.IntegrityProtectionAlgorithms), 16), 16)
}

func (c *UESecurityCapabilities) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	extBit, err := r.ReadBit()
	if err != nil {
		return err
	}

	extContainer, err := r.ReadBit()
	if err != nil {
		return err
	}

	encAlgs, encBits, err := per.DecodeBitString(r, enc, 16, 16, true, true, true)
	if err != nil {
		return err
	}

	integ, integBits, err := per.DecodeBitString(r, enc, 16, 16, true, true, true)
	if err != nil {
		return err
	}

	if err := skipSequenceExtensionsPER(r, enc, extContainer, extBit); err != nil {
		return err
	}

	*c = UESecurityCapabilities{
		EncryptionAlgorithms:          uint16(bitsToUint(encAlgs, encBits)),
		IntegrityProtectionAlgorithms: uint16(bitsToUint(integ, integBits)),
	}

	return nil
}

// UERadioCapability ::= OCTET STRING (unconstrained) — TS 36.413 §9.2.1.27.
type UERadioCapability []byte

func (c UERadioCapability) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return per.EncodeOctetString(w, enc, 0, 0, true, false, false, c)
}

func (c *UERadioCapability) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	b, err := per.DecodeOctetString(r, enc, 0, 0, true, false, false)
	if err != nil {
		return err
	}

	*c = UERadioCapability(b)

	return nil
}

// UERadioCapabilityForPaging ::= OCTET STRING — TS 36.413 §9.2.1.98. NGAP's
// counterpart is a SEQUENCE of a separate NR and E-UTRA capability.
type UERadioCapabilityForPaging []byte

func (c UERadioCapabilityForPaging) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return per.EncodeOctetString(w, enc, 0, 0, true, false, false, c)
}

func (c *UERadioCapabilityForPaging) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	b, err := per.DecodeOctetString(r, enc, 0, 0, true, false, false)
	if err != nil {
		return err
	}

	*c = UERadioCapabilityForPaging(b)

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

	return encodeBitStringUint(w, enc, uint64(c.CellID), cellIDBits)
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
// iE-Extensions OPTIONAL, ... } (extensible) — TS 36.413 §9.2.1.93.
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
