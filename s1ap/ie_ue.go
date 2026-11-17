// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
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
	DL           BitRate
	UL           BitRate
	ExtendedDL   *ExtendedBitRate       `per:",extie:IDExtendedUEAggregateMaximumBitRateDL,crit:ignore"`
	ExtendedUL   *ExtendedBitRate       `per:",extie:IDExtendedUEAggregateMaximumBitRateUL,crit:ignore"`
	iEExtensions extensionMeta          `per:",exttable"`
	extensions   per.ExtensionAdditions `per:",extraw"`
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
	EncryptionAlgorithms          SecurityAlgorithms
	IntegrityProtectionAlgorithms SecurityAlgorithms
	iEExtensions                  extensionMeta          `per:",exttable"`
	extensions                    per.ExtensionAdditions `per:",extraw"`
}

type SecurityAlgorithms struct {
	Value    uint16
	extended bool
	octets   string
	bits     int
}

const securityAlgorithmsBits = 16

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
	extensions   per.ExtensionAdditions `per:",extraw"`
	PLMNIdentity PLMNIdentity
	TAC          TAC
	iEExtensions extensionMeta `per:",exttable"`
}

// EUTRANCGI ::= SEQUENCE { pLMNidentity, cell-ID CellIdentity, iE-Extensions
// OPTIONAL } (extensible). CellIdentity ::= BIT STRING (SIZE(28)).
type EUTRANCGI struct {
	PLMNIdentity PLMNIdentity
	CellID       CellIdentity
	iEExtensions extensionMeta          `per:",exttable"`
	extensions   per.ExtensionAdditions `per:",extraw"`
}

type CellIdentity uint32

const cellIDBits = 28

// UserLocationInformation ::= SEQUENCE { eutran-CGI EUTRAN-CGI, tai TAI,
// iE-Extensions OPTIONAL, ... } (extensible) — TS 36.413 §9.2.1.93.
type UserLocationInformation struct {
	extensions           per.ExtensionAdditions `per:",extraw"`
	EUTRANCGI            EUTRANCGI
	TAI                  TAI
	PSCellInformation    *PSCellInformation    `per:",extie:IDPSCellInformation,crit:ignore"`
	LTENTNTAIInformation *LTENTNTAIInformation `per:",extie:IDLTENTNTAIInformation,crit:ignore"`
	iEExtensions         extensionMeta         `per:",exttable"`
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
	RRCCauseDelayTolerantAccess
	RRCCauseMOVoiceCall
	RRCCauseMOExceptionData
)

type GWContextReleaseIndication uint8

const (
	GWContextReleaseIndicationTrue GWContextReleaseIndication = iota
)
