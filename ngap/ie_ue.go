// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"github.com/ellanetworks/core/per"
	"github.com/ellanetworks/core/per/ap"
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
	RRCCauseNotAvailable
	RRCCauseMOExceptionData
)

// UEContextRequest ::= ENUMERATED { requested, ... } (extensible). No S1AP
// counterpart: only NGAP lets the NG-RAN node ask for a UE context alongside
// the initial NAS message (TS 38.413 §9.2.5.1).
type UEContextRequest uint8

const (
	UEContextRequested UEContextRequest = iota
)

type GWContextReleaseIndication uint8

const (
	GWContextReleaseIndicationTrue GWContextReleaseIndication = iota
)

// Cell identity widths of the two 3GPP-access CGIs (TS 38.413 §9.3.1.7,
// §9.3.1.9). S1AP has one, 28 bits wide.
const (
	EUTRACellIdentityBits = 28
	NRCellIdentityBits    = 36
)

// PortNumber ::= OCTET STRING (SIZE(2)).
type PortNumber uint16

func (p PortNumber) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return per.EncodeOctetString(w, enc, 2, 2, true, true, false, []byte{byte(p >> 8), byte(p)})
}

func (p *PortNumber) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	b, err := per.DecodeOctetString(r, enc, 2, 2, true, true, false)
	if err != nil {
		return err
	}

	*p = PortNumber(uint16(b[0])<<8 | uint16(b[1]))

	return nil
}

// TimeStamp ::= OCTET STRING (SIZE(4)) — TS 38.413 §9.3.1.75. No S1AP
// counterpart: the S1AP User Location Information carries no timestamp.
type TimeStamp [4]byte

func (t TimeStamp) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return per.EncodeOctetString(w, enc, 4, 4, true, true, false, t[:])
}

func (t *TimeStamp) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	b, err := per.DecodeOctetString(r, enc, 4, 4, true, true, false)
	if err != nil {
		return err
	}

	copy(t[:], b)

	return nil
}

// UserLocationInformation ::= CHOICE { userLocationInformationEUTRA,
// userLocationInformationNR, userLocationInformationN3IWF-with-PortNumber,
// choice-Extensions } — TS 38.413 §9.3.1.16.
//
// S1AP carries the same information as two separate mandatory IEs, E-UTRAN CGI
// and TAI, with no CHOICE and no timestamp.
type UserLocationInformation struct {
	EUTRA                  *UserLocationInformationEUTRA                  `per:",choice:0"`
	NR                     *UserLocationInformationNR                     `per:",choice:1"`
	N3IWF                  *UserLocationInformationN3IWFWithPortNumber    `per:",choice:2"`
	TNGF                   *UserLocationInformationTNGF                   `per:",choice:3,extie:IDUserLocationInformationTNGF,crit:ignore"`
	TWIF                   *UserLocationInformationTWIF                   `per:",choice:3,extie:IDUserLocationInformationTWIF,crit:ignore"`
	WAGF                   *UserLocationInformationWAGF                   `per:",choice:3,extie:IDUserLocationInformationWAGF,crit:ignore"`
	N3IWFWithoutPortNumber *UserLocationInformationN3IWFWithoutPortNumber `per:",choice:3,extie:IDUserLocationInformationN3IWFwithoutPortNumber,crit:ignore"`
	ChoiceExtension        *SingleContainer                               `per:",choice:3,extie"`
	choiceCriticality      ap.WireCriticality                             `per:",choicecrit"`
}

type UserLocationInformationEUTRA struct {
	EUTRACGI          EUTRACGI
	TAI               TAI
	TimeStamp         *TimeStamp             `per:",optional"`
	PSCellInformation *NGRANCGI              `per:",extie:IDPSCellInformation,crit:ignore"`
	iEExtensions      extensionMeta          `per:",exttable"`
	extensions        per.ExtensionAdditions `per:",extraw"`
}

type UserLocationInformationNR struct {
	NRCGI                              NRCGI
	TAI                                TAI
	TimeStamp                          *TimeStamp                          `per:",optional"`
	PSCellInformation                  *NGRANCGI                           `per:",extie:IDPSCellInformation,crit:ignore"`
	NID                                *NID                                `per:",extie:IDNID,crit:reject"`
	NRNTNTAIInformation                *NRNTNTAIInformation                `per:",extie:IDNRNTNTAIInformation,crit:ignore"`
	MobileIABMTUserLocationInformation *MobileIABMTUserLocationInformation `per:",extie:IDMobileIABMTUserLocationInformation,crit:ignore"`
	AdditionalULI                      *AdditionalULI                      `per:",extie:IDAdditionalULI,crit:ignore"`
	AerialUEFlightInformationReporting *AerialUEFlightInformationReporting `per:",extie:IDAerialUEFlightInformationReporting,crit:ignore"`
	iEExtensions                       extensionMeta                       `per:",exttable"`
	extensions                         per.ExtensionAdditions              `per:",extraw"`
}

type UserLocationInformationN3IWFWithPortNumber struct {
	IPAddress    TransportLayerAddress
	PortNumber   PortNumber
	TAI          *TAI                   `per:",extie:IDTAI,crit:ignore"`
	iEExtensions extensionMeta          `per:",exttable"`
	extensions   per.ExtensionAdditions `per:",extraw"`
}

type UserLocationInformationTNGF struct {
	TNAPID       TNAPID
	IPAddress    TransportLayerAddress
	PortNumber   *PortNumber            `per:",optional"`
	TAI          *TAI                   `per:",extie:IDTAI,crit:ignore"`
	iEExtensions extensionMeta          `per:",exttable"`
	extensions   per.ExtensionAdditions `per:",extraw"`
}

type UserLocationInformationTWIF struct {
	TWAPID       TWAPID
	IPAddress    TransportLayerAddress
	PortNumber   *PortNumber            `per:",optional"`
	TAI          *TAI                   `per:",extie:IDTAI,crit:ignore"`
	iEExtensions extensionMeta          `per:",exttable"`
	extensions   per.ExtensionAdditions `per:",extraw"`
}

type UserLocationInformationWAGF struct {
	GlobalLineID      *GlobalLineID      `per:",choice:0"`
	HFCNodeID         *HFCNodeID         `per:",choice:1"`
	GlobalCableID     *GlobalCableID     `per:",choice:2,extie:IDGlobalCableID,crit:ignore"`
	HFCNodeIDNew      *HFCNodeIDNew      `per:",choice:2,extie:IDHFCNodeIDnew,crit:ignore"`
	GlobalCableIDNew  *GlobalCableIDNew  `per:",choice:2,extie:IDGlobalCableIDnew,crit:ignore"`
	ChoiceExtension   *SingleContainer   `per:",choice:2,extie"`
	choiceCriticality ap.WireCriticality `per:",choicecrit"`
}

type UserLocationInformationN3IWFWithoutPortNumber struct {
	IPAddress    TransportLayerAddress
	TAI          *TAI                   `per:",optional"`
	iEExtensions extensionMeta          `per:",exttable"`
	extensions   per.ExtensionAdditions `per:",extraw"`
}

type GlobalLineID struct {
	GlobalLineIdentity GlobalLineIdentity
	LineType           *LineType              `per:",optional"`
	TAI                *TAI                   `per:",extie:IDTAI,crit:ignore"`
	iEExtensions       extensionMeta          `per:",exttable"`
	extensions         per.ExtensionAdditions `per:",extraw"`
}

type LineType uint8

const (
	LineTypeDSL LineType = iota
	LineTypePON
)

type HFCNodeIDNew struct {
	HFCNodeID     HFCNodeID
	TAI           TAI
	GlobalCableID GlobalCableID          `per:",extie:IDGlobalCableID,crit:ignore"`
	iEExtensions  extensionMeta          `per:",exttable"`
	extensions    per.ExtensionAdditions `per:",extraw"`
}

type GlobalCableIDNew struct {
	GlobalCableID GlobalCableID
	TAI           TAI
	iEExtensions  extensionMeta          `per:",exttable"`
	extensions    per.ExtensionAdditions `per:",extraw"`
}

type NRNTNTAIInformation struct {
	ServingPLMN                 PLMNIdentity
	TACListInNRNTN              TACListInNRNTN
	UELocationDerivedTACInNRNTN *TAC                   `per:",optional"`
	iEExtensions                extensionMeta          `per:",exttable"`
	extensions                  per.ExtensionAdditions `per:",extraw"`
}

const maxnoofTACsinNTN = 12

type TACListInNRNTN []TAC

func (l TACListInNRNTN) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return marshalSeqOf(w, enc, 1, maxnoofTACsinNTN, []TAC(l))
}

func (l *TACListInNRNTN) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	items, err := unmarshalSeqOf[TAC](r, enc, 1, maxnoofTACsinNTN)
	if err != nil {
		return err
	}

	*l = items

	return nil
}

type MobileIABMTUserLocationInformation struct {
	NRCGI        NRCGI
	TAI          TAI
	iEExtensions extensionMeta          `per:",exttable"`
	extensions   per.ExtensionAdditions `per:",extraw"`
}

type AdditionalULI struct {
	NRCGI        NRCGI
	TAI          TAI
	iEExtensions extensionMeta          `per:",exttable"`
	extensions   per.ExtensionAdditions `per:",extraw"`
}

type AerialUEFlightInformationReporting struct {
	Altitude     Altitude `per:",range:-420..10000,..."`
	TimeStamp    TimeStamp
	iEExtensions extensionMeta          `per:",exttable"`
	extensions   per.ExtensionAdditions `per:",extraw"`
}

type Altitude int32

type TNAPID []byte

func (v TNAPID) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return per.EncodeOctetString(w, enc, 0, 0, true, false, false, v)
}

func (v *TNAPID) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	b, err := per.DecodeOctetString(r, enc, 0, 0, true, false, false)
	*v = b

	return err
}

type TWAPID []byte

func (v TWAPID) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return per.EncodeOctetString(w, enc, 0, 0, true, false, false, v)
}

func (v *TWAPID) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	b, err := per.DecodeOctetString(r, enc, 0, 0, true, false, false)
	*v = b

	return err
}

type GlobalLineIdentity []byte

func (v GlobalLineIdentity) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return per.EncodeOctetString(w, enc, 0, 0, true, false, false, v)
}

func (v *GlobalLineIdentity) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	b, err := per.DecodeOctetString(r, enc, 0, 0, true, false, false)
	*v = b

	return err
}

type HFCNodeID []byte

func (v HFCNodeID) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return per.EncodeOctetString(w, enc, 0, 0, true, false, false, v)
}

func (v *HFCNodeID) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	b, err := per.DecodeOctetString(r, enc, 0, 0, true, false, false)
	*v = b

	return err
}

type GlobalCableID []byte

func (v GlobalCableID) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return per.EncodeOctetString(w, enc, 0, 0, true, false, false, v)
}

func (v *GlobalCableID) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	b, err := per.DecodeOctetString(r, enc, 0, 0, true, false, false)
	*v = b

	return err
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
	extensions   per.ExtensionAdditions `per:",extraw"`
	PLMNIdentity PLMNIdentity
	TAC          TAC
	iEExtensions extensionMeta `per:",exttable"`
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
	v, err := decodeExtensibleInt[BitRate](r, enc, 0, bitRateMax)
	if err != nil {
		return err
	}

	*b = v

	return nil
}

func (v BitRate) Diagnose(*ap.Diagnosis[ProtocolIEID]) bool {
	return outsideRoot(int64(v), 0, bitRateMax)
}

// UEAggregateMaximumBitRate ::= SEQUENCE { ...DL, ...UL, iE-Extensions OPTIONAL }
// (extensible) — TS 38.413 §9.3.1.58.
type UEAggregateMaximumBitRate struct {
	extensions   per.ExtensionAdditions `per:",extraw"`
	DL           BitRate
	UL           BitRate
	iEExtensions extensionMeta `per:",exttable"`
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
	NREncryptionAlgorithms             SecurityAlgorithms
	NRIntegrityProtectionAlgorithms    SecurityAlgorithms
	EUTRAEncryptionAlgorithms          SecurityAlgorithms
	EUTRAIntegrityProtectionAlgorithms SecurityAlgorithms
	iEExtensions                       extensionMeta          `per:",exttable"`
	extensions                         per.ExtensionAdditions `per:",extraw"`
}

const securityAlgorithmsBits = 16

type SecurityAlgorithms struct {
	Value    uint16
	extended bool
	raw      BitString
}

func SecurityAlgorithmsFrom(v uint16) SecurityAlgorithms { return SecurityAlgorithms{Value: v} }

func (a SecurityAlgorithms) Uint16() uint16 { return a.Value }

func (a SecurityAlgorithms) MarshalPER(w *per.Writer, enc per.Encoding) error {
	if a.extended {
		return a.raw.encode(w, enc, securityAlgorithmsBits, securityAlgorithmsBits)
	}

	return BitStringFromUint(uint64(a.Value), securityAlgorithmsBits).encode(w, enc, securityAlgorithmsBits, securityAlgorithmsBits)
}

func (a *SecurityAlgorithms) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	b, err := decodeBitString(r, enc, securityAlgorithmsBits, securityAlgorithmsBits)
	if err != nil {
		return err
	}

	*a = SecurityAlgorithms{Value: uint16(BitStringFrom(append(b.Bytes(), 0, 0), securityAlgorithmsBits).Uint64())}
	if b.Bits() != securityAlgorithmsBits {
		a.extended, a.raw = true, b
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
