// Copyright 2024 Ella Networks
// SPDX-FileCopyrightText: 2022-present Intel Corporation
// SPDX-FileCopyrightText: 2021 Open Networking Foundation <info@opennetworking.org>
// SPDX-License-Identifier: Apache-2.0

package fsm

import (
	"fmt"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/smf/context"
	"github.com/ellanetworks/core/internal/smf/msgtypes/svcmsgtypes"
	"github.com/ellanetworks/core/internal/smf/producer"
	"github.com/ellanetworks/core/internal/smf/transaction"
)

func (SmfTxnFsm) TxnInit(txn *transaction.Transaction) (transaction.TxnEvent, error) {
	txn.TxnFsmLog.Debugf("handle event[%v] ", transaction.TxnEventInit.String())
	return transaction.TxnEventDecode, nil
}

func (SmfTxnFsm) TxnDecode(txn *transaction.Transaction) (transaction.TxnEvent, error) {
	return transaction.TxnEventLoadCtxt, nil
}

func (SmfTxnFsm) TxnLoadCtxt(txn *transaction.Transaction) (transaction.TxnEvent, error) {
	switch txn.MsgType {
	case svcmsgtypes.PfcpSessCreate:
		fallthrough
	case svcmsgtypes.PfcpSessCreateFailure:
		// Pre-loaded- No action
	default:
		txn.TxnFsmLog.Errorf("handle event[%v], next-event[%v], unknown msgtype [%v] ",
			transaction.TxnEventLoadCtxt.String(), transaction.TxnEventFailure.String(), txn.MsgType)
		return transaction.TxnEventFailure, fmt.Errorf("invalid Msg to load Txn")
	}

	if txn.Ctxt.(*context.SMContext) == nil {
		txn.TxnFsmLog.Errorf("handle event[%v], ctxt [%v] not found", transaction.TxnEventLoadCtxt.String(), txn.CtxtKey)
		return transaction.TxnEventFailure, fmt.Errorf("ctxt not found")
	}

	return transaction.TxnEventCtxtPost, nil
}

func (SmfTxnFsm) TxnCtxtPost(txn *transaction.Transaction) (transaction.TxnEvent, error) {
	smContext := txn.Ctxt.(*context.SMContext)

	// Lock the bus before modifying
	smContext.SMTxnBusLock.Lock()
	defer smContext.SMTxnBusLock.Unlock()

	// If already Active Txn running then post it to SMF Txn Bus
	if smContext.ActiveTxn != nil {
		smContext.TxnBus = smContext.TxnBus.AddTxn(txn)

		// Txn has been posted and shall be scheduled later
		txn.TxnFsmLog.Debugf("event[%v], next-event[%v], txn queued ", transaction.TxnEventCtxtPost.String(), transaction.TxnEventExit.String())
		return transaction.TxnEventQueue, nil
	}

	// No other Txn running, lets proceed with current Txn

	return transaction.TxnEventRun, nil
}

func (SmfTxnFsm) TxnCtxtRun(txn *transaction.Transaction) (transaction.TxnEvent, error) {
	smContext := txn.Ctxt.(*context.SMContext)

	// There shouldn't be any active Txn if current Txn has reached to Run state
	// Probably, abort it
	smContext.SMTxnBusLock.Lock()
	defer smContext.SMTxnBusLock.Unlock()

	if smContext.ActiveTxn != nil {
		logger.SmfLog.Errorf("active transaction [%v] not completed", smContext.ActiveTxn)
	}

	// make current txn as Active now, move it to processing
	smContext.ActiveTxn = txn
	return transaction.TxnEventProcess, nil
}

func (SmfTxnFsm) TxnProcess(txn *transaction.Transaction) (transaction.TxnEvent, error) {
	smContext := txn.Ctxt.(*context.SMContext)
	if smContext == nil {
		txn.TxnFsmLog.Errorf("event[%v], next-event[%v], SM context invalid ", transaction.TxnEventProcess.String(), transaction.TxnEventFailure.String())
		return transaction.TxnEventFailure, fmt.Errorf("TxnProcess, invalid SM Ctxt")
	}

	var event SmEvent

	switch txn.MsgType {
	case svcmsgtypes.PfcpSessCreate:
		event = SmEventPfcpSessCreate
	case svcmsgtypes.PfcpSessCreateFailure:
		event = SmEventPfcpSessCreateFailure
	default:
		event = SmEventInvalid
	}

	eventData := SmEventData{Txn: txn}

	if err := HandleEvent(smContext, event, eventData); err != nil {
		return transaction.TxnEventFailure, fmt.Errorf("couldn't handle event [%v]: %s", transaction.TxnEventProcess.String(), err.Error())
	}
	return transaction.TxnEventSuccess, nil
}

func HandleStateN1N2TransferPendingEventN1N2Transfer(smCtxt *context.SMContext) (context.SMContextState, error) {
	if err := producer.SendPduSessN1N2Transfer(smCtxt, true); err != nil {
		smCtxt.SubFsmLog.Errorf("N1N2 transfer failure error, %v ", err.Error())
		return context.SmStateN1N2TransferPending, fmt.Errorf("N1N2 Transfer failure error, %v ", err.Error())
	}
	return context.SmStateActive, nil
}

func (SmfTxnFsm) TxnSuccess(txn *transaction.Transaction) (transaction.TxnEvent, error) {
	switch txn.MsgType {
	case svcmsgtypes.PfcpSessCreate:

		go func() {
			smContext := txn.Ctxt.(*context.SMContext)
			nextState, err := HandleStateN1N2TransferPendingEventN1N2Transfer(smContext)
			smContext.ChangeState(nextState)
			if err != nil {
				logger.SmfLog.Errorf("error processing state machine transaction")
			}
		}()
	}

	// put Success Rsp
	txn.Status <- true
	return transaction.TxnEventSave, nil
}

func (SmfTxnFsm) TxnFailure(txn *transaction.Transaction) (transaction.TxnEvent, error) {
	// Put Failure Rsp
	switch txn.MsgType {
	case svcmsgtypes.PfcpSessCreate:
		if txn.Ctxt != nil && txn.Ctxt.(*context.SMContext).SMContextState == context.SmStatePfcpCreatePending {
			nextTxn := transaction.NewTransaction(nil, nil, svcmsgtypes.PfcpSessCreateFailure)
			nextTxn.Ctxt = txn.Ctxt
			smContext := txn.Ctxt.(*context.SMContext)
			smContext.SMTxnBusLock.Lock()
			smContext.TxnBus = smContext.TxnBus.AddTxn(nextTxn)
			smContext.SMTxnBusLock.Unlock()
			go func(nextTxn *transaction.Transaction) {
				// Initiate N1N2 Transfer

				// nextTxn.StartTxnLifeCycle(SmfTxnFsmHandle)
				<-nextTxn.Status
			}(nextTxn)
		}
	}
	txn.Status <- false
	return transaction.TxnEventEnd, nil
}

func (SmfTxnFsm) TxnAbort(txn *transaction.Transaction) (transaction.TxnEvent, error) {
	return transaction.TxnEventEnd, nil
}

func (SmfTxnFsm) TxnSave(txn *transaction.Transaction) (transaction.TxnEvent, error) {
	return transaction.TxnEventEnd, nil
}

func (SmfTxnFsm) TxnTimeout(txn *transaction.Transaction) (transaction.TxnEvent, error) {
	return transaction.TxnEventEnd, nil
}

func (SmfTxnFsm) TxnCollision(txn *transaction.Transaction) (transaction.TxnEvent, error) {
	return transaction.TxnEventEnd, nil
}

func (SmfTxnFsm) TxnEnd(txn *transaction.Transaction) (transaction.TxnEvent, error) {
	txn.TransactionEnd()

	smContext := txn.Ctxt.(*context.SMContext)
	if smContext == nil {
		return transaction.TxnEventExit, nil
	}

	// Lock txnbus to access
	smContext.SMTxnBusLock.Lock()
	defer smContext.SMTxnBusLock.Unlock()

	// Reset Active Txn
	smContext.ActiveTxn = nil

	var nextTxn *transaction.Transaction
	// Active Txn is over, now Pull out head Txn and Run it
	if len(smContext.TxnBus) > 0 {
		nextTxn, smContext.TxnBus = smContext.TxnBus.PopTxn()
		txn.NextTxn = nextTxn
		return transaction.TxnEventRun, nil
	}

	return transaction.TxnEventExit, nil
}
