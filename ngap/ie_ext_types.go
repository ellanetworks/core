// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"github.com/ellanetworks/core/per"
	"github.com/ellanetworks/core/per/ap"
	"github.com/ellanetworks/core/per/ap/apimpl"
)

const (
	maxnoofCAGSperCell                          = 64
	maxnoofCandidateCells                       = 32
	maxnoofCellsUEMovingTrajectory              = 16
	maxnoofCellsinNGRANNode                     = 16384
	maxnoofPC5QoSFlows                          = 2048
	maxnoofRSPPQoSFlows                         = 2048
	maxnoofRecommendedCells                     = 16
	maxnoofRecommendedRANNodes                  = 16
	maxnoofReportedCells                        = 256
	maxnoofTAIforInactive                       = 16
	maxnoofTargetSNSSAIs                        = 8
	maxNRARFCN                                  = 3279165
	maxnoofAllowedCAGsperPLMN                   = 256
	maxnoofAreaNTN                              = 8
	maxnoofBluetoothName                        = 4
	maxnoofCAGforMDT                            = 256
	maxnoofCandidateRelayUEs                    = 32
	maxnoofCellIDforMDT                         = 32
	maxnoofCellIDforQMC                         = 32
	maxnoofCellsforMBS                          = 8192
	maxnoofESNPNs                               = 15
	maxnoofFreqforMDT                           = 8
	maxnoofIntendedAreasforMBS                  = 65536
	maxnoofMBSQoSFlows                          = 64
	maxnoofMBSServiceAreaInformation            = 256
	maxnoofMBSSessions                          = 32
	maxnoofMBSSessionsofUE                      = 256
	maxnoofMDTPLMNs                             = 16
	maxnoofMDTSNPNs                             = 16
	maxnoofMRBs                                 = 32
	maxnoofNGAPIESupportInfo                    = 32
	maxnoofNRCellBands                          = 32
	maxnoofNeighPCIforMDT                       = 32
	maxnoofPLMNforQMC                           = 16
	maxnoofPSCellsPerPrimaryCellinUEHistoryInfo = 8
	maxnoofPeriodicities                        = 8
	maxnoofQosParaSets                          = 8
	maxnoofSNSSAIforQMC                         = 16
	maxnoofSensorName                           = 3
	maxnoofSliceItemsforMDT                     = 1024
	maxnoofTAIforMBS                            = 1024
	maxnoofTAforMDT                             = 8
	maxnoofTAforQMC                             = 8
	maxnoofThresholds                           = 8
	maxnoofThresholdsForExcessPacketDelay       = 255
	maxnoofTimePeriods                          = 2
	maxnoofUEAppLayerMeas                       = 16
	maxnoofWLANName                             = 4
)

type RedundantPDUSessionInformation struct {
	RSN              RSN
	PDUSessionPairID *PDUSessionPairID      `per:",extie:IDPDUSessionPairID,crit:ignore"`
	iEExtensions     extensionMeta          `per:",exttable"`
	extensions       per.ExtensionAdditions `per:",extraw"`
}

type MBSSupportIndicator uint8

const (
	MBSSupportIndicatorMulticastSupported MBSSupportIndicator = iota
	MBSSupportIndicatorMulticastSupportedWithReceptionInRRCInactive
)

type PDUSetbasedHandlingIndicator uint8

const (
	PDUSetbasedHandlingIndicatorSupported PDUSetbasedHandlingIndicator = iota
)

type ECNMarkingorCongestionInformationReportingStatus []ECNMarkingorCongestionInformationReportingStatusItem

func (l ECNMarkingorCongestionInformationReportingStatus) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return marshalSeqOf(w, enc, 1, maxnoofQosFlows, []ECNMarkingorCongestionInformationReportingStatusItem(l))
}

func (l *ECNMarkingorCongestionInformationReportingStatus) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	items, err := unmarshalSeqOf[ECNMarkingorCongestionInformationReportingStatusItem](r, enc, 1, maxnoofQosFlows)
	if err != nil {
		return err
	}

	*l = items

	return nil
}

type QosFlowParametersList []QosFlowParametersItem

func (l QosFlowParametersList) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return marshalSeqOf(w, enc, 1, maxnoofQosFlows, []QosFlowParametersItem(l))
}

func (l *QosFlowParametersList) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	items, err := unmarshalSeqOf[QosFlowParametersItem](r, enc, 1, maxnoofQosFlows)
	if err != nil {
		return err
	}

	*l = items

	return nil
}

type CommonNetworkInstance []byte

func (v CommonNetworkInstance) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return per.EncodeOctetString(w, enc, 0, 0, true, false, false, v)
}

func (v *CommonNetworkInstance) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	b, err := per.DecodeOctetString(r, enc, 0, 0, true, false, false)
	if err != nil {
		return err
	}

	*v = b

	return nil
}

type ExtendedRATRestrictionInformationPrimaryRATRestriction struct{ BitString }

func (v ExtendedRATRestrictionInformationPrimaryRATRestriction) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return v.encode(w, enc, 8, 8)
}

func (v *ExtendedRATRestrictionInformationPrimaryRATRestriction) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	b, err := decodeBitString(r, enc, 8, 8)
	if err != nil {
		return err
	}

	*v = ExtendedRATRestrictionInformationPrimaryRATRestriction{b}

	return nil
}

type ExtendedRATRestrictionInformationSecondaryRATRestriction struct{ BitString }

func (v ExtendedRATRestrictionInformationSecondaryRATRestriction) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return v.encode(w, enc, 8, 8)
}

func (v *ExtendedRATRestrictionInformationSecondaryRATRestriction) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	b, err := decodeBitString(r, enc, 8, 8)
	if err != nil {
		return err
	}

	*v = ExtendedRATRestrictionInformationSecondaryRATRestriction{b}

	return nil
}

type ExtendedRATRestrictionInformation struct {
	PrimaryRATRestriction   ExtendedRATRestrictionInformationPrimaryRATRestriction
	SecondaryRATRestriction ExtendedRATRestrictionInformationSecondaryRATRestriction
	iEExtensions            extensionMeta          `per:",exttable"`
	extensions              per.ExtensionAdditions `per:",extraw"`
}

type CNTypeRestrictionsForServing uint8

const (
	CNTypeRestrictionsForServingEPCForbidden CNTypeRestrictionsForServing = iota
)

type CNTypeRestrictionsForEquivalent []CNTypeRestrictionsForEquivalentItem

func (l CNTypeRestrictionsForEquivalent) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return marshalSeqOf(w, enc, 1, maxnoofEPLMNs, []CNTypeRestrictionsForEquivalentItem(l))
}

func (l *CNTypeRestrictionsForEquivalent) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	items, err := unmarshalSeqOf[CNTypeRestrictionsForEquivalentItem](r, enc, 1, maxnoofEPLMNs)
	if err != nil {
		return err
	}

	*l = items

	return nil
}

type NPNMobilityInformation struct {
	SNPNMobilityInformation   *SNPNMobilityInformation   `per:",choice:0"`
	PNINPNMobilityInformation *PNINPNMobilityInformation `per:",choice:1"`
	ChoiceExtension           *SingleContainer           `per:",choice:2,extie"`
}

type ExpectedUEActivityBehaviour struct {
	ExpectedActivityPeriod                 *ExpectedActivityPeriod                 `per:",optional"`
	ExpectedIdlePeriod                     *ExpectedIdlePeriod                     `per:",optional"`
	SourceOfUEActivityBehaviourInformation *SourceOfUEActivityBehaviourInformation `per:",optional"`
	iEExtensions                           extensionMeta                           `per:",exttable"`
	extensions                             per.ExtensionAdditions                  `per:",extraw"`
}

type ExtendedPacketDelayBudget int64

func (v ExtendedPacketDelayBudget) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return per.EncodeInteger(w, enc, per.Bounds{LB: 1, HasLB: true, UB: 65535, HasUB: true, Extensible: true}, int64(v))
}

func (v *ExtendedPacketDelayBudget) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	n, err := per.DecodeInteger(r, enc, per.Bounds{LB: 1, HasLB: true, UB: 65535, HasUB: true, Extensible: true})
	if err != nil {
		return err
	}

	*v = ExtendedPacketDelayBudget(n)

	return nil
}

func (v ExtendedPacketDelayBudget) Diagnose(*ap.Diagnosis[ProtocolIEID]) bool {
	return outsideRoot(int64(v), 1, 109999)
}

type QosMonitoringRequest uint8

const (
	QosMonitoringRequestUL QosMonitoringRequest = iota
	QosMonitoringRequestDL
	QosMonitoringRequestBoth
	QosMonitoringRequestStop
)

type QosMonitoringReportingFrequency int64

func (v QosMonitoringReportingFrequency) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return per.EncodeInteger(w, enc, per.Bounds{LB: 1, HasLB: true, UB: 1800, HasUB: true, Extensible: true}, int64(v))
}

func (v *QosMonitoringReportingFrequency) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	n, err := per.DecodeInteger(r, enc, per.Bounds{LB: 1, HasLB: true, UB: 1800, HasUB: true, Extensible: true})
	if err != nil {
		return err
	}

	*v = QosMonitoringReportingFrequency(n)

	return nil
}

func (v QosMonitoringReportingFrequency) Diagnose(*ap.Diagnosis[ProtocolIEID]) bool {
	return outsideRoot(int64(v), 1, 1800)
}

type PDUsetQoSParameters struct {
	UlPDUSetQoSInformation *PDUsetQoSInformation  `per:",optional"`
	DlPDUSetQoSInformation *PDUsetQoSInformation  `per:",optional"`
	iEExtensions           extensionMeta          `per:",exttable"`
	extensions             per.ExtensionAdditions `per:",extraw"`
}

type DLPDUSetInformationMarkingSupportIndication uint8

const (
	DLPDUSetInformationMarkingSupportIndicationTrue DLPDUSetInformationMarkingSupportIndication = iota
)

type MMSID []byte

func (v MMSID) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return per.EncodeOctetString(w, enc, 1, 1, true, true, false, v)
}

func (v *MMSID) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	b, err := per.DecodeOctetString(r, enc, 1, 1, true, true, false)
	if err != nil {
		return err
	}

	*v = b

	return nil
}

type IndicationofBitrateAdaptation uint8

const (
	IndicationofBitrateAdaptationUplink IndicationofBitrateAdaptation = iota
)

type AlternativeQoSParaSetList []AlternativeQoSParaSetItem

func (l AlternativeQoSParaSetList) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return marshalSeqOf(w, enc, 1, maxnoofQosParaSets, []AlternativeQoSParaSetItem(l))
}

func (l *AlternativeQoSParaSetList) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	items, err := unmarshalSeqOf[AlternativeQoSParaSetItem](r, enc, 1, maxnoofQosParaSets)
	if err != nil {
		return err
	}

	*l = items

	return nil
}

type MonitoringRequestonAvailableBitrate struct {
	MonitoringRequest                  MonitoringRequest
	DlAvailableBitrateReportThresholds AvailableBitrateReportThresholdList `per:",optional"`
	UlAvailableBitrateReportThresholds AvailableBitrateReportThresholdList `per:",optional"`
	iEExtensions                       extensionMeta                       `per:",exttable"`
	extensions                         per.ExtensionAdditions              `per:",extraw"`
}

type TSCTrafficCharacteristics struct {
	TSCAssistanceInformationDL *TSCAssistanceInformation `per:",optional"`
	TSCAssistanceInformationUL *TSCAssistanceInformation `per:",optional"`
	iEExtensions               extensionMeta             `per:",exttable"`
	extensions                 per.ExtensionAdditions    `per:",extraw"`
}

type RedundantQosFlowIndicator uint8

const (
	RedundantQosFlowIndicatorTrue RedundantQosFlowIndicator = iota
	RedundantQosFlowIndicatorFalse
)

type ECNMarkingorCongestionInformationReportingRequest struct {
	ECNMarkingAtRANRequest       *ECNMarkingAtRANRequest       `per:",choice:0"`
	ECNMarkingAtUPFRequest       *ECNMarkingAtUPFRequest       `per:",choice:1"`
	CongestionInformationRequest *CongestionInformationRequest `per:",choice:2"`
	ChoiceExtension              *SingleContainer              `per:",choice:3,extie"`
}

type AlternativeQoSParaSetIndex int64

func (v AlternativeQoSParaSetIndex) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return per.EncodeInteger(w, enc, per.Bounds{LB: 1, HasLB: true, UB: 8, HasUB: true, Extensible: true}, int64(v))
}

func (v *AlternativeQoSParaSetIndex) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	n, err := per.DecodeInteger(r, enc, per.Bounds{LB: 1, HasLB: true, UB: 8, HasUB: true, Extensible: true})
	if err != nil {
		return err
	}

	*v = AlternativeQoSParaSetIndex(n)

	return nil
}

func (v AlternativeQoSParaSetIndex) Diagnose(*ap.Diagnosis[ProtocolIEID]) bool {
	return outsideRoot(int64(v), 1, 8)
}

type TLContainer []byte

func (v TLContainer) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return per.EncodeOctetString(w, enc, 0, 0, true, false, false, v)
}

func (v *TLContainer) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	b, err := per.DecodeOctetString(r, enc, 0, 0, true, false, false)
	if err != nil {
		return err
	}

	*v = b

	return nil
}

type TSCTrafficCharacteristicsFeedback struct {
	TSCFeedbackInformationDL *TSCFeedbackInformation `per:",optional"`
	TSCFeedbackInformationUL *TSCFeedbackInformation `per:",optional"`
	iEExtensions             extensionMeta           `per:",exttable"`
	extensions               per.ExtensionAdditions  `per:",extraw"`
}

type SecondaryRATUsageInformation struct {
	PDUSessionUsageReport   *PDUSessionUsageReport  `per:",optional"`
	QosFlowsUsageReportList QoSFlowsUsageReportList `per:",optional"`
	iEExtensions            extensionMeta           `per:",exttable"`
	extensions              per.ExtensionAdditions  `per:",extraw"`
}

type MBSSessionSetupResponseList []MBSSessionSetupResponseItem

func (l MBSSessionSetupResponseList) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return marshalSeqOf(w, enc, 1, maxnoofMBSSessions, []MBSSessionSetupResponseItem(l))
}

func (l *MBSSessionSetupResponseList) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	items, err := unmarshalSeqOf[MBSSessionSetupResponseItem](r, enc, 1, maxnoofMBSSessions)
	if err != nil {
		return err
	}

	*l = items

	return nil
}

type MBSSessionFailedtoSetupList []MBSSessionFailedtoSetupItem

func (l MBSSessionFailedtoSetupList) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return marshalSeqOf(w, enc, 1, maxnoofMBSSessions, []MBSSessionFailedtoSetupItem(l))
}

func (l *MBSSessionFailedtoSetupList) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	items, err := unmarshalSeqOf[MBSSessionFailedtoSetupItem](r, enc, 1, maxnoofMBSSessions)
	if err != nil {
		return err
	}

	*l = items

	return nil
}

type UserPlaneFailureIndicationReport uint8

const (
	UserPlaneFailureIndicationReportQoSFlowsAndTunnelToBeReleased UserPlaneFailureIndicationReport = iota
	UserPlaneFailureIndicationReportQoSFlowsToBeMoved
	UserPlaneFailureIndicationReportNewTransportAddressAllocated
)

type AlternativeQoSParaSetNotifyIndex int64

func (v AlternativeQoSParaSetNotifyIndex) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return per.EncodeInteger(w, enc, per.Bounds{LB: 0, HasLB: true, UB: 8, HasUB: true, Extensible: true}, int64(v))
}

func (v *AlternativeQoSParaSetNotifyIndex) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	n, err := per.DecodeInteger(r, enc, per.Bounds{LB: 0, HasLB: true, UB: 8, HasUB: true, Extensible: true})
	if err != nil {
		return err
	}

	*v = AlternativeQoSParaSetNotifyIndex(n)

	return nil
}

func (v AlternativeQoSParaSetNotifyIndex) Diagnose(*ap.Diagnosis[ProtocolIEID]) bool {
	return outsideRoot(int64(v), 0, 8)
}

type QosFlowFeedbackList []QosFlowFeedbackItem

func (l QosFlowFeedbackList) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return marshalSeqOf(w, enc, 1, maxnoofQosFlows, []QosFlowFeedbackItem(l))
}

func (l *QosFlowFeedbackList) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	items, err := unmarshalSeqOf[QosFlowFeedbackItem](r, enc, 1, maxnoofQosFlows)
	if err != nil {
		return err
	}

	*l = items

	return nil
}

type QosFlowAdditionalInfoListRelCom []QosFlowAdditionalInfoItemRelCom

func (l QosFlowAdditionalInfoListRelCom) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return marshalSeqOf(w, enc, 1, maxnoofQosFlows, []QosFlowAdditionalInfoItemRelCom(l))
}

func (l *QosFlowAdditionalInfoListRelCom) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	items, err := unmarshalSeqOf[QosFlowAdditionalInfoItemRelCom](r, enc, 1, maxnoofQosFlows)
	if err != nil {
		return err
	}

	*l = items

	return nil
}

type QosFlowAdditionalInfoListRelRes []QosFlowAdditionalInfoItemRelRes

func (l QosFlowAdditionalInfoListRelRes) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return marshalSeqOf(w, enc, 1, maxnoofQosFlows, []QosFlowAdditionalInfoItemRelRes(l))
}

func (l *QosFlowAdditionalInfoListRelRes) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	items, err := unmarshalSeqOf[QosFlowAdditionalInfoItemRelRes](r, enc, 1, maxnoofQosFlows)
	if err != nil {
		return err
	}

	*l = items

	return nil
}

type QoSFlowTSCList []QoSFlowTSCItem

func (l QoSFlowTSCList) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return marshalSeqOf(w, enc, 1, maxnoofQosFlows, []QoSFlowTSCItem(l))
}

func (l *QoSFlowTSCList) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	items, err := unmarshalSeqOf[QoSFlowTSCItem](r, enc, 1, maxnoofQosFlows)
	if err != nil {
		return err
	}

	*l = items

	return nil
}

type MDTConfiguration struct {
	MdtConfigNR         *MDTConfigurationNR    `per:",optional"`
	MdtConfigEUTRA      *MDTConfigurationEUTRA `per:",optional"`
	MNonlyMDTcollection *MNonlyMDTcollection   `per:",extie:IDMNonlyMDTcollection,crit:ignore"`
	iEExtensions        extensionMeta          `per:",exttable"`
	extensions          per.ExtensionAdditions `per:",extraw"`
}

type SgNBUEX2APID uint32

func (v SgNBUEX2APID) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return per.EncodeInteger(w, enc, per.Bounds{LB: 0, HasLB: true, UB: 4294967295, HasUB: true}, int64(v))
}

func (v *SgNBUEX2APID) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	n, err := per.DecodeInteger(r, enc, per.Bounds{LB: 0, HasLB: true, UB: 4294967295, HasUB: true})
	if err != nil {
		return err
	}

	*v = SgNBUEX2APID(n)

	return nil
}

type UEHistoryInformationFromTheUE struct {
	NR              *NRMobilityHistoryReport `per:",choice:0"`
	ChoiceExtension *SingleContainer         `per:",choice:1,extie"`
}

type SourceNodeID struct {
	SourceengNBID   *GlobalGNBID     `per:",choice:0"`
	ChoiceExtension *SingleContainer `per:",choice:1,extie"`
}

type MBSActiveSessionInformationSourcetoTargetList []MBSActiveSessionInformationSourcetoTargetItem

func (l MBSActiveSessionInformationSourcetoTargetList) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return marshalSeqOf(w, enc, 1, maxnoofMBSSessionsofUE, []MBSActiveSessionInformationSourcetoTargetItem(l))
}

func (l *MBSActiveSessionInformationSourcetoTargetList) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	items, err := unmarshalSeqOf[MBSActiveSessionInformationSourcetoTargetItem](r, enc, 1, maxnoofMBSSessionsofUE)
	if err != nil {
		return err
	}

	*l = items

	return nil
}

type QMCConfigInfo struct {
	UEAppLayerMeasInfoList UEAppLayerMeasInfoList
	iEExtensions           extensionMeta          `per:",exttable"`
	extensions             per.ExtensionAdditions `per:",extraw"`
}

type NGAPIESupportInformationRequestList []NGAPIESupportInformationRequestItem

func (l NGAPIESupportInformationRequestList) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return marshalSeqOf(w, enc, 1, maxnoofNGAPIESupportInfo, []NGAPIESupportInformationRequestItem(l))
}

func (l *NGAPIESupportInformationRequestList) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	items, err := unmarshalSeqOf[NGAPIESupportInformationRequestItem](r, enc, 1, maxnoofNGAPIESupportInfo)
	if err != nil {
		return err
	}

	*l = items

	return nil
}

type CandidateRelayUEInformationList []CandidateRelayUEInformationItem

func (l CandidateRelayUEInformationList) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return marshalSeqOf(w, enc, 1, maxnoofCandidateRelayUEs, []CandidateRelayUEInformationItem(l))
}

func (l *CandidateRelayUEInformationList) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	items, err := unmarshalSeqOf[CandidateRelayUEInformationItem](r, enc, 1, maxnoofCandidateRelayUEs)
	if err != nil {
		return err
	}

	*l = items

	return nil
}

type TimeBasedHandoverInformation struct {
	HOWindowStart    HandoverWindowStart
	HOWindowDuration HandoverWindowDuration
	iEExtensions     extensionMeta          `per:",exttable"`
	extensions       per.ExtensionAdditions `per:",extraw"`
}

type ULForwarding uint8

const (
	ULForwardingULForwardingProposed ULForwarding = iota
)

type DAPSRequestInfoDAPSIndicator uint8

const (
	DAPSRequestInfoDAPSIndicatorDapsHoRequired DAPSRequestInfoDAPSIndicator = iota
)

type DAPSRequestInfo struct {
	DAPSIndicator DAPSRequestInfoDAPSIndicator
	iEExtensions  extensionMeta          `per:",exttable"`
	extensions    per.ExtensionAdditions `per:",extraw"`
}

type LastVisitedPSCellList []LastVisitedPSCellInformation

func (l LastVisitedPSCellList) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return marshalSeqOf(w, enc, 1, maxnoofPSCellsPerPrimaryCellinUEHistoryInfo, []LastVisitedPSCellInformation(l))
}

func (l *LastVisitedPSCellList) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	items, err := unmarshalSeqOf[LastVisitedPSCellInformation](r, enc, 1, maxnoofPSCellsPerPrimaryCellinUEHistoryInfo)
	if err != nil {
		return err
	}

	*l = items

	return nil
}

type RSN uint8

const (
	RSNV1 RSN = iota
	RSNV2
)

type PDUSessionPairID int64

func (v PDUSessionPairID) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return per.EncodeInteger(w, enc, per.Bounds{LB: 0, HasLB: true, UB: 255, HasUB: true, Extensible: true}, int64(v))
}

func (v *PDUSessionPairID) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	n, err := per.DecodeInteger(r, enc, per.Bounds{LB: 0, HasLB: true, UB: 255, HasUB: true, Extensible: true})
	if err != nil {
		return err
	}

	*v = PDUSessionPairID(n)

	return nil
}

func (v PDUSessionPairID) Diagnose(*ap.Diagnosis[ProtocolIEID]) bool {
	return outsideRoot(int64(v), 0, 255)
}

type ECNMarkingorCongestionInformationReportingStatusItem struct {
	QosFlowIdentifier QosFlowIdentifier
	ActivationStatus  ActivationStatus
	iEExtensions      extensionMeta          `per:",exttable"`
	extensions        per.ExtensionAdditions `per:",extraw"`
}

type QosFlowParametersItem struct {
	QosFlowIdentifier         QosFlowIdentifier
	AlternativeQoSParaSetList AlternativeQoSParaSetList  `per:",optional"`
	CNPacketDelayBudgetDL     *ExtendedPacketDelayBudget `per:",extie:IDCNPacketDelayBudgetDL,crit:ignore"`
	CNPacketDelayBudgetUL     *ExtendedPacketDelayBudget `per:",extie:IDCNPacketDelayBudgetUL,crit:ignore"`
	BurstArrivalTimeDownlink  *BurstArrivalTime          `per:",extie:IDBurstArrivalTimeDownlink,crit:ignore"`
	iEExtensions              extensionMeta              `per:",exttable"`
	extensions                per.ExtensionAdditions     `per:",extraw"`
}

type CNTypeRestrictionsForEquivalentItemCnType uint8

const (
	CNTypeRestrictionsForEquivalentItemCnTypeEPCForbidden CNTypeRestrictionsForEquivalentItemCnType = iota
	CNTypeRestrictionsForEquivalentItemCnTypeFiveGCForbidden
)

type CNTypeRestrictionsForEquivalentItem struct {
	PlmnIdentity PLMNIdentity
	CnType       CNTypeRestrictionsForEquivalentItemCnType
	iEExtensions extensionMeta          `per:",exttable"`
	extensions   per.ExtensionAdditions `per:",extraw"`
}

type SNPNMobilityInformation struct {
	ServingNID          NID
	EquivalentSNPNsList EquivalentSNPNsList    `per:",extie:IDEquivalentSNPNsList,crit:reject"`
	iEExtensions        extensionMeta          `per:",exttable"`
	extensions          per.ExtensionAdditions `per:",extraw"`
}

type PNINPNMobilityInformation struct {
	AllowedPNINPIList AllowedPNINPNList
	iEExtensions      extensionMeta          `per:",exttable"`
	extensions        per.ExtensionAdditions `per:",extraw"`
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

func (v ExpectedActivityPeriod) Diagnose(*ap.Diagnosis[ProtocolIEID]) bool {
	return outsideRoot(int64(v), 1, 181)
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

func (v ExpectedIdlePeriod) Diagnose(*ap.Diagnosis[ProtocolIEID]) bool {
	return outsideRoot(int64(v), 1, 181)
}

type SourceOfUEActivityBehaviourInformation uint8

const (
	SourceOfUEActivityBehaviourInformationSubscriptionInformation SourceOfUEActivityBehaviourInformation = iota
	SourceOfUEActivityBehaviourInformationStatistics
)

type PDUsetQoSInformationPduSetIntegratedHandlingInformation uint8

const (
	PDUsetQoSInformationPduSetIntegratedHandlingInformationTrue PDUsetQoSInformationPduSetIntegratedHandlingInformation = iota
	PDUsetQoSInformationPduSetIntegratedHandlingInformationFalse
)

type PDUsetQoSInformation struct {
	PduSetDelayBudget                   *ExtendedPacketDelayBudget                               `per:",optional"`
	PduSetErrorRate                     *PacketErrorRate                                         `per:",optional"`
	PduSetIntegratedHandlingInformation *PDUsetQoSInformationPduSetIntegratedHandlingInformation `per:",optional"`
	iEExtensions                        extensionMeta                                            `per:",exttable"`
	extensions                          per.ExtensionAdditions                                   `per:",extraw"`
}

type AlternativeQoSParaSetItem struct {
	AlternativeQoSParaSetIndex AlternativeQoSParaSetIndex
	GuaranteedFlowBitRateDL    *BitRate                   `per:",optional"`
	GuaranteedFlowBitRateUL    *BitRate                   `per:",optional"`
	PacketDelayBudget          *PacketDelayBudget         `per:",optional"`
	PacketErrorRate            *PacketErrorRate           `per:",optional"`
	MaximumDataBurstVolume     *MaximumDataBurstVolume    `per:",extie:IDMaximumDataBurstVolume,crit:ignore"`
	PduSetDelayBudgetDownlink  *ExtendedPacketDelayBudget `per:",extie:IDPduSetDelayBudgetDownlink,crit:ignore"`
	PduSetDelayBudgetUplink    *ExtendedPacketDelayBudget `per:",extie:IDPduSetDelayBudgetUplink,crit:ignore"`
	PduSetErrorRateDownlink    *PacketErrorRate           `per:",extie:IDPduSetErrorRateDownlink,crit:ignore"`
	PduSetErrorRateUplink      *PacketErrorRate           `per:",extie:IDPduSetErrorRateUplink,crit:ignore"`
	iEExtensions               extensionMeta              `per:",exttable"`
	extensions                 per.ExtensionAdditions     `per:",extraw"`
}

type MonitoringRequest uint8

const (
	MonitoringRequestUL MonitoringRequest = iota
	MonitoringRequestDL
	MonitoringRequestBoth
	MonitoringRequestStop
)

type AvailableBitrateReportThresholdList []AvailableBitrateReportThresholdItem

func (l AvailableBitrateReportThresholdList) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return marshalSeqOf(w, enc, 1, maxnoofThresholds, []AvailableBitrateReportThresholdItem(l))
}

func (l *AvailableBitrateReportThresholdList) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	items, err := unmarshalSeqOf[AvailableBitrateReportThresholdItem](r, enc, 1, maxnoofThresholds)
	if err != nil {
		return err
	}

	*l = items

	return nil
}

type TSCAssistanceInformation struct {
	Periodicity         Periodicity
	BurstArrivalTime    *BurstArrivalTime      `per:",optional"`
	SurvivalTime        *SurvivalTime          `per:",extie:IDSurvivalTime,crit:ignore"`
	RANfeedbacktype     *RANfeedbacktype       `per:",extie:IDRANfeedbacktype,crit:ignore"`
	N6JitterInformation *N6JitterInformation   `per:",extie:IDN6JitterInformation,crit:ignore"`
	iEExtensions        extensionMeta          `per:",exttable"`
	extensions          per.ExtensionAdditions `per:",extraw"`
}

type ECNMarkingAtRANRequest uint8

const (
	ECNMarkingAtRANRequestUL ECNMarkingAtRANRequest = iota
	ECNMarkingAtRANRequestDL
	ECNMarkingAtRANRequestBoth
	ECNMarkingAtRANRequestStop
)

type ECNMarkingAtUPFRequest uint8

const (
	ECNMarkingAtUPFRequestUL ECNMarkingAtUPFRequest = iota
	ECNMarkingAtUPFRequestDL
	ECNMarkingAtUPFRequestBoth
	ECNMarkingAtUPFRequestStop
)

type CongestionInformationRequest uint8

const (
	CongestionInformationRequestUL CongestionInformationRequest = iota
	CongestionInformationRequestDL
	CongestionInformationRequestBoth
	CongestionInformationRequestStop
)

type TSCFeedbackInformationBurstArrivalTimeOffset int64

func (v TSCFeedbackInformationBurstArrivalTimeOffset) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return per.EncodeInteger(w, enc, per.Bounds{LB: -640000, HasLB: true, UB: 640000, HasUB: true, Extensible: true}, int64(v))
}

func (v *TSCFeedbackInformationBurstArrivalTimeOffset) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	n, err := per.DecodeInteger(r, enc, per.Bounds{LB: -640000, HasLB: true, UB: 640000, HasUB: true, Extensible: true})
	if err != nil {
		return err
	}

	*v = TSCFeedbackInformationBurstArrivalTimeOffset(n)

	return nil
}

func (v TSCFeedbackInformationBurstArrivalTimeOffset) Diagnose(*ap.Diagnosis[ProtocolIEID]) bool {
	return outsideRoot(int64(v), -640000, 640000)
}

type TSCFeedbackInformation struct {
	BurstArrivalTimeOffset TSCFeedbackInformationBurstArrivalTimeOffset
	AdjustedPeriodicity    *Periodicity           `per:",optional"`
	iEExtensions           extensionMeta          `per:",exttable"`
	extensions             per.ExtensionAdditions `per:",extraw"`
}

type PDUSessionUsageReportRATType uint8

const (
	PDUSessionUsageReportRATTypeNR PDUSessionUsageReportRATType = iota
	PDUSessionUsageReportRATTypeEUTRA
	PDUSessionUsageReportRATTypeNRUnlicensed
	PDUSessionUsageReportRATTypeEUtraUnlicensed
)

type PDUSessionUsageReport struct {
	RATType                   PDUSessionUsageReportRATType
	PDUSessionTimedReportList VolumeTimedReportList
	iEExtensions              extensionMeta          `per:",exttable"`
	extensions                per.ExtensionAdditions `per:",extraw"`
}

type QoSFlowsUsageReportList []QoSFlowsUsageReportItem

func (l QoSFlowsUsageReportList) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return marshalSeqOf(w, enc, 1, maxnoofQosFlows, []QoSFlowsUsageReportItem(l))
}

func (l *QoSFlowsUsageReportList) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	items, err := unmarshalSeqOf[QoSFlowsUsageReportItem](r, enc, 1, maxnoofQosFlows)
	if err != nil {
		return err
	}

	*l = items

	return nil
}

type MBSSessionSetupResponseItem struct {
	MBSSessionID     MBSSessionID
	MBSAreaSessionID *MBSAreaSessionID      `per:",optional"`
	iEExtensions     extensionMeta          `per:",exttable"`
	extensions       per.ExtensionAdditions `per:",extraw"`
}

type MBSSessionFailedtoSetupItem struct {
	MBSSessionID     MBSSessionID
	MBSAreaSessionID *MBSAreaSessionID `per:",optional"`
	Cause            Cause
	iEExtensions     extensionMeta          `per:",exttable"`
	extensions       per.ExtensionAdditions `per:",extraw"`
}

type QosFlowFeedbackItem struct {
	QosFlowIdentifier     QosFlowIdentifier
	UpdateFeedback        *UpdateFeedback            `per:",optional"`
	CNpacketDelayBudgetDL *ExtendedPacketDelayBudget `per:",optional"`
	CNpacketDelayBudgetUL *ExtendedPacketDelayBudget `per:",optional"`
	iEExtensions          extensionMeta              `per:",exttable"`
	extensions            per.ExtensionAdditions     `per:",extraw"`
}

type QosFlowAdditionalInfoItemRelCom struct {
	QosFlowIdentifier   QosFlowIdentifier
	DownlinkTLContainer TLContainer            `per:",optional"`
	iEExtensions        extensionMeta          `per:",exttable"`
	extensions          per.ExtensionAdditions `per:",extraw"`
}

type QosFlowAdditionalInfoItemRelRes struct {
	QosFlowIdentifier QosFlowIdentifier
	UplinkTLContainer TLContainer            `per:",optional"`
	iEExtensions      extensionMeta          `per:",exttable"`
	extensions        per.ExtensionAdditions `per:",extraw"`
}

type QoSFlowTSCItem struct {
	QosFlowIdentifier                 QosFlowIdentifier
	TSCTrafficCharacteristicsFeedback *TSCTrafficCharacteristicsFeedback `per:",optional"`
	ANPacketDelayBudgetUL             *ExtendedPacketDelayBudget         `per:",optional"`
	iEExtensions                      extensionMeta                      `per:",exttable"`
	extensions                        per.ExtensionAdditions             `per:",extraw"`
}

type MDTConfigurationNR struct {
	MdtActivation              MDTActivation
	AreaScopeOfMDT             AreaScopeOfMDTNR
	MDTModeNr                  MDTModeNr
	SignallingBasedMDTPLMNList MDTPLMNList                 `per:",optional"`
	PNINPNAreaScopeofMDT       *PNINPNAreaScopeofMDT       `per:",extie:IDPNINPNAreaScopeofMDT,crit:ignore"`
	NetworkSliceAreaScopeofMDT *NetworkSliceAreaScopeofMDT `per:",extie:IDNetworkSliceAreaScopeofMDT,crit:ignore"`
	iEExtensions               extensionMeta               `per:",exttable"`
	extensions                 per.ExtensionAdditions      `per:",extraw"`
}

type MDTConfigurationEUTRA struct {
	MdtActivation              MDTActivation
	AreaScopeOfMDT             AreaScopeOfMDTEUTRA
	MDTMode                    MDTModeEutra
	SignallingBasedMDTPLMNList MDTPLMNList            `per:",optional"`
	iEExtensions               extensionMeta          `per:",exttable"`
	extensions                 per.ExtensionAdditions `per:",extraw"`
}

type MNonlyMDTcollection uint8

const (
	MNonlyMDTcollectionMnOnly MNonlyMDTcollection = iota
)

type NRMobilityHistoryReport []byte

func (v NRMobilityHistoryReport) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return per.EncodeOctetString(w, enc, 0, 0, true, false, false, v)
}

func (v *NRMobilityHistoryReport) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	b, err := per.DecodeOctetString(r, enc, 0, 0, true, false, false)
	if err != nil {
		return err
	}

	*v = b

	return nil
}

type MBSActiveSessionInformationSourcetoTargetItem struct {
	MBSSessionID                           MBSSessionID
	MBSAreaSessionID                       *MBSAreaSessionID `per:",optional"`
	MBSServiceArea                         *MBSServiceArea   `per:",optional"`
	MBSQoSFlowsToBeSetupList               MBSQoSFlowsToBeSetupList
	MBSMappingandDataForwardingRequestList MBSMappingandDataForwardingRequestList `per:",optional"`
	iEExtensions                           extensionMeta                          `per:",exttable"`
	extensions                             per.ExtensionAdditions                 `per:",extraw"`
}

type UEAppLayerMeasInfoList []UEAppLayerMeasInfoItem

func (l UEAppLayerMeasInfoList) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return marshalSeqOf(w, enc, 1, maxnoofUEAppLayerMeas, []UEAppLayerMeasInfoItem(l))
}

func (l *UEAppLayerMeasInfoList) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	items, err := unmarshalSeqOf[UEAppLayerMeasInfoItem](r, enc, 1, maxnoofUEAppLayerMeas)
	if err != nil {
		return err
	}

	*l = items

	return nil
}

type NGAPIESupportInformationRequestItem struct {
	NgapProtocolIEId ProtocolIEID
	iEExtensions     extensionMeta          `per:",exttable"`
	extensions       per.ExtensionAdditions `per:",extraw"`
}

type CandidateRelayUEInformationItem struct {
	CandidateRelayUEId CandidateRelayUEID
	iEExtensions       extensionMeta          `per:",exttable"`
	extensions         per.ExtensionAdditions `per:",extraw"`
}

type HandoverWindowStart uint64

func (v HandoverWindowStart) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return per.EncodeInteger(w, enc, per.Bounds{LB: 0, HasLB: true, UB: 549755813887, HasUB: true}, int64(v))
}

func (v *HandoverWindowStart) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	n, err := per.DecodeInteger(r, enc, per.Bounds{LB: 0, HasLB: true, UB: 549755813887, HasUB: true})
	if err != nil {
		return err
	}

	*v = HandoverWindowStart(n)

	return nil
}

type HandoverWindowDuration uint16

func (v HandoverWindowDuration) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return per.EncodeInteger(w, enc, per.Bounds{LB: 1, HasLB: true, UB: 6000, HasUB: true}, int64(v))
}

func (v *HandoverWindowDuration) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	n, err := per.DecodeInteger(r, enc, per.Bounds{LB: 1, HasLB: true, UB: 6000, HasUB: true})
	if err != nil {
		return err
	}

	*v = HandoverWindowDuration(n)

	return nil
}

type LastVisitedPSCellInformationTimeStay uint16

func (v LastVisitedPSCellInformationTimeStay) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return per.EncodeInteger(w, enc, per.Bounds{LB: 0, HasLB: true, UB: 40950, HasUB: true}, int64(v))
}

func (v *LastVisitedPSCellInformationTimeStay) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	n, err := per.DecodeInteger(r, enc, per.Bounds{LB: 0, HasLB: true, UB: 40950, HasUB: true})
	if err != nil {
		return err
	}

	*v = LastVisitedPSCellInformationTimeStay(n)

	return nil
}

type LastVisitedPSCellInformation struct {
	PSCellID          *NGRANCGI `per:",optional"`
	TimeStay          LastVisitedPSCellInformationTimeStay
	SCGActivationTime *SCGActivationTime     `per:",extie:IDSCGActivationTime,crit:ignore"`
	iEExtensions      extensionMeta          `per:",exttable"`
	extensions        per.ExtensionAdditions `per:",extraw"`
}

type ActivationStatus uint8

const (
	ActivationStatusActive ActivationStatus = iota
	ActivationStatusNotActive
)

type BurstArrivalTime []byte

func (v BurstArrivalTime) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return per.EncodeOctetString(w, enc, 0, 0, true, false, false, v)
}

func (v *BurstArrivalTime) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	b, err := per.DecodeOctetString(r, enc, 0, 0, true, false, false)
	if err != nil {
		return err
	}

	*v = b

	return nil
}

type EquivalentSNPNsList []EquivalentSNPNsItem

func (l EquivalentSNPNsList) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return marshalSeqOf(w, enc, 1, maxnoofESNPNs, []EquivalentSNPNsItem(l))
}

func (l *EquivalentSNPNsList) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	items, err := unmarshalSeqOf[EquivalentSNPNsItem](r, enc, 1, maxnoofESNPNs)
	if err != nil {
		return err
	}

	*l = items

	return nil
}

type AllowedPNINPNList []AllowedPNINPNItem

func (l AllowedPNINPNList) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return marshalSeqOf(w, enc, 1, maxnoofEPLMNsPlusOne, []AllowedPNINPNItem(l))
}

func (l *AllowedPNINPNList) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	items, err := unmarshalSeqOf[AllowedPNINPNItem](r, enc, 1, maxnoofEPLMNsPlusOne)
	if err != nil {
		return err
	}

	*l = items

	return nil
}

type AvailableBitrateReportThresholdItem struct {
	ReportingThreshold ReportingThreshold
	iEExtensions       extensionMeta          `per:",exttable"`
	extensions         per.ExtensionAdditions `per:",extraw"`
}

type Periodicity int64

func (v Periodicity) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return per.EncodeInteger(w, enc, per.Bounds{LB: 0, HasLB: true, UB: 640000, HasUB: true, Extensible: true}, int64(v))
}

func (v *Periodicity) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	n, err := per.DecodeInteger(r, enc, per.Bounds{LB: 0, HasLB: true, UB: 640000, HasUB: true, Extensible: true})
	if err != nil {
		return err
	}

	*v = Periodicity(n)

	return nil
}

func (v Periodicity) Diagnose(*ap.Diagnosis[ProtocolIEID]) bool {
	return outsideRoot(int64(v), 0, 640000)
}

type SurvivalTime int64

func (v SurvivalTime) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return per.EncodeInteger(w, enc, per.Bounds{LB: 0, HasLB: true, UB: 1920000, HasUB: true, Extensible: true}, int64(v))
}

func (v *SurvivalTime) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	n, err := per.DecodeInteger(r, enc, per.Bounds{LB: 0, HasLB: true, UB: 1920000, HasUB: true, Extensible: true})
	if err != nil {
		return err
	}

	*v = SurvivalTime(n)

	return nil
}

func (v SurvivalTime) Diagnose(*ap.Diagnosis[ProtocolIEID]) bool {
	return outsideRoot(int64(v), 0, 1920000)
}

type RANfeedbacktype struct {
	Proactive       *RANfeedbacktypeproactive `per:",choice:0"`
	Reactive        *RANfeedbacktypereactive  `per:",choice:1"`
	ChoiceExtension *SingleContainer          `per:",choice:2,extie"`
}

type N6JitterInformationN6JitterLowerBound int32

func (v N6JitterInformationN6JitterLowerBound) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return per.EncodeInteger(w, enc, per.Bounds{LB: -127, HasLB: true, UB: 127, HasUB: true}, int64(v))
}

func (v *N6JitterInformationN6JitterLowerBound) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	n, err := per.DecodeInteger(r, enc, per.Bounds{LB: -127, HasLB: true, UB: 127, HasUB: true})
	if err != nil {
		return err
	}

	*v = N6JitterInformationN6JitterLowerBound(n)

	return nil
}

type N6JitterInformationN6JitterUpperBound int32

func (v N6JitterInformationN6JitterUpperBound) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return per.EncodeInteger(w, enc, per.Bounds{LB: -127, HasLB: true, UB: 127, HasUB: true}, int64(v))
}

func (v *N6JitterInformationN6JitterUpperBound) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	n, err := per.DecodeInteger(r, enc, per.Bounds{LB: -127, HasLB: true, UB: 127, HasUB: true})
	if err != nil {
		return err
	}

	*v = N6JitterInformationN6JitterUpperBound(n)

	return nil
}

type N6JitterInformation struct {
	N6JitterLowerBound N6JitterInformationN6JitterLowerBound
	N6JitterUpperBound N6JitterInformationN6JitterUpperBound
	iEExtensions       extensionMeta          `per:",exttable"`
	extensions         per.ExtensionAdditions `per:",extraw"`
}

type VolumeTimedReportList []VolumeTimedReportItem

func (l VolumeTimedReportList) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return marshalSeqOf(w, enc, 1, maxnoofTimePeriods, []VolumeTimedReportItem(l))
}

func (l *VolumeTimedReportList) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	items, err := unmarshalSeqOf[VolumeTimedReportItem](r, enc, 1, maxnoofTimePeriods)
	if err != nil {
		return err
	}

	*l = items

	return nil
}

type QoSFlowsUsageReportItemRATType uint8

const (
	QoSFlowsUsageReportItemRATTypeNR QoSFlowsUsageReportItemRATType = iota
	QoSFlowsUsageReportItemRATTypeEUTRA
	QoSFlowsUsageReportItemRATTypeNRUnlicensed
	QoSFlowsUsageReportItemRATTypeEUtraUnlicensed
)

type QoSFlowsUsageReportItem struct {
	QosFlowIdentifier       QosFlowIdentifier
	RATType                 QoSFlowsUsageReportItemRATType
	QoSFlowsTimedReportList VolumeTimedReportList
	iEExtensions            extensionMeta          `per:",exttable"`
	extensions              per.ExtensionAdditions `per:",extraw"`
}

type MBSSessionID struct {
	TMGI         TMGI
	NID          *NID                   `per:",optional"`
	iEExtensions extensionMeta          `per:",exttable"`
	extensions   per.ExtensionAdditions `per:",extraw"`
}

type MBSAreaSessionID int64

func (v MBSAreaSessionID) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return per.EncodeInteger(w, enc, per.Bounds{LB: 0, HasLB: true, UB: 65535, HasUB: true, Extensible: true}, int64(v))
}

func (v *MBSAreaSessionID) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	n, err := per.DecodeInteger(r, enc, per.Bounds{LB: 0, HasLB: true, UB: 65535, HasUB: true, Extensible: true})
	if err != nil {
		return err
	}

	*v = MBSAreaSessionID(n)

	return nil
}

func (v MBSAreaSessionID) Diagnose(*ap.Diagnosis[ProtocolIEID]) bool {
	return outsideRoot(int64(v), 0, 65535)
}

type UpdateFeedback struct{ BitString }

func (v UpdateFeedback) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return v.encode(w, enc, 8, 8)
}

func (v *UpdateFeedback) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	b, err := decodeBitString(r, enc, 8, 8)
	if err != nil {
		return err
	}

	*v = UpdateFeedback{b}

	return nil
}

type MDTActivation uint8

const (
	MDTActivationImmediateMDTOnly MDTActivation = iota
	MDTActivationLoggedMDTOnly
	MDTActivationImmediateMDTAndTrace
)

type AreaScopeOfMDTNR struct {
	CellBased         *CellBasedMDTNR        `per:",choice:0"`
	TABased           *TABasedMDT            `per:",choice:1"`
	PLMNWide          *per.Null              `per:",choice:2"`
	TAIBased          *TAIBasedMDT           `per:",choice:3"`
	PNINPNBasedMDT    *PNINPNBasedMDT        `per:",choice:4,extie:IDPNINPNBasedMDT,crit:ignore"`
	SNPNCellBasedMDT  *SNPNCellBasedMDT      `per:",choice:4,extie:IDSNPNCellBasedMDT,crit:ignore"`
	SNPNTAIBasedMDT   *SNPNTAIBasedMDT       `per:",choice:4,extie:IDSNPNTAIBasedMDT,crit:ignore"`
	SNPNBasedMDT      *SNPNBasedMDT          `per:",choice:4,extie:IDSNPNBasedMDT,crit:ignore"`
	GeographyBasedMDT *GeographyBasedMDT     `per:",choice:4,extie:IDGeographyBasedMDT,crit:ignore"`
	ChoiceExtension   *SingleContainer       `per:",choice:4,extie"`
	choiceCriticality apimpl.WireCriticality `per:",choicecrit"`
}

type MDTModeNr struct {
	ImmediateMDTNr  *ImmediateMDTNr  `per:",choice:0"`
	LoggedMDTNr     *LoggedMDTNr     `per:",choice:1"`
	ChoiceExtension *SingleContainer `per:",choice:2,extie"`
}

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

type PNINPNAreaScopeofMDT struct {
	CAGListforMDT CAGListforMDT
	iEExtensions  extensionMeta          `per:",exttable"`
	extensions    per.ExtensionAdditions `per:",extraw"`
}

type NetworkSliceAreaScopeofMDT struct {
	NetworkSliceListforMDT NetworkSliceListforMDT
	iEExtensions           extensionMeta          `per:",exttable"`
	extensions             per.ExtensionAdditions `per:",extraw"`
}

type AreaScopeOfMDTEUTRA struct {
	CellBased       *CellBasedMDTEUTRA `per:",choice:0"`
	TABased         *TABasedMDT        `per:",choice:1"`
	PLMNWide        *per.Null          `per:",choice:2"`
	TAIBased        *TAIBasedMDT       `per:",choice:3"`
	ChoiceExtension *SingleContainer   `per:",choice:4,extie"`
}

type MDTModeEutra []byte

func (v MDTModeEutra) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return per.EncodeOctetString(w, enc, 0, 0, true, false, false, v)
}

func (v *MDTModeEutra) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	b, err := per.DecodeOctetString(r, enc, 0, 0, true, false, false)
	if err != nil {
		return err
	}

	*v = b

	return nil
}

type MBSServiceArea struct {
	Locationindependent *MBSServiceAreaInformation     `per:",choice:0"`
	Locationdependent   *MBSServiceAreaInformationList `per:",choice:1"`
	ChoiceExtension     *SingleContainer               `per:",choice:2,extie"`
}

type MBSQoSFlowsToBeSetupList []MBSQoSFlowsToBeSetupItem

func (l MBSQoSFlowsToBeSetupList) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return marshalSeqOf(w, enc, 1, maxnoofMBSQoSFlows, []MBSQoSFlowsToBeSetupItem(l))
}

func (l *MBSQoSFlowsToBeSetupList) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	items, err := unmarshalSeqOf[MBSQoSFlowsToBeSetupItem](r, enc, 1, maxnoofMBSQoSFlows)
	if err != nil {
		return err
	}

	*l = items

	return nil
}

type MBSMappingandDataForwardingRequestList []MBSMappingandDataForwardingRequestItem

func (l MBSMappingandDataForwardingRequestList) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return marshalSeqOf(w, enc, 1, maxnoofMRBs, []MBSMappingandDataForwardingRequestItem(l))
}

func (l *MBSMappingandDataForwardingRequestList) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	items, err := unmarshalSeqOf[MBSMappingandDataForwardingRequestItem](r, enc, 1, maxnoofMRBs)
	if err != nil {
		return err
	}

	*l = items

	return nil
}

type UEAppLayerMeasInfoItem struct {
	UEAppLayerMeasConfigInfo UEAppLayerMeasConfigInfo
	iEExtensions             extensionMeta          `per:",exttable"`
	extensions               per.ExtensionAdditions `per:",extraw"`
}

type CandidateRelayUEID uint32

func (v CandidateRelayUEID) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return encodeBitStringUint(w, enc, uint64(v), 24)
}

func (v *CandidateRelayUEID) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	n, err := decodeBitStringUint(r, enc, 24)
	if err != nil {
		return err
	}

	*v = CandidateRelayUEID(n)

	return nil
}

type SCGActivationTime uint16

func (v SCGActivationTime) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return per.EncodeInteger(w, enc, per.Bounds{LB: 0, HasLB: true, UB: 40950, HasUB: true}, int64(v))
}

func (v *SCGActivationTime) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	n, err := per.DecodeInteger(r, enc, per.Bounds{LB: 0, HasLB: true, UB: 40950, HasUB: true})
	if err != nil {
		return err
	}

	*v = SCGActivationTime(n)

	return nil
}

type EquivalentSNPNsItem struct {
	PlmnIdentity PLMNIdentity
	NID          NID
	iEExtensions extensionMeta          `per:",exttable"`
	extensions   per.ExtensionAdditions `per:",extraw"`
}

type AllowedPNINPNItemPNINPNrestricted uint8

const (
	AllowedPNINPNItemPNINPNrestrictedRestricted AllowedPNINPNItemPNINPNrestricted = iota
	AllowedPNINPNItemPNINPNrestrictedNotRestricted
)

type AllowedPNINPNItem struct {
	PLMNIdentity          PLMNIdentity
	PNINPNrestricted      AllowedPNINPNItemPNINPNrestricted
	AllowedCAGListperPLMN AllowedCAGListperPLMN
	iEExtensions          extensionMeta          `per:",exttable"`
	extensions            per.ExtensionAdditions `per:",extraw"`
}

type ReportingThreshold int64

func (v ReportingThreshold) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return per.EncodeInteger(w, enc, per.Bounds{LB: 0, HasLB: true, UB: 4000000000, HasUB: true, Extensible: true}, int64(v))
}

func (v *ReportingThreshold) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	n, err := per.DecodeInteger(r, enc, per.Bounds{LB: 0, HasLB: true, UB: 4000000000, HasUB: true, Extensible: true})
	if err != nil {
		return err
	}

	*v = ReportingThreshold(n)

	return nil
}

func (v ReportingThreshold) Diagnose(*ap.Diagnosis[ProtocolIEID]) bool {
	return outsideRoot(int64(v), 0, 4000000000)
}

type RANfeedbacktypeproactive struct {
	BurstArrivalTimeWindow BurstArrivalTimeWindow
	PeriodicityRange       *PeriodicityRange      `per:",optional"`
	iEExtensions           extensionMeta          `per:",exttable"`
	extensions             per.ExtensionAdditions `per:",extraw"`
}

type RANfeedbacktypereactiveCapabilityForBATAdaptation uint8

const (
	RANfeedbacktypereactiveCapabilityForBATAdaptationTrue RANfeedbacktypereactiveCapabilityForBATAdaptation = iota
)

type RANfeedbacktypereactive struct {
	CapabilityForBATAdaptation RANfeedbacktypereactiveCapabilityForBATAdaptation
	iEExtensions               extensionMeta          `per:",exttable"`
	extensions                 per.ExtensionAdditions `per:",extraw"`
}

type VolumeTimedReportItemStartTimeStamp []byte

func (v VolumeTimedReportItemStartTimeStamp) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return per.EncodeOctetString(w, enc, 4, 4, true, true, false, v)
}

func (v *VolumeTimedReportItemStartTimeStamp) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	b, err := per.DecodeOctetString(r, enc, 4, 4, true, true, false)
	if err != nil {
		return err
	}

	*v = b

	return nil
}

type VolumeTimedReportItemEndTimeStamp []byte

func (v VolumeTimedReportItemEndTimeStamp) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return per.EncodeOctetString(w, enc, 4, 4, true, true, false, v)
}

func (v *VolumeTimedReportItemEndTimeStamp) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	b, err := per.DecodeOctetString(r, enc, 4, 4, true, true, false)
	if err != nil {
		return err
	}

	*v = b

	return nil
}

type VolumeTimedReportItemUsageCountUL uint64

func (v VolumeTimedReportItemUsageCountUL) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return encodeUint64Range(w, enc, uint64(v))
}

func (v *VolumeTimedReportItemUsageCountUL) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	n, err := decodeUint64Range(r, enc)
	*v = VolumeTimedReportItemUsageCountUL(n)

	return err
}

type VolumeTimedReportItemUsageCountDL uint64

func (v VolumeTimedReportItemUsageCountDL) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return encodeUint64Range(w, enc, uint64(v))
}

func (v *VolumeTimedReportItemUsageCountDL) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	n, err := decodeUint64Range(r, enc)
	*v = VolumeTimedReportItemUsageCountDL(n)

	return err
}

type VolumeTimedReportItem struct {
	StartTimeStamp VolumeTimedReportItemStartTimeStamp
	EndTimeStamp   VolumeTimedReportItemEndTimeStamp
	UsageCountUL   VolumeTimedReportItemUsageCountUL
	UsageCountDL   VolumeTimedReportItemUsageCountDL
	iEExtensions   extensionMeta          `per:",exttable"`
	extensions     per.ExtensionAdditions `per:",extraw"`
}

type TMGI []byte

func (v TMGI) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return per.EncodeOctetString(w, enc, 6, 6, true, true, false, v)
}

func (v *TMGI) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	b, err := per.DecodeOctetString(r, enc, 6, 6, true, true, false)
	if err != nil {
		return err
	}

	*v = b

	return nil
}

type CellBasedMDTNR struct {
	CellIdListforMDT CellIdListforMDTNR
	iEExtensions     extensionMeta          `per:",exttable"`
	extensions       per.ExtensionAdditions `per:",extraw"`
}

type TABasedMDT struct {
	TAListforMDT TAListforMDT
	iEExtensions extensionMeta          `per:",exttable"`
	extensions   per.ExtensionAdditions `per:",extraw"`
}

type TAIBasedMDT struct {
	TAIListforMDT TAIListforMDT
	iEExtensions  extensionMeta          `per:",exttable"`
	extensions    per.ExtensionAdditions `per:",extraw"`
}

type PNINPNBasedMDT struct {
	CAGListforMDT CAGListforMDT
	iEExtensions  extensionMeta          `per:",exttable"`
	extensions    per.ExtensionAdditions `per:",extraw"`
}

type SNPNCellBasedMDT struct {
	SNPNcellIdListforMDT SNPNCellIdListforMDT
	iEExtensions         extensionMeta          `per:",exttable"`
	extensions           per.ExtensionAdditions `per:",extraw"`
}

type SNPNTAIBasedMDT struct {
	SNPNTAIListforMDT SNPNTAIListforMDT
	iEExtensions      extensionMeta          `per:",exttable"`
	extensions        per.ExtensionAdditions `per:",extraw"`
}

type SNPNBasedMDT struct {
	SNPNListforMDT SNPNListforMDT
	iEExtensions   extensionMeta          `per:",exttable"`
	extensions     per.ExtensionAdditions `per:",extraw"`
}

type GeographyBasedMDT struct {
	NTNGeographicalArea NTNGeographicalAreaList
	NTNPLMNList         MDTPLMNList            `per:",optional"`
	iEExtensions        extensionMeta          `per:",exttable"`
	extensions          per.ExtensionAdditions `per:",extraw"`
}

type ImmediateMDTNr struct {
	MeasurementsToActivate            MeasurementsToActivate
	M1Configuration                   *M1Configuration                   `per:",optional"`
	M4Configuration                   *M4Configuration                   `per:",optional"`
	M5Configuration                   *M5Configuration                   `per:",optional"`
	M6Configuration                   *M6Configuration                   `per:",optional"`
	M7Configuration                   *M7Configuration                   `per:",optional"`
	BluetoothMeasurementConfiguration *BluetoothMeasurementConfiguration `per:",optional"`
	WLANMeasurementConfiguration      *WLANMeasurementConfiguration      `per:",optional"`
	MDTLocationInfo                   *MDTLocationInfo                   `per:",optional"`
	SensorMeasurementConfiguration    *SensorMeasurementConfiguration    `per:",optional"`
	iEExtensions                      extensionMeta                      `per:",exttable"`
	extensions                        per.ExtensionAdditions             `per:",extraw"`
}

type LoggedMDTNr struct {
	LoggingInterval                   LoggingInterval
	LoggingDuration                   LoggingDuration
	LoggedMDTTrigger                  LoggedMDTTrigger
	BluetoothMeasurementConfiguration *BluetoothMeasurementConfiguration `per:",optional"`
	WLANMeasurementConfiguration      *WLANMeasurementConfiguration      `per:",optional"`
	SensorMeasurementConfiguration    *SensorMeasurementConfiguration    `per:",optional"`
	AreaScopeOfNeighCellsList         AreaScopeOfNeighCellsList          `per:",optional"`
	EarlyMeasurement                  *EarlyMeasurement                  `per:",extie:IDEarlyMeasurement,crit:ignore"`
	iEExtensions                      extensionMeta                      `per:",exttable"`
	extensions                        per.ExtensionAdditions             `per:",extraw"`
}

type CAGListforMDT []CAGListforMDTItem

func (l CAGListforMDT) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return marshalSeqOf(w, enc, 1, maxnoofCAGforMDT, []CAGListforMDTItem(l))
}

func (l *CAGListforMDT) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	items, err := unmarshalSeqOf[CAGListforMDTItem](r, enc, 1, maxnoofCAGforMDT)
	if err != nil {
		return err
	}

	*l = items

	return nil
}

type NetworkSliceListforMDT []NetworkSliceItemforMDT

func (l NetworkSliceListforMDT) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return marshalSeqOf(w, enc, 1, maxnoofMDTPLMNs, []NetworkSliceItemforMDT(l))
}

func (l *NetworkSliceListforMDT) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	items, err := unmarshalSeqOf[NetworkSliceItemforMDT](r, enc, 1, maxnoofMDTPLMNs)
	if err != nil {
		return err
	}

	*l = items

	return nil
}

type CellBasedMDTEUTRA struct {
	CellIdListforMDT CellIdListforMDTEUTRA
	iEExtensions     extensionMeta          `per:",exttable"`
	extensions       per.ExtensionAdditions `per:",extraw"`
}

type MBSServiceAreaInformation struct {
	MBSServiceAreaCellList     MBSServiceAreaCellList     `per:",optional"`
	MBSServiceAreaTAIList      MBSServiceAreaTAIList      `per:",optional"`
	MBSIntendedServiceAreaList MBSIntendedServiceAreaList `per:",extie:IDMBSIntendedServiceAreaList,crit:ignore"`
	iEExtensions               extensionMeta              `per:",exttable"`
	extensions                 per.ExtensionAdditions     `per:",extraw"`
}

type MBSServiceAreaInformationList []MBSServiceAreaInformationItem

func (l MBSServiceAreaInformationList) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return marshalSeqOf(w, enc, 1, maxnoofMBSServiceAreaInformation, []MBSServiceAreaInformationItem(l))
}

func (l *MBSServiceAreaInformationList) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	items, err := unmarshalSeqOf[MBSServiceAreaInformationItem](r, enc, 1, maxnoofMBSServiceAreaInformation)
	if err != nil {
		return err
	}

	*l = items

	return nil
}

type MBSQoSFlowsToBeSetupItem struct {
	MBSqosFlowIdentifier         QosFlowIdentifier
	MBSqosFlowLevelQosParameters QosFlowLevelQosParameters
	iEExtensions                 extensionMeta          `per:",exttable"`
	extensions                   per.ExtensionAdditions `per:",extraw"`
}

type MBSMappingandDataForwardingRequestItem struct {
	MRBID                  MRBID
	MBSQoSFlowList         MBSQoSFlowList
	MRBProgressInformation *MRBProgressInformation `per:",optional"`
	iEExtensions           extensionMeta           `per:",exttable"`
	extensions             per.ExtensionAdditions  `per:",extraw"`
}

type UEAppLayerMeasConfigInfoQoEMeasurementStatus uint8

const (
	UEAppLayerMeasConfigInfoQoEMeasurementStatusOngoing UEAppLayerMeasConfigInfoQoEMeasurementStatus = iota
)

type UEAppLayerMeasConfigInfoContainerForAppLayerMeasConfig []byte

func (v UEAppLayerMeasConfigInfoContainerForAppLayerMeasConfig) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return per.EncodeOctetString(w, enc, 1, 8000, true, true, false, v)
}

func (v *UEAppLayerMeasConfigInfoContainerForAppLayerMeasConfig) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	b, err := per.DecodeOctetString(r, enc, 1, 8000, true, true, false)
	if err != nil {
		return err
	}

	*v = b

	return nil
}

type UEAppLayerMeasConfigInfoMeasConfigAppLayerID int64

func (v UEAppLayerMeasConfigInfoMeasConfigAppLayerID) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return per.EncodeInteger(w, enc, per.Bounds{LB: 0, HasLB: true, UB: 15, HasUB: true, Extensible: true}, int64(v))
}

func (v *UEAppLayerMeasConfigInfoMeasConfigAppLayerID) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	n, err := per.DecodeInteger(r, enc, per.Bounds{LB: 0, HasLB: true, UB: 15, HasUB: true, Extensible: true})
	if err != nil {
		return err
	}

	*v = UEAppLayerMeasConfigInfoMeasConfigAppLayerID(n)

	return nil
}

func (v UEAppLayerMeasConfigInfoMeasConfigAppLayerID) Diagnose(*ap.Diagnosis[ProtocolIEID]) bool {
	return outsideRoot(int64(v), 0, 15)
}

type UEAppLayerMeasConfigInfo struct {
	QoEReference                   QoEReference
	ServiceType                    ServiceType
	AreaScopeOfQMC                 *AreaScopeOfQMC `per:",optional"`
	MeasCollEntityIPAddress        TransportLayerAddress
	QoEMeasurementStatus           *UEAppLayerMeasConfigInfoQoEMeasurementStatus          `per:",optional"`
	ContainerForAppLayerMeasConfig UEAppLayerMeasConfigInfoContainerForAppLayerMeasConfig `per:",optional"`
	MeasConfigAppLayerID           *UEAppLayerMeasConfigInfoMeasConfigAppLayerID          `per:",optional"`
	SliceSupportListQMC            SliceSupportListQMC                                    `per:",optional"`
	MDTAlignmentInfo               *MDTAlignmentInfo                                      `per:",optional"`
	AvailableRANVisibleQoEMetrics  *AvailableRANVisibleQoEMetrics                         `per:",optional"`
	AssistanceInformationQoEMeas   *AssistanceInformationQoEMeas                          `per:",extie:IDAssistanceInformationQoEMeas,crit:ignore"`
	CommServiceType                *CommServiceType                                       `per:",extie:IDCommServiceType,crit:ignore"`
	QoERVQoEReportingPaths         *QoERVQoEReportingPaths                                `per:",extie:IDQoERVQoEReportingPaths,crit:ignore"`
	iEExtensions                   extensionMeta                                          `per:",exttable"`
	extensions                     per.ExtensionAdditions                                 `per:",extraw"`
}

type AllowedCAGListperPLMN []CAGID

func (l AllowedCAGListperPLMN) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return marshalSeqOf(w, enc, 1, maxnoofAllowedCAGsperPLMN, []CAGID(l))
}

func (l *AllowedCAGListperPLMN) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	items, err := unmarshalSeqOf[CAGID](r, enc, 1, maxnoofAllowedCAGsperPLMN)
	if err != nil {
		return err
	}

	*l = items

	return nil
}

type BurstArrivalTimeWindowBurstArrivalTimeWindowStart int64

func (v BurstArrivalTimeWindowBurstArrivalTimeWindowStart) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return per.EncodeInteger(w, enc, per.Bounds{LB: 0, HasLB: true, UB: 640000, HasUB: true, Extensible: true}, int64(v))
}

func (v *BurstArrivalTimeWindowBurstArrivalTimeWindowStart) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	n, err := per.DecodeInteger(r, enc, per.Bounds{LB: 0, HasLB: true, UB: 640000, HasUB: true, Extensible: true})
	if err != nil {
		return err
	}

	*v = BurstArrivalTimeWindowBurstArrivalTimeWindowStart(n)

	return nil
}

func (v BurstArrivalTimeWindowBurstArrivalTimeWindowStart) Diagnose(*ap.Diagnosis[ProtocolIEID]) bool {
	return outsideRoot(int64(v), 0, 640000)
}

type BurstArrivalTimeWindowBurstArrivalTimeWindowEnd int64

func (v BurstArrivalTimeWindowBurstArrivalTimeWindowEnd) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return per.EncodeInteger(w, enc, per.Bounds{LB: 0, HasLB: true, UB: 640000, HasUB: true, Extensible: true}, int64(v))
}

func (v *BurstArrivalTimeWindowBurstArrivalTimeWindowEnd) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	n, err := per.DecodeInteger(r, enc, per.Bounds{LB: 0, HasLB: true, UB: 640000, HasUB: true, Extensible: true})
	if err != nil {
		return err
	}

	*v = BurstArrivalTimeWindowBurstArrivalTimeWindowEnd(n)

	return nil
}

func (v BurstArrivalTimeWindowBurstArrivalTimeWindowEnd) Diagnose(*ap.Diagnosis[ProtocolIEID]) bool {
	return outsideRoot(int64(v), 0, 640000)
}

type BurstArrivalTimeWindow struct {
	BurstArrivalTimeWindowStart BurstArrivalTimeWindowBurstArrivalTimeWindowStart
	BurstArrivalTimeWindowEnd   BurstArrivalTimeWindowBurstArrivalTimeWindowEnd
	iEExtensions                extensionMeta          `per:",exttable"`
	extensions                  per.ExtensionAdditions `per:",extraw"`
}

type PeriodicityRange struct {
	PeriodicityBound *PeriodicityBound       `per:",choice:0"`
	PeriodicityList  *AllowedPeriodicityList `per:",choice:1"`
	ChoiceExtension  *SingleContainer        `per:",choice:2,extie"`
}

type CellIdListforMDTNR []NRCGI

func (l CellIdListforMDTNR) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return marshalSeqOf(w, enc, 1, maxnoofCellIDforMDT, []NRCGI(l))
}

func (l *CellIdListforMDTNR) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	items, err := unmarshalSeqOf[NRCGI](r, enc, 1, maxnoofCellIDforMDT)
	if err != nil {
		return err
	}

	*l = items

	return nil
}

type TAListforMDT []TAC

func (l TAListforMDT) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return marshalSeqOf(w, enc, 1, maxnoofTAforMDT, []TAC(l))
}

func (l *TAListforMDT) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	items, err := unmarshalSeqOf[TAC](r, enc, 1, maxnoofTAforMDT)
	if err != nil {
		return err
	}

	*l = items

	return nil
}

type TAIListforMDT []TAI

func (l TAIListforMDT) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return marshalSeqOf(w, enc, 1, maxnoofTAforMDT, []TAI(l))
}

func (l *TAIListforMDT) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	items, err := unmarshalSeqOf[TAI](r, enc, 1, maxnoofTAforMDT)
	if err != nil {
		return err
	}

	*l = items

	return nil
}

type SNPNCellIdListforMDT []SNPNCellIdListforMDTItem

func (l SNPNCellIdListforMDT) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return marshalSeqOf(w, enc, 1, maxnoofCellIDforMDT, []SNPNCellIdListforMDTItem(l))
}

func (l *SNPNCellIdListforMDT) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	items, err := unmarshalSeqOf[SNPNCellIdListforMDTItem](r, enc, 1, maxnoofCellIDforMDT)
	if err != nil {
		return err
	}

	*l = items

	return nil
}

type SNPNTAIListforMDT []SNPNTAIListforMDTItem

func (l SNPNTAIListforMDT) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return marshalSeqOf(w, enc, 1, maxnoofTAforMDT, []SNPNTAIListforMDTItem(l))
}

func (l *SNPNTAIListforMDT) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	items, err := unmarshalSeqOf[SNPNTAIListforMDTItem](r, enc, 1, maxnoofTAforMDT)
	if err != nil {
		return err
	}

	*l = items

	return nil
}

type SNPNListforMDT []SNPNListforMDTItem

func (l SNPNListforMDT) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return marshalSeqOf(w, enc, 1, maxnoofMDTSNPNs, []SNPNListforMDTItem(l))
}

func (l *SNPNListforMDT) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	items, err := unmarshalSeqOf[SNPNListforMDTItem](r, enc, 1, maxnoofMDTSNPNs)
	if err != nil {
		return err
	}

	*l = items

	return nil
}

type NTNGeographicalAreaList []NTNGeographicalAreaItem

func (l NTNGeographicalAreaList) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return marshalSeqOf(w, enc, 1, maxnoofAreaNTN, []NTNGeographicalAreaItem(l))
}

func (l *NTNGeographicalAreaList) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	items, err := unmarshalSeqOf[NTNGeographicalAreaItem](r, enc, 1, maxnoofAreaNTN)
	if err != nil {
		return err
	}

	*l = items

	return nil
}

type MeasurementsToActivate uint8

func (v MeasurementsToActivate) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return encodeBitStringUint(w, enc, uint64(v), 8)
}

func (v *MeasurementsToActivate) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	n, err := decodeBitStringUint(r, enc, 8)
	if err != nil {
		return err
	}

	*v = MeasurementsToActivate(n)

	return nil
}

type M1Configuration struct {
	M1reportingTrigger                  M1ReportingTrigger
	M1thresholdEventA2                  *M1ThresholdEventA2                  `per:",optional"`
	M1periodicReporting                 *M1PeriodicReporting                 `per:",optional"`
	IncludeBeamMeasurementsIndication   *IncludeBeamMeasurementsIndication   `per:",extie:IDIncludeBeamMeasurementsIndication,crit:ignore"`
	BeamMeasurementsReportConfiguration *BeamMeasurementsReportConfiguration `per:",extie:IDBeamMeasurementsReportConfiguration,crit:ignore,presence:conditional,cond:m1IncludesBeamMeasurements"`
	iEExtensions                        extensionMeta                        `per:",exttable"`
	extensions                          per.ExtensionAdditions               `per:",extraw"`
}

type M4Configuration struct {
	M4period       M4period
	M4linkstolog   Linkstolog
	M4ReportAmount *M4ReportAmountMDT     `per:",extie:IDM4ReportAmount,crit:ignore"`
	iEExtensions   extensionMeta          `per:",exttable"`
	extensions     per.ExtensionAdditions `per:",extraw"`
}

type M5Configuration struct {
	M5period       M5period
	M5linkstolog   Linkstolog
	M5ReportAmount *M5ReportAmountMDT     `per:",extie:IDM5ReportAmount,crit:ignore"`
	iEExtensions   extensionMeta          `per:",exttable"`
	extensions     per.ExtensionAdditions `per:",extraw"`
}

type M6Configuration struct {
	M6reportInterval                        M6reportInterval
	M6linkstolog                            Linkstolog
	M6ReportAmount                          *M6ReportAmountMDT                      `per:",extie:IDM6ReportAmount,crit:ignore"`
	ExcessPacketDelayThresholdConfiguration ExcessPacketDelayThresholdConfiguration `per:",extie:IDExcessPacketDelayThresholdConfiguration,crit:ignore"`
	iEExtensions                            extensionMeta                           `per:",exttable"`
	extensions                              per.ExtensionAdditions                  `per:",extraw"`
}

type M7Configuration struct {
	M7period       M7period
	M7linkstolog   Linkstolog
	M7ReportAmount *M7ReportAmountMDT     `per:",extie:IDM7ReportAmount,crit:ignore"`
	iEExtensions   extensionMeta          `per:",exttable"`
	extensions     per.ExtensionAdditions `per:",extraw"`
}

type BluetoothMeasurementConfigurationBtrssi uint8

const (
	BluetoothMeasurementConfigurationBtrssiTrue BluetoothMeasurementConfigurationBtrssi = iota
)

type BluetoothMeasurementConfiguration struct {
	BluetoothMeasConfig         BluetoothMeasConfig
	BluetoothMeasConfigNameList BluetoothMeasConfigNameList              `per:",optional"`
	Btrssi                      *BluetoothMeasurementConfigurationBtrssi `per:",optional"`
	iEExtensions                extensionMeta                            `per:",exttable"`
	extensions                  per.ExtensionAdditions                   `per:",extraw"`
}

type WLANMeasurementConfigurationWlanrssi uint8

const (
	WLANMeasurementConfigurationWlanrssiTrue WLANMeasurementConfigurationWlanrssi = iota
)

type WLANMeasurementConfigurationWlanrtt uint8

const (
	WLANMeasurementConfigurationWlanrttTrue WLANMeasurementConfigurationWlanrtt = iota
)

type WLANMeasurementConfiguration struct {
	WlanMeasConfig         WLANMeasConfig
	WlanMeasConfigNameList WLANMeasConfigNameList                `per:",optional"`
	Wlanrssi               *WLANMeasurementConfigurationWlanrssi `per:",optional"`
	Wlanrtt                *WLANMeasurementConfigurationWlanrtt  `per:",optional"`
	iEExtensions           extensionMeta                         `per:",exttable"`
	extensions             per.ExtensionAdditions                `per:",extraw"`
}

type MDTLocationInfo struct {
	MDTLocationInformation MDTLocationInformation
	iEExtensions           extensionMeta          `per:",exttable"`
	extensions             per.ExtensionAdditions `per:",extraw"`
}

type SensorMeasurementConfiguration struct {
	SensorMeasConfig         SensorMeasConfig
	SensorMeasConfigNameList SensorMeasConfigNameList `per:",optional"`
	iEExtensions             extensionMeta            `per:",exttable"`
	extensions               per.ExtensionAdditions   `per:",extraw"`
}

type LoggingInterval uint8

const (
	LoggingIntervalMs320 LoggingInterval = iota
	LoggingIntervalMs640
	LoggingIntervalMs1280
	LoggingIntervalMs2560
	LoggingIntervalMs5120
	LoggingIntervalMs10240
	LoggingIntervalMs20480
	LoggingIntervalMs30720
	LoggingIntervalMs40960
	LoggingIntervalMs61440
	LoggingIntervalInfinity
)

type LoggingDuration uint8

const (
	LoggingDurationM10 LoggingDuration = iota
	LoggingDurationM20
	LoggingDurationM40
	LoggingDurationM60
	LoggingDurationM90
	LoggingDurationM120
)

type LoggedMDTTrigger struct {
	Periodical      *per.Null        `per:",choice:0"`
	EventTrigger    *EventTrigger    `per:",choice:1"`
	ChoiceExtension *SingleContainer `per:",choice:2,extie"`
}

type AreaScopeOfNeighCellsList []AreaScopeOfNeighCellsItem

func (l AreaScopeOfNeighCellsList) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return marshalSeqOf(w, enc, 1, maxnoofFreqforMDT, []AreaScopeOfNeighCellsItem(l))
}

func (l *AreaScopeOfNeighCellsList) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	items, err := unmarshalSeqOf[AreaScopeOfNeighCellsItem](r, enc, 1, maxnoofFreqforMDT)
	if err != nil {
		return err
	}

	*l = items

	return nil
}

type EarlyMeasurement uint8

const (
	EarlyMeasurementTrue EarlyMeasurement = iota
)

type CAGListforMDTItem struct {
	PlmnID       PLMNIdentity
	CAGID        CAGID
	iEExtensions extensionMeta          `per:",exttable"`
	extensions   per.ExtensionAdditions `per:",extraw"`
}

type NetworkSliceItemforMDT struct {
	PlmnID       PLMNIdentity
	SliceMDTList SliceMDTList
	iEExtensions extensionMeta          `per:",exttable"`
	extensions   per.ExtensionAdditions `per:",extraw"`
}

type CellIdListforMDTEUTRA []EUTRACGI

func (l CellIdListforMDTEUTRA) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return marshalSeqOf(w, enc, 1, maxnoofCellIDforMDT, []EUTRACGI(l))
}

func (l *CellIdListforMDTEUTRA) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	items, err := unmarshalSeqOf[EUTRACGI](r, enc, 1, maxnoofCellIDforMDT)
	if err != nil {
		return err
	}

	*l = items

	return nil
}

type MBSServiceAreaCellList []NRCGI

func (l MBSServiceAreaCellList) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return marshalSeqOf(w, enc, 1, maxnoofCellsforMBS, []NRCGI(l))
}

func (l *MBSServiceAreaCellList) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	items, err := unmarshalSeqOf[NRCGI](r, enc, 1, maxnoofCellsforMBS)
	if err != nil {
		return err
	}

	*l = items

	return nil
}

type MBSServiceAreaTAIList []TAI

func (l MBSServiceAreaTAIList) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return marshalSeqOf(w, enc, 1, maxnoofTAIforMBS, []TAI(l))
}

func (l *MBSServiceAreaTAIList) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	items, err := unmarshalSeqOf[TAI](r, enc, 1, maxnoofTAIforMBS)
	if err != nil {
		return err
	}

	*l = items

	return nil
}

type MBSIntendedServiceAreaList []IntendedServiceAreaCoordinates

func (l MBSIntendedServiceAreaList) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return marshalSeqOf(w, enc, 1, maxnoofIntendedAreasforMBS, []IntendedServiceAreaCoordinates(l))
}

func (l *MBSIntendedServiceAreaList) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	items, err := unmarshalSeqOf[IntendedServiceAreaCoordinates](r, enc, 1, maxnoofIntendedAreasforMBS)
	if err != nil {
		return err
	}

	*l = items

	return nil
}

type MBSServiceAreaInformationItem struct {
	MBSAreaSessionID          MBSAreaSessionID
	MBSServiceAreaInformation MBSServiceAreaInformation
	iEExtensions              extensionMeta          `per:",exttable"`
	extensions                per.ExtensionAdditions `per:",extraw"`
}

type MRBID int64

func (v MRBID) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return per.EncodeInteger(w, enc, per.Bounds{LB: 1, HasLB: true, UB: 512, HasUB: true, Extensible: true}, int64(v))
}

func (v *MRBID) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	n, err := per.DecodeInteger(r, enc, per.Bounds{LB: 1, HasLB: true, UB: 512, HasUB: true, Extensible: true})
	if err != nil {
		return err
	}

	*v = MRBID(n)

	return nil
}

func (v MRBID) Diagnose(*ap.Diagnosis[ProtocolIEID]) bool { return outsideRoot(int64(v), 1, 512) }

type MBSQoSFlowList []QosFlowIdentifier

func (l MBSQoSFlowList) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return marshalSeqOf(w, enc, 1, maxnoofMBSQoSFlows, []QosFlowIdentifier(l))
}

func (l *MBSQoSFlowList) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	items, err := unmarshalSeqOf[QosFlowIdentifier](r, enc, 1, maxnoofMBSQoSFlows)
	if err != nil {
		return err
	}

	*l = items

	return nil
}

type MRBProgressInformationPDCPSNLength12 uint16

func (v MRBProgressInformationPDCPSNLength12) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return per.EncodeInteger(w, enc, per.Bounds{LB: 0, HasLB: true, UB: 4095, HasUB: true}, int64(v))
}

func (v *MRBProgressInformationPDCPSNLength12) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	n, err := per.DecodeInteger(r, enc, per.Bounds{LB: 0, HasLB: true, UB: 4095, HasUB: true})
	if err != nil {
		return err
	}

	*v = MRBProgressInformationPDCPSNLength12(n)

	return nil
}

type MRBProgressInformationPDCPSNLength18 uint32

func (v MRBProgressInformationPDCPSNLength18) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return per.EncodeInteger(w, enc, per.Bounds{LB: 0, HasLB: true, UB: 262143, HasUB: true}, int64(v))
}

func (v *MRBProgressInformationPDCPSNLength18) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	n, err := per.DecodeInteger(r, enc, per.Bounds{LB: 0, HasLB: true, UB: 262143, HasUB: true})
	if err != nil {
		return err
	}

	*v = MRBProgressInformationPDCPSNLength18(n)

	return nil
}

type MRBProgressInformation struct {
	PDCPSNLength12  *MRBProgressInformationPDCPSNLength12 `per:",choice:0"`
	PDCPSNLength18  *MRBProgressInformationPDCPSNLength18 `per:",choice:1"`
	ChoiceExtension *SingleContainer                      `per:",choice:2,extie"`
}

type QoEReference []byte

func (v QoEReference) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return per.EncodeOctetString(w, enc, 6, 6, true, true, false, v)
}

func (v *QoEReference) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	b, err := per.DecodeOctetString(r, enc, 6, 6, true, true, false)
	if err != nil {
		return err
	}

	*v = b

	return nil
}

type ServiceType uint8

const (
	ServiceTypeStreaming ServiceType = iota
	ServiceTypeMTSI
	ServiceTypeVR
)

type AreaScopeOfQMC struct {
	CellBased       *CellBasedQMC     `per:",choice:0"`
	TABased         *TABasedQMC       `per:",choice:1"`
	TAIBased        *TAIBasedQMC      `per:",choice:2"`
	PLMNAreaBased   *PLMNAreaBasedQMC `per:",choice:3"`
	ChoiceExtension *SingleContainer  `per:",choice:4,extie"`
}

type SliceSupportListQMC []SliceSupportQMCItem

func (l SliceSupportListQMC) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return marshalSeqOf(w, enc, 1, maxnoofSNSSAIforQMC, []SliceSupportQMCItem(l))
}

func (l *SliceSupportListQMC) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	items, err := unmarshalSeqOf[SliceSupportQMCItem](r, enc, 1, maxnoofSNSSAIforQMC)
	if err != nil {
		return err
	}

	*l = items

	return nil
}

type MDTAlignmentInfo struct {
	SbasedMDT       *NGRANTraceID    `per:",choice:0"`
	ChoiceExtension *SingleContainer `per:",choice:1,extie"`
}

type AvailableRANVisibleQoEMetricsApplicationLayerBufferLevelList uint8

const (
	AvailableRANVisibleQoEMetricsApplicationLayerBufferLevelListTrue AvailableRANVisibleQoEMetricsApplicationLayerBufferLevelList = iota
)

type AvailableRANVisibleQoEMetricsPlayoutDelayForMediaStartup uint8

const (
	AvailableRANVisibleQoEMetricsPlayoutDelayForMediaStartupTrue AvailableRANVisibleQoEMetricsPlayoutDelayForMediaStartup = iota
)

type AvailableRANVisibleQoEMetrics struct {
	ApplicationLayerBufferLevelList *AvailableRANVisibleQoEMetricsApplicationLayerBufferLevelList `per:",optional"`
	PlayoutDelayForMediaStartup     *AvailableRANVisibleQoEMetricsPlayoutDelayForMediaStartup     `per:",optional"`
	iEExtensions                    extensionMeta                                                 `per:",exttable"`
	extensions                      per.ExtensionAdditions                                        `per:",extraw"`
}

type AssistanceInformationQoEMeas int64

func (v AssistanceInformationQoEMeas) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return per.EncodeInteger(w, enc, per.Bounds{LB: 1, HasLB: true, UB: 16, HasUB: true, Extensible: true}, int64(v))
}

func (v *AssistanceInformationQoEMeas) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	n, err := per.DecodeInteger(r, enc, per.Bounds{LB: 1, HasLB: true, UB: 16, HasUB: true, Extensible: true})
	if err != nil {
		return err
	}

	*v = AssistanceInformationQoEMeas(n)

	return nil
}

func (v AssistanceInformationQoEMeas) Diagnose(*ap.Diagnosis[ProtocolIEID]) bool {
	return outsideRoot(int64(v), 1, 16)
}

type CommServiceType uint8

const (
	CommServiceTypeMBSBroadcast CommServiceType = iota
	CommServiceTypeMBSMulticast
	CommServiceTypeUnicast
)

type QoERVQoEReportingPathsQoEReportingPath uint8

const (
	QoERVQoEReportingPathsQoEReportingPathSrb4 QoERVQoEReportingPathsQoEReportingPath = iota
	QoERVQoEReportingPathsQoEReportingPathSrb5
)

type QoERVQoEReportingPathsRVQoEReportingPath uint8

const (
	QoERVQoEReportingPathsRVQoEReportingPathSrb4 QoERVQoEReportingPathsRVQoEReportingPath = iota
	QoERVQoEReportingPathsRVQoEReportingPathSrb5
)

type QoERVQoEReportingPaths struct {
	QoEReportingPath   *QoERVQoEReportingPathsQoEReportingPath   `per:",optional"`
	RVQoEReportingPath *QoERVQoEReportingPathsRVQoEReportingPath `per:",optional"`
	iEExtensions       extensionMeta                             `per:",exttable"`
	extensions         per.ExtensionAdditions                    `per:",extraw"`
}

type CAGID uint32

func (v CAGID) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return encodeBitStringUint(w, enc, uint64(v), 32)
}

func (v *CAGID) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	n, err := decodeBitStringUint(r, enc, 32)
	if err != nil {
		return err
	}

	*v = CAGID(n)

	return nil
}

type PeriodicityBound struct {
	PeriodicityLowerBound Periodicity
	PeriodicityUpperBound Periodicity
	iEExtensions          extensionMeta          `per:",exttable"`
	extensions            per.ExtensionAdditions `per:",extraw"`
}

type AllowedPeriodicityList []Periodicity

func (l AllowedPeriodicityList) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return marshalSeqOf(w, enc, 1, maxnoofPeriodicities, []Periodicity(l))
}

func (l *AllowedPeriodicityList) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	items, err := unmarshalSeqOf[Periodicity](r, enc, 1, maxnoofPeriodicities)
	if err != nil {
		return err
	}

	*l = items

	return nil
}

type SNPNCellIdListforMDTItem struct {
	NRCGI        NRCGI
	NID          NID
	iEExtensions extensionMeta          `per:",exttable"`
	extensions   per.ExtensionAdditions `per:",extraw"`
}

type SNPNTAIListforMDTItem struct {
	TAI          TAI
	NID          NID
	iEExtensions extensionMeta          `per:",exttable"`
	extensions   per.ExtensionAdditions `per:",extraw"`
}

type SNPNListforMDTItem struct {
	PLMNIdentity PLMNIdentity
	NID          NID
	iEExtensions extensionMeta          `per:",exttable"`
	extensions   per.ExtensionAdditions `per:",extraw"`
}

type NTNGeographicalAreaItem struct {
	Circle          *MDTcircle       `per:",choice:0"`
	Polygon         *MDTpolygon      `per:",choice:1"`
	ChoiceExtension *SingleContainer `per:",choice:2,extie"`
}

type M1ReportingTrigger uint8

const (
	M1ReportingTriggerPeriodic M1ReportingTrigger = iota
	M1ReportingTriggerA2eventtriggered
	M1ReportingTriggerA2eventtriggeredPeriodic
)

type M1ThresholdEventA2 struct {
	M1ThresholdType M1ThresholdType
	iEExtensions    extensionMeta          `per:",exttable"`
	extensions      per.ExtensionAdditions `per:",extraw"`
}

type M1PeriodicReporting struct {
	ReportInterval            ReportIntervalMDT
	ReportAmount              ReportAmountMDT
	ExtendedReportIntervalMDT *ExtendedReportIntervalMDT `per:",extie:IDExtendedReportIntervalMDT,crit:ignore"`
	iEExtensions              extensionMeta              `per:",exttable"`
	extensions                per.ExtensionAdditions     `per:",extraw"`
}

type IncludeBeamMeasurementsIndication uint8

const (
	IncludeBeamMeasurementsIndicationTrue IncludeBeamMeasurementsIndication = iota
)

type BeamMeasurementsReportConfiguration struct {
	BeamMeasurementsReportQuantity *BeamMeasurementsReportQuantity `per:",optional"`
	MaxNrofRSIndexesToReport       *MaxNrofRSIndexesToReport       `per:",optional"`
	iEExtensions                   extensionMeta                   `per:",exttable"`
	extensions                     per.ExtensionAdditions          `per:",extraw"`
}

type M4period uint8

const (
	M4periodMs1024 M4period = iota
	M4periodMs2048
	M4periodMs5120
	M4periodMs10240
	M4periodMin1
)

type Linkstolog uint8

const (
	LinkstologUplink Linkstolog = iota
	LinkstologDownlink
	LinkstologBothUplinkAndDownlink
)

type M4ReportAmountMDT uint8

const (
	M4ReportAmountMDTR1 M4ReportAmountMDT = iota
	M4ReportAmountMDTR2
	M4ReportAmountMDTR4
	M4ReportAmountMDTR8
	M4ReportAmountMDTR16
	M4ReportAmountMDTR32
	M4ReportAmountMDTR64
	M4ReportAmountMDTInfinity
)

type M5period uint8

const (
	M5periodMs1024 M5period = iota
	M5periodMs2048
	M5periodMs5120
	M5periodMs10240
	M5periodMin1
)

type M5ReportAmountMDT uint8

const (
	M5ReportAmountMDTR1 M5ReportAmountMDT = iota
	M5ReportAmountMDTR2
	M5ReportAmountMDTR4
	M5ReportAmountMDTR8
	M5ReportAmountMDTR16
	M5ReportAmountMDTR32
	M5ReportAmountMDTR64
	M5ReportAmountMDTInfinity
)

type M6reportInterval uint8

const (
	M6reportIntervalMs120 M6reportInterval = iota
	M6reportIntervalMs240
	M6reportIntervalMs480
	M6reportIntervalMs640
	M6reportIntervalMs1024
	M6reportIntervalMs2048
	M6reportIntervalMs5120
	M6reportIntervalMs10240
	M6reportIntervalMs20480
	M6reportIntervalMs40960
	M6reportIntervalMin1
	M6reportIntervalMin6
	M6reportIntervalMin12
	M6reportIntervalMin30
)

type M6ReportAmountMDT uint8

const (
	M6ReportAmountMDTR1 M6ReportAmountMDT = iota
	M6ReportAmountMDTR2
	M6ReportAmountMDTR4
	M6ReportAmountMDTR8
	M6ReportAmountMDTR16
	M6ReportAmountMDTR32
	M6ReportAmountMDTR64
	M6ReportAmountMDTInfinity
)

type ExcessPacketDelayThresholdConfiguration []ExcessPacketDelayThresholdItem

func (l ExcessPacketDelayThresholdConfiguration) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return marshalSeqOf(w, enc, 1, maxnoofThresholdsForExcessPacketDelay, []ExcessPacketDelayThresholdItem(l))
}

func (l *ExcessPacketDelayThresholdConfiguration) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	items, err := unmarshalSeqOf[ExcessPacketDelayThresholdItem](r, enc, 1, maxnoofThresholdsForExcessPacketDelay)
	if err != nil {
		return err
	}

	*l = items

	return nil
}

type M7period int64

func (v M7period) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return per.EncodeInteger(w, enc, per.Bounds{LB: 1, HasLB: true, UB: 60, HasUB: true, Extensible: true}, int64(v))
}

func (v *M7period) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	n, err := per.DecodeInteger(r, enc, per.Bounds{LB: 1, HasLB: true, UB: 60, HasUB: true, Extensible: true})
	if err != nil {
		return err
	}

	*v = M7period(n)

	return nil
}

func (v M7period) Diagnose(*ap.Diagnosis[ProtocolIEID]) bool { return outsideRoot(int64(v), 1, 60) }

type M7ReportAmountMDT uint8

const (
	M7ReportAmountMDTR1 M7ReportAmountMDT = iota
	M7ReportAmountMDTR2
	M7ReportAmountMDTR4
	M7ReportAmountMDTR8
	M7ReportAmountMDTR16
	M7ReportAmountMDTR32
	M7ReportAmountMDTR64
	M7ReportAmountMDTInfinity
)

type BluetoothMeasConfig uint8

const (
	BluetoothMeasConfigSetup BluetoothMeasConfig = iota
)

type BluetoothMeasConfigNameList []BluetoothMeasConfigNameItem

func (l BluetoothMeasConfigNameList) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return marshalSeqOf(w, enc, 1, maxnoofBluetoothName, []BluetoothMeasConfigNameItem(l))
}

func (l *BluetoothMeasConfigNameList) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	items, err := unmarshalSeqOf[BluetoothMeasConfigNameItem](r, enc, 1, maxnoofBluetoothName)
	if err != nil {
		return err
	}

	*l = items

	return nil
}

type WLANMeasConfig uint8

const (
	WLANMeasConfigSetup WLANMeasConfig = iota
)

type WLANMeasConfigNameList []WLANMeasConfigNameItem

func (l WLANMeasConfigNameList) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return marshalSeqOf(w, enc, 1, maxnoofWLANName, []WLANMeasConfigNameItem(l))
}

func (l *WLANMeasConfigNameList) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	items, err := unmarshalSeqOf[WLANMeasConfigNameItem](r, enc, 1, maxnoofWLANName)
	if err != nil {
		return err
	}

	*l = items

	return nil
}

type MDTLocationInformation uint8

func (v MDTLocationInformation) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return encodeBitStringUint(w, enc, uint64(v), 8)
}

func (v *MDTLocationInformation) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	n, err := decodeBitStringUint(r, enc, 8)
	if err != nil {
		return err
	}

	*v = MDTLocationInformation(n)

	return nil
}

type SensorMeasConfig uint8

const (
	SensorMeasConfigSetup SensorMeasConfig = iota
)

type SensorMeasConfigNameList []SensorMeasConfigNameItem

func (l SensorMeasConfigNameList) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return marshalSeqOf(w, enc, 1, maxnoofSensorName, []SensorMeasConfigNameItem(l))
}

func (l *SensorMeasConfigNameList) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	items, err := unmarshalSeqOf[SensorMeasConfigNameItem](r, enc, 1, maxnoofSensorName)
	if err != nil {
		return err
	}

	*l = items

	return nil
}

type EventTriggerOutOfCoverage uint8

const (
	EventTriggerOutOfCoverageTrue EventTriggerOutOfCoverage = iota
)

type EventTrigger struct {
	OutOfCoverage          *EventTriggerOutOfCoverage `per:",choice:0"`
	EventL1LoggedMDTConfig *EventL1LoggedMDTConfig    `per:",choice:1"`
	ChoiceExtension        *SingleContainer           `per:",choice:2,extie"`
}

type AreaScopeOfNeighCellsItem struct {
	NrFrequencyInfo NRFrequencyInfo
	PciListForMDT   PCIListForMDT          `per:",optional"`
	iEExtensions    extensionMeta          `per:",exttable"`
	extensions      per.ExtensionAdditions `per:",extraw"`
}

type SliceMDTList []SliceMDTItem

func (l SliceMDTList) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return marshalSeqOf(w, enc, 1, maxnoofSliceItemsforMDT, []SliceMDTItem(l))
}

func (l *SliceMDTList) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	items, err := unmarshalSeqOf[SliceMDTItem](r, enc, 1, maxnoofSliceItemsforMDT)
	if err != nil {
		return err
	}

	*l = items

	return nil
}

type IntendedServiceAreaCoordinates []byte

func (v IntendedServiceAreaCoordinates) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return per.EncodeOctetString(w, enc, 0, 0, true, false, false, v)
}

func (v *IntendedServiceAreaCoordinates) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	b, err := per.DecodeOctetString(r, enc, 0, 0, true, false, false)
	if err != nil {
		return err
	}

	*v = b

	return nil
}

type CellBasedQMC struct {
	CellIdListforQMC CellIdListforQMC
	iEExtensions     extensionMeta          `per:",exttable"`
	extensions       per.ExtensionAdditions `per:",extraw"`
}

type TABasedQMC struct {
	TAListforQMC TAListforQMC
	iEExtensions extensionMeta          `per:",exttable"`
	extensions   per.ExtensionAdditions `per:",extraw"`
}

type TAIBasedQMC struct {
	TAIListforQMC TAIListforQMC
	iEExtensions  extensionMeta          `per:",exttable"`
	extensions    per.ExtensionAdditions `per:",extraw"`
}

type PLMNAreaBasedQMC struct {
	PlmnListforQMC PLMNListforQMC
	iEExtensions   extensionMeta          `per:",exttable"`
	extensions     per.ExtensionAdditions `per:",extraw"`
}

type SliceSupportQMCItem struct {
	SNSSAI       SNSSAI
	iEExtensions extensionMeta          `per:",exttable"`
	extensions   per.ExtensionAdditions `per:",extraw"`
}

type MDTcircleReferencelocation []byte

func (v MDTcircleReferencelocation) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return per.EncodeOctetString(w, enc, 0, 0, true, false, false, v)
}

func (v *MDTcircleReferencelocation) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	b, err := per.DecodeOctetString(r, enc, 0, 0, true, false, false)
	if err != nil {
		return err
	}

	*v = b

	return nil
}

type MDTcircleDistanceradius uint16

func (v MDTcircleDistanceradius) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return per.EncodeInteger(w, enc, per.Bounds{LB: 1, HasLB: true, UB: 65535, HasUB: true}, int64(v))
}

func (v *MDTcircleDistanceradius) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	n, err := per.DecodeInteger(r, enc, per.Bounds{LB: 1, HasLB: true, UB: 65535, HasUB: true})
	if err != nil {
		return err
	}

	*v = MDTcircleDistanceradius(n)

	return nil
}

type MDTcircle struct {
	Referencelocation MDTcircleReferencelocation
	Distanceradius    MDTcircleDistanceradius
	iEExtensions      extensionMeta          `per:",exttable"`
	extensions        per.ExtensionAdditions `per:",extraw"`
}

type MDTpolygon []byte

func (v MDTpolygon) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return per.EncodeOctetString(w, enc, 0, 0, true, false, false, v)
}

func (v *MDTpolygon) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	b, err := per.DecodeOctetString(r, enc, 0, 0, true, false, false)
	if err != nil {
		return err
	}

	*v = b

	return nil
}

type M1ThresholdType struct {
	ThresholdRSRP   *ThresholdRSRP   `per:",choice:0"`
	ThresholdRSRQ   *ThresholdRSRQ   `per:",choice:1"`
	ThresholdSINR   *ThresholdSINR   `per:",choice:2"`
	ChoiceExtension *SingleContainer `per:",choice:3,extie"`
}

type ReportIntervalMDT uint8

const (
	ReportIntervalMDTMs120 ReportIntervalMDT = iota
	ReportIntervalMDTMs240
	ReportIntervalMDTMs480
	ReportIntervalMDTMs640
	ReportIntervalMDTMs1024
	ReportIntervalMDTMs2048
	ReportIntervalMDTMs5120
	ReportIntervalMDTMs10240
	ReportIntervalMDTMin1
	ReportIntervalMDTMin6
	ReportIntervalMDTMin12
	ReportIntervalMDTMin30
	ReportIntervalMDTMin60
)

type ReportAmountMDT uint8

const (
	ReportAmountMDTR1 ReportAmountMDT = iota
	ReportAmountMDTR2
	ReportAmountMDTR4
	ReportAmountMDTR8
	ReportAmountMDTR16
	ReportAmountMDTR32
	ReportAmountMDTR64
	ReportAmountMDTRinfinity
)

type ExtendedReportIntervalMDT uint8

const (
	ExtendedReportIntervalMDTMs20480 ExtendedReportIntervalMDT = iota
	ExtendedReportIntervalMDTMs40960
)

type BeamMeasurementsReportQuantityRSRP uint8

const (
	BeamMeasurementsReportQuantityRSRPTrue BeamMeasurementsReportQuantityRSRP = iota
	BeamMeasurementsReportQuantityRSRPFalse
)

type BeamMeasurementsReportQuantityRSRQ uint8

const (
	BeamMeasurementsReportQuantityRSRQTrue BeamMeasurementsReportQuantityRSRQ = iota
	BeamMeasurementsReportQuantityRSRQFalse
)

type BeamMeasurementsReportQuantitySINR uint8

const (
	BeamMeasurementsReportQuantitySINRTrue BeamMeasurementsReportQuantitySINR = iota
	BeamMeasurementsReportQuantitySINRFalse
)

type BeamMeasurementsReportQuantity struct {
	RSRP         BeamMeasurementsReportQuantityRSRP
	RSRQ         BeamMeasurementsReportQuantityRSRQ
	SINR         BeamMeasurementsReportQuantitySINR
	iEExtensions extensionMeta          `per:",exttable"`
	extensions   per.ExtensionAdditions `per:",extraw"`
}

type MaxNrofRSIndexesToReport int64

func (v MaxNrofRSIndexesToReport) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return per.EncodeInteger(w, enc, per.Bounds{LB: 1, HasLB: true, UB: 64, HasUB: true, Extensible: true}, int64(v))
}

func (v *MaxNrofRSIndexesToReport) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	n, err := per.DecodeInteger(r, enc, per.Bounds{LB: 1, HasLB: true, UB: 64, HasUB: true, Extensible: true})
	if err != nil {
		return err
	}

	*v = MaxNrofRSIndexesToReport(n)

	return nil
}

func (v MaxNrofRSIndexesToReport) Diagnose(*ap.Diagnosis[ProtocolIEID]) bool {
	return outsideRoot(int64(v), 1, 64)
}

type ExcessPacketDelayThresholdItem struct {
	FiveQi                          FiveQI
	ExcessPacketDelayThresholdValue ExcessPacketDelayThresholdValue
	iEExtensions                    extensionMeta          `per:",exttable"`
	extensions                      per.ExtensionAdditions `per:",extraw"`
}

type BluetoothMeasConfigNameItem struct {
	BluetoothName BluetoothName
	iEExtensions  extensionMeta          `per:",exttable"`
	extensions    per.ExtensionAdditions `per:",extraw"`
}

type WLANMeasConfigNameItem struct {
	WLANName     WLANName
	iEExtensions extensionMeta          `per:",exttable"`
	extensions   per.ExtensionAdditions `per:",extraw"`
}

type SensorMeasConfigNameItem struct {
	SensorNameConfig SensorNameConfig
	iEExtensions     extensionMeta          `per:",exttable"`
	extensions       per.ExtensionAdditions `per:",extraw"`
}

type EventL1LoggedMDTConfig struct {
	L1Threshold   MeasurementThresholdL1LoggedMDT
	Hysteresis    Hysteresis
	TimeToTrigger TimeToTrigger
	iEExtensions  extensionMeta          `per:",exttable"`
	extensions    per.ExtensionAdditions `per:",extraw"`
}

type NRFrequencyInfo struct {
	NrARFCN           NRARFCN
	FrequencyBandList NRFrequencyBandList
	iEExtensions      extensionMeta          `per:",exttable"`
	extensions        per.ExtensionAdditions `per:",extraw"`
}

type PCIListForMDT []NRPCI

func (l PCIListForMDT) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return marshalSeqOf(w, enc, 1, maxnoofNeighPCIforMDT, []NRPCI(l))
}

func (l *PCIListForMDT) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	items, err := unmarshalSeqOf[NRPCI](r, enc, 1, maxnoofNeighPCIforMDT)
	if err != nil {
		return err
	}

	*l = items

	return nil
}

type SliceMDTItem struct {
	SNSSAI       SNSSAI
	iEExtensions extensionMeta          `per:",exttable"`
	extensions   per.ExtensionAdditions `per:",extraw"`
}

type CellIdListforQMC []NGRANCGI

func (l CellIdListforQMC) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return marshalSeqOf(w, enc, 1, maxnoofCellIDforQMC, []NGRANCGI(l))
}

func (l *CellIdListforQMC) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	items, err := unmarshalSeqOf[NGRANCGI](r, enc, 1, maxnoofCellIDforQMC)
	if err != nil {
		return err
	}

	*l = items

	return nil
}

type TAListforQMC []TAC

func (l TAListforQMC) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return marshalSeqOf(w, enc, 1, maxnoofTAforQMC, []TAC(l))
}

func (l *TAListforQMC) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	items, err := unmarshalSeqOf[TAC](r, enc, 1, maxnoofTAforQMC)
	if err != nil {
		return err
	}

	*l = items

	return nil
}

type TAIListforQMC []TAI

func (l TAIListforQMC) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return marshalSeqOf(w, enc, 1, maxnoofTAforQMC, []TAI(l))
}

func (l *TAIListforQMC) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	items, err := unmarshalSeqOf[TAI](r, enc, 1, maxnoofTAforQMC)
	if err != nil {
		return err
	}

	*l = items

	return nil
}

type PLMNListforQMC []PLMNIdentity

func (l PLMNListforQMC) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return marshalSeqOf(w, enc, 1, maxnoofPLMNforQMC, []PLMNIdentity(l))
}

func (l *PLMNListforQMC) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	items, err := unmarshalSeqOf[PLMNIdentity](r, enc, 1, maxnoofPLMNforQMC)
	if err != nil {
		return err
	}

	*l = items

	return nil
}

type ThresholdRSRP uint8

func (v ThresholdRSRP) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return per.EncodeInteger(w, enc, per.Bounds{LB: 0, HasLB: true, UB: 127, HasUB: true}, int64(v))
}

func (v *ThresholdRSRP) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	n, err := per.DecodeInteger(r, enc, per.Bounds{LB: 0, HasLB: true, UB: 127, HasUB: true})
	if err != nil {
		return err
	}

	*v = ThresholdRSRP(n)

	return nil
}

type ThresholdRSRQ uint8

func (v ThresholdRSRQ) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return per.EncodeInteger(w, enc, per.Bounds{LB: 0, HasLB: true, UB: 127, HasUB: true}, int64(v))
}

func (v *ThresholdRSRQ) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	n, err := per.DecodeInteger(r, enc, per.Bounds{LB: 0, HasLB: true, UB: 127, HasUB: true})
	if err != nil {
		return err
	}

	*v = ThresholdRSRQ(n)

	return nil
}

type ThresholdSINR uint8

func (v ThresholdSINR) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return per.EncodeInteger(w, enc, per.Bounds{LB: 0, HasLB: true, UB: 127, HasUB: true}, int64(v))
}

func (v *ThresholdSINR) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	n, err := per.DecodeInteger(r, enc, per.Bounds{LB: 0, HasLB: true, UB: 127, HasUB: true})
	if err != nil {
		return err
	}

	*v = ThresholdSINR(n)

	return nil
}

type ExcessPacketDelayThresholdValue uint8

const (
	ExcessPacketDelayThresholdValueMs0dot25 ExcessPacketDelayThresholdValue = iota
	ExcessPacketDelayThresholdValueMs0dot5
	ExcessPacketDelayThresholdValueMs1
	ExcessPacketDelayThresholdValueMs2
	ExcessPacketDelayThresholdValueMs4
	ExcessPacketDelayThresholdValueMs5
	ExcessPacketDelayThresholdValueMs10
	ExcessPacketDelayThresholdValueMs20
	ExcessPacketDelayThresholdValueMs30
	ExcessPacketDelayThresholdValueMs40
	ExcessPacketDelayThresholdValueMs50
	ExcessPacketDelayThresholdValueMs60
	ExcessPacketDelayThresholdValueMs70
	ExcessPacketDelayThresholdValueMs80
	ExcessPacketDelayThresholdValueMs90
	ExcessPacketDelayThresholdValueMs100
	ExcessPacketDelayThresholdValueMs150
	ExcessPacketDelayThresholdValueMs300
	ExcessPacketDelayThresholdValueMs500
)

type BluetoothName []byte

func (v BluetoothName) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return per.EncodeOctetString(w, enc, 1, 248, true, true, false, v)
}

func (v *BluetoothName) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	b, err := per.DecodeOctetString(r, enc, 1, 248, true, true, false)
	if err != nil {
		return err
	}

	*v = b

	return nil
}

type WLANName []byte

func (v WLANName) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return per.EncodeOctetString(w, enc, 1, 32, true, true, false, v)
}

func (v *WLANName) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	b, err := per.DecodeOctetString(r, enc, 1, 32, true, true, false)
	if err != nil {
		return err
	}

	*v = b

	return nil
}

type SensorNameConfigUncompensatedBarometricConfig uint8

const (
	SensorNameConfigUncompensatedBarometricConfigTrue SensorNameConfigUncompensatedBarometricConfig = iota
)

type SensorNameConfigUeSpeedConfig uint8

const (
	SensorNameConfigUeSpeedConfigTrue SensorNameConfigUeSpeedConfig = iota
)

type SensorNameConfigUeOrientationConfig uint8

const (
	SensorNameConfigUeOrientationConfigTrue SensorNameConfigUeOrientationConfig = iota
)

type SensorNameConfig struct {
	UncompensatedBarometricConfig *SensorNameConfigUncompensatedBarometricConfig `per:",choice:0"`
	UeSpeedConfig                 *SensorNameConfigUeSpeedConfig                 `per:",choice:1"`
	UeOrientationConfig           *SensorNameConfigUeOrientationConfig           `per:",choice:2"`
	ChoiceExtension               *SingleContainer                               `per:",choice:3,extie"`
}

type MeasurementThresholdL1LoggedMDT struct {
	ThresholdRSRP   *ThresholdRSRP   `per:",choice:0"`
	ThresholdRSRQ   *ThresholdRSRQ   `per:",choice:1"`
	ChoiceExtension *SingleContainer `per:",choice:2,extie"`
}

type Hysteresis uint8

func (v Hysteresis) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return per.EncodeInteger(w, enc, per.Bounds{LB: 0, HasLB: true, UB: 30, HasUB: true}, int64(v))
}

func (v *Hysteresis) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	n, err := per.DecodeInteger(r, enc, per.Bounds{LB: 0, HasLB: true, UB: 30, HasUB: true})
	if err != nil {
		return err
	}

	*v = Hysteresis(n)

	return nil
}

type TimeToTrigger uint8

const (
	TimeToTriggerMs0 TimeToTrigger = iota
	TimeToTriggerMs40
	TimeToTriggerMs64
	TimeToTriggerMs80
	TimeToTriggerMs100
	TimeToTriggerMs128
	TimeToTriggerMs160
	TimeToTriggerMs256
	TimeToTriggerMs320
	TimeToTriggerMs480
	TimeToTriggerMs512
	TimeToTriggerMs640
	TimeToTriggerMs1024
	TimeToTriggerMs1280
	TimeToTriggerMs2560
	TimeToTriggerMs5120
)

type NRARFCN uint32

func (v NRARFCN) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return per.EncodeInteger(w, enc, per.Bounds{LB: 0, HasLB: true, UB: maxNRARFCN, HasUB: true}, int64(v))
}

func (v *NRARFCN) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	n, err := per.DecodeInteger(r, enc, per.Bounds{LB: 0, HasLB: true, UB: maxNRARFCN, HasUB: true})
	if err != nil {
		return err
	}

	*v = NRARFCN(n)

	return nil
}

type NRFrequencyBandList []NRFrequencyBandItem

func (l NRFrequencyBandList) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return marshalSeqOf(w, enc, 1, maxnoofNRCellBands, []NRFrequencyBandItem(l))
}

func (l *NRFrequencyBandList) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	items, err := unmarshalSeqOf[NRFrequencyBandItem](r, enc, 1, maxnoofNRCellBands)
	if err != nil {
		return err
	}

	*l = items

	return nil
}

type NRPCI int64

func (v NRPCI) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return per.EncodeInteger(w, enc, per.Bounds{LB: 0, HasLB: true, UB: 1007, HasUB: true, Extensible: true}, int64(v))
}

func (v *NRPCI) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	n, err := per.DecodeInteger(r, enc, per.Bounds{LB: 0, HasLB: true, UB: 1007, HasUB: true, Extensible: true})
	if err != nil {
		return err
	}

	*v = NRPCI(n)

	return nil
}

func (v NRPCI) Diagnose(*ap.Diagnosis[ProtocolIEID]) bool { return outsideRoot(int64(v), 0, 1007) }

type NRFrequencyBandItem struct {
	Nrfrequencyband NRFrequencyBand
	iEExtensions    extensionMeta          `per:",exttable"`
	extensions      per.ExtensionAdditions `per:",extraw"`
}

type NRFrequencyBand int64

func (v NRFrequencyBand) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return per.EncodeInteger(w, enc, per.Bounds{LB: 1, HasLB: true, UB: 1024, HasUB: true, Extensible: true}, int64(v))
}

func (v *NRFrequencyBand) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	n, err := per.DecodeInteger(r, enc, per.Bounds{LB: 1, HasLB: true, UB: 1024, HasUB: true, Extensible: true})
	if err != nil {
		return err
	}

	*v = NRFrequencyBand(n)

	return nil
}

func (v NRFrequencyBand) Diagnose(*ap.Diagnosis[ProtocolIEID]) bool {
	return outsideRoot(int64(v), 1, 1024)
}

type ENDCSONConfigurationTransfer []byte

func (v ENDCSONConfigurationTransfer) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return per.EncodeOctetString(w, enc, 0, 0, true, false, false, v)
}

func (v *ENDCSONConfigurationTransfer) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	b, err := per.DecodeOctetString(r, enc, 0, 0, true, false, false)
	if err != nil {
		return err
	}

	*v = b

	return nil
}

type IntersystemSONConfigurationTransfer struct {
	TransferType              IntersystemSONTransferType
	IntersystemSONInformation IntersystemSONInformation
	iEExtensions              extensionMeta          `per:",exttable"`
	extensions                per.ExtensionAdditions `per:",extraw"`
}

type CoreNetworkAssistanceInformationForInactive struct {
	UEIdentityIndexValue                 UEIdentityIndexValue
	UESpecificDRX                        *PagingDRX `per:",optional"`
	PeriodicRegistrationUpdateTimer      PeriodicRegistrationUpdateTimer
	MICOModeIndication                   *MICOModeIndication `per:",optional"`
	TAIListForInactive                   TAIListForInactive
	ExpectedUEBehaviour                  *ExpectedUEBehaviour                  `per:",optional"`
	EUTRAPagingeDRXInformation           *EUTRAPagingeDRXInformation           `per:",extie:IDEUTRAPagingeDRXInformation,crit:ignore"`
	ExtendedUEIdentityIndexValue         *ExtendedUEIdentityIndexValue         `per:",extie:IDExtendedUEIdentityIndexValue,crit:ignore"`
	UERadioCapabilityForPaging           *UERadioCapabilityForPaging           `per:",extie:IDUERadioCapabilityForPaging,crit:ignore"`
	MicoAllPLMN                          *MicoAllPLMN                          `per:",extie:IDMicoAllPLMN,crit:ignore"`
	NRPagingeDRXInformation              *NRPagingeDRXInformation              `per:",extie:IDNRPagingeDRXInformation,crit:ignore"`
	PagingCauseIndicationForVoiceService *PagingCauseIndicationForVoiceService `per:",extie:IDPagingCauseIndicationForVoiceService,crit:ignore"`
	PEIPSassistanceInformation           *PEIPSassistanceInformation           `per:",extie:IDPEIPSassistanceInformation,crit:ignore"`
	HashedUEIdentityIndexValue           *HashedUEIdentityIndexValue           `per:",extie:IDHashedUEIdentityIndexValue,crit:ignore"`
	CNMTCommunicationHandling            *CNMTCommunicationHandling            `per:",extie:IDCNMTCommunicationHandling,crit:ignore"`
	LPWUSPSAssistanceInformation         *LPWUSPSAssistanceInformation         `per:",extie:IDLPWUSPSAssistanceInformation,crit:ignore"`
	FurtherExtendedUEIdentityIndexValue  *FurtherExtendedUEIdentityIndexValue  `per:",extie:IDFurtherExtendedUEIdentityIndexValue,crit:ignore"`
	LPWUSDisableIndication               *LPWUSDisableIndication               `per:",extie:IDLPWUSDisableIndication,crit:ignore"`
	iEExtensions                         extensionMeta                         `per:",exttable"`
	extensions                           per.ExtensionAdditions                `per:",extraw"`
}

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

type RRCInactiveTransitionReportRequest uint8

const (
	RRCInactiveTransitionReportRequestSubsequentStateTransitionReport RRCInactiveTransitionReportRequest = iota
	RRCInactiveTransitionReportRequestSingleRRCConnectedStateReport
	RRCInactiveTransitionReportRequestCancelReport
)

type RedirectionVoiceFallback uint8

const (
	RedirectionVoiceFallbackPossible RedirectionVoiceFallback = iota
	RedirectionVoiceFallbackNotPossible
)

type CNAssistedRANTuning struct {
	ExpectedUEBehaviour *ExpectedUEBehaviour   `per:",optional"`
	iEExtensions        extensionMeta          `per:",exttable"`
	extensions          per.ExtensionAdditions `per:",extraw"`
}

type SRVCCOperationPossible uint8

const (
	SRVCCOperationPossiblePossible SRVCCOperationPossible = iota
	SRVCCOperationPossibleNotPossible
)

type IABAuthorized uint8

const (
	IABAuthorizedAuthorized IABAuthorized = iota
	IABAuthorizedNotAuthorized
)

type EnhancedCoverageRestriction uint8

const (
	EnhancedCoverageRestrictionRestricted EnhancedCoverageRestriction = iota
)

type UEDifferentiationInfoPeriodicCommunicationIndicator uint8

const (
	UEDifferentiationInfoPeriodicCommunicationIndicatorPeriodically UEDifferentiationInfoPeriodicCommunicationIndicator = iota
	UEDifferentiationInfoPeriodicCommunicationIndicatorOndemand
)

type UEDifferentiationInfoPeriodicTime int64

func (v UEDifferentiationInfoPeriodicTime) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return per.EncodeInteger(w, enc, per.Bounds{LB: 1, HasLB: true, UB: 3600, HasUB: true, Extensible: true}, int64(v))
}

func (v *UEDifferentiationInfoPeriodicTime) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	n, err := per.DecodeInteger(r, enc, per.Bounds{LB: 1, HasLB: true, UB: 3600, HasUB: true, Extensible: true})
	if err != nil {
		return err
	}

	*v = UEDifferentiationInfoPeriodicTime(n)

	return nil
}

func (v UEDifferentiationInfoPeriodicTime) Diagnose(*ap.Diagnosis[ProtocolIEID]) bool {
	return outsideRoot(int64(v), 1, 3600)
}

type UEDifferentiationInfoStationaryIndication uint8

const (
	UEDifferentiationInfoStationaryIndicationStationary UEDifferentiationInfoStationaryIndication = iota
	UEDifferentiationInfoStationaryIndicationMobile
)

type UEDifferentiationInfoTrafficProfile uint8

const (
	UEDifferentiationInfoTrafficProfileSinglePacket UEDifferentiationInfoTrafficProfile = iota
	UEDifferentiationInfoTrafficProfileDualPackets
	UEDifferentiationInfoTrafficProfileMultiplePackets
)

type UEDifferentiationInfoBatteryIndication uint8

const (
	UEDifferentiationInfoBatteryIndicationBatteryPowered UEDifferentiationInfoBatteryIndication = iota
	UEDifferentiationInfoBatteryIndicationBatteryPoweredNotRechargeableOrReplaceable
	UEDifferentiationInfoBatteryIndicationNotBatteryPowered
)

type UEDifferentiationInfo struct {
	PeriodicCommunicationIndicator *UEDifferentiationInfoPeriodicCommunicationIndicator `per:",optional"`
	PeriodicTime                   *UEDifferentiationInfoPeriodicTime                   `per:",optional"`
	ScheduledCommunicationTime     *ScheduledCommunicationTime                          `per:",optional"`
	StationaryIndication           *UEDifferentiationInfoStationaryIndication           `per:",optional"`
	TrafficProfile                 *UEDifferentiationInfoTrafficProfile                 `per:",optional"`
	BatteryIndication              *UEDifferentiationInfoBatteryIndication              `per:",optional"`
	iEExtensions                   extensionMeta                                        `per:",exttable"`
	extensions                     per.ExtensionAdditions                               `per:",extraw"`
}

type NRV2XServicesAuthorized struct {
	VehicleUE    *VehicleUE             `per:",optional"`
	PedestrianUE *PedestrianUE          `per:",optional"`
	iEExtensions extensionMeta          `per:",exttable"`
	extensions   per.ExtensionAdditions `per:",extraw"`
}

type LTEV2XServicesAuthorized struct {
	VehicleUE    *VehicleUE             `per:",optional"`
	PedestrianUE *PedestrianUE          `per:",optional"`
	iEExtensions extensionMeta          `per:",exttable"`
	extensions   per.ExtensionAdditions `per:",extraw"`
}

type NRUESidelinkAggregateMaximumBitrate struct {
	UESidelinkAggregateMaximumBitRate BitRate
	iEExtensions                      extensionMeta          `per:",exttable"`
	extensions                        per.ExtensionAdditions `per:",extraw"`
}

type LTEUESidelinkAggregateMaximumBitrate struct {
	UESidelinkAggregateMaximumBitRate BitRate
	iEExtensions                      extensionMeta          `per:",exttable"`
	extensions                        per.ExtensionAdditions `per:",extraw"`
}

type PC5QoSParameters struct {
	Pc5QoSFlowList           PC5QoSFlowList
	Pc5LinkAggregateBitRates *BitRate               `per:",optional"`
	iEExtensions             extensionMeta          `per:",exttable"`
	extensions               per.ExtensionAdditions `per:",extraw"`
}

type CEmodeBrestricted uint8

const (
	CEmodeBrestrictedRestricted CEmodeBrestricted = iota
	CEmodeBrestrictedNotRestricted
)

type UEUPCIoTSupport uint8

const (
	UEUPCIoTSupportSupported UEUPCIoTSupport = iota
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

type ExtendedConnectedTime uint8

func (v ExtendedConnectedTime) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return per.EncodeInteger(w, enc, per.Bounds{LB: 0, HasLB: true, UB: 255, HasUB: true}, int64(v))
}

func (v *ExtendedConnectedTime) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	n, err := per.DecodeInteger(r, enc, per.Bounds{LB: 0, HasLB: true, UB: 255, HasUB: true})
	if err != nil {
		return err
	}

	*v = ExtendedConnectedTime(n)

	return nil
}

type TimeSyncAssistanceInfoTimeDistributionIndication uint8

const (
	TimeSyncAssistanceInfoTimeDistributionIndicationEnabled TimeSyncAssistanceInfoTimeDistributionIndication = iota
	TimeSyncAssistanceInfoTimeDistributionIndicationDisabled
)

type TimeSyncAssistanceInfoUUTimeSyncErrorBudget int64

func (v TimeSyncAssistanceInfoUUTimeSyncErrorBudget) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return per.EncodeInteger(w, enc, per.Bounds{LB: 1, HasLB: true, UB: 1000000, HasUB: true, Extensible: true}, int64(v))
}

func (v *TimeSyncAssistanceInfoUUTimeSyncErrorBudget) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	n, err := per.DecodeInteger(r, enc, per.Bounds{LB: 1, HasLB: true, UB: 1000000, HasUB: true, Extensible: true})
	if err != nil {
		return err
	}

	*v = TimeSyncAssistanceInfoUUTimeSyncErrorBudget(n)

	return nil
}

func (v TimeSyncAssistanceInfoUUTimeSyncErrorBudget) Diagnose(*ap.Diagnosis[ProtocolIEID]) bool {
	return outsideRoot(int64(v), 1, 1000000)
}

type TimeSyncAssistanceInfo struct {
	TimeDistributionIndication       TimeSyncAssistanceInfoTimeDistributionIndication
	UUTimeSyncErrorBudget            *TimeSyncAssistanceInfoUUTimeSyncErrorBudget `per:",optional"`
	ClockQualityReportingControlInfo *ClockQualityReportingControlInfo            `per:",extie:IDClockQualityReportingControlInfo,crit:ignore"`
	iEExtensions                     extensionMeta                                `per:",exttable"`
	extensions                       per.ExtensionAdditions                       `per:",extraw"`
}

type UESliceMaximumBitRateList []UESliceMaximumBitRateItem

func (l UESliceMaximumBitRateList) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return marshalSeqOf(w, enc, 1, maxnoofAllowedSNSSAIs, []UESliceMaximumBitRateItem(l))
}

func (l *UESliceMaximumBitRateList) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	items, err := unmarshalSeqOf[UESliceMaximumBitRateItem](r, enc, 1, maxnoofAllowedSNSSAIs)
	if err != nil {
		return err
	}

	*l = items

	return nil
}

type FiveGProSeAuthorized struct {
	FiveGProSeDirectDiscovery                      *FiveGProSeDirectDiscovery                      `per:",optional"`
	FiveGProSeDirectCommunication                  *FiveGProSeDirectCommunication                  `per:",optional"`
	FiveGProSeLayer2UEtoNetworkRelay               *FiveGProSeLayer2UEtoNetworkRelay               `per:",optional"`
	FiveGProSeLayer3UEtoNetworkRelay               *FiveGProSeLayer3UEtoNetworkRelay               `per:",optional"`
	FiveGProSeLayer2RemoteUE                       *FiveGProSeLayer2RemoteUE                       `per:",optional"`
	FiveGProSeLayer2Multipath                      *FiveGProSeLayer2Multipath                      `per:",extie:IDFiveGProSeLayer2Multipath,crit:ignore"`
	FiveGProSeLayer2UEtoUERelay                    *FiveGProSeLayer2UEtoUERelay                    `per:",extie:IDFiveGProSeLayer2UEtoUERelay,crit:ignore"`
	FiveGProSeLayer2UEtoUERemote                   *FiveGProSeLayer2UEtoUERemote                   `per:",extie:IDFiveGProSeLayer2UEtoUERemote,crit:ignore"`
	FiveGProSeLayer2MHUEtoNetworkRelay             *FiveGProSeLayer2MHUEtoNetworkRelay             `per:",extie:IDFiveGProSeLayer2MHUEtoNetworkRelay,crit:ignore"`
	FiveGProSeLayer2MHIntermediateUEtoNetworkRelay *FiveGProSeLayer2MHIntermediateUEtoNetworkRelay `per:",extie:IDFiveGProSeLayer2MHIntermediateUEtoNetworkRelay,crit:ignore"`
	FiveGProSeLayer2MHRemote                       *FiveGProSeLayer2MHRemote                       `per:",extie:IDFiveGProSeLayer2MHRemote,crit:ignore"`
	FiveGProSeLayer3MHUEtoNetworkRelay             *FiveGProSeLayer3MHUEtoNetworkRelay             `per:",extie:IDFiveGProSeLayer3MHUEtoNetworkRelay,crit:ignore"`
	iEExtensions                                   extensionMeta                                   `per:",exttable"`
	extensions                                     per.ExtensionAdditions                          `per:",extraw"`
}

type FiveGProSePC5QoSParameters struct {
	FiveGProSepc5QoSFlowList           FiveGProSePC5QoSFlowList
	FiveGProSepc5LinkAggregateBitRates *BitRate               `per:",optional"`
	iEExtensions                       extensionMeta          `per:",exttable"`
	extensions                         per.ExtensionAdditions `per:",extraw"`
}

type AerialUEsubscriptionInformation uint8

const (
	AerialUEsubscriptionInformationAllowed AerialUEsubscriptionInformation = iota
	AerialUEsubscriptionInformationNotAllowed
)

type NRA2XServicesAuthorizedAerialUE uint8

const (
	NRA2XServicesAuthorizedAerialUEAuthorized NRA2XServicesAuthorizedAerialUE = iota
	NRA2XServicesAuthorizedAerialUENotAuthorized
)

type NRA2XServicesAuthorizedAerialControllerUE uint8

const (
	NRA2XServicesAuthorizedAerialControllerUEAuthorized NRA2XServicesAuthorizedAerialControllerUE = iota
	NRA2XServicesAuthorizedAerialControllerUENotAuthorized
)

type NRA2XServicesAuthorized struct {
	AerialUE           *NRA2XServicesAuthorizedAerialUE           `per:",optional"`
	AerialControllerUE *NRA2XServicesAuthorizedAerialControllerUE `per:",optional"`
	iEExtensions       extensionMeta                              `per:",exttable"`
	extensions         per.ExtensionAdditions                     `per:",extraw"`
}

type LTEA2XServicesAuthorizedAerialUE uint8

const (
	LTEA2XServicesAuthorizedAerialUEAuthorized LTEA2XServicesAuthorizedAerialUE = iota
	LTEA2XServicesAuthorizedAerialUENotAuthorized
)

type LTEA2XServicesAuthorizedAerialControllerUE uint8

const (
	LTEA2XServicesAuthorizedAerialControllerUEAuthorized LTEA2XServicesAuthorizedAerialControllerUE = iota
	LTEA2XServicesAuthorizedAerialControllerUENotAuthorized
)

type LTEA2XServicesAuthorized struct {
	AerialUE           *LTEA2XServicesAuthorizedAerialUE           `per:",optional"`
	AerialControllerUE *LTEA2XServicesAuthorizedAerialControllerUE `per:",optional"`
	iEExtensions       extensionMeta                               `per:",exttable"`
	extensions         per.ExtensionAdditions                      `per:",extraw"`
}

type A2XPC5QoSParameters struct {
	A2XPC5QoSFlowList           A2XPC5QoSFlowList
	A2XPC5LinkAggregateBitRates *BitRate               `per:",optional"`
	iEExtensions                extensionMeta          `per:",exttable"`
	extensions                  per.ExtensionAdditions `per:",extraw"`
}

type MobileIABAuthorized uint8

const (
	MobileIABAuthorizedAuthorized MobileIABAuthorized = iota
	MobileIABAuthorizedNotAuthorized
)

type NoPDUSessionIndication uint8

const (
	NoPDUSessionIndicationTrue NoPDUSessionIndication = iota
)

type SLPositioningRangingServiceInfo struct {
	SLPositioningRangingAuthorized    SLPositioningRangingAuthorized
	SLPositioningRangingQoSParameters *SLPositioningRangingQoSParameters `per:",optional"`
	iEExtensions                      extensionMeta                      `per:",exttable"`
	extensions                        per.ExtensionAdditions             `per:",extraw"`
}

type NPNAccessInformation struct {
	PNINPNAccessInformation *CellCAGList     `per:",choice:0"`
	ChoiceExtension         *SingleContainer `per:",choice:1,extie"`
}

type RedCapIndication uint8

const (
	RedCapIndicationRedcap RedCapIndication = iota
)

type ERedCapIndication uint8

const (
	ERedCapIndicationTrue ERedCapIndication = iota
)

type EmergencyFallbackIndicator struct {
	EmergencyFallbackRequestIndicator EmergencyFallbackRequestIndicator
	EmergencyServiceTargetCN          *EmergencyServiceTargetCN `per:",optional"`
	iEExtensions                      extensionMeta             `per:",exttable"`
	extensions                        per.ExtensionAdditions    `per:",extraw"`
}

type RGLevelWirelineAccessCharacteristics []byte

func (v RGLevelWirelineAccessCharacteristics) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return per.EncodeOctetString(w, enc, 0, 0, true, false, false, v)
}

func (v *RGLevelWirelineAccessCharacteristics) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	b, err := per.DecodeOctetString(r, enc, 0, 0, true, false, false)
	if err != nil {
		return err
	}

	*v = b

	return nil
}

type TargetNSSAIInformation struct {
	TargetNSSAI  TargetNSSAI
	IndexToRFSP  IndexToRFSP
	iEExtensions extensionMeta          `per:",exttable"`
	extensions   per.ExtensionAdditions `per:",extraw"`
}

type NetworkControlledRepeaterAuthorized uint8

const (
	NetworkControlledRepeaterAuthorizedAuthorized NetworkControlledRepeaterAuthorized = iota
	NetworkControlledRepeaterAuthorizedNotAuthorized
)

type IABNodeIndication uint8

const (
	IABNodeIndicationTrue IABNodeIndication = iota
)

type CEmodeBSupportIndicator uint8

const (
	CEmodeBSupportIndicatorSupported CEmodeBSupportIndicator = iota
)

type LTEMIndication uint8

const (
	LTEMIndicationLteM LTEMIndication = iota
)

type EDTSession uint8

const (
	EDTSessionTrue EDTSession = iota
)

type AuthenticatedIndication uint8

const (
	AuthenticatedIndicationTrue AuthenticatedIndication = iota
)

type MobileIABNodeIndication uint8

const (
	MobileIABNodeIndicationTrue MobileIABNodeIndication = iota
)

type AUN3DeviceAccessInfoAUN3DeviceAccess uint8

const (
	AUN3DeviceAccessInfoAUN3DeviceAccessTrue AUN3DeviceAccessInfoAUN3DeviceAccess = iota
)

type AUN3DeviceAccessInfoExistingNGConnectionwithSameLocation uint8

const (
	AUN3DeviceAccessInfoExistingNGConnectionwithSameLocationTrue AUN3DeviceAccessInfoExistingNGConnectionwithSameLocation = iota
	AUN3DeviceAccessInfoExistingNGConnectionwithSameLocationFalse
)

type AUN3DeviceAccessInfo struct {
	AUN3DeviceAccess                     AUN3DeviceAccessInfoAUN3DeviceAccess
	ExistingNGConnectionwithSameLocation AUN3DeviceAccessInfoExistingNGConnectionwithSameLocation
	iEExtensions                         extensionMeta          `per:",exttable"`
	extensions                           per.ExtensionAdditions `per:",extraw"`
}

type RANPagingPriority uint16

func (v RANPagingPriority) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return per.EncodeInteger(w, enc, per.Bounds{LB: 1, HasLB: true, UB: 256, HasUB: true}, int64(v))
}

func (v *RANPagingPriority) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	n, err := per.DecodeInteger(r, enc, per.Bounds{LB: 1, HasLB: true, UB: 256, HasUB: true})
	if err != nil {
		return err
	}

	*v = RANPagingPriority(n)

	return nil
}

type UECapabilityInfoRequest uint8

const (
	UECapabilityInfoRequestRequested UECapabilityInfoRequest = iota
)

type EndIndication uint8

const (
	EndIndicationNoFurtherData EndIndication = iota
	EndIndicationFurtherDataExists
)

type NBIoTDefaultPagingDRX uint8

const (
	NBIoTDefaultPagingDRXRf128 NBIoTDefaultPagingDRX = iota
	NBIoTDefaultPagingDRXRf256
	NBIoTDefaultPagingDRXRf512
	NBIoTDefaultPagingDRXRf1024
)

type ExtendedRANNodeName struct {
	RANNodeNameVisibleString *RANNodeNameVisibleString `per:",optional"`
	RANNodeNameUTF8String    *RANNodeNameUTF8String    `per:",optional"`
	iEExtensions             extensionMeta             `per:",exttable"`
	extensions               per.ExtensionAdditions    `per:",extraw"`
}

type AIoTSupport uint8

const (
	AIoTSupportAIoTOnly AIoTSupport = iota
	AIoTSupportAIoTAndNRUu
)

type IABSupported uint8

const (
	IABSupportedTrue IABSupported = iota
)

type MobileIABSupported uint8

const (
	MobileIABSupportedTrue MobileIABSupported = iota
)

type AIOTFIdentifier []byte

func (v AIOTFIdentifier) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return per.EncodeOctetString(w, enc, 16, 36, true, true, true, v)
}

func (v *AIOTFIdentifier) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	b, err := per.DecodeOctetString(r, enc, 16, 36, true, true, true)
	if err != nil {
		return err
	}

	*v = b

	return nil
}

type AIOTFName struct {
	AIOTFNameVisibleString *AIOTFNameVisibleString `per:",optional"`
	AIOTFNameUTF8String    *AIOTFNameUTF8String    `per:",optional"`
	iEExtensions           extensionMeta           `per:",exttable"`
	extensions             per.ExtensionAdditions  `per:",extraw"`
}

type AssistanceDataForPaging struct {
	AssistanceDataForRecommendedCells *AssistanceDataForRecommendedCells `per:",optional"`
	PagingAttemptInformation          *PagingAttemptInformation          `per:",optional"`
	NPNPagingAssistanceInformation    *NPNPagingAssistanceInformation    `per:",extie:IDNPNPagingAssistanceInformation,crit:ignore"`
	PagingAssisDataforCEcapabUE       *PagingAssisDataforCEcapabUE       `per:",extie:IDPagingAssisDataforCEcapabUE,crit:ignore"`
	iEExtensions                      extensionMeta                      `per:",exttable"`
	extensions                        per.ExtensionAdditions             `per:",extraw"`
}

type NBIoTPagingeDRXInfo struct {
	NBIoTPagingeDRXCycle  NBIoTPagingeDRXCycle
	NBIoTPagingTimeWindow *NBIoTPagingTimeWindow `per:",optional"`
	iEExtensions          extensionMeta          `per:",exttable"`
	extensions            per.ExtensionAdditions `per:",extraw"`
}

type NBIoTPagingDRX uint8

const (
	NBIoTPagingDRXRf32 NBIoTPagingDRX = iota
	NBIoTPagingDRXRf64
	NBIoTPagingDRXRf128
	NBIoTPagingDRXRf256
	NBIoTPagingDRXRf512
	NBIoTPagingDRXRf1024
)

type WUSAssistanceInformation struct {
	PagingProbabilityInformation PagingProbabilityInformation
	iEExtensions                 extensionMeta          `per:",exttable"`
	extensions                   per.ExtensionAdditions `per:",extraw"`
}

type EUTRAPagingeDRXInformation struct {
	EUTRApagingeDRXCycle  EUTRAPagingeDRXCycle
	EUTRApagingTimeWindow *EUTRAPagingTimeWindow `per:",optional"`
	iEExtensions          extensionMeta          `per:",exttable"`
	extensions            per.ExtensionAdditions `per:",extraw"`
}

type NRPagingeDRXInformation struct {
	NRpagingeDRXCycle  NRPagingeDRXCycle
	NRpagingTimeWindow *NRPagingTimeWindow    `per:",optional"`
	iEExtensions       extensionMeta          `per:",exttable"`
	extensions         per.ExtensionAdditions `per:",extraw"`
}

type PagingCause uint8

const (
	PagingCauseVoice PagingCause = iota
)

type PEIPSassistanceInformation struct {
	CNsubgroupID CNsubgroupID
	iEExtensions extensionMeta          `per:",exttable"`
	extensions   per.ExtensionAdditions `per:",extraw"`
}

type LPWUSPSAssistanceInformation struct {
	LPWUScNsubgroupID LPWUSCNsubgroupID
	iEExtensions      extensionMeta          `per:",exttable"`
	extensions        per.ExtensionAdditions `per:",extraw"`
}

type LPWUSDisableIndication uint8

const (
	LPWUSDisableIndicationTrue LPWUSDisableIndication = iota
)

type MDTPLMNModificationList []PLMNIdentity

func (l MDTPLMNModificationList) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return marshalSeqOf(w, enc, 0, maxnoofMDTPLMNs, []PLMNIdentity(l))
}

func (l *MDTPLMNModificationList) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	items, err := unmarshalSeqOf[PLMNIdentity](r, enc, 0, maxnoofMDTPLMNs)
	if err != nil {
		return err
	}

	*l = items

	return nil
}

type MBSSessionSetuporModifyRequestList []MBSSessionSetuporModifyRequestItem

func (l MBSSessionSetuporModifyRequestList) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return marshalSeqOf(w, enc, 1, maxnoofMBSSessions, []MBSSessionSetuporModifyRequestItem(l))
}

func (l *MBSSessionSetuporModifyRequestList) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	items, err := unmarshalSeqOf[MBSSessionSetuporModifyRequestItem](r, enc, 1, maxnoofMBSSessions)
	if err != nil {
		return err
	}

	*l = items

	return nil
}

type MBSSessionToReleaseList []MBSSessionToReleaseItem

func (l MBSSessionToReleaseList) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return marshalSeqOf(w, enc, 1, maxnoofMBSSessions, []MBSSessionToReleaseItem(l))
}

func (l *MBSSessionToReleaseList) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	items, err := unmarshalSeqOf[MBSSessionToReleaseItem](r, enc, 1, maxnoofMBSSessions)
	if err != nil {
		return err
	}

	*l = items

	return nil
}

type MBSSessionSetupRequestList []MBSSessionSetupRequestItem

func (l MBSSessionSetupRequestList) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return marshalSeqOf(w, enc, 1, maxnoofMBSSessions, []MBSSessionSetupRequestItem(l))
}

func (l *MBSSessionSetupRequestList) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	items, err := unmarshalSeqOf[MBSSessionSetupRequestItem](r, enc, 1, maxnoofMBSSessions)
	if err != nil {
		return err
	}

	*l = items

	return nil
}

type QMCDeactivation struct {
	QoEReferenceList QoEReferenceList
	iEExtensions     extensionMeta          `per:",exttable"`
	extensions       per.ExtensionAdditions `per:",extraw"`
}

type RRCState uint8

const (
	RRCStateInactive RRCState = iota
	RRCStateConnected
)

type InfoOnRecommendedCellsAndRANNodesForPaging struct {
	RecommendedCellsForPaging  RecommendedCellsForPaging
	RecommendRANNodesForPaging RecommendedRANNodesForPaging
	iEExtensions               extensionMeta          `per:",exttable"`
	extensions                 per.ExtensionAdditions `per:",extraw"`
}

type PagingAssisDataforCEcapabUE struct {
	EUTRACGI                 EUTRACGI
	CoverageEnhancementLevel CoverageEnhancementLevel
	iEExtensions             extensionMeta          `per:",exttable"`
	extensions               per.ExtensionAdditions `per:",extraw"`
}

type XrDeviceWith2Rx uint8

const (
	XrDeviceWith2RxTrue XrDeviceWith2Rx = iota
)

type IntersystemSONTransferType struct {
	FromEUTRANtoNGRAN *FromEUTRANtoNGRAN `per:",choice:0"`
	FromNGRANtoEUTRAN *FromNGRANtoEUTRAN `per:",choice:1"`
	ChoiceExtension   *SingleContainer   `per:",choice:2,extie"`
}

type IntersystemSONInformation struct {
	IntersystemSONInformationReport  *IntersystemSONInformationReport  `per:",choice:0"`
	IntersystemSONInformationRequest *IntersystemSONInformationRequest `per:",choice:1,extie:IDIntersystemSONInformationRequest,crit:ignore"`
	IntersystemSONInformationReply   *IntersystemSONInformationReply   `per:",choice:1,extie:IDIntersystemSONInformationReply,crit:ignore"`
	ChoiceExtension                  *SingleContainer                  `per:",choice:1,extie"`
	choiceCriticality                apimpl.WireCriticality            `per:",choicecrit"`
}

type UEIdentityIndexValueIndexLength10 uint16

func (v UEIdentityIndexValueIndexLength10) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return encodeBitStringUint(w, enc, uint64(v), 10)
}

func (v *UEIdentityIndexValueIndexLength10) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	n, err := decodeBitStringUint(r, enc, 10)
	if err != nil {
		return err
	}

	*v = UEIdentityIndexValueIndexLength10(n)

	return nil
}

type UEIdentityIndexValue struct {
	IndexLength10   *UEIdentityIndexValueIndexLength10 `per:",choice:0"`
	ChoiceExtension *SingleContainer                   `per:",choice:1,extie"`
}

type PeriodicRegistrationUpdateTimer uint8

func (v PeriodicRegistrationUpdateTimer) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return encodeBitStringUint(w, enc, uint64(v), 8)
}

func (v *PeriodicRegistrationUpdateTimer) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	n, err := decodeBitStringUint(r, enc, 8)
	if err != nil {
		return err
	}

	*v = PeriodicRegistrationUpdateTimer(n)

	return nil
}

type MICOModeIndication uint8

const (
	MICOModeIndicationTrue MICOModeIndication = iota
)

type TAIListForInactive []TAIListForInactiveItem

func (l TAIListForInactive) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return marshalSeqOf(w, enc, 1, maxnoofTAIforInactive, []TAIListForInactiveItem(l))
}

func (l *TAIListForInactive) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	items, err := unmarshalSeqOf[TAIListForInactiveItem](r, enc, 1, maxnoofTAIforInactive)
	if err != nil {
		return err
	}

	*l = items

	return nil
}

type ExpectedUEBehaviour struct {
	ExpectedUEActivityBehaviour *ExpectedUEActivityBehaviour `per:",optional"`
	ExpectedHOInterval          *ExpectedHOInterval          `per:",optional"`
	ExpectedUEMobility          *ExpectedUEMobility          `per:",optional"`
	ExpectedUEMovingTrajectory  ExpectedUEMovingTrajectory   `per:",optional"`
	iEExtensions                extensionMeta                `per:",exttable"`
	extensions                  per.ExtensionAdditions       `per:",extraw"`
}

type ExtendedUEIdentityIndexValue uint16

func (v ExtendedUEIdentityIndexValue) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return encodeBitStringUint(w, enc, uint64(v), 16)
}

func (v *ExtendedUEIdentityIndexValue) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	n, err := decodeBitStringUint(r, enc, 16)
	if err != nil {
		return err
	}

	*v = ExtendedUEIdentityIndexValue(n)

	return nil
}

type MicoAllPLMN uint8

const (
	MicoAllPLMNTrue MicoAllPLMN = iota
)

type PagingCauseIndicationForVoiceService uint8

const (
	PagingCauseIndicationForVoiceServiceSupported PagingCauseIndicationForVoiceService = iota
)

type HashedUEIdentityIndexValue struct{ BitString }

func (v HashedUEIdentityIndexValue) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return v.encode(w, enc, 13, 13)
}

func (v *HashedUEIdentityIndexValue) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	b, err := decodeBitString(r, enc, 13, 13)
	if err != nil {
		return err
	}

	*v = HashedUEIdentityIndexValue{b}

	return nil
}

type CNMTCommunicationHandling uint8

const (
	CNMTCommunicationHandlingSupported CNMTCommunicationHandling = iota
)

type FurtherExtendedUEIdentityIndexValue uint32

func (v FurtherExtendedUEIdentityIndexValue) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return encodeBitStringUint(w, enc, uint64(v), 20)
}

func (v *FurtherExtendedUEIdentityIndexValue) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	n, err := decodeBitStringUint(r, enc, 20)
	if err != nil {
		return err
	}

	*v = FurtherExtendedUEIdentityIndexValue(n)

	return nil
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

func (v ScheduledCommunicationTimeTimeofDayStart) Diagnose(*ap.Diagnosis[ProtocolIEID]) bool {
	return outsideRoot(int64(v), 0, 86399)
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

func (v ScheduledCommunicationTimeTimeofDayEnd) Diagnose(*ap.Diagnosis[ProtocolIEID]) bool {
	return outsideRoot(int64(v), 0, 86399)
}

type ScheduledCommunicationTime struct {
	DayofWeek      *ScheduledCommunicationTimeDayofWeek      `per:",optional"`
	TimeofDayStart *ScheduledCommunicationTimeTimeofDayStart `per:",optional"`
	TimeofDayEnd   *ScheduledCommunicationTimeTimeofDayEnd   `per:",optional"`
	iEExtensions   extensionMeta                             `per:",exttable"`
	extensions     per.ExtensionAdditions                    `per:",extraw"`
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

type ClockQualityReportingControlInfo struct {
	ClockQualityDetailLevel ClockQualityDetailLevel
	iEExtensions            extensionMeta          `per:",exttable"`
	extensions              per.ExtensionAdditions `per:",extraw"`
}

type UESliceMaximumBitRateItem struct {
	SNSSAI                  SNSSAI
	UESliceMaximumBitRateDL BitRate
	UESliceMaximumBitRateUL BitRate
	iEExtensions            extensionMeta          `per:",exttable"`
	extensions              per.ExtensionAdditions `per:",extraw"`
}

type FiveGProSeDirectDiscovery uint8

const (
	FiveGProSeDirectDiscoveryAuthorized FiveGProSeDirectDiscovery = iota
	FiveGProSeDirectDiscoveryNotAuthorized
)

type FiveGProSeDirectCommunication uint8

const (
	FiveGProSeDirectCommunicationAuthorized FiveGProSeDirectCommunication = iota
	FiveGProSeDirectCommunicationNotAuthorized
)

type FiveGProSeLayer2UEtoNetworkRelay uint8

const (
	FiveGProSeLayer2UEtoNetworkRelayAuthorized FiveGProSeLayer2UEtoNetworkRelay = iota
	FiveGProSeLayer2UEtoNetworkRelayNotAuthorized
)

type FiveGProSeLayer3UEtoNetworkRelay uint8

const (
	FiveGProSeLayer3UEtoNetworkRelayAuthorized FiveGProSeLayer3UEtoNetworkRelay = iota
	FiveGProSeLayer3UEtoNetworkRelayNotAuthorized
)

type FiveGProSeLayer2RemoteUE uint8

const (
	FiveGProSeLayer2RemoteUEAuthorized FiveGProSeLayer2RemoteUE = iota
	FiveGProSeLayer2RemoteUENotAuthorized
)

type FiveGProSeLayer2Multipath uint8

const (
	FiveGProSeLayer2MultipathAuthorized FiveGProSeLayer2Multipath = iota
	FiveGProSeLayer2MultipathNotAuthorized
)

type FiveGProSeLayer2UEtoUERelay uint8

const (
	FiveGProSeLayer2UEtoUERelayAuthorized FiveGProSeLayer2UEtoUERelay = iota
	FiveGProSeLayer2UEtoUERelayNotAuthorized
)

type FiveGProSeLayer2UEtoUERemote uint8

const (
	FiveGProSeLayer2UEtoUERemoteAuthorized FiveGProSeLayer2UEtoUERemote = iota
	FiveGProSeLayer2UEtoUERemoteNotAuthorized
)

type FiveGProSeLayer2MHUEtoNetworkRelay uint8

const (
	FiveGProSeLayer2MHUEtoNetworkRelayAuthorized FiveGProSeLayer2MHUEtoNetworkRelay = iota
	FiveGProSeLayer2MHUEtoNetworkRelayNotAuthorized
)

type FiveGProSeLayer2MHIntermediateUEtoNetworkRelay uint8

const (
	FiveGProSeLayer2MHIntermediateUEtoNetworkRelayAuthorized FiveGProSeLayer2MHIntermediateUEtoNetworkRelay = iota
	FiveGProSeLayer2MHIntermediateUEtoNetworkRelayNotAuthorized
)

type FiveGProSeLayer2MHRemote uint8

const (
	FiveGProSeLayer2MHRemoteAuthorized FiveGProSeLayer2MHRemote = iota
	FiveGProSeLayer2MHRemoteNotAuthorized
)

type FiveGProSeLayer3MHUEtoNetworkRelay uint8

const (
	FiveGProSeLayer3MHUEtoNetworkRelayAuthorized FiveGProSeLayer3MHUEtoNetworkRelay = iota
	FiveGProSeLayer3MHUEtoNetworkRelayNotAuthorized
)

type FiveGProSePC5QoSFlowList []FiveGProSePC5QoSFlowItem

func (l FiveGProSePC5QoSFlowList) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return marshalSeqOf(w, enc, 1, maxnoofPC5QoSFlows, []FiveGProSePC5QoSFlowItem(l))
}

func (l *FiveGProSePC5QoSFlowList) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	items, err := unmarshalSeqOf[FiveGProSePC5QoSFlowItem](r, enc, 1, maxnoofPC5QoSFlows)
	if err != nil {
		return err
	}

	*l = items

	return nil
}

type A2XPC5QoSFlowList []A2XPC5QoSFlowItem

func (l A2XPC5QoSFlowList) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return marshalSeqOf(w, enc, 1, maxnoofPC5QoSFlows, []A2XPC5QoSFlowItem(l))
}

func (l *A2XPC5QoSFlowList) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	items, err := unmarshalSeqOf[A2XPC5QoSFlowItem](r, enc, 1, maxnoofPC5QoSFlows)
	if err != nil {
		return err
	}

	*l = items

	return nil
}

type SLPositioningRangingAuthorized uint8

const (
	SLPositioningRangingAuthorizedAuthorized SLPositioningRangingAuthorized = iota
	SLPositioningRangingAuthorizedNotAuthorized
)

type SLPositioningRangingQoSParameters struct {
	RSPPQoSFlowList           RSPPQoSFlowList
	RSPPLinkAggregateBitRates *BitRate               `per:",optional"`
	iEExtensions              extensionMeta          `per:",exttable"`
	extensions                per.ExtensionAdditions `per:",extraw"`
}

type CellCAGList []CAGID

func (l CellCAGList) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return marshalSeqOf(w, enc, 1, maxnoofCAGSperCell, []CAGID(l))
}

func (l *CellCAGList) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	items, err := unmarshalSeqOf[CAGID](r, enc, 1, maxnoofCAGSperCell)
	if err != nil {
		return err
	}

	*l = items

	return nil
}

type EmergencyFallbackRequestIndicator uint8

const (
	EmergencyFallbackRequestIndicatorEmergencyFallbackRequested EmergencyFallbackRequestIndicator = iota
)

type EmergencyServiceTargetCN uint8

const (
	EmergencyServiceTargetCNFiveGC EmergencyServiceTargetCN = iota
	EmergencyServiceTargetCNEPC
)

type TargetNSSAI []TargetNSSAIItem

func (l TargetNSSAI) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return marshalSeqOf(w, enc, 1, maxnoofTargetSNSSAIs, []TargetNSSAIItem(l))
}

func (l *TargetNSSAI) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	items, err := unmarshalSeqOf[TargetNSSAIItem](r, enc, 1, maxnoofTargetSNSSAIs)
	if err != nil {
		return err
	}

	*l = items

	return nil
}

type RANNodeNameVisibleString string

func (v RANNodeNameVisibleString) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return per.EncodeKnownMultiplierString(w, enc, per.CharVisibleString, 1, 150, true, true, true, string(v))
}

func (v *RANNodeNameVisibleString) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	b, err := per.DecodeKnownMultiplierString(r, enc, per.CharVisibleString, 1, 150, true, true, true)
	if err != nil {
		return err
	}

	*v = RANNodeNameVisibleString(b)

	return nil
}

type RANNodeNameUTF8String string

func (v RANNodeNameUTF8String) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return per.EncodeString(w, enc, []byte(v))
}

func (v *RANNodeNameUTF8String) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	b, err := per.DecodeString(r, enc)
	if err != nil {
		return err
	}

	*v = RANNodeNameUTF8String(string(b))

	return nil
}

type AIOTFNameVisibleString string

func (v AIOTFNameVisibleString) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return per.EncodeKnownMultiplierString(w, enc, per.CharVisibleString, 1, 150, true, true, true, string(v))
}

func (v *AIOTFNameVisibleString) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	b, err := per.DecodeKnownMultiplierString(r, enc, per.CharVisibleString, 1, 150, true, true, true)
	if err != nil {
		return err
	}

	*v = AIOTFNameVisibleString(b)

	return nil
}

type AIOTFNameUTF8String string

func (v AIOTFNameUTF8String) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return per.EncodeString(w, enc, []byte(v))
}

func (v *AIOTFNameUTF8String) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	b, err := per.DecodeString(r, enc)
	if err != nil {
		return err
	}

	*v = AIOTFNameUTF8String(string(b))

	return nil
}

type AssistanceDataForRecommendedCells struct {
	RecommendedCellsForPaging RecommendedCellsForPaging
	iEExtensions              extensionMeta          `per:",exttable"`
	extensions                per.ExtensionAdditions `per:",extraw"`
}

type PagingAttemptInformation struct {
	PagingAttemptCount             PagingAttemptCount
	IntendedNumberOfPagingAttempts IntendedNumberOfPagingAttempts
	NextPagingAreaScope            *NextPagingAreaScope   `per:",optional"`
	iEExtensions                   extensionMeta          `per:",exttable"`
	extensions                     per.ExtensionAdditions `per:",extraw"`
}

type NPNPagingAssistanceInformation struct {
	PNINPNPagingAssistance *AllowedPNINPNList `per:",choice:0"`
	ChoiceExtension        *SingleContainer   `per:",choice:1,extie"`
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

type EUTRAPagingeDRXCycle uint8

const (
	EUTRAPagingeDRXCycleHfhalf EUTRAPagingeDRXCycle = iota
	EUTRAPagingeDRXCycleHf1
	EUTRAPagingeDRXCycleHf2
	EUTRAPagingeDRXCycleHf4
	EUTRAPagingeDRXCycleHf6
	EUTRAPagingeDRXCycleHf8
	EUTRAPagingeDRXCycleHf10
	EUTRAPagingeDRXCycleHf12
	EUTRAPagingeDRXCycleHf14
	EUTRAPagingeDRXCycleHf16
	EUTRAPagingeDRXCycleHf32
	EUTRAPagingeDRXCycleHf64
	EUTRAPagingeDRXCycleHf128
	EUTRAPagingeDRXCycleHf256
)

type EUTRAPagingTimeWindow uint8

const (
	EUTRAPagingTimeWindowS1 EUTRAPagingTimeWindow = iota
	EUTRAPagingTimeWindowS2
	EUTRAPagingTimeWindowS3
	EUTRAPagingTimeWindowS4
	EUTRAPagingTimeWindowS5
	EUTRAPagingTimeWindowS6
	EUTRAPagingTimeWindowS7
	EUTRAPagingTimeWindowS8
	EUTRAPagingTimeWindowS9
	EUTRAPagingTimeWindowS10
	EUTRAPagingTimeWindowS11
	EUTRAPagingTimeWindowS12
	EUTRAPagingTimeWindowS13
	EUTRAPagingTimeWindowS14
	EUTRAPagingTimeWindowS15
	EUTRAPagingTimeWindowS16
)

type NRPagingeDRXCycle uint8

const (
	NRPagingeDRXCycleHfquarter NRPagingeDRXCycle = iota
	NRPagingeDRXCycleHfhalf
	NRPagingeDRXCycleHf1
	NRPagingeDRXCycleHf2
	NRPagingeDRXCycleHf4
	NRPagingeDRXCycleHf8
	NRPagingeDRXCycleHf16
	NRPagingeDRXCycleHf32
	NRPagingeDRXCycleHf64
	NRPagingeDRXCycleHf128
	NRPagingeDRXCycleHf256
	NRPagingeDRXCycleHf512
	NRPagingeDRXCycleHf1024
)

type NRPagingTimeWindow uint8

const (
	NRPagingTimeWindowS1 NRPagingTimeWindow = iota
	NRPagingTimeWindowS2
	NRPagingTimeWindowS3
	NRPagingTimeWindowS4
	NRPagingTimeWindowS5
	NRPagingTimeWindowS6
	NRPagingTimeWindowS7
	NRPagingTimeWindowS8
	NRPagingTimeWindowS9
	NRPagingTimeWindowS10
	NRPagingTimeWindowS11
	NRPagingTimeWindowS12
	NRPagingTimeWindowS13
	NRPagingTimeWindowS14
	NRPagingTimeWindowS15
	NRPagingTimeWindowS16
	NRPagingTimeWindowS17
	NRPagingTimeWindowS18
	NRPagingTimeWindowS19
	NRPagingTimeWindowS20
	NRPagingTimeWindowS21
	NRPagingTimeWindowS22
	NRPagingTimeWindowS23
	NRPagingTimeWindowS24
	NRPagingTimeWindowS25
	NRPagingTimeWindowS26
	NRPagingTimeWindowS27
	NRPagingTimeWindowS28
	NRPagingTimeWindowS29
	NRPagingTimeWindowS30
	NRPagingTimeWindowS31
	NRPagingTimeWindowS32
)

type CNsubgroupID int64

func (v CNsubgroupID) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return per.EncodeInteger(w, enc, per.Bounds{LB: 0, HasLB: true, UB: 7, HasUB: true, Extensible: true}, int64(v))
}

func (v *CNsubgroupID) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	n, err := per.DecodeInteger(r, enc, per.Bounds{LB: 0, HasLB: true, UB: 7, HasUB: true, Extensible: true})
	if err != nil {
		return err
	}

	*v = CNsubgroupID(n)

	return nil
}

func (v CNsubgroupID) Diagnose(*ap.Diagnosis[ProtocolIEID]) bool { return outsideRoot(int64(v), 0, 7) }

type LPWUSCNsubgroupID int64

func (v LPWUSCNsubgroupID) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return per.EncodeInteger(w, enc, per.Bounds{LB: 0, HasLB: true, UB: 30, HasUB: true, Extensible: true}, int64(v))
}

func (v *LPWUSCNsubgroupID) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	n, err := per.DecodeInteger(r, enc, per.Bounds{LB: 0, HasLB: true, UB: 30, HasUB: true, Extensible: true})
	if err != nil {
		return err
	}

	*v = LPWUSCNsubgroupID(n)

	return nil
}

func (v LPWUSCNsubgroupID) Diagnose(*ap.Diagnosis[ProtocolIEID]) bool {
	return outsideRoot(int64(v), 0, 30)
}

type MBSSessionSetuporModifyRequestItem struct {
	MBSSessionID                                 MBSSessionID
	MBSAreaSessionID                             *MBSAreaSessionID                            `per:",optional"`
	AssociatedMBSQosFlowSetuporModifyRequestList AssociatedMBSQosFlowSetuporModifyRequestList `per:",optional"`
	MBSQosFlowToReleaseList                      QosFlowListWithCause                         `per:",optional"`
	MBSAssistanceInformation                     *MBSAssistanceInformation                    `per:",extie:IDMBSAssistanceInformation,crit:ignore"`
	iEExtensions                                 extensionMeta                                `per:",exttable"`
	extensions                                   per.ExtensionAdditions                       `per:",extraw"`
}

type MBSSessionToReleaseItem struct {
	MBSSessionID MBSSessionID
	Cause        Cause
	iEExtensions extensionMeta          `per:",exttable"`
	extensions   per.ExtensionAdditions `per:",extraw"`
}

type MBSSessionSetupRequestItem struct {
	MBSSessionID                         MBSSessionID
	MBSAreaSessionID                     *MBSAreaSessionID                    `per:",optional"`
	AssociatedMBSQosFlowSetupRequestList AssociatedMBSQosFlowSetupRequestList `per:",optional"`
	MBSAssistanceInformation             *MBSAssistanceInformation            `per:",extie:IDMBSAssistanceInformation,crit:ignore"`
	iEExtensions                         extensionMeta                        `per:",exttable"`
	extensions                           per.ExtensionAdditions               `per:",extraw"`
}

type QoEReferenceList []QoEReference

func (l QoEReferenceList) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return marshalSeqOf(w, enc, 1, maxnoofUEAppLayerMeas, []QoEReference(l))
}

func (l *QoEReferenceList) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	items, err := unmarshalSeqOf[QoEReference](r, enc, 1, maxnoofUEAppLayerMeas)
	if err != nil {
		return err
	}

	*l = items

	return nil
}

type RecommendedCellsForPaging struct {
	RecommendedCellList RecommendedCellList
	iEExtensions        extensionMeta          `per:",exttable"`
	extensions          per.ExtensionAdditions `per:",extraw"`
}

type RecommendedRANNodesForPaging struct {
	RecommendedRANNodeList RecommendedRANNodeList
	iEExtensions           extensionMeta          `per:",exttable"`
	extensions             per.ExtensionAdditions `per:",extraw"`
}

type CoverageEnhancementLevel []byte

func (v CoverageEnhancementLevel) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return per.EncodeOctetString(w, enc, 0, 0, true, false, false, v)
}

func (v *CoverageEnhancementLevel) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	b, err := per.DecodeOctetString(r, enc, 0, 0, true, false, false)
	if err != nil {
		return err
	}

	*v = b

	return nil
}

type FromEUTRANtoNGRAN struct {
	SourceeNBID       IntersystemSONeNBID
	TargetNGRANnodeID IntersystemSONNGRANnodeID
	iEExtensions      extensionMeta `per:",exttable"`
}

type FromNGRANtoEUTRAN struct {
	SourceNGRANnodeID IntersystemSONNGRANnodeID
	TargeteNBID       IntersystemSONeNBID
	iEExtensions      extensionMeta `per:",exttable"`
}

type IntersystemSONInformationReport struct {
	HOReportInformation             *InterSystemHOReport             `per:",choice:0"`
	FailureIndicationInformation    *InterSystemFailureIndication    `per:",choice:1"`
	EnergySavingIndication          *IntersystemCellStateIndication  `per:",choice:2,extie:IDEnergySavingIndication,crit:ignore"`
	IntersystemResourceStatusUpdate *IntersystemResourceStatusReport `per:",choice:2,extie:IDIntersystemResourceStatusUpdate,crit:ignore"`
	ChoiceExtension                 *SingleContainer                 `per:",choice:2,extie"`
	choiceCriticality               apimpl.WireCriticality           `per:",choicecrit"`
}

type IntersystemSONInformationRequest struct {
	NGRANCellActivation *IntersystemCellActivationRequest `per:",choice:0"`
	ResourceStatus      *IntersystemResourceStatusRequest `per:",choice:1"`
	ChoiceExtension     *SingleContainer                  `per:",choice:2,extie"`
}

type IntersystemSONInformationReply struct {
	NGRANCellActivation *IntersystemCellActivationReply `per:",choice:0"`
	ResourceStatus      *IntersystemResourceStatusReply `per:",choice:1"`
	ChoiceExtension     *SingleContainer                `per:",choice:2,extie"`
}

type TAIListForInactiveItem struct {
	TAI          TAI
	iEExtensions extensionMeta          `per:",exttable"`
	extensions   per.ExtensionAdditions `per:",extraw"`
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

type ExpectedUEMobility uint8

const (
	ExpectedUEMobilityStationary ExpectedUEMobility = iota
	ExpectedUEMobilityMobile
)

type ExpectedUEMovingTrajectory []ExpectedUEMovingTrajectoryItem

func (l ExpectedUEMovingTrajectory) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return marshalSeqOf(w, enc, 1, maxnoofCellsUEMovingTrajectory, []ExpectedUEMovingTrajectoryItem(l))
}

func (l *ExpectedUEMovingTrajectory) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	items, err := unmarshalSeqOf[ExpectedUEMovingTrajectoryItem](r, enc, 1, maxnoofCellsUEMovingTrajectory)
	if err != nil {
		return err
	}

	*l = items

	return nil
}

type PC5QoSFlowItem struct {
	PQI             FiveQI
	Pc5FlowBitRates *PC5FlowBitRates       `per:",optional"`
	Range           *RangeType             `per:",optional"`
	iEExtensions    extensionMeta          `per:",exttable"`
	extensions      per.ExtensionAdditions `per:",extraw"`
}

type ClockQualityDetailLevel struct {
	ClockQualityMetrics              *per.Null                         `per:",choice:0"`
	ClockQualityAcceptanceIndication *ClockQualityAcceptanceIndication `per:",choice:1"`
	ChoiceExtension                  *SingleContainer                  `per:",choice:2,extie"`
}

type FiveGProSePC5QoSFlowItem struct {
	FiveGproSepQI             FiveQI
	FiveGproSepc5FlowBitRates *FiveGProSePC5FlowBitRates `per:",optional"`
	FiveGproSerange           *RangeType                 `per:",optional"`
	iEExtensions              extensionMeta              `per:",exttable"`
	extensions                per.ExtensionAdditions     `per:",extraw"`
}

type A2XPC5QoSFlowItemA2XPQI int64

func (v A2XPC5QoSFlowItemA2XPQI) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return per.EncodeInteger(w, enc, per.Bounds{LB: 0, HasLB: true, UB: 255, HasUB: true, Extensible: true}, int64(v))
}

func (v *A2XPC5QoSFlowItemA2XPQI) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	n, err := per.DecodeInteger(r, enc, per.Bounds{LB: 0, HasLB: true, UB: 255, HasUB: true, Extensible: true})
	if err != nil {
		return err
	}

	*v = A2XPC5QoSFlowItemA2XPQI(n)

	return nil
}

func (v A2XPC5QoSFlowItemA2XPQI) Diagnose(*ap.Diagnosis[ProtocolIEID]) bool {
	return outsideRoot(int64(v), 0, 255)
}

type A2XPC5QoSFlowItemA2XRange uint8

const (
	A2XPC5QoSFlowItemA2XRangeM50 A2XPC5QoSFlowItemA2XRange = iota
	A2XPC5QoSFlowItemA2XRangeM80
	A2XPC5QoSFlowItemA2XRangeM180
	A2XPC5QoSFlowItemA2XRangeM200
	A2XPC5QoSFlowItemA2XRangeM350
	A2XPC5QoSFlowItemA2XRangeM400
	A2XPC5QoSFlowItemA2XRangeM500
	A2XPC5QoSFlowItemA2XRangeM700
	A2XPC5QoSFlowItemA2XRangeM1000
)

type A2XPC5QoSFlowItem struct {
	A2XPQI             A2XPC5QoSFlowItemA2XPQI
	A2XPC5FlowBitRates *A2XPC5FlowBitRates        `per:",optional"`
	A2XRange           *A2XPC5QoSFlowItemA2XRange `per:",optional"`
	iEExtensions       extensionMeta              `per:",exttable"`
	extensions         per.ExtensionAdditions     `per:",extraw"`
}

type RSPPQoSFlowList []RSPPQoSFlowItem

func (l RSPPQoSFlowList) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return marshalSeqOf(w, enc, 1, maxnoofRSPPQoSFlows, []RSPPQoSFlowItem(l))
}

func (l *RSPPQoSFlowList) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	items, err := unmarshalSeqOf[RSPPQoSFlowItem](r, enc, 1, maxnoofRSPPQoSFlows)
	if err != nil {
		return err
	}

	*l = items

	return nil
}

type TargetNSSAIItem struct {
	SNSSAI       SNSSAI
	iEExtensions extensionMeta          `per:",exttable"`
	extensions   per.ExtensionAdditions `per:",extraw"`
}

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

func (v PagingAttemptCount) Diagnose(*ap.Diagnosis[ProtocolIEID]) bool {
	return outsideRoot(int64(v), 1, 16)
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

func (v IntendedNumberOfPagingAttempts) Diagnose(*ap.Diagnosis[ProtocolIEID]) bool {
	return outsideRoot(int64(v), 1, 16)
}

type NextPagingAreaScope uint8

const (
	NextPagingAreaScopeSame NextPagingAreaScope = iota
	NextPagingAreaScopeChanged
)

type AssociatedMBSQosFlowSetuporModifyRequestList []AssociatedMBSQosFlowSetuporModifyRequestItem

func (l AssociatedMBSQosFlowSetuporModifyRequestList) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return marshalSeqOf(w, enc, 1, maxnoofMBSQoSFlows, []AssociatedMBSQosFlowSetuporModifyRequestItem(l))
}

func (l *AssociatedMBSQosFlowSetuporModifyRequestList) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	items, err := unmarshalSeqOf[AssociatedMBSQosFlowSetuporModifyRequestItem](r, enc, 1, maxnoofMBSQoSFlows)
	if err != nil {
		return err
	}

	*l = items

	return nil
}

type MBSAssistanceInformation uint8

const (
	MBSAssistanceInformationTrue MBSAssistanceInformation = iota
)

type AssociatedMBSQosFlowSetupRequestList []AssociatedMBSQosFlowSetupRequestItem

func (l AssociatedMBSQosFlowSetupRequestList) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return marshalSeqOf(w, enc, 1, maxnoofMBSQoSFlows, []AssociatedMBSQosFlowSetupRequestItem(l))
}

func (l *AssociatedMBSQosFlowSetupRequestList) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	items, err := unmarshalSeqOf[AssociatedMBSQosFlowSetupRequestItem](r, enc, 1, maxnoofMBSQoSFlows)
	if err != nil {
		return err
	}

	*l = items

	return nil
}

type RecommendedCellList []RecommendedCellItem

func (l RecommendedCellList) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return marshalSeqOf(w, enc, 1, maxnoofRecommendedCells, []RecommendedCellItem(l))
}

func (l *RecommendedCellList) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	items, err := unmarshalSeqOf[RecommendedCellItem](r, enc, 1, maxnoofRecommendedCells)
	if err != nil {
		return err
	}

	*l = items

	return nil
}

type RecommendedRANNodeList []RecommendedRANNodeItem

func (l RecommendedRANNodeList) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return marshalSeqOf(w, enc, 1, maxnoofRecommendedRANNodes, []RecommendedRANNodeItem(l))
}

func (l *RecommendedRANNodeList) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	items, err := unmarshalSeqOf[RecommendedRANNodeItem](r, enc, 1, maxnoofRecommendedRANNodes)
	if err != nil {
		return err
	}

	*l = items

	return nil
}

type IntersystemSONeNBID struct {
	GlobaleNBID    GlobalENBID
	SelectedEPSTAI EPSTAI
	iEExtensions   extensionMeta          `per:",exttable"`
	extensions     per.ExtensionAdditions `per:",extraw"`
}

type IntersystemSONNGRANnodeID struct {
	GlobalRANNodeID GlobalRANNodeID
	SelectedTAI     TAI
	iEExtensions    extensionMeta          `per:",exttable"`
	extensions      per.ExtensionAdditions `per:",extraw"`
}

type InterSystemHOReport struct {
	HandoverReportType InterSystemHandoverReportType
	iEExtensions       extensionMeta          `per:",exttable"`
	extensions         per.ExtensionAdditions `per:",extraw"`
}

type InterSystemFailureIndication struct {
	UERLFReportContainer *UERLFReportContainer  `per:",optional"`
	iEExtensions         extensionMeta          `per:",exttable"`
	extensions           per.ExtensionAdditions `per:",extraw"`
}

type IntersystemCellStateIndication struct {
	NotificationCellList NotificationCellList
	iEExtensions         extensionMeta          `per:",exttable"`
	extensions           per.ExtensionAdditions `per:",extraw"`
}

type IntersystemResourceStatusReport struct {
	ReportingSystem ResourceStatusReportingSystem
	iEExtensions    extensionMeta          `per:",exttable"`
	extensions      per.ExtensionAdditions `per:",extraw"`
}

type IntersystemCellActivationRequestActivationID int64

func (v IntersystemCellActivationRequestActivationID) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return per.EncodeInteger(w, enc, per.Bounds{LB: 0, HasLB: true, UB: 16384, HasUB: true, Extensible: true}, int64(v))
}

func (v *IntersystemCellActivationRequestActivationID) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	n, err := per.DecodeInteger(r, enc, per.Bounds{LB: 0, HasLB: true, UB: 16384, HasUB: true, Extensible: true})
	if err != nil {
		return err
	}

	*v = IntersystemCellActivationRequestActivationID(n)

	return nil
}

func (v IntersystemCellActivationRequestActivationID) Diagnose(*ap.Diagnosis[ProtocolIEID]) bool {
	return outsideRoot(int64(v), 0, 16384)
}

type IntersystemCellActivationRequest struct {
	ActivationID        IntersystemCellActivationRequestActivationID
	CellsToActivateList CellsToActivateList
	iEExtensions        extensionMeta          `per:",exttable"`
	extensions          per.ExtensionAdditions `per:",extraw"`
}

type IntersystemResourceStatusRequest struct {
	ReportingSystem       ReportingSystem
	ReportCharacteristics ReportCharacteristics
	ReportType            ReportType
	iEExtensions          extensionMeta          `per:",exttable"`
	extensions            per.ExtensionAdditions `per:",extraw"`
}

type IntersystemCellActivationReplyActivationID int64

func (v IntersystemCellActivationReplyActivationID) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return per.EncodeInteger(w, enc, per.Bounds{LB: 0, HasLB: true, UB: 16384, HasUB: true, Extensible: true}, int64(v))
}

func (v *IntersystemCellActivationReplyActivationID) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	n, err := per.DecodeInteger(r, enc, per.Bounds{LB: 0, HasLB: true, UB: 16384, HasUB: true, Extensible: true})
	if err != nil {
		return err
	}

	*v = IntersystemCellActivationReplyActivationID(n)

	return nil
}

func (v IntersystemCellActivationReplyActivationID) Diagnose(*ap.Diagnosis[ProtocolIEID]) bool {
	return outsideRoot(int64(v), 0, 16384)
}

type IntersystemCellActivationReply struct {
	ActivatedCellList ActivatedCellList
	ActivationID      IntersystemCellActivationReplyActivationID
	iEExtensions      extensionMeta          `per:",exttable"`
	extensions        per.ExtensionAdditions `per:",extraw"`
}

type IntersystemResourceStatusReply struct {
	Reportingsystem ReportingSystem
	iEExtensions    extensionMeta          `per:",exttable"`
	extensions      per.ExtensionAdditions `per:",extraw"`
}

type ExpectedUEMovingTrajectoryItemTimeStayedInCell uint16

func (v ExpectedUEMovingTrajectoryItemTimeStayedInCell) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return per.EncodeInteger(w, enc, per.Bounds{LB: 0, HasLB: true, UB: 4095, HasUB: true}, int64(v))
}

func (v *ExpectedUEMovingTrajectoryItemTimeStayedInCell) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	n, err := per.DecodeInteger(r, enc, per.Bounds{LB: 0, HasLB: true, UB: 4095, HasUB: true})
	if err != nil {
		return err
	}

	*v = ExpectedUEMovingTrajectoryItemTimeStayedInCell(n)

	return nil
}

type ExpectedUEMovingTrajectoryItem struct {
	NGRANCGI         NGRANCGI
	TimeStayedInCell *ExpectedUEMovingTrajectoryItemTimeStayedInCell `per:",optional"`
	iEExtensions     extensionMeta                                   `per:",exttable"`
	extensions       per.ExtensionAdditions                          `per:",extraw"`
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

type ClockQualityAcceptanceIndication struct {
	ClockQualityAcceptanceCriteria ClockQualityAcceptanceCriteria
	iEExtensions                   extensionMeta          `per:",exttable"`
	extensions                     per.ExtensionAdditions `per:",extraw"`
}

type FiveGProSePC5FlowBitRates struct {
	FiveGproSeguaranteedFlowBitRate BitRate
	FiveGproSemaximumFlowBitRate    BitRate
	iEExtensions                    extensionMeta          `per:",exttable"`
	extensions                      per.ExtensionAdditions `per:",extraw"`
}

type A2XPC5FlowBitRates struct {
	A2XGuaranteedFlowBitRate BitRate
	A2XMaximumFlowBitRate    BitRate
	iEExtensions             extensionMeta          `per:",exttable"`
	extensions               per.ExtensionAdditions `per:",extraw"`
}

type RSPPQoSFlowItem struct {
	PQI              FiveQI
	RSPPFlowBitRates *RSPPFlowBitRates      `per:",optional"`
	Range            *RangeType             `per:",optional"`
	iEExtensions     extensionMeta          `per:",exttable"`
	extensions       per.ExtensionAdditions `per:",extraw"`
}

type AssociatedMBSQosFlowSetuporModifyRequestItem struct {
	MBSQosFlowIdentifier               QosFlowIdentifier
	AssociatedUnicastQosFlowIdentifier QosFlowIdentifier
	iEExtensions                       extensionMeta          `per:",exttable"`
	extensions                         per.ExtensionAdditions `per:",extraw"`
}

type AssociatedMBSQosFlowSetupRequestItem struct {
	MBSQosFlowIdentifier               QosFlowIdentifier
	AssociatedUnicastQosFlowIdentifier QosFlowIdentifier
	iEExtensions                       extensionMeta          `per:",exttable"`
	extensions                         per.ExtensionAdditions `per:",extraw"`
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
	NGRANCGI         NGRANCGI
	TimeStayedInCell *RecommendedCellItemTimeStayedInCell `per:",optional"`
	iEExtensions     extensionMeta                        `per:",exttable"`
	extensions       per.ExtensionAdditions               `per:",extraw"`
}

type RecommendedRANNodeItem struct {
	AMFPagingTarget AMFPagingTarget
	iEExtensions    extensionMeta          `per:",exttable"`
	extensions      per.ExtensionAdditions `per:",extraw"`
}

type GlobalENBID struct {
	PLMNidentity PLMNIdentity
	ENBID        ENBID
	iEExtensions extensionMeta          `per:",exttable"`
	extensions   per.ExtensionAdditions `per:",extraw"`
}

type InterSystemHandoverReportType struct {
	TooearlyIntersystemHO                      *TooearlyIntersystemHO                      `per:",choice:0"`
	IntersystemUnnecessaryHO                   *IntersystemUnnecessaryHO                   `per:",choice:1"`
	IntersystemMobilityFailureforVoiceFallback *IntersystemMobilityFailureforVoiceFallback `per:",choice:2,extie:IDIntersystemMobilityFailureforVoiceFallback,crit:ignore"`
	ChoiceExtension                            *SingleContainer                            `per:",choice:2,extie"`
	choiceCriticality                          apimpl.WireCriticality                      `per:",choicecrit"`
}

type UERLFReportContainer struct {
	NR              *NRUERLFReportContainer  `per:",choice:0"`
	LTE             *LTEUERLFReportContainer `per:",choice:1"`
	ChoiceExtension *SingleContainer         `per:",choice:2,extie"`
}

type NotificationCellList []NotificationCellItem

func (l NotificationCellList) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return marshalSeqOf(w, enc, 1, maxnoofCellsinNGRANNode, []NotificationCellItem(l))
}

func (l *NotificationCellList) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	items, err := unmarshalSeqOf[NotificationCellItem](r, enc, 1, maxnoofCellsinNGRANNode)
	if err != nil {
		return err
	}

	*l = items

	return nil
}

type ResourceStatusReportingSystem struct {
	EUTRANReportingStatus *EUTRANReportingStatusIEs `per:",choice:0"`
	NGRANReportingStatus  *NGRANReportingStatusIEs  `per:",choice:1"`
	ChoiceExtension       *SingleContainer          `per:",choice:2,extie"`
}

type CellsToActivateList []NGRANCGI

func (l CellsToActivateList) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return marshalSeqOf(w, enc, 1, maxnoofCellsinNGRANNode, []NGRANCGI(l))
}

func (l *CellsToActivateList) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	items, err := unmarshalSeqOf[NGRANCGI](r, enc, 1, maxnoofCellsinNGRANNode)
	if err != nil {
		return err
	}

	*l = items

	return nil
}

type ReportingSystem struct {
	EUTRAN          *EUTRANReportingSystemIEs `per:",choice:0"`
	NGRAN           *NGRANReportingSystemIEs  `per:",choice:1"`
	NoReporting     *per.Null                 `per:",choice:2"`
	ChoiceExtension *SingleContainer          `per:",choice:3,extie"`
}

type ReportCharacteristics uint32

func (v ReportCharacteristics) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return encodeBitStringUint(w, enc, uint64(v), 32)
}

func (v *ReportCharacteristics) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	n, err := decodeBitStringUint(r, enc, 32)
	if err != nil {
		return err
	}

	*v = ReportCharacteristics(n)

	return nil
}

type ReportType struct {
	EventBasedReporting *EventBasedReportingIEs `per:",choice:0"`
	PeriodicReporting   *PeriodicReportingIEs   `per:",choice:1"`
	ChoiceExtension     *SingleContainer        `per:",choice:2,extie"`
}

type ActivatedCellList []NGRANCGI

func (l ActivatedCellList) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return marshalSeqOf(w, enc, 1, maxnoofCellsinNGRANNode, []NGRANCGI(l))
}

func (l *ActivatedCellList) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	items, err := unmarshalSeqOf[NGRANCGI](r, enc, 1, maxnoofCellsinNGRANNode)
	if err != nil {
		return err
	}

	*l = items

	return nil
}

type ClockQualityAcceptanceCriteriaSynchronisationState struct{ BitString }

func (v ClockQualityAcceptanceCriteriaSynchronisationState) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return v.encode(w, enc, 8, 8)
}

func (v *ClockQualityAcceptanceCriteriaSynchronisationState) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	b, err := decodeBitString(r, enc, 8, 8)
	if err != nil {
		return err
	}

	*v = ClockQualityAcceptanceCriteriaSynchronisationState{b}

	return nil
}

type ClockQualityAcceptanceCriteriaTraceabletoUTC uint8

const (
	ClockQualityAcceptanceCriteriaTraceabletoUTCTrue ClockQualityAcceptanceCriteriaTraceabletoUTC = iota
)

type ClockQualityAcceptanceCriteriaTraceabletoGNSS uint8

const (
	ClockQualityAcceptanceCriteriaTraceabletoGNSSTrue ClockQualityAcceptanceCriteriaTraceabletoGNSS = iota
)

type ClockQualityAcceptanceCriteriaClockFrequencyStability uint16

func (v ClockQualityAcceptanceCriteriaClockFrequencyStability) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return encodeBitStringUint(w, enc, uint64(v), 16)
}

func (v *ClockQualityAcceptanceCriteriaClockFrequencyStability) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	n, err := decodeBitStringUint(r, enc, 16)
	if err != nil {
		return err
	}

	*v = ClockQualityAcceptanceCriteriaClockFrequencyStability(n)

	return nil
}

type ClockQualityAcceptanceCriteriaClockAccuracy int64

func (v ClockQualityAcceptanceCriteriaClockAccuracy) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return per.EncodeInteger(w, enc, per.Bounds{LB: 1, HasLB: true, UB: 40000000, HasUB: true, Extensible: true}, int64(v))
}

func (v *ClockQualityAcceptanceCriteriaClockAccuracy) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	n, err := per.DecodeInteger(r, enc, per.Bounds{LB: 1, HasLB: true, UB: 40000000, HasUB: true, Extensible: true})
	if err != nil {
		return err
	}

	*v = ClockQualityAcceptanceCriteriaClockAccuracy(n)

	return nil
}

func (v ClockQualityAcceptanceCriteriaClockAccuracy) Diagnose(*ap.Diagnosis[ProtocolIEID]) bool {
	return outsideRoot(int64(v), 1, 40000000)
}

type ClockQualityAcceptanceCriteriaParentTImeSource struct{ BitString }

func (v ClockQualityAcceptanceCriteriaParentTImeSource) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return v.encode(w, enc, 16, 16)
}

func (v *ClockQualityAcceptanceCriteriaParentTImeSource) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	b, err := decodeBitString(r, enc, 16, 16)
	if err != nil {
		return err
	}

	*v = ClockQualityAcceptanceCriteriaParentTImeSource{b}

	return nil
}

type ClockQualityAcceptanceCriteria struct {
	SynchronisationState    *ClockQualityAcceptanceCriteriaSynchronisationState    `per:",optional"`
	TraceabletoUTC          *ClockQualityAcceptanceCriteriaTraceabletoUTC          `per:",optional"`
	TraceabletoGNSS         *ClockQualityAcceptanceCriteriaTraceabletoGNSS         `per:",optional"`
	ClockFrequencyStability *ClockQualityAcceptanceCriteriaClockFrequencyStability `per:",optional"`
	ClockAccuracy           *ClockQualityAcceptanceCriteriaClockAccuracy           `per:",optional"`
	ParentTImeSource        *ClockQualityAcceptanceCriteriaParentTImeSource        `per:",optional"`
	iEExtensions            extensionMeta                                          `per:",exttable"`
	extensions              per.ExtensionAdditions                                 `per:",extraw"`
}

type RSPPFlowBitRates struct {
	GuaranteedFlowBitRate BitRate
	MaximumFlowBitRate    BitRate
	iEExtensions          extensionMeta          `per:",exttable"`
	extensions            per.ExtensionAdditions `per:",extraw"`
}

type AMFPagingTarget struct {
	GlobalRANNodeID *GlobalRANNodeID `per:",choice:0"`
	TAI             *TAI             `per:",choice:1"`
	ChoiceExtension *SingleContainer `per:",choice:2,extie"`
}

type ENBIDMacroENBID uint32

func (v ENBIDMacroENBID) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return encodeBitStringUint(w, enc, uint64(v), 20)
}

func (v *ENBIDMacroENBID) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	n, err := decodeBitStringUint(r, enc, 20)
	if err != nil {
		return err
	}

	*v = ENBIDMacroENBID(n)

	return nil
}

type ENBIDHomeENBID uint32

func (v ENBIDHomeENBID) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return encodeBitStringUint(w, enc, uint64(v), 28)
}

func (v *ENBIDHomeENBID) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	n, err := decodeBitStringUint(r, enc, 28)
	if err != nil {
		return err
	}

	*v = ENBIDHomeENBID(n)

	return nil
}

type ENBIDShortmacroENBID uint32

func (v ENBIDShortmacroENBID) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return encodeBitStringUint(w, enc, uint64(v), 18)
}

func (v *ENBIDShortmacroENBID) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	n, err := decodeBitStringUint(r, enc, 18)
	if err != nil {
		return err
	}

	*v = ENBIDShortmacroENBID(n)

	return nil
}

type ENBIDLongmacroENBID uint32

func (v ENBIDLongmacroENBID) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return encodeBitStringUint(w, enc, uint64(v), 21)
}

func (v *ENBIDLongmacroENBID) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	n, err := decodeBitStringUint(r, enc, 21)
	if err != nil {
		return err
	}

	*v = ENBIDLongmacroENBID(n)

	return nil
}

type ENBID struct {
	MacroENBID      *ENBIDMacroENBID      `per:",choice:0"`
	HomeENBID       *ENBIDHomeENBID       `per:",choice:1"`
	ShortmacroENBID *ENBIDShortmacroENBID `per:",choice:2"`
	LongmacroENBID  *ENBIDLongmacroENBID  `per:",choice:3"`
	ChoiceExtension *SingleContainer      `per:",choice:4,extie"`
}

type TooearlyIntersystemHO struct {
	SourcecellID         EUTRACGI
	FailurecellID        NGRANCGI
	UERLFReportContainer *UERLFReportContainer  `per:",optional"`
	iEExtensions         extensionMeta          `per:",exttable"`
	extensions           per.ExtensionAdditions `per:",extraw"`
}

type IntersystemUnnecessaryHOEarlyIRATHO uint8

const (
	IntersystemUnnecessaryHOEarlyIRATHOTrue IntersystemUnnecessaryHOEarlyIRATHO = iota
	IntersystemUnnecessaryHOEarlyIRATHOFalse
)

type IntersystemUnnecessaryHO struct {
	SourcecellID      NGRANCGI
	TargetcellID      EUTRACGI
	EarlyIRATHO       IntersystemUnnecessaryHOEarlyIRATHO
	CandidateCellList CandidateCellList
	iEExtensions      extensionMeta          `per:",exttable"`
	extensions        per.ExtensionAdditions `per:",extraw"`
}

type IntersystemMobilityFailureforVoiceFallback struct {
	SourcecellID         NGRANCGI
	TargetcellID         EUTRACGI
	ReconnectCellID      *EUTRACGI              `per:",optional"`
	UERLFReportContainer *UERLFReportContainer  `per:",optional"`
	iEExtensions         extensionMeta          `per:",exttable"`
	extensions           per.ExtensionAdditions `per:",extraw"`
}

type NRUERLFReportContainer []byte

func (v NRUERLFReportContainer) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return per.EncodeOctetString(w, enc, 0, 0, true, false, false, v)
}

func (v *NRUERLFReportContainer) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	b, err := per.DecodeOctetString(r, enc, 0, 0, true, false, false)
	if err != nil {
		return err
	}

	*v = b

	return nil
}

type LTEUERLFReportContainer []byte

func (v LTEUERLFReportContainer) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return per.EncodeOctetString(w, enc, 0, 0, true, false, false, v)
}

func (v *LTEUERLFReportContainer) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	b, err := per.DecodeOctetString(r, enc, 0, 0, true, false, false)
	if err != nil {
		return err
	}

	*v = b

	return nil
}

type NotificationCellItemNotifyFlag uint8

const (
	NotificationCellItemNotifyFlagActivated NotificationCellItemNotifyFlag = iota
	NotificationCellItemNotifyFlagDeactivated
)

type NotificationCellItem struct {
	NGRANCGI     NGRANCGI
	NotifyFlag   NotificationCellItemNotifyFlag
	iEExtensions extensionMeta          `per:",exttable"`
	extensions   per.ExtensionAdditions `per:",extraw"`
}

type EUTRANReportingStatusIEs struct {
	EUTRANCellReportList EUTRANCellReportList
	iEExtensions         extensionMeta          `per:",exttable"`
	extensions           per.ExtensionAdditions `per:",extraw"`
}

type NGRANReportingStatusIEs struct {
	NGRANCellReportList NGRANCellReportList
	iEExtensions        extensionMeta          `per:",exttable"`
	extensions          per.ExtensionAdditions `per:",extraw"`
}

type EUTRANReportingSystemIEs struct {
	EUTRANCellToReportList EUTRANCellToReportList
	iEExtensions           extensionMeta          `per:",exttable"`
	extensions             per.ExtensionAdditions `per:",extraw"`
}

type NGRANReportingSystemIEs struct {
	NGRANCellToReportList NGRANCellToReportList
	iEExtensions          extensionMeta          `per:",exttable"`
	extensions            per.ExtensionAdditions `per:",extraw"`
}

type EventBasedReportingIEs struct {
	IntersystemResourceThresholdLow    IntersystemResourceThreshold
	IntersystemResourceThresholdHigh   IntersystemResourceThreshold
	NumberOfMeasurementReportingLevels NumberOfMeasurementReportingLevels
	iEExtensions                       extensionMeta          `per:",exttable"`
	extensions                         per.ExtensionAdditions `per:",extraw"`
}

type PeriodicReportingIEs struct {
	ReportingPeriodicity ReportingPeriodicity
	iEExtensions         extensionMeta          `per:",exttable"`
	extensions           per.ExtensionAdditions `per:",extraw"`
}

type CandidateCellList []CandidateCellItem

func (l CandidateCellList) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return marshalSeqOf(w, enc, 1, maxnoofCandidateCells, []CandidateCellItem(l))
}

func (l *CandidateCellList) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	items, err := unmarshalSeqOf[CandidateCellItem](r, enc, 1, maxnoofCandidateCells)
	if err != nil {
		return err
	}

	*l = items

	return nil
}

type EUTRANCellReportList []EUTRANCellReportItem

func (l EUTRANCellReportList) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return marshalSeqOf(w, enc, 1, maxnoofReportedCells, []EUTRANCellReportItem(l))
}

func (l *EUTRANCellReportList) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	items, err := unmarshalSeqOf[EUTRANCellReportItem](r, enc, 1, maxnoofReportedCells)
	if err != nil {
		return err
	}

	*l = items

	return nil
}

type NGRANCellReportList []NGRANCellReportItem

func (l NGRANCellReportList) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return marshalSeqOf(w, enc, 1, maxnoofReportedCells, []NGRANCellReportItem(l))
}

func (l *NGRANCellReportList) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	items, err := unmarshalSeqOf[NGRANCellReportItem](r, enc, 1, maxnoofReportedCells)
	if err != nil {
		return err
	}

	*l = items

	return nil
}

type EUTRANCellToReportList []EUTRANCellToReportItem

func (l EUTRANCellToReportList) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return marshalSeqOf(w, enc, 1, maxnoofReportedCells, []EUTRANCellToReportItem(l))
}

func (l *EUTRANCellToReportList) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	items, err := unmarshalSeqOf[EUTRANCellToReportItem](r, enc, 1, maxnoofReportedCells)
	if err != nil {
		return err
	}

	*l = items

	return nil
}

type NGRANCellToReportList []NGRANCellToReportItem

func (l NGRANCellToReportList) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return marshalSeqOf(w, enc, 1, maxnoofReportedCells, []NGRANCellToReportItem(l))
}

func (l *NGRANCellToReportList) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	items, err := unmarshalSeqOf[NGRANCellToReportItem](r, enc, 1, maxnoofReportedCells)
	if err != nil {
		return err
	}

	*l = items

	return nil
}

type IntersystemResourceThreshold uint8

func (v IntersystemResourceThreshold) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return per.EncodeInteger(w, enc, per.Bounds{LB: 0, HasLB: true, UB: 100, HasUB: true}, int64(v))
}

func (v *IntersystemResourceThreshold) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	n, err := per.DecodeInteger(r, enc, per.Bounds{LB: 0, HasLB: true, UB: 100, HasUB: true})
	if err != nil {
		return err
	}

	*v = IntersystemResourceThreshold(n)

	return nil
}

type NumberOfMeasurementReportingLevels uint8

const (
	NumberOfMeasurementReportingLevelsN2 NumberOfMeasurementReportingLevels = iota
	NumberOfMeasurementReportingLevelsN3
	NumberOfMeasurementReportingLevelsN4
	NumberOfMeasurementReportingLevelsN5
	NumberOfMeasurementReportingLevelsN10
	NumberOfMeasurementReportingLevelsN0
)

type ReportingPeriodicity uint8

const (
	ReportingPeriodicityStop ReportingPeriodicity = iota
	ReportingPeriodicitySingle
	ReportingPeriodicityMs1000
	ReportingPeriodicityMs2000
	ReportingPeriodicityMs5000
	ReportingPeriodicityMs10000
)

type CandidateCellItem struct {
	CandidateCell CandidateCell
	iEExtensions  extensionMeta          `per:",exttable"`
	extensions    per.ExtensionAdditions `per:",extraw"`
}

type EUTRANCellReportItem struct {
	ECGI                                  EUTRACGI
	EUTRANCompositeAvailableCapacityGroup EUTRANCompositeAvailableCapacityGroup
	EUTRANNumberOfActiveUEs               *EUTRANNumberOfActiveUEs   `per:",optional"`
	EUTRANNoofRRCConnections              *NGRANNoofRRCConnections   `per:",optional"`
	EUTRANRadioResourceStatus             *EUTRANRadioResourceStatus `per:",optional"`
	iEExtensions                          extensionMeta              `per:",exttable"`
	extensions                            per.ExtensionAdditions     `per:",extraw"`
}

type NGRANCellReportItem struct {
	NGRANCGI                             NGRANCGI
	NGRANCompositeAvailableCapacityGroup EUTRANCompositeAvailableCapacityGroup
	NGRANNumberOfActiveUEs               *NGRANNumberOfActiveUEs   `per:",optional"`
	NGRANNoofRRCConnections              *NGRANNoofRRCConnections  `per:",optional"`
	NGRANRadioResourceStatus             *NGRANRadioResourceStatus `per:",optional"`
	iEExtensions                         extensionMeta             `per:",exttable"`
	extensions                           per.ExtensionAdditions    `per:",extraw"`
}

type EUTRANCellToReportItem struct {
	ECGI         EUTRACGI
	iEExtensions extensionMeta          `per:",exttable"`
	extensions   per.ExtensionAdditions `per:",extraw"`
}

type NGRANCellToReportItem struct {
	NGRANCGI     NGRANCGI
	iEExtensions extensionMeta          `per:",exttable"`
	extensions   per.ExtensionAdditions `per:",extraw"`
}

type CandidateCell struct {
	CandidateCGI    *CandidateCellID `per:",choice:0"`
	CandidatePCI    *CandidatePCI    `per:",choice:1"`
	ChoiceExtension *SingleContainer `per:",choice:2,extie"`
}

type EUTRANCompositeAvailableCapacityGroup struct {
	DLCompositeAvailableCapacity CompositeAvailableCapacity
	ULCompositeAvailableCapacity CompositeAvailableCapacity
	iEExtensions                 extensionMeta          `per:",exttable"`
	extensions                   per.ExtensionAdditions `per:",extraw"`
}

type EUTRANNumberOfActiveUEs int64

func (v EUTRANNumberOfActiveUEs) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return per.EncodeInteger(w, enc, per.Bounds{LB: 0, HasLB: true, UB: 16777215, HasUB: true, Extensible: true}, int64(v))
}

func (v *EUTRANNumberOfActiveUEs) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	n, err := per.DecodeInteger(r, enc, per.Bounds{LB: 0, HasLB: true, UB: 16777215, HasUB: true, Extensible: true})
	if err != nil {
		return err
	}

	*v = EUTRANNumberOfActiveUEs(n)

	return nil
}

func (v EUTRANNumberOfActiveUEs) Diagnose(*ap.Diagnosis[ProtocolIEID]) bool {
	return outsideRoot(int64(v), 0, 16777215)
}

type NGRANNoofRRCConnections int64

func (v NGRANNoofRRCConnections) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return per.EncodeInteger(w, enc, per.Bounds{LB: 1, HasLB: true, UB: 65536, HasUB: true, Extensible: true}, int64(v))
}

func (v *NGRANNoofRRCConnections) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	n, err := per.DecodeInteger(r, enc, per.Bounds{LB: 1, HasLB: true, UB: 65536, HasUB: true, Extensible: true})
	if err != nil {
		return err
	}

	*v = NGRANNoofRRCConnections(n)

	return nil
}

func (v NGRANNoofRRCConnections) Diagnose(*ap.Diagnosis[ProtocolIEID]) bool {
	return outsideRoot(int64(v), 1, 65536)
}

type EUTRANRadioResourceStatusDLGBRPRBusage uint8

func (v EUTRANRadioResourceStatusDLGBRPRBusage) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return per.EncodeInteger(w, enc, per.Bounds{LB: 0, HasLB: true, UB: 100, HasUB: true}, int64(v))
}

func (v *EUTRANRadioResourceStatusDLGBRPRBusage) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	n, err := per.DecodeInteger(r, enc, per.Bounds{LB: 0, HasLB: true, UB: 100, HasUB: true})
	if err != nil {
		return err
	}

	*v = EUTRANRadioResourceStatusDLGBRPRBusage(n)

	return nil
}

type EUTRANRadioResourceStatusULGBRPRBusage uint8

func (v EUTRANRadioResourceStatusULGBRPRBusage) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return per.EncodeInteger(w, enc, per.Bounds{LB: 0, HasLB: true, UB: 100, HasUB: true}, int64(v))
}

func (v *EUTRANRadioResourceStatusULGBRPRBusage) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	n, err := per.DecodeInteger(r, enc, per.Bounds{LB: 0, HasLB: true, UB: 100, HasUB: true})
	if err != nil {
		return err
	}

	*v = EUTRANRadioResourceStatusULGBRPRBusage(n)

	return nil
}

type EUTRANRadioResourceStatusDLnonGBRPRBusage uint8

func (v EUTRANRadioResourceStatusDLnonGBRPRBusage) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return per.EncodeInteger(w, enc, per.Bounds{LB: 0, HasLB: true, UB: 100, HasUB: true}, int64(v))
}

func (v *EUTRANRadioResourceStatusDLnonGBRPRBusage) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	n, err := per.DecodeInteger(r, enc, per.Bounds{LB: 0, HasLB: true, UB: 100, HasUB: true})
	if err != nil {
		return err
	}

	*v = EUTRANRadioResourceStatusDLnonGBRPRBusage(n)

	return nil
}

type EUTRANRadioResourceStatusULnonGBRPRBusage uint8

func (v EUTRANRadioResourceStatusULnonGBRPRBusage) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return per.EncodeInteger(w, enc, per.Bounds{LB: 0, HasLB: true, UB: 100, HasUB: true}, int64(v))
}

func (v *EUTRANRadioResourceStatusULnonGBRPRBusage) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	n, err := per.DecodeInteger(r, enc, per.Bounds{LB: 0, HasLB: true, UB: 100, HasUB: true})
	if err != nil {
		return err
	}

	*v = EUTRANRadioResourceStatusULnonGBRPRBusage(n)

	return nil
}

type EUTRANRadioResourceStatusDLTotalPRBusage uint8

func (v EUTRANRadioResourceStatusDLTotalPRBusage) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return per.EncodeInteger(w, enc, per.Bounds{LB: 0, HasLB: true, UB: 100, HasUB: true}, int64(v))
}

func (v *EUTRANRadioResourceStatusDLTotalPRBusage) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	n, err := per.DecodeInteger(r, enc, per.Bounds{LB: 0, HasLB: true, UB: 100, HasUB: true})
	if err != nil {
		return err
	}

	*v = EUTRANRadioResourceStatusDLTotalPRBusage(n)

	return nil
}

type EUTRANRadioResourceStatusULTotalPRBusage uint8

func (v EUTRANRadioResourceStatusULTotalPRBusage) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return per.EncodeInteger(w, enc, per.Bounds{LB: 0, HasLB: true, UB: 100, HasUB: true}, int64(v))
}

func (v *EUTRANRadioResourceStatusULTotalPRBusage) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	n, err := per.DecodeInteger(r, enc, per.Bounds{LB: 0, HasLB: true, UB: 100, HasUB: true})
	if err != nil {
		return err
	}

	*v = EUTRANRadioResourceStatusULTotalPRBusage(n)

	return nil
}

type EUTRANRadioResourceStatusDLschedulingPDCCHCCEusage uint8

func (v EUTRANRadioResourceStatusDLschedulingPDCCHCCEusage) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return per.EncodeInteger(w, enc, per.Bounds{LB: 0, HasLB: true, UB: 100, HasUB: true}, int64(v))
}

func (v *EUTRANRadioResourceStatusDLschedulingPDCCHCCEusage) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	n, err := per.DecodeInteger(r, enc, per.Bounds{LB: 0, HasLB: true, UB: 100, HasUB: true})
	if err != nil {
		return err
	}

	*v = EUTRANRadioResourceStatusDLschedulingPDCCHCCEusage(n)

	return nil
}

type EUTRANRadioResourceStatusULschedulingPDCCHCCEusage uint8

func (v EUTRANRadioResourceStatusULschedulingPDCCHCCEusage) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return per.EncodeInteger(w, enc, per.Bounds{LB: 0, HasLB: true, UB: 100, HasUB: true}, int64(v))
}

func (v *EUTRANRadioResourceStatusULschedulingPDCCHCCEusage) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	n, err := per.DecodeInteger(r, enc, per.Bounds{LB: 0, HasLB: true, UB: 100, HasUB: true})
	if err != nil {
		return err
	}

	*v = EUTRANRadioResourceStatusULschedulingPDCCHCCEusage(n)

	return nil
}

type EUTRANRadioResourceStatus struct {
	DLGBRPRBusage             EUTRANRadioResourceStatusDLGBRPRBusage
	ULGBRPRBusage             EUTRANRadioResourceStatusULGBRPRBusage
	DLnonGBRPRBusage          EUTRANRadioResourceStatusDLnonGBRPRBusage
	ULnonGBRPRBusage          EUTRANRadioResourceStatusULnonGBRPRBusage
	DLTotalPRBusage           EUTRANRadioResourceStatusDLTotalPRBusage
	ULTotalPRBusage           EUTRANRadioResourceStatusULTotalPRBusage
	DLschedulingPDCCHCCEusage *EUTRANRadioResourceStatusDLschedulingPDCCHCCEusage `per:",optional"`
	ULschedulingPDCCHCCEusage *EUTRANRadioResourceStatusULschedulingPDCCHCCEusage `per:",optional"`
	iEExtensions              extensionMeta                                       `per:",exttable"`
	extensions                per.ExtensionAdditions                              `per:",extraw"`
}

type NGRANNumberOfActiveUEs int64

func (v NGRANNumberOfActiveUEs) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return per.EncodeInteger(w, enc, per.Bounds{LB: 0, HasLB: true, UB: 16777215, HasUB: true, Extensible: true}, int64(v))
}

func (v *NGRANNumberOfActiveUEs) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	n, err := per.DecodeInteger(r, enc, per.Bounds{LB: 0, HasLB: true, UB: 16777215, HasUB: true, Extensible: true})
	if err != nil {
		return err
	}

	*v = NGRANNumberOfActiveUEs(n)

	return nil
}

func (v NGRANNumberOfActiveUEs) Diagnose(*ap.Diagnosis[ProtocolIEID]) bool {
	return outsideRoot(int64(v), 0, 16777215)
}

type NGRANRadioResourceStatusDLGBRPRBusageforMIMO uint8

func (v NGRANRadioResourceStatusDLGBRPRBusageforMIMO) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return per.EncodeInteger(w, enc, per.Bounds{LB: 0, HasLB: true, UB: 100, HasUB: true}, int64(v))
}

func (v *NGRANRadioResourceStatusDLGBRPRBusageforMIMO) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	n, err := per.DecodeInteger(r, enc, per.Bounds{LB: 0, HasLB: true, UB: 100, HasUB: true})
	if err != nil {
		return err
	}

	*v = NGRANRadioResourceStatusDLGBRPRBusageforMIMO(n)

	return nil
}

type NGRANRadioResourceStatusULGBRPRBusageforMIMO uint8

func (v NGRANRadioResourceStatusULGBRPRBusageforMIMO) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return per.EncodeInteger(w, enc, per.Bounds{LB: 0, HasLB: true, UB: 100, HasUB: true}, int64(v))
}

func (v *NGRANRadioResourceStatusULGBRPRBusageforMIMO) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	n, err := per.DecodeInteger(r, enc, per.Bounds{LB: 0, HasLB: true, UB: 100, HasUB: true})
	if err != nil {
		return err
	}

	*v = NGRANRadioResourceStatusULGBRPRBusageforMIMO(n)

	return nil
}

type NGRANRadioResourceStatusDLnonGBRPRBusageforMIMO uint8

func (v NGRANRadioResourceStatusDLnonGBRPRBusageforMIMO) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return per.EncodeInteger(w, enc, per.Bounds{LB: 0, HasLB: true, UB: 100, HasUB: true}, int64(v))
}

func (v *NGRANRadioResourceStatusDLnonGBRPRBusageforMIMO) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	n, err := per.DecodeInteger(r, enc, per.Bounds{LB: 0, HasLB: true, UB: 100, HasUB: true})
	if err != nil {
		return err
	}

	*v = NGRANRadioResourceStatusDLnonGBRPRBusageforMIMO(n)

	return nil
}

type NGRANRadioResourceStatusULnonGBRPRBusageforMIMO uint8

func (v NGRANRadioResourceStatusULnonGBRPRBusageforMIMO) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return per.EncodeInteger(w, enc, per.Bounds{LB: 0, HasLB: true, UB: 100, HasUB: true}, int64(v))
}

func (v *NGRANRadioResourceStatusULnonGBRPRBusageforMIMO) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	n, err := per.DecodeInteger(r, enc, per.Bounds{LB: 0, HasLB: true, UB: 100, HasUB: true})
	if err != nil {
		return err
	}

	*v = NGRANRadioResourceStatusULnonGBRPRBusageforMIMO(n)

	return nil
}

type NGRANRadioResourceStatusDLTotalPRBusageforMIMO uint8

func (v NGRANRadioResourceStatusDLTotalPRBusageforMIMO) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return per.EncodeInteger(w, enc, per.Bounds{LB: 0, HasLB: true, UB: 100, HasUB: true}, int64(v))
}

func (v *NGRANRadioResourceStatusDLTotalPRBusageforMIMO) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	n, err := per.DecodeInteger(r, enc, per.Bounds{LB: 0, HasLB: true, UB: 100, HasUB: true})
	if err != nil {
		return err
	}

	*v = NGRANRadioResourceStatusDLTotalPRBusageforMIMO(n)

	return nil
}

type NGRANRadioResourceStatusULTotalPRBusageforMIMO uint8

func (v NGRANRadioResourceStatusULTotalPRBusageforMIMO) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return per.EncodeInteger(w, enc, per.Bounds{LB: 0, HasLB: true, UB: 100, HasUB: true}, int64(v))
}

func (v *NGRANRadioResourceStatusULTotalPRBusageforMIMO) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	n, err := per.DecodeInteger(r, enc, per.Bounds{LB: 0, HasLB: true, UB: 100, HasUB: true})
	if err != nil {
		return err
	}

	*v = NGRANRadioResourceStatusULTotalPRBusageforMIMO(n)

	return nil
}

type NGRANRadioResourceStatus struct {
	DLGBRPRBusageforMIMO    NGRANRadioResourceStatusDLGBRPRBusageforMIMO
	ULGBRPRBusageforMIMO    NGRANRadioResourceStatusULGBRPRBusageforMIMO
	DLnonGBRPRBusageforMIMO NGRANRadioResourceStatusDLnonGBRPRBusageforMIMO
	ULnonGBRPRBusageforMIMO NGRANRadioResourceStatusULnonGBRPRBusageforMIMO
	DLTotalPRBusageforMIMO  NGRANRadioResourceStatusDLTotalPRBusageforMIMO
	ULTotalPRBusageforMIMO  NGRANRadioResourceStatusULTotalPRBusageforMIMO
	iEExtensions            extensionMeta          `per:",exttable"`
	extensions              per.ExtensionAdditions `per:",extraw"`
}

type CandidateCellID struct {
	CandidateCellID NRCGI
	iEExtensions    extensionMeta          `per:",exttable"`
	extensions      per.ExtensionAdditions `per:",extraw"`
}

type CandidatePCICandidatePCI int64

func (v CandidatePCICandidatePCI) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return per.EncodeInteger(w, enc, per.Bounds{LB: 0, HasLB: true, UB: 1007, HasUB: true, Extensible: true}, int64(v))
}

func (v *CandidatePCICandidatePCI) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	n, err := per.DecodeInteger(r, enc, per.Bounds{LB: 0, HasLB: true, UB: 1007, HasUB: true, Extensible: true})
	if err != nil {
		return err
	}

	*v = CandidatePCICandidatePCI(n)

	return nil
}

func (v CandidatePCICandidatePCI) Diagnose(*ap.Diagnosis[ProtocolIEID]) bool {
	return outsideRoot(int64(v), 0, 1007)
}

type CandidatePCICandidateNRARFCN uint32

func (v CandidatePCICandidateNRARFCN) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return per.EncodeInteger(w, enc, per.Bounds{LB: 0, HasLB: true, UB: maxNRARFCN, HasUB: true}, int64(v))
}

func (v *CandidatePCICandidateNRARFCN) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	n, err := per.DecodeInteger(r, enc, per.Bounds{LB: 0, HasLB: true, UB: maxNRARFCN, HasUB: true})
	if err != nil {
		return err
	}

	*v = CandidatePCICandidateNRARFCN(n)

	return nil
}

type CandidatePCI struct {
	CandidatePCI     CandidatePCICandidatePCI
	CandidateNRARFCN CandidatePCICandidateNRARFCN
	iEExtensions     extensionMeta          `per:",exttable"`
	extensions       per.ExtensionAdditions `per:",extraw"`
}

type CompositeAvailableCapacityCellCapacityClassValue int64

func (v CompositeAvailableCapacityCellCapacityClassValue) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return per.EncodeInteger(w, enc, per.Bounds{LB: 1, HasLB: true, UB: 100, HasUB: true, Extensible: true}, int64(v))
}

func (v *CompositeAvailableCapacityCellCapacityClassValue) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	n, err := per.DecodeInteger(r, enc, per.Bounds{LB: 1, HasLB: true, UB: 100, HasUB: true, Extensible: true})
	if err != nil {
		return err
	}

	*v = CompositeAvailableCapacityCellCapacityClassValue(n)

	return nil
}

func (v CompositeAvailableCapacityCellCapacityClassValue) Diagnose(*ap.Diagnosis[ProtocolIEID]) bool {
	return outsideRoot(int64(v), 1, 100)
}

type CompositeAvailableCapacityCapacityValue uint8

func (v CompositeAvailableCapacityCapacityValue) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return per.EncodeInteger(w, enc, per.Bounds{LB: 0, HasLB: true, UB: 100, HasUB: true}, int64(v))
}

func (v *CompositeAvailableCapacityCapacityValue) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	n, err := per.DecodeInteger(r, enc, per.Bounds{LB: 0, HasLB: true, UB: 100, HasUB: true})
	if err != nil {
		return err
	}

	*v = CompositeAvailableCapacityCapacityValue(n)

	return nil
}

type CompositeAvailableCapacity struct {
	CellCapacityClassValue *CompositeAvailableCapacityCellCapacityClassValue `per:",optional"`
	CapacityValue          CompositeAvailableCapacityCapacityValue
	iEExtensions           extensionMeta          `per:",exttable"`
	extensions             per.ExtensionAdditions `per:",extraw"`
}
