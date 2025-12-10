// Copyright 2024 Ella Networks
// SPDX-FileCopyrightText: 2021 Open Networking Foundation <info@opennetworking.org>
// Copyright 2019 free5GC.org
//
// SPDX-License-Identifier: Apache-2.0

package gmm

import (
	"github.com/ellanetworks/core/internal/amf/context"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/util/fsm"
	"go.uber.org/zap"
)

const (
	GmmMessageEvent                fsm.EventType = "Gmm Message"
	StartAuthEvent                 fsm.EventType = "Start Authentication"
	AuthSuccessEvent               fsm.EventType = "Authentication Success"
	AuthRestartEvent               fsm.EventType = "Authentication Restart"
	AuthFailEvent                  fsm.EventType = "Authentication Fail"
	AuthErrorEvent                 fsm.EventType = "Authentication Error"
	SecurityModeSuccessEvent       fsm.EventType = "SecurityMode Success"
	SecurityModeFailEvent          fsm.EventType = "SecurityMode Fail"
	SecuritySkipEvent              fsm.EventType = "Security Skip"
	SecurityModeAbortEvent         fsm.EventType = "SecurityMode Abort"
	ContextSetupSuccessEvent       fsm.EventType = "ContextSetup Success"
	ContextSetupFailEvent          fsm.EventType = "ContextSetup Fail"
	InitDeregistrationEvent        fsm.EventType = "Initialize Deregistration"
	NwInitiatedDeregistrationEvent fsm.EventType = "Network Initiated Deregistration Event"
	SliceInfoDeleteEvent           fsm.EventType = "Slice Info Delete Event"
	SliceInfoAddEvent              fsm.EventType = "Slice Info Add Event"
	DeregistrationAcceptEvent      fsm.EventType = "Deregistration Accept"
)

const (
	ArgAmfUe               string = "AMF Ue"
	ArgNASMessage          string = "NAS Message"
	ArgEAPSuccess          string = "EAP Success"
	ArgEAPMessage          string = "EAP Message"
	Arg3GPPDeregistered    string = "3GPP Deregistered"
	ArgNon3GPPDeregistered string = "Non3GPP Deregistered"
	ArgNssai               string = "Nssai"
)

var transitions = fsm.Transitions{
	{Event: GmmMessageEvent, From: context.Deregistered, To: context.Deregistered},
	{Event: GmmMessageEvent, From: context.Authentication, To: context.Authentication},
	{Event: GmmMessageEvent, From: context.SecurityMode, To: context.SecurityMode},
	{Event: GmmMessageEvent, From: context.ContextSetup, To: context.ContextSetup},
	{Event: GmmMessageEvent, From: context.Registered, To: context.Registered},
	{Event: GmmMessageEvent, From: context.DeregistrationInitiated, To: context.DeregistrationInitiated},
	{Event: StartAuthEvent, From: context.Deregistered, To: context.Authentication},
	{Event: StartAuthEvent, From: context.Registered, To: context.Authentication},
	{Event: AuthRestartEvent, From: context.Authentication, To: context.Authentication},
	{Event: AuthSuccessEvent, From: context.Authentication, To: context.SecurityMode},
	{Event: AuthFailEvent, From: context.Authentication, To: context.Deregistered},
	{Event: AuthErrorEvent, From: context.Authentication, To: context.Deregistered},
	{Event: SecurityModeSuccessEvent, From: context.SecurityMode, To: context.ContextSetup},
	{Event: SecuritySkipEvent, From: context.SecurityMode, To: context.ContextSetup},
	{Event: SecurityModeFailEvent, From: context.SecurityMode, To: context.Deregistered},
	{Event: SecurityModeAbortEvent, From: context.SecurityMode, To: context.Deregistered},
	{Event: ContextSetupSuccessEvent, From: context.ContextSetup, To: context.Registered},
	{Event: ContextSetupFailEvent, From: context.ContextSetup, To: context.Deregistered},
	{Event: InitDeregistrationEvent, From: context.Registered, To: context.DeregistrationInitiated},
	{Event: NwInitiatedDeregistrationEvent, From: context.Registered, To: context.DeregistrationInitiated},
	{Event: DeregistrationAcceptEvent, From: context.DeregistrationInitiated, To: context.Deregistered},
}

var callbacks = fsm.Callbacks{
	context.Deregistered:            DeRegistered,
	context.Authentication:          Authentication,
	context.SecurityMode:            SecurityMode,
	context.ContextSetup:            ContextSetup,
	context.Registered:              Registered,
	context.DeregistrationInitiated: DeregisteredInitiated,
}

var GmmFSM *fsm.FSM

func init() {
	if f, err := fsm.NewFSM(transitions, callbacks); err != nil {
		logger.AmfLog.Error("Initialize Gmm FSM Error", zap.Error(err))
	} else {
		GmmFSM = f
	}
}
