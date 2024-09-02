// Copyright 2024 Canonical Ltd.
//
// SPDX-License-Identifier: Apache-2.0

package udp

import "github.com/wmnsk/go-pfcp/message"

func IsRequest(msg message.Message) bool {
	switch msg.MessageType() {
	case message.MsgTypeHeartbeatRequest,
		message.MsgTypePFDManagementRequest,
		message.MsgTypeAssociationSetupRequest,
		message.MsgTypeAssociationUpdateRequest,
		message.MsgTypeAssociationReleaseRequest,
		message.MsgTypeNodeReportRequest,
		message.MsgTypeSessionSetDeletionRequest,
		message.MsgTypeSessionEstablishmentRequest,
		message.MsgTypeSessionModificationRequest,
		message.MsgTypeSessionDeletionRequest,
		message.MsgTypeSessionReportRequest:
		return true
	default:
		return false
	}
}

func IsResponse(msg message.Message) bool {
	switch msg.MessageType() {
	case message.MsgTypeHeartbeatResponse,
		message.MsgTypePFDManagementResponse,
		message.MsgTypeAssociationSetupResponse,
		message.MsgTypeAssociationUpdateResponse,
		message.MsgTypeAssociationReleaseResponse,
		message.MsgTypeNodeReportResponse,
		message.MsgTypeSessionSetDeletionResponse,
		message.MsgTypeSessionEstablishmentResponse,
		message.MsgTypeSessionModificationResponse,
		message.MsgTypeSessionDeletionResponse,
		message.MsgTypeSessionReportResponse:
		return true
	default:
		return false
	}
}
