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
