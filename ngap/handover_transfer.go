// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"github.com/ellanetworks/core/per"
)

// The §9.3.4 transfers the handover and path switch procedures carry inside
// their PDU session lists. TS 36.413 has no counterpart: S1AP carries the
// forwarding tunnels as plain IEs in the message body rather than in a nested
// PER encoding.

// DirectForwardingPathAvailability ::= ENUMERATED { direct-path-available,
// ... }.
type DirectForwardingPathAvailability uint8

const (
	DirectForwardingPathAvailable DirectForwardingPathAvailability = iota

	directForwardingPathAvailabilityRootCount = 1
)

func (d DirectForwardingPathAvailability) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return encodeRootEnumerated(w, enc, directForwardingPathAvailabilityRootCount, int64(d), "DirectForwardingPathAvailability")
}

func (d *DirectForwardingPathAvailability) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	idx, err := decodeRootEnumerated(r, enc, directForwardingPathAvailabilityRootCount, "DirectForwardingPathAvailability")
	if err != nil {
		return err
	}

	*d = DirectForwardingPathAvailability(idx)

	return nil
}

// DL-NGU-TNLInformationReused ::= ENUMERATED { true, ... }. Signals that the
// NG-RAN node kept the downlink tunnel it already had.
type DLNGUTNLInformationReused uint8

const (
	DLNGUTNLInformationReusedTrue DLNGUTNLInformationReused = iota

	dlNGUTNLInformationReusedRootCount = 1
)

func (d DLNGUTNLInformationReused) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return encodeRootEnumerated(w, enc, dlNGUTNLInformationReusedRootCount, int64(d), "DL-NGU-TNLInformationReused")
}

func (d *DLNGUTNLInformationReused) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	idx, err := decodeRootEnumerated(r, enc, dlNGUTNLInformationReusedRootCount, "DL-NGU-TNLInformationReused")
	if err != nil {
		return err
	}

	*d = DLNGUTNLInformationReused(idx)

	return nil
}

// UserPlaneSecurityInformation ::= SEQUENCE { securityResult,
// securityIndication, iE-Extensions OPTIONAL } (extensible) — TS 38.413
// §9.3.1.87. Both fields are mandatory: the NG-RAN node reports what it applied
// alongside what it was asked for.
type UserPlaneSecurityInformation struct {
	_                  [0]struct{} `per:"extseq"`
	SecurityResult     SecurityResult
	SecurityIndication SecurityIndication
	_                  ieExtensions `per:",skip"`
}

// HandoverCommandTransfer ::= SEQUENCE { dLForwardingUP-TNLInformation
// OPTIONAL, qosFlowToBeForwardedList OPTIONAL, dataForwardingResponseDRBList
// OPTIONAL, iE-Extensions OPTIONAL } (extensible) — TS 38.413 §9.3.4.10.
type HandoverCommandTransfer struct {
	_                            [0]struct{}                   `per:"extseq"`
	DLForwardingUPTNLInformation *UPTransportLayerInformation  `per:",optional"`
	QosFlowToBeForwarded         QosFlowToBeForwardedList      `per:",optional"`
	DataForwardingResponseDRB    DataForwardingResponseDRBList `per:",optional"`
	_                            ieExtensions                  `per:",skip"`
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
	var t HandoverCommandTransfer

	if err := t.UnmarshalPER(per.NewReader(b), per.Aligned); err != nil {
		return nil, err
	}

	return &t, nil
}

// HandoverPreparationUnsuccessfulTransfer ::= SEQUENCE { cause, iE-Extensions
// OPTIONAL } (extensible) — TS 38.413 §9.3.4.18.
type HandoverPreparationUnsuccessfulTransfer struct {
	_     [0]struct{} `per:"extseq"`
	Cause Cause
	_     ieExtensions `per:",skip"`
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
	var t HandoverPreparationUnsuccessfulTransfer

	if err := t.UnmarshalPER(per.NewReader(b), per.Aligned); err != nil {
		return nil, err
	}

	return &t, nil
}

// HandoverRequestAcknowledgeTransfer ::= SEQUENCE { dL-NGU-UP-TNLInformation,
// dLForwardingUP-TNLInformation OPTIONAL, securityResult OPTIONAL,
// qosFlowSetupResponseList, qosFlowFailedToSetupList OPTIONAL,
// dataForwardingResponseDRBList OPTIONAL, iE-Extensions OPTIONAL }
// (extensible) — TS 38.413 §9.3.4.11.
type HandoverRequestAcknowledgeTransfer struct {
	_                            [0]struct{} `per:"extseq"`
	DLNGUUPTNLInformation        UPTransportLayerInformation
	DLForwardingUPTNLInformation *UPTransportLayerInformation `per:",optional"`
	SecurityResult               *SecurityResult              `per:",optional"`
	QosFlowSetupResponse         QosFlowListWithDataForwarding
	QosFlowFailedToSetup         QosFlowListWithCause          `per:",optional"`
	DataForwardingResponseDRB    DataForwardingResponseDRBList `per:",optional"`
	_                            ieExtensions                  `per:",skip"`
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
	var t HandoverRequestAcknowledgeTransfer

	if err := t.UnmarshalPER(per.NewReader(b), per.Aligned); err != nil {
		return nil, err
	}

	return &t, nil
}

// HandoverResourceAllocationUnsuccessfulTransfer ::= SEQUENCE { cause,
// criticalityDiagnostics OPTIONAL, iE-Extensions OPTIONAL } (extensible) —
// TS 38.413 §9.3.4.17.
type HandoverResourceAllocationUnsuccessfulTransfer struct {
	_                      [0]struct{} `per:"extseq"`
	Cause                  Cause
	CriticalityDiagnostics *CriticalityDiagnostics `per:",optional"`
	_                      ieExtensions            `per:",skip"`
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
	var t HandoverResourceAllocationUnsuccessfulTransfer

	if err := t.UnmarshalPER(per.NewReader(b), per.Aligned); err != nil {
		return nil, err
	}

	return &t, nil
}

// HandoverRequiredTransfer ::= SEQUENCE { directForwardingPathAvailability
// OPTIONAL, iE-Extensions OPTIONAL } (extensible) — TS 38.413 §9.3.4.12.
type HandoverRequiredTransfer struct {
	_                                [0]struct{}                       `per:"extseq"`
	DirectForwardingPathAvailability *DirectForwardingPathAvailability `per:"ENUMERATED,range:0..0,...,optional"`
	_                                ieExtensions                      `per:",skip"`
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
	var t HandoverRequiredTransfer

	if err := t.UnmarshalPER(per.NewReader(b), per.Aligned); err != nil {
		return nil, err
	}

	return &t, nil
}

// PathSwitchRequestTransfer ::= SEQUENCE { dL-NGU-UP-TNLInformation,
// dL-NGU-TNLInformationReused OPTIONAL, userPlaneSecurityInformation OPTIONAL,
// qosFlowAcceptedList, iE-Extensions OPTIONAL } (extensible) — TS 38.413
// §9.3.4.8.
type PathSwitchRequestTransfer struct {
	_                            [0]struct{} `per:"extseq"`
	DLNGUUPTNLInformation        UPTransportLayerInformation
	DLNGUTNLInformationReused    *DLNGUTNLInformationReused    `per:"ENUMERATED,range:0..0,...,optional"`
	UserPlaneSecurityInformation *UserPlaneSecurityInformation `per:",optional"`
	QosFlowAccepted              QosFlowAcceptedList
	_                            ieExtensions `per:",skip"`
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
	var t PathSwitchRequestTransfer

	if err := t.UnmarshalPER(per.NewReader(b), per.Aligned); err != nil {
		return nil, err
	}

	return &t, nil
}

// PathSwitchRequestAcknowledgeTransfer ::= SEQUENCE { uL-NGU-UP-TNLInformation
// OPTIONAL, securityIndication OPTIONAL, iE-Extensions OPTIONAL }
// (extensible) — TS 38.413 §9.3.4.9.
type PathSwitchRequestAcknowledgeTransfer struct {
	_                     [0]struct{}                  `per:"extseq"`
	ULNGUUPTNLInformation *UPTransportLayerInformation `per:",optional"`
	SecurityIndication    *SecurityIndication          `per:",optional"`
	_                     ieExtensions                 `per:",skip"`
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
	var t PathSwitchRequestAcknowledgeTransfer

	if err := t.UnmarshalPER(per.NewReader(b), per.Aligned); err != nil {
		return nil, err
	}

	return &t, nil
}

// PathSwitchRequestSetupFailedTransfer ::= SEQUENCE { cause, iE-Extensions
// OPTIONAL } (extensible) — TS 38.413 §9.3.4.19.
type PathSwitchRequestSetupFailedTransfer struct {
	_     [0]struct{} `per:"extseq"`
	Cause Cause
	_     ieExtensions `per:",skip"`
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
	var t PathSwitchRequestSetupFailedTransfer

	if err := t.UnmarshalPER(per.NewReader(b), per.Aligned); err != nil {
		return nil, err
	}

	return &t, nil
}

// PathSwitchRequestUnsuccessfulTransfer ::= SEQUENCE { cause, iE-Extensions
// OPTIONAL } (extensible) — TS 38.413 §9.3.4.7.
type PathSwitchRequestUnsuccessfulTransfer struct {
	_     [0]struct{} `per:"extseq"`
	Cause Cause
	_     ieExtensions `per:",skip"`
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
	var t PathSwitchRequestUnsuccessfulTransfer

	if err := t.UnmarshalPER(per.NewReader(b), per.Aligned); err != nil {
		return nil, err
	}

	return &t, nil
}
