package core

import (
	"net"

	"github.com/wmnsk/go-pfcp/ie"
	"github.com/wmnsk/go-pfcp/message"
	"github.com/yeastengine/ella/internal/upf/logger"
)

func HandlePfcpHeartbeatRequest(conn *PfcpConnection, msg message.Message, addr string) (message.Message, error) {
	hbreq := msg.(*message.HeartbeatRequest)
	if association := conn.GetAssociation(addr); association != nil {
		association.ResetFailedHeartbeats()
	}
	ts, err := hbreq.RecoveryTimeStamp.RecoveryTimeStamp()
	if err != nil {
		logger.AppLog.Warnf("Got Heartbeat Request with invalid TS: %s, from: %s", err, addr)
		return nil, err
	} else {
		logger.AppLog.Debugf("Got Heartbeat Request with TS: %s, from: %s", ts, addr)
	}

	hbres := message.NewHeartbeatResponse(hbreq.SequenceNumber, ie.NewRecoveryTimeStamp(conn.RecoveryTimestamp))
	logger.AppLog.Debugf("Sent Heartbeat Response to: %s", addr)
	return hbres, nil
}

func HandlePfcpHeartbeatResponse(conn *PfcpConnection, msg message.Message, addr string) (message.Message, error) {
	hbresp := msg.(*message.HeartbeatResponse)
	ts, err := hbresp.RecoveryTimeStamp.RecoveryTimeStamp()
	if err != nil {
		logger.AppLog.Warnf("Got Heartbeat Response with invalid TS: %s, from: %s", err, addr)
		return nil, err
	} else {
		logger.AppLog.Debugf("Got Heartbeat Response with TS: %s, from: %s", ts, addr)
	}

	if association := conn.GetAssociation(addr); association != nil {
		association.HandleHeartbeat(msg.Sequence())
	}
	return nil, err
}

func SendHeartbeatRequest(conn *PfcpConnection, sequenceID uint32, associationAddr string) {
	hbreq := message.NewHeartbeatRequest(sequenceID, ie.NewRecoveryTimeStamp(conn.RecoveryTimestamp), nil)
	logger.AppLog.Debugf("Sent Heartbeat Request to: %s", associationAddr)
	udpAddr, err := net.ResolveUDPAddr("udp", associationAddr+":8805")
	if err == nil {
		if err := conn.SendMessage(hbreq, udpAddr); err != nil {
			logger.AppLog.Infof("Failed to send Heartbeat Request: %s\n", err.Error())
		}
	} else {
		logger.AppLog.Infof("Failed to send Heartbeat Request: %s\n", err.Error())
	}
}
