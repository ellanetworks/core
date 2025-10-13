// Copyright 2024 Ella Networks
// SPDX-FileCopyrightText: 2022-present Intel Corporation
// SPDX-FileCopyrightText: 2021 Open Networking Foundation <info@opennetworking.org>
// Copyright 2019 free5GC.org
//
// SPDX-License-Identifier: Apache-2.0

package gmm

import (
	"bytes"
	ctxt "context"
	"fmt"

	"github.com/ellanetworks/core/internal/amf/context"
	gmm_message "github.com/ellanetworks/core/internal/amf/gmm/message"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/util/fsm"
	"github.com/omec-project/nas"
	"github.com/omec-project/nas/nasMessage"
	"github.com/omec-project/nas/security"
	"go.uber.org/zap"
)

func DeRegistered(ctx ctxt.Context, state *fsm.State, event fsm.EventType, args fsm.ArgsType) {
	switch event {
	case fsm.EntryEvent:
		amfUe := args[ArgAmfUe].(*context.AmfUe)
		accessType := args[ArgAccessType].(models.AccessType)
		amfUe.ClearRegistrationRequestData(accessType)
		amfUe.GmmLog.Debug("EntryEvent at GMM State[DeRegistered]")
	case GmmMessageEvent:
		amfUe := args[ArgAmfUe].(*context.AmfUe)
		procedureCode := args[ArgProcedureCode].(int64)
		gmmMessage := args[ArgNASMessage].(*nas.GmmMessage)
		accessType := args[ArgAccessType].(models.AccessType)
		amfUe.GmmLog.Debug("GmmMessageEvent at GMM State[DeRegistered]")
		switch gmmMessage.GetMessageType() {
		case nas.MsgTypeRegistrationRequest:
			logger.LogNetworkEvent(
				logger.NASNetworkProtocol,
				logger.SubscriberRegistrationRequest,
				logger.DirectionInbound,
				rawGmmNasMessage(gmmMessage),
				zap.String("imsi", amfUe.Supi),
				zap.String("ran", amfUe.RanUe[accessType].Ran.Name),
				zap.String("plmnID", amfUe.PlmnID.Mcc+amfUe.PlmnID.Mnc),
			)
			if err := HandleRegistrationRequest(ctx, amfUe, accessType, procedureCode, gmmMessage.RegistrationRequest); err != nil {
				logger.AmfLog.Error("Error handling registration request", zap.Error(err))
			} else {
				if err := GmmFSM.SendEvent(ctx, state, StartAuthEvent, fsm.ArgsType{
					ArgAmfUe:         amfUe,
					ArgAccessType:    accessType,
					ArgProcedureCode: procedureCode,
				}); err != nil {
					logger.AmfLog.Error("Error sending event", zap.Error(err))
				}
			}
		case nas.MsgTypeServiceRequest:
			logger.LogNetworkEvent(
				logger.NASNetworkProtocol,
				logger.SubscriberServiceRequest,
				logger.DirectionInbound,
				rawGmmNasMessage(gmmMessage),
				zap.String("imsi", amfUe.Supi),
				zap.String("ran", amfUe.RanUe[models.AccessType3GPPAccess].Ran.Name),
				zap.String("plmnID", amfUe.PlmnID.Mcc+amfUe.PlmnID.Mnc),
			)
			if err := HandleServiceRequest(ctx, amfUe, accessType, gmmMessage.ServiceRequest); err != nil {
				logger.AmfLog.Error("Error handling service request", zap.Error(err))
			}
		default:
			amfUe.GmmLog.Error("state mismatch: receive gmm message", zap.String("message type", fmt.Sprintf("0x%0x", gmmMessage.GetMessageType())), zap.Any("state", state.Current()))
		}
	case NwInitiatedDeregistrationEvent:
		amfUe := args[ArgAmfUe].(*context.AmfUe)
		accessType := args[ArgAccessType].(models.AccessType)
		if err := NetworkInitiatedDeregistrationProcedure(ctx, amfUe, accessType); err != nil {
			logger.AmfLog.Error("Error handling network initiated deregistration", zap.Error(err))
		}
	case StartAuthEvent:
		logger.AmfLog.Debug("StartAuthEvent")
	case fsm.ExitEvent:
		logger.AmfLog.Debug("ExitEvent")
	default:
		logger.AmfLog.Error("Unknown event", zap.Any("event", event))
	}
}

func Registered(ctx ctxt.Context, state *fsm.State, event fsm.EventType, args fsm.ArgsType) {
	switch event {
	case fsm.EntryEvent:
		// clear stored registration request data for this registration
		amfUe := args[ArgAmfUe].(*context.AmfUe)
		accessType := args[ArgAccessType].(models.AccessType)
		amfUe.ClearRegistrationRequestData(accessType)
		amfUe.GmmLog.Debug("EntryEvent at GMM State[Registered]")
		// store context in DB. Registration procedure is complete.
	case GmmMessageEvent:
		amfUe := args[ArgAmfUe].(*context.AmfUe)
		procedureCode := args[ArgProcedureCode].(int64)
		gmmMessage := args[ArgNASMessage].(*nas.GmmMessage)
		accessType := args[ArgAccessType].(models.AccessType)
		amfUe.GmmLog.Debug("GmmMessageEvent at GMM State[Registered]")
		switch gmmMessage.GetMessageType() {
		// Mobility Registration update / Periodic Registration update
		case nas.MsgTypeRegistrationRequest:
			logger.LogNetworkEvent(
				logger.NASNetworkProtocol,
				logger.SubscriberRegistrationRequest,
				logger.DirectionInbound,
				rawGmmNasMessage(gmmMessage),
				zap.String("imsi", amfUe.Supi),
				zap.String("ran", amfUe.RanUe[accessType].Ran.Name),
				zap.String("plmnID", amfUe.PlmnID.Mcc+amfUe.PlmnID.Mnc),
			)
			if err := HandleRegistrationRequest(ctx, amfUe, accessType, procedureCode, gmmMessage.RegistrationRequest); err != nil {
				logger.AmfLog.Error("Error handling registration request", zap.Error(err))
			} else {
				if err := GmmFSM.SendEvent(ctx, state, StartAuthEvent, fsm.ArgsType{
					ArgAmfUe:         amfUe,
					ArgAccessType:    accessType,
					ArgProcedureCode: procedureCode,
				}); err != nil {
					logger.AmfLog.Error("Error sending event", zap.Error(err))
				}
			}
		case nas.MsgTypeULNASTransport:
			if err := HandleULNASTransport(ctx, amfUe, accessType, gmmMessage.ULNASTransport); err != nil {
				logger.AmfLog.Error("Error handling UL NASTransport", zap.Error(err))
			}
		case nas.MsgTypeConfigurationUpdateComplete:
			logger.LogNetworkEvent(
				logger.NASNetworkProtocol,
				logger.SubscriberConfigurationUpdateComplete,
				logger.DirectionInbound,
				rawGmmNasMessage(gmmMessage),
				zap.String("imsi", amfUe.Supi),
				zap.String("ran", amfUe.RanUe[models.AccessType3GPPAccess].Ran.Name),
				zap.String("plmnID", amfUe.PlmnID.Mcc+amfUe.PlmnID.Mnc),
			)
			if err := HandleConfigurationUpdateComplete(amfUe, gmmMessage.ConfigurationUpdateComplete); err != nil {
				logger.AmfLog.Error("Error handling configuration update complete", zap.Error(err))
			}
		case nas.MsgTypeServiceRequest:
			logger.LogNetworkEvent(
				logger.NASNetworkProtocol,
				logger.SubscriberServiceRequest,
				logger.DirectionInbound,
				rawGmmNasMessage(gmmMessage),
				zap.String("imsi", amfUe.Supi),
				zap.String("ran", amfUe.RanUe[models.AccessType3GPPAccess].Ran.Name),
				zap.String("plmnID", amfUe.PlmnID.Mcc+amfUe.PlmnID.Mnc),
			)
			if err := HandleServiceRequest(ctx, amfUe, accessType, gmmMessage.ServiceRequest); err != nil {
				logger.AmfLog.Error("Error handling service request", zap.Error(err))
			}
		case nas.MsgTypeNotificationResponse:
			logger.LogNetworkEvent(
				logger.NASNetworkProtocol,
				logger.SubscriberNotificationResponse,
				logger.DirectionInbound,
				rawGmmNasMessage(gmmMessage),
				zap.String("imsi", amfUe.Supi),
				zap.String("ran", amfUe.RanUe[models.AccessType3GPPAccess].Ran.Name),
				zap.String("plmnID", amfUe.PlmnID.Mcc+amfUe.PlmnID.Mnc),
			)
			if err := HandleNotificationResponse(ctx, amfUe, gmmMessage.NotificationResponse); err != nil {
				logger.AmfLog.Error("Error handling notification response", zap.Error(err))
			}
		case nas.MsgTypeDeregistrationRequestUEOriginatingDeregistration:
			if err := GmmFSM.SendEvent(ctx, state, InitDeregistrationEvent, fsm.ArgsType{
				ArgAmfUe:      amfUe,
				ArgAccessType: accessType,
				ArgNASMessage: gmmMessage,
			}); err != nil {
				logger.AmfLog.Error("Error sending event", zap.Error(err))
			}
		case nas.MsgTypeStatus5GMM:
			logger.LogNetworkEvent(
				logger.NASNetworkProtocol,
				logger.SubscriberStatus5GMM,
				logger.DirectionInbound,
				rawGmmNasMessage(gmmMessage),
				zap.String("imsi", amfUe.Supi),
				zap.String("ran", amfUe.RanUe[accessType].Ran.Name),
				zap.String("plmnID", amfUe.PlmnID.Mcc+amfUe.PlmnID.Mnc),
			)
			if err := HandleStatus5GMM(amfUe, accessType, gmmMessage.Status5GMM); err != nil {
				logger.AmfLog.Error("Error handling status 5GMM", zap.Error(err))
			}
		default:
			amfUe.GmmLog.Error("state mismatch: receive gmm message", zap.String("message type", fmt.Sprintf("0x%0x", gmmMessage.GetMessageType())), zap.Any("state", state.Current()))
		}
	case StartAuthEvent:
		logger.AmfLog.Debug("StartAuthEvent")
	case InitDeregistrationEvent:
		logger.AmfLog.Debug("InitDeregistrationEvent")
	case NwInitiatedDeregistrationEvent:
		amfUe := args[ArgAmfUe].(*context.AmfUe)
		accessType := args[ArgAccessType].(models.AccessType)
		if err := NetworkInitiatedDeregistrationProcedure(ctx, amfUe, accessType); err != nil {
			logger.AmfLog.Error("Error handling network initiated deregistration", zap.Error(err))
		}
	case SliceInfoAddEvent:
	case SliceInfoDeleteEvent:
	case fsm.ExitEvent:
		logger.AmfLog.Debug("ExitEvent")
	default:
		logger.AmfLog.Error("Unknown event", zap.Any("event", event))
	}
}

func Authentication(ctx ctxt.Context, state *fsm.State, event fsm.EventType, args fsm.ArgsType) {
	var amfUe *context.AmfUe
	switch event {
	case fsm.EntryEvent:
		amfUe = args[ArgAmfUe].(*context.AmfUe)
		amfUe.GmmLog = amfUe.GmmLog.With(zap.String("suci", amfUe.Suci))
		amfUe.TxLog = amfUe.TxLog.With(zap.String("suci", amfUe.Suci))
		amfUe.GmmLog.Debug("EntryEvent at GMM State[Authentication]")
		fallthrough
	case AuthRestartEvent:
		amfUe = args[ArgAmfUe].(*context.AmfUe)
		accessType := args[ArgAccessType].(models.AccessType)
		amfUe.GmmLog.Debug("AuthRestartEvent at GMM State[Authentication]")

		pass, err := AuthenticationProcedure(ctx, amfUe, accessType)
		if err != nil {
			if err := GmmFSM.SendEvent(ctx, state, AuthErrorEvent, fsm.ArgsType{
				ArgAmfUe:      amfUe,
				ArgAccessType: accessType,
			}); err != nil {
				logger.AmfLog.Error("Error sending event", zap.Error(err))
			}
		}
		if pass {
			if err := GmmFSM.SendEvent(ctx, state, AuthSuccessEvent, fsm.ArgsType{
				ArgAmfUe:      amfUe,
				ArgAccessType: accessType,
			}); err != nil {
				logger.AmfLog.Error("Error sending event", zap.Error(err))
			}
		}
	case GmmMessageEvent:
		amfUe = args[ArgAmfUe].(*context.AmfUe)
		gmmMessage := args[ArgNASMessage].(*nas.GmmMessage)
		accessType := args[ArgAccessType].(models.AccessType)
		amfUe.GmmLog.Debug("GmmMessageEvent at GMM State[Authentication]")

		switch gmmMessage.GetMessageType() {
		case nas.MsgTypeIdentityResponse:
			if err := HandleIdentityResponse(amfUe, gmmMessage.IdentityResponse); err != nil {
				logger.AmfLog.Error("Error handling identity response", zap.Error(err))
			}
			err := GmmFSM.SendEvent(ctx, state, AuthRestartEvent, fsm.ArgsType{ArgAmfUe: amfUe, ArgAccessType: accessType})
			if err != nil {
				logger.AmfLog.Error("Error sending event", zap.Error(err))
			}
		case nas.MsgTypeAuthenticationResponse:
			logger.LogNetworkEvent(
				logger.NASNetworkProtocol,
				logger.SubscriberAuthenticationResponse,
				logger.DirectionInbound,
				rawGmmNasMessage(gmmMessage),
				zap.String("imsi", amfUe.Supi),
				zap.String("ran", amfUe.RanUe[accessType].Ran.Name),
				zap.String("plmnID", amfUe.PlmnID.Mcc+amfUe.PlmnID.Mnc),
			)
			if err := HandleAuthenticationResponse(ctx, amfUe, accessType, gmmMessage.AuthenticationResponse); err != nil {
				logger.AmfLog.Error("Error handling authentication response", zap.Error(err))
			}
		case nas.MsgTypeAuthenticationFailure:
			logger.LogNetworkEvent(
				logger.NASNetworkProtocol,
				logger.SubscriberAuthenticationFailure,
				logger.DirectionInbound,
				rawGmmNasMessage(gmmMessage),
				zap.String("imsi", amfUe.Supi),
				zap.String("ran", amfUe.RanUe[accessType].Ran.Name),
				zap.String("plmnID", amfUe.PlmnID.Mcc+amfUe.PlmnID.Mnc),
			)
			if err := HandleAuthenticationFailure(ctx, amfUe, accessType, gmmMessage.AuthenticationFailure); err != nil {
				logger.AmfLog.Error("Error handling authentication failure", zap.Error(err))
			}
		case nas.MsgTypeStatus5GMM:
			logger.LogNetworkEvent(
				logger.NASNetworkProtocol,
				logger.SubscriberStatus5GMM,
				logger.DirectionInbound,
				rawGmmNasMessage(gmmMessage),
				zap.String("imsi", amfUe.Supi),
				zap.String("ran", amfUe.RanUe[accessType].Ran.Name),
				zap.String("plmnID", amfUe.PlmnID.Mcc+amfUe.PlmnID.Mnc),
			)
			if err := HandleStatus5GMM(amfUe, accessType, gmmMessage.Status5GMM); err != nil {
				logger.AmfLog.Error("Error handling status 5GMM", zap.Error(err))
			}
		default:
			logger.AmfLog.Error("state mismatch: receive gmm message", zap.String("message type", fmt.Sprintf("0x%0x", gmmMessage.GetMessageType())), zap.Any("state", state.Current()))
			// called SendEvent() to move to deregistered state if state mismatch occurs
			err := GmmFSM.SendEvent(ctx, state, AuthFailEvent, fsm.ArgsType{
				ArgAmfUe:      amfUe,
				ArgAccessType: accessType,
			})
			if err != nil {
				logger.AmfLog.Error("Error sending event", zap.Error(err))
			} else {
				amfUe.GmmLog.Info("state reset to Deregistered")
			}
		}
	case AuthSuccessEvent:
		logger.AmfLog.Debug("AuthSuccessEvent")
	case AuthFailEvent:
		logger.AmfLog.Debug("AuthFailEvent")
		logger.AmfLog.Warn("Reject authentication")
	case AuthErrorEvent:
		amfUe := args[ArgAmfUe].(*context.AmfUe)
		accessType := args[ArgAccessType].(models.AccessType)

		if err := HandleAuthenticationError(amfUe, accessType); err != nil {
			logger.AmfLog.Error("Error handling authentication error", zap.Error(err))
		}
	case NwInitiatedDeregistrationEvent:
		amfUe := args[ArgAmfUe].(*context.AmfUe)
		accessType := args[ArgAccessType].(models.AccessType)
		if err := NetworkInitiatedDeregistrationProcedure(ctx, amfUe, accessType); err != nil {
			logger.AmfLog.Error("Error handling network initiated deregistration", zap.Error(err))
		}
	case fsm.ExitEvent:
		// clear authentication related data at exit
		amfUe := args[ArgAmfUe].(*context.AmfUe)
		amfUe.GmmLog.Debug("ExitEvent")
		amfUe.AuthenticationCtx = nil
		amfUe.AuthFailureCauseSynchFailureTimes = 0
	default:
		logger.AmfLog.Error("Unknown event", zap.Any("event", event))
	}
}

func SecurityMode(ctx ctxt.Context, state *fsm.State, event fsm.EventType, args fsm.ArgsType) {
	switch event {
	case fsm.EntryEvent:
		amfUe := args[ArgAmfUe].(*context.AmfUe)
		accessType := args[ArgAccessType].(models.AccessType)
		// set log information
		amfUe.NASLog = amfUe.NASLog.With(zap.String("supi", amfUe.Supi))
		amfUe.TxLog = amfUe.NASLog.With(zap.String("supi", amfUe.Supi))
		amfUe.GmmLog = amfUe.GmmLog.With(zap.String("supi", amfUe.Supi))
		amfUe.ProducerLog = amfUe.GmmLog.With(zap.String("supi", amfUe.Supi))
		amfUe.GmmLog.Debug("EntryEvent at GMM State[SecurityMode]")
		if amfUe.SecurityContextIsValid() {
			amfUe.GmmLog.Debug("UE has a valid security context - skip security mode control procedure")
			if err := GmmFSM.SendEvent(ctx, state, SecurityModeSuccessEvent, fsm.ArgsType{
				ArgAmfUe:      amfUe,
				ArgAccessType: accessType,
				ArgNASMessage: amfUe.RegistrationRequest,
			}); err != nil {
				logger.AmfLog.Error("Error sending event", zap.Error(err))
			}
		} else {
			eapSuccess := args[ArgEAPSuccess].(bool)
			eapMessage := args[ArgEAPMessage].(string)
			// Select enc/int algorithm based on ue security capability & amf's policy,
			amfSelf := context.AMFSelf()
			amfUe.SelectSecurityAlg(amfSelf.SecurityAlgorithm.IntegrityOrder, amfSelf.SecurityAlgorithm.CipheringOrder)
			// Generate KnasEnc, KnasInt
			amfUe.DerivateAlgKey()
			if amfUe.CipheringAlg == security.AlgCiphering128NEA0 && amfUe.IntegrityAlg == security.AlgIntegrity128NIA0 {
				err := GmmFSM.SendEvent(ctx, state, SecuritySkipEvent, fsm.ArgsType{
					ArgAmfUe:      amfUe,
					ArgAccessType: accessType,
					ArgNASMessage: amfUe.RegistrationRequest,
				})
				if err != nil {
					logger.AmfLog.Error("Error sending event", zap.Error(err))
				}
			} else {
				err := gmm_message.SendSecurityModeCommand(amfUe.RanUe[accessType], eapSuccess, eapMessage)
				if err != nil {
					logger.AmfLog.Error("error sending security mode command", zap.Error(err))
				}
				logger.AmfLog.Info("Sent GMM security mode command to UE")
			}
		}
	case GmmMessageEvent:
		amfUe := args[ArgAmfUe].(*context.AmfUe)
		procedureCode := args[ArgProcedureCode].(int64)
		gmmMessage := args[ArgNASMessage].(*nas.GmmMessage)
		accessType := args[ArgAccessType].(models.AccessType)
		amfUe.GmmLog.Debug("GmmMessageEvent to GMM State[SecurityMode]")
		switch gmmMessage.GetMessageType() {
		case nas.MsgTypeSecurityModeComplete:
			logger.LogNetworkEvent(
				logger.NASNetworkProtocol,
				logger.SubscriberSecurityModeComplete,
				logger.DirectionInbound,
				rawGmmNasMessage(gmmMessage),
				zap.String("imsi", amfUe.Supi),
				zap.String("ran", amfUe.RanUe[accessType].Ran.Name),
				zap.String("plmnID", amfUe.PlmnID.Mcc+amfUe.PlmnID.Mnc),
			)
			if err := HandleSecurityModeComplete(ctx, amfUe, accessType, procedureCode, gmmMessage.SecurityModeComplete); err != nil {
				logger.AmfLog.Error("Error handling security mode complete", zap.Error(err))
			}
		case nas.MsgTypeSecurityModeReject:
			logger.LogNetworkEvent(
				logger.NASNetworkProtocol,
				logger.SubscriberSecurityModeReject,
				logger.DirectionInbound,
				rawGmmNasMessage(gmmMessage),
				zap.String("imsi", amfUe.Supi),
				zap.String("ran", amfUe.RanUe[accessType].Ran.Name),
				zap.String("plmnID", amfUe.PlmnID.Mcc+amfUe.PlmnID.Mnc),
			)
			if err := HandleSecurityModeReject(amfUe, accessType, gmmMessage.SecurityModeReject); err != nil {
				logger.AmfLog.Error("Error handling security mode reject", zap.Error(err))
			}
			err := GmmFSM.SendEvent(ctx, state, SecurityModeFailEvent, fsm.ArgsType{
				ArgAmfUe:      amfUe,
				ArgAccessType: accessType,
			})
			if err != nil {
				logger.AmfLog.Error("Error sending event", zap.Error(err))
			}
		case nas.MsgTypeRegistrationRequest:
			// Sending AbortEvent to ongoing procedure
			err := GmmFSM.SendEvent(ctx, state, SecurityModeAbortEvent, fsm.ArgsType{
				ArgAmfUe:      amfUe,
				ArgAccessType: accessType,
			})
			if err != nil {
				logger.AmfLog.Error("Error sending event", zap.Error(err))
			}

			err = GmmFSM.SendEvent(ctx, state, GmmMessageEvent, fsm.ArgsType{
				ArgAmfUe:         amfUe,
				ArgAccessType:    accessType,
				ArgNASMessage:    gmmMessage,
				ArgProcedureCode: procedureCode,
			})
			if err != nil {
				logger.AmfLog.Error("Error sending event", zap.Error(err))
			}

		case nas.MsgTypeStatus5GMM:
			logger.LogNetworkEvent(
				logger.NASNetworkProtocol,
				logger.SubscriberStatus5GMM,
				logger.DirectionInbound,
				rawGmmNasMessage(gmmMessage),
				zap.String("imsi", amfUe.Supi),
				zap.String("ran", amfUe.RanUe[accessType].Ran.Name),
				zap.String("plmnID", amfUe.PlmnID.Mcc+amfUe.PlmnID.Mnc),
			)
			if err := HandleStatus5GMM(amfUe, accessType, gmmMessage.Status5GMM); err != nil {
				logger.AmfLog.Error("Error handling status 5GMM", zap.Error(err))
			}
		default:
			amfUe.GmmLog.Error("state mismatch: receive gmm message", zap.String("message type", fmt.Sprintf("0x%0x", gmmMessage.GetMessageType())), zap.Any("state", state.Current()))
			// called SendEvent() to move to deregistered state if state mismatch occurs
			err := GmmFSM.SendEvent(ctx, state, SecurityModeFailEvent, fsm.ArgsType{
				ArgAmfUe:      amfUe,
				ArgAccessType: accessType,
			})
			if err != nil {
				logger.AmfLog.Error("Error sending event", zap.Error(err))
			} else {
				amfUe.GmmLog.Info("state reset to Deregistered")
			}
		}
	case SecurityModeAbortEvent:
		logger.AmfLog.Debug("SecurityModeAbortEvent")
		amfUe := args[ArgAmfUe].(*context.AmfUe)
		// stopping security mode command timer
		amfUe.SecurityContextAvailable = false
		amfUe.T3560.Stop()
		amfUe.T3560 = nil
	case NwInitiatedDeregistrationEvent:
		logger.AmfLog.Debug("NwInitiatedDeregistrationEvent")
		amfUe := args[ArgAmfUe].(*context.AmfUe)
		accessType := args[ArgAccessType].(models.AccessType)
		amfUe.T3560.Stop()
		amfUe.T3560 = nil
		if err := NetworkInitiatedDeregistrationProcedure(ctx, amfUe, accessType); err != nil {
			logger.AmfLog.Error("Error handling network initiated deregistration", zap.Error(err))
		}
	case SecurityModeSuccessEvent:
		logger.AmfLog.Debug("SecurityModeSuccessEvent")
	case SecurityModeFailEvent:
		logger.AmfLog.Debug("SecurityModeFailEvent")
	case fsm.ExitEvent:
		logger.AmfLog.Debug("ExitEvent")
		return
	default:
		logger.AmfLog.Error("Unknown event", zap.Any("event", event))
	}
}

func rawGmmNasMessage(gmmMsg *nas.GmmMessage) []byte {
	msg := nas.Message{
		GmmMessage: gmmMsg,
	}

	data := new(bytes.Buffer)

	err := msg.GmmMessageEncode(data)
	if err != nil {
		logger.AmfLog.Error("Error encoding NAS message", zap.Error(err))
		return nil
	}

	return data.Bytes()
}

func ContextSetup(ctx ctxt.Context, state *fsm.State, event fsm.EventType, args fsm.ArgsType) {
	switch event {
	case fsm.EntryEvent:
		amfUe := args[ArgAmfUe].(*context.AmfUe)
		gmmMessage := args[ArgNASMessage]
		accessType := args[ArgAccessType].(models.AccessType)
		amfUe.GmmLog.Debug("EntryEvent at GMM State[ContextSetup]")
		switch message := gmmMessage.(type) {
		case *nasMessage.RegistrationRequest:
			amfUe.RegistrationRequest = message
			switch amfUe.RegistrationType5GS {
			case nasMessage.RegistrationType5GSInitialRegistration:
				gmmMessage := &nas.GmmMessage{RegistrationRequest: message}
				gmmMessage.GmmHeader.SetMessageType(nas.MsgTypeRegistrationRequest)
				logger.LogNetworkEvent(
					logger.NASNetworkProtocol,
					logger.SubscriberRegistrationRequest,
					logger.DirectionInbound,
					rawGmmNasMessage(gmmMessage),
					zap.String("imsi", amfUe.Supi),
					zap.String("ran", amfUe.RanUe[accessType].Ran.Name),
					zap.String("plmnID", amfUe.PlmnID.Mcc+amfUe.PlmnID.Mnc),
				)
				if err := HandleInitialRegistration(ctx, amfUe, accessType); err != nil {
					logger.AmfLog.Error("Error handling initial registration", zap.Error(err))
				}
			case nasMessage.RegistrationType5GSMobilityRegistrationUpdating:
				fallthrough
			case nasMessage.RegistrationType5GSPeriodicRegistrationUpdating:
				nasMessage := &nas.GmmMessage{RegistrationRequest: message}
				nasMessage.GmmHeader.SetMessageType(nas.MsgTypeRegistrationRequest)
				logger.LogNetworkEvent(
					logger.NASNetworkProtocol,
					logger.SubscriberRegistrationRequest,
					logger.DirectionInbound,
					rawGmmNasMessage(nasMessage),
					zap.String("imsi", amfUe.Supi),
					zap.String("ran", amfUe.RanUe[accessType].Ran.Name),
					zap.String("plmnID", amfUe.PlmnID.Mcc+amfUe.PlmnID.Mnc),
				)
				if err := HandleMobilityAndPeriodicRegistrationUpdating(ctx, amfUe, accessType); err != nil {
					logger.AmfLog.Error("Error handling mobility and periodic registration updating", zap.Error(err))
				}
			}
		case *nasMessage.ServiceRequest:
			nasMessage := &nas.GmmMessage{ServiceRequest: message}
			nasMessage.GmmHeader.SetMessageType(nas.MsgTypeServiceRequest)
			logger.LogNetworkEvent(
				logger.NASNetworkProtocol,
				logger.SubscriberServiceRequest,
				logger.DirectionInbound,
				rawGmmNasMessage(nasMessage),
				zap.String("imsi", amfUe.Supi),
				zap.String("ran", amfUe.RanUe[accessType].Ran.Name),
				zap.String("plmnID", amfUe.PlmnID.Mcc+amfUe.PlmnID.Mnc),
			)
			if err := HandleServiceRequest(ctx, amfUe, accessType, message); err != nil {
				logger.AmfLog.Error("Error handling service request", zap.Error(err))
			}
		default:
			logger.AmfLog.Error("UE state mismatch: receieve wrong gmm message")
		}
	case GmmMessageEvent:
		amfUe := args[ArgAmfUe].(*context.AmfUe)
		gmmMessage := args[ArgNASMessage].(*nas.GmmMessage)
		accessType := args[ArgAccessType].(models.AccessType)
		amfUe.GmmLog.Debug("GmmMessageEvent at GMM State[ContextSetup]")
		switch gmmMessage.GetMessageType() {
		case nas.MsgTypeIdentityResponse:
			logger.LogNetworkEvent(
				logger.NASNetworkProtocol,
				logger.SubscriberIdentityResponse,
				logger.DirectionInbound,
				rawGmmNasMessage(gmmMessage),
				zap.String("imsi", amfUe.Supi),
				zap.String("ran", amfUe.RanUe[accessType].Ran.Name),
				zap.String("plmnID", amfUe.PlmnID.Mcc+amfUe.PlmnID.Mnc),
			)
			if err := HandleIdentityResponse(amfUe, gmmMessage.IdentityResponse); err != nil {
				logger.AmfLog.Error("Error handling identity response", zap.Error(err))
			}
			switch amfUe.RegistrationType5GS {
			case nasMessage.RegistrationType5GSInitialRegistration:
				logger.LogNetworkEvent(
					logger.NASNetworkProtocol,
					logger.SubscriberRegistrationRequest,
					logger.DirectionInbound,
					rawGmmNasMessage(gmmMessage),
					zap.String("imsi", amfUe.Supi),
					zap.String("ran", amfUe.RanUe[accessType].Ran.Name),
					zap.String("plmnID", amfUe.PlmnID.Mcc+amfUe.PlmnID.Mnc),
				)
				if err := HandleInitialRegistration(ctx, amfUe, accessType); err != nil {
					logger.AmfLog.Error("Error handling initial registration", zap.Error(err))
					err = GmmFSM.SendEvent(ctx, state, ContextSetupFailEvent, fsm.ArgsType{
						ArgAmfUe:      amfUe,
						ArgAccessType: accessType,
					})
					if err != nil {
						logger.AmfLog.Error("Error sending event", zap.Error(err))
					}
				}
			case nasMessage.RegistrationType5GSMobilityRegistrationUpdating:
				fallthrough
			case nasMessage.RegistrationType5GSPeriodicRegistrationUpdating:
				logger.LogNetworkEvent(
					logger.NASNetworkProtocol,
					logger.SubscriberRegistrationRequest,
					logger.DirectionInbound,
					rawGmmNasMessage(gmmMessage),
					zap.String("imsi", amfUe.Supi),
					zap.String("ran", amfUe.RanUe[accessType].Ran.Name),
					zap.String("plmnID", amfUe.PlmnID.Mcc+amfUe.PlmnID.Mnc),
				)
				if err := HandleMobilityAndPeriodicRegistrationUpdating(ctx, amfUe, accessType); err != nil {
					logger.AmfLog.Error("Error handling mobility and periodic registration updating", zap.Error(err))
					err = GmmFSM.SendEvent(ctx, state, ContextSetupFailEvent, fsm.ArgsType{
						ArgAmfUe:      amfUe,
						ArgAccessType: accessType,
					})
					if err != nil {
						logger.AmfLog.Error("Error sending event", zap.Error(err))
					}
				}
			}
		case nas.MsgTypeRegistrationComplete:
			logger.LogNetworkEvent(
				logger.NASNetworkProtocol,
				logger.SubscriberRegistrationComplete,
				logger.DirectionInbound,
				rawGmmNasMessage(gmmMessage),
				zap.String("imsi", amfUe.Supi),
				zap.String("ran", amfUe.RanUe[accessType].Ran.Name),
				zap.String("plmnID", amfUe.PlmnID.Mcc+amfUe.PlmnID.Mnc),
			)
			if err := HandleRegistrationComplete(ctx, amfUe, accessType, gmmMessage.RegistrationComplete); err != nil {
				logger.AmfLog.Error("Error handling registration complete", zap.Error(err))
			}
		case nas.MsgTypeStatus5GMM:
			logger.LogNetworkEvent(
				logger.NASNetworkProtocol,
				logger.SubscriberStatus5GMM,
				logger.DirectionInbound,
				rawGmmNasMessage(gmmMessage),
				zap.String("imsi", amfUe.Supi),
				zap.String("ran", amfUe.RanUe[accessType].Ran.Name),
				zap.String("plmnID", amfUe.PlmnID.Mcc+amfUe.PlmnID.Mnc),
			)
			if err := HandleStatus5GMM(amfUe, accessType, gmmMessage.Status5GMM); err != nil {
				logger.AmfLog.Error("Error handling status 5GMM", zap.Error(err))
			}
		default:
			amfUe.GmmLog.Error("state mismatch: receive gmm message", zap.String("message type", fmt.Sprintf("0x%0x", gmmMessage.GetMessageType())), zap.Any("state", state.Current()))
			msgType := gmmMessage.GetMessageType()
			if msgType == nas.MsgTypeRegistrationRequest {
				// called SendEvent() to move to deregistered state if state mismatch occurs
				err := GmmFSM.SendEvent(ctx, state, ContextSetupFailEvent, fsm.ArgsType{
					ArgAmfUe:      amfUe,
					ArgAccessType: accessType,
				})
				if err != nil {
					logger.AmfLog.Error("Error sending event", zap.Error(err))
				} else {
					amfUe.GmmLog.Info("state reset to Deregistered")
				}
			}
		}
	case ContextSetupSuccessEvent:
		logger.AmfLog.Debug("ContextSetupSuccessEvent")
	case NwInitiatedDeregistrationEvent:
		logger.AmfLog.Debug("NwInitiatedDeregistrationEvent")
		amfUe := args[ArgAmfUe].(*context.AmfUe)
		accessType := args[ArgAccessType].(models.AccessType)
		amfUe.T3550.Stop()
		amfUe.T3550 = nil
		amfUe.State[accessType].Set(context.Registered)
		if err := NetworkInitiatedDeregistrationProcedure(ctx, amfUe, accessType); err != nil {
			logger.AmfLog.Error("Error handling network initiated deregistration", zap.Error(err))
		}
	case ContextSetupFailEvent:
		logger.AmfLog.Debug("ContextSetupFailEvent")
	case fsm.ExitEvent:
		logger.AmfLog.Debug("ExitEvent")
	default:
		logger.AmfLog.Error("Unknown event", zap.Any("event", event))
	}
}

func DeregisteredInitiated(ctx ctxt.Context, state *fsm.State, event fsm.EventType, args fsm.ArgsType) {
	switch event {
	case fsm.EntryEvent:
		amfUe := args[ArgAmfUe].(*context.AmfUe)
		if args[ArgNASMessage] != nil {
			gmmMessage := args[ArgNASMessage].(*nas.GmmMessage)
			if gmmMessage != nil {
				accessType := args[ArgAccessType].(models.AccessType)
				logger.LogNetworkEvent(
					logger.NASNetworkProtocol,
					logger.SubscriberDeregistrationRequest,
					logger.DirectionInbound,
					rawGmmNasMessage(gmmMessage),
					zap.String("imsi", amfUe.Supi),
					zap.String("ran", amfUe.RanUe[accessType].Ran.Name),
					zap.String("plmnID", amfUe.PlmnID.Mcc+amfUe.PlmnID.Mnc),
				)
				if err := HandleDeregistrationRequest(ctx, amfUe, accessType,
					gmmMessage.DeregistrationRequestUEOriginatingDeregistration); err != nil {
					logger.AmfLog.Error("Error handling deregistration request", zap.Error(err))
				}
			}
		}
	case GmmMessageEvent:
		amfUe := args[ArgAmfUe].(*context.AmfUe)
		gmmMessage := args[ArgNASMessage].(*nas.GmmMessage)
		accessType := args[ArgAccessType].(models.AccessType)
		amfUe.GmmLog.Debug("GmmMessageEvent at GMM State[DeregisteredInitiated]")
		switch gmmMessage.GetMessageType() {
		case nas.MsgTypeDeregistrationAcceptUETerminatedDeregistration:
			logger.LogNetworkEvent(
				logger.NASNetworkProtocol,
				logger.SubscriberDeregistrationAccept,
				logger.DirectionInbound,
				rawGmmNasMessage(gmmMessage),
				zap.String("imsi", amfUe.Supi),
				zap.String("ran", amfUe.RanUe[accessType].Ran.Name),
				zap.String("plmnID", amfUe.PlmnID.Mcc+amfUe.PlmnID.Mnc),
			)
			if err := HandleDeregistrationAccept(ctx, amfUe, accessType,
				gmmMessage.DeregistrationAcceptUETerminatedDeregistration); err != nil {
				logger.AmfLog.Error("Error handling deregistration accept", zap.Error(err))
			}
		default:
			amfUe.GmmLog.Error("state mismatch: receive gmm message", zap.String("message type", fmt.Sprintf("0x%0x", gmmMessage.GetMessageType())), zap.Any("state", state.Current()))
		}
	case DeregistrationAcceptEvent:
		amfUe := args[ArgAmfUe].(*context.AmfUe)
		accessType := args[ArgAccessType].(models.AccessType)
		SetDeregisteredState(amfUe, AnTypeToNas(accessType))
		logger.AmfLog.Debug("DeregistrationAcceptEvent")
	case fsm.ExitEvent:
		logger.AmfLog.Debug("ExitEvent")
	default:
		logger.AmfLog.Error("Unknown event", zap.Any("event", event))
	}
}

func SetDeregisteredState(amfUe *context.AmfUe, anType uint8) {
	amfUe.SubscriptionDataValid = false
	if anType == nasMessage.AccessType3GPP {
		amfUe.GmmLog.Debug("UE accessType[3GPP] transfer to Deregistered state")
		amfUe.State[models.AccessType3GPPAccess].Set(context.Deregistered)
	} else if anType == nasMessage.AccessTypeNon3GPP {
		amfUe.GmmLog.Debug("UE accessType[Non3GPP] transfer to Deregistered state")
		amfUe.State[models.AccessTypeNon3GPPAccess].Set(context.Deregistered)
	} else {
		amfUe.GmmLog.Debug("UE accessType[3GPP] transfer to Deregistered state")
		amfUe.State[models.AccessType3GPPAccess].Set(context.Deregistered)
		amfUe.GmmLog.Debug("UE accessType[Non3GPP] transfer to Deregistered state")
		amfUe.State[models.AccessTypeNon3GPPAccess].Set(context.Deregistered)
	}
}
