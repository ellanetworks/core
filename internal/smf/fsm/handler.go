// Copyright 2024 Ella Networks
// SPDX-FileCopyrightText: 2021 Open Networking Foundation <info@opennetworking.org>
// SPDX-License-Identifier: Apache-2.0

package fsm

import (
	"fmt"

	"github.com/ellanetworks/core/internal/smf/context"
	"github.com/ellanetworks/core/internal/smf/producer"
	"github.com/ellanetworks/core/internal/smf/transaction"
	"github.com/omec-project/openapi/models"
)

// Define SM Context level Events
type SmEvent uint

const (
	SmEventInvalid SmEvent = iota
	SmEventPfcpSessCreate
	SmEventPfcpSessCreateFailure
	SmEventMax
)

type SmEventData struct {
	Txn interface{}
}

// Define FSM Func Point Struct here
type eventHandler func(event SmEvent, eventData *SmEventData) (context.SMContextState, error)

var SmfFsmHandler [context.SmStateMax][SmEventMax]eventHandler

func init() {
	// Initilise with default invalid handler
	for state := context.SmStateInit; state < context.SmStateMax; state++ {
		for event := SmEventInvalid; event < SmEventMax; event++ {
			SmfFsmHandler[state][event] = EmptyEventHandler
		}
	}

	InitFsm()
	transaction.InitTxnFsm(SmfTxnFsmHandle)
}

// Override with specific handler
func InitFsm() {
	SmfFsmHandler[context.SmStatePfcpCreatePending][SmEventPfcpSessCreate] = HandleStatePfcpCreatePendingEventPfcpSessCreate
	SmfFsmHandler[context.SmStatePfcpCreatePending][SmEventPfcpSessCreateFailure] = HandleStatePfcpCreatePendingEventPfcpSessCreateFailure
}

func HandleEvent(smContext *context.SMContext, event SmEvent, eventData SmEventData) error {
	ctxtState := smContext.SMContextState
	smContext.SubFsmLog.Debugf("handle fsm event[%v], state[%v] ", event.String(), ctxtState.String())
	if nextState, err := SmfFsmHandler[smContext.SMContextState][event](event, &eventData); err != nil {
		return fmt.Errorf("fsm state[%v] event[%v], next-state[%v]: %v", smContext.SMContextState.String(), event.String(), nextState.String(), err.Error())
	} else {
		smContext.ChangeState(nextState)
	}

	return nil
}

type SmfTxnFsm struct{}

var SmfTxnFsmHandle SmfTxnFsm

func EmptyEventHandler(event SmEvent, eventData *SmEventData) (context.SMContextState, error) {
	txn := eventData.Txn.(*transaction.Transaction)
	smCtxt := txn.Ctxt.(*context.SMContext)
	smCtxt.SubFsmLog.Errorf("unhandled event[%s] ", event.String())
	return smCtxt.SMContextState, fmt.Errorf("fsm error, unhandled event[%s] and event data[%s] ", event.String(), eventData.String())
}

func HandleStatePfcpCreatePendingEventPfcpSessCreate(event SmEvent, eventData *SmEventData) (context.SMContextState, error) {
	txn := eventData.Txn.(*transaction.Transaction)
	smCtxt := txn.Ctxt.(*context.SMContext)

	producer.SendPFCPRules(smCtxt)
	smCtxt.SubFsmLog.Debug("waiting for pfcp session establish response")
	switch <-smCtxt.SBIPFCPCommunicationChan {
	case context.SessionEstablishSuccess:
		smCtxt.SubFsmLog.Debug("pfcp session establish response success")
		return context.SmStateN1N2TransferPending, nil
	case context.SessionEstablishFailed:
		fallthrough
	default:
		smCtxt.SubFsmLog.Errorf("pfcp session establish response failure")
		return context.SmStatePfcpCreatePending, fmt.Errorf("pfcp establishment failure")
	}
}

func HandleStatePfcpCreatePendingEventPfcpSessCreateFailure(event SmEvent, eventData *SmEventData) (context.SMContextState, error) {
	txn := eventData.Txn.(*transaction.Transaction)
	smCtxt := txn.Ctxt.(*context.SMContext)

	// sending n1n2 transfer failure to amf
	if err := producer.SendPduSessN1N2Transfer(smCtxt, false); err != nil {
		smCtxt.SubFsmLog.Errorf("N1N2 transfer failure error, %v ", err.Error())
		return context.SmStateN1N2TransferPending, fmt.Errorf("N1N2 Transfer failure error, %v ", err.Error())
	}
	return context.SmStateInit, nil
}

func HandleStateActiveEventPduSessRelease(event SmEvent, eventData *SmEventData) (context.SMContextState, error) {
	txn := eventData.Txn.(*transaction.Transaction)
	smCtxt := txn.Ctxt.(*context.SMContext)
	request := txn.Req.(models.ReleaseSmContextRequest)

	rsp, err := producer.HandlePDUSessionSMContextRelease(request, smCtxt)
	txn.Rsp = rsp
	if err != nil {
		txn.Err = err
		return context.SmStateInit, fmt.Errorf("error releasing pdu session: %v ", err.Error())
	}

	return context.SmStateInit, nil
}

// func HandleStateActiveEventPduSessN1N2TransFailInd(event SmEvent, eventData *SmEventData) (context.SMContextState, error) {
// 	txn := eventData.Txn.(*transaction.Transaction)
// 	smCtxt := txn.Ctxt.(*context.SMContext)
// 	rsp, err := producer.HandlePduSessN1N2TransFailInd(smCtxt)
// 	txn.Rsp = rsp
// 	if err != nil {
// 		txn.Err = err
// 		return context.SmStateInit, fmt.Errorf("error handling pdu session n1n2 transfer failure: %v ", err.Error())
// 	}
// 	return context.SmStateInit, nil
// }
