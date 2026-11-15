// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"github.com/ellanetworks/core/per"
)

// The §9.3.4 transfers the handover and path switch procedures carry inside
// their PDU session lists. TS 36.413 has no counterpart: S1AP carries the
// forwarding tunnels as plain IEs in the message body and not in a nested
// PER encoding.

// DirectForwardingPathAvailability ::= ENUMERATED { direct-path-available,
// ... }.
type DirectForwardingPathAvailability uint8

const (
	DirectForwardingPathAvailable DirectForwardingPathAvailability = iota
)

// DL-NGU-TNLInformationReused ::= ENUMERATED { true, ... }. Signals that the
// NG-RAN node kept the downlink tunnel it already had.
type DLNGUTNLInformationReused uint8

const (
	DLNGUTNLInformationReusedTrue DLNGUTNLInformationReused = iota
)

// UserPlaneSecurityInformation ::= SEQUENCE { securityResult,
// securityIndication, iE-Extensions OPTIONAL } (extensible) — TS 38.413
// §9.3.1.60. Both fields are mandatory: the NG-RAN node reports what it applied
// alongside what it was asked for.
type UserPlaneSecurityInformation struct {
	SecurityResult     SecurityResult
	SecurityIndication SecurityIndication
	iEExtensions       extensionMeta          `per:",exttable"`
	extensions         per.ExtensionAdditions `per:",extraw"`
}

// HandoverCommandTransfer ::= SEQUENCE { dLForwardingUP-TNLInformation
// OPTIONAL, qosFlowToBeForwardedList OPTIONAL, dataForwardingResponseDRBList
// OPTIONAL, iE-Extensions OPTIONAL } (extensible) — TS 38.413 §9.3.4.10.
type HandoverCommandTransfer struct {
	DLForwardingUPTNLInformation           *UPTransportLayerInformation    `per:",optional"`
	QosFlowToBeForwarded                   QosFlowToBeForwardedList        `per:",optional"`
	DataForwardingResponseDRB              DataForwardingResponseDRBList   `per:",optional"`
	AdditionalDLForwardingUPTNLInformation QosFlowPerTNLInformationList    `per:",extie:IDAdditionalDLForwardingUPTNLInformation,crit:ignore"`
	ULForwardingUPTNLInformation           *UPTransportLayerInformation    `per:",extie:IDULForwardingUPTNLInformation,crit:reject"`
	AdditionalULForwardingUPTNLInformation UPTransportLayerInformationList `per:",extie:IDAdditionalULForwardingUPTNLInformation,crit:reject"`
	DataForwardingResponseERAB             DataForwardingResponseERABList  `per:",extie:IDDataForwardingResponseERABList,crit:ignore"`
	QosFlowFailedToSetupList               QosFlowListWithCause            `per:",extie:IDQosFlowFailedToSetupList,crit:ignore"`
	iEExtensions                           extensionMeta                   `per:",exttable"`
	extensions                             per.ExtensionAdditions          `per:",extraw"`
}

// Marshal encodes the transfer for the OCTET STRING that carries it.
func (t *HandoverCommandTransfer) Marshal() (TransferContainer, error) {
	w := per.NewWriter()

	if err := t.MarshalPER(w, per.Aligned); err != nil {
		return nil, err
	}

	w.AlignToByte()

	return TransferContainer(w.Bytes()), nil
}

// ParseHandoverCommandTransfer decodes the transfer the SMF returns naming the
// tunnels the source node may forward on.
func ParseHandoverCommandTransfer(b TransferContainer) (*HandoverCommandTransfer, error) {
	return parseTransfer[HandoverCommandTransfer](b)
}

func DecodeHandoverCommandTransfer(b TransferContainer) (*HandoverCommandTransfer, error) {
	return decodeTransfer[HandoverCommandTransfer](b)
}

func (t *HandoverCommandTransfer) Diagnostics() Diagnostics { return diagnosticsOf(t) }

// HandoverPreparationUnsuccessfulTransfer ::= SEQUENCE { cause, iE-Extensions
// OPTIONAL } (extensible) — TS 38.413 §9.3.4.18.
type HandoverPreparationUnsuccessfulTransfer struct {
	extensions   per.ExtensionAdditions `per:",extraw"`
	Cause        Cause
	iEExtensions extensionMeta `per:",exttable"`
}

// Marshal encodes the transfer for the OCTET STRING that carries it.
func (t *HandoverPreparationUnsuccessfulTransfer) Marshal() (TransferContainer, error) {
	w := per.NewWriter()

	if err := t.MarshalPER(w, per.Aligned); err != nil {
		return nil, err
	}

	w.AlignToByte()

	return TransferContainer(w.Bytes()), nil
}

// ParseHandoverPreparationUnsuccessfulTransfer decodes the transfer naming why
// a session could not be handed over.
func ParseHandoverPreparationUnsuccessfulTransfer(b TransferContainer) (*HandoverPreparationUnsuccessfulTransfer, error) {
	return parseTransfer[HandoverPreparationUnsuccessfulTransfer](b)
}

func DecodeHandoverPreparationUnsuccessfulTransfer(b TransferContainer) (*HandoverPreparationUnsuccessfulTransfer, error) {
	return decodeTransfer[HandoverPreparationUnsuccessfulTransfer](b)
}

func (t *HandoverPreparationUnsuccessfulTransfer) Diagnostics() Diagnostics { return diagnosticsOf(t) }

// HandoverRequestAcknowledgeTransfer ::= SEQUENCE { dL-NGU-UP-TNLInformation,
// dLForwardingUP-TNLInformation OPTIONAL, securityResult OPTIONAL,
// qosFlowSetupResponseList, qosFlowFailedToSetupList OPTIONAL,
// dataForwardingResponseDRBList OPTIONAL, iE-Extensions OPTIONAL }
// (extensible) — TS 38.413 §9.3.4.11.
type HandoverRequestAcknowledgeTransfer struct {
	DLNGUUPTNLInformation                            UPTransportLayerInformation
	DLForwardingUPTNLInformation                     *UPTransportLayerInformation `per:",optional"`
	SecurityResult                                   *SecurityResult              `per:",optional"`
	QosFlowSetupResponse                             QosFlowListWithDataForwarding
	QosFlowFailedToSetup                             QosFlowListWithCause                             `per:",optional"`
	DataForwardingResponseDRB                        DataForwardingResponseDRBList                    `per:",optional"`
	AdditionalDLUPTNLInformationForHO                AdditionalDLUPTNLInformationForHOList            `per:",extie:IDAdditionalDLUPTNLInformationForHOList,crit:ignore"`
	ULForwardingUPTNLInformation                     *UPTransportLayerInformation                     `per:",extie:IDULForwardingUPTNLInformation,crit:reject"`
	AdditionalULForwardingUPTNLInformation           UPTransportLayerInformationList                  `per:",extie:IDAdditionalULForwardingUPTNLInformation,crit:reject"`
	DataForwardingResponseERAB                       DataForwardingResponseERABList                   `per:",extie:IDDataForwardingResponseERABList,crit:ignore"`
	RedundantDLNGUUPTNLInformation                   *UPTransportLayerInformation                     `per:",extie:IDRedundantDLNGUUPTNLInformation,crit:ignore"`
	UsedRSNInformation                               *RedundantPDUSessionInformation                  `per:",extie:IDUsedRSNInformation,crit:ignore"`
	GlobalRANNodeID                                  *GlobalRANNodeID                                 `per:",extie:IDGlobalRANNodeID,crit:ignore"`
	MBSSupportIndicator                              *MBSSupportIndicator                             `per:",extie:IDMBSSupportIndicator,crit:ignore"`
	PDUSetbasedHandlingIndicator                     *PDUSetbasedHandlingIndicator                    `per:",extie:IDPDUSetbasedHandlingIndicator,crit:ignore"`
	ECNMarkingorCongestionInformationReportingStatus ECNMarkingorCongestionInformationReportingStatus `per:",extie:IDECNMarkingorCongestionInformationReportingStatus,crit:ignore"`
	iEExtensions                                     extensionMeta                                    `per:",exttable"`
	extensions                                       per.ExtensionAdditions                           `per:",extraw"`
}

// Marshal encodes the transfer for the OCTET STRING that carries it.
func (t *HandoverRequestAcknowledgeTransfer) Marshal() (TransferContainer, error) {
	w := per.NewWriter()

	if err := t.MarshalPER(w, per.Aligned); err != nil {
		return nil, err
	}

	w.AlignToByte()

	return TransferContainer(w.Bytes()), nil
}

// ParseHandoverRequestAcknowledgeTransfer decodes the transfer the target
// NG-RAN node returns for a session it admitted.
func ParseHandoverRequestAcknowledgeTransfer(b TransferContainer) (*HandoverRequestAcknowledgeTransfer, error) {
	return parseTransfer[HandoverRequestAcknowledgeTransfer](b)
}

func DecodeHandoverRequestAcknowledgeTransfer(b TransferContainer) (*HandoverRequestAcknowledgeTransfer, error) {
	return decodeTransfer[HandoverRequestAcknowledgeTransfer](b)
}

func (t *HandoverRequestAcknowledgeTransfer) Diagnostics() Diagnostics { return diagnosticsOf(t) }

// HandoverResourceAllocationUnsuccessfulTransfer ::= SEQUENCE { cause,
// criticalityDiagnostics OPTIONAL, iE-Extensions OPTIONAL } (extensible) —
// TS 38.413 §9.3.4.19.
type HandoverResourceAllocationUnsuccessfulTransfer struct {
	extensions             per.ExtensionAdditions `per:",extraw"`
	Cause                  Cause
	CriticalityDiagnostics *CriticalityDiagnostics `per:",optional"`
	iEExtensions           extensionMeta           `per:",exttable"`
}

// Marshal encodes the transfer for the OCTET STRING that carries it.
func (t *HandoverResourceAllocationUnsuccessfulTransfer) Marshal() (TransferContainer, error) {
	w := per.NewWriter()

	if err := t.MarshalPER(w, per.Aligned); err != nil {
		return nil, err
	}

	w.AlignToByte()

	return TransferContainer(w.Bytes()), nil
}

// ParseHandoverResourceAllocationUnsuccessfulTransfer decodes the transfer the
// target NG-RAN node returns for a session it could not admit.
func ParseHandoverResourceAllocationUnsuccessfulTransfer(b TransferContainer) (*HandoverResourceAllocationUnsuccessfulTransfer, error) {
	return parseTransfer[HandoverResourceAllocationUnsuccessfulTransfer](b)
}

func DecodeHandoverResourceAllocationUnsuccessfulTransfer(b TransferContainer) (*HandoverResourceAllocationUnsuccessfulTransfer, error) {
	return decodeTransfer[HandoverResourceAllocationUnsuccessfulTransfer](b)
}

func (t *HandoverResourceAllocationUnsuccessfulTransfer) Diagnostics() Diagnostics {
	return diagnosticsOf(t)
}

// HandoverRequiredTransfer ::= SEQUENCE { directForwardingPathAvailability
// OPTIONAL, iE-Extensions OPTIONAL } (extensible) — TS 38.413 §9.3.4.14.
type HandoverRequiredTransfer struct {
	extensions                       per.ExtensionAdditions            `per:",extraw"`
	DirectForwardingPathAvailability *DirectForwardingPathAvailability `per:",optional"`
	iEExtensions                     extensionMeta                     `per:",exttable"`
}

// Marshal encodes the transfer for the OCTET STRING that carries it.
func (t *HandoverRequiredTransfer) Marshal() (TransferContainer, error) {
	w := per.NewWriter()

	if err := t.MarshalPER(w, per.Aligned); err != nil {
		return nil, err
	}

	w.AlignToByte()

	return TransferContainer(w.Bytes()), nil
}

// ParseHandoverRequiredTransfer decodes the transfer the source NG-RAN node
// sends per session when it asks for a handover.
func ParseHandoverRequiredTransfer(b TransferContainer) (*HandoverRequiredTransfer, error) {
	return parseTransfer[HandoverRequiredTransfer](b)
}

func DecodeHandoverRequiredTransfer(b TransferContainer) (*HandoverRequiredTransfer, error) {
	return decodeTransfer[HandoverRequiredTransfer](b)
}

func (t *HandoverRequiredTransfer) Diagnostics() Diagnostics { return diagnosticsOf(t) }

// PathSwitchRequestTransfer ::= SEQUENCE { dL-NGU-UP-TNLInformation,
// dL-NGU-TNLInformationReused OPTIONAL, userPlaneSecurityInformation OPTIONAL,
// qosFlowAcceptedList, iE-Extensions OPTIONAL } (extensible) — TS 38.413
// §9.3.4.8.
type PathSwitchRequestTransfer struct {
	DLNGUUPTNLInformation                            UPTransportLayerInformation
	DLNGUTNLInformationReused                        *DLNGUTNLInformationReused    `per:",optional"`
	UserPlaneSecurityInformation                     *UserPlaneSecurityInformation `per:",optional"`
	QosFlowAccepted                                  QosFlowAcceptedList
	AdditionalDLQosFlowPerTNLInformation             QosFlowPerTNLInformationList                     `per:",extie:IDAdditionalDLQosFlowPerTNLInformation,crit:ignore"`
	RedundantDLNGUUPTNLInformation                   *UPTransportLayerInformation                     `per:",extie:IDRedundantDLNGUUPTNLInformation,crit:ignore"`
	RedundantDLNGUTNLInformationReused               *DLNGUTNLInformationReused                       `per:",extie:IDRedundantDLNGUTNLInformationReused,crit:ignore"`
	AdditionalRedundantDLQosFlowPerTNLInformation    QosFlowPerTNLInformationList                     `per:",extie:IDAdditionalRedundantDLQosFlowPerTNLInformation,crit:ignore"`
	UsedRSNInformation                               *RedundantPDUSessionInformation                  `per:",extie:IDUsedRSNInformation,crit:ignore"`
	GlobalRANNodeID                                  *GlobalRANNodeID                                 `per:",extie:IDGlobalRANNodeID,crit:ignore"`
	MBSSupportIndicator                              *MBSSupportIndicator                             `per:",extie:IDMBSSupportIndicator,crit:ignore"`
	PDUSetbasedHandlingIndicator                     *PDUSetbasedHandlingIndicator                    `per:",extie:IDPDUSetbasedHandlingIndicator,crit:ignore"`
	ECNMarkingorCongestionInformationReportingStatus ECNMarkingorCongestionInformationReportingStatus `per:",extie:IDECNMarkingorCongestionInformationReportingStatus,crit:ignore"`
	iEExtensions                                     extensionMeta                                    `per:",exttable"`
	extensions                                       per.ExtensionAdditions                           `per:",extraw"`
}

// Marshal encodes the transfer for the OCTET STRING that carries it.
func (t *PathSwitchRequestTransfer) Marshal() (TransferContainer, error) {
	w := per.NewWriter()

	if err := t.MarshalPER(w, per.Aligned); err != nil {
		return nil, err
	}

	w.AlignToByte()

	return TransferContainer(w.Bytes()), nil
}

// ParsePathSwitchRequestTransfer decodes the transfer the target NG-RAN node
// sends per session when it takes over the downlink tunnel.
func ParsePathSwitchRequestTransfer(b TransferContainer) (*PathSwitchRequestTransfer, error) {
	return parseTransfer[PathSwitchRequestTransfer](b)
}

func DecodePathSwitchRequestTransfer(b TransferContainer) (*PathSwitchRequestTransfer, error) {
	return decodeTransfer[PathSwitchRequestTransfer](b)
}

func (t *PathSwitchRequestTransfer) Diagnostics() Diagnostics { return diagnosticsOf(t) }

// PathSwitchRequestAcknowledgeTransfer ::= SEQUENCE { uL-NGU-UP-TNLInformation
// OPTIONAL, securityIndication OPTIONAL, iE-Extensions OPTIONAL }
// (extensible) — TS 38.413 §9.3.4.9.
type PathSwitchRequestAcknowledgeTransfer struct {
	ULNGUUPTNLInformation                  *UPTransportLayerInformation        `per:",optional"`
	SecurityIndication                     *SecurityIndication                 `per:",optional"`
	AdditionalNGUUPTNLInformation          UPTransportLayerInformationPairList `per:",extie:IDAdditionalNGUUPTNLInformation,crit:ignore"`
	RedundantULNGUUPTNLInformation         *UPTransportLayerInformation        `per:",extie:IDRedundantULNGUUPTNLInformation,crit:ignore"`
	AdditionalRedundantNGUUPTNLInformation UPTransportLayerInformationPairList `per:",extie:IDAdditionalRedundantNGUUPTNLInformation,crit:ignore"`
	QosFlowParametersList                  QosFlowParametersList               `per:",extie:IDQosFlowParametersList,crit:ignore"`
	iEExtensions                           extensionMeta                       `per:",exttable"`
	extensions                             per.ExtensionAdditions              `per:",extraw"`
}

// Marshal encodes the transfer for the OCTET STRING that carries it.
func (t *PathSwitchRequestAcknowledgeTransfer) Marshal() (TransferContainer, error) {
	w := per.NewWriter()

	if err := t.MarshalPER(w, per.Aligned); err != nil {
		return nil, err
	}

	w.AlignToByte()

	return TransferContainer(w.Bytes()), nil
}

// ParsePathSwitchRequestAcknowledgeTransfer decodes the transfer the SMF
// returns for a session whose path was switched.
func ParsePathSwitchRequestAcknowledgeTransfer(b TransferContainer) (*PathSwitchRequestAcknowledgeTransfer, error) {
	return parseTransfer[PathSwitchRequestAcknowledgeTransfer](b)
}

func DecodePathSwitchRequestAcknowledgeTransfer(b TransferContainer) (*PathSwitchRequestAcknowledgeTransfer, error) {
	return decodeTransfer[PathSwitchRequestAcknowledgeTransfer](b)
}

func (t *PathSwitchRequestAcknowledgeTransfer) Diagnostics() Diagnostics { return diagnosticsOf(t) }

// PathSwitchRequestSetupFailedTransfer ::= SEQUENCE { cause, iE-Extensions
// OPTIONAL } (extensible) — TS 38.413 §9.3.4.15.
type PathSwitchRequestSetupFailedTransfer struct {
	extensions   per.ExtensionAdditions `per:",extraw"`
	Cause        Cause
	iEExtensions extensionMeta `per:",exttable"`
}

// Marshal encodes the transfer for the OCTET STRING that carries it.
func (t *PathSwitchRequestSetupFailedTransfer) Marshal() (TransferContainer, error) {
	w := per.NewWriter()

	if err := t.MarshalPER(w, per.Aligned); err != nil {
		return nil, err
	}

	w.AlignToByte()

	return TransferContainer(w.Bytes()), nil
}

// ParsePathSwitchRequestSetupFailedTransfer decodes the transfer the SMF
// returns for a session it could not switch.
func ParsePathSwitchRequestSetupFailedTransfer(b TransferContainer) (*PathSwitchRequestSetupFailedTransfer, error) {
	return parseTransfer[PathSwitchRequestSetupFailedTransfer](b)
}

func DecodePathSwitchRequestSetupFailedTransfer(b TransferContainer) (*PathSwitchRequestSetupFailedTransfer, error) {
	return decodeTransfer[PathSwitchRequestSetupFailedTransfer](b)
}

func (t *PathSwitchRequestSetupFailedTransfer) Diagnostics() Diagnostics { return diagnosticsOf(t) }

// PathSwitchRequestUnsuccessfulTransfer ::= SEQUENCE { cause, iE-Extensions
// OPTIONAL } (extensible) — TS 38.413 §9.3.4.20.
type PathSwitchRequestUnsuccessfulTransfer struct {
	extensions   per.ExtensionAdditions `per:",extraw"`
	Cause        Cause
	iEExtensions extensionMeta `per:",exttable"`
}

// Marshal encodes the transfer for the OCTET STRING that carries it.
func (t *PathSwitchRequestUnsuccessfulTransfer) Marshal() (TransferContainer, error) {
	w := per.NewWriter()

	if err := t.MarshalPER(w, per.Aligned); err != nil {
		return nil, err
	}

	w.AlignToByte()

	return TransferContainer(w.Bytes()), nil
}

// ParsePathSwitchRequestUnsuccessfulTransfer decodes the transfer naming why a
// session's path could not be switched.
func ParsePathSwitchRequestUnsuccessfulTransfer(b TransferContainer) (*PathSwitchRequestUnsuccessfulTransfer, error) {
	return parseTransfer[PathSwitchRequestUnsuccessfulTransfer](b)
}

func DecodePathSwitchRequestUnsuccessfulTransfer(b TransferContainer) (*PathSwitchRequestUnsuccessfulTransfer, error) {
	return decodeTransfer[PathSwitchRequestUnsuccessfulTransfer](b)
}

func (t *PathSwitchRequestUnsuccessfulTransfer) Diagnostics() Diagnostics { return diagnosticsOf(t) }
