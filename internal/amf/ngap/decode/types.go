// Copyright 2026 Ella Networks
// SPDX-License-Identifier: Apache-2.0

package decode

import (
	"github.com/free5gc/aper"
	"github.com/free5gc/ngap/ngapType"
)

type RRCEstablishmentCause aper.Enumerated

type GlobalRANNodeKind uint8

const (
	GlobalRANNodeKindUnknown GlobalRANNodeKind = iota
	GlobalRANNodeKindGNB
	GlobalRANNodeKindNgENB
	GlobalRANNodeKindN3IWF
)

// GlobalRANNodeID wraps the validated free5gc CHOICE. When Kind is not
// Unknown, the variant pointer matching Kind and any nested
// *aper.BitString are non-nil. Raw is a transitional accessor — like
// UserLocationInformation.Raw, the bytes inside the returned pointer
// alias the source PDU buffer and must be consumed within the
// synchronous handler invocation.
type GlobalRANNodeID struct {
	kind GlobalRANNodeKind
	raw  *ngapType.GlobalRANNodeID
}

func (g GlobalRANNodeID) Kind() GlobalRANNodeKind        { return g.kind }
func (g GlobalRANNodeID) Raw() *ngapType.GlobalRANNodeID { return g.raw }

type UserLocationKind uint8

const (
	UserLocationKindUnknown UserLocationKind = iota
	UserLocationKindNR
	UserLocationKindEUTRA
	UserLocationKindN3IWF
)

// UserLocationInformation wraps the free5gc CHOICE. When Kind is not
// Unknown, raw and the variant pointer matching Kind are both non-nil.
//
// Raw is a transitional accessor for callers not yet migrated to typed
// fields. Unlike NASPDU and FiveGTMSI, the bytes inside the returned
// pointer are NOT copied out of the source PDU buffer; callers must
// finish consuming the value within the synchronous handler invocation
// driven by the dispatcher.
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

// InitialUEMessage is a decoded NGAP InitialUEMessage (3GPP TS 38.413
// §9.2.5.1). Non-pointer fields are mandatory and populated when the
// accompanying *Report is non-fatal; pointer fields are optional.
type InitialUEMessage struct {
	RANUENGAPID             int64
	NASPDU                  []byte
	UserLocationInformation UserLocationInformation
	RRCEstablishmentCause   RRCEstablishmentCause
	FiveGSTMSI              *FiveGSTMSI
	UEContextRequest        bool
}

// NGSetupRequest is a decoded NGAP NGSetupRequest (3GPP TS 38.413
// §9.2.6.1). GlobalRANNodeID and SupportedTAItems are mandatory and
// populated when the accompanying *Report is non-fatal. RANNodeName is
// optional ("" when absent).
//
// SupportedTAItems aliases the source PDU buffer (TAC, PLMNIdentity
// and SNSSAI octet strings); like UserLocationInformation.Raw, callers
// must finish consuming it within the synchronous handler invocation.
// SupportedTAItems may be empty even after a non-fatal decode: the IE
// container itself was present but carried zero items, which TS
// 38.413 forbids structurally but real gNBs occasionally send. The
// handler decides whether to reject empty lists.
type NGSetupRequest struct {
	GlobalRANNodeID  GlobalRANNodeID
	SupportedTAItems []ngapType.SupportedTAItem
	RANNodeName      string
}

// PathSwitchRequest is a decoded NGAP PathSwitchRequest (3GPP TS 38.413
// §9.2.3.1). RANUENGAPID, SourceAMFUENGAPID and PDUSessionResourceItems
// are mandatory-reject and populated when the accompanying *Report is
// non-fatal. UserLocationInformation and UESecurityCapabilities are
// mandatory-ignore: missing or malformed values yield a non-fatal report
// and a zero-value field, so the handler must still cope with an empty
// ULI Kind and a nil UESecurityCapabilities. FailedToSetupItems is
// optional and may be nil.
//
// PDUSessionResourceItems and FailedToSetupItems alias the source PDU
// buffer (PathSwitchRequest{Transfer,SetupFailedTransfer} octet
// strings); like UserLocationInformation.Raw, callers must finish
// consuming them within the synchronous handler invocation.
//
// PDUSessionResourceItems may be structurally empty even on a non-fatal
// decode: TS 38.413 sizeLB:1 forbids it, but the decoder does not
// enforce sizeLB and the handler decides how to react.
type PathSwitchRequest struct {
	RANUENGAPID             int64
	SourceAMFUENGAPID       int64
	UserLocationInformation UserLocationInformation
	UESecurityCapabilities  *ngapType.UESecurityCapabilities
	PDUSessionResourceItems []ngapType.PDUSessionResourceToBeSwitchedDLItem
	FailedToSetupItems      []ngapType.PDUSessionResourceFailedToSetupItemPSReq
}

// HandoverRequired is a decoded NGAP HandoverRequired (3GPP TS 38.413
// §9.2.3.1 — Handover Preparation procedure). All fields except Cause
// correspond to mandatory-reject IEs and are populated when the
// accompanying *Report is non-fatal. Cause is mandatory-ignore: a
// missing or malformed value yields a non-fatal report and a zero-value
// Cause, so the handler must cope with Cause.Present == 0.
//
// TargetID, PDUSessionResourceItems and SourceToTargetTransparentContainer
// alias the source PDU buffer; callers must finish consuming them within
// the synchronous handler invocation driven by the dispatcher.
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
// (3GPP TS 38.413 §9.2.2.2). AMFUENGAPID and RANUENGAPID are
// mandatory-ignore — missing or malformed values yield a non-fatal
// report with the relevant field left zero. SetupItems and
// FailedToSetupItems are optional and may be nil. Both alias the source
// PDU buffer (PDUSessionResourceSetupResponseTransfer /
// PDUSessionResourceSetupUnsuccessfulTransfer octet strings) and must be
// consumed within the synchronous handler invocation.
type InitialContextSetupResponse struct {
	AMFUENGAPID        int64
	RANUENGAPID        int64
	SetupItems         []ngapType.PDUSessionResourceSetupItemCxtRes
	FailedToSetupItems []ngapType.PDUSessionResourceFailedToSetupItemCxtRes
}

// UplinkNASTransport is a decoded NGAP UplinkNASTransport (3GPP TS
// 38.413 §9.2.5.3). AMFUENGAPID, RANUENGAPID and NASPDU are mandatory-
// reject and populated when the accompanying *Report is non-fatal.
// UserLocationInformation is mandatory-ignore: a missing or malformed
// value yields a non-fatal report and a zero-value UserLocationKind.
//
// NASPDU is copied out of the source PDU buffer so the handler may
// store it across asynchronous boundaries; UserLocationInformation
// aliases the source buffer like in InitialUEMessage.
type UplinkNASTransport struct {
	AMFUENGAPID             int64
	RANUENGAPID             int64
	NASPDU                  []byte
	UserLocationInformation UserLocationInformation
}

// UEContextReleaseRequest is a decoded NGAP UEContextReleaseRequest
// (3GPP TS 38.413 §9.2.2.5). AMFUENGAPID and RANUENGAPID are
// mandatory-reject and populated when the accompanying *Report is
// non-fatal. Cause is mandatory-ignore: a missing or malformed value
// yields a non-fatal report and a nil Cause pointer. PDUSessionResourceList
// is optional-reject: when the IE is absent the slice is nil; when the
// IE is present (even with an empty inner list) the slice is non-nil so
// callers can distinguish "no IE" from "IE present, no items".
//
// PDUSessionResourceList aliases the source PDU buffer; callers must
// finish consuming it within the synchronous handler invocation driven
// by the dispatcher.
type UEContextReleaseRequest struct {
	AMFUENGAPID            int64
	RANUENGAPID            int64
	PDUSessionResourceList []ngapType.PDUSessionResourceItemCxtRelReq
	Cause                  *ngapType.Cause
}

// NGReset is a decoded NGAP NGReset (3GPP TS 38.413 §9.2.6.10).
// Cause is mandatory-ignore: a missing or malformed value yields a
// non-fatal report and a zero-value Cause. ResetType is mandatory-reject;
// when populated, the inner CHOICE pointer matching ResetType.Present is
// non-nil. ResetType aliases the source PDU buffer.
type NGReset struct {
	Cause     ngapType.Cause
	ResetType *ngapType.ResetType
}

// ErrorIndication is a decoded NGAP ErrorIndication (3GPP TS 38.413
// §9.2.7.2). All four IEs are optional-ignore. The decoder records
// malformed-IE diagnostics in *Report but never raises a fatal error.
// Per the spec the message must contain at least one of Cause or
// CriticalityDiagnostics; the decoder does not enforce this — handlers
// that care must check it. AMF-UE-NGAP-ID and RAN-UE-NGAP-ID are
// validated structurally but not surfaced because no current handler
// uses them.
type ErrorIndication struct {
	Cause                  *ngapType.Cause
	CriticalityDiagnostics *ngapType.CriticalityDiagnostics
}

// HandoverCancel is a decoded NGAP HandoverCancel (3GPP TS 38.413
// §9.2.3.4). AMFUENGAPID and RANUENGAPID are mandatory-reject; Cause is
// mandatory-ignore (missing/malformed → nil pointer in the decoded
// struct). All fields are populated when the accompanying *Report is
// non-fatal.
type HandoverCancel struct {
	AMFUENGAPID int64
	RANUENGAPID int64
	Cause       *ngapType.Cause
}

// UERadioCapabilityInfoIndication is a decoded NGAP
// UERadioCapabilityInfoIndication (3GPP TS 38.413 §9.2.7.7).
// AMFUENGAPID and RANUENGAPID are mandatory-reject. UERadioCapability
// is mandatory-ignore — missing or malformed yields a non-fatal report
// and a nil byte slice. UERadioCapabilityForPaging is optional-ignore
// and is nil when absent or malformed.
//
// UERadioCapability and UERadioCapabilityForPaging alias the source
// PDU buffer; callers must finish consuming them within the synchronous
// handler invocation driven by the dispatcher.
type UERadioCapabilityInfoIndication struct {
	AMFUENGAPID                int64
	RANUENGAPID                int64
	UERadioCapability          []byte
	UERadioCapabilityForPaging *ngapType.UERadioCapabilityForPaging
}

// NASNonDeliveryIndication is a decoded NGAP NASNonDeliveryIndication
// (3GPP TS 38.413 §9.2.5.5). AMFUENGAPID and RANUENGAPID are
// mandatory-reject. NASPDU and Cause are mandatory-ignore: a missing or
// malformed NASPDU yields an empty byte slice and a missing or
// malformed Cause yields a zero-value Cause; both leave the report
// non-fatal so the handler still runs.
//
// NASPDU is copied out of the source PDU buffer (like in
// UplinkNASTransport) so the handler may forward it across asynchronous
// boundaries to NAS processing.
type NASNonDeliveryIndication struct {
	AMFUENGAPID int64
	RANUENGAPID int64
	NASPDU      []byte
	Cause       ngapType.Cause
}

// RANConfigurationUpdate is a decoded NGAP RANConfigurationUpdate
// (3GPP TS 38.413 §9.2.6.6). All IEs in this message are optional;
// the decoder records malformed-IE diagnostics in *Report but never
// reports a missing-mandatory IE. SupportedTAItems is the only IE
// surfaced because it is the only one the handler currently consumes;
// when the SupportedTAList IE is absent the slice is nil, when present
// (even structurally empty) the slice is non-nil.
//
// SupportedTAItems aliases the source PDU buffer (TAC, PLMNIdentity and
// SNSSAI octet strings); like UserLocationInformation.Raw, callers must
// finish consuming it within the synchronous handler invocation.
type RANConfigurationUpdate struct {
	SupportedTAItems []ngapType.SupportedTAItem
}
