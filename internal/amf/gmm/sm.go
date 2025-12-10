// Copyright 2024 Ella Networks
// SPDX-FileCopyrightText: 2022-present Intel Corporation
// SPDX-FileCopyrightText: 2021 Open Networking Foundation <info@opennetworking.org>
// Copyright 2019 free5GC.org
//
// SPDX-License-Identifier: Apache-2.0

package gmm

import (
	ctxt "context"
	"fmt"

	"github.com/ellanetworks/core/internal/amf/context"
	gmm_message "github.com/ellanetworks/core/internal/amf/gmm/message"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/util/fsm"
	"github.com/free5gc/nas"
	"github.com/free5gc/nas/nasMessage"
	"github.com/free5gc/nas/security"
	"go.opentelemetry.io/otel/attribute"
	"go.uber.org/zap"
)

func handleRegistrationRequest(ctx ctxt.Context, ue *context.AmfUe, msg *nas.GmmMessage) error {
	switch ue.State.Current() {
	case context.Deregistered, context.Registered:
		if err := HandleRegistrationRequest(ctx, ue, msg.RegistrationRequest); err != nil {
			return fmt.Errorf("failed handling registration request")
		}

		err := GmmFSM.SendEvent(ctx, ue.State, StartAuthEvent, fsm.ArgsType{ArgAmfUe: ue})
		if err != nil {
			return fmt.Errorf("error sending event: %v", err)
		}
	case context.SecurityMode:
		err := GmmFSM.SendEvent(ctx, ue.State, SecurityModeAbortEvent, fsm.ArgsType{
			ArgAmfUe: ue,
		})
		if err != nil {
			return fmt.Errorf("error sending event: %v", err)
		}

		return HandleGmmMessage(ctx, ue, msg)
	case context.ContextSetup:
		err := GmmFSM.SendEvent(ctx, ue.State, ContextSetupFailEvent, fsm.ArgsType{
			ArgAmfUe: ue,
		})
		if err != nil {
			return fmt.Errorf("error sending event: %v", err)
		}

		ue.GmmLog.Info("state reset to Deregistered")
	}

	return nil
}

func handleServiceRequest(ctx ctxt.Context, ue *context.AmfUe, msg *nas.GmmMessage) error {
	switch ue.State.Current() {
	case context.Deregistered, context.Registered:
		if err := HandleServiceRequest(ctx, ue, msg.ServiceRequest); err != nil {
			return fmt.Errorf("error handling service request: %v", err)
		}
	}

	return nil
}

func handleULNASTransport(ctx ctxt.Context, ue *context.AmfUe, msg *nas.GmmMessage) error {
	switch ue.State.Current() {
	case context.Registered:
		err := HandleULNASTransport(ctx, ue, msg.ULNASTransport)
		if err != nil {
			return fmt.Errorf("error handling UL NASTransport: %v", err)
		}
	}

	return nil
}

func handleConfigurationUpdateComplete(ue *context.AmfUe, msg *nas.GmmMessage) error {
	switch ue.State.Current() {
	case context.Registered:
		err := HandleConfigurationUpdateComplete(ue, msg.ConfigurationUpdateComplete)
		if err != nil {
			return fmt.Errorf("error handling configuration update complete: %v", err)
		}
	}
	return nil
}

func handleNotificationResponse(ctx ctxt.Context, ue *context.AmfUe, msg *nas.GmmMessage) error {
	switch ue.State.Current() {
	case context.Registered:
		err := HandleNotificationResponse(ctx, ue, msg.NotificationResponse)
		if err != nil {
			return fmt.Errorf("error handling notification response: %v", err)
		}
	}
	return nil
}

func handleDeregistrationRequestUEOriginatingDeregistration(ctx ctxt.Context, ue *context.AmfUe, msg *nas.GmmMessage) error {
	switch ue.State.Current() {
	case context.Registered:
		err := GmmFSM.SendEvent(ctx, ue.State, InitDeregistrationEvent, fsm.ArgsType{
			ArgAmfUe:      ue,
			ArgNASMessage: msg,
		})
		if err != nil {
			return fmt.Errorf("error sending event: %v", err)
		}
	}
	return nil
}

func handleStatus5GMM(ue *context.AmfUe, msg *nas.GmmMessage) error {
	switch ue.State.Current() {
	case context.Registered, context.Authentication, context.SecurityMode, context.ContextSetup:
		if err := HandleStatus5GMM(ue, msg.Status5GMM); err != nil {
			return fmt.Errorf("error handling status 5GMM: %v", err)
		}
	}
	return nil
}

func handleIdentityResponse(ctx ctxt.Context, ue *context.AmfUe, msg *nas.GmmMessage) error {
	switch ue.State.Current() {
	case context.Authentication:
		if err := HandleIdentityResponse(ue, msg.IdentityResponse); err != nil {
			return fmt.Errorf("error handling identity response: %v", err)
		}
		err := GmmFSM.SendEvent(ctx, ue.State, AuthRestartEvent, fsm.ArgsType{ArgAmfUe: ue})
		if err != nil {
			return fmt.Errorf("error sending event: %v", err)
		}
	case context.ContextSetup:
		if err := HandleIdentityResponse(ue, msg.IdentityResponse); err != nil {
			return fmt.Errorf("error handling identity response: %v", err)
		}
		switch ue.RegistrationType5GS {
		case nasMessage.RegistrationType5GSInitialRegistration:
			if err := HandleInitialRegistration(ctx, ue); err != nil {
				logger.AmfLog.Error("Error handling initial registration", zap.Error(err))
				err = GmmFSM.SendEvent(ctx, ue.State, ContextSetupFailEvent, fsm.ArgsType{
					ArgAmfUe: ue,
				})
				if err != nil {
					return fmt.Errorf("error sending event: %v", err)
				}
			}
		case nasMessage.RegistrationType5GSMobilityRegistrationUpdating:
			fallthrough
		case nasMessage.RegistrationType5GSPeriodicRegistrationUpdating:
			if err := HandleMobilityAndPeriodicRegistrationUpdating(ctx, ue); err != nil {
				logger.AmfLog.Error("Error handling mobility and periodic registration updating", zap.Error(err))
				err = GmmFSM.SendEvent(ctx, ue.State, ContextSetupFailEvent, fsm.ArgsType{
					ArgAmfUe: ue,
				})
				if err != nil {
					return fmt.Errorf("error sending event: %v", err)
				}
			}
		}
	default:
		return fmt.Errorf("state mismatch: receive Identity Response message in state %s", ue.State.Current())
	}
	return nil
}

func handleAuthenticationResponse(ctx ctxt.Context, ue *context.AmfUe, msg *nas.GmmMessage) error {
	switch ue.State.Current() {
	case context.Authentication:
		err := HandleAuthenticationResponse(ctx, ue, msg.AuthenticationResponse)
		if err != nil {
			return fmt.Errorf("error handling authentication response: %v", err)
		}
	default:
		return fmt.Errorf("state mismatch: receive Authentication Response message in state %s", ue.State.Current())
	}
	return nil
}

func handleAuthenticationFailure(ctx ctxt.Context, ue *context.AmfUe, msg *nas.GmmMessage) error {
	switch ue.State.Current() {
	case context.Authentication:
		err := HandleAuthenticationFailure(ctx, ue, msg.AuthenticationFailure)
		if err != nil {
			return fmt.Errorf("error handling authentication failure :%v", err)
		}
	default:
		return fmt.Errorf("state mismatch: receive Authentication Failure message in state %s", ue.State.Current())
	}

	return nil
}

func handleSecurityModeComplete(ctx ctxt.Context, ue *context.AmfUe, msg *nas.GmmMessage) error {
	switch ue.State.Current() {
	case context.SecurityMode:
		err := HandleSecurityModeComplete(ctx, ue, msg.SecurityModeComplete)
		if err != nil {
			return fmt.Errorf("error handling security mode complete: %v", err)
		}
	default:
		return fmt.Errorf("state mismatch: receive Security Mode Complete message in state %s", ue.State.Current())
	}

	return nil
}

func handleSecurityModeReject(ctx ctxt.Context, ue *context.AmfUe, msg *nas.GmmMessage) error {
	switch ue.State.Current() {
	case context.SecurityMode:
		err := HandleSecurityModeReject(ctx, ue, msg.SecurityModeReject)
		if err != nil {
			return fmt.Errorf("error handling security mode reject: %v", err)
		}

		err = GmmFSM.SendEvent(ctx, ue.State, SecurityModeFailEvent, fsm.ArgsType{
			ArgAmfUe: ue,
		})
		if err != nil {
			return fmt.Errorf("error sending event: %v", err)
		}
	default:
		return fmt.Errorf("state mismatch: receive Security Mode Reject message in state %s", ue.State.Current())
	}
	return nil
}

func handleRegistrationComplete(ctx ctxt.Context, ue *context.AmfUe, msg *nas.GmmMessage) error {
	switch ue.State.Current() {
	case context.ContextSetup:
		if err := HandleRegistrationComplete(ctx, ue, msg.RegistrationComplete); err != nil {
			return fmt.Errorf("error handling registration complete: %v", err)
		}
	default:
		return fmt.Errorf("state mismatch: receive Registration Complete message in state %s", ue.State.Current())
	}

	return nil
}

func handleDeregistrationAccept(ctx ctxt.Context, ue *context.AmfUe, msg *nasMessage.DeregistrationAcceptUETerminatedDeregistration) error {
	switch ue.State.Current() {
	case context.DeregistrationInitiated:
		if err := HandleDeregistrationAccept(ctx, ue, msg); err != nil {
			return fmt.Errorf("error handling deregistration accept: %v", err)
		}
	default:
		return fmt.Errorf("state mismatch: receive Deregistration Accept message in state %s", ue.State.Current())
	}

	return nil
}

func HandleGmmMessage(ctx ctxt.Context, ue *context.AmfUe, msg *nas.GmmMessage) error {
	msgType := msg.GetMessageType()
	ctx, span := tracer.Start(ctx, "AMF HandleGmmMessage")
	span.SetAttributes(
		attribute.String("ue", ue.Supi),
		attribute.String("state", string(ue.State.Current())),
		attribute.String("messageType", getMessageName(msgType)),
	)
	defer span.End()

	switch msgType {
	case nas.MsgTypeRegistrationRequest:
		return handleRegistrationRequest(ctx, ue, msg)
	case nas.MsgTypeServiceRequest:
		return handleServiceRequest(ctx, ue, msg)
	case nas.MsgTypeULNASTransport:
		return handleULNASTransport(ctx, ue, msg)
	case nas.MsgTypeConfigurationUpdateComplete:
		return handleConfigurationUpdateComplete(ue, msg)
	case nas.MsgTypeNotificationResponse:
		return handleNotificationResponse(ctx, ue, msg)
	case nas.MsgTypeDeregistrationRequestUEOriginatingDeregistration:
		return handleDeregistrationRequestUEOriginatingDeregistration(ctx, ue, msg)
	case nas.MsgTypeStatus5GMM:
		return handleStatus5GMM(ue, msg)
	case nas.MsgTypeIdentityResponse:
		return handleIdentityResponse(ctx, ue, msg)
	case nas.MsgTypeAuthenticationResponse:
		return handleAuthenticationResponse(ctx, ue, msg)
	case nas.MsgTypeAuthenticationFailure:
		return handleAuthenticationFailure(ctx, ue, msg)
	case nas.MsgTypeSecurityModeComplete:
		return handleSecurityModeComplete(ctx, ue, msg)
	case nas.MsgTypeSecurityModeReject:
		return handleSecurityModeReject(ctx, ue, msg)
	case nas.MsgTypeRegistrationComplete:
		return handleRegistrationComplete(ctx, ue, msg)
	case nas.MsgTypeDeregistrationAcceptUETerminatedDeregistration:
		return handleDeregistrationAccept(ctx, ue, msg.DeregistrationAcceptUETerminatedDeregistration)
	default:
		return fmt.Errorf("message type %d handling not implemented", msg.GetMessageType())
	}
}

func DeRegistered(ctx ctxt.Context, state *fsm.State, event fsm.EventType, args fsm.ArgsType) {
	_, span := tracer.Start(ctx, "AMF SM DeRegistered")
	span.SetAttributes(
		attribute.String("event", string(event)),
		attribute.String("state", string(state.Current())),
	)
	defer span.End()

	switch event {
	case fsm.EntryEvent:
		amfUe := args[ArgAmfUe].(*context.AmfUe)
		amfUe.ClearRegistrationRequestData()
		amfUe.GmmLog.Debug("EntryEvent at GMM State[DeRegistered]")
	case StartAuthEvent:
		logger.AmfLog.Debug("StartAuthEvent")
	case fsm.ExitEvent:
		logger.AmfLog.Debug("ExitEvent")
	default:
		logger.AmfLog.Error("Unknown event", zap.Any("event", event))
	}
}

func Registered(ctx ctxt.Context, state *fsm.State, event fsm.EventType, args fsm.ArgsType) {
	_, span := tracer.Start(ctx, "AMF SM Registered")
	span.SetAttributes(
		attribute.String("event", string(event)),
		attribute.String("state", string(state.Current())),
	)
	defer span.End()

	switch event {
	case fsm.EntryEvent:
		// clear stored registration request data for this registration
		amfUe := args[ArgAmfUe].(*context.AmfUe)
		amfUe.ClearRegistrationRequestData()
		amfUe.GmmLog.Debug("EntryEvent at GMM State[Registered]")
		// store context in DB. Registration procedure is complete.
	case StartAuthEvent:
		logger.AmfLog.Debug("StartAuthEvent")
	case InitDeregistrationEvent:
		logger.AmfLog.Debug("InitDeregistrationEvent")
	case fsm.ExitEvent:
		logger.AmfLog.Debug("ExitEvent")
	default:
		logger.AmfLog.Error("Unknown event", zap.Any("event", event))
	}
}

func Authentication(ctx ctxt.Context, state *fsm.State, event fsm.EventType, args fsm.ArgsType) {
	ctx, span := tracer.Start(ctx, "AMF SM Authentication")
	span.SetAttributes(
		attribute.String("event", string(event)),
		attribute.String("state", string(state.Current())),
	)
	defer span.End()

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
		amfUe.GmmLog.Debug("AuthRestartEvent at GMM State[Authentication]")

		pass, err := AuthenticationProcedure(ctx, amfUe)
		if err != nil {
			if err := GmmFSM.SendEvent(ctx, state, AuthErrorEvent, fsm.ArgsType{
				ArgAmfUe: amfUe,
			}); err != nil {
				logger.AmfLog.Error("Error sending event", zap.Error(err))
			}
		}
		if pass {
			if err := GmmFSM.SendEvent(ctx, state, AuthSuccessEvent, fsm.ArgsType{
				ArgAmfUe: amfUe,
			}); err != nil {
				logger.AmfLog.Error("Error sending event", zap.Error(err))
			}
		}
	case AuthSuccessEvent:
		logger.AmfLog.Debug("AuthSuccessEvent")
	case AuthFailEvent:
		logger.AmfLog.Debug("AuthFailEvent")
		logger.AmfLog.Warn("Reject authentication")
	case AuthErrorEvent:
		amfUe := args[ArgAmfUe].(*context.AmfUe)

		if err := HandleAuthenticationError(ctx, amfUe); err != nil {
			logger.AmfLog.Error("Error handling authentication error", zap.Error(err))
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
	ctx, span := tracer.Start(ctx, "AMF SM SecurityMode")
	span.SetAttributes(
		attribute.String("event", string(event)),
		attribute.String("state", string(state.Current())),
	)
	defer span.End()

	switch event {
	case fsm.EntryEvent:
		amfUe := args[ArgAmfUe].(*context.AmfUe)
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
				ArgNASMessage: amfUe.RegistrationRequest,
			}); err != nil {
				logger.AmfLog.Error("Error sending event", zap.Error(err))
			}
		} else {
			// Select enc/int algorithm based on ue security capability & amf's policy,
			amfSelf := context.AMFSelf()
			amfUe.SelectSecurityAlg(amfSelf.SecurityAlgorithm.IntegrityOrder, amfSelf.SecurityAlgorithm.CipheringOrder)
			// Generate KnasEnc, KnasInt
			amfUe.DerivateAlgKey()
			if amfUe.CipheringAlg == security.AlgCiphering128NEA0 && amfUe.IntegrityAlg == security.AlgIntegrity128NIA0 {
				err := GmmFSM.SendEvent(ctx, state, SecuritySkipEvent, fsm.ArgsType{
					ArgAmfUe:      amfUe,
					ArgNASMessage: amfUe.RegistrationRequest,
				})
				if err != nil {
					logger.AmfLog.Error("Error sending event", zap.Error(err))
				}
			} else {
				err := gmm_message.SendSecurityModeCommand(ctx, amfUe.RanUe)
				if err != nil {
					logger.AmfLog.Error("error sending security mode command", zap.Error(err))
				}
			}
		}
	case SecurityModeAbortEvent:
		logger.AmfLog.Debug("SecurityModeAbortEvent")
		amfUe := args[ArgAmfUe].(*context.AmfUe)
		// stopping security mode command timer
		amfUe.SecurityContextAvailable = false
		amfUe.T3560.Stop()
		amfUe.T3560 = nil
	case SecurityModeSuccessEvent:
		logger.AmfLog.Debug("SecurityModeSuccessEvent")
	case SecurityModeFailEvent:
		logger.AmfLog.Debug("SecurityModeFailEvent")
	case fsm.ExitEvent:
		logger.AmfLog.Debug("ExitEvent")
	default:
		logger.AmfLog.Error("Unknown event", zap.Any("event", event))
	}
}

func ContextSetup(ctx ctxt.Context, state *fsm.State, event fsm.EventType, args fsm.ArgsType) {
	ctx, span := tracer.Start(ctx, "AMF SM ContextSetup")
	span.SetAttributes(
		attribute.String("event", string(event)),
		attribute.String("state", string(state.Current())),
	)
	defer span.End()

	switch event {
	case fsm.EntryEvent:
		amfUe := args[ArgAmfUe].(*context.AmfUe)
		gmmMessage := args[ArgNASMessage]
		amfUe.GmmLog.Debug("EntryEvent at GMM State[ContextSetup]")
		switch message := gmmMessage.(type) {
		case *nasMessage.RegistrationRequest:
			amfUe.RegistrationRequest = message
			switch amfUe.RegistrationType5GS {
			case nasMessage.RegistrationType5GSInitialRegistration:
				if err := HandleInitialRegistration(ctx, amfUe); err != nil {
					logger.AmfLog.Error("Error handling initial registration", zap.Error(err))
				}
			case nasMessage.RegistrationType5GSMobilityRegistrationUpdating:
				fallthrough
			case nasMessage.RegistrationType5GSPeriodicRegistrationUpdating:
				if err := HandleMobilityAndPeriodicRegistrationUpdating(ctx, amfUe); err != nil {
					logger.AmfLog.Error("Error handling mobility and periodic registration updating", zap.Error(err))
				}
			}
		case *nasMessage.ServiceRequest:
			if err := HandleServiceRequest(ctx, amfUe, message); err != nil {
				logger.AmfLog.Error("Error handling service request", zap.Error(err))
			}
		default:
			logger.AmfLog.Error("UE state mismatch: receieve wrong gmm message")
		}
	case ContextSetupSuccessEvent:
		logger.AmfLog.Debug("ContextSetupSuccessEvent")
	case ContextSetupFailEvent:
		logger.AmfLog.Debug("ContextSetupFailEvent")
	case fsm.ExitEvent:
		logger.AmfLog.Debug("ExitEvent")
	default:
		logger.AmfLog.Error("Unknown event", zap.Any("event", event))
	}
}

func DeregisteredInitiated(ctx ctxt.Context, state *fsm.State, event fsm.EventType, args fsm.ArgsType) {
	ctx, span := tracer.Start(ctx, "AMF SM DeregisteredInitiated")
	span.SetAttributes(
		attribute.String("event", string(event)),
		attribute.String("state", string(state.Current())),
	)
	defer span.End()

	switch event {
	case fsm.EntryEvent:
		amfUe := args[ArgAmfUe].(*context.AmfUe)
		if args[ArgNASMessage] != nil {
			gmmMessage := args[ArgNASMessage].(*nas.GmmMessage)
			if gmmMessage != nil {
				if err := HandleDeregistrationRequest(ctx, amfUe, gmmMessage.DeregistrationRequestUEOriginatingDeregistration); err != nil {
					logger.AmfLog.Error("Error handling deregistration request", zap.Error(err))
				}
			}
		}
	case DeregistrationAcceptEvent:
		amfUe := args[ArgAmfUe].(*context.AmfUe)
		SetDeregisteredState(amfUe)
		logger.AmfLog.Debug("DeregistrationAcceptEvent")
	case fsm.ExitEvent:
		logger.AmfLog.Debug("ExitEvent")
	default:
		logger.AmfLog.Error("Unknown event", zap.Any("event", event))
	}
}

func SetDeregisteredState(amfUe *context.AmfUe) {
	amfUe.SubscriptionDataValid = false
	amfUe.State.Set(context.Deregistered)
	amfUe.GmmLog.Debug("UE accessType[3GPP] transfer to Deregistered state")
}
