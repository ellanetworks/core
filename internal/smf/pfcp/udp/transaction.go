// Copyright 2024 Ella Networks
// Copyright 2019 free5GC.org
// Copyright 2024 Canonical Ltd.
// SPDX-License-Identifier: Apache-2.0

package udp

import (
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/smf/context"
	"github.com/wmnsk/go-pfcp/message"
)

type TransactionType uint8

type TxTable struct {
	m sync.Map // map[uint32]*Transaction
}

func (t *TxTable) Store(sequenceNumber uint32, tx *Transaction) {
	t.m.Store(sequenceNumber, tx)
}

func (t *TxTable) Load(sequenceNumber uint32) (*Transaction, bool) {
	if t == nil {
		logger.SmfLog.Warnf("TxTable is nil")
		return nil, false
	}

	tx, ok := t.m.Load(sequenceNumber)
	if ok {
		return tx.(*Transaction), ok
	}
	return nil, false
}

func (t *TxTable) Delete(sequenceNumber uint32) {
	t.m.Delete(sequenceNumber)
}

const (
	SendingRequest TransactionType = iota
	SendingResponse
)

const (
	NumOfResend                 = 3
	ResendRequestTimeOutPeriod  = 3
	ResendResponseTimeOutPeriod = 15
)

type Transaction struct {
	EventChannel   chan EventType
	Conn           *net.UDPConn
	DestAddr       *net.UDPAddr
	ConsumerAddr   string
	ErrHandler     func(*message.Message, error)
	EventData      interface{}
	SendMsg        []byte
	SequenceNumber uint32
	MessageType    uint8
	TxType         TransactionType
}

func NewTransaction(pfcpMSG message.Message, binaryMSG []byte, Conn *net.UDPConn, DestAddr *net.UDPAddr, eventData interface{}) *Transaction {
	tx := &Transaction{
		SendMsg:        binaryMSG,
		SequenceNumber: pfcpMSG.Sequence(),
		MessageType:    pfcpMSG.MessageType(),
		EventChannel:   make(chan EventType, 1),
		Conn:           Conn,
		DestAddr:       DestAddr,
		EventData:      eventData,
	}
	self := context.SMF_Self()
	if IsRequest(pfcpMSG) {
		tx.TxType = SendingRequest
		udpAddr := &net.UDPAddr{
			IP:   net.ParseIP(context.SMF_Self().CPNodeID.ResolveNodeIdToIp().String()),
			Port: self.PFCPPort,
		}
		tx.ConsumerAddr = udpAddr.String()
	} else if IsResponse(pfcpMSG) {
		tx.TxType = SendingResponse
		tx.ConsumerAddr = DestAddr.String()
	}
	logger.SmfLog.Debugf("New Transaction SEQ[%d] DestAddr[%s]", tx.SequenceNumber, DestAddr.String())
	return tx
}

func (transaction *Transaction) Start() error {
	if transaction.TxType == SendingRequest {
		for iter := 0; iter < NumOfResend; iter++ {
			timer := time.NewTimer(ResendRequestTimeOutPeriod * time.Second)
			_, err := transaction.Conn.WriteToUDP(transaction.SendMsg, transaction.DestAddr)
			if err != nil {
				return err
			}

			select {
			case event := <-transaction.EventChannel:

				if event == ReceiveValidResponse {
					logger.SmfLog.Debugf("Request Transaction [%d]: receive valid response\n", transaction.SequenceNumber)
					return nil
				}
			case <-timer.C:
				logger.SmfLog.Debugf("Request Transaction [%d]: timeout expire\n", transaction.SequenceNumber)
				logger.SmfLog.Debugf("Request Transaction [%d]: Resend packet\n", transaction.SequenceNumber)
				continue
			}
		}
		return fmt.Errorf("request timeout, seq [%d]", transaction.SequenceNumber)
	} else if transaction.TxType == SendingResponse {
		timer := time.NewTimer(ResendResponseTimeOutPeriod * time.Second)
		for iter := 0; iter < NumOfResend; iter++ {
			_, err := transaction.Conn.WriteToUDP(transaction.SendMsg, transaction.DestAddr)
			if err != nil {
				logger.SmfLog.Warnf("Response Transaction [%d]: sending error\n", transaction.SequenceNumber)
				return err
			}

			select {
			case event := <-transaction.EventChannel:

				if event == ReceiveResendRequest {
					logger.SmfLog.Debugf("Response Transaction [%d]: receive resend request\n", transaction.SequenceNumber)
					logger.SmfLog.Debugf("Response Transaction [%d]: Resend packet\n", transaction.SequenceNumber)
					continue
				}
			case <-timer.C:
				logger.SmfLog.Debugf("Response Transaction [%d]: timeout expire\n", transaction.SequenceNumber)
				return fmt.Errorf("response timeout, seq [%d]", transaction.SequenceNumber)
			}
		}
	}
	return nil
}
