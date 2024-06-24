package fsm

import (
	"fmt"
	"net/http"

	"github.com/omec-project/openapi/models"
	"github.com/omec-project/util/httpwrapper"
	"github.com/yeastengine/ella/internal/smf/context"
	smf_context "github.com/yeastengine/ella/internal/smf/context"
	"github.com/yeastengine/ella/internal/smf/logger"
	"github.com/yeastengine/ella/internal/smf/msgtypes/svcmsgtypes"
	"github.com/yeastengine/ella/internal/smf/producer"
	"github.com/yeastengine/ella/internal/smf/transaction"
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
	case svcmsgtypes.CreateSmContext:
		req := txn.Req.(models.PostSmContextsRequest)
		createData := req.JsonData
		if smCtxtRef, err := smf_context.ResolveRef(createData.Supi, createData.PduSessionId); err == nil {
			// Previous context exist
			producer.HandlePduSessionContextReplacement(smCtxtRef)
		}
		// Create fresh context
		txn.Ctxt = smf_context.NewSMContext(createData.Supi, createData.PduSessionId)
		txn.CtxtKey, _ = smf_context.ResolveRef(createData.Supi, createData.PduSessionId)
	case svcmsgtypes.UpdateSmContext:
		fallthrough
	case svcmsgtypes.ReleaseSmContext:
		fallthrough
	case svcmsgtypes.SmPolicyUpdateNotification:
		txn.Ctxt = smf_context.GetSMContext(txn.CtxtKey)

	case svcmsgtypes.PfcpSessCreate:
		fallthrough
		// txn.Ctxt = smf_context.GetSMContext(txn.CtxtKey)
	case svcmsgtypes.N1N2MessageTransfer:
		// Pre-loaded- No action
	case svcmsgtypes.PfcpSessCreateFailure:
		// Pre-loaded- No action
	case svcmsgtypes.N1N2MessageTransferFailureNotification:
		txn.Ctxt = smf_context.GetSMContext(txn.CtxtKey)
	default:
		txn.TxnFsmLog.Errorf("handle event[%v], next-event[%v], unknown msgtype [%v] ",
			transaction.TxnEventLoadCtxt.String(), transaction.TxnEventFailure.String(), txn.MsgType)
		return transaction.TxnEventFailure, fmt.Errorf("invalid Msg to load Txn")
	}

	if txn.Ctxt.(*smf_context.SMContext) == nil {
		txn.TxnFsmLog.Errorf("handle event[%v], ctxt [%v] not found", transaction.TxnEventLoadCtxt.String(), txn.CtxtKey)
		return transaction.TxnEventFailure, fmt.Errorf("ctxt not found")
	}

	return transaction.TxnEventCtxtPost, nil
}

func (SmfTxnFsm) TxnCtxtPost(txn *transaction.Transaction) (transaction.TxnEvent, error) {
	smContext := txn.Ctxt.(*smf_context.SMContext)

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
	smContext := txn.Ctxt.(*smf_context.SMContext)

	// There shouldn't be any active Txn if current Txn has reached to Run state
	// Probably, abort it
	smContext.SMTxnBusLock.Lock()
	defer smContext.SMTxnBusLock.Unlock()

	if smContext.ActiveTxn != nil {
		logger.TxnFsmLog.Errorf("active transaction [%v] not completed", smContext.ActiveTxn)
	}

	// make current txn as Active now, move it to processing
	smContext.ActiveTxn = txn
	return transaction.TxnEventProcess, nil
}

func (SmfTxnFsm) TxnProcess(txn *transaction.Transaction) (transaction.TxnEvent, error) {
	smContext := txn.Ctxt.(*smf_context.SMContext)
	if smContext == nil {
		txn.TxnFsmLog.Errorf("event[%v], next-event[%v], SM context invalid ", transaction.TxnEventProcess.String(), transaction.TxnEventFailure.String())
		return transaction.TxnEventFailure, fmt.Errorf("TxnProcess, invalid SM Ctxt")
	}

	var event SmEvent

	switch txn.MsgType {
	case svcmsgtypes.CreateSmContext:
		event = SmEventPduSessCreate
	case svcmsgtypes.UpdateSmContext:
		event = SmEventPduSessModify
		// req := txn.Req.(models.UpdateSmContextRequest)
	case svcmsgtypes.ReleaseSmContext:
		event = SmEventPduSessRelease
	case svcmsgtypes.PfcpSessCreate:
		event = SmEventPfcpSessCreate
	case svcmsgtypes.PfcpSessCreateFailure:
		event = SmEventPfcpSessCreateFailure
	case svcmsgtypes.N1N2MessageTransfer:
		event = SmEventPduSessN1N2Transfer
	case svcmsgtypes.N1N2MessageTransferFailureNotification:
		event = SmEventPduSessN1N2TransferFailureIndication
	case svcmsgtypes.SmPolicyUpdateNotification:
		event = SmEventPolicyUpdateNotify
	default:
		event = SmEventInvalid
	}

	eventData := SmEventData{Txn: txn}

	if err := HandleEvent(smContext, event, eventData); err != nil {
		smContext.SubFsmLog.Errorf("handle event[%v], err [%s]", transaction.TxnEventProcess.String(), err.Error())
		return transaction.TxnEventFailure, err
	}
	return transaction.TxnEventSuccess, nil
}

func (SmfTxnFsm) TxnSuccess(txn *transaction.Transaction) (transaction.TxnEvent, error) {
	switch txn.MsgType {
	case svcmsgtypes.PfcpSessCreate:

		nextTxn := transaction.NewTransaction(nil, nil, svcmsgtypes.N1N2MessageTransfer)
		nextTxn.Ctxt = txn.Ctxt
		smContext := txn.Ctxt.(*smf_context.SMContext)
		smContext.SMTxnBusLock.Lock()
		smContext.TxnBus = smContext.TxnBus.AddTxn(nextTxn)
		smContext.SMTxnBusLock.Unlock()
		go func(nextTxn *transaction.Transaction) {
			// Initiate N1N2 Transfer

			// nextTxn.StartTxnLifeCycle(SmfTxnFsmHandle)
			<-nextTxn.Status
		}(nextTxn)
	}

	// put Success Rsp
	txn.Status <- true
	return transaction.TxnEventSave, nil
}

func (SmfTxnFsm) TxnFailure(txn *transaction.Transaction) (transaction.TxnEvent, error) {
	// Put Failure Rsp
	switch txn.MsgType {
	case svcmsgtypes.PfcpSessCreate:
		if txn.Ctxt != nil && txn.Ctxt.(*smf_context.SMContext).SMContextState == context.SmStatePfcpCreatePending {
			nextTxn := transaction.NewTransaction(nil, nil, svcmsgtypes.PfcpSessCreateFailure)
			nextTxn.Ctxt = txn.Ctxt
			smContext := txn.Ctxt.(*smf_context.SMContext)
			smContext.SMTxnBusLock.Lock()
			smContext.TxnBus = smContext.TxnBus.AddTxn(nextTxn)
			smContext.SMTxnBusLock.Unlock()
			go func(nextTxn *transaction.Transaction) {
				// Initiate N1N2 Transfer

				// nextTxn.StartTxnLifeCycle(SmfTxnFsmHandle)
				<-nextTxn.Status
			}(nextTxn)
		}

	case svcmsgtypes.UpdateSmContext:
		if txn.Ctxt == nil {
			logger.PduSessLog.Warnf("PDUSessionSMContextUpdate, SMContext[%s] is not found", txn.CtxtKey)

			httpResponse := &httpwrapper.Response{
				Header: nil,
				Status: http.StatusNotFound,
				Body: models.UpdateSmContextErrorResponse{
					JsonData: &models.SmContextUpdateError{
						UpCnxState: models.UpCnxState_DEACTIVATED,
						Error: &models.ProblemDetails{
							Type:   "Resource Not Found",
							Title:  "SMContext Ref is not found",
							Status: http.StatusNotFound,
						},
					},
				},
			}
			txn.Rsp = httpResponse
		}

	case svcmsgtypes.ReleaseSmContext:
		if txn.Ctxt == nil {
			logger.PduSessLog.Warnf("PDUSessionSMContextRelease [%s] is not found", txn.CtxtKey)

			// 4xx/5xx Error not defined in spec 29502 for Release SM ctxt error
			// Send Not Found
			httpResponse := &httpwrapper.Response{
				Header: nil,
				Status: http.StatusNotFound,

				Body: &models.ProblemDetails{
					Type:   "Resource Not Found",
					Title:  "SMContext Ref is not found",
					Status: http.StatusNotFound,
				},
			}
			txn.Rsp = httpResponse
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

	smContext := txn.Ctxt.(*smf_context.SMContext)
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

/// Suggestions
//1. Global pipeline for txns
//2. Memory alloc pool for txns
