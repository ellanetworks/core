// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"github.com/ellanetworks/core/per"
	"github.com/ellanetworks/core/per/ap"
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
)

// Pre-emptionVulnerability ::= ENUMERATED { not-pre-emptable, pre-emptable,
// ... }. Extensible, where S1AP's is not.
type PreemptionVulnerability uint8

const (
	PreemptionNotPreemptable PreemptionVulnerability = iota
	PreemptionPreemptable
)

// NotificationControl ::= ENUMERATED { notification-requested, ... }.
type NotificationControl uint8

const (
	NotificationRequested NotificationControl = iota
)

// PacketLossRate ::= INTEGER (0..1000, ...) in tenths of a percent.
type PacketLossRate uint16

func (p PacketLossRate) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return encodeExtensibleInt(w, enc, 0, 1000, int64(p))
}

func (p *PacketLossRate) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	v, err := decodeExtensibleInt[PacketLossRate](r, enc, 0, 1000)
	if err != nil {
		return err
	}

	*p = v

	return nil
}

func (v PacketLossRate) Diagnose(*ap.Diagnosis[ProtocolIEID]) bool {
	return outsideRoot(int64(v), 0, 1000)
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
	v, err := decodeExtensibleInt[FiveQI](r, enc, 0, 255)
	if err != nil {
		return err
	}

	*f = v

	return nil
}

func (v FiveQI) Diagnose(*ap.Diagnosis[ProtocolIEID]) bool { return outsideRoot(int64(v), 0, 255) }

// PriorityLevelQos ::= INTEGER (1..127, ...) — TS 38.413 §9.3.1.84. Distinct
// from PriorityLevelARP: this one ranks QoS flows, that one ranks allocation.
type PriorityLevelQos uint8

func (p PriorityLevelQos) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return encodeExtensibleInt(w, enc, 1, 127, int64(p))
}

func (p *PriorityLevelQos) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	v, err := decodeExtensibleInt[PriorityLevelQos](r, enc, 1, 127)
	if err != nil {
		return err
	}

	*p = v

	return nil
}

func (v PriorityLevelQos) Diagnose(*ap.Diagnosis[ProtocolIEID]) bool {
	return outsideRoot(int64(v), 1, 127)
}

// PacketDelayBudget ::= INTEGER (0..1023, ...) in units of 0.5 ms.
type PacketDelayBudget uint16

func (p PacketDelayBudget) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return encodeExtensibleInt(w, enc, 0, 1023, int64(p))
}

func (p *PacketDelayBudget) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	v, err := decodeExtensibleInt[PacketDelayBudget](r, enc, 0, 1023)
	if err != nil {
		return err
	}

	*p = v

	return nil
}

func (v PacketDelayBudget) Diagnose(*ap.Diagnosis[ProtocolIEID]) bool {
	return outsideRoot(int64(v), 0, 1023)
}

// AveragingWindow ::= INTEGER (0..4095, ...) in milliseconds.
type AveragingWindow uint16

func (a AveragingWindow) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return encodeExtensibleInt(w, enc, 0, 4095, int64(a))
}

func (a *AveragingWindow) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	v, err := decodeExtensibleInt[AveragingWindow](r, enc, 0, 4095)
	if err != nil {
		return err
	}

	*a = v

	return nil
}

func (v AveragingWindow) Diagnose(*ap.Diagnosis[ProtocolIEID]) bool {
	return outsideRoot(int64(v), 0, 4095)
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
	return encodeExtensibleInt(w, enc, 0, maximumDataBurstVolumeRootUB, int64(m))
}

func (m *MaximumDataBurstVolume) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	v, err := decodeExtensibleInt[MaximumDataBurstVolume](r, enc, 0, maximumDataBurstVolumeRootUB)
	if err != nil {
		return err
	}

	*m = v

	return nil
}

func (m MaximumDataBurstVolume) Diagnose(*ap.Diagnosis[ProtocolIEID]) bool {
	return outsideRoot(int64(m), 0, maximumDataBurstVolumeUB)
}

// DelayCritical ::= ENUMERATED { delay-critical, non-delay-critical, ... }.
type DelayCritical uint8

const (
	DelayCriticalYes DelayCritical = iota
	DelayCriticalNo
)

// ReflectiveQosAttribute ::= ENUMERATED { subject-to, ... }.
type ReflectiveQosAttribute uint8

const (
	ReflectiveQosSubjectTo ReflectiveQosAttribute = iota
)

// AdditionalQosFlowInformation ::= ENUMERATED { more-likely, ... }.
type AdditionalQosFlowInformation uint8

const (
	AdditionalQosFlowMoreLikely AdditionalQosFlowInformation = iota
)

// PacketErrorRate ::= SEQUENCE { pERScalar INTEGER (0..9, ...), pERExponent
// INTEGER (0..9, ...), iE-Extensions OPTIONAL } (extensible) — the rate is
// scalar × 10^-exponent.
type PacketErrorRate struct {
	Scalar       PERDigit
	Exponent     PERDigit
	iEExtensions extensionMeta          `per:",exttable"`
	extensions   per.ExtensionAdditions `per:",extraw"`
}

// PERDigit is the shared INTEGER (0..9, ...) of both PacketErrorRate fields.
type PERDigit uint8

func (d PERDigit) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return encodeExtensibleInt(w, enc, 0, 9, int64(d))
}

func (d *PERDigit) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	v, err := decodeExtensibleInt[PERDigit](r, enc, 0, 9)
	if err != nil {
		return err
	}

	*d = v

	return nil
}

func (v PERDigit) Diagnose(*ap.Diagnosis[ProtocolIEID]) bool { return outsideRoot(int64(v), 0, 9) }

// NonDynamic5QIDescriptor ::= SEQUENCE { fiveQI, priorityLevelQos OPTIONAL,
// averagingWindow OPTIONAL, maximumDataBurstVolume OPTIONAL, iE-Extensions
// OPTIONAL } (extensible) — TS 38.413 §9.3.1.28.
type NonDynamic5QIDescriptor struct {
	FiveQI                 FiveQI
	PriorityLevelQos       *PriorityLevelQos          `per:",optional"`
	AveragingWindow        *AveragingWindow           `per:",optional"`
	MaximumDataBurstVolume *MaximumDataBurstVolume    `per:",optional"`
	CNPacketDelayBudgetDL  *ExtendedPacketDelayBudget `per:",extie:IDCNPacketDelayBudgetDL,crit:ignore"`
	CNPacketDelayBudgetUL  *ExtendedPacketDelayBudget `per:",extie:IDCNPacketDelayBudgetUL,crit:ignore"`
	iEExtensions           extensionMeta              `per:",exttable"`
	extensions             per.ExtensionAdditions     `per:",extraw"`
}

// Dynamic5QIDescriptor ::= SEQUENCE { priorityLevelQos, packetDelayBudget,
// packetErrorRate, fiveQI OPTIONAL, delayCritical OPTIONAL, averagingWindow
// OPTIONAL, maximumDataBurstVolume OPTIONAL, iE-Extensions OPTIONAL }
// (extensible) — TS 38.413 §9.3.1.18. The spec conditions delayCritical and
// averagingWindow on the flow being GBR, and maximumDataBurstVolume on
// delayCritical being "delay critical".
type Dynamic5QIDescriptor struct {
	PriorityLevelQos          PriorityLevelQos
	PacketDelayBudget         PacketDelayBudget
	PacketErrorRate           PacketErrorRate
	FiveQI                    *FiveQI                    `per:",optional"`
	DelayCritical             *DelayCritical             `per:",optional"`
	AveragingWindow           *AveragingWindow           `per:",optional"`
	MaximumDataBurstVolume    *MaximumDataBurstVolume    `per:",optional"`
	ExtendedPacketDelayBudget *ExtendedPacketDelayBudget `per:",extie:IDExtendedPacketDelayBudget,crit:ignore"`
	CNPacketDelayBudgetDL     *ExtendedPacketDelayBudget `per:",extie:IDCNPacketDelayBudgetDL,crit:ignore"`
	CNPacketDelayBudgetUL     *ExtendedPacketDelayBudget `per:",extie:IDCNPacketDelayBudgetUL,crit:ignore"`
	iEExtensions              extensionMeta              `per:",exttable"`
	extensions                per.ExtensionAdditions     `per:",extraw"`
}

// QosCharacteristics ::= CHOICE { nonDynamic5QI, dynamic5QI, choice-Extensions
// } — the CHOICE row of TS 38.413 §9.3.1.12, which has no section of its
// own. The two descriptors keep their own types rather
// than merging into shared fields: fiveQI and priorityLevelQos swap between
// mandatory and optional across the alternatives, and flattening would lose
// that. S1AP has no counterpart — an E-RAB carries a bare QCI.
type QosCharacteristics struct {
	NonDynamic5QI   *NonDynamic5QIDescriptor `per:",choice:0"`
	Dynamic5QI      *Dynamic5QIDescriptor    `per:",choice:1"`
	ChoiceExtension *SingleContainer         `per:",choice:2,extie"`
}

// QosFlowLevelQosParameters ::= SEQUENCE { qosCharacteristics,
// allocationAndRetentionPriority, gBR-QosInformation OPTIONAL,
// reflectiveQosAttribute OPTIONAL, additionalQosFlowInformation OPTIONAL,
// iE-Extensions OPTIONAL } (extensible) — TS 38.413 §9.3.1.12. S1AP's
// E-RABLevelQoSParameters is the counterpart: a QCI where this has a
// QosCharacteristics CHOICE, and no reflective/additional fields.
type QosFlowLevelQosParameters struct {
	QosCharacteristics                          QosCharacteristics
	AllocationAndRetentionPriority              AllocationAndRetentionPriority
	GBRQosInformation                           *GBRQosInformation                           `per:",optional"`
	ReflectiveQosAttribute                      *ReflectiveQosAttribute                      `per:",optional"`
	AdditionalQosFlowInformation                *AdditionalQosFlowInformation                `per:",optional"`
	QosMonitoringRequest                        *QosMonitoringRequest                        `per:",extie:IDQosMonitoringRequest,crit:ignore"`
	QosMonitoringReportingFrequency             *QosMonitoringReportingFrequency             `per:",extie:IDQosMonitoringReportingFrequency,crit:ignore"`
	PDUsetQoSParameters                         *PDUsetQoSParameters                         `per:",extie:IDPDUsetQoSParameters,crit:ignore"`
	DLPDUSetInformationMarkingSupportIndication *DLPDUSetInformationMarkingSupportIndication `per:",extie:IDDLPDUSetInformationMarkingSupportIndication,crit:ignore"`
	MMSID                                       *MMSID                                       `per:",extie:IDMMSID,crit:ignore"`
	IndicationofBitrateAdaptation               *IndicationofBitrateAdaptation               `per:",extie:IDIndicationofBitrateAdaptation,crit:ignore"`
	iEExtensions                                extensionMeta                                `per:",exttable"`
	extensions                                  per.ExtensionAdditions                       `per:",extraw"`
}

// AllocationAndRetentionPriority ::= SEQUENCE { priorityLevelARP,
// pre-emptionCapability, pre-emptionVulnerability, iE-Extensions OPTIONAL }
// (extensible) — TS 38.413 §9.3.1.19.
type AllocationAndRetentionPriority struct {
	PriorityLevelARP        PriorityLevelARP
	PreemptionCapability    PreemptionCapability
	PreemptionVulnerability PreemptionVulnerability
	iEExtensions            extensionMeta          `per:",exttable"`
	extensions              per.ExtensionAdditions `per:",extraw"`
}

// GBR-QosInformation ::= SEQUENCE { four BitRates, notificationControl
// OPTIONAL, maximumPacketLossRateDL OPTIONAL, maximumPacketLossRateUL
// OPTIONAL, iE-Extensions OPTIONAL } (extensible) — TS 38.413 §9.3.1.10,
// where 3GPP names the IE GBR QoS Flow Information. S1AP's GBR-QosInformation
// carries only the four bit rates; its packet loss rate sits one level up, as
// an optional extension on E-RABLevelQoSParameters, and is non-extensible
// there where NGAP's PacketLossRate is extensible.
type GBRQosInformation struct {
	MaximumFlowBitRateDL                BitRate
	MaximumFlowBitRateUL                BitRate
	GuaranteedFlowBitRateDL             BitRate
	GuaranteedFlowBitRateUL             BitRate
	NotificationControl                 *NotificationControl                 `per:",optional"`
	MaximumPacketLossRateDL             *PacketLossRate                      `per:",optional"`
	MaximumPacketLossRateUL             *PacketLossRate                      `per:",optional"`
	AlternativeQoSParaSetList           AlternativeQoSParaSetList            `per:",extie:IDAlternativeQoSParaSetList,crit:ignore"`
	MonitoringRequestonAvailableBitrate *MonitoringRequestonAvailableBitrate `per:",extie:IDMonitoringRequestonAvailableBitrate,crit:ignore"`
	iEExtensions                        extensionMeta                        `per:",exttable"`
	extensions                          per.ExtensionAdditions               `per:",extraw"`
}

// QosFlowIdentifier ::= INTEGER (0..63, ...) — TS 38.413 §9.3.1.51.
type QosFlowIdentifier uint8

func (q QosFlowIdentifier) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return encodeExtensibleInt(w, enc, 0, 63, int64(q))
}

func (q *QosFlowIdentifier) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	v, err := decodeExtensibleInt[QosFlowIdentifier](r, enc, 0, 63)
	if err != nil {
		return err
	}

	*q = v

	return nil
}

func (v QosFlowIdentifier) Diagnose(*ap.Diagnosis[ProtocolIEID]) bool {
	return outsideRoot(int64(v), 0, 63)
}

// ERABID ::= INTEGER (0..15, ...) — TS 38.413 §9.3.2.3. NGAP carries an E-RAB
// ID so a QoS flow can be tied to the EPS bearer it maps to during
// interworking; the definition matches TS 36.413 §9.2.1.2 exactly.
type ERABID uint8

func (id ERABID) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return encodeExtensibleInt(w, enc, 0, 15, int64(id))
}

func (id *ERABID) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	v, err := decodeExtensibleInt[ERABID](r, enc, 0, 15)
	if err != nil {
		return err
	}

	*id = v

	return nil
}

func (v ERABID) Diagnose(*ap.Diagnosis[ProtocolIEID]) bool { return outsideRoot(int64(v), 0, 15) }

// QosFlowMappingIndication ::= ENUMERATED { ul, dl, ... }, inlined in
// AssociatedQosFlowItem.
type QosFlowMappingIndication uint8

const (
	QosFlowMappingUL QosFlowMappingIndication = iota
	QosFlowMappingDL
)

// QosFlowSetupRequestItem ::= SEQUENCE { qosFlowIdentifier,
// qosFlowLevelQosParameters, e-RAB-ID OPTIONAL, iE-Extensions OPTIONAL }
// (extensible).
type QosFlowSetupRequestItem struct {
	QosFlowIdentifier                                 QosFlowIdentifier
	QosFlowLevelQosParameters                         QosFlowLevelQosParameters
	ERABID                                            *ERABID                                            `per:",optional"`
	TSCTrafficCharacteristics                         *TSCTrafficCharacteristics                         `per:",extie:IDTSCTrafficCharacteristics,crit:ignore"`
	RedundantQosFlowIndicator                         *RedundantQosFlowIndicator                         `per:",extie:IDRedundantQosFlowIndicator,crit:ignore"`
	ECNMarkingorCongestionInformationReportingRequest *ECNMarkingorCongestionInformationReportingRequest `per:",extie:IDECNMarkingorCongestionInformationReportingRequest,crit:ignore"`
	iEExtensions                                      extensionMeta                                      `per:",exttable"`
	extensions                                        per.ExtensionAdditions                             `per:",extraw"`
}

// QosFlowSetupRequestList ::= SEQUENCE (SIZE(1..maxnoofQosFlows)) OF
// QosFlowSetupRequestItem.
type QosFlowSetupRequestList []QosFlowSetupRequestItem

// QosFlowWithCauseItem ::= SEQUENCE { qosFlowIdentifier, cause, iE-Extensions
// OPTIONAL } (extensible).
type QosFlowWithCauseItem struct {
	QosFlowIdentifier QosFlowIdentifier
	Cause             Cause
	iEExtensions      extensionMeta          `per:",exttable"`
	extensions        per.ExtensionAdditions `per:",extraw"`
}

// QosFlowListWithCause ::= SEQUENCE (SIZE(1..maxnoofQosFlows)) OF
// QosFlowWithCauseItem.
type QosFlowListWithCause []QosFlowWithCauseItem

// AssociatedQosFlowItem ::= SEQUENCE { qosFlowIdentifier,
// qosFlowMappingIndication OPTIONAL, iE-Extensions OPTIONAL } (extensible).
type AssociatedQosFlowItem struct {
	QosFlowIdentifier        QosFlowIdentifier
	QosFlowMappingIndication *QosFlowMappingIndication   `per:",optional"`
	CurrentQoSParaSetIndex   *AlternativeQoSParaSetIndex `per:",extie:IDCurrentQoSParaSetIndex,crit:ignore"`
	iEExtensions             extensionMeta               `per:",exttable"`
	extensions               per.ExtensionAdditions      `per:",extraw"`
}

// AssociatedQosFlowList ::= SEQUENCE (SIZE(1..maxnoofQosFlows)) OF
// AssociatedQosFlowItem.
type AssociatedQosFlowList []AssociatedQosFlowItem

// QosFlowModifyConfirmItem ::= SEQUENCE { qosFlowIdentifier, iE-Extensions
// OPTIONAL } (extensible).
type QosFlowModifyConfirmItem struct {
	QosFlowIdentifier QosFlowIdentifier
	iEExtensions      extensionMeta          `per:",exttable"`
	extensions        per.ExtensionAdditions `per:",extraw"`
}

// QosFlowModifyConfirmList ::= SEQUENCE (SIZE(1..maxnoofQosFlows)) OF
// QosFlowModifyConfirmItem.
type QosFlowModifyConfirmList []QosFlowModifyConfirmItem

// QosFlowAddOrModifyRequestItem ::= SEQUENCE { qosFlowIdentifier,
// qosFlowLevelQosParameters OPTIONAL, e-RAB-ID OPTIONAL, iE-Extensions
// OPTIONAL } (extensible). The QoS parameters are optional here where
// QosFlowSetupRequestItem has them mandatory: a modify may touch only the
// E-RAB mapping.
type QosFlowAddOrModifyRequestItem struct {
	QosFlowIdentifier                                 QosFlowIdentifier
	QosFlowLevelQosParameters                         *QosFlowLevelQosParameters                         `per:",optional"`
	ERABID                                            *ERABID                                            `per:",optional"`
	TSCTrafficCharacteristics                         *TSCTrafficCharacteristics                         `per:",extie:IDTSCTrafficCharacteristics,crit:ignore"`
	RedundantQosFlowIndicator                         *RedundantQosFlowIndicator                         `per:",extie:IDRedundantQosFlowIndicator,crit:ignore"`
	ULNGUUPTNLInformation                             *UPTransportLayerInformation                       `per:",extie:IDULNGUUPTNLInformation,crit:ignore"`
	DownlinkTLContainer                               *TLContainer                                       `per:",extie:IDDownlinkTLContainer,crit:ignore"`
	ECNMarkingorCongestionInformationReportingRequest *ECNMarkingorCongestionInformationReportingRequest `per:",extie:IDECNMarkingorCongestionInformationReportingRequest,crit:ignore"`
	iEExtensions                                      extensionMeta                                      `per:",exttable"`
	extensions                                        per.ExtensionAdditions                             `per:",extraw"`
}

// QosFlowAddOrModifyRequestList ::= SEQUENCE (SIZE(1..maxnoofQosFlows)) OF
// QosFlowAddOrModifyRequestItem.
type QosFlowAddOrModifyRequestList []QosFlowAddOrModifyRequestItem

// DataForwardingAccepted ::= ENUMERATED { data-forwarding-accepted, ... }.
type DataForwardingAccepted uint8

const (
	DataForwardingAcceptedTrue DataForwardingAccepted = iota
)

// QosFlowAcceptedItem ::= SEQUENCE { qosFlowIdentifier, iE-Extensions OPTIONAL
// } (extensible).
type QosFlowAcceptedItem struct {
	QosFlowIdentifier      QosFlowIdentifier
	CurrentQoSParaSetIndex *AlternativeQoSParaSetIndex `per:",extie:IDCurrentQoSParaSetIndex,crit:ignore"`
	iEExtensions           extensionMeta               `per:",exttable"`
	extensions             per.ExtensionAdditions      `per:",extraw"`
}

// QosFlowAcceptedList ::= SEQUENCE (SIZE(1..maxnoofQosFlows)) OF
// QosFlowAcceptedItem.
type QosFlowAcceptedList []QosFlowAcceptedItem

// QosFlowToBeForwardedItem ::= SEQUENCE { qosFlowIdentifier, iE-Extensions
// OPTIONAL } (extensible).
type QosFlowToBeForwardedItem struct {
	QosFlowIdentifier QosFlowIdentifier
	iEExtensions      extensionMeta          `per:",exttable"`
	extensions        per.ExtensionAdditions `per:",extraw"`
}

// QosFlowToBeForwardedList ::= SEQUENCE (SIZE(1..maxnoofQosFlows)) OF
// QosFlowToBeForwardedItem.
type QosFlowToBeForwardedList []QosFlowToBeForwardedItem

// QosFlowItemWithDataForwarding ::= SEQUENCE { qosFlowIdentifier,
// dataForwardingAccepted OPTIONAL, iE-Extensions OPTIONAL } (extensible).
type QosFlowItemWithDataForwarding struct {
	QosFlowIdentifier      QosFlowIdentifier
	DataForwardingAccepted *DataForwardingAccepted     `per:",optional"`
	CurrentQoSParaSetIndex *AlternativeQoSParaSetIndex `per:",extie:IDCurrentQoSParaSetIndex,crit:ignore"`
	iEExtensions           extensionMeta               `per:",exttable"`
	extensions             per.ExtensionAdditions      `per:",extraw"`
}

// QosFlowListWithDataForwarding ::= SEQUENCE (SIZE(1..maxnoofQosFlows)) OF
// QosFlowItemWithDataForwarding.
type QosFlowListWithDataForwarding []QosFlowItemWithDataForwarding

// QosFlowPerTNLInformation ::= SEQUENCE { uPTransportLayerInformation,
// associatedQosFlowList, iE-Extensions OPTIONAL } (extensible) — TS 38.413
// §9.3.2.8.
type QosFlowPerTNLInformation struct {
	UPTransportLayerInformation UPTransportLayerInformation
	AssociatedQosFlowList       AssociatedQosFlowList
	iEExtensions                extensionMeta          `per:",exttable"`
	extensions                  per.ExtensionAdditions `per:",extraw"`
}

// QosFlowPerTNLInformationItem ::= SEQUENCE { qosFlowPerTNLInformation,
// iE-Extensions OPTIONAL } (extensible).
type QosFlowPerTNLInformationItem struct {
	QosFlowPerTNLInformation QosFlowPerTNLInformation
	iEExtensions             extensionMeta          `per:",exttable"`
	extensions               per.ExtensionAdditions `per:",extraw"`
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
)

// NetworkInstance ::= INTEGER (1..256, ...) — TS 38.413 §9.3.1.113.
type NetworkInstance uint16

func (n NetworkInstance) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return encodeExtensibleInt(w, enc, 1, 256, int64(n))
}

func (n *NetworkInstance) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	v, err := decodeExtensibleInt[NetworkInstance](r, enc, 1, 256)
	if err != nil {
		return err
	}

	*n = v

	return nil
}

func (v NetworkInstance) Diagnose(*ap.Diagnosis[ProtocolIEID]) bool {
	return outsideRoot(int64(v), 1, 256)
}

// PDUSessionAggregateMaximumBitRate ::= SEQUENCE { DL, UL, iE-Extensions
// OPTIONAL } (extensible) — TS 38.413 §9.3.1.102. Session-level, where
// UEAggregateMaximumBitRate is UE-level; S1AP has only the UE-level form.
type PDUSessionAggregateMaximumBitRate struct {
	DL           BitRate
	UL           BitRate
	iEExtensions extensionMeta          `per:",exttable"`
	extensions   per.ExtensionAdditions `per:",extraw"`
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
)

// ConfidentialityProtectionIndication ::= ENUMERATED { required, preferred,
// not-needed, ... }. NGAP-only: TS 36.413 has the Security Indication IE but
// no confidentiality field in it.
type ConfidentialityProtectionIndication uint8

const (
	ConfidentialityProtectionRequired ConfidentialityProtectionIndication = iota
	ConfidentialityProtectionPreferred
	ConfidentialityProtectionNotNeeded
)

// IntegrityProtectionResult ::= ENUMERATED { performed, not-performed, ... }.
// TS 36.413 defines this identically.
type IntegrityProtectionResult uint8

const (
	IntegrityProtectionPerformed IntegrityProtectionResult = iota
	IntegrityProtectionNotPerformed
)

// ConfidentialityProtectionResult ::= ENUMERATED { performed, not-performed,
// ... }. NGAP-only, mirroring the indication: TS 36.413 has the Security
// Result IE but no confidentiality field in it.
type ConfidentialityProtectionResult uint8

const (
	ConfidentialityProtectionPerformed ConfidentialityProtectionResult = iota
	ConfidentialityProtectionNotPerformed
)

// MaximumIntegrityProtectedDataRate ::= ENUMERATED { bitrate64kbs,
// maximum-UE-rate, ... } — TS 38.413 §9.3.1.103. NGAP-only.
type MaximumIntegrityProtectedDataRate uint8

const (
	MaximumIntegrityProtectedDataRate64kbs MaximumIntegrityProtectedDataRate = iota
	MaximumIntegrityProtectedDataRateMaximumUERate
)

// SecurityIndication ::= SEQUENCE { integrityProtectionIndication,
// confidentialityProtectionIndication, maximumIntegrityProtectedDataRate-UL
// OPTIONAL, iE-Extensions OPTIONAL } (extensible) — TS 38.413 §9.3.1.27. The
// spec conditions the UL rate on the integrity indication being required or
// preferred; that is a sender obligation, so the field stays a plain pointer
// and a decoder accepts whatever arrives. TS 36.413 §9.2.1.163 defines the
// same IE carrying the integrity indication alone.
type SecurityIndication struct {
	IntegrityProtectionIndication       IntegrityProtectionIndication
	ConfidentialityProtectionIndication ConfidentialityProtectionIndication
	MaximumIntegrityProtectedDataRateUL *MaximumIntegrityProtectedDataRate `per:",optional"`
	MaximumIntegrityProtectedDataRateDL *MaximumIntegrityProtectedDataRate `per:",extie:IDMaximumIntegrityProtectedDataRateDL,crit:ignore"`
	iEExtensions                        extensionMeta                      `per:",exttable"`
	extensions                          per.ExtensionAdditions             `per:",extraw"`
}

// SecurityResult ::= SEQUENCE { integrityProtectionResult,
// confidentialityProtectionResult, iE-Extensions OPTIONAL } (extensible) — TS
// 38.413 §9.3.1.59. TS 36.413 §9.2.1.164 defines the same IE carrying the
// integrity result alone.
type SecurityResult struct {
	IntegrityProtectionResult       IntegrityProtectionResult
	ConfidentialityProtectionResult ConfidentialityProtectionResult
	iEExtensions                    extensionMeta          `per:",exttable"`
	extensions                      per.ExtensionAdditions `per:",extraw"`
}
