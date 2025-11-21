// SPDX-FileCopyrightText: 2025-present Ella Networks
// SPDX-License-Identifier: Apache-2.0

package pfcp_dispatcher

import (
	"context"

	"github.com/wmnsk/go-pfcp/message"
)

type UPF interface {
	HandlePfcpSessionEstablishmentRequest(context.Context, *message.SessionEstablishmentRequest) (*message.SessionEstablishmentResponse, error)
	HandlePfcpSessionDeletionRequest(context.Context, *message.SessionDeletionRequest) (*message.SessionDeletionResponse, error)
	HandlePfcpSessionModificationRequest(context.Context, *message.SessionModificationRequest) (*message.SessionModificationResponse, error)
}

type SMF interface {
	HandlePfcpSessionReportRequest(context.Context, *message.SessionReportRequest) (*message.SessionReportResponse, error)
}

type PfcpDispatcher struct {
	SMF SMF
	UPF UPF
}

var Dispatcher PfcpDispatcher

func NewPfcpDispatcher(smf SMF, upf UPF) PfcpDispatcher {
	return PfcpDispatcher{SMF: smf, UPF: upf}
}
