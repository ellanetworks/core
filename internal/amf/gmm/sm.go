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
	switch event {
	case SecurityModeAbortEvent:
		logger.AmfLog.Debug("SecurityModeAbortEvent")
		amfUe := args[ArgAmfUe].(*context.AmfUe)
		// stopping security mode command timer
		amfUe.SecurityContextAvailable = false
		amfUe.T3560.Stop()
		amfUe.T3560 = nil

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

	// case ContextSetupSuccessEvent:
	// 	logger.AmfLog.Debug("ContextSetupSuccessEvent")

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
