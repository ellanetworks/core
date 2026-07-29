// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

// This file is one line per message type: the type each message reports and the
// marker that seals the Message interface. The interfaces in messages.go
// document what they mean, so the methods carry no comment of their own.
//
//nolint:revive // one uncommented line per message type; see above
package fgs

import "github.com/ellanetworks/core/nas"

// Every 5GMM message reports its type.
func (m *AuthenticationRequest) MessageType() MessageType      { return MsgAuthenticationRequest }
func (m *AuthenticationReject) MessageType() MessageType       { return MsgAuthenticationReject }
func (m *AuthenticationResponse) MessageType() MessageType     { return MsgAuthenticationResponse }
func (m *AuthenticationFailure) MessageType() MessageType      { return MsgAuthenticationFailure }
func (m *ConfigurationUpdateCommand) MessageType() MessageType { return MsgConfigurationUpdateCommand }
func (m *ConfigurationUpdateComplete) MessageType() MessageType {
	return MsgConfigurationUpdateComplete
}

func (m *DeregistrationRequestUETerminated) MessageType() MessageType {
	return MsgDeregistrationRequestUETerm
}

func (m *DeregistrationRequestUEOriginating) MessageType() MessageType {
	return MsgDeregistrationRequestUEOrig
}

func (m *DeregistrationAcceptUEOriginating) MessageType() MessageType {
	return MsgDeregistrationAcceptUEOrig
}

func (m *DeregistrationAcceptUETerminated) MessageType() MessageType {
	return MsgDeregistrationAcceptUETerm
}

func (m *PDUSessionAuthenticationComplete) MessageType() GSMMessageType {
	return MsgPDUSessionAuthenticationComplete
}

func (m *PDUSessionAuthenticationComplete) SessionIdentity() PDUSessionID { return m.PDUSessionID }

func (m *PDUSessionAuthenticationComplete) TransactionIdentity() nas.ProcedureTransactionIdentity {
	return m.PTI
}
func (m *IdentityRequest) MessageType() MessageType      { return MsgIdentityRequest }
func (m *IdentityResponse) MessageType() MessageType     { return MsgIdentityResponse }
func (m *NotificationResponse) MessageType() MessageType { return MsgNotificationResponse }
func (m *RegistrationRequest) MessageType() MessageType  { return MsgRegistrationRequest }
func (m *RegistrationAccept) MessageType() MessageType   { return MsgRegistrationAccept }
func (m *RegistrationComplete) MessageType() MessageType { return MsgRegistrationComplete }
func (m *GMMStatus) MessageType() MessageType            { return MsgGMMStatus }
func (m *SecurityModeReject) MessageType() MessageType   { return MsgSecurityModeReject }
func (m *ServiceReject) MessageType() MessageType        { return MsgServiceReject }
func (m *RegistrationReject) MessageType() MessageType   { return MsgRegistrationReject }
func (m *SecurityModeCommand) MessageType() MessageType  { return MsgSecurityModeCommand }
func (m *SecurityModeComplete) MessageType() MessageType { return MsgSecurityModeComplete }
func (m *ServiceRequest) MessageType() MessageType       { return MsgServiceRequest }
func (m *ServiceAccept) MessageType() MessageType        { return MsgServiceAccept }
func (m *DLNASTransport) MessageType() MessageType       { return MsgDLNASTransport }
func (m *ULNASTransport) MessageType() MessageType       { return MsgULNASTransport }

// Every 5GSM message reports its type.
func (m *PDUSessionEstablishmentRequest) MessageType() GSMMessageType {
	return MsgPDUSessionEstablishmentRequest
}

func (m *PDUSessionEstablishmentReject) MessageType() GSMMessageType {
	return MsgPDUSessionEstablishmentReject
}

func (m *PDUSessionEstablishmentAccept) MessageType() GSMMessageType {
	return MsgPDUSessionEstablishmentAccept
}

func (m *PDUSessionModificationRequest) MessageType() GSMMessageType {
	return MsgPDUSessionModificationRequest
}

func (m *PDUSessionModificationReject) MessageType() GSMMessageType {
	return MsgPDUSessionModificationReject
}

func (m *PDUSessionModificationCommand) MessageType() GSMMessageType {
	return MsgPDUSessionModificationCommand
}

func (m *PDUSessionModificationComplete) MessageType() GSMMessageType {
	return MsgPDUSessionModificationComplete
}

func (m *PDUSessionModificationCommandReject) MessageType() GSMMessageType {
	return MsgPDUSessionModificationCmdReject
}
func (m *PDUSessionReleaseCommand) MessageType() GSMMessageType  { return MsgPDUSessionReleaseCommand }
func (m *PDUSessionReleaseRequest) MessageType() GSMMessageType  { return MsgPDUSessionReleaseRequest }
func (m *PDUSessionReleaseComplete) MessageType() GSMMessageType { return MsgPDUSessionReleaseComplete }
func (m *GSMStatus) MessageType() GSMMessageType                 { return MsgGSMStatus }

// The messages of this package, and only they, are Messages.
func (m *AuthenticationRequest) isMessage()               {}
func (m *AuthenticationReject) isMessage()                {}
func (m *AuthenticationResponse) isMessage()              {}
func (m *AuthenticationFailure) isMessage()               {}
func (m *ConfigurationUpdateCommand) isMessage()          {}
func (m *ConfigurationUpdateComplete) isMessage()         {}
func (m *DeregistrationRequestUETerminated) isMessage()   {}
func (m *DeregistrationRequestUEOriginating) isMessage()  {}
func (m *DeregistrationAcceptUEOriginating) isMessage()   {}
func (m *DeregistrationAcceptUETerminated) isMessage()    {}
func (m *PDUSessionAuthenticationComplete) isMessage()    {}
func (m *IdentityRequest) isMessage()                     {}
func (m *IdentityResponse) isMessage()                    {}
func (m *NotificationResponse) isMessage()                {}
func (m *RegistrationRequest) isMessage()                 {}
func (m *RegistrationAccept) isMessage()                  {}
func (m *RegistrationComplete) isMessage()                {}
func (m *GMMStatus) isMessage()                           {}
func (m *SecurityModeReject) isMessage()                  {}
func (m *ServiceReject) isMessage()                       {}
func (m *RegistrationReject) isMessage()                  {}
func (m *SecurityModeCommand) isMessage()                 {}
func (m *SecurityModeComplete) isMessage()                {}
func (m *ServiceRequest) isMessage()                      {}
func (m *ServiceAccept) isMessage()                       {}
func (m *DLNASTransport) isMessage()                      {}
func (m *ULNASTransport) isMessage()                      {}
func (m *PDUSessionEstablishmentRequest) isMessage()      {}
func (m *PDUSessionEstablishmentReject) isMessage()       {}
func (m *PDUSessionEstablishmentAccept) isMessage()       {}
func (m *PDUSessionModificationRequest) isMessage()       {}
func (m *PDUSessionModificationReject) isMessage()        {}
func (m *PDUSessionModificationCommand) isMessage()       {}
func (m *PDUSessionModificationComplete) isMessage()      {}
func (m *PDUSessionModificationCommandReject) isMessage() {}
func (m *PDUSessionReleaseCommand) isMessage()            {}
func (m *PDUSessionReleaseRequest) isMessage()            {}
func (m *PDUSessionReleaseComplete) isMessage()           {}
func (m *GSMStatus) isMessage()                           {}

// Every message of this package implements its generation's interface, whether
// or not the dispatch table reaches it.
var (
	_ GMMMessage = (*AuthenticationRequest)(nil)
	_ GMMMessage = (*AuthenticationReject)(nil)
	_ GMMMessage = (*AuthenticationResponse)(nil)
	_ GMMMessage = (*AuthenticationFailure)(nil)
	_ GMMMessage = (*ConfigurationUpdateCommand)(nil)
	_ GMMMessage = (*ConfigurationUpdateComplete)(nil)
	_ GMMMessage = (*DeregistrationRequestUETerminated)(nil)
	_ GMMMessage = (*DeregistrationRequestUEOriginating)(nil)
	_ GMMMessage = (*DeregistrationAcceptUEOriginating)(nil)
	_ GMMMessage = (*DeregistrationAcceptUETerminated)(nil)
	_ GSMMessage = (*PDUSessionAuthenticationComplete)(nil)
	_ GMMMessage = (*IdentityRequest)(nil)
	_ GMMMessage = (*IdentityResponse)(nil)
	_ GMMMessage = (*NotificationResponse)(nil)
	_ GMMMessage = (*RegistrationRequest)(nil)
	_ GMMMessage = (*RegistrationAccept)(nil)
	_ GMMMessage = (*RegistrationComplete)(nil)
	_ GMMMessage = (*GMMStatus)(nil)
	_ GMMMessage = (*SecurityModeReject)(nil)
	_ GMMMessage = (*ServiceReject)(nil)
	_ GMMMessage = (*RegistrationReject)(nil)
	_ GMMMessage = (*SecurityModeCommand)(nil)
	_ GMMMessage = (*SecurityModeComplete)(nil)
	_ GMMMessage = (*ServiceRequest)(nil)
	_ GMMMessage = (*ServiceAccept)(nil)
	_ GMMMessage = (*DLNASTransport)(nil)
	_ GMMMessage = (*ULNASTransport)(nil)
	_ GSMMessage = (*PDUSessionEstablishmentRequest)(nil)
	_ GSMMessage = (*PDUSessionEstablishmentReject)(nil)
	_ GSMMessage = (*PDUSessionEstablishmentAccept)(nil)
	_ GSMMessage = (*PDUSessionModificationReject)(nil)
	_ GSMMessage = (*PDUSessionModificationCommand)(nil)
	_ GSMMessage = (*PDUSessionModificationComplete)(nil)
	_ GSMMessage = (*PDUSessionModificationCommandReject)(nil)
	_ GSMMessage = (*PDUSessionReleaseCommand)(nil)
	_ GSMMessage = (*PDUSessionReleaseRequest)(nil)
	_ GSMMessage = (*PDUSessionReleaseComplete)(nil)
	_ GSMMessage = (*GSMStatus)(nil)
	_ Message    = (*UnknownMessage)(nil)
)

// Every 5GSM message names the PDU session it concerns.
func (m *PDUSessionEstablishmentRequest) SessionIdentity() PDUSessionID      { return m.PDUSessionID }
func (m *PDUSessionEstablishmentReject) SessionIdentity() PDUSessionID       { return m.PDUSessionID }
func (m *PDUSessionEstablishmentAccept) SessionIdentity() PDUSessionID       { return m.PDUSessionID }
func (m *PDUSessionModificationRequest) SessionIdentity() PDUSessionID       { return m.PDUSessionID }
func (m *PDUSessionModificationReject) SessionIdentity() PDUSessionID        { return m.PDUSessionID }
func (m *PDUSessionModificationCommand) SessionIdentity() PDUSessionID       { return m.PDUSessionID }
func (m *PDUSessionModificationComplete) SessionIdentity() PDUSessionID      { return m.PDUSessionID }
func (m *PDUSessionModificationCommandReject) SessionIdentity() PDUSessionID { return m.PDUSessionID }
func (m *PDUSessionReleaseCommand) SessionIdentity() PDUSessionID            { return m.PDUSessionID }
func (m *PDUSessionReleaseRequest) SessionIdentity() PDUSessionID            { return m.PDUSessionID }
func (m *PDUSessionReleaseComplete) SessionIdentity() PDUSessionID           { return m.PDUSessionID }
func (m *GSMStatus) SessionIdentity() PDUSessionID                           { return m.PDUSessionID }

// Every 5GSM message names the transaction it belongs to.
func (m *PDUSessionEstablishmentRequest) TransactionIdentity() nas.ProcedureTransactionIdentity {
	return m.PTI
}

func (m *PDUSessionEstablishmentReject) TransactionIdentity() nas.ProcedureTransactionIdentity {
	return m.PTI
}

func (m *PDUSessionEstablishmentAccept) TransactionIdentity() nas.ProcedureTransactionIdentity {
	return m.PTI
}

func (m *PDUSessionModificationRequest) TransactionIdentity() nas.ProcedureTransactionIdentity {
	return m.PTI
}

func (m *PDUSessionModificationReject) TransactionIdentity() nas.ProcedureTransactionIdentity {
	return m.PTI
}

func (m *PDUSessionModificationCommand) TransactionIdentity() nas.ProcedureTransactionIdentity {
	return m.PTI
}

func (m *PDUSessionModificationComplete) TransactionIdentity() nas.ProcedureTransactionIdentity {
	return m.PTI
}

func (m *PDUSessionModificationCommandReject) TransactionIdentity() nas.ProcedureTransactionIdentity {
	return m.PTI
}

func (m *PDUSessionReleaseCommand) TransactionIdentity() nas.ProcedureTransactionIdentity {
	return m.PTI
}

func (m *PDUSessionReleaseRequest) TransactionIdentity() nas.ProcedureTransactionIdentity {
	return m.PTI
}

func (m *PDUSessionReleaseComplete) TransactionIdentity() nas.ProcedureTransactionIdentity {
	return m.PTI
}
func (m *GSMStatus) TransactionIdentity() nas.ProcedureTransactionIdentity { return m.PTI }
