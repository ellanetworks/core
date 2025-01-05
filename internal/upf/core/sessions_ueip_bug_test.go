// Copyright 2024 Ella Networks
package core

// // Test for the bug where the session's UE IP address is overwritten because it is a pointer to the buffer for incoming UDP packets.
// func TestSessionUEIpOverwrite(t *testing.T) {
// 	mapOps := MapOperationsMock{}

// 	pfcpHandlers := PfcpHandlerMap{
// 		message.MsgTypeSessionDeletionRequest:     HandlePfcpSessionDeletionRequest,
// 		message.MsgTypeSessionModificationRequest: HandlePfcpSessionModificationRequest,
// 	}
// 	smfIP := "127.0.0.1"
// 	pfcpConn := PfcpConnection{
// 		NodeAssociations: make(map[string]*NodeAssociation),
// 		nodeId:           "test-node",
// 		mapOperations:    &mapOps,
// 		pfcpHandlerMap:   pfcpHandlers,
// 	}

// 	pfcpConn.NodeAssociations[smfIP] = NewNodeAssociation("0.0.0.0", "0.0.0.0")

// 	ip1, _ := net.ResolveIPAddr("ip", "1.1.1.1")
// 	ip2, _ := net.ResolveIPAddr("ip", "2.2.2.2")

// 	// uip1, _ := net.ResolveUDPAddr("ip", "1.1.1.1")
// 	// uip2, _ := net.ResolveUDPAddr("ip", "2.2.2.2")

// 	// Create two send two Session Establishment Requests with downlink PDRs
// 	// and check that the first session is not overwritten
// 	seReq1 := message.NewSessionEstablishmentRequest(0, 0,
// 		1, 1, 0,
// 		ie.NewNodeID("", "", "test"),
// 		ie.NewFSEID(1, net.ParseIP(smfIP), nil),
// 		ie.NewCreatePDR(
// 			ie.NewPDRID(1),
// 			ie.NewPDI(
// 				ie.NewSourceInterface(ie.SrcInterfaceCore),
// 				// ie.NewFTEID(0, 0, ip1.IP, nil, 0),
// 				ie.NewUEIPAddress(2, ip1.IP.String(), "", 0, 0),
// 			),
// 		),
// 	)

// 	seReq2 := message.NewSessionEstablishmentRequest(0, 0,
// 		2, 1, 0,
// 		ie.NewNodeID("", "", "test"),
// 		ie.NewFSEID(2, net.ParseIP(smfIP), nil),
// 		ie.NewCreatePDR(
// 			ie.NewPDRID(1),
// 			ie.NewPDI(
// 				ie.NewSourceInterface(ie.SrcInterfaceCore),
// 				// ie.NewFTEID(0, 0, ip2.IP, nil, 0),
// 				ie.NewUEIPAddress(2, ip2.IP.String(), "", 0, 0),
// 			),
// 		),
// 	)

// 	// Send first request
// 	_, err := HandlePfcpSessionEstablishmentRequest(seReq1)
// 	if err != nil {
// 		t.Errorf("Error handling session establishment request: %s", err)
// 	}

// 	// Send second request
// 	_, err = HandlePfcpSessionEstablishmentRequest(seReq2)
// 	if err != nil {
// 		t.Errorf("Error handling session establishment request: %s", err)
// 	}

// 	// Check that session PDRs are correct
// 	if pfcpConn.NodeAssociations[smfIP].Sessions[2].PDRs[1].Ipv4.String() != "1.1.1.1" {
// 		t.Errorf("Session 1, got broken")
// 	}
// 	if pfcpConn.NodeAssociations[smfIP].Sessions[3].PDRs[1].Ipv4.String() != "2.2.2.2" {
// 		t.Errorf("Session 2, got broken")
// 	}
// }
