// Copyright 2024 Ella Networks
package core

import (
	"net"

	"github.com/ellanetworks/core/internal/upf/ebpf"
)

const (
	RemoteIP   = "127.0.0.1"
	UeAddress1 = "1.1.1.1"
	UeAddress2 = "2.2.2.2"
	NodeId     = "test-node"
)

type MapOperationsMock struct{}

func (mapOps *MapOperationsMock) PutPdrUplink(teid uint32, pdrInfo ebpf.PdrInfo) error {
	return nil
}

func (mapOps *MapOperationsMock) PutPdrDownlink(ipv4 net.IP, pdrInfo ebpf.PdrInfo) error {
	return nil
}

func (mapOps *MapOperationsMock) UpdatePdrUplink(teid uint32, pdrInfo ebpf.PdrInfo) error {
	return nil
}

func (mapOps *MapOperationsMock) UpdatePdrDownlink(ipv4 net.IP, pdrInfo ebpf.PdrInfo) error {
	return nil
}

func (mapOps *MapOperationsMock) DeletePdrUplink(teid uint32) error {
	return nil
}

func (mapOps *MapOperationsMock) DeletePdrDownlink(ipv4 net.IP) error {
	return nil
}

func (mapOps *MapOperationsMock) PutDownlinkPdrIp6(ipv6 net.IP, pdrInfo ebpf.PdrInfo) error {
	return nil
}

func (mapOps *MapOperationsMock) UpdateDownlinkPdrIp6(ipv6 net.IP, pdrInfo ebpf.PdrInfo) error {
	return nil
}

func (mapOps *MapOperationsMock) DeleteDownlinkPdrIp6(ipv6 net.IP) error {
	return nil
}

func (mapOps *MapOperationsMock) NewFar(farInfo ebpf.FarInfo) (uint32, error) {
	return 0, nil
}

func (mapOps *MapOperationsMock) UpdateFar(internalId uint32, farInfo ebpf.FarInfo) error {
	return nil
}

func (mapOps *MapOperationsMock) DeleteFar(internalId uint32) error {
	return nil
}

func (mapOps *MapOperationsMock) NewQer(qerInfo ebpf.QerInfo) (uint32, error) {
	return 0, nil
}

func (mapOps *MapOperationsMock) UpdateQer(internalId uint32, qerInfo ebpf.QerInfo) error {
	return nil
}

func (mapOps *MapOperationsMock) DeleteQer(internalId uint32) error {
	return nil
}

// func TestSessionOverwrite(t *testing.T) {
// 	mapOps := MapOperationsMock{}

// 	pfcpConn, err := CreatePfcpConnection("0.0.0.0:8805", nil, "0.0.0.0", RemoteIP, &mapOps, nil)
// 	if err != nil {
// 		t.Errorf("Error creating PFCP connection: %s", err)
// 	}

// 	pfcpConn.NodeAssociations[RemoteIP] = NewNodeAssociation("0.0.0.0", "0.0.0.0")

// 	// Create two send two Session Establishment Requests with downlink PDRs
// 	// and check that the first session is not overwritten
// 	seReq1 := message.NewSessionEstablishmentRequest(0, 0,
// 		1, 1, 0,
// 		ie.NewNodeID("", "", "test"),
// 		ie.NewFSEID(1, net.ParseIP(RemoteIP), nil),
// 		ie.NewCreatePDR(
// 			ie.NewPDRID(1),
// 			ie.NewPDI(
// 				ie.NewSourceInterface(ie.SrcInterfaceCore),
// 				// ie.NewFTEID(0, 0, ip1.IP, nil, 0),
// 				ie.NewUEIPAddress(2, UeAddress1, "", 0, 0),
// 			),
// 		),
// 	)

// 	seReq2 := message.NewSessionEstablishmentRequest(0, 0,
// 		2, 1, 0,
// 		ie.NewNodeID("", "", "test"),
// 		ie.NewFSEID(2, net.ParseIP(RemoteIP), nil),
// 		ie.NewCreatePDR(
// 			ie.NewPDRID(1),
// 			ie.NewPDI(
// 				ie.NewSourceInterface(ie.SrcInterfaceCore),
// 				// ie.NewFTEID(0, 0, ip2.IP, nil, 0),
// 				ie.NewUEIPAddress(2, UeAddress2, "", 0, 0),
// 			),
// 		),
// 	)

// 	// Send first request
// 	_, err = HandlePfcpSessionEstablishmentRequest(seReq1)
// 	if err != nil {
// 		t.Errorf("Error handling session establishment request: %s", err)
// 	}

// 	// Send second request
// 	_, err = HandlePfcpSessionEstablishmentRequest(seReq2)
// 	if err != nil {
// 		t.Errorf("Error handling session establishment request: %s", err)
// 	}

// 	// Check that session PDRs are correct
// 	if pfcpConn.NodeAssociations[RemoteIP].Sessions[2].PDRs[1].Ipv4.String() != UeAddress1 {
// 		t.Errorf("Session 1, got broken")
// 	}
// 	if pfcpConn.NodeAssociations[RemoteIP].Sessions[3].PDRs[1].Ipv4.String() != UeAddress2 {
// 		t.Errorf("Session 2, got broken")
// 	}

// 	// Send Session Modification Request, create FAR
// 	smReq := message.NewSessionModificationRequest(0, 0,
// 		2, 1, 0,
// 		ie.NewNodeID("", "", "test"),
// 		ie.NewFSEID(2, net.ParseIP(RemoteIP), nil),
// 		ie.NewCreateFAR(
// 			ie.NewFARID(1),
// 			ie.NewApplyAction(2),
// 			ie.NewForwardingParameters(
// 				ie.NewDestinationInterface(ie.DstInterfaceAccess),
// 				ie.NewNetworkInstance(""),
// 			),
// 		),
// 	)

// 	// Send modification request
// 	_, err = HandlePfcpSessionModificationRequest(pfcpConn, smReq, RemoteIP)
// 	if err != nil {
// 		t.Errorf("Error handling session modification request: %s", err)
// 	}

// 	// Check that session PDRs are correct
// 	if pfcpConn.NodeAssociations[RemoteIP].Sessions[2].PDRs[1].Ipv4.String() != UeAddress1 {
// 		t.Errorf("Session 1, got broken")
// 	}
// 	if pfcpConn.NodeAssociations[RemoteIP].Sessions[3].PDRs[1].Ipv4.String() != UeAddress2 {
// 		t.Errorf("Session 2, got broken")
// 	}
// }
