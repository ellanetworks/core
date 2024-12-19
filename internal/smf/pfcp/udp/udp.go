package udp

import (
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/wmnsk/go-pfcp/message"
	"github.com/yeastengine/ella/internal/logger"
	"github.com/yeastengine/ella/internal/smf/context"
)

const PFCP_MAX_UDP_LEN = 2048

type ConsumerTable struct {
	m sync.Map // map[string]TxTable
}

type PfcpEventData struct {
	ErrHandler func(message.Message, error)
	LSEID      uint64
}

type PfcpServer struct {
	Addr *net.UDPAddr
	Conn *net.UDPConn
	// Consumer Table
	// Map Consumer IP to its tx table
	ConsumerTable ConsumerTable
}

var Server *PfcpServer

var ServerStartTime time.Time

func (t *ConsumerTable) Load(consumerAddr string) (*TxTable, bool) {
	txTable, ok := t.m.Load(consumerAddr)
	if ok {
		return txTable.(*TxTable), ok
	}
	return nil, false
}

func (t *ConsumerTable) Store(consumerAddr string, txTable *TxTable) {
	t.m.Store(consumerAddr, txTable)
}

func Run(Dispatch func(*Message)) {
	addr := &net.UDPAddr{
		IP:   net.ParseIP(context.SMF_Self().CPNodeID.ResolveNodeIdToIp().String()),
		Port: context.SMF_Self().PFCPPort,
	}
	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		logger.SmfLog.Errorf("Failed to listen on %s: %v", addr.String(), err)
		return
	}
	Server = &PfcpServer{
		Addr: addr,
		Conn: conn,
	}
	logger.SmfLog.Infof("PFCP server started on %s", addr.String())

	go func() {
		for {
			remoteAddr, pfcpMessage, eventData, err := readPfcpMessage()
			if err != nil {
				if err.Error() == "Receive resend PFCP request" {
					logger.SmfLog.Infoln(err)
				} else {
					logger.SmfLog.Warnf("Read PFCP error: %v", err)
				}
				continue
			}
			msg := NewMessage(remoteAddr, pfcpMessage, eventData)
			go Dispatch(&msg)
		}
	}()

	ServerStartTime = time.Now()
}

func WaitForServer() error {
	timeout := 10 * time.Second
	t0 := time.Now()
	for {
		if time.Since(t0) > timeout {
			return fmt.Errorf("timeout waiting for PFCP server to start")
		}
		if Server != nil && Server.Conn != nil {
			return nil
		}
		logger.SmfLog.Infof("Waiting for PFCP server to start...")
		time.Sleep(1 * time.Second)
	}
}

func SendPfcp(msg message.Message, addr *net.UDPAddr, eventData interface{}) error {
	if Server == nil {
		return fmt.Errorf("PFCP server is not initialized")
	}
	if Server.Conn == nil {
		return fmt.Errorf("PFCP server is not listening")
	}

	buf := make([]byte, msg.MarshalLen())
	err := msg.MarshalTo(buf)
	if err != nil {
		return err
	}

	tx := NewTransaction(msg, buf, Server.Conn, addr, eventData)
	err = PutTransaction(tx)
	if err != nil {
		logger.SmfLog.Errorf("Failed to send PFCP message: %v", err)
		return err
	}
	go startTxLifeCycle(tx)
	return nil
}

func readPfcpMessage() (*net.UDPAddr, message.Message, interface{}, error) {
	if Server == nil {
		return nil, nil, nil, fmt.Errorf("PFCP server is not initialized")
	}
	if Server.Conn == nil {
		return nil, nil, nil, fmt.Errorf("PFCP server is not listening")
	}

	buf := make([]byte, PFCP_MAX_UDP_LEN)
	n, addr, err := Server.Conn.ReadFromUDP(buf)
	if err != nil {
		return addr, nil, nil, err
	}

	msg, err := message.Parse(buf[:n])
	if err != nil {
		logger.SmfLog.Errorf("error parsing PFCP message: %v", err)
		return addr, nil, nil, err
	}

	var eventData interface{}
	if IsRequest(msg) {
		tx, err := findTransaction(msg, addr)
		if err != nil {
			return addr, msg, nil, err
		} else if tx != nil {
			// err == nil && tx != nil => Resend Request
			err = fmt.Errorf("receive resend PFCP request")
			tx.EventChannel <- ReceiveResendRequest
			return addr, msg, nil, err
		} else {
			// err == nil && tx == nil => New Request
			return addr, msg, nil, nil
		}
	} else if IsResponse(msg) {
		tx, err := findTransaction(msg, Server.Addr)
		if err != nil {
			return addr, msg, nil, err
		}
		eventData = tx.EventData
		tx.EventChannel <- ReceiveValidResponse
	}

	return addr, msg, eventData, nil
}

func findTransaction(msg message.Message, addr *net.UDPAddr) (*Transaction, error) {
	var tx *Transaction
	consumerAddr := addr.String()

	if Server == nil {
		return nil, fmt.Errorf("PFCP server is not initialized")
	}

	if IsResponse(msg) {
		if _, exist := Server.ConsumerTable.Load(consumerAddr); !exist {
			return nil, fmt.Errorf("txTable not found for consumer %s", consumerAddr)
		}

		txTable, _ := Server.ConsumerTable.Load(consumerAddr)
		seqNum := msg.Sequence()

		if _, exist := txTable.Load(seqNum); !exist {
			return nil, fmt.Errorf("sequence number [%d] not found", seqNum)
		}

		tx, _ = txTable.Load(seqNum)
	} else if IsRequest(msg) {
		if _, exist := Server.ConsumerTable.Load(consumerAddr); !exist {
			return nil, nil
		}
		txTable, _ := Server.ConsumerTable.Load(consumerAddr)
		seqNum := msg.Sequence()
		if _, exist := txTable.Load(seqNum); !exist {
			return nil, nil
		}
		tx, _ = txTable.Load(seqNum)
	}
	return tx, nil
}

func PutTransaction(tx *Transaction) error {
	consumerAddr := tx.ConsumerAddr
	if _, exist := Server.ConsumerTable.Load(consumerAddr); !exist {
		Server.ConsumerTable.Store(consumerAddr, &TxTable{})
	}
	txTable, _ := Server.ConsumerTable.Load(consumerAddr)
	if _, exist := txTable.Load(tx.SequenceNumber); !exist {
		txTable.Store(tx.SequenceNumber, tx)
	} else {
		return fmt.Errorf("insert tx error: duplicate sequence number %d", tx.SequenceNumber)
	}
	return nil
}

func startTxLifeCycle(tx *Transaction) {
	sendErr := tx.Start()

	err := removeTransaction(tx)
	if err != nil {
		logger.SmfLog.Warnln(err)
	}

	if sendErr != nil && tx.EventData != nil {
		if eventData, ok := tx.EventData.(PfcpEventData); ok {
			if errHandler := eventData.ErrHandler; errHandler != nil {
				msg, err := message.Parse(tx.SendMsg)
				if err != nil {
					logger.SmfLog.Warnf("Parse message error: %v", err)
					return
				}
				errHandler(msg, sendErr)
			}
		}
	}
}

func removeTransaction(tx *Transaction) error {
	if Server == nil {
		return fmt.Errorf("PFCP server is not initialized")
	}
	consumerAddr := tx.ConsumerAddr
	txTable, _ := Server.ConsumerTable.Load(consumerAddr)

	if txTmp, exist := txTable.Load(tx.SequenceNumber); exist {
		tx = txTmp

		txTable.Delete(tx.SequenceNumber)
	} else {
		return fmt.Errorf("remove tx error: transaction [%d] doesn't exist", tx.SequenceNumber)
	}
	return nil
}
