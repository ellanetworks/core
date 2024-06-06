package gmm

import (
	"github.com/omec-project/util/fsm"
	"github.com/yeastengine/ella/internal/amf/context"
	"github.com/yeastengine/ella/internal/amf/logger"
)

var (
	MockRegisteredCallCount            uint32 = 0
	MockDeregisteredInitiatedCallCount uint32 = 0
	MockContextSetupCallCount          uint32 = 0
	MockDeRegisteredCallCount          uint32 = 0
	MockSecurityModeCallCount          uint32 = 0
	MockAuthenticationCallCount        uint32 = 0
)

var mockCallbacks = fsm.Callbacks{
	context.Deregistered:            MockDeRegistered,
	context.Authentication:          MockAuthentication,
	context.SecurityMode:            MockSecurityMode,
	context.ContextSetup:            MockContextSetup,
	context.Registered:              MockRegistered,
	context.DeregistrationInitiated: MockDeregisteredInitiated,
}

func Mockinit() {
	if f, err := fsm.NewFSM(transitions, mockCallbacks); err != nil {
		logger.GmmLog.Errorf("Initialize Gmm FSM Error: %+v", err)
	} else {
		GmmFSM = f
	}
}

func MockDeRegistered(state *fsm.State, event fsm.EventType, args fsm.ArgsType) {
	logger.GmmLog.Info("MockDeRegistered")
	MockDeRegisteredCallCount++
}

func MockAuthentication(state *fsm.State, event fsm.EventType, args fsm.ArgsType) {
	logger.GmmLog.Info("MockAuthentication")
	MockAuthenticationCallCount++
}

func MockSecurityMode(state *fsm.State, event fsm.EventType, args fsm.ArgsType) {
	logger.GmmLog.Info("MockSecurityMode")
	MockSecurityModeCallCount++
}

func MockContextSetup(state *fsm.State, event fsm.EventType, args fsm.ArgsType) {
	logger.GmmLog.Info("MockContextSetup")
	MockContextSetupCallCount++
}

func MockRegistered(state *fsm.State, event fsm.EventType, args fsm.ArgsType) {
	logger.GmmLog.Info(event)
	logger.GmmLog.Info("MockRegistered")
	MockRegisteredCallCount++
}

func MockDeregisteredInitiated(state *fsm.State, event fsm.EventType, args fsm.ArgsType) {
	logger.GmmLog.Info("MockDeregisteredInitiated")
	MockDeregisteredInitiatedCallCount++

	amfUe := args[ArgAmfUe].(*context.AmfUe)
	amfUe.Remove()
}
