// Copyright 2019 free5GC.org
//
// SPDX-License-Identifier: Apache-2.0

package handler_test

import (
	"net"
	"testing"
	"time"

	"github.com/wmnsk/go-pfcp/ie"
	"github.com/wmnsk/go-pfcp/message"
	"github.com/yeastengine/ella/internal/smf/context"
	"github.com/yeastengine/ella/internal/smf/factory"
	"github.com/yeastengine/ella/internal/smf/pfcp/handler"
	pfcp_message "github.com/yeastengine/ella/internal/smf/pfcp/message"
	"github.com/yeastengine/ella/internal/smf/pfcp/udp"
)

type Flag uint8

// setBit sets the bit at the given position to the specified value (true or false)
// Positions go from 1 to 8
func (f *Flag) setBit(position uint8) {
	if position < 1 || position > 8 {
		return
	}
	*f |= 1 << (position - 1)
}

func boolPointer(b bool) *bool {
	return &b
}

func TestFindUEIPAddressNoAddressInCreatedPDR(t *testing.T) {
	sessionEstablishmentResponse := message.NewSessionEstablishmentResponse(
		0,
		0,
		0,
		0,
		0,
		ie.NewCreatedPDR(
			ie.NewPDRID(12345),
		),
	)

	createdPDRIEs := sessionEstablishmentResponse.CreatedPDR

	ipAddress := handler.FindUEIPAddress(createdPDRIEs)

	if ipAddress != nil {
		t.Errorf("Expected nil, got %v", ipAddress)
	}
}

func TestFindUEIPAddressNoUEIPAddressInCreatedPDR(t *testing.T) {
	ueIPAddressFlags := new(Flag)
	ueIPAddressFlags.setBit(2)
	sessionEstablishmentResponse := message.NewSessionEstablishmentResponse(
		0,
		0,
		0,
		0,
		0,
		ie.NewCreatedPDR(
			ie.NewPDRID(12345),
			ie.NewUEIPAddress(uint8(*ueIPAddressFlags), "1.2.3.4", "", 0, 0),
		),
	)

	createdPDRIEs := sessionEstablishmentResponse.CreatedPDR

	ipAddress := handler.FindUEIPAddress(createdPDRIEs)

	if !ipAddress.Equal(net.IPv4(1, 2, 3, 4)) {
		t.Errorf("Expected %v, got %v", "1.2.3.4", ipAddress)
	}
}

func TestHandlePfcpAssociationSetupResponse(t *testing.T) {
	configuration := &factory.Configuration{}
	factory.SmfConfig = factory.Config{
		Configuration: configuration,
	}
	upNodeID := context.NewNodeID("1.1.1.1")
	upf := context.NewUPF(upNodeID, nil)
	SnssaiInfos := make([]context.SnssaiUPFInfo, 0)
	snssaiInfo := context.SnssaiUPFInfo{
		DnnList: []context.DnnUPFInfoItem{
			{
				Dnn: "internet",
			},
		},
	}
	SnssaiInfos = append(SnssaiInfos, snssaiInfo)
	upf.SNssaiInfos = SnssaiInfos
	pfcp_message.InsertPfcpTxn(1, upNodeID)
	recoveryTimestamp := time.Now()
	msg := message.NewAssociationSetupResponse(
		1,
		ie.NewCause(ie.CauseRequestAccepted),
		ie.NewNodeID("1.1.1.1", "", ""),
		ie.NewUserPlaneIPResourceInformation(
			uint8(0x61),
			0,
			"1.2.3.4",
			"",
			".internet", // Note the additional character here. This is because the SD-Core UPF sends the network instance with 1 leading character
			ie.SrcInterfaceAccess,
		),
		ie.NewRecoveryTimeStamp(recoveryTimestamp),
	)

	remoteAddress := &net.UDPAddr{
		IP:   net.ParseIP("1.1.1.1"),
		Port: factory.DEFAULT_PFCP_PORT,
	}
	udpMessage := udp.Message{
		RemoteAddr:  remoteAddress,
		PfcpMessage: msg,
	}

	handler.HandlePfcpAssociationSetupResponse(&udpMessage)

	if upf.UPFStatus != context.AssociatedSetUpSuccess {
		t.Errorf("Expected UPFStatus %v, got %v", context.AssociatedSetUpSuccess, upf.UPFStatus)
	}
	if upf.RecoveryTimeStamp.RecoveryTimeStamp.Truncate(1*time.Second) != recoveryTimestamp.Truncate(1*time.Second) {
		t.Errorf("Expected RecoveryTimeStamp %v, got %v", recoveryTimestamp.Truncate(1*time.Second), upf.RecoveryTimeStamp.RecoveryTimeStamp.Truncate(1*time.Second))
	}
	if upf.UPIPInfo.Ipv4Address.String() != "1.2.3.4" {
		t.Errorf("Expected IP address %v, got %v", "1.2.3.4", upf.UPIPInfo.Ipv4Address.String())
	}
	if upf.UPIPInfo.SourceInterface != ie.SrcInterfaceAccess {
		t.Errorf("Expected Source Interface Access, got %v", upf.UPIPInfo.SourceInterface)
	}
	if string(upf.UPIPInfo.NetworkInstance) != "internet" {
		t.Errorf("Expected Network Instance %v, got %v", "internet", upf.UPIPInfo.NetworkInstance)
	}
}
