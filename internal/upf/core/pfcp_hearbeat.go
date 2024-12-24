package core

import (
	"net"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/wmnsk/go-pfcp/ie"
	"github.com/wmnsk/go-pfcp/message"
)

func HandlePfcpHeartbeatRequest(conn *PfcpConnection, msg message.Message, addr string) (message.Message, error) {
	hbreq := msg.(*message.HeartbeatRequest)
	if association := conn.GetAssociation(addr); association != nil {
		association.ResetFailedHeartbeats()
	}
	ts, err := hbreq.RecoveryTimeStamp.RecoveryTimeStamp()
	if err != nil {
		logger.UpfLog.Warnf("Got Heartbeat Request with invalid TS: %s, from: %s", err, addr)
		return nil, err
	} else {
		logger.UpfLog.Debugf("Got Heartbeat Request with TS: %s, from: %s", ts, addr)
	}

	hbres := message.NewHeartbeatResponse(hbreq.SequenceNumber, ie.NewRecoveryTimeStamp(conn.RecoveryTimestamp))
	logger.UpfLog.Debugf("Sent Heartbeat Response to: %s", addr)
	return hbres, nil
}

func HandlePfcpHeartbeatResponse(conn *PfcpConnection, msg message.Message, addr string) (message.Message, error) {
	hbresp := msg.(*message.HeartbeatResponse)
	ts, err := hbresp.RecoveryTimeStamp.RecoveryTimeStamp()
	if err != nil {
		logger.UpfLog.Warnf("Got Heartbeat Response with invalid TS: %s, from: %s", err, addr)
		return nil, err
	}
	logger.UpfLog.Debugf("Got Heartbeat Response with TS: %s, from: %s", ts, addr)

	if association := conn.GetAssociation(addr); association != nil {
		association.HandleHeartbeat(msg.Sequence())
	} else {
		logger.UpfLog.Warnf("Got Heartbeat Response from unknown peer: %s", addr)
	}
	return nil, err
}

func SendHeartbeatRequest(conn *PfcpConnection, sequenceID uint32, associationAddr string) {
	hbreq := message.NewHeartbeatRequest(sequenceID, ie.NewRecoveryTimeStamp(conn.RecoveryTimestamp), nil)
	logger.UpfLog.Debugf("Sent Heartbeat Request to: %s", associationAddr)
	udpAddr, err := net.ResolveUDPAddr("udp", associationAddr+":8805")
	if err == nil {
		if err := conn.SendMessage(hbreq, udpAddr); err != nil {
			logger.UpfLog.Infof("Failed to send Heartbeat Request: %s\n", err.Error())
		}
	} else {
		logger.UpfLog.Infof("Failed to send Heartbeat Request: %s\n", err.Error())
	}
}
