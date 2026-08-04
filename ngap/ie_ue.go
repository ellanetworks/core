// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"fmt"

	"github.com/ellanetworks/core/per"
)

// UE NGAP ID ranges (TS 38.413 §9.3.3). The AMF id is 40 bits wide, so both
// are held in a uint64.
const (
	amfUENGAPIDMax = 1099511627775 // INTEGER (0..2^40-1)
	ranUENGAPIDMax = 4294967295    // INTEGER (0..2^32-1)
)

// AMFUENGAPID ::= INTEGER (0..1099511627775).
type AMFUENGAPID uint64

func (id AMFUENGAPID) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return per.EncodeInteger(w, enc, per.Bounds{LB: 0, HasLB: true, UB: amfUENGAPIDMax, HasUB: true}, int64(id))
}

func (id *AMFUENGAPID) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	v, err := per.DecodeInteger(r, enc, per.Bounds{LB: 0, HasLB: true, UB: amfUENGAPIDMax, HasUB: true})
	if err != nil {
		return err
	}

	*id = AMFUENGAPID(v)

	return nil
}

// RANUENGAPID ::= INTEGER (0..4294967295).
type RANUENGAPID uint32

func (id RANUENGAPID) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return per.EncodeInteger(w, enc, per.Bounds{LB: 0, HasLB: true, UB: ranUENGAPIDMax, HasUB: true}, int64(id))
}

func (id *RANUENGAPID) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	v, err := per.DecodeInteger(r, enc, per.Bounds{LB: 0, HasLB: true, UB: ranUENGAPIDMax, HasUB: true})
	if err != nil {
		return err
	}

	*id = RANUENGAPID(v)

	return nil
}

// RRCEstablishmentCause ::= ENUMERATED { emergency, highPriorityAccess,
// mt-Access, mo-Signalling, mo-Data, mo-VoiceCall, mo-VideoCall, mo-SMS,
// mps-PriorityAccess, mcs-PriorityAccess, ... } (extensible).
//
// The first five match S1AP's root, which stops there. S1AP reaches mo-VoiceCall
// as an extension addition; mo-VideoCall, mo-SMS and the two priority-access
// values have no S1AP counterpart at all (TS 38.413 §9.3.1.111 vs
// TS 36.413 §9.2.1.3a).
type RRCEstablishmentCause uint8

const (
	RRCCauseEmergency RRCEstablishmentCause = iota
	RRCCauseHighPriorityAccess
	RRCCauseMTAccess
	RRCCauseMOSignalling
	RRCCauseMOData
	RRCCauseMOVoiceCall
	RRCCauseMOVideoCall
	RRCCauseMOSMS
	RRCCauseMPSPriorityAccess
	RRCCauseMCSPriorityAccess

	rrcEstablishmentCauseRootCount = 10
)

func (c RRCEstablishmentCause) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return encodeRootEnumerated(w, enc, rrcEstablishmentCauseRootCount, int64(c), "RRCEstablishmentCause")
}

func (c *RRCEstablishmentCause) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	idx, err := decodeRootEnumerated(r, enc, rrcEstablishmentCauseRootCount, "RRCEstablishmentCause")
	if err != nil {
		return fmt.Errorf("ngap: rrc establishment cause: %w", err)
	}

	*c = RRCEstablishmentCause(idx)

	return nil
}

// UEContextRequest ::= ENUMERATED { requested, ... } (extensible). No S1AP
// counterpart: only NGAP lets the NG-RAN node ask for a UE context alongside
// the initial NAS message (TS 38.413 §9.2.5.1).
type UEContextRequest uint8

const (
	UEContextRequested UEContextRequest = iota

	ueContextRequestRootCount = 1
)

func (u UEContextRequest) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return encodeRootEnumerated(w, enc, ueContextRequestRootCount, int64(u), "UEContextRequest")
}

func (u *UEContextRequest) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	idx, err := decodeRootEnumerated(r, enc, ueContextRequestRootCount, "UEContextRequest")
	if err != nil {
		return err
	}

	*u = UEContextRequest(idx)

	return nil
}

// Cell identity widths of the two 3GPP-access CGIs (TS 38.413 §9.3.1.7,
// §9.3.1.9). S1AP has one, 28 bits wide.
const (
	EUTRACellIdentityBits = 28
	NRCellIdentityBits    = 36
)

// UserLocationInformation CHOICE alternatives (TS 38.413 §9.3.1.16). The CHOICE
// is closed by a choice-Extensions alternative rather than an extension marker,
// so the index is constrained across all four.
const (
	userLocationInformationEUTRA = iota
	userLocationInformationNR
	userLocationInformationN3IWF
	userLocationInformationChoiceExtensions

	userLocationInformationAlternatives = 4
)

// UserLocationInformationKind selects which CGI the cell identity belongs to.
type UserLocationInformationKind uint8

const (
	// UserLocationEUTRA is userLocationInformationEUTRA: EUTRA-CGI, 28 bits.
	UserLocationEUTRA UserLocationInformationKind = iota
	// UserLocationNR is userLocationInformationNR: NR-CGI, 36 bits.
	UserLocationNR
	// UserLocationN3IWF is userLocationInformationN3IWF-with-PortNumber: an IP
	// address and port instead of a cell.
	UserLocationN3IWF
)

// PortNumber ::= OCTET STRING (SIZE(2)).
type PortNumber uint16

// TimeStamp ::= OCTET STRING (SIZE(4)) — TS 38.413 §9.3.1.16. No S1AP
// counterpart: the S1AP User Location Information carries no timestamp.
type TimeStamp [4]byte

// UserLocationInformation ::= CHOICE { userLocationInformationEUTRA,
// userLocationInformationNR, userLocationInformationN3IWF-with-PortNumber,
// choice-Extensions } — TS 38.413 §9.3.1.16.
//
// The two 3GPP-access alternatives are an extensible SEQUENCE of a CGI, a TAI
// and an optional timeStamp, so they flatten into one struct: Kind says which
// CGI CellIdentity came from and therefore how wide it is. The N3IWF
// alternative reports an IP address and port instead, and uses IPAddress and
// PortNumber; Ella Core does not serve non-3GPP access, but the alternative is
// modeled so an N3IWF's messages decode rather than being rejected — the IE is
// mandatory-reject in INITIAL UE MESSAGE (§10.3.4.2).
//
// S1AP carries the same information as two separate mandatory IEs, E-UTRAN CGI
// and TAI, with no CHOICE and no timestamp.
type UserLocationInformation struct {
	Kind UserLocationInformationKind

	// Set for UserLocationEUTRA and UserLocationNR.
	PLMNIdentity PLMNIdentity
	// CellIdentity is right-aligned in the width Kind implies.
	CellIdentity uint64
	TAI          TAI
	TimeStamp    *TimeStamp

	// Set for UserLocationN3IWF.
	IPAddress  TransportLayerAddress
	PortNumber PortNumber
}

// NASPDU ::= OCTET STRING (unbounded), carried opaquely (TS 24.501).
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

// TAI ::= SEQUENCE { pLMNIdentity, tAC, iE-Extensions OPTIONAL } (extensible)
// — TS 38.413 §9.3.3.11.
type TAI struct {
	_            [0]struct{} `per:"extseq"`
	PLMNIdentity PLMNIdentity
	TAC          TAC
	_            ieExtensions `per:",skip"`
}

// PDUSessionID ::= INTEGER (0..255) — TS 38.413 §9.3.1.50.
type PDUSessionID uint8

func (p PDUSessionID) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return per.EncodeInteger(w, enc, per.Bounds{LB: 0, HasLB: true, UB: 255, HasUB: true}, int64(p))
}

func (p *PDUSessionID) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	v, err := per.DecodeInteger(r, enc, per.Bounds{LB: 0, HasLB: true, UB: 255, HasUB: true})
	if err != nil {
		return err
	}

	*p = PDUSessionID(v)

	return nil
}

// BitRate ::= INTEGER (0..4000000000000, ...) (extensible, where S1AP's is a
// plain INTEGER (0..10000000000)).
const bitRateMax = 4000000000000

type BitRate uint64

func (b BitRate) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return encodeExtensibleInt(w, enc, 0, bitRateMax, int64(b))
}

func (b *BitRate) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	v, err := decodeExtensibleInt(r, enc, 0, bitRateMax, "BitRate")
	if err != nil {
		return err
	}

	*b = BitRate(v)

	return nil
}

// UEAggregateMaximumBitRate ::= SEQUENCE { ...DL, ...UL, iE-Extensions OPTIONAL }
// (extensible) — TS 38.413 §9.3.1.58.
type UEAggregateMaximumBitRate struct {
	_  [0]struct{} `per:"extseq"`
	DL BitRate
	UL BitRate
	_  ieExtensions `per:",skip"`
}

// SecurityKey ::= BIT STRING (SIZE(256)) — the K_gNB.
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

// UESecurityCapabilities ::= SEQUENCE { four algorithm bit strings,
// iE-Extensions OPTIONAL } (extensible) — TS 38.413 §9.3.1.86. Each field is
// BIT STRING (SIZE(16, ...)). S1AP carries only the E-UTRA pair.
type UESecurityCapabilities struct {
	NREncryptionAlgorithms             uint16
	NRIntegrityProtectionAlgorithms    uint16
	EUTRAEncryptionAlgorithms          uint16
	EUTRAIntegrityProtectionAlgorithms uint16
}

func (c UESecurityCapabilities) MarshalPER(w *per.Writer, enc per.Encoding) error {
	w.WriteBit(false)
	w.WriteBit(false)

	for _, v := range [4]uint16{
		c.NREncryptionAlgorithms, c.NRIntegrityProtectionAlgorithms,
		c.EUTRAEncryptionAlgorithms, c.EUTRAIntegrityProtectionAlgorithms,
	} {
		if err := per.EncodeBitString(w, enc, 16, 16, true, true, true, uintToBits(uint64(v), 16), 16); err != nil {
			return err
		}
	}

	return nil
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

	var algs [4]uint16

	for i := range algs {
		b, nbits, err := per.DecodeBitString(r, enc, 16, 16, true, true, true)
		if err != nil {
			return err
		}

		algs[i] = uint16(bitsToUint(b, nbits))
	}

	if err := skipSequenceExtensionsPER(r, enc, extContainer, extBit); err != nil {
		return err
	}

	*c = UESecurityCapabilities{
		NREncryptionAlgorithms:             algs[0],
		NRIntegrityProtectionAlgorithms:    algs[1],
		EUTRAEncryptionAlgorithms:          algs[2],
		EUTRAIntegrityProtectionAlgorithms: algs[3],
	}

	return nil
}

// UERadioCapability ::= OCTET STRING (unconstrained).
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
