// Copyright 2024 Ella Networks
// SPDX-FileCopyrightText: 2022-present Intel Corporation
// SPDX-FileCopyrightText: 2021 Open Networking Foundation <info@opennetworking.org>
// Copyright 2019 free5GC.org
//
// SPDX-License-Identifier: Apache-2.0

package gmm

import (
	ctxt "context"

	"github.com/ellanetworks/core/internal/amf/context"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/util/fsm"
	"github.com/free5gc/nas"
	"go.uber.org/zap"
)

func DeRegistered(ctx ctxt.Context, state *fsm.State, event fsm.EventType, args fsm.ArgsType) {
	switch event {
	case fsm.EntryEvent:
		amfUe := args[ArgAmfUe].(*context.AmfUe)
		amfUe.ClearRegistrationRequestData()
		amfUe.GmmLog.Debug("EntryEvent at GMM State[DeRegistered]")
	case NwInitiatedDeregistrationEvent:
		amfUe := args[ArgAmfUe].(*context.AmfUe)
		if err := NetworkInitiatedDeregistrationProcedure(ctx, amfUe); err != nil {
			logger.AmfLog.Error("Error handling network initiated deregistration", zap.Error(err))
		}
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
		amfUe.ClearRegistrationRequestData()
		amfUe.GmmLog.Debug("EntryEvent at GMM State[Registered]")
	case InitDeregistrationEvent:
		logger.AmfLog.Debug("InitDeregistrationEvent")
	case NwInitiatedDeregistrationEvent:
		amfUe := args[ArgAmfUe].(*context.AmfUe)
		if err := NetworkInitiatedDeregistrationProcedure(ctx, amfUe); err != nil {
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
	// case AuthRestartEvent:
	// 	amfUe = args[ArgAmfUe].(*context.AmfUe)
	// amfUe.GmmLog.Debug("AuthRestartEvent at GMM State[Authentication]")

	// pass, err := AuthenticationProcedure(ctx, amfUe)
	// if err != nil {
	// 	if err := GmmFSM.SendEvent(ctx, state, AuthErrorEvent, fsm.ArgsType{
	// 		ArgAmfUe: amfUe,
	// 	}); err != nil {
	// 		logger.AmfLog.Error("Error sending event", zap.Error(err))
	// 	}
	// }
	// if pass {
	// 	if err := GmmFSM.SendEvent(ctx, state, AuthSuccessEvent, fsm.ArgsType{
	// 		ArgAmfUe: amfUe,
	// 	}); err != nil {
	// 		logger.AmfLog.Error("Error sending event", zap.Error(err))
	// 	}
	// }
	// case AuthSuccessEvent:
	// 	logger.AmfLog.Debug("AuthSuccessEvent")
	case AuthFailEvent:
		logger.AmfLog.Debug("AuthFailEvent")
		logger.AmfLog.Warn("Reject authentication")
	case AuthErrorEvent:
		amfUe := args[ArgAmfUe].(*context.AmfUe)

		if err := HandleAuthenticationError(ctx, amfUe); err != nil {
			logger.AmfLog.Error("Error handling authentication error", zap.Error(err))
		}
	case NwInitiatedDeregistrationEvent:
		amfUe := args[ArgAmfUe].(*context.AmfUe)
		if err := NetworkInitiatedDeregistrationProcedure(ctx, amfUe); err != nil {
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
	// case fsm.EntryEvent:
	// 	amfUe := args[ArgAmfUe].(*context.AmfUe)
	// 	// set log information
	// 	amfUe.NASLog = amfUe.NASLog.With(zap.String("supi", amfUe.Supi))
	// 	amfUe.TxLog = amfUe.NASLog.With(zap.String("supi", amfUe.Supi))
	// 	amfUe.GmmLog = amfUe.GmmLog.With(zap.String("supi", amfUe.Supi))
	// 	amfUe.ProducerLog = amfUe.GmmLog.With(zap.String("supi", amfUe.Supi))
	// 	amfUe.GmmLog.Debug("EntryEvent at GMM State[SecurityMode]")
	// 	if amfUe.SecurityContextIsValid() {
	// 		amfUe.GmmLog.Debug("UE has a valid security context - skip security mode control procedure")
	// 		if err := GmmFSM.SendEvent(ctx, state, SecurityModeSuccessEvent, fsm.ArgsType{
	// 			ArgAmfUe:      amfUe,
	// 			ArgNASMessage: amfUe.RegistrationRequest,
	// 		}); err != nil {
	// 			logger.AmfLog.Error("Error sending event", zap.Error(err))
	// 		}
	// 	} else {
	// 		// Select enc/int algorithm based on ue security capability & amf's policy,
	// 		amfSelf := context.AMFSelf()
	// 		amfUe.SelectSecurityAlg(amfSelf.SecurityAlgorithm.IntegrityOrder, amfSelf.SecurityAlgorithm.CipheringOrder)
	// 		// Generate KnasEnc, KnasInt
	// 		amfUe.DerivateAlgKey()
	// 		if amfUe.CipheringAlg == security.AlgCiphering128NEA0 && amfUe.IntegrityAlg == security.AlgIntegrity128NIA0 {
	// 			err := GmmFSM.SendEvent(ctx, state, SecuritySkipEvent, fsm.ArgsType{
	// 				ArgAmfUe:      amfUe,
	// 				ArgNASMessage: amfUe.RegistrationRequest,
	// 			})
	// 			if err != nil {
	// 				logger.AmfLog.Error("Error sending event", zap.Error(err))
	// 			}
	// 		} else {
	// 			err := gmm_message.SendSecurityModeCommand(ctx, amfUe.RanUe)
	// 			if err != nil {
	// 				logger.AmfLog.Error("error sending security mode command", zap.Error(err))
	// 			}
	// 		}
	// 	}
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
		amfUe.T3560.Stop()
		amfUe.T3560 = nil
		if err := NetworkInitiatedDeregistrationProcedure(ctx, amfUe); err != nil {
			logger.AmfLog.Error("Error handling network initiated deregistration", zap.Error(err))
		}
	// case SecurityModeSuccessEvent:
	// 	logger.AmfLog.Debug("SecurityModeSuccessEvent")
	case SecurityModeFailEvent:
		logger.AmfLog.Debug("SecurityModeFailEvent")
	case fsm.ExitEvent:
		logger.AmfLog.Debug("ExitEvent")
		return
	default:
		logger.AmfLog.Error("Unknown event", zap.Any("event", event))
	}
}

func ContextSetup(ctx ctxt.Context, state *fsm.State, event fsm.EventType, args fsm.ArgsType) {
	switch event {
	case fsm.EntryEvent:
		// amfUe := args[ArgAmfUe].(*context.AmfUe)
		// gmmMessage := args[ArgNASMessage]
		// amfUe.GmmLog.Debug("EntryEvent at GMM State[ContextSetup]")
		// switch message := gmmMessage.(type) {
		// case *nasMessage.RegistrationRequest:
		// 	amfUe.RegistrationRequest = message
		// 	switch amfUe.RegistrationType5GS {
		// 	case nasMessage.RegistrationType5GSInitialRegistration:
		// 		gmmMessage := &nas.GmmMessage{RegistrationRequest: message}
		// 		gmmMessage.GmmHeader.SetMessageType(nas.MsgTypeRegistrationRequest)
		// 		if err := HandleInitialRegistration(ctx, amfUe); err != nil {
		// 			logger.AmfLog.Error("Error handling initial registration", zap.Error(err))
		// 		}
		// 	case nasMessage.RegistrationType5GSMobilityRegistrationUpdating:
		// 		fallthrough
		// 	case nasMessage.RegistrationType5GSPeriodicRegistrationUpdating:
		// 		nasMessage := &nas.GmmMessage{RegistrationRequest: message}
		// 		nasMessage.GmmHeader.SetMessageType(nas.MsgTypeRegistrationRequest)
		// 		if err := HandleMobilityAndPeriodicRegistrationUpdating(ctx, amfUe); err != nil {
		// 			logger.AmfLog.Error("Error handling mobility and periodic registration updating", zap.Error(err))
		// 		}
		// 	}
		// case *nasMessage.ServiceRequest:
		// 	nasMessage := &nas.GmmMessage{ServiceRequest: message}
		// 	nasMessage.GmmHeader.SetMessageType(nas.MsgTypeServiceRequest)
		// 	if err := HandleServiceRequest(ctx, amfUe, message); err != nil {
		// 		logger.AmfLog.Error("Error handling service request", zap.Error(err))
		// 	}
		// default:
		// 	logger.AmfLog.Error("UE state mismatch: receieve wrong gmm message")
		// }
	case ContextSetupSuccessEvent:
		logger.AmfLog.Debug("ContextSetupSuccessEvent")
	case NwInitiatedDeregistrationEvent:
		logger.AmfLog.Debug("NwInitiatedDeregistrationEvent")
		amfUe := args[ArgAmfUe].(*context.AmfUe)
		amfUe.T3550.Stop()
		amfUe.T3550 = nil
		amfUe.State.Set(context.Registered)
		if err := NetworkInitiatedDeregistrationProcedure(ctx, amfUe); err != nil {
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
