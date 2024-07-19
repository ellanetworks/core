package core

import (
	"fmt"
	"net"

	"github.com/yeastengine/ella/internal/upf/config"
	"github.com/yeastengine/ella/internal/upf/logger"

	"github.com/wmnsk/go-pfcp/ie"
	"github.com/wmnsk/go-pfcp/message"
)

type PfcpFunc func(conn *PfcpConnection, msg message.Message, addr string) (message.Message, error)

type PfcpHandlerMap map[uint8]PfcpFunc

func (handlerMap PfcpHandlerMap) Handle(conn *PfcpConnection, buf []byte, addr *net.UDPAddr) error {
	logger.AppLog.Debugf("Handling PFCP message from %s", addr)
	incomingMsg, err := message.Parse(buf)
	if err != nil {
		logger.AppLog.Warnf("Ignored undecodable message: %x, error: %s", buf, err)
		return err
	}
	if handler, ok := handlerMap[incomingMsg.MessageType()]; ok {
		// TODO: Trim port as a workaround for NAT changing the port. Explore proper solutions.
		stringIpAddr := addr.IP.String()
		if stringIpAddr == "::1" {
			logger.AppLog.Debugf("Got loopback address, setting to 0.0.0.0")
			stringIpAddr = "0.0.0.0"
		}
		outgoingMsg, err := handler(conn, incomingMsg, stringIpAddr)
		if err != nil {
			logger.AppLog.Warnf("Error handling PFCP message: %s", err.Error())
			return err
		}
		// Now assumption that all handlers will return a message to send is not true.
		if outgoingMsg != nil {
			return conn.SendMessage(outgoingMsg, addr)
		}
		return nil
	} else {
		logger.AppLog.Warnf("Got unexpected message %s: %s, from: %s", incomingMsg.MessageTypeName(), incomingMsg, addr)
	}
	return nil
}

func setBit(n uint8, pos uint) uint8 {
	n |= (1 << pos)
	return n
}

// https://www.etsi.org/deliver/etsi_ts/129200_129299/129244/16.04.00_60/ts_129244v160400p.pdf page 95
func HandlePfcpAssociationSetupRequest(conn *PfcpConnection, msg message.Message, addr string) (message.Message, error) {
	asreq := msg.(*message.AssociationSetupRequest)
	logger.AppLog.Infof("Got Association Setup Request from: %s. \n", addr)
	if asreq.NodeID == nil {
		logger.AppLog.Warnf("Got Association Setup Request without NodeID from: %s", addr)
		// Reject with cause

		asres := message.NewAssociationSetupResponse(asreq.SequenceNumber,
			ie.NewCause(ie.CauseMandatoryIEMissing),
		)
		return asres, nil
	}
	printAssociationSetupRequest(asreq)
	// Get NodeID
	remoteNodeID, err := asreq.NodeID.NodeID()
	if err != nil {
		logger.AppLog.Warnf("Got Association Setup Request with invalid NodeID from: %s", addr)
		asres := message.NewAssociationSetupResponse(asreq.SequenceNumber,
			ie.NewCause(ie.CauseMandatoryIEMissing),
		)
		return asres, nil
	}
	// Check if the PFCP Association Setup Request contains a Node ID for which a PFCP association was already established
	if _, ok := conn.NodeAssociations[remoteNodeID]; ok {
		logger.AppLog.Warnf("Association Setup Request with NodeID: %s from: %s already exists", remoteNodeID, addr)
		// retain the PFCP sessions that were established with the existing PFCP association and that are requested to be retained, if the PFCP Session Retention Information IE was received in the request; otherwise, delete the PFCP sessions that were established with the existing PFCP association;
		logger.AppLog.Warnf("Session retention is not yet implemented")
	}

	// If the PFCP Association Setup Request contains a Node ID for which a PFCP association was already established
	// proceed with establishing the new PFCP association (regardless of the Recovery AssociationStart received in the request), overwriting the existing association;
	// if the request is accepted:
	// shall store the Node ID of the CP function as the identifier of the PFCP association;
	// Create RemoteNode from AssociationSetupRequest

	// String to IP
	remoteAddress := net.ParseIP(addr).To4().String()
	fmt.Println("ok! Remote address: ", remoteAddress)
	remoteNode := NewNodeAssociation(remoteNodeID, addr)
	fmt.Println("New node association with address: ", addr)
	// Add or replace RemoteNode to NodeAssociationMap
	conn.NodeAssociations[addr] = remoteNode
	logger.AppLog.Infof("Saving new association: %+v", remoteNode)
	featuresOctets := []uint8{0, 0, 0}
	if config.Conf.FeatureFTUP {
		featuresOctets[0] = setBit(featuresOctets[0], 4)
	}
	if config.Conf.FeatureUEIP {
		featuresOctets[2] = setBit(featuresOctets[2], 2)
	}
	upFunctionFeaturesIE := ie.NewUPFunctionFeatures(featuresOctets[:]...)

	// shall send a PFCP Association Setup Response including:
	dnn := "internet"
	flags := uint8(0x61)
	networkInstance := string(ie.NewNetworkInstanceFQDN(dnn).Payload)
	asres := message.NewAssociationSetupResponse(asreq.SequenceNumber,
		ie.NewCause(ie.CauseRequestAccepted), // a successful cause
		newIeNodeID(conn.nodeId),             // its Node ID;
		ie.NewRecoveryTimeStamp(conn.RecoveryTimestamp),
		ie.NewUserPlaneIPResourceInformation(flags, 0, conn.n3Address.String(), "", networkInstance, ie.SrcInterfaceAccess),
		upFunctionFeaturesIE,
	)

	// Send AssociationSetupResponse
	return asres, nil
}

func newIeNodeID(nodeID string) *ie.IE {
	ip := net.ParseIP(nodeID)
	if ip != nil {
		if ip.To4() != nil {
			return ie.NewNodeID(nodeID, "", "")
		}
		return ie.NewNodeID("", nodeID, "")
	}
	return ie.NewNodeID("", "", nodeID)
}
