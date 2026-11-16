// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"math"

	"github.com/ellanetworks/core/per"
	"github.com/ellanetworks/core/per/ap"
	"github.com/ellanetworks/core/per/ap/apimpl"
)

const (
	maxnoofPSCellsPerPrimaryCellinUEHistoryInfo = 8
	maxnoofTACsInNTN                            = 12
	maxnooffrequencies                          = 64
	maxnoofeNBX2TLAs                            = 2
	maxnoofeNBX2ExtTLAs                         = 16
	maxnoofeNBX2GTPTLAs                         = 16
	nrCellIdentityBits                          = 36
	rATRestrictionInformationBits               = 8
	stratumLevelRootMax                         = 3
	patternOffsetRootMax                        = 10239
)

type GUMMEIType uint8

const (
	GUMMEITypeNative GUMMEIType = iota
	GUMMEITypeMapped
	GUMMEITypeMappedFrom5G
)

type NRrestrictioninEPSasSecondaryRAT uint8

const NRrestrictedinEPSasSecondaryRAT NRrestrictioninEPSasSecondaryRAT = 0

type UnlicensedSpectrumRestriction uint8

const UnlicensedRestricted UnlicensedSpectrumRestriction = 0

type NRrestrictionin5GS uint8

const NRrestrictedin5GS NRrestrictionin5GS = 0

type CNType uint8

const (
	CNTypeFiveGCForbidden CNType = iota
	CNTypeEPCForbidden
)

type CNTypeRestrictionsItem struct {
	PLMNIdentity PLMNIdentity
	CNType       CNType
	iEExtensions extensionMeta          `per:",exttable"`
	extensions   per.ExtensionAdditions `per:",extraw"`
}

type CNTypeRestrictions []CNTypeRestrictionsItem

func (l CNTypeRestrictions) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return marshalSeqOf(w, enc, 1, maxnoofEPLMNsPlusOne, []CNTypeRestrictionsItem(l))
}

func (l *CNTypeRestrictions) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	items, err := unmarshalSeqOf[CNTypeRestrictionsItem](r, enc, 1, maxnoofEPLMNsPlusOne)
	*l = items

	return err
}

type RATRestrictionInformation struct {
	Value    uint8
	extended bool
	octets   string
	bits     int
}

func (v RATRestrictionInformation) MarshalPER(w *per.Writer, enc per.Encoding) error {
	if v.extended {
		return encodeExtSizedBits(w, enc, rATRestrictionInformationBits, v.octets, v.bits)
	}

	return encodeExtSizedBits(w, enc, rATRestrictionInformationBits, string([]byte{v.Value}), rATRestrictionInformationBits)
}

func (v *RATRestrictionInformation) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	b, nbits, err := per.DecodeBitString(r, enc, rATRestrictionInformationBits, rATRestrictionInformationBits, true, true, true)
	if err != nil {
		return err
	}

	head := make([]byte, 1)
	copy(head, b)

	*v = RATRestrictionInformation{Value: head[0]}

	if nbits != rATRestrictionInformationBits {
		v.extended, v.octets, v.bits = true, string(b), nbits
	}

	return nil
}

func encodeExtSizedBits(w *per.Writer, enc per.Encoding, size int64, octets string, nbits int) error {
	return per.EncodeBitString(w, enc, size, size, true, true, true, []byte(octets), nbits)
}

type RATRestrictionsItem struct {
	PLMNIdentity              PLMNIdentity
	RATRestrictionInformation RATRestrictionInformation
	iEExtensions              extensionMeta          `per:",exttable"`
	extensions                per.ExtensionAdditions `per:",extraw"`
}

type RATRestrictions []RATRestrictionsItem

func (l RATRestrictions) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return marshalSeqOf(w, enc, 1, maxnoofEPLMNsPlusOne, []RATRestrictionsItem(l))
}

func (l *RATRestrictions) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	items, err := unmarshalSeqOf[RATRestrictionsItem](r, enc, 1, maxnoofEPLMNsPlusOne)
	*l = items

	return err
}

type NRCellIdentity uint64

func (c NRCellIdentity) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return encodeBitStringUint(w, enc, uint64(c), nrCellIdentityBits)
}

func (c *NRCellIdentity) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	v, err := decodeBitStringUint(r, enc, nrCellIdentityBits)
	*c = NRCellIdentity(v)

	return err
}

type NRCGI struct {
	PLMNIdentity   PLMNIdentity
	NRCellIdentity NRCellIdentity
	iEExtensions   extensionMeta          `per:",exttable"`
	extensions     per.ExtensionAdditions `per:",extraw"`
}

type PSCellInformation struct {
	NCGI         NRCGI
	iEExtensions extensionMeta          `per:",exttable"`
	extensions   per.ExtensionAdditions `per:",extraw"`
}

type LTENTNTAIInformation struct {
	ServingPLMN          PLMNIdentity
	TACListInLTENTN      []TAC                  `per:"SEQUENCE-OF,size:1..12"`
	UELocationDerivedTAC *TAC                   `per:",optional"`
	iEExtensions         extensionMeta          `per:",exttable"`
	extensions           per.ExtensionAdditions `per:",extraw"`
}

type RequestTypeAdditionalInfo uint8

const RequestTypeAdditionalInfoIncludePSCell RequestTypeAdditionalInfo = 0

type DAPSIndicator uint8

const DAPSIndicatorDAPSHORequired DAPSIndicator = 0

type DAPSRequestInfo struct {
	DAPSIndicator DAPSIndicator
	iEExtensions  extensionMeta          `per:",exttable"`
	extensions    per.ExtensionAdditions `per:",extraw"`
}

type TimeUEStayedInCellEnhancedGranularity uint16

func (t TimeUEStayedInCellEnhancedGranularity) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return per.EncodeInteger(w, enc, timeStayBounds, int64(t))
}

func (t *TimeUEStayedInCellEnhancedGranularity) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	v, err := per.DecodeInteger(r, enc, timeStayBounds)
	*t = TimeUEStayedInCellEnhancedGranularity(v)

	return err
}

var timeStayBounds = per.Bounds{LB: 0, HasLB: true, UB: 40950, HasUB: true}

type LastVisitedPSCellInformation struct {
	PSCellID     *PSCellInformation     `per:",optional"`
	TimeStay     uint16                 `per:",range:0..40950"`
	iEExtensions extensionMeta          `per:",exttable"`
	extensions   per.ExtensionAdditions `per:",extraw"`
}

type LastVisitedPSCellList []LastVisitedPSCellInformation

func (l LastVisitedPSCellList) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return marshalSeqOf(w, enc, 1, maxnoofPSCellsPerPrimaryCellinUEHistoryInfo, []LastVisitedPSCellInformation(l))
}

func (l *LastVisitedPSCellList) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	items, err := unmarshalSeqOf[LastVisitedPSCellInformation](r, enc, 1, maxnoofPSCellsPerPrimaryCellinUEHistoryInfo)
	*l = items

	return err
}

type MobilityInformation uint32

func (m MobilityInformation) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return encodeBitStringUint(w, enc, uint64(m), 32)
}

func (m *MobilityInformation) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	v, err := decodeBitStringUint(r, enc, 32)
	*m = MobilityInformation(v)

	return err
}

type AdditionalRRMPriorityIndex uint32

func (a AdditionalRRMPriorityIndex) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return encodeBitStringUint(w, enc, uint64(a), 32)
}

func (a *AdditionalRRMPriorityIndex) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	v, err := decodeBitStringUint(r, enc, 32)
	*a = AdditionalRRMPriorityIndex(v)

	return err
}

type UEHistoryInformationFromTheUE []byte

func (u UEHistoryInformationFromTheUE) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return per.EncodeOctetString(w, enc, 0, 0, false, false, false, u)
}

func (u *UEHistoryInformationFromTheUE) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	b, err := per.DecodeOctetString(r, enc, 0, 0, false, false, false)
	*u = b

	return err
}

type IMSvoiceEPSfallbackfrom5G uint8

const IMSvoiceEPSfallbackfrom5GTrue IMSvoiceEPSfallbackfrom5G = 0

type EmergencyIndicator uint8

const EmergencyIndicatorTrue EmergencyIndicator = 0

type RANUENGAPID uint32

func (id RANUENGAPID) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return per.EncodeInteger(w, enc, per.Bounds{LB: 0, HasLB: true, UB: math.MaxUint32, HasUB: true}, int64(id))
}

func (id *RANUENGAPID) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	v, err := per.DecodeInteger(r, enc, per.Bounds{LB: 0, HasLB: true, UB: math.MaxUint32, HasUB: true})
	*id = RANUENGAPID(v)

	return err
}

type ContextatSource struct {
	SourceNGRANNodeID GlobalRANNodeID
	RANUENGAPID       RANUENGAPID
	iEExtensions      extensionMeta          `per:",exttable"`
	extensions        per.ExtensionAdditions `per:",extraw"`
}

type IntersystemMeasurementConfiguration struct {
	RSRP                             *uint8 `per:",optional,range:0..127"`
	RSRQ                             *uint8 `per:",optional,range:0..127"`
	SINR                             *uint8 `per:",optional,range:0..127"`
	InterSystemMeasurementParameters InterSystemMeasurementParameters
	iEExtensions                     extensionMeta          `per:",exttable"`
	extensions                       per.ExtensionAdditions `per:",extraw"`
}

type InterSystemMeasurementParameters struct {
	MeasurementDuration        uint8                      `per:",range:1..100"`
	InterSystemMeasurementList InterSystemMeasurementList `per:",optional"`
	iEExtensions               extensionMeta              `per:",exttable"`
	extensions                 per.ExtensionAdditions     `per:",extraw"`
}

type InterSystemMeasurementList []InterSystemMeasurementItem

func (l InterSystemMeasurementList) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return marshalSeqOf(w, enc, 1, maxnooffrequencies, []InterSystemMeasurementItem(l))
}

func (l *InterSystemMeasurementList) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	items, err := unmarshalSeqOf[InterSystemMeasurementItem](r, enc, 1, maxnooffrequencies)
	*l = items

	return err
}

type SubcarrierSpacingSSB uint8

const (
	SubcarrierSpacingSSBkHz15 SubcarrierSpacingSSB = iota
	SubcarrierSpacingSSBkHz30
	SubcarrierSpacingSSBkHz60
	SubcarrierSpacingSSBkHz120
	SubcarrierSpacingSSBkHz240
	SubcarrierSpacingSSBkHz480
	SubcarrierSpacingSSBkHz960
)

type InterSystemMeasurementItem struct {
	FreqBandIndicatorNR       uint16 `per:",range:1..1024"`
	SSBFrequencies            uint32 `per:",range:0..3279165"`
	SubcarrierSpacingSSB      SubcarrierSpacingSSB
	MaxRSIndexCellQual        *uint8        `per:",optional,range:1..16"`
	SMTC                      []byte        `per:",optional"`
	ThreshRSIndexR15          []byte        `per:",optional"`
	SSBToMeasure              []byte        `per:",optional"`
	SSRSSIMeasurement         []byte        `per:",optional"`
	QuantityConfigNRR15       []byte        `per:",optional"`
	ExcludedCellsToAddModList []byte        `per:",optional"`
	iEExtensions              extensionMeta `per:",exttable"`
}

type SourceNgRanNodeID struct {
	GlobalRANNodeID GlobalRANNodeID
	SelectedTAI     FiveGSTAI
	iEExtensions    extensionMeta          `per:",exttable"`
	extensions      per.ExtensionAdditions `per:",extraw"`
}

type SourceNodeID struct {
	SourceNgRanNodeID *SourceNgRanNodeID `per:",choice:0"`
	ChoiceExtension   *SingleContainer   `per:",choice:1,extie"`
}

type TimeBasedHandoverInformation struct {
	HOWindowStart    uint32                 `per:",range:0..1048575"`
	HOWindowDuration uint16                 `per:",range:1..6000"`
	iEExtensions     extensionMeta          `per:",exttable"`
	extensions       per.ExtensionAdditions `per:",extraw"`
}

type RATType uint8

const (
	RATTypeNBIoT RATType = iota
	RATTypeNBIoTLEO
	RATTypeNBIoTMEO
	RATTypeNBIoTGEO
	RATTypeNBIoTOtherSat
	RATTypeEUTRANLEO
	RATTypeEUTRANMEO
	RATTypeEUTRANGEO
	RATTypeEUTRANOtherSat
)

type MDTConfigurationNR []byte

func (m MDTConfigurationNR) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return per.EncodeOctetString(w, enc, 0, 0, false, false, false, m)
}

func (m *MDTConfigurationNR) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	b, err := per.DecodeOctetString(r, enc, 0, 0, false, false, false)
	*m = b

	return err
}

type URIAddress string

func (u URIAddress) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return per.EncodeKnownMultiplierString(w, enc, per.CharVisibleString, 0, 0, false, false, false, string(u))
}

func (u *URIAddress) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	s, err := per.DecodeKnownMultiplierString(r, enc, per.CharVisibleString, 0, 0, false, false, false)
	*u = URIAddress(s)

	return err
}

type CoarseUELocationRequested uint8

const CoarseUELocationRequestedTrue CoarseUELocationRequested = 0

type TimeRefDistribution uint8

const TimeRefDistributionTrue TimeRefDistribution = 0

type CoarseUELocation []byte

func (c CoarseUELocation) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return per.EncodeOctetString(w, enc, 0, 0, false, false, false, c)
}

func (c *CoarseUELocation) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	b, err := per.DecodeOctetString(r, enc, 0, 0, false, false, false)
	*c = b

	return err
}

type RequestedTNLInfo struct {
	PLMNIdentity PLMNIdentity
	iEExtensions extensionMeta          `per:",exttable"`
	extensions   per.ExtensionAdditions `per:",extraw"`
}

type SourceeNBID struct {
	GlobalENBID  GlobalENBID
	SelectedTAI  TAI
	iEExtensions extensionMeta `per:",exttable"`
}

type SONInformationRequest uint8

const (
	SONInformationRequestX2TNLConfigurationInfo SONInformationRequest = iota
	SONInformationRequestTimeSynchronisationInfo
	SONInformationRequestActivateMuting
	SONInformationRequestDeactivateMuting
)

type SONInformation struct {
	SONInformationRequest   *SONInformationRequest    `per:",choice:0"`
	SONInformationReply     *SONInformationReply      `per:",choice:1"`
	SONInformationExtension *SONInformationExtension  `per:",choice:0,ext"`
	Ext                     *per.ExtensionAlternative `per:",extalt"`
}

type SONInformationExtension struct {
	SONInformationReport *SONInformationReport  `per:",choice:0,extie:IDSONInformationReport,crit:ignore"`
	ChoiceExtension      *SingleContainer       `per:",choice:0,extie"`
	choiceCriticality    apimpl.WireCriticality `per:",choicecrit"`
}

type SONInformationReport struct {
	RLFReportInformation *RLFReportInformation     `per:",choice:0"`
	Ext                  *per.ExtensionAlternative `per:",extalt"`
}

type RLFReportContainer []byte

func (c RLFReportContainer) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return per.EncodeOctetString(w, enc, 0, 0, false, false, false, c)
}

func (c *RLFReportContainer) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	b, err := per.DecodeOctetString(r, enc, 0, 0, false, false, false)
	*c = b

	return err
}

type RLFReportInformation struct {
	UERLFReportContainer                 RLFReportContainer
	UERLFReportContainerForExtendedBands *RLFReportContainer    `per:",optional"`
	NBIoTRLFReportContainer              *RLFReportContainer    `per:",extie:IDNBIoTRLFReportContainer,crit:ignore"`
	iEExtensions                         extensionMeta          `per:",exttable"`
	extensions                           per.ExtensionAdditions `per:",extraw"`
}

type SONInformationReply struct {
	X2TNLConfigurationInfo   *X2TNLConfigurationInfo   `per:",optional"`
	TimeSynchronisationInfo  *TimeSynchronisationInfo  `per:",extie:IDTimeSynchronisationInfo,crit:ignore"`
	MutingPatternInformation *MutingPatternInformation `per:",extie:IDMutingPatternInformation,crit:ignore"`
	iEExtensions             extensionMeta             `per:",exttable"`
	extensions               per.ExtensionAdditions    `per:",extraw"`
}

type ENBX2TLAs []TransportLayerAddress

func (l ENBX2TLAs) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return marshalSeqOf(w, enc, 1, maxnoofeNBX2TLAs, []TransportLayerAddress(l))
}

func (l *ENBX2TLAs) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	items, err := unmarshalSeqOf[TransportLayerAddress](r, enc, 1, maxnoofeNBX2TLAs)
	*l = items

	return err
}

type ENBX2GTPTLAs []TransportLayerAddress

func (l ENBX2GTPTLAs) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return marshalSeqOf(w, enc, 1, maxnoofeNBX2GTPTLAs, []TransportLayerAddress(l))
}

func (l *ENBX2GTPTLAs) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	items, err := unmarshalSeqOf[TransportLayerAddress](r, enc, 1, maxnoofeNBX2GTPTLAs)
	*l = items

	return err
}

type ENBX2ExtTLA struct {
	IPsecTLA     *TransportLayerAddress `per:",optional"`
	GTPTLAa      ENBX2GTPTLAs           `per:",optional"`
	iEExtensions extensionMeta          `per:",exttable"`
	extensions   per.ExtensionAdditions `per:",extraw"`
}

type ENBX2ExtTLAs []ENBX2ExtTLA

func (l ENBX2ExtTLAs) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return marshalSeqOf(w, enc, 1, maxnoofeNBX2ExtTLAs, []ENBX2ExtTLA(l))
}

func (l *ENBX2ExtTLAs) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	items, err := unmarshalSeqOf[ENBX2ExtTLA](r, enc, 1, maxnoofeNBX2ExtTLAs)
	*l = items

	return err
}

type X2TNLConfigurationInfo struct {
	ENBX2TransportLayerAddresses         ENBX2TLAs
	ENBX2ExtendedTransportLayerAddresses ENBX2ExtTLAs           `per:",extie:IDENBX2ExtendedTransportLayerAddresses,crit:ignore"`
	ENBIndirectX2TransportLayerAddresses ENBX2TLAs              `per:",extie:IDENBIndirectX2TransportLayerAddresses,crit:ignore"`
	iEExtensions                         extensionMeta          `per:",exttable"`
	extensions                           per.ExtensionAdditions `per:",extraw"`
}

type StratumLevel uint8

func (s StratumLevel) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return encodeExtensibleInt(w, enc, 0, stratumLevelRootMax, int64(s))
}

func (s *StratumLevel) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	v, err := decodeExtensibleInt(r, enc, 0, stratumLevelRootMax)
	if err != nil {
		return err
	}

	if v < 0 || v > math.MaxUint8 {
		return per.ErrOverflow
	}

	*s = StratumLevel(v)

	return nil
}

func (s *StratumLevel) Diagnose(*ap.Diagnosis[ProtocolIEID]) bool { return *s > stratumLevelRootMax }

type PatternOffset uint16

func (p PatternOffset) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return encodeExtensibleInt(w, enc, 0, patternOffsetRootMax, int64(p))
}

func (p *PatternOffset) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	v, err := decodeExtensibleInt(r, enc, 0, patternOffsetRootMax)
	if err != nil {
		return err
	}

	if v < 0 || v > math.MaxUint16 {
		return per.ErrOverflow
	}

	*p = PatternOffset(v)

	return nil
}

func (p *PatternOffset) Diagnose(*ap.Diagnosis[ProtocolIEID]) bool { return *p > patternOffsetRootMax }

type SynchronisationStatus uint8

const (
	SynchronisationStatusSynchronous SynchronisationStatus = iota
	SynchronisationStatusAsynchronous
)

type MutingAvailabilityIndication uint8

const (
	MutingAvailabilityIndicationAvailable MutingAvailabilityIndication = iota
	MutingAvailabilityIndicationUnavailable
)

type TimeSynchronisationInfo struct {
	StratumLevel                 StratumLevel
	SynchronisationStatus        SynchronisationStatus
	MutingAvailabilityIndication *MutingAvailabilityIndication `per:",extie:IDMutingAvailabilityIndication,crit:ignore"`
	iEExtensions                 extensionMeta                 `per:",exttable"`
	extensions                   per.ExtensionAdditions        `per:",extraw"`
}

type MutingPatternPeriod uint8

const (
	MutingPatternPeriodMs0 MutingPatternPeriod = iota
	MutingPatternPeriodMs1280
	MutingPatternPeriodMs2560
	MutingPatternPeriodMs5120
	MutingPatternPeriodMs10240
)

type MutingPatternInformation struct {
	MutingPatternPeriod MutingPatternPeriod
	MutingPatternOffset *PatternOffset         `per:",optional"`
	iEExtensions        extensionMeta          `per:",exttable"`
	extensions          per.ExtensionAdditions `per:",extraw"`
}

type PatternPeriod uint8

const (
	PatternPeriodMs1280 PatternPeriod = iota
	PatternPeriodMs2560
	PatternPeriodMs5120
	PatternPeriodMs10240
)

type ListeningSubframePattern struct {
	PatternPeriod PatternPeriod
	PatternOffset PatternOffset
	iEExtensions  extensionMeta          `per:",exttable"`
	extensions    per.ExtensionAdditions `per:",extraw"`
}

type SynchronisationInformation struct {
	SourceStratumLevel       *StratumLevel             `per:",optional"`
	ListeningSubframePattern *ListeningSubframePattern `per:",optional"`
	AggressoreCGIList        []EUTRANCGI               `per:"SEQUENCE-OF,size:1..256,optional"`
	iEExtensions             extensionMeta             `per:",exttable"`
	extensions               per.ExtensionAdditions    `per:",extraw"`
}

type SONConfigurationTransfer struct {
	TargeteNBID                TargeteNBID
	SourceeNBID                SourceeNBID
	SONInformation             SONInformation
	X2TNLConfigurationInfo     *X2TNLConfigurationInfo     `per:",extie:IDX2TNLConfigurationInfo,crit:ignore,presence:conditional,cond:sonRequestsX2TNLConfigurationInfo"`
	SynchronisationInformation *SynchronisationInformation `per:",extie:IDSynchronisationInformation,crit:ignore,presence:conditional,cond:sonRequestsActivateMuting"`
	RequestedTNLInfo           *RequestedTNLInfo           `per:",extie:IDRequestedTNLInfo,crit:ignore"`
	iEExtensions               extensionMeta               `per:",exttable"`
	extensions                 per.ExtensionAdditions      `per:",extraw"`
}

func sonRequests(c *SONConfigurationTransfer, want SONInformationRequest) bool {
	r := c.SONInformation.SONInformationRequest

	return r != nil && *r == want
}

func sonRequestsX2TNLConfigurationInfo(c *SONConfigurationTransfer) bool {
	return sonRequests(c, SONInformationRequestX2TNLConfigurationInfo)
}

func sonRequestsActivateMuting(c *SONConfigurationTransfer) bool {
	return sonRequests(c, SONInformationRequestActivateMuting)
}

type PortNumber [2]byte

func (p PortNumber) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return per.EncodeOctetString(w, enc, 2, 2, true, true, false, p[:])
}

func (p *PortNumber) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	b, err := per.DecodeOctetString(r, enc, 2, 2, true, true, false)
	if err != nil {
		return err
	}

	copy(p[:], b)

	return nil
}

type TunnelInformation struct {
	TransportLayerAddress TransportLayerAddress
	UDPPortNumber         *PortNumber            `per:",optional"`
	iEExtensions          extensionMeta          `per:",exttable"`
	extensions            per.ExtensionAdditions `per:",extraw"`
}
