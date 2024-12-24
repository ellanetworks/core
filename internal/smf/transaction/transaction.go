package transaction

import (
	"fmt"
	"sync/atomic"
	"time"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/smf/msgtypes/svcmsgtypes"
	"go.uber.org/zap"
)

type Transaction struct {
	startTime, endTime time.Time
	Req                interface{}
	Rsp                interface{}
	Ctxt               interface{}
	CtxtKey            string
	Err                error
	Status             chan bool
	NextTxn            *Transaction
	TxnFsmLog          *zap.SugaredLogger
	MsgType            svcmsgtypes.SmfMsgType
	TxnId              uint32
	Priority           uint32
}

func (t *Transaction) initLogTags() {
	t.TxnFsmLog = logger.SmfLog.With("txnid", t.TxnId, "txntype", string(t.MsgType), "ctxtkey", t.CtxtKey)
}

type TxnEvent uint

const (
	TxnEventInit TxnEvent = iota
	TxnEventDecode
	TxnEventLoadCtxt
	TxnEventCtxtPost
	TxnEventRun
	TxnEventProcess
	TxnEventSuccess
	TxnEventFailure
	TxnEventTimeout
	TxnEventAbort
	TxnEventSave
	TxnEventCollision
	TxnEventQueue
	TxnEventEnd
	TxnEventExit
)

func (e TxnEvent) String() string {
	switch e {
	case TxnEventInit:
		return "TxnEventInit"
	case TxnEventDecode:
		return "TxnEventDecode"
	case TxnEventLoadCtxt:
		return "TxnEventLoadCtxt"
	case TxnEventCtxtPost:
		return "TxnEventPost"
	case TxnEventRun:
		return "TxnEventRun"
	case TxnEventProcess:
		return "TxnEventProcess"
	case TxnEventSuccess:
		return "TxnEventSuccess"
	case TxnEventFailure:
		return "TxnEventFailure"
	case TxnEventTimeout:
		return "TxnEventTimeout"
	case TxnEventAbort:
		return "TxnEventAbort"
	case TxnEventSave:
		return "TxnEventSave"
	case TxnEventCollision:
		return "TxnEventCollision"
	case TxnEventQueue:
		return "TxnEventQueue"
	case TxnEventEnd:
		return "TxnEventEnd"
	case TxnEventExit:
		return "TxnEventExit"
	default:
		return "TxnEventInvalid"
	}
}

var TxnId uint32

func getNewTxnId() uint32 {
	atomic.AddUint32(&TxnId, 1)
	return TxnId
}

func NewTransaction(req, rsp interface{}, msgType svcmsgtypes.SmfMsgType) *Transaction {
	t := &Transaction{
		Req:       req,
		Rsp:       rsp,
		MsgType:   msgType,
		startTime: time.Now(),
		TxnId:     getNewTxnId(),
		Status:    make(chan bool),
	}

	t.initLogTags()
	return t
}

func (t *Transaction) TransactionEnd() {
	t.endTime = time.Now()
}

type TxnBus []*Transaction

func (txnBus TxnBus) AddTxn(t *Transaction) TxnBus {
	txnBus = append(txnBus, t)
	return txnBus
}

func (txnBus TxnBus) PopTxn() (*Transaction, TxnBus) {
	if len(txnBus) != 0 {
		txn := txnBus[0]
		txnBus = txnBus[1:]
		return txn, txnBus
	}
	return nil, txnBus
}

type txnFsm interface {
	TxnInit(t *Transaction) (TxnEvent, error)
	TxnDecode(t *Transaction) (TxnEvent, error)
	TxnLoadCtxt(t *Transaction) (TxnEvent, error)
	TxnCtxtPost(t *Transaction) (TxnEvent, error)
	TxnCtxtRun(t *Transaction) (TxnEvent, error)
	TxnProcess(t *Transaction) (TxnEvent, error)
	TxnSuccess(t *Transaction) (TxnEvent, error)
	TxnFailure(t *Transaction) (TxnEvent, error)
	TxnAbort(t *Transaction) (TxnEvent, error)
	TxnSave(t *Transaction) (TxnEvent, error)
	TxnTimeout(t *Transaction) (TxnEvent, error)
	TxnCollision(t *Transaction) (TxnEvent, error)
	TxnEnd(t *Transaction) (TxnEvent, error)
}

type txnFsmHandler [TxnEventExit]func(t *Transaction) (TxnEvent, error)

var TxnFsmHandler txnFsmHandler

func InitTxnFsm(fsm txnFsm) {
	TxnFsmHandler[TxnEventInit] = fsm.TxnInit
	TxnFsmHandler[TxnEventDecode] = fsm.TxnDecode
	TxnFsmHandler[TxnEventLoadCtxt] = fsm.TxnLoadCtxt
	TxnFsmHandler[TxnEventCtxtPost] = fsm.TxnCtxtPost
	TxnFsmHandler[TxnEventRun] = fsm.TxnCtxtRun
	TxnFsmHandler[TxnEventProcess] = fsm.TxnProcess
	TxnFsmHandler[TxnEventSuccess] = fsm.TxnSuccess
	TxnFsmHandler[TxnEventFailure] = fsm.TxnFailure
	TxnFsmHandler[TxnEventTimeout] = fsm.TxnTimeout
	TxnFsmHandler[TxnEventAbort] = fsm.TxnAbort
	TxnFsmHandler[TxnEventSave] = fsm.TxnSave
	TxnFsmHandler[TxnEventEnd] = fsm.TxnEnd
}

func (t *Transaction) StartTxnLifeCycle(fsm txnFsm) {
	nextEvent := TxnEventInit
	var err error

	for {
		currEvent := nextEvent
		if nextEvent, err = TxnFsmHandler[currEvent](t); err != nil {
			t.TxnFsmLog.Errorf("error processing state machine transaction at stage [%s]: %v", currEvent.String(), err.Error())
		}

		// Current active txn is over, Schedule Next Txn if available
		if currEvent == TxnEventEnd && nextEvent == TxnEventRun {
			if t.NextTxn != nil {
				t = t.NextTxn
			}
		} else

		// Finish FSM
		// Note- Pipelined Txn will not get chance to run immediately,
		// so they shall exit FSM and shall wait to run in TxnBus
		if nextEvent == TxnEventExit || nextEvent == TxnEventQueue {
			t.TxnFsmLog.Debugf("TxnFsm [%v] ", nextEvent.String())
			return
		}
	}
}

func (t Transaction) String() string {
	return fmt.Sprintf(" txn-id [%v], txn-type [%v], txn-key [%v] ", t.TxnId, t.MsgType, t.CtxtKey)
}
