// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"github.com/ellanetworks/core/per"
	"github.com/ellanetworks/core/per/ap"
)

const (
	maxnoofCSGs             = 256
	maxnoofConnectedengNBs  = 256
	maxnoofDCNs             = 32
	maxnoofMDTPLMNs         = 16
	maxnoofPC5QoSFlows      = 2048
	maxnoofRecommendedCells = 16
	maxnoofRecommendedENBs  = 16
	maxnooftimeperiods      = 2
)

type AdditionalCSFallbackIndicator uint8

const (
	AdditionalCSFallbackIndicatorNoRestriction AdditionalCSFallbackIndicator = iota
	AdditionalCSFallbackIndicatorRestriction
)

type AerialUEsubscriptionInformation uint8

const (
	AerialUEsubscriptionInformationAllowed AerialUEsubscriptionInformation = iota
	AerialUEsubscriptionInformationNotAllowed
)

type AssistanceDataForPaging struct {
	AssistanceDataForRecommendedCells *AssistanceDataForRecommendedCells `per:",optional"`
	AssistanceDataForCECapableUEs     *AssistanceDataForCECapableUEs     `per:",optional"`
	PagingAttemptInformation          *PagingAttemptInformation          `per:",optional"`
	iEExtensions                      extensionMeta                      `per:",exttable"`
	extensions                        per.ExtensionAdditions             `per:",extraw"`
}

type CEModeBRestricted uint8

const (
	CEModeBRestrictedRestricted CEModeBRestricted = iota
	CEModeBRestrictedNotRestricted
)

type CEmodeBSupportIndicator uint8

const (
	CEmodeBSupportIndicatorSupported CEmodeBSupportIndicator = iota
)

type CSFallbackIndicator uint8

const (
	CSFallbackIndicatorCSFallbackRequired CSFallbackIndicator = iota
	CSFallbackIndicatorCSFallbackHighPriority
)

type CSGId uint32

func (v CSGId) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return encodeBitStringUint(w, enc, uint64(v), 27)
}

func (v *CSGId) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	n, err := decodeBitStringUint(r, enc, 27)
	if err != nil {
		return err
	}

	*v = CSGId(n)

	return nil
}

type CSGIdList []CSGIdListItem

func (l CSGIdList) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return marshalSeqOf(w, enc, 1, maxnoofCSGs, []CSGIdListItem(l))
}

func (l *CSGIdList) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	items, err := unmarshalSeqOf[CSGIdListItem](r, enc, 1, maxnoofCSGs)
	if err != nil {
		return err
	}

	*l = items

	return nil
}

type CSGMembershipInfo struct {
	CSGMembershipStatus CSGMembershipStatus
	CSGId               CSGId
	CellAccessMode      *CellAccessMode        `per:",optional"`
	PLMNidentity        *PLMNIdentity          `per:",optional"`
	iEExtensions        extensionMeta          `per:",exttable"`
	extensions          per.ExtensionAdditions `per:",extraw"`
}

type CSGMembershipStatus uint8

const (
	CSGMembershipStatusMember CSGMembershipStatus = iota
	CSGMembershipStatusNotMember
)

type CellAccessMode uint8

const (
	CellAccessModeHybrid CellAccessMode = iota
)

type CellIdentifierAndCELevelForCECapableUEs struct {
	GlobalCellID EUTRANCGI
	CELevel      CELevel
	iEExtensions extensionMeta          `per:",exttable"`
	extensions   per.ExtensionAdditions `per:",extraw"`
}

type ConnectedengNBList []ConnectedengNBItem

func (l ConnectedengNBList) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return marshalSeqOf(w, enc, 1, maxnoofConnectedengNBs, []ConnectedengNBItem(l))
}

func (l *ConnectedengNBList) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	items, err := unmarshalSeqOf[ConnectedengNBItem](r, enc, 1, maxnoofConnectedengNBs)
	if err != nil {
		return err
	}

	*l = items

	return nil
}

type CoverageLevel uint8

const (
	CoverageLevelExtendedcoverage CoverageLevel = iota
)

type DCNID uint16

func (v DCNID) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return per.EncodeInteger(w, enc, per.Bounds{LB: 0, HasLB: true, UB: 65535, HasUB: true}, int64(v))
}

func (v *DCNID) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	n, err := per.DecodeInteger(r, enc, per.Bounds{LB: 0, HasLB: true, UB: 65535, HasUB: true})
	if err != nil {
		return err
	}

	*v = DCNID(n)

	return nil
}

type DLNASPDUDeliveryAckRequest uint8

const (
	DLNASPDUDeliveryAckRequestRequested DLNASPDUDeliveryAckRequest = iota
)

type DataSize int64

func (v DataSize) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return per.EncodeInteger(w, enc, per.Bounds{LB: 1, HasLB: true, UB: 4095, HasUB: true, Extensible: true}, int64(v))
}

func (v *DataSize) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	n, err := per.DecodeInteger(r, enc, per.Bounds{LB: 1, HasLB: true, UB: 4095, HasUB: true, Extensible: true})
	if err != nil {
		return err
	}

	*v = DataSize(n)

	return nil
}

type ERABToBeUpdatedList []ERABToBeUpdatedItem

func (l ERABToBeUpdatedList) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return encodeSingleContainerList(w, enc, maxnoofERABs, IDERABToBeUpdatedItem, CriticalityIgnore, l)
}

func (l *ERABToBeUpdatedList) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	items, err := decodeItemList[ERABToBeUpdatedItem](r, enc, maxnoofERABs, IDERABToBeUpdatedItem, CriticalityIgnore)
	*l = items

	return err
}

type EDTSession uint8

const (
	EDTSessionTrue EDTSession = iota
)

type ENDCSONConfigurationTransfer struct {
	Transfertype    ENDCSONTransferType
	SONInformation  SONInformation
	X2TNLConfigInfo *X2TNLConfigurationInfo `per:",optional"`
	iEExtensions    extensionMeta           `per:",exttable"`
	extensions      per.ExtensionAdditions  `per:",extraw"`
}

type EndIndication uint8

const (
	EndIndicationNoFurtherData EndIndication = iota
	EndIndicationFurtherDataExists
)

type EnhancedCoverageRestricted uint8

const (
	EnhancedCoverageRestrictedRestricted EnhancedCoverageRestricted = iota
)

type ExpectedUEBehaviour struct {
	ExpectedActivity   *ExpectedUEActivityBehaviour `per:",optional"`
	ExpectedHOInterval *ExpectedHOInterval          `per:",optional"`
	iEExtensions       extensionMeta                `per:",exttable"`
	extensions         per.ExtensionAdditions       `per:",extraw"`
}

type ExtendedUEIdentityIndexValue uint16

func (v ExtendedUEIdentityIndexValue) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return encodeBitStringUint(w, enc, uint64(v), 14)
}

func (v *ExtendedUEIdentityIndexValue) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	n, err := decodeBitStringUint(r, enc, 14)
	if err != nil {
		return err
	}

	*v = ExtendedUEIdentityIndexValue(n)

	return nil
}

type IABAuthorized uint8

const (
	IABAuthorizedAuthorized IABAuthorized = iota
	IABAuthorizedNotAuthorized
)

type IABNodeIndication uint8

const (
	IABNodeIndicationTrue IABNodeIndication = iota
)

type IABSupported uint8

const (
	IABSupportedTrue IABSupported = iota
)

type InformationOnRecommendedCellsAndENBsForPaging struct {
	RecommendedCellsForPaging RecommendedCellsForPaging
	RecommendENBsForPaging    RecommendedENBsForPaging
	iEExtensions              extensionMeta          `per:",exttable"`
	extensions                per.ExtensionAdditions `per:",extraw"`
}

type IntersystemSONConfigurationTransfer []byte

func (v IntersystemSONConfigurationTransfer) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return per.EncodeOctetString(w, enc, 0, 0, true, false, false, v)
}

func (v *IntersystemSONConfigurationTransfer) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	b, err := per.DecodeOctetString(r, enc, 0, 0, true, false, false)
	if err != nil {
		return err
	}

	*v = b

	return nil
}

type LHNID []byte

func (v LHNID) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return per.EncodeOctetString(w, enc, 32, 256, true, true, false, v)
}

func (v *LHNID) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	b, err := per.DecodeOctetString(r, enc, 32, 256, true, true, false)
	if err != nil {
		return err
	}

	*v = b

	return nil
}

type LTEMIndication uint8

const (
	LTEMIndicationLTEM LTEMIndication = iota
)

type MDTPLMNList []PLMNIdentity

func (l MDTPLMNList) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return marshalSeqOf(w, enc, 1, maxnoofMDTPLMNs, []PLMNIdentity(l))
}

func (l *MDTPLMNList) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	items, err := unmarshalSeqOf[PLMNIdentity](r, enc, 1, maxnoofMDTPLMNs)
	if err != nil {
		return err
	}

	*l = items

	return nil
}

type MMERelaySupportIndicator uint8

const (
	MMERelaySupportIndicatorTrue MMERelaySupportIndicator = iota
)

type MSClassmark2 []byte

func (v MSClassmark2) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return per.EncodeOctetString(w, enc, 0, 0, true, false, false, v)
}

func (v *MSClassmark2) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	b, err := per.DecodeOctetString(r, enc, 0, 0, true, false, false)
	if err != nil {
		return err
	}

	*v = b

	return nil
}

type MSClassmark3 []byte

func (v MSClassmark3) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return per.EncodeOctetString(w, enc, 0, 0, true, false, false, v)
}

func (v *MSClassmark3) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	b, err := per.DecodeOctetString(r, enc, 0, 0, true, false, false)
	if err != nil {
		return err
	}

	*v = b

	return nil
}

type ManagementBasedMDTAllowed uint8

const (
	ManagementBasedMDTAllowedAllowed ManagementBasedMDTAllowed = iota
)

type MaskedIMEISV uint64

func (v MaskedIMEISV) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return encodeBitStringUint(w, enc, uint64(v), 64)
}

func (v *MaskedIMEISV) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	n, err := decodeBitStringUint(r, enc, 64)
	if err != nil {
		return err
	}

	*v = MaskedIMEISV(n)

	return nil
}

type NBIoTDefaultPagingDRX uint8

const (
	NBIoTDefaultPagingDRXV128 NBIoTDefaultPagingDRX = iota
	NBIoTDefaultPagingDRXV256
	NBIoTDefaultPagingDRXV512
	NBIoTDefaultPagingDRXV1024
)

type NBIoTPagingeDRXInformation struct {
	NBIoTpagingeDRXCycle  NBIoTPagingeDRXCycle
	NBIoTpagingTimeWindow *NBIoTPagingTimeWindow `per:",optional"`
	iEExtensions          extensionMeta          `per:",exttable"`
	extensions            per.ExtensionAdditions `per:",extraw"`
}

type NBIoTPagingDRX uint8

const (
	NBIoTPagingDRXV32 NBIoTPagingDRX = iota
	NBIoTPagingDRXV64
	NBIoTPagingDRXV128
	NBIoTPagingDRXV256
	NBIoTPagingDRXV512
	NBIoTPagingDRXV1024
)

type NBIoTUEIdentityIndexValue uint16

func (v NBIoTUEIdentityIndexValue) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return encodeBitStringUint(w, enc, uint64(v), 12)
}

func (v *NBIoTUEIdentityIndexValue) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	n, err := decodeBitStringUint(r, enc, 12)
	if err != nil {
		return err
	}

	*v = NBIoTUEIdentityIndexValue(n)

	return nil
}

type NRUESecurityCapabilities struct {
	NRencryptionAlgorithms          NRencryptionAlgorithms
	NRintegrityProtectionAlgorithms NRintegrityProtectionAlgorithms
	iEExtensions                    extensionMeta          `per:",exttable"`
	extensions                      per.ExtensionAdditions `per:",extraw"`
}

type NRUESidelinkAggregateMaximumBitrate struct {
	UEaggregateMaximumBitRate BitRate
	iEExtensions              extensionMeta          `per:",exttable"`
	extensions                per.ExtensionAdditions `per:",extraw"`
}

type NRV2XServicesAuthorized struct {
	VehicleUE    *VehicleUE             `per:",optional"`
	PedestrianUE *PedestrianUE          `per:",optional"`
	iEExtensions extensionMeta          `per:",exttable"`
	extensions   per.ExtensionAdditions `per:",extraw"`
}

type NotifySourceeNB uint8

const (
	NotifySourceeNBNotifySource NotifySourceeNB = iota
)

type PC5QoSParameters struct {
	Pc5QoSFlowList            PC5QoSFlowList
	Pc5LinkAggregatedBitRates *BitRate               `per:",optional"`
	iEExtensions              extensionMeta          `per:",exttable"`
	extensions                per.ExtensionAdditions `per:",extraw"`
}

type PSServiceNotAvailable uint8

const (
	PSServiceNotAvailablePSServiceNotAvailable PSServiceNotAvailable = iota
)

type PagingeDRXInformation struct {
	PagingeDRXCycle  PagingeDRXCycle
	PagingTimeWindow *PagingTimeWindow      `per:",optional"`
	iEExtensions     extensionMeta          `per:",exttable"`
	extensions       per.ExtensionAdditions `per:",extraw"`
}

type PagingCause uint8

const (
	PagingCauseVoice PagingCause = iota
)

type PendingDataIndication uint8

const (
	PendingDataIndicationTrue PendingDataIndication = iota
)

type ProSeAuthorized struct {
	ProSeDirectDiscovery     *ProSeDirectDiscovery     `per:",optional"`
	ProSeDirectCommunication *ProSeDirectCommunication `per:",optional"`
	ProSeUEtoNetworkRelaying *ProSeUEtoNetworkRelaying `per:",extie:IDProSeUEtoNetworkRelaying,crit:ignore"`
	iEExtensions             extensionMeta             `per:",exttable"`
	extensions               per.ExtensionAdditions    `per:",extraw"`
}

type RelayNodeIndicator uint8

const (
	RelayNodeIndicatorTrue RelayNodeIndicator = iota
)

type SRVCCHOIndication uint8

const (
	SRVCCHOIndicationPSandCS SRVCCHOIndication = iota
	SRVCCHOIndicationCSonly
)

type SRVCCOperationNotPossible uint8

const (
	SRVCCOperationNotPossibleNotPossible SRVCCOperationNotPossible = iota
)

type SRVCCOperationPossible uint8

const (
	SRVCCOperationPossiblePossible SRVCCOperationPossible = iota
)

type SecondaryRATDataUsageReportList []SecondaryRATDataUsageReportItem

func (l SecondaryRATDataUsageReportList) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return encodeSingleContainerList(w, enc, maxnoofERABs, IDSecondaryRATDataUsageReportItem, CriticalityIgnore, l)
}

func (l *SecondaryRATDataUsageReportList) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	items, err := decodeItemList[SecondaryRATDataUsageReportItem](r, enc, maxnoofERABs, IDSecondaryRATDataUsageReportItem, CriticalityIgnore)
	*l = items

	return err
}

type SecondaryRATDataUsageRequest uint8

const (
	SecondaryRATDataUsageRequestRequested SecondaryRATDataUsageRequest = iota
)

type ServedDCNs []ServedDCNsItem

func (l ServedDCNs) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return marshalSeqOf(w, enc, 0, maxnoofDCNs, []ServedDCNsItem(l))
}

func (l *ServedDCNs) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	items, err := unmarshalSeqOf[ServedDCNsItem](r, enc, 0, maxnoofDCNs)
	if err != nil {
		return err
	}

	*l = items

	return nil
}

type SubscriptionBasedUEDifferentiationInfoPeriodicCommunicationIndicator uint8

const (
	SubscriptionBasedUEDifferentiationInfoPeriodicCommunicationIndicatorPeriodically SubscriptionBasedUEDifferentiationInfoPeriodicCommunicationIndicator = iota
	SubscriptionBasedUEDifferentiationInfoPeriodicCommunicationIndicatorOndemand
)

type SubscriptionBasedUEDifferentiationInfoPeriodicTime int64

func (v SubscriptionBasedUEDifferentiationInfoPeriodicTime) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return per.EncodeInteger(w, enc, per.Bounds{LB: 1, HasLB: true, UB: 3600, HasUB: true, Extensible: true}, int64(v))
}

func (v *SubscriptionBasedUEDifferentiationInfoPeriodicTime) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	n, err := per.DecodeInteger(r, enc, per.Bounds{LB: 1, HasLB: true, UB: 3600, HasUB: true, Extensible: true})
	if err != nil {
		return err
	}

	*v = SubscriptionBasedUEDifferentiationInfoPeriodicTime(n)

	return nil
}

type SubscriptionBasedUEDifferentiationInfoStationaryIndication uint8

const (
	SubscriptionBasedUEDifferentiationInfoStationaryIndicationStationary SubscriptionBasedUEDifferentiationInfoStationaryIndication = iota
	SubscriptionBasedUEDifferentiationInfoStationaryIndicationMobile
)

type SubscriptionBasedUEDifferentiationInfoTrafficProfile uint8

const (
	SubscriptionBasedUEDifferentiationInfoTrafficProfileSinglePacket SubscriptionBasedUEDifferentiationInfoTrafficProfile = iota
	SubscriptionBasedUEDifferentiationInfoTrafficProfileDualPackets
	SubscriptionBasedUEDifferentiationInfoTrafficProfileMultiplePackets
)

type SubscriptionBasedUEDifferentiationInfoBatteryIndication uint8

const (
	SubscriptionBasedUEDifferentiationInfoBatteryIndicationBatteryPowered SubscriptionBasedUEDifferentiationInfoBatteryIndication = iota
	SubscriptionBasedUEDifferentiationInfoBatteryIndicationBatteryPoweredNotRechargeableOrReplaceable
	SubscriptionBasedUEDifferentiationInfoBatteryIndicationNotBatteryPowered
)

type SubscriptionBasedUEDifferentiationInfo struct {
	PeriodicCommunicationIndicator *SubscriptionBasedUEDifferentiationInfoPeriodicCommunicationIndicator `per:",optional"`
	PeriodicTime                   *SubscriptionBasedUEDifferentiationInfoPeriodicTime                   `per:",optional"`
	ScheduledCommunicationTime     *ScheduledCommunicationTime                                           `per:",optional"`
	StationaryIndication           *SubscriptionBasedUEDifferentiationInfoStationaryIndication           `per:",optional"`
	TrafficProfile                 *SubscriptionBasedUEDifferentiationInfoTrafficProfile                 `per:",optional"`
	BatteryIndication              *SubscriptionBasedUEDifferentiationInfoBatteryIndication              `per:",optional"`
	iEExtensions                   extensionMeta                                                         `per:",exttable"`
	extensions                     per.ExtensionAdditions                                                `per:",extraw"`
}

type TimeSinceSecondaryNodeRelease []byte

func (v TimeSinceSecondaryNodeRelease) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return per.EncodeOctetString(w, enc, 4, 4, true, true, false, v)
}

func (v *TimeSinceSecondaryNodeRelease) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	b, err := per.DecodeOctetString(r, enc, 4, 4, true, true, false)
	if err != nil {
		return err
	}

	*v = b

	return nil
}

type UEApplicationLayerMeasurementCapability uint8

func (v UEApplicationLayerMeasurementCapability) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return encodeBitStringUint(w, enc, uint64(v), 8)
}

func (v *UEApplicationLayerMeasurementCapability) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	n, err := decodeBitStringUint(r, enc, 8)
	if err != nil {
		return err
	}

	*v = UEApplicationLayerMeasurementCapability(n)

	return nil
}

type UEUsageType uint8

func (v UEUsageType) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return per.EncodeInteger(w, enc, per.Bounds{LB: 0, HasLB: true, UB: 255, HasUB: true}, int64(v))
}

func (v *UEUsageType) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	n, err := per.DecodeInteger(r, enc, per.Bounds{LB: 0, HasLB: true, UB: 255, HasUB: true})
	if err != nil {
		return err
	}

	*v = UEUsageType(n)

	return nil
}

type UECapabilityInfoRequest uint8

const (
	UECapabilityInfoRequestRequested UECapabilityInfoRequest = iota
)

type UERadioCapabilityID []byte

func (v UERadioCapabilityID) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return per.EncodeOctetString(w, enc, 0, 0, true, false, false, v)
}

func (v *UERadioCapabilityID) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	b, err := per.DecodeOctetString(r, enc, 0, 0, true, false, false)
	if err != nil {
		return err
	}

	*v = b

	return nil
}

type UESidelinkAggregateMaximumBitrate struct {
	UESidelinkAggregateMaximumBitRate BitRate
	iEExtensions                      extensionMeta          `per:",exttable"`
	extensions                        per.ExtensionAdditions `per:",extraw"`
}

type UEUserPlaneCIoTSupportIndicator uint8

const (
	UEUserPlaneCIoTSupportIndicatorSupported UEUserPlaneCIoTSupportIndicator = iota
)

type V2XServicesAuthorized struct {
	VehicleUE    *VehicleUE             `per:",optional"`
	PedestrianUE *PedestrianUE          `per:",optional"`
	iEExtensions extensionMeta          `per:",exttable"`
	extensions   per.ExtensionAdditions `per:",extraw"`
}

type WUSAssistanceInformation struct {
	PagingProbabilityInformation PagingProbabilityInformation
	iEExtensions                 extensionMeta          `per:",exttable"`
	extensions                   per.ExtensionAdditions `per:",extraw"`
}

type AssistanceDataForRecommendedCells struct {
	RecommendedCellsForPaging RecommendedCellsForPaging
	iEExtensions              extensionMeta          `per:",exttable"`
	extensions                per.ExtensionAdditions `per:",extraw"`
}

type AssistanceDataForCECapableUEs struct {
	CellIdentifierAndCELevelForCECapableUEs CellIdentifierAndCELevelForCECapableUEs
	iEExtensions                            extensionMeta          `per:",exttable"`
	extensions                              per.ExtensionAdditions `per:",extraw"`
}

type PagingAttemptInformation struct {
	PagingAttemptCount             PagingAttemptCount
	IntendedNumberOfPagingAttempts IntendedNumberOfPagingAttempts
	NextPagingAreaScope            *NextPagingAreaScope   `per:",optional"`
	iEExtensions                   extensionMeta          `per:",exttable"`
	extensions                     per.ExtensionAdditions `per:",extraw"`
}

type CSGIdListItem struct {
	CSGId        CSGId
	iEExtensions extensionMeta          `per:",exttable"`
	extensions   per.ExtensionAdditions `per:",extraw"`
}

type CELevel []byte

func (v CELevel) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return per.EncodeOctetString(w, enc, 0, 0, true, false, false, v)
}

func (v *CELevel) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	b, err := per.DecodeOctetString(r, enc, 0, 0, true, false, false)
	if err != nil {
		return err
	}

	*v = b

	return nil
}

type ConnectedengNBItem struct {
	EngNBID      EngNBID
	SupportedTAs SupportedTAs
	iEExtensions extensionMeta          `per:",exttable"`
	extensions   per.ExtensionAdditions `per:",extraw"`
}

type ERABToBeUpdatedItem struct {
	ERABID             ERABID
	SecurityIndication *SecurityIndication    `per:",optional"`
	iEExtensions       extensionMeta          `per:",exttable"`
	extensions         per.ExtensionAdditions `per:",extraw"`
	containerCrit      `per:"-"`
}

type ENDCSONTransferType struct {
	Request *ENDCTransferTypeRequest  `per:",choice:0"`
	Reply   *ENDCTransferTypeReply    `per:",choice:1"`
	Ext     *per.ExtensionAlternative `per:",extalt"`
}

type ExpectedUEActivityBehaviour struct {
	ExpectedActivityPeriod                 *ExpectedActivityPeriod                 `per:",optional"`
	ExpectedIdlePeriod                     *ExpectedIdlePeriod                     `per:",optional"`
	SourceofUEActivityBehaviourInformation *SourceOfUEActivityBehaviourInformation `per:",optional"`
	iEExtensions                           extensionMeta                           `per:",exttable"`
	extensions                             per.ExtensionAdditions                  `per:",extraw"`
}

type ExpectedHOInterval uint8

const (
	ExpectedHOIntervalSec15 ExpectedHOInterval = iota
	ExpectedHOIntervalSec30
	ExpectedHOIntervalSec60
	ExpectedHOIntervalSec90
	ExpectedHOIntervalSec120
	ExpectedHOIntervalSec180
	ExpectedHOIntervalLongTime
)

type RecommendedCellsForPaging struct {
	RecommendedCellList RecommendedCellList
	iEExtensions        extensionMeta          `per:",exttable"`
	extensions          per.ExtensionAdditions `per:",extraw"`
}

type RecommendedENBsForPaging struct {
	RecommendedENBList RecommendedENBList
	iEExtensions       extensionMeta          `per:",exttable"`
	extensions         per.ExtensionAdditions `per:",extraw"`
}

type NBIoTPagingeDRXCycle uint8

const (
	NBIoTPagingeDRXCycleHf2 NBIoTPagingeDRXCycle = iota
	NBIoTPagingeDRXCycleHf4
	NBIoTPagingeDRXCycleHf6
	NBIoTPagingeDRXCycleHf8
	NBIoTPagingeDRXCycleHf10
	NBIoTPagingeDRXCycleHf12
	NBIoTPagingeDRXCycleHf14
	NBIoTPagingeDRXCycleHf16
	NBIoTPagingeDRXCycleHf32
	NBIoTPagingeDRXCycleHf64
	NBIoTPagingeDRXCycleHf128
	NBIoTPagingeDRXCycleHf256
	NBIoTPagingeDRXCycleHf512
	NBIoTPagingeDRXCycleHf1024
)

type NBIoTPagingTimeWindow uint8

const (
	NBIoTPagingTimeWindowS1 NBIoTPagingTimeWindow = iota
	NBIoTPagingTimeWindowS2
	NBIoTPagingTimeWindowS3
	NBIoTPagingTimeWindowS4
	NBIoTPagingTimeWindowS5
	NBIoTPagingTimeWindowS6
	NBIoTPagingTimeWindowS7
	NBIoTPagingTimeWindowS8
	NBIoTPagingTimeWindowS9
	NBIoTPagingTimeWindowS10
	NBIoTPagingTimeWindowS11
	NBIoTPagingTimeWindowS12
	NBIoTPagingTimeWindowS13
	NBIoTPagingTimeWindowS14
	NBIoTPagingTimeWindowS15
	NBIoTPagingTimeWindowS16
)

type NRencryptionAlgorithms struct{ BitString }

func (v NRencryptionAlgorithms) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return v.encode(w, enc, 16, 16, true)
}

func (v *NRencryptionAlgorithms) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	b, err := decodeBitString(r, enc, 16, 16, true)
	if err != nil {
		return err
	}

	*v = NRencryptionAlgorithms{b}

	return nil
}

type NRintegrityProtectionAlgorithms struct{ BitString }

func (v NRintegrityProtectionAlgorithms) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return v.encode(w, enc, 16, 16, true)
}

func (v *NRintegrityProtectionAlgorithms) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	b, err := decodeBitString(r, enc, 16, 16, true)
	if err != nil {
		return err
	}

	*v = NRintegrityProtectionAlgorithms{b}

	return nil
}

type VehicleUE uint8

const (
	VehicleUEAuthorized VehicleUE = iota
	VehicleUENotAuthorized
)

type PedestrianUE uint8

const (
	PedestrianUEAuthorized PedestrianUE = iota
	PedestrianUENotAuthorized
)

type PC5QoSFlowList []PC5QoSFlowItem

func (l PC5QoSFlowList) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return marshalSeqOf(w, enc, 1, maxnoofPC5QoSFlows, []PC5QoSFlowItem(l))
}

func (l *PC5QoSFlowList) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	items, err := unmarshalSeqOf[PC5QoSFlowItem](r, enc, 1, maxnoofPC5QoSFlows)
	if err != nil {
		return err
	}

	*l = items

	return nil
}

type PagingeDRXCycle uint8

const (
	PagingeDRXCycleHfhalf PagingeDRXCycle = iota
	PagingeDRXCycleHf1
	PagingeDRXCycleHf2
	PagingeDRXCycleHf4
	PagingeDRXCycleHf6
	PagingeDRXCycleHf8
	PagingeDRXCycleHf10
	PagingeDRXCycleHf12
	PagingeDRXCycleHf14
	PagingeDRXCycleHf16
	PagingeDRXCycleHf32
	PagingeDRXCycleHf64
	PagingeDRXCycleHf128
	PagingeDRXCycleHf256
)

type PagingTimeWindow uint8

const (
	PagingTimeWindowS1 PagingTimeWindow = iota
	PagingTimeWindowS2
	PagingTimeWindowS3
	PagingTimeWindowS4
	PagingTimeWindowS5
	PagingTimeWindowS6
	PagingTimeWindowS7
	PagingTimeWindowS8
	PagingTimeWindowS9
	PagingTimeWindowS10
	PagingTimeWindowS11
	PagingTimeWindowS12
	PagingTimeWindowS13
	PagingTimeWindowS14
	PagingTimeWindowS15
	PagingTimeWindowS16
)

type ProSeDirectDiscovery uint8

const (
	ProSeDirectDiscoveryAuthorized ProSeDirectDiscovery = iota
	ProSeDirectDiscoveryNotAuthorized
)

type ProSeDirectCommunication uint8

const (
	ProSeDirectCommunicationAuthorized ProSeDirectCommunication = iota
	ProSeDirectCommunicationNotAuthorized
)

type ProSeUEtoNetworkRelaying uint8

const (
	ProSeUEtoNetworkRelayingAuthorized ProSeUEtoNetworkRelaying = iota
	ProSeUEtoNetworkRelayingNotAuthorized
)

type SecondaryRATDataUsageReportItem struct {
	ERABID              ERABID
	SecondaryRATType    SecondaryRATType
	ERABUsageReportList ERABUsageReportList
	iEExtensions        extensionMeta          `per:",exttable"`
	extensions          per.ExtensionAdditions `per:",extraw"`
	containerCrit       `per:"-"`
}

type ServedDCNsItem struct {
	DCNID               DCNID
	RelativeDCNCapacity RelativeMMECapacity
	iEExtensions        extensionMeta          `per:",exttable"`
	extensions          per.ExtensionAdditions `per:",extraw"`
}

type ScheduledCommunicationTimeDayofWeek uint8

func (v ScheduledCommunicationTimeDayofWeek) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return encodeBitStringUint(w, enc, uint64(v), 7)
}

func (v *ScheduledCommunicationTimeDayofWeek) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	n, err := decodeBitStringUint(r, enc, 7)
	if err != nil {
		return err
	}

	*v = ScheduledCommunicationTimeDayofWeek(n)

	return nil
}

type ScheduledCommunicationTimeTimeofDayStart int64

func (v ScheduledCommunicationTimeTimeofDayStart) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return per.EncodeInteger(w, enc, per.Bounds{LB: 0, HasLB: true, UB: 86399, HasUB: true, Extensible: true}, int64(v))
}

func (v *ScheduledCommunicationTimeTimeofDayStart) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	n, err := per.DecodeInteger(r, enc, per.Bounds{LB: 0, HasLB: true, UB: 86399, HasUB: true, Extensible: true})
	if err != nil {
		return err
	}

	*v = ScheduledCommunicationTimeTimeofDayStart(n)

	return nil
}

type ScheduledCommunicationTimeTimeofDayEnd int64

func (v ScheduledCommunicationTimeTimeofDayEnd) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return per.EncodeInteger(w, enc, per.Bounds{LB: 0, HasLB: true, UB: 86399, HasUB: true, Extensible: true}, int64(v))
}

func (v *ScheduledCommunicationTimeTimeofDayEnd) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	n, err := per.DecodeInteger(r, enc, per.Bounds{LB: 0, HasLB: true, UB: 86399, HasUB: true, Extensible: true})
	if err != nil {
		return err
	}

	*v = ScheduledCommunicationTimeTimeofDayEnd(n)

	return nil
}

type ScheduledCommunicationTime struct {
	DayofWeek      *ScheduledCommunicationTimeDayofWeek      `per:",optional"`
	TimeofDayStart *ScheduledCommunicationTimeTimeofDayStart `per:",optional"`
	TimeofDayEnd   *ScheduledCommunicationTimeTimeofDayEnd   `per:",optional"`
	iEExtensions   extensionMeta                             `per:",exttable"`
	extensions     per.ExtensionAdditions                    `per:",extraw"`
}

type PagingProbabilityInformation uint8

const (
	PagingProbabilityInformationP00 PagingProbabilityInformation = iota
	PagingProbabilityInformationP05
	PagingProbabilityInformationP10
	PagingProbabilityInformationP15
	PagingProbabilityInformationP20
	PagingProbabilityInformationP25
	PagingProbabilityInformationP30
	PagingProbabilityInformationP35
	PagingProbabilityInformationP40
	PagingProbabilityInformationP45
	PagingProbabilityInformationP50
	PagingProbabilityInformationP55
	PagingProbabilityInformationP60
	PagingProbabilityInformationP65
	PagingProbabilityInformationP70
	PagingProbabilityInformationP75
	PagingProbabilityInformationP80
	PagingProbabilityInformationP85
	PagingProbabilityInformationP90
	PagingProbabilityInformationP95
	PagingProbabilityInformationP100
)

type PagingAttemptCount int64

func (v PagingAttemptCount) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return per.EncodeInteger(w, enc, per.Bounds{LB: 1, HasLB: true, UB: 16, HasUB: true, Extensible: true}, int64(v))
}

func (v *PagingAttemptCount) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	n, err := per.DecodeInteger(r, enc, per.Bounds{LB: 1, HasLB: true, UB: 16, HasUB: true, Extensible: true})
	if err != nil {
		return err
	}

	*v = PagingAttemptCount(n)

	return nil
}

type IntendedNumberOfPagingAttempts int64

func (v IntendedNumberOfPagingAttempts) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return per.EncodeInteger(w, enc, per.Bounds{LB: 1, HasLB: true, UB: 16, HasUB: true, Extensible: true}, int64(v))
}

func (v *IntendedNumberOfPagingAttempts) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	n, err := per.DecodeInteger(r, enc, per.Bounds{LB: 1, HasLB: true, UB: 16, HasUB: true, Extensible: true})
	if err != nil {
		return err
	}

	*v = IntendedNumberOfPagingAttempts(n)

	return nil
}

type NextPagingAreaScope uint8

const (
	NextPagingAreaScopeSame NextPagingAreaScope = iota
	NextPagingAreaScopeChanged
)

type EngNBID struct{ BitString }

func (v EngNBID) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return v.encode(w, enc, 22, 32, true)
}

func (v *EngNBID) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	b, err := decodeBitString(r, enc, 22, 32, true)
	if err != nil {
		return err
	}

	*v = EngNBID{b}

	return nil
}

type ENDCTransferTypeRequest struct {
	SourceeNB       ENDCSONeNBIdentification
	TargetengNB     ENDCSONengNBIdentification
	TargeteNB       *ENDCSONeNBIdentification `per:",optional"`
	AssociatedTAI   *TAI                      `per:",optional"`
	Broadcast5GSTAI *FiveGSTAI                `per:",optional"`
	iEExtensions    extensionMeta             `per:",exttable"`
	extensions      per.ExtensionAdditions    `per:",extraw"`
}

type ENDCTransferTypeReply struct {
	SourceengNB  ENDCSONengNBIdentification
	TargeteNB    ENDCSONeNBIdentification
	iEExtensions extensionMeta          `per:",exttable"`
	extensions   per.ExtensionAdditions `per:",extraw"`
}

type ExpectedActivityPeriod int64

func (v ExpectedActivityPeriod) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return per.EncodeInteger(w, enc, per.Bounds{LB: 1, HasLB: true, UB: 181, HasUB: true, Extensible: true}, int64(v))
}

func (v *ExpectedActivityPeriod) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	n, err := per.DecodeInteger(r, enc, per.Bounds{LB: 1, HasLB: true, UB: 181, HasUB: true, Extensible: true})
	if err != nil {
		return err
	}

	*v = ExpectedActivityPeriod(n)

	return nil
}

type ExpectedIdlePeriod int64

func (v ExpectedIdlePeriod) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return per.EncodeInteger(w, enc, per.Bounds{LB: 1, HasLB: true, UB: 181, HasUB: true, Extensible: true}, int64(v))
}

func (v *ExpectedIdlePeriod) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	n, err := per.DecodeInteger(r, enc, per.Bounds{LB: 1, HasLB: true, UB: 181, HasUB: true, Extensible: true})
	if err != nil {
		return err
	}

	*v = ExpectedIdlePeriod(n)

	return nil
}

type SourceOfUEActivityBehaviourInformation uint8

const (
	SourceOfUEActivityBehaviourInformationSubscriptionInformation SourceOfUEActivityBehaviourInformation = iota
	SourceOfUEActivityBehaviourInformationStatistics
)

type RecommendedCellList []RecommendedCellItem

func (l RecommendedCellList) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return encodeSingleContainerList(w, enc, maxnoofRecommendedCells, IDRecommendedCellItem, CriticalityIgnore, l)
}

func (l *RecommendedCellList) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	items, err := decodeItemList[RecommendedCellItem](r, enc, maxnoofRecommendedCells, IDRecommendedCellItem, CriticalityIgnore)
	*l = items

	return err
}

type RecommendedENBList []RecommendedENBItem

func (l RecommendedENBList) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return encodeSingleContainerList(w, enc, maxnoofRecommendedENBs, IDRecommendedENBItem, CriticalityIgnore, l)
}

func (l *RecommendedENBList) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	items, err := decodeItemList[RecommendedENBItem](r, enc, maxnoofRecommendedENBs, IDRecommendedENBItem, CriticalityIgnore)
	*l = items

	return err
}

type PC5QoSFlowItem struct {
	PQI             FiveQI
	Pc5FlowBitRates *PC5FlowBitRates       `per:",optional"`
	Range           *RangeType             `per:",optional"`
	iEExtensions    extensionMeta          `per:",exttable"`
	extensions      per.ExtensionAdditions `per:",extraw"`
}

type SecondaryRATType uint8

const (
	SecondaryRATTypeNR SecondaryRATType = iota
	SecondaryRATTypeUnlicensed
)

type ERABUsageReportList []ERABUsageReportItem

func (l ERABUsageReportList) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return encodeSingleContainerList(w, enc, maxnooftimeperiods, IDERABUsageReportItem, CriticalityIgnore, l)
}

func (l *ERABUsageReportList) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	items, err := decodeItemList[ERABUsageReportItem](r, enc, maxnooftimeperiods, IDERABUsageReportItem, CriticalityIgnore)
	*l = items

	return err
}

type RelativeMMECapacity uint8

func (v RelativeMMECapacity) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return per.EncodeInteger(w, enc, per.Bounds{LB: 0, HasLB: true, UB: 255, HasUB: true}, int64(v))
}

func (v *RelativeMMECapacity) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	n, err := per.DecodeInteger(r, enc, per.Bounds{LB: 0, HasLB: true, UB: 255, HasUB: true})
	if err != nil {
		return err
	}

	*v = RelativeMMECapacity(n)

	return nil
}

type ENDCSONeNBIdentification struct {
	GlobaleNBID  GlobalENBID
	SelectedTAI  TAI
	iEExtensions extensionMeta          `per:",exttable"`
	extensions   per.ExtensionAdditions `per:",extraw"`
}

type ENDCSONengNBIdentification struct {
	GlobalengNBID GlobalengNBID
	SelectedTAI   TAI
	iEExtensions  extensionMeta          `per:",exttable"`
	extensions    per.ExtensionAdditions `per:",extraw"`
}

type RecommendedCellItemTimeStayedInCell uint16

func (v RecommendedCellItemTimeStayedInCell) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return per.EncodeInteger(w, enc, per.Bounds{LB: 0, HasLB: true, UB: 4095, HasUB: true}, int64(v))
}

func (v *RecommendedCellItemTimeStayedInCell) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	n, err := per.DecodeInteger(r, enc, per.Bounds{LB: 0, HasLB: true, UB: 4095, HasUB: true})
	if err != nil {
		return err
	}

	*v = RecommendedCellItemTimeStayedInCell(n)

	return nil
}

type RecommendedCellItem struct {
	EUTRANCGI        EUTRANCGI
	TimeStayedInCell *RecommendedCellItemTimeStayedInCell `per:",optional"`
	iEExtensions     extensionMeta                        `per:",exttable"`
	extensions       per.ExtensionAdditions               `per:",extraw"`
	containerCrit    `per:"-"`
}

type RecommendedENBItem struct {
	MMEPagingTarget MMEPagingTarget
	iEExtensions    extensionMeta          `per:",exttable"`
	extensions      per.ExtensionAdditions `per:",extraw"`
	containerCrit   `per:"-"`
}

type FiveQI int64

func (v FiveQI) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return per.EncodeInteger(w, enc, per.Bounds{LB: 0, HasLB: true, UB: 255, HasUB: true, Extensible: true}, int64(v))
}

func (v *FiveQI) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	n, err := per.DecodeInteger(r, enc, per.Bounds{LB: 0, HasLB: true, UB: 255, HasUB: true, Extensible: true})
	if err != nil {
		return err
	}

	*v = FiveQI(n)

	return nil
}

type PC5FlowBitRates struct {
	GuaranteedFlowBitRate BitRate
	MaximumFlowBitRate    BitRate
	iEExtensions          extensionMeta          `per:",exttable"`
	extensions            per.ExtensionAdditions `per:",extraw"`
}

type RangeType uint8

const (
	RangeTypeM50 RangeType = iota
	RangeTypeM80
	RangeTypeM180
	RangeTypeM200
	RangeTypeM350
	RangeTypeM400
	RangeTypeM500
	RangeTypeM700
	RangeTypeM1000
)

type ERABUsageReportItemStartTimestamp []byte

func (v ERABUsageReportItemStartTimestamp) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return per.EncodeOctetString(w, enc, 4, 4, true, true, false, v)
}

func (v *ERABUsageReportItemStartTimestamp) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	b, err := per.DecodeOctetString(r, enc, 4, 4, true, true, false)
	if err != nil {
		return err
	}

	*v = b

	return nil
}

type ERABUsageReportItemEndTimestamp []byte

func (v ERABUsageReportItemEndTimestamp) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return per.EncodeOctetString(w, enc, 4, 4, true, true, false, v)
}

func (v *ERABUsageReportItemEndTimestamp) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	b, err := per.DecodeOctetString(r, enc, 4, 4, true, true, false)
	if err != nil {
		return err
	}

	*v = b

	return nil
}

type ERABUsageReportItemUsageCountUL uint64

func (v ERABUsageReportItemUsageCountUL) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return encodeUint64Range(w, enc, uint64(v))
}

func (v *ERABUsageReportItemUsageCountUL) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	n, err := decodeUint64Range(r, enc)
	*v = ERABUsageReportItemUsageCountUL(n)

	return err
}

type ERABUsageReportItemUsageCountDL uint64

func (v ERABUsageReportItemUsageCountDL) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return encodeUint64Range(w, enc, uint64(v))
}

func (v *ERABUsageReportItemUsageCountDL) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	n, err := decodeUint64Range(r, enc)
	*v = ERABUsageReportItemUsageCountDL(n)

	return err
}

type ERABUsageReportItem struct {
	StartTimestamp ERABUsageReportItemStartTimestamp
	EndTimestamp   ERABUsageReportItemEndTimestamp
	UsageCountUL   ERABUsageReportItemUsageCountUL
	UsageCountDL   ERABUsageReportItemUsageCountDL
	iEExtensions   extensionMeta          `per:",exttable"`
	extensions     per.ExtensionAdditions `per:",extraw"`
	containerCrit  `per:"-"`
}

type GlobalengNBID struct {
	PLMNidentity PLMNIdentity
	EngNBID      EngNBID
	iEExtensions extensionMeta          `per:",exttable"`
	extensions   per.ExtensionAdditions `per:",extraw"`
}

type MMEPagingTarget struct {
	GlobalENBID *GlobalENBID              `per:",choice:0"`
	TAI         *TAI                      `per:",choice:1"`
	Ext         *per.ExtensionAlternative `per:",extalt"`
}

func (l *ERABToBeUpdatedList) Diagnose(d *ap.Diagnosis[ProtocolIEID]) bool {
	return diagnoseItemList(d, IDERABToBeUpdatedItem, CriticalityIgnore, l)
}

func (l *SecondaryRATDataUsageReportList) Diagnose(d *ap.Diagnosis[ProtocolIEID]) bool {
	return diagnoseItemList(d, IDSecondaryRATDataUsageReportItem, CriticalityIgnore, l)
}

func (l *RecommendedCellList) Diagnose(d *ap.Diagnosis[ProtocolIEID]) bool {
	return diagnoseItemList(d, IDRecommendedCellItem, CriticalityIgnore, l)
}

func (l *RecommendedENBList) Diagnose(d *ap.Diagnosis[ProtocolIEID]) bool {
	return diagnoseItemList(d, IDRecommendedENBItem, CriticalityIgnore, l)
}

func (l *ERABUsageReportList) Diagnose(d *ap.Diagnosis[ProtocolIEID]) bool {
	return diagnoseItemList(d, IDERABUsageReportItem, CriticalityIgnore, l)
}
