// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package decode

import (
	"github.com/free5gc/aper"
	"github.com/free5gc/ngap/ngapType"
)

type RRCEstablishmentCause aper.Enumerated

type UserLocationKind uint8

const (
	UserLocationKindUnknown UserLocationKind = iota
	UserLocationKindNR
	UserLocationKindEUTRA
	UserLocationKindN3IWF
)

// UserLocationInformation wraps a validated UserLocationInformation CHOICE. When
// Kind is not Unknown, raw and the variant pointer matching Kind are both
// non-nil. Raw aliases the source PDU buffer and must be consumed within the
// synchronous handler invocation.
type UserLocationInformation struct {
	kind UserLocationKind
	raw  *ngapType.UserLocationInformation
}

func (u UserLocationInformation) Kind() UserLocationKind                 { return u.kind }
func (u UserLocationInformation) Raw() *ngapType.UserLocationInformation { return u.raw }

type FiveGSTMSI struct {
	AMFSetID   aper.BitString
	AMFPointer aper.BitString
	FiveGTMSI  []byte
}

// InitialUEMessage is a decoded NGAP InitialUEMessage (3GPP TS 38.413).
// Non-pointer fields are mandatory; pointer fields are optional and nil when
// absent.
type InitialUEMessage struct {
	RANUENGAPID             int64
	NASPDU                  []byte
	UserLocationInformation UserLocationInformation
	RRCEstablishmentCause   RRCEstablishmentCause
	FiveGSTMSI              *FiveGSTMSI
	UEContextRequest        bool
}

// PathSwitchRequest is a decoded NGAP PathSwitchRequest (3GPP TS 38.413).
// RANUENGAPID, SourceAMFUENGAPID and PDUSessionResourceItems are
// mandatory-reject. UserLocationInformation and UESecurityCapabilities are
// mandatory-ignore: a missing or malformed value yields a zero-value field.
// FailedToSetupItems is optional and may be nil.
//
// PDUSessionResourceItems and FailedToSetupItems alias the source PDU buffer
// (PathSwitchRequest{Transfer,SetupFailedTransfer} octet strings) and must be
// consumed within the synchronous handler invocation. PDUSessionResourceItems
// may be structurally empty on a non-fatal decode: TS 38.413 sizeLB:1 forbids
// it but the decoder does not enforce sizeLB.
type PathSwitchRequest struct {
	RANUENGAPID             int64
	SourceAMFUENGAPID       int64
	UserLocationInformation UserLocationInformation
	UESecurityCapabilities  *ngapType.UESecurityCapabilities
	PDUSessionResourceItems []ngapType.PDUSessionResourceToBeSwitchedDLItem
	FailedToSetupItems      []ngapType.PDUSessionResourceFailedToSetupItemPSReq
}

// HandoverRequired is a decoded NGAP HandoverRequired (3GPP TS 38.413, Handover
// Preparation). All fields except Cause are mandatory-reject. Cause is
// mandatory-ignore, yielding a zero-value Cause (Present == 0) when missing or
// malformed.
//
// TargetID, PDUSessionResourceItems and SourceToTargetTransparentContainer
// alias the source PDU buffer and must be consumed within the synchronous
// handler invocation.
type HandoverRequired struct {
	AMFUENGAPID                        int64
	RANUENGAPID                        int64
	HandoverType                       ngapType.HandoverType
	Cause                              ngapType.Cause
	TargetID                           *ngapType.TargetID
	PDUSessionResourceItems            []ngapType.PDUSessionResourceItemHORqd
	SourceToTargetTransparentContainer ngapType.SourceToTargetTransparentContainer
}

// InitialContextSetupResponse is a decoded NGAP InitialContextSetupResponse
// (3GPP TS 38.413). AMFUENGAPID and RANUENGAPID are mandatory-ignore, left zero
// on a missing or malformed value. SetupItems and FailedToSetupItems are
// optional and may be nil; both alias the source PDU buffer
// (PDUSessionResourceSetupResponseTransfer /
// PDUSessionResourceSetupUnsuccessfulTransfer octet strings) and must be
// consumed within the synchronous handler invocation.
type InitialContextSetupResponse struct {
	AMFUENGAPID        int64
	RANUENGAPID        int64
	SetupItems         []ngapType.PDUSessionResourceSetupItemCxtRes
	FailedToSetupItems []ngapType.PDUSessionResourceFailedToSetupItemCxtRes
}

// UEContextReleaseRequest is a decoded NGAP UEContextReleaseRequest
// (3GPP TS 38.413). AMFUENGAPID and RANUENGAPID are mandatory-reject. Cause is
// mandatory-ignore (nil when absent or malformed). PDUSessionResourceList is
// optional-reject: nil when the IE is absent, non-nil (possibly empty) when the
// IE is present, distinguishing "no IE" from "IE present, no items".
//
// PDUSessionResourceList aliases the source PDU buffer and must be consumed
// within the synchronous handler invocation.
type UEContextReleaseRequest struct {
	AMFUENGAPID            int64
	RANUENGAPID            int64
	PDUSessionResourceList []ngapType.PDUSessionResourceItemCxtRelReq
	Cause                  *ngapType.Cause
}

// HandoverCancel is a decoded NGAP HandoverCancel (3GPP TS 38.413). AMFUENGAPID
// and RANUENGAPID are mandatory-reject; Cause is mandatory-ignore (nil when
// absent or malformed).
type HandoverCancel struct {
	AMFUENGAPID int64
	RANUENGAPID int64
	Cause       *ngapType.Cause
}

// UERadioCapabilityInfoIndication is a decoded NGAP
// UERadioCapabilityInfoIndication (3GPP TS 38.413). AMFUENGAPID and RANUENGAPID
// are mandatory-reject. UERadioCapability is mandatory-ignore (nil when absent
// or malformed). UERadioCapabilityForPaging is optional-ignore.
//
// UERadioCapability and UERadioCapabilityForPaging alias the source PDU buffer
// and must be consumed within the synchronous handler invocation.
type UERadioCapabilityInfoIndication struct {
	AMFUENGAPID                int64
	RANUENGAPID                int64
	UERadioCapability          []byte
	UERadioCapabilityForPaging *ngapType.UERadioCapabilityForPaging
}

// InitialContextSetupFailure is a decoded NGAP InitialContextSetupFailure
// (3GPP TS 38.413). AMFUENGAPID and RANUENGAPID are mandatory-reject. Cause is
// mandatory-ignore, yielding a zero-value Cause when absent or malformed.
// PDUSessionResourceFailedToSetupItems is optional-ignore and may be nil;
// CriticalityDiagnostics is optional-ignore and not surfaced.
//
// PDUSessionResourceFailedToSetupItems aliases the source PDU buffer
// (PDUSessionResourceSetupUnsuccessfulTransfer octet strings) and must be
// consumed within the synchronous handler invocation.
type InitialContextSetupFailure struct {
	AMFUENGAPID                          int64
	RANUENGAPID                          int64
	Cause                                ngapType.Cause
	PDUSessionResourceFailedToSetupItems []ngapType.PDUSessionResourceFailedToSetupItemCxtFail
}

// HandoverFailure is a decoded NGAP HandoverFailure (3GPP TS 38.413).
// AMFUENGAPID is mandatory-reject. Cause is mandatory-ignore (nil when absent).
// CriticalityDiagnostics is optional-ignore.
type HandoverFailure struct {
	AMFUENGAPID            int64
	Cause                  *ngapType.Cause
	CriticalityDiagnostics *ngapType.CriticalityDiagnostics
}

// UEContextReleaseComplete is a decoded NGAP UEContextReleaseComplete
// (3GPP TS 38.413). All IEs are criticality-ignore. AMFUENGAPID and RANUENGAPID
// are mandatory-ignore, nil on a missing or malformed value.
// UserLocationInformation, InfoOnRecommendedCellsAndRANNodesForPaging and
// PDUSessionResourceList are optional and may be nil.
//
// All non-scalar fields alias the source PDU buffer and must be consumed within
// the synchronous handler invocation.
type UEContextReleaseComplete struct {
	AMFUENGAPID                                *int64
	RANUENGAPID                                *int64
	UserLocationInformation                    *ngapType.UserLocationInformation
	InfoOnRecommendedCellsAndRANNodesForPaging *ngapType.InfoOnRecommendedCellsAndRANNodesForPaging
	PDUSessionResourceList                     *ngapType.PDUSessionResourceListCxtRelCpl
}

// PDUSessionResourceReleaseResponse is a decoded NGAP
// PDUSessionResourceReleaseResponse (3GPP TS 38.413). AMFUENGAPID, RANUENGAPID
// and PDUSessionResourceReleasedListRelRes are mandatory-ignore; AMFUENGAPID and
// RANUENGAPID are pointers. PDUSessionResourceReleasedItems aliases the source
// PDU buffer (PDUSessionResourceReleaseResponseTransfer octet strings).
// UserLocationInformation is optional-ignore.
type PDUSessionResourceReleaseResponse struct {
	AMFUENGAPID                     *int64
	RANUENGAPID                     *int64
	PDUSessionResourceReleasedItems []ngapType.PDUSessionResourceReleasedItemRelRes
	UserLocationInformation         *ngapType.UserLocationInformation
}

// PDUSessionResourceSetupResponse is a decoded NGAP
// PDUSessionResourceSetupResponse (3GPP TS 38.413). All IEs are
// criticality-ignore. AMFUENGAPID and RANUENGAPID are mandatory-ignore pointers.
// SetupItems and FailedToSetupItems are optional and may be nil; both alias the
// source PDU buffer.
type PDUSessionResourceSetupResponse struct {
	AMFUENGAPID        *int64
	RANUENGAPID        *int64
	SetupItems         []ngapType.PDUSessionResourceSetupItemSURes
	FailedToSetupItems []ngapType.PDUSessionResourceFailedToSetupItemSURes
}

// PDUSessionResourceModifyResponse is a decoded NGAP
// PDUSessionResourceModifyResponse (3GPP TS 38.413). All IEs are
// criticality-ignore. AMFUENGAPID and RANUENGAPID are mandatory-ignore pointers.
// UserLocationInformation is optional. The per-PDU-session lists are not
// surfaced.
type PDUSessionResourceModifyResponse struct {
	AMFUENGAPID             *int64
	RANUENGAPID             *int64
	UserLocationInformation *ngapType.UserLocationInformation
}

// HandoverRequestAcknowledge is a decoded NGAP HandoverRequestAcknowledge
// (3GPP TS 38.413). AMFUENGAPID, RANUENGAPID and PDUSessionResourceAdmittedList
// are mandatory-ignore pointers (0 is a valid UE NGAP ID, so absent differs from
// present). TargetToSourceTransparentContainer is mandatory-reject; a missing or
// malformed value yields a fatal report. PDUSessionResourceFailedToSetupItems is
// optional-ignore.
//
// AdmittedItems and FailedToSetupItems alias the source PDU buffer
// (HandoverRequestAcknowledgeTransfer /
// HandoverResourceAllocationUnsuccessfulTransfer octet strings) and must be
// consumed within the synchronous handler invocation.
type HandoverRequestAcknowledge struct {
	AMFUENGAPID                        *int64
	RANUENGAPID                        *int64
	AdmittedItems                      []ngapType.PDUSessionResourceAdmittedItem
	FailedToSetupItems                 []ngapType.PDUSessionResourceFailedToSetupItemHOAck
	TargetToSourceTransparentContainer ngapType.TargetToSourceTransparentContainer
}

// HandoverNotify is a decoded NGAP HandoverNotify (3GPP TS 38.413). AMFUENGAPID
// and RANUENGAPID are mandatory-reject. UserLocationInformation is
// mandatory-ignore (nil when absent or malformed) and aliases the source PDU
// buffer, so it must be consumed within the synchronous handler invocation.
type HandoverNotify struct {
	AMFUENGAPID             int64
	RANUENGAPID             int64
	UserLocationInformation *ngapType.UserLocationInformation
}

// PDUSessionResourceNotify is a decoded NGAP PDUSessionResourceNotify
// (3GPP TS 38.413). AMFUENGAPID and RANUENGAPID are mandatory-reject.
// HasNotifyList records whether the optional-reject
// PDUSessionResourceNotifyList IE was present; its contents are not surfaced.
// PDUSessionResourceReleasedItems and UserLocationInformation are
// optional-ignore.
//
// PDUSessionResourceReleasedItems aliases the source PDU buffer
// (PDUSessionResourceNotifyReleasedTransfer octet strings) and must be consumed
// within the synchronous handler invocation.
type PDUSessionResourceNotify struct {
	AMFUENGAPID                     int64
	RANUENGAPID                     int64
	HasNotifyList                   bool
	PDUSessionResourceReleasedItems []ngapType.PDUSessionResourceReleasedItemNot
	UserLocationInformation         *ngapType.UserLocationInformation
}

// LocationReport is a decoded NGAP LocationReport (3GPP TS 38.413). AMFUENGAPID
// and RANUENGAPID are mandatory-reject. UserLocationInformation and
// LocationReportingRequestType are mandatory-ignore, nil when absent or
// malformed. UEPresenceInAreaOfInterestList is optional-ignore.
//
// All pointer fields alias the source PDU buffer and must be consumed within the
// synchronous handler invocation.
type LocationReport struct {
	AMFUENGAPID                    int64
	RANUENGAPID                    int64
	UserLocationInformation        *ngapType.UserLocationInformation
	UEPresenceInAreaOfInterestList *ngapType.UEPresenceInAreaOfInterestList
	LocationReportingRequestType   *ngapType.LocationReportingRequestType
}

// UplinkRANConfigurationTransfer is a decoded NGAP UplinkRANConfigurationTransfer
// (3GPP TS 38.413). All IEs are optional-ignore. Only SONConfigurationTransferUL
// is surfaced; ENDCSONConfigurationTransferUL is validated but not surfaced.
//
// SONConfigurationTransferUL aliases the source PDU buffer (TargetRANNodeID,
// SourceRANNodeID and XnTNLConfigurationInfo inside) and must be consumed within
// the synchronous handler invocation.
type UplinkRANConfigurationTransfer struct {
	SONConfigurationTransferUL *ngapType.SONConfigurationTransfer
}

// PDUSessionResourceModifyIndication is a decoded NGAP
// PDUSessionResourceModifyIndication (3GPP TS 38.413). All three IEs are
// mandatory-reject, so a missing or malformed value yields a fatal report.
type PDUSessionResourceModifyIndication struct {
	AMFUENGAPID             int64
	RANUENGAPID             int64
	PDUSessionResourceItems []ngapType.PDUSessionResourceModifyItemModInd
}

// RANConfigurationUpdate is a decoded NGAP RANConfigurationUpdate
// (3GPP TS 38.413). All IEs are optional; the decoder records malformed-IE
// diagnostics but never reports a missing-mandatory IE. Only SupportedTAItems
// and RANNodeName are surfaced.
//
// SupportedTAItems aliases the source PDU buffer (TAC, PLMNIdentity and SNSSAI
// octet strings) and must be consumed within the synchronous handler invocation.
type RANConfigurationUpdate struct {
	// SupportedTAItems is nil when the (optional) Supported TA List IE is absent,
	// signalling the stored TAs are unchanged (TS 38.413 §8.7.2.2).
	SupportedTAItems []ngapType.SupportedTAItem
	RANNodeName      string
}
