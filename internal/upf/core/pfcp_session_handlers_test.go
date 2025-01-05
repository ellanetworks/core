// Copyright 2024 Ella Networks
package core

import (
	"net"
	"testing"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/upf/config"
	"github.com/ellanetworks/core/internal/upf/core/service"

	"github.com/wmnsk/go-pfcp/ie"
	"github.com/wmnsk/go-pfcp/message"
)

func PreparePfcpConnection(t *testing.T) (PfcpConnection, string) {
	mapOps := MapOperationsMock{}

	pfcpHandlers := PfcpHandlerMap{
		message.MsgTypeSessionEstablishmentRequest: HandlePfcpSessionEstablishmentRequest,
		message.MsgTypeSessionDeletionRequest:      HandlePfcpSessionDeletionRequest,
		message.MsgTypeSessionModificationRequest:  HandlePfcpSessionModificationRequest,
	}

	smfIP := "127.0.0.1"
	pfcpConn := PfcpConnection{
		NodeAssociations: make(map[string]*NodeAssociation),
		nodeId:           "NodeId",
		mapOperations:    &mapOps,
		pfcpHandlerMap:   pfcpHandlers,
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
	_, err = HandlePfcpSessionEstablishmentRequest(pfcpConn, seReqPre1, smfIP)
	if err != nil {
		t.Errorf("Error handling session establishment request: %s", err)
	}

	_, err = HandlePfcpSessionEstablishmentRequest(pfcpConn, seReqPre2, smfIP)
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

func TestSdfFilterStoreValid(t *testing.T) {
	pfcpConn, smfIP := PreparePfcpConnection(t)
	SendDefaulMappingPdrs(t, &pfcpConn, smfIP)

	if len(pfcpConn.NodeAssociations[smfIP].Sessions[2].PDRs) != 1 {
		t.Errorf("Session 1, should have already stored 1 PDR")
	}

	if len(pfcpConn.NodeAssociations[smfIP].Sessions[3].PDRs) != 1 {
		t.Errorf("Session 2, should have already stored 1 PDR")
	}

	ip1, _ := net.ResolveIPAddr("ip", UeAddress1)
	ip2, _ := net.ResolveIPAddr("ip", UeAddress2)

	fd := SdfFilterTestStruct{
		FlowDescription: "permit out ip from 10.62.0.1 to 8.8.8.8/32", Protocol: 1,
		SrcType: 1, SrcAddress: "10.62.0.1", SrcMask: "ffffffff", SrcPortLower: 0, SrcPortUpper: 65535,
		DstType: 1, DstAddress: "8.8.8.8", DstMask: "ffffffff", DstPortLower: 0, DstPortUpper: 65535,
	}

	// Requests for additional mapping (with SDF filter)

	// Request with UEIP Address
	seReq1 := message.NewSessionModificationRequest(0, 0,
		2, 1, 0,
		ie.NewNodeID("", "", "test"),
		ie.NewFSEID(1, net.ParseIP(smfIP), nil), // Why do we need FSEID?
		ie.NewCreatePDR(
			ie.NewPDRID(2),
			ie.NewPDI(
				ie.NewSourceInterface(ie.SrcInterfaceCore),
				// ie.NewFTEID(0, 0, ip1.IP, nil, 0),
				ie.NewUEIPAddress(2, ip1.IP.String(), "", 0, 0),
				ie.NewSDFFilter(fd.FlowDescription, "", "", "", 0),
			),
		),
	)

	// Request with TEID
	seReq2 := message.NewSessionModificationRequest(0, 0,
		3, 1, 0,
		ie.NewNodeID("", "", "test"),
		ie.NewFSEID(2, net.ParseIP(smfIP), nil),
		ie.NewCreatePDR(
			ie.NewPDRID(2),
			ie.NewPDI(
				ie.NewSourceInterface(ie.SrcInterfaceCore),
				ie.NewFTEID(0, 0, ip2.IP, nil, 0),
				// ie.NewUEIPAddress(2, ip2.IP.String(), "", 0, 0),
				ie.NewSDFFilter(fd.FlowDescription, "", "", "", 0),
			),
		),
	)

	var err error
	_, err = HandlePfcpSessionModificationRequest(&pfcpConn, seReq1, smfIP)
	if err != nil {
		t.Errorf("Error handling session establishment request: %s", err)
	}

	_, err = HandlePfcpSessionModificationRequest(&pfcpConn, seReq2, smfIP)
	if err != nil {
		t.Errorf("Error handling session establishment request: %s", err)
	}

	// Check that session PDRs are correct
	if pfcpConn.NodeAssociations[smfIP].Sessions[2].PDRs[2].Ipv4.String() != UeAddress1 {
		t.Errorf("Session 1, got broken")
	}

	if pfcpConn.NodeAssociations[smfIP].Sessions[3].PDRs[2].Teid != 0 {
		t.Errorf("Session 2, got broken")
	}

	// Check that SDF filter is stored inside session
	pdrInfo := pfcpConn.NodeAssociations[smfIP].Sessions[2].PDRs[2].PdrInfo
	err = CheckSdfFilterEquality(pdrInfo.SdfFilter, fd)
	if err != nil {
		t.Error(err.Error())
	}

	pdrInfo = pfcpConn.NodeAssociations[smfIP].Sessions[3].PDRs[2].PdrInfo
	err = CheckSdfFilterEquality(pdrInfo.SdfFilter, fd)
	if err != nil {
		t.Error(err.Error())
	}
}

func TestSdfFilterStoreInvalid(t *testing.T) {
	pfcpConn, smfIP := PreparePfcpConnection(t)
	SendDefaulMappingPdrs(t, &pfcpConn, smfIP)

	if len(pfcpConn.NodeAssociations[smfIP].Sessions[2].PDRs) != 1 {
		t.Errorf("Session 1, should have already stored 1 PDR")
	}

	ip1, _ := net.ResolveIPAddr("ip", UeAddress1)

	// Request with bad/unsuported SDF
	seReq1 := message.NewSessionModificationRequest(0, 0,
		2, 1, 0,
		ie.NewNodeID("", "", "test"),
		ie.NewFSEID(1, net.ParseIP(smfIP), nil),
		ie.NewCreatePDR(
			ie.NewPDRID(1),
			ie.NewPDI(
				ie.NewSourceInterface(ie.SrcInterfaceCore),
				ie.NewFTEID(0, 0, ip1.IP, nil, 0),
				ie.NewSDFFilter("deny out ip from 10.62.0.1 to 8.8.8.8/32", "", "", "", 0),
			),
		),
	)

	var err error
	_, err = HandlePfcpSessionModificationRequest(&pfcpConn, seReq1, smfIP)
	if err != nil {
		t.Errorf("No error should appear while handling session establishment request. PDR with bad SDF should be skipped?")
	}

	// Check that session PDR wasn't stored? Now it is, just without SDF.
	if pfcpConn.NodeAssociations[smfIP].Sessions[2].PDRs[2].PdrInfo.SdfFilter != nil {
		t.Errorf("Bad SDF shouldn't be stored")
	}
}

func TestTEIDAllocationInSessionEstablishmentResponse(t *testing.T) {
	pfcpConn, smfIP := PreparePfcpConnection(t)

	resourceManager, err := service.NewResourceManager(65536)
	if err != nil {
		logger.UpfLog.Errorf("failed to create ResourceManager. err: %v", err)
	}
	pfcpConn.ResourceManager = resourceManager

	fteid1 := ie.NewFTEID(0x04, 0, net.ParseIP("127.0.0.1"), nil, 1) // 0x04 - CH true
	createPDR1 := ie.NewCreatePDR(
		ie.NewPDRID(1),
		ie.NewPDI(
			ie.NewSourceInterface(ie.SrcInterfaceCore),
			fteid1,
		),
	)

	fteid2 := ie.NewFTEID(0x04, 0, net.ParseIP("127.0.0.2"), nil, 1)
	createPDR2 := ie.NewCreatePDR(
		ie.NewPDRID(2),
		ie.NewPDI(
			ie.NewSourceInterface(ie.SrcInterfaceCore),
			fteid2,
		),
	)

	fteid3 := ie.NewFTEID(0, 0, net.ParseIP("127.0.0.2"), nil, 1)
	createPDR3 := ie.NewCreatePDR(
		ie.NewPDRID(2),
		ie.NewPDI(
			ie.NewSourceInterface(ie.SrcInterfaceCore),
			fteid3,
		),
	)

	// Creating a Session Establishment Request
	seReq := message.NewSessionEstablishmentRequest(0, 0,
		2, 1, 0,
		ie.NewNodeID("", "", "test"),
		ie.NewFSEID(1, net.ParseIP(smfIP), nil),
		createPDR1,
		createPDR2,
		createPDR3,
	)

	// Processing Session Establishment Request
	response, err := HandlePfcpSessionEstablishmentRequest(&pfcpConn, seReq, smfIP)
	if err != nil {
		t.Errorf("Error handling Session Establishment Request: %s", err)
	}

	// Checking if expected TEIDs are allocated in Session Establishment Response
	seRes, ok := response.(*message.SessionEstablishmentResponse)
	if !ok {
		t.Error("Unexpected response type")
	}

	// Checking TEID for each PDR
	logger.UpfLog.Infof("seRes.CreatedPDR len: %d", len(seRes.CreatedPDR))
	if len(seRes.CreatedPDR) != 2 {
		t.Errorf("Unexpected count TEIDs: got %d, expected %d", len(seRes.CreatedPDR), 2)
	}

	for _, pdr := range seRes.CreatedPDR {
		fteid, err := pdr.FindByType(ie.FTEID)
		if err != nil {
			logger.UpfLog.Fatalf("FindByType err: %v", err)
		}

		teid, err := fteid.FTEID()
		if err != nil {
			logger.UpfLog.Fatalf("FTEID err: %v", err)
		}

		if teid.TEID != 1 && teid.TEID != 2 {
			t.Errorf("Unexpected TEID for PDR ID 2: got %d, expected %d or %d", teid.TEID, 1, 2)
		}

		if !teid.HasIPv4() {
			t.Error("HasIPv4 flag is not enabled in TEID")
		}

		if teid.IPv4Address == nil {
			t.Error("TEID has no ip")
		}
	}
}

func TestIPAllocationInSessionEstablishmentResponse(t *testing.T) {
	if config.Conf.FeatureUEIP {
		pfcpConn, smfIP := PreparePfcpConnection(t)

		resourceManager, err := service.NewResourceManager(65536)
		if err != nil {
			logger.UpfLog.Errorf("failed to create ResourceManager. err: %v", err)
		}
		pfcpConn.ResourceManager = resourceManager

		ueip1 := ie.NewUEIPAddress(16, "", "", 0, 0)
		createPDR1 := ie.NewCreatePDR(
			ie.NewPDRID(1),
			ie.NewPDI(
				ie.NewSourceInterface(ie.SrcInterfaceCore),
				ueip1,
			),
		)

		// Creating a Session Establishment Request
		seReq := message.NewSessionEstablishmentRequest(0, 0,
			2, 1, 0,
			ie.NewNodeID("", "", "test"),
			ie.NewFSEID(1, net.ParseIP(smfIP), nil),
			createPDR1,
		)

		// Processing Session Establishment Request
		response, err := HandlePfcpSessionEstablishmentRequest(&pfcpConn, seReq, smfIP)
		if err != nil {
			t.Errorf("Error handling Session Establishment Request: %s", err)
		}

		// Checking if expected IPs are allocated in Session Establishment Response
		seRes, ok := response.(*message.SessionEstablishmentResponse)
		if !ok {
			t.Error("Unexpected response type")
		}

		// Checking UEIP for each PDR
		logger.UpfLog.Infof("seRes.CreatedPDR len: %d", len(seRes.CreatedPDR))
		if len(seRes.CreatedPDR) != 1 {
			t.Errorf("Unexpected count PRD's: got %d, expected %d", len(seRes.CreatedPDR), 1)
		}

		for _, pdr := range seRes.CreatedPDR {
			ueipType, err := pdr.FindByType(ie.UEIPAddress)
			if err != nil {
				t.Errorf("FindByType err: %v", err)
			}

			ueip, err := ueipType.UEIPAddress()
			if err != nil {
				t.Errorf("UEIPAddress err: %v", err)
			}

			if ueip.IPv4Address == nil {
				logger.UpfLog.Infof("IPv4Address is nil")
			} else {
				if ueip.IPv4Address.String() == "10.61.0.0" {
					logger.UpfLog.Infof("PASSED. IPv4: %s", ueip.IPv4Address.String())
				} else {
					t.Errorf("Unexpected IPv4, got %s, expected %s", ueip.IPv4Address.String(), "10.61.0.0")
				}
			}
		}
	}
}
