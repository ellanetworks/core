// SPDX-License-Identifier: Apache-2.0
// Copyright 2024 Canonical Ltd.

package message_test

import (
	"net"
	"testing"
	"time"

	"github.com/ellanetworks/core/internal/smf/context"
	"github.com/ellanetworks/core/internal/smf/pfcp/message"
	"github.com/ellanetworks/core/internal/util/dnn"
	"github.com/wmnsk/go-pfcp/ie"
	pfcp_message "github.com/wmnsk/go-pfcp/message"
)

const cpNodeID = "1.2.3.4"

func outerHeaderRemovalSet(pdrIEs []*ie.IE) bool {
	for _, pdrIE := range pdrIEs {
		createPdr, err := pdrIE.CreatePDR()
		if err != nil {
			return false
		}

		for _, x := range createPdr {
			outerHeaderRemoval, err := x.OuterHeaderRemoval()
			if err == nil && outerHeaderRemoval != nil {
				return true
			}
		}
	}
	return false
}

func outerHeaderCreationSet(farIEs []*ie.IE, expectedIPv4Address string) bool {
	for _, farIE := range farIEs {
		createFar, err := farIE.UpdateFAR()
		if err != nil {
			continue
		}

		for _, x := range createFar {
			forwardingParamers, err := x.UpdateForwardingParameters()
			if err != nil {
				continue
			}

			for _, y := range forwardingParamers {
				outerHeaderCreation, err := y.OuterHeaderCreation()
				if err == nil && outerHeaderCreation != nil {
					return outerHeaderCreation.IPv4Address.String() == expectedIPv4Address
				}
			}
		}
	}
	return false
}

func TestBuildPfcpHeartbeatRequest(t *testing.T) {
	serverStartTime := time.Now()
	msg := message.BuildPfcpHeartbeatRequest(32, serverStartTime)

	if msg.MessageTypeName() != "Heartbeat Request" {
		t.Errorf("expected message type to be 'Heartbeat Request', got %v", msg.MessageTypeName())
	}

	buf := make([]byte, msg.MarshalLen())
	err := msg.MarshalTo(buf)
	if err != nil {
		t.Fatalf("error marshalling PFCP heartbeat request: %v", err)
	}

	req, err := pfcp_message.ParseHeartbeatRequest(buf)
	if err != nil {
		t.Fatalf("error parsing PFCP heartbeat request: %v", err)
	}

	seq := req.SequenceNumber
	if seq != 32 {
		t.Errorf("expected SequenceNumber to be 32, got %v", seq)
	}

	recoveryTimestamp, err := req.RecoveryTimeStamp.RecoveryTimeStamp()
	if err != nil {
		t.Fatalf("error getting RecoveryTimeStamp from PFCP heartbeat request: %v", err)
	}

	if recoveryTimestamp.Truncate(1*time.Second) != serverStartTime.Truncate(1*time.Second) {
		t.Errorf("expected RecoveryTimeStamp to be %v, got %v", serverStartTime, recoveryTimestamp)
	}
}

func TestBuildPfcpHeartbeatResponse(t *testing.T) {
	serverStartTime := time.Now()

	msg := message.BuildPfcpHeartbeatResponse(22, serverStartTime)

	if msg.MessageTypeName() != "Heartbeat Response" {
		t.Errorf("expected message type to be 'Heartbeat Response', got %v", msg.MessageTypeName())
	}

	buf := make([]byte, msg.MarshalLen())
	err := msg.MarshalTo(buf)
	if err != nil {
		t.Fatalf("error marshalling PFCP heartbeat response: %v", err)
	}

	resp, err := pfcp_message.ParseHeartbeatResponse(buf)
	if err != nil {
		t.Fatalf("error parsing PFCP heartbeat response: %v", err)
	}

	seq := resp.SequenceNumber
	if seq != 22 {
		t.Errorf("expected SequenceNumber to be 22, got %v", seq)
	}

	recoveryTimestamp, err := resp.RecoveryTimeStamp.RecoveryTimeStamp()
	if err != nil {
		t.Fatalf("error getting RecoveryTimeStamp from PFCP heartbeat request: %v", err)
	}

	if recoveryTimestamp.Truncate(1*time.Second) != serverStartTime.Truncate(1*time.Second) {
		t.Errorf("expected RecoveryTimeStamp to be %v, got %v", serverStartTime, recoveryTimestamp)
	}
}

func TestBuildPfcpAssociationSetupRequest(t *testing.T) {
	timestamp := time.Now()
	msg := message.BuildPfcpAssociationSetupRequest(42, timestamp, cpNodeID)

	if msg.MessageTypeName() != "Association Setup Request" {
		t.Errorf("expected message type to be 'Association Setup Request', got %v", msg.MessageTypeName())
	}

	buf := make([]byte, msg.MarshalLen())
	err := msg.MarshalTo(buf)
	if err != nil {
		t.Fatalf("error marshalling PFCP association setup request: %v", err)
	}

	req, err := pfcp_message.ParseAssociationSetupRequest(buf)
	if err != nil {
		t.Fatalf("error parsing PFCP association setup request: %v", err)
	}

	seq := req.SequenceNumber
	if seq != 42 {
		t.Errorf("expected SequenceNumber to be 42, got %v", seq)
	}

	recoveryTimestamp, err := req.RecoveryTimeStamp.RecoveryTimeStamp()
	if err != nil {
		t.Fatalf("error getting RecoveryTimeStamp from PFCP association setup request: %v", err)
	}

	if recoveryTimestamp.Truncate(1*time.Second) != timestamp.Truncate(1*time.Second) {
		t.Errorf("expected RecoveryTimeStamp to be %v, got %v", timestamp, recoveryTimestamp)
	}

	nodeID, err := req.NodeID.NodeID()
	if err != nil {
		t.Fatalf("error getting NodeID from PFCP association setup request: %v", err)
	}

	if nodeID != cpNodeID {
		t.Errorf("expected NodeID to be %v got %v", cpNodeID, nodeID)
	}
}

func TestBuildPfcpAssociationSetupResponse(t *testing.T) {
	timestamp := time.Now()
	msg := message.BuildPfcpAssociationSetupResponse(ie.CauseRequestAccepted, timestamp, cpNodeID)

	if msg.MessageTypeName() != "Association Setup Response" {
		t.Errorf("expected message type to be 'Association Setup Response', got %v", msg.MessageTypeName())
	}

	buf := make([]byte, msg.MarshalLen())
	err := msg.MarshalTo(buf)
	if err != nil {
		t.Fatalf("error marshalling PFCP association setup response: %v", err)
	}

	resp, err := pfcp_message.ParseAssociationSetupResponse(buf)
	if err != nil {
		t.Fatalf("error parsing PFCP association setup response: %v", err)
	}

	causeValue, err := resp.Cause.Cause()
	if err != nil {
		t.Fatalf("error getting Cause from PFCP association setup response: %v", err)
	}

	if causeValue != ie.CauseRequestAccepted {
		t.Errorf("expected SequenceNumber to be %v, got %v", ie.CauseRequestAccepted, causeValue)
	}

	recoveryTimestamp, err := resp.RecoveryTimeStamp.RecoveryTimeStamp()
	if err != nil {
		t.Fatalf("error getting RecoveryTimeStamp from PFCP association setup response: %v", err)
	}

	if recoveryTimestamp.Truncate(1*time.Second) != timestamp.Truncate(1*time.Second) {
		t.Errorf("expected RecoveryTimeStamp to be %v, got %v", timestamp, recoveryTimestamp)
	}

	nodeID, err := resp.NodeID.NodeID()
	if err != nil {
		t.Fatalf("error getting NodeID from PFCP association setup response: %v", err)
	}

	if nodeID != cpNodeID {
		t.Errorf("expected NodeID to be %v got %v", cpNodeID, nodeID)
	}
}

func TestBuildPfcpAssociationReleaseResponse(t *testing.T) {
	msg := message.BuildPfcpAssociationReleaseResponse(ie.CauseRequestAccepted, cpNodeID)

	if msg.MessageTypeName() != "Association Release Response" {
		t.Errorf("expected message type to be 'Association Release Response', got %v", msg.MessageTypeName())
	}

	buf := make([]byte, msg.MarshalLen())
	err := msg.MarshalTo(buf)
	if err != nil {
		t.Fatalf("error marshalling PFCP association release response: %v", err)
	}

	resp, err := pfcp_message.ParseAssociationReleaseResponse(buf)
	if err != nil {
		t.Fatalf("error parsing PFCP association release response: %v", err)
	}

	causeValue, err := resp.Cause.Cause()
	if err != nil {
		t.Fatalf("error getting Cause from PFCP association release response: %v", err)
	}

	if causeValue != ie.CauseRequestAccepted {
		t.Errorf("expected SequenceNumber to be %v, got %v", ie.CauseRequestAccepted, causeValue)
	}

	nodeID, err := resp.NodeID.NodeID()
	if err != nil {
		t.Fatalf("Error getting NodeID from PFCP association release response: %v", err)
	}

	if nodeID != cpNodeID {
		t.Errorf("expected NodeID to be %v got %v", cpNodeID, nodeID)
	}
}

func TestBuildPfcpSessionEstablishmentRequest(t *testing.T) {
	pdrList := []*context.PDR{
		{
			OuterHeaderRemoval: &context.OuterHeaderRemoval{},
			PDRID:              1,
			Precedence:         123,
			FAR:                &context.FAR{},
			PDI: context.PDI{
				LocalFTeid:      &context.FTEID{},
				UEIPAddress:     &context.UEIPAddress{},
				SDFFilter:       &context.SDFFilter{},
				ApplicationID:   "app",
				NetworkInstance: dnn.Dnn{},
				SourceInterface: context.SourceInterface{
					InterfaceValue: 0x11,
				},
			},
		},
	}
	farList := []*context.FAR{}
	qerList := []*context.QER{}
	msg, err := message.BuildPfcpSessionEstablishmentRequest(43, cpNodeID, net.ParseIP(cpNodeID), 1, pdrList, farList, qerList)
	if err != nil {
		t.Fatalf("error building PFCP session establishment request: %v", err)
	}

	if msg.MessageTypeName() != "Session Establishment Request" {
		t.Errorf("expected message type to be 'ban', got %v", msg.MessageTypeName())
	}

	buf := make([]byte, msg.MarshalLen())
	err = msg.MarshalTo(buf)
	if err != nil {
		t.Fatalf("error marshalling PFCP session establishment request: %v", err)
	}

	req, err := pfcp_message.ParseSessionEstablishmentRequest(buf)
	if err != nil {
		t.Fatalf("error parsing PFCP session establishment request: %v", err)
	}

	seq := req.SequenceNumber
	if seq != 43 {
		t.Errorf("expected SequenceNumber to be 43, got %v", seq)
	}

	nodeID, err := req.NodeID.NodeID()
	if err != nil {
		t.Fatalf("error getting NodeID from PFCP session establishment request: %v", err)
	}

	if nodeID != cpNodeID {
		t.Errorf("expected NodeID to be %v, got %v", cpNodeID, nodeID)
	}

	pdr := req.CreatePDR
	if pdr == nil {
		t.Fatalf("expected CreatedPDR to be non-nil")
	}

	if !outerHeaderRemovalSet(pdr) {
		t.Errorf("expected OuterHeaderRemoval to be set")
	}
}

func TestBuildPfcpSessionModificationRequest(t *testing.T) {
	pdrList := []*context.PDR{
		{
			OuterHeaderRemoval: &context.OuterHeaderRemoval{},
			PDRID:              1,
			Precedence:         123,
			FAR:                &context.FAR{},
			PDI: context.PDI{
				LocalFTeid:      &context.FTEID{},
				UEIPAddress:     &context.UEIPAddress{},
				SDFFilter:       &context.SDFFilter{},
				ApplicationID:   "app",
				NetworkInstance: dnn.Dnn{},
				SourceInterface: context.SourceInterface{
					InterfaceValue: 0x11,
				},
			},
		},
	}
	farList := []*context.FAR{
		{
			ForwardingParameters: &context.ForwardingParameters{
				OuterHeaderCreation: &context.OuterHeaderCreation{
					Ipv4Address:                    net.ParseIP("1.2.3.4"),
					Ipv6Address:                    net.ParseIP(""),
					Teid:                           1,
					PortNumber:                     1,
					OuterHeaderCreationDescription: 256,
				},
			},
			State:       context.RULE_UPDATE,
			FARID:       1,
			ApplyAction: context.ApplyAction{},
		},
	}
	qerList := []*context.QER{}

	msg, err := message.BuildPfcpSessionModificationRequest(64, 1, 2, net.ParseIP("2.3.4.5"), pdrList, farList, qerList)
	if err != nil {
		t.Fatalf("error building PFCP session modification request: %v", err)
	}

	if msg.MessageTypeName() != "Session Modification Request" {
		t.Errorf("expected message type to be 'ban', got %v", msg.MessageTypeName())
	}

	buf := make([]byte, msg.MarshalLen())
	err = msg.MarshalTo(buf)
	if err != nil {
		t.Fatalf("error marshalling PFCP session modification request: %v", err)
	}

	req, err := pfcp_message.ParseSessionModificationRequest(buf)
	if err != nil {
		t.Fatalf("error parsing PFCP session modification request: %v", err)
	}

	seq := req.SequenceNumber
	if seq != 64 {
		t.Errorf("expected SequenceNumber to be 64, got %v", seq)
	}

	updateFars := req.UpdateFAR
	if len(updateFars) == 0 {
		t.Fatalf("expected UpdateFAR to be non-nil")
	}

	if !outerHeaderCreationSet(updateFars, "1.2.3.4") {
		t.Errorf("expected OuterHeaderCreation to be set")
	}
}

func TestBuildPfcpSessionModificationRequestNoOuterHeader(t *testing.T) {
	pdrList := []*context.PDR{
		{
			OuterHeaderRemoval: &context.OuterHeaderRemoval{},
			PDRID:              1,
			Precedence:         123,
			FAR:                &context.FAR{},
			PDI: context.PDI{
				LocalFTeid:      &context.FTEID{},
				UEIPAddress:     &context.UEIPAddress{},
				SDFFilter:       &context.SDFFilter{},
				ApplicationID:   "app",
				NetworkInstance: dnn.Dnn{},
				SourceInterface: context.SourceInterface{
					InterfaceValue: 0x11,
				},
			},
		},
	}
	farList := []*context.FAR{
		{
			ForwardingParameters: &context.ForwardingParameters{},
			State:                context.RULE_UPDATE,
			FARID:                1,
			ApplyAction:          context.ApplyAction{},
		},
	}
	qerList := []*context.QER{}

	msg, err := message.BuildPfcpSessionModificationRequest(64, 1, 2, net.ParseIP("2.3.4.5"), pdrList, farList, qerList)
	if err != nil {
		t.Fatalf("error building PFCP session modification request: %v", err)
	}

	if msg.MessageTypeName() != "Session Modification Request" {
		t.Errorf("expected message type to be 'ban', got %v", msg.MessageTypeName())
	}

	buf := make([]byte, msg.MarshalLen())
	err = msg.MarshalTo(buf)
	if err != nil {
		t.Fatalf("error marshalling PFCP session modification request: %v", err)
	}

	req, err := pfcp_message.ParseSessionModificationRequest(buf)
	if err != nil {
		t.Fatalf("error parsing PFCP session modification request: %v", err)
	}

	seq := req.SequenceNumber
	if seq != 64 {
		t.Errorf("expected SequenceNumber to be 64, got %v", seq)
	}

	updateFars := req.UpdateFAR
	if len(updateFars) == 0 {
		t.Fatalf("expected UpdateFAR to be non-nil")
	}

	if outerHeaderCreationSet(updateFars, "1.2.3.4") {
		t.Errorf("expected OuterHeaderCreation to not be set")
	}
}

func TestBuildPfcpSessionDeletionRequest(t *testing.T) {
	msg := message.BuildPfcpSessionDeletionRequest(12, 2, 3, net.ParseIP("2.2.2.2"))

	if msg.MessageTypeName() != "Session Deletion Request" {
		t.Errorf("expected message type to be 'Session Deletion Request', got %v", msg.MessageTypeName())
	}

	buf := make([]byte, msg.MarshalLen())
	err := msg.MarshalTo(buf)
	if err != nil {
		t.Fatalf("error marshalling PFCP session deletion request: %v", err)
	}

	req, err := pfcp_message.ParseSessionDeletionRequest(buf)
	if err != nil {
		t.Fatalf("error parsing PFCP session deletion request: %v", err)
	}

	seq := req.SequenceNumber
	if seq != 12 {
		t.Errorf("expected SequenceNumber to be 12, got %v", seq)
	}
}

func TestBuildPfcpSessionReportResponse(t *testing.T) {
	msg := message.BuildPfcpSessionReportResponse(ie.CauseRequestAccepted, true, 1, 22)

	if msg.MessageTypeName() != "Session Report Response" {
		t.Errorf("expected message type to be 'Session Report Response', got %v", msg.MessageTypeName())
	}

	buf := make([]byte, msg.MarshalLen())
	err := msg.MarshalTo(buf)
	if err != nil {
		t.Fatalf("error marshalling PFCP session report response: %v", err)
	}

	resp, err := pfcp_message.ParseSessionReportResponse(buf)
	if err != nil {
		t.Fatalf("error parsing PFCP session report response: %v", err)
	}

	causeValue, err := resp.Cause.Cause()
	if err != nil {
		t.Fatalf("error getting Cause from PFCP session report response: %v", err)
	}

	if causeValue != ie.CauseRequestAccepted {
		t.Errorf("expected SequenceNumber to be %v, got %v", ie.CauseRequestAccepted, causeValue)
	}

	flags, err := resp.PFCPSRRspFlags.PFCPSRRspFlags()
	if err != nil {
		t.Fatalf("error getting PFCPSRRspFlags from PFCP session report response: %v", err)
	}

	if flags != 1 {
		t.Errorf("expected PFCPSRRspFlags to be 1, got %v", flags)
	}
}
