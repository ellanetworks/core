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
