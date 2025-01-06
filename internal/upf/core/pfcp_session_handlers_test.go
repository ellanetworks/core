// Copyright 2024 Ella Networks
package core

import (
	"net"
	"testing"

	"github.com/wmnsk/go-pfcp/ie"
	"github.com/wmnsk/go-pfcp/message"
)

func PreparePfcpConnection(t *testing.T) (PfcpConnection, string) {
	mapOps := MapOperationsMock{}

	smfIP := "127.0.0.1"
	pfcpConn := PfcpConnection{
		NodeAssociations: make(map[string]*NodeAssociation),
		nodeId:           "NodeId",
		mapOperations:    &mapOps,
		n3Address:        net.ParseIP("1.2.3.4"),
	}

	pfcpConn.NodeAssociations[smfIP] = NewNodeAssociation("0.0.0.0", "0.0.0.0")

	return pfcpConn, smfIP
}

func SendDefaulMappingPdrs(t *testing.T, pfcpConn *PfcpConnection, smfIP string) {
	ip1, _ := net.ResolveIPAddr("ip", UeAddress1)
	ip2, _ := net.ResolveIPAddr("ip", UeAddress2)

	// Requests for default mapping (without SDF filter)

	// Request with UEIP Address
	seReqPre1 := message.NewSessionEstablishmentRequest(0, 0,
		2, 1, 0,
		ie.NewNodeID("", "", "test"),
		ie.NewFSEID(1, net.ParseIP(smfIP), nil),
		ie.NewCreatePDR(
			ie.NewPDRID(1),
			ie.NewPDI(
				ie.NewSourceInterface(ie.SrcInterfaceCore),
				ie.NewUEIPAddress(2, ip1.IP.String(), "", 0, 0),
			),
		),
	)

	// Request with TEID
	seReqPre2 := message.NewSessionEstablishmentRequest(0, 0,
		3, 1, 0,
		ie.NewNodeID("", "", "test"),
		ie.NewFSEID(2, net.ParseIP(smfIP), nil),
		ie.NewCreatePDR(
			ie.NewPDRID(1),
			ie.NewPDI(
				ie.NewSourceInterface(ie.SrcInterfaceCore),
				ie.NewFTEID(0, 0, ip2.IP, nil, 0),
			),
		),
	)

	var err error
	_, err = HandlePfcpSessionEstablishmentRequest(seReqPre1)
	if err != nil {
		t.Errorf("Error handling session establishment request: %s", err)
	}

	_, err = HandlePfcpSessionEstablishmentRequest(seReqPre2)
	if err != nil {
		t.Errorf("Error handling session establishment request: %s", err)
	}

	// Check that session PDRs are correct
	if pfcpConn.NodeAssociations[smfIP].Sessions[2].PDRs[1].Ipv4.String() != UeAddress1 {
		t.Errorf("Session 1, got broken")
	}
	if pfcpConn.NodeAssociations[smfIP].Sessions[3].PDRs[1].Teid != 0 {
		t.Errorf("Session 2, got broken")
	}
}
