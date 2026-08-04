// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"fmt"

	"github.com/ellanetworks/core/per"
)

// PriorityLevelARP ::= INTEGER (1..15) — TS 38.413 §9.3.1.19. S1AP's
// counterpart is PriorityLevel ::= INTEGER (0..15), so the two encode over
// different ranges.
type PriorityLevelARP uint8

func (p PriorityLevelARP) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return per.EncodeConstrainedWholeNumber(w, enc, 1, 15, int64(p))
}

func (p *PriorityLevelARP) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	v, err := per.DecodeConstrainedWholeNumber(r, enc, 1, 15)
	if err != nil {
		return err
	}

	*p = PriorityLevelARP(v)

	return nil
}

// Pre-emptionCapability ::= ENUMERATED { shall-not-trigger-pre-emption,
// may-trigger-pre-emption, ... }. Extensible, where S1AP's is not.
type PreemptionCapability uint8

const (
	PreemptionShallNotTrigger PreemptionCapability = iota
	PreemptionMayTrigger

	preemptionCapabilityRootCount = 2
)

// Pre-emptionVulnerability ::= ENUMERATED { not-pre-emptable, pre-emptable,
// ... }. Extensible, where S1AP's is not.
type PreemptionVulnerability uint8

const (
	PreemptionNotPreemptable PreemptionVulnerability = iota
	PreemptionPreemptable

	preemptionVulnerabilityRootCount = 2
)

// NotificationControl ::= ENUMERATED { notification-requested, ... }.
type NotificationControl uint8

const (
	NotificationRequested NotificationControl = iota

	notificationControlRootCount = 1
)

// PacketLossRate ::= INTEGER (0..1000, ...) in tenths of a percent.
type PacketLossRate uint16

func (p PacketLossRate) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return encodeExtensibleInt(w, enc, 0, 1000, int64(p))
}

func (p *PacketLossRate) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	v, err := decodeExtensibleInt(r, enc, 0, 1000, "PacketLossRate")
	if err != nil {
		return err
	}

	*p = PacketLossRate(v)

	return nil
}

// FiveQI ::= INTEGER (0..255, ...). 3GPP gives 5QI no section of its own; it
// is the first row of TS 38.413 §9.3.1.28. S1AP's counterpart
// is QCI ::= INTEGER (0..255), which carries no extension marker, so the two
// differ by one leading bit on the wire.
type FiveQI uint8

func (f FiveQI) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return encodeExtensibleInt(w, enc, 0, 255, int64(f))
}

func (f *FiveQI) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	v, err := decodeExtensibleInt(r, enc, 0, 255, "FiveQI")
	if err != nil {
		return err
	}

	*f = FiveQI(v)

	return nil
}

// PriorityLevelQos ::= INTEGER (1..127, ...) — TS 38.413 §9.3.1.84. Distinct
// from PriorityLevelARP: this one ranks QoS flows, that one ranks allocation.
type PriorityLevelQos uint8

func (p PriorityLevelQos) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return encodeExtensibleInt(w, enc, 1, 127, int64(p))
}

func (p *PriorityLevelQos) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	v, err := decodeExtensibleInt(r, enc, 1, 127, "PriorityLevelQos")
	if err != nil {
		return err
	}

	*p = PriorityLevelQos(v)

	return nil
}

// PacketDelayBudget ::= INTEGER (0..1023, ...) in units of 0.5 ms.
type PacketDelayBudget uint16

func (p PacketDelayBudget) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return encodeExtensibleInt(w, enc, 0, 1023, int64(p))
}

func (p *PacketDelayBudget) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	v, err := decodeExtensibleInt(r, enc, 0, 1023, "PacketDelayBudget")
	if err != nil {
		return err
	}

	*p = PacketDelayBudget(v)

	return nil
}

// AveragingWindow ::= INTEGER (0..4095, ...) in milliseconds.
type AveragingWindow uint16

func (a AveragingWindow) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return encodeExtensibleInt(w, enc, 0, 4095, int64(a))
}

func (a *AveragingWindow) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	v, err := decodeExtensibleInt(r, enc, 0, 4095, "AveragingWindow")
	if err != nil {
		return err
	}

	*a = AveragingWindow(v)

	return nil
}

// MaximumDataBurstVolume ::= INTEGER (0..4095, ..., 4096..2000000) in bytes.
// Unlike its neighbours this constraint defines an extension range, so values
// above the root are legitimate and encode unconstrained after a set extension
// bit — rejecting them would drop conformant peers.
type MaximumDataBurstVolume uint32

const (
	maximumDataBurstVolumeRootUB = 4095
	maximumDataBurstVolumeUB     = 2000000
)

func (m MaximumDataBurstVolume) MarshalPER(w *per.Writer, enc per.Encoding) error {
	if m > maximumDataBurstVolumeUB {
		return fmt.Errorf("ngap: MaximumDataBurstVolume %d exceeds the extension range ceiling %d", uint32(m), maximumDataBurstVolumeUB)
	}

	return per.EncodeInteger(w, enc, per.Bounds{
		LB: 0, HasLB: true, UB: maximumDataBurstVolumeRootUB, HasUB: true, Extensible: true,
	}, int64(m))
}

// The extension branch decodes as an unconstrained integer, so the value
// arrives unbounded and signed; without this check a negative would land in
// the unsigned Go type as a very large burst volume.
func (m *MaximumDataBurstVolume) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	v, err := per.DecodeInteger(r, enc, per.Bounds{
		LB: 0, HasLB: true, UB: maximumDataBurstVolumeRootUB, HasUB: true, Extensible: true,
	})
	if err != nil {
		return err
	}

	if v < 0 || v > maximumDataBurstVolumeUB {
		return fmt.Errorf("ngap: MaximumDataBurstVolume %d outside 0..%d", v, maximumDataBurstVolumeUB)
	}

	*m = MaximumDataBurstVolume(v)

	return nil
}

// DelayCritical ::= ENUMERATED { delay-critical, non-delay-critical, ... }.
type DelayCritical uint8

const (
	DelayCriticalYes DelayCritical = iota
	DelayCriticalNo

	delayCriticalRootCount = 2
)

// ReflectiveQosAttribute ::= ENUMERATED { subject-to, ... }.
type ReflectiveQosAttribute uint8

const (
	ReflectiveQosSubjectTo ReflectiveQosAttribute = iota

	reflectiveQosAttributeRootCount = 1
)

// AdditionalQosFlowInformation ::= ENUMERATED { more-likely, ... }.
type AdditionalQosFlowInformation uint8

const (
	AdditionalQosFlowMoreLikely AdditionalQosFlowInformation = iota

	additionalQosFlowInformationRootCount = 1
)

// PacketErrorRate ::= SEQUENCE { pERScalar INTEGER (0..9, ...), pERExponent
// INTEGER (0..9, ...), iE-Extensions OPTIONAL } (extensible) — the rate is
// scalar × 10^-exponent.
type PacketErrorRate struct {
	_        [0]struct{} `per:"extseq"`
	Scalar   PERDigit
	Exponent PERDigit
	_        ieExtensions `per:",skip"`
}

// PERDigit is the shared INTEGER (0..9, ...) of both PacketErrorRate fields.
type PERDigit uint8

func (d PERDigit) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return encodeExtensibleInt(w, enc, 0, 9, int64(d))
}

func (d *PERDigit) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	v, err := decodeExtensibleInt(r, enc, 0, 9, "PacketErrorRate digit")
	if err != nil {
		return err
	}

	*d = PERDigit(v)

	return nil
}

// NonDynamic5QIDescriptor ::= SEQUENCE { fiveQI, priorityLevelQos OPTIONAL,
// averagingWindow OPTIONAL, maximumDataBurstVolume OPTIONAL, iE-Extensions
// OPTIONAL } (extensible) — TS 38.413 §9.3.1.28.
type NonDynamic5QIDescriptor struct {
	_                      [0]struct{} `per:"extseq"`
	FiveQI                 FiveQI
	PriorityLevelQos       *PriorityLevelQos       `per:",optional"`
	AveragingWindow        *AveragingWindow        `per:",optional"`
	MaximumDataBurstVolume *MaximumDataBurstVolume `per:",optional"`
	_                      ieExtensions            `per:",skip"`
}

// Dynamic5QIDescriptor ::= SEQUENCE { priorityLevelQos, packetDelayBudget,
// packetErrorRate, fiveQI OPTIONAL, delayCritical OPTIONAL, averagingWindow
// OPTIONAL, maximumDataBurstVolume OPTIONAL, iE-Extensions OPTIONAL }
// (extensible) — TS 38.413 §9.3.1.18. The spec marks delayCritical,
// averagingWindow and maximumDataBurstVolume as present for a GBR QoS flow.
type Dynamic5QIDescriptor struct {
	_                      [0]struct{} `per:"extseq"`
	PriorityLevelQos       PriorityLevelQos
	PacketDelayBudget      PacketDelayBudget
	PacketErrorRate        PacketErrorRate
	FiveQI                 *FiveQI                 `per:",optional"`
	DelayCritical          *DelayCritical          `per:"ENUMERATED,range:0..1,...,optional"`
	AveragingWindow        *AveragingWindow        `per:",optional"`
	MaximumDataBurstVolume *MaximumDataBurstVolume `per:",optional"`
	_                      ieExtensions            `per:",skip"`
}

// QosCharacteristicsKind selects the QosCharacteristics CHOICE alternative.
type QosCharacteristicsKind uint8

const (
	QosCharacteristicsNonDynamic5QI QosCharacteristicsKind = iota
	QosCharacteristicsDynamic5QI

	qosCharacteristicsChoiceExtensions = 2
	qosCharacteristicsAlternatives     = 3
)

// QosCharacteristics ::= CHOICE { nonDynamic5QI, dynamic5QI, choice-Extensions
// } — the CHOICE row of TS 38.413 §9.3.1.12, which has no section of its
// own. The two descriptors keep their own types rather
// than merging into shared fields: fiveQI and priorityLevelQos swap between
// mandatory and optional across the alternatives, and flattening would lose
// that. S1AP has no counterpart — an E-RAB carries a bare QCI.
type QosCharacteristics struct {
	Kind          QosCharacteristicsKind
	NonDynamic5QI NonDynamic5QIDescriptor
	Dynamic5QI    Dynamic5QIDescriptor
}

func (q QosCharacteristics) MarshalPER(w *per.Writer, enc per.Encoding) error {
	if q.Kind > QosCharacteristicsDynamic5QI {
		return fmt.Errorf("ngap: invalid QosCharacteristics kind %d", q.Kind)
	}

	if err := per.EncodeConstrainedWholeNumber(w, enc, 0, qosCharacteristicsAlternatives-1, int64(q.Kind)); err != nil {
		return err
	}

	if q.Kind == QosCharacteristicsNonDynamic5QI {
		return q.NonDynamic5QI.MarshalPER(w, enc)
	}

	return q.Dynamic5QI.MarshalPER(w, enc)
}

func (q *QosCharacteristics) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	idx, err := per.DecodeConstrainedWholeNumber(r, enc, 0, qosCharacteristicsAlternatives-1)
	if err != nil {
		return err
	}

	if idx == qosCharacteristicsChoiceExtensions {
		return decodeChoiceExtension(r, enc, "QosCharacteristics")
	}

	q.Kind = QosCharacteristicsKind(idx)

	if q.Kind == QosCharacteristicsNonDynamic5QI {
		return q.NonDynamic5QI.UnmarshalPER(r, enc)
	}

	return q.Dynamic5QI.UnmarshalPER(r, enc)
}

// QosFlowLevelQosParameters ::= SEQUENCE { qosCharacteristics,
// allocationAndRetentionPriority, gBR-QosInformation OPTIONAL,
// reflectiveQosAttribute OPTIONAL, additionalQosFlowInformation OPTIONAL,
// iE-Extensions OPTIONAL } (extensible) — TS 38.413 §9.3.1.12. S1AP's
// E-RABLevelQoSParameters is the counterpart: a QCI where this has a
// QosCharacteristics CHOICE, and no reflective/additional fields.
type QosFlowLevelQosParameters struct {
	_                              [0]struct{} `per:"extseq"`
	QosCharacteristics             QosCharacteristics
	AllocationAndRetentionPriority AllocationAndRetentionPriority
	GBRQosInformation              *GBRQosInformation            `per:",optional"`
	ReflectiveQosAttribute         *ReflectiveQosAttribute       `per:"ENUMERATED,range:0..0,...,optional"`
	AdditionalQosFlowInformation   *AdditionalQosFlowInformation `per:"ENUMERATED,range:0..0,...,optional"`
	_                              ieExtensions                  `per:",skip"`
}

// AllocationAndRetentionPriority ::= SEQUENCE { priorityLevelARP,
// pre-emptionCapability, pre-emptionVulnerability, iE-Extensions OPTIONAL }
// (extensible) — TS 38.413 §9.3.1.19.
type AllocationAndRetentionPriority struct {
	_                       [0]struct{} `per:"extseq"`
	PriorityLevelARP        PriorityLevelARP
	PreemptionCapability    PreemptionCapability    `per:"ENUMERATED,range:0..1,..."`
	PreemptionVulnerability PreemptionVulnerability `per:"ENUMERATED,range:0..1,..."`
	_                       ieExtensions            `per:",skip"`
}

// GBR-QosInformation ::= SEQUENCE { four BitRates, notificationControl
// OPTIONAL, maximumPacketLossRateDL OPTIONAL, maximumPacketLossRateUL
// OPTIONAL, iE-Extensions OPTIONAL } (extensible) — TS 38.413 §9.3.1.10,
// where 3GPP names the IE GBR QoS Flow Information. S1AP's GBR-QosInformation
// carries only the four bit rates; its packet loss rate sits one level up, as
// an optional extension on E-RABLevelQoSParameters, and is non-extensible
// there where NGAP's PacketLossRate is extensible.
type GBRQosInformation struct {
	_                       [0]struct{} `per:"extseq"`
	MaximumFlowBitRateDL    BitRate
	MaximumFlowBitRateUL    BitRate
	GuaranteedFlowBitRateDL BitRate
	GuaranteedFlowBitRateUL BitRate
	NotificationControl     *NotificationControl `per:"ENUMERATED,range:0..0,...,optional"`
	MaximumPacketLossRateDL *PacketLossRate      `per:",optional"`
	MaximumPacketLossRateUL *PacketLossRate      `per:",optional"`
	_                       ieExtensions         `per:",skip"`
}

// QosFlowIdentifier ::= INTEGER (0..63, ...) — TS 38.413 §9.3.1.51.
type QosFlowIdentifier uint8

func (q QosFlowIdentifier) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return encodeExtensibleInt(w, enc, 0, 63, int64(q))
}

func (q *QosFlowIdentifier) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	v, err := decodeExtensibleInt(r, enc, 0, 63, "QosFlowIdentifier")
	if err != nil {
		return err
	}

	*q = QosFlowIdentifier(v)

	return nil
}

// ERABID ::= INTEGER (0..15, ...) — TS 38.413 §9.3.2.3. NGAP carries an E-RAB
// ID so a QoS flow can be tied to the EPS bearer it maps to during
// interworking; the definition matches TS 36.413 §9.2.1.2 exactly.
type ERABID uint8

func (id ERABID) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return encodeExtensibleInt(w, enc, 0, 15, int64(id))
}

func (id *ERABID) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	v, err := decodeExtensibleInt(r, enc, 0, 15, "E-RAB-ID")
	if err != nil {
		return err
	}

	*id = ERABID(v)

	return nil
}

// QosFlowMappingIndication ::= ENUMERATED { ul, dl, ... }, inlined in
// AssociatedQosFlowItem.
type QosFlowMappingIndication uint8

const (
	QosFlowMappingUL QosFlowMappingIndication = iota
	QosFlowMappingDL

	qosFlowMappingIndicationRootCount = 2
)

// QosFlowSetupRequestItem ::= SEQUENCE { qosFlowIdentifier,
// qosFlowLevelQosParameters, e-RAB-ID OPTIONAL, iE-Extensions OPTIONAL }
// (extensible).
type QosFlowSetupRequestItem struct {
	_                         [0]struct{} `per:"extseq"`
	QosFlowIdentifier         QosFlowIdentifier
	QosFlowLevelQosParameters QosFlowLevelQosParameters
	ERABID                    *ERABID      `per:",optional"`
	_                         ieExtensions `per:",skip"`
}

// QosFlowSetupRequestList ::= SEQUENCE (SIZE(1..maxnoofQosFlows)) OF
// QosFlowSetupRequestItem.
type QosFlowSetupRequestList []QosFlowSetupRequestItem

// QosFlowWithCauseItem ::= SEQUENCE { qosFlowIdentifier, cause, iE-Extensions
// OPTIONAL } (extensible).
type QosFlowWithCauseItem struct {
	_                 [0]struct{} `per:"extseq"`
	QosFlowIdentifier QosFlowIdentifier
	Cause             Cause
	_                 ieExtensions `per:",skip"`
}

// QosFlowListWithCause ::= SEQUENCE (SIZE(1..maxnoofQosFlows)) OF
// QosFlowWithCauseItem.
type QosFlowListWithCause []QosFlowWithCauseItem

// AssociatedQosFlowItem ::= SEQUENCE { qosFlowIdentifier,
// qosFlowMappingIndication OPTIONAL, iE-Extensions OPTIONAL } (extensible).
type AssociatedQosFlowItem struct {
	_                        [0]struct{} `per:"extseq"`
	QosFlowIdentifier        QosFlowIdentifier
	QosFlowMappingIndication *QosFlowMappingIndication `per:"ENUMERATED,range:0..1,...,optional"`
	_                        ieExtensions              `per:",skip"`
}

// AssociatedQosFlowList ::= SEQUENCE (SIZE(1..maxnoofQosFlows)) OF
// AssociatedQosFlowItem.
type AssociatedQosFlowList []AssociatedQosFlowItem

// QosFlowPerTNLInformation ::= SEQUENCE { uPTransportLayerInformation,
// associatedQosFlowList, iE-Extensions OPTIONAL } (extensible) — TS 38.413
// §9.3.2.8.
type QosFlowPerTNLInformation struct {
	_                           [0]struct{} `per:"extseq"`
	UPTransportLayerInformation UPTransportLayerInformation
	AssociatedQosFlowList       AssociatedQosFlowList
	_                           ieExtensions `per:",skip"`
}

// QosFlowPerTNLInformationItem ::= SEQUENCE { qosFlowPerTNLInformation,
// iE-Extensions OPTIONAL } (extensible).
type QosFlowPerTNLInformationItem struct {
	_                        [0]struct{} `per:"extseq"`
	QosFlowPerTNLInformation QosFlowPerTNLInformation
	_                        ieExtensions `per:",skip"`
}

// QosFlowPerTNLInformationList ::= SEQUENCE
// (SIZE(1..maxnoofMultiConnectivityMinusOne)) OF QosFlowPerTNLInformationItem.
// The bound is one less than maxnoofMultiConnectivity: the list carries the
// additional legs, not the first.
type QosFlowPerTNLInformationList []QosFlowPerTNLInformationItem

// PDUSessionType ::= ENUMERATED { ipv4, ipv6, ipv4v6, ethernet, unstructured,
// ... } — TS 38.413 §9.3.1.52. No S1AP counterpart: EPS carries the PDN type
// in NAS rather than S1AP.
type PDUSessionType uint8

const (
	PDUSessionTypeIPv4 PDUSessionType = iota
	PDUSessionTypeIPv6
	PDUSessionTypeIPv4v6
	PDUSessionTypeEthernet
	PDUSessionTypeUnstructured

	pduSessionTypeRootCount = 5
)

func (p PDUSessionType) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return encodeRootEnumerated(w, enc, pduSessionTypeRootCount, int64(p), "PDUSessionType")
}

func (p *PDUSessionType) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	idx, err := decodeRootEnumerated(r, enc, pduSessionTypeRootCount, "PDUSessionType")
	if err != nil {
		return err
	}

	*p = PDUSessionType(idx)

	return nil
}

// NetworkInstance ::= INTEGER (1..256, ...) — TS 38.413 §9.3.1.113.
type NetworkInstance uint16

func (n NetworkInstance) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return encodeExtensibleInt(w, enc, 1, 256, int64(n))
}

func (n *NetworkInstance) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	v, err := decodeExtensibleInt(r, enc, 1, 256, "NetworkInstance")
	if err != nil {
		return err
	}

	*n = NetworkInstance(v)

	return nil
}

// PDUSessionAggregateMaximumBitRate ::= SEQUENCE { DL, UL, iE-Extensions
// OPTIONAL } (extensible) — TS 38.413 §9.3.1.102. Session-level, where
// UEAggregateMaximumBitRate is UE-level; S1AP has only the UE-level form.
type PDUSessionAggregateMaximumBitRate struct {
	_  [0]struct{} `per:"extseq"`
	DL BitRate
	UL BitRate
	_  ieExtensions `per:",skip"`
}

// IntegrityProtectionIndication ::= ENUMERATED { required, preferred,
// not-needed, ... }. An inline row of TS 38.413 §9.3.1.27 rather than a
// section of its own. TS 36.413 defines it identically for EPS user plane
// integrity protection.
type IntegrityProtectionIndication uint8

const (
	IntegrityProtectionRequired IntegrityProtectionIndication = iota
	IntegrityProtectionPreferred
	IntegrityProtectionNotNeeded

	integrityProtectionIndicationRootCount = 3
)

// ConfidentialityProtectionIndication ::= ENUMERATED { required, preferred,
// not-needed, ... }. NGAP-only: TS 36.413 has the Security Indication IE but
// no confidentiality field in it.
type ConfidentialityProtectionIndication uint8

const (
	ConfidentialityProtectionRequired ConfidentialityProtectionIndication = iota
	ConfidentialityProtectionPreferred
	ConfidentialityProtectionNotNeeded

	confidentialityProtectionIndicationRootCount = 3
)

// IntegrityProtectionResult ::= ENUMERATED { performed, not-performed, ... }.
// TS 36.413 defines this identically.
type IntegrityProtectionResult uint8

const (
	IntegrityProtectionPerformed IntegrityProtectionResult = iota
	IntegrityProtectionNotPerformed

	integrityProtectionResultRootCount = 2
)

// ConfidentialityProtectionResult ::= ENUMERATED { performed, not-performed,
// ... }. NGAP-only, mirroring the indication: TS 36.413 has the Security
// Result IE but no confidentiality field in it.
type ConfidentialityProtectionResult uint8

const (
	ConfidentialityProtectionPerformed ConfidentialityProtectionResult = iota
	ConfidentialityProtectionNotPerformed

	confidentialityProtectionResultRootCount = 2
)

// MaximumIntegrityProtectedDataRate ::= ENUMERATED { bitrate64kbs,
// maximum-UE-rate, ... } — TS 38.413 §9.3.1.103. NGAP-only.
type MaximumIntegrityProtectedDataRate uint8

const (
	MaximumIntegrityProtectedDataRate64kbs MaximumIntegrityProtectedDataRate = iota
	MaximumIntegrityProtectedDataRateMaximumUERate

	maximumIntegrityProtectedDataRateRootCount = 2
)

// SecurityIndication ::= SEQUENCE { integrityProtectionIndication,
// confidentialityProtectionIndication, maximumIntegrityProtectedDataRate-UL
// OPTIONAL, iE-Extensions OPTIONAL } (extensible) — TS 38.413 §9.3.1.27. The
// spec conditions the UL rate on the integrity indication being required or
// preferred; that is a sender obligation, so the field stays a plain pointer
// and a decoder accepts whatever arrives. TS 36.413 §9.2.1.163 defines the
// same IE carrying the integrity indication alone.
type SecurityIndication struct {
	_                                   [0]struct{}                         `per:"extseq"`
	IntegrityProtectionIndication       IntegrityProtectionIndication       `per:"ENUMERATED,range:0..2,..."`
	ConfidentialityProtectionIndication ConfidentialityProtectionIndication `per:"ENUMERATED,range:0..2,..."`
	MaximumIntegrityProtectedDataRateUL *MaximumIntegrityProtectedDataRate  `per:"ENUMERATED,range:0..1,...,optional"`
	_                                   ieExtensions                        `per:",skip"`
}

// SecurityResult ::= SEQUENCE { integrityProtectionResult,
// confidentialityProtectionResult, iE-Extensions OPTIONAL } (extensible) — TS
// 38.413 §9.3.1.59. TS 36.413 §9.2.1.164 defines the same IE carrying the
// integrity result alone.
type SecurityResult struct {
	_                               [0]struct{}                     `per:"extseq"`
	IntegrityProtectionResult       IntegrityProtectionResult       `per:"ENUMERATED,range:0..1,..."`
	ConfidentialityProtectionResult ConfidentialityProtectionResult `per:"ENUMERATED,range:0..1,..."`
	_                               ieExtensions                    `per:",skip"`
}
