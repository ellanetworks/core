// SPDX-FileCopyrightText: 2025-present Ella Networks
// SPDX-License-Identifier: Apache-2.0

package pfcp

import (
	"context"
	"fmt"

	"github.com/wmnsk/go-pfcp/message"
)

type SmfPfcpHandler struct{}

func (s SmfPfcpHandler) HandlePfcpSessionReportRequest(ctx context.Context, msg *message.SessionReportRequest) (*message.SessionReportResponse, error) {
	return HandlePfcpSessionReportRequest(ctx, msg)
}

func HandlePfcpSessionReportRequest(context.Context, *message.SessionReportRequest) (*message.SessionReportResponse, error) {
	return nil, fmt.Errorf("not implemented")
}
