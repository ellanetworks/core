// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package decode

import (
	"github.com/free5gc/ngap/ngapType"
)

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

// HandoverCancel is a decoded NGAP HandoverCancel (3GPP TS 38.413). AMFUENGAPID
// and RANUENGAPID are mandatory-reject; Cause is mandatory-ignore (nil when
// absent or malformed).
type HandoverCancel struct {
	AMFUENGAPID int64
	RANUENGAPID int64
	Cause       *ngapType.Cause
}

// HandoverFailure is a decoded NGAP HandoverFailure (3GPP TS 38.413).
// AMFUENGAPID is mandatory-reject. Cause is mandatory-ignore (nil when absent).
// CriticalityDiagnostics is optional-ignore.
type HandoverFailure struct {
	AMFUENGAPID            int64
	Cause                  *ngapType.Cause
	CriticalityDiagnostics *ngapType.CriticalityDiagnostics
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
