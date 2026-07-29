// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

// This file is one line per message type: the type each message reports and the
// marker that seals the Message interface. The interfaces in messages.go
// document what they mean, so the methods carry no comment of their own.
//
//nolint:revive // one uncommented line per message type; see above
package eps

import "github.com/ellanetworks/core/nas"

// Every EMM message reports its type.
func (m *AttachRequest) MessageType() MessageType              { return MsgAttachRequest }
func (m *AttachAccept) MessageType() MessageType               { return MsgAttachAccept }
func (m *AttachComplete) MessageType() MessageType             { return MsgAttachComplete }
func (m *AttachReject) MessageType() MessageType               { return MsgAttachReject }
func (m *AuthenticationRequest) MessageType() MessageType      { return MsgAuthenticationRequest }
func (m *AuthenticationResponse) MessageType() MessageType     { return MsgAuthenticationResponse }
func (m *AuthenticationReject) MessageType() MessageType       { return MsgAuthenticationReject }
func (m *AuthenticationFailure) MessageType() MessageType      { return MsgAuthenticationFailure }
func (m *DetachRequestUE) MessageType() MessageType            { return MsgDetachRequest }
func (m *DetachRequestNetwork) MessageType() MessageType       { return MsgDetachRequest }
func (m *DetachAccept) MessageType() MessageType               { return MsgDetachAccept }
func (m *GUTIReallocationCommand) MessageType() MessageType    { return MsgGUTIReallocationCommand }
func (m *GUTIReallocationComplete) MessageType() MessageType   { return MsgGUTIReallocationComplete }
func (m *IdentityRequest) MessageType() MessageType            { return MsgIdentityRequest }
func (m *IdentityResponse) MessageType() MessageType           { return MsgIdentityResponse }
func (m *EMMInformation) MessageType() MessageType             { return MsgEMMInformation }
func (m *SecurityModeCommand) MessageType() MessageType        { return MsgSecurityModeCommand }
func (m *SecurityModeComplete) MessageType() MessageType       { return MsgSecurityModeComplete }
func (m *SecurityModeReject) MessageType() MessageType         { return MsgSecurityModeReject }
func (m *ServiceReject) MessageType() MessageType              { return MsgServiceReject }
func (m *ServiceAccept) MessageType() MessageType              { return MsgServiceAccept }
func (m *EMMStatus) MessageType() MessageType                  { return MsgEMMStatus }
func (m *TrackingAreaUpdateRequest) MessageType() MessageType  { return MsgTrackingAreaUpdateRequest }
func (m *TrackingAreaUpdateAccept) MessageType() MessageType   { return MsgTrackingAreaUpdateAccept }
func (m *TrackingAreaUpdateComplete) MessageType() MessageType { return MsgTrackingAreaUpdateComplete }
func (m *TrackingAreaUpdateReject) MessageType() MessageType   { return MsgTrackingAreaUpdateReject }

// Every ESM message reports its type.
func (m *ActivateDefaultEPSBearerContextRequest) MessageType() ESMMessageType {
	return MsgActivateDefaultEPSBearerContextRequest
}

func (m *ActivateDefaultEPSBearerContextAccept) MessageType() ESMMessageType {
	return MsgActivateDefaultEPSBearerContextAccept
}

func (m *ActivateDefaultEPSBearerContextReject) MessageType() ESMMessageType {
	return MsgActivateDefaultEPSBearerContextReject
}

func (m *BearerResourceAllocationRequest) MessageType() ESMMessageType {
	return MsgBearerResourceAllocationRequest
}

func (m *BearerResourceAllocationReject) MessageType() ESMMessageType {
	return MsgBearerResourceAllocationReject
}

func (m *BearerResourceModificationRequest) MessageType() ESMMessageType {
	return MsgBearerResourceModificationRequest
}

func (m *BearerResourceModificationReject) MessageType() ESMMessageType {
	return MsgBearerResourceModificationReject
}

func (m *DeactivateEPSBearerContextRequest) MessageType() ESMMessageType {
	return MsgDeactivateEPSBearerContextRequest
}

func (m *DeactivateEPSBearerContextAccept) MessageType() ESMMessageType {
	return MsgDeactivateEPSBearerContextAccept
}
func (m *ESMInformationRequest) MessageType() ESMMessageType  { return MsgESMInformationRequest }
func (m *ESMInformationResponse) MessageType() ESMMessageType { return MsgESMInformationResponse }
func (m *ModifyEPSBearerContextRequest) MessageType() ESMMessageType {
	return MsgModifyEPSBearerContextRequest
}

func (m *ModifyEPSBearerContextAccept) MessageType() ESMMessageType {
	return MsgModifyEPSBearerContextAccept
}

func (m *ModifyEPSBearerContextReject) MessageType() ESMMessageType {
	return MsgModifyEPSBearerContextReject
}
func (m *PDNConnectivityRequest) MessageType() ESMMessageType { return MsgPDNConnectivityRequest }
func (m *PDNConnectivityReject) MessageType() ESMMessageType  { return MsgPDNConnectivityReject }
func (m *PDNDisconnectRequest) MessageType() ESMMessageType   { return MsgPDNDisconnectRequest }
func (m *PDNDisconnectReject) MessageType() ESMMessageType    { return MsgPDNDisconnectReject }
func (m *ESMStatus) MessageType() ESMMessageType              { return MsgESMStatus }

// The messages of this package, and only they, are Messages.
func (m *AttachRequest) isMessage()                          {}
func (m *AttachAccept) isMessage()                           {}
func (m *AttachComplete) isMessage()                         {}
func (m *AttachReject) isMessage()                           {}
func (m *AuthenticationRequest) isMessage()                  {}
func (m *AuthenticationResponse) isMessage()                 {}
func (m *AuthenticationReject) isMessage()                   {}
func (m *AuthenticationFailure) isMessage()                  {}
func (m *DetachRequestUE) isMessage()                        {}
func (m *DetachRequestNetwork) isMessage()                   {}
func (m *DetachAccept) isMessage()                           {}
func (m *GUTIReallocationCommand) isMessage()                {}
func (m *GUTIReallocationComplete) isMessage()               {}
func (m *IdentityRequest) isMessage()                        {}
func (m *IdentityResponse) isMessage()                       {}
func (m *EMMInformation) isMessage()                         {}
func (m *SecurityModeCommand) isMessage()                    {}
func (m *SecurityModeComplete) isMessage()                   {}
func (m *SecurityModeReject) isMessage()                     {}
func (m *ServiceReject) isMessage()                          {}
func (m *ServiceAccept) isMessage()                          {}
func (m *EMMStatus) isMessage()                              {}
func (m *TrackingAreaUpdateRequest) isMessage()              {}
func (m *TrackingAreaUpdateAccept) isMessage()               {}
func (m *TrackingAreaUpdateComplete) isMessage()             {}
func (m *TrackingAreaUpdateReject) isMessage()               {}
func (m *ActivateDefaultEPSBearerContextRequest) isMessage() {}
func (m *ActivateDefaultEPSBearerContextAccept) isMessage()  {}
func (m *ActivateDefaultEPSBearerContextReject) isMessage()  {}
func (m *BearerResourceAllocationRequest) isMessage()        {}
func (m *BearerResourceAllocationReject) isMessage()         {}
func (m *BearerResourceModificationRequest) isMessage()      {}
func (m *BearerResourceModificationReject) isMessage()       {}
func (m *DeactivateEPSBearerContextRequest) isMessage()      {}
func (m *DeactivateEPSBearerContextAccept) isMessage()       {}
func (m *ESMInformationRequest) isMessage()                  {}
func (m *ESMInformationResponse) isMessage()                 {}
func (m *ModifyEPSBearerContextRequest) isMessage()          {}
func (m *ModifyEPSBearerContextAccept) isMessage()           {}
func (m *ModifyEPSBearerContextReject) isMessage()           {}
func (m *PDNConnectivityRequest) isMessage()                 {}
func (m *PDNConnectivityReject) isMessage()                  {}
func (m *PDNDisconnectRequest) isMessage()                   {}
func (m *PDNDisconnectReject) isMessage()                    {}
func (m *ESMStatus) isMessage()                              {}
func (m *ServiceRequest) isMessage()                         {}

// Every message of this package implements its generation's interface, whether
// or not the dispatch table reaches it.
var (
	_ EMMMessage = (*AttachRequest)(nil)
	_ EMMMessage = (*AttachAccept)(nil)
	_ EMMMessage = (*AttachComplete)(nil)
	_ EMMMessage = (*AttachReject)(nil)
	_ EMMMessage = (*AuthenticationRequest)(nil)
	_ EMMMessage = (*AuthenticationResponse)(nil)
	_ EMMMessage = (*AuthenticationReject)(nil)
	_ EMMMessage = (*AuthenticationFailure)(nil)
	_ EMMMessage = (*DetachRequestUE)(nil)
	_ EMMMessage = (*DetachRequestNetwork)(nil)
	_ EMMMessage = (*DetachAccept)(nil)
	_ EMMMessage = (*GUTIReallocationCommand)(nil)
	_ EMMMessage = (*GUTIReallocationComplete)(nil)
	_ EMMMessage = (*IdentityRequest)(nil)
	_ EMMMessage = (*IdentityResponse)(nil)
	_ EMMMessage = (*EMMInformation)(nil)
	_ EMMMessage = (*SecurityModeCommand)(nil)
	_ EMMMessage = (*SecurityModeComplete)(nil)
	_ EMMMessage = (*SecurityModeReject)(nil)
	_ EMMMessage = (*ServiceReject)(nil)
	_ EMMMessage = (*ServiceAccept)(nil)
	_ EMMMessage = (*EMMStatus)(nil)
	_ EMMMessage = (*TrackingAreaUpdateRequest)(nil)
	_ EMMMessage = (*TrackingAreaUpdateAccept)(nil)
	_ EMMMessage = (*TrackingAreaUpdateComplete)(nil)
	_ EMMMessage = (*TrackingAreaUpdateReject)(nil)
	_ ESMMessage = (*ActivateDefaultEPSBearerContextRequest)(nil)
	_ ESMMessage = (*ActivateDefaultEPSBearerContextAccept)(nil)
	_ ESMMessage = (*ActivateDefaultEPSBearerContextReject)(nil)
	_ ESMMessage = (*BearerResourceAllocationRequest)(nil)
	_ ESMMessage = (*BearerResourceAllocationReject)(nil)
	_ ESMMessage = (*BearerResourceModificationRequest)(nil)
	_ ESMMessage = (*BearerResourceModificationReject)(nil)
	_ ESMMessage = (*DeactivateEPSBearerContextRequest)(nil)
	_ ESMMessage = (*DeactivateEPSBearerContextAccept)(nil)
	_ ESMMessage = (*ESMInformationRequest)(nil)
	_ ESMMessage = (*ESMInformationResponse)(nil)
	_ ESMMessage = (*ModifyEPSBearerContextRequest)(nil)
	_ ESMMessage = (*ModifyEPSBearerContextAccept)(nil)
	_ ESMMessage = (*ModifyEPSBearerContextReject)(nil)
	_ ESMMessage = (*PDNConnectivityRequest)(nil)
	_ ESMMessage = (*PDNConnectivityReject)(nil)
	_ ESMMessage = (*PDNDisconnectRequest)(nil)
	_ ESMMessage = (*PDNDisconnectReject)(nil)
	_ ESMMessage = (*ESMStatus)(nil)
	_ Message    = (*ServiceRequest)(nil)
	_ EMMMessage = (*UnknownEMMMessage)(nil)
	_ ESMMessage = (*UnknownESMMessage)(nil)
)

// Every ESM message names the EPS bearer it concerns.
func (m *ActivateDefaultEPSBearerContextRequest) BearerIdentity() EPSBearerIdentity {
	return m.EPSBearerIdentity
}

func (m *ActivateDefaultEPSBearerContextAccept) BearerIdentity() EPSBearerIdentity {
	return m.EPSBearerIdentity
}

func (m *ActivateDefaultEPSBearerContextReject) BearerIdentity() EPSBearerIdentity {
	return m.EPSBearerIdentity
}

func (m *BearerResourceAllocationRequest) BearerIdentity() EPSBearerIdentity {
	return m.EPSBearerIdentity
}

func (m *BearerResourceAllocationReject) BearerIdentity() EPSBearerIdentity {
	return m.EPSBearerIdentity
}

func (m *BearerResourceModificationRequest) BearerIdentity() EPSBearerIdentity {
	return m.EPSBearerIdentity
}

func (m *BearerResourceModificationReject) BearerIdentity() EPSBearerIdentity {
	return m.EPSBearerIdentity
}

func (m *DeactivateEPSBearerContextRequest) BearerIdentity() EPSBearerIdentity {
	return m.EPSBearerIdentity
}

func (m *DeactivateEPSBearerContextAccept) BearerIdentity() EPSBearerIdentity {
	return m.EPSBearerIdentity
}
func (m *ESMInformationRequest) BearerIdentity() EPSBearerIdentity  { return m.EPSBearerIdentity }
func (m *ESMInformationResponse) BearerIdentity() EPSBearerIdentity { return m.EPSBearerIdentity }
func (m *ModifyEPSBearerContextRequest) BearerIdentity() EPSBearerIdentity {
	return m.EPSBearerIdentity
}
func (m *ModifyEPSBearerContextAccept) BearerIdentity() EPSBearerIdentity { return m.EPSBearerIdentity }
func (m *ModifyEPSBearerContextReject) BearerIdentity() EPSBearerIdentity { return m.EPSBearerIdentity }
func (m *PDNConnectivityRequest) BearerIdentity() EPSBearerIdentity       { return m.EPSBearerIdentity }
func (m *PDNConnectivityReject) BearerIdentity() EPSBearerIdentity        { return m.EPSBearerIdentity }
func (m *PDNDisconnectRequest) BearerIdentity() EPSBearerIdentity         { return m.EPSBearerIdentity }
func (m *PDNDisconnectReject) BearerIdentity() EPSBearerIdentity          { return m.EPSBearerIdentity }
func (m *ESMStatus) BearerIdentity() EPSBearerIdentity                    { return m.EPSBearerIdentity }

// Every ESM message names the transaction it belongs to.
func (m *ActivateDefaultEPSBearerContextRequest) TransactionIdentity() nas.ProcedureTransactionIdentity {
	return m.PTI
}

func (m *ActivateDefaultEPSBearerContextAccept) TransactionIdentity() nas.ProcedureTransactionIdentity {
	return m.PTI
}

func (m *ActivateDefaultEPSBearerContextReject) TransactionIdentity() nas.ProcedureTransactionIdentity {
	return m.PTI
}

func (m *BearerResourceAllocationRequest) TransactionIdentity() nas.ProcedureTransactionIdentity {
	return m.PTI
}

func (m *BearerResourceAllocationReject) TransactionIdentity() nas.ProcedureTransactionIdentity {
	return m.PTI
}

func (m *BearerResourceModificationRequest) TransactionIdentity() nas.ProcedureTransactionIdentity {
	return m.PTI
}

func (m *BearerResourceModificationReject) TransactionIdentity() nas.ProcedureTransactionIdentity {
	return m.PTI
}

func (m *DeactivateEPSBearerContextRequest) TransactionIdentity() nas.ProcedureTransactionIdentity {
	return m.PTI
}

func (m *DeactivateEPSBearerContextAccept) TransactionIdentity() nas.ProcedureTransactionIdentity {
	return m.PTI
}
func (m *ESMInformationRequest) TransactionIdentity() nas.ProcedureTransactionIdentity  { return m.PTI }
func (m *ESMInformationResponse) TransactionIdentity() nas.ProcedureTransactionIdentity { return m.PTI }
func (m *ModifyEPSBearerContextRequest) TransactionIdentity() nas.ProcedureTransactionIdentity {
	return m.PTI
}

func (m *ModifyEPSBearerContextAccept) TransactionIdentity() nas.ProcedureTransactionIdentity {
	return m.PTI
}

func (m *ModifyEPSBearerContextReject) TransactionIdentity() nas.ProcedureTransactionIdentity {
	return m.PTI
}
func (m *PDNConnectivityRequest) TransactionIdentity() nas.ProcedureTransactionIdentity { return m.PTI }
func (m *PDNConnectivityReject) TransactionIdentity() nas.ProcedureTransactionIdentity  { return m.PTI }
func (m *PDNDisconnectRequest) TransactionIdentity() nas.ProcedureTransactionIdentity   { return m.PTI }
func (m *PDNDisconnectReject) TransactionIdentity() nas.ProcedureTransactionIdentity    { return m.PTI }
func (m *ESMStatus) TransactionIdentity() nas.ProcedureTransactionIdentity              { return m.PTI }
