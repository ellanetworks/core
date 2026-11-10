// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"context"

	"github.com/ellanetworks/core/internal/mme"
	"github.com/ellanetworks/core/s1ap"
)

func handleTraceFailureIndication(ctx context.Context, m *mme.MME, radio *mme.Radio, value []byte) {
	msg, err := s1ap.ParseTraceFailureIndication(value)
	if err != nil {
		handleParseError(ctx, m, radio.Conn, s1ap.ProcTraceFailureIndication, s1ap.TriggeringInitiatingMessage, err)
		return
	}

	_, ueConn, ok := resolveUE(ctx, m, radio.Conn, msg.MMEUES1APID, msg.ENBUES1APID)
	if !ok {
		return
	}

	var (
		id    s1ap.EUTRANTraceID
		cause s1ap.Cause
	)

	if msg.EUTRANTraceID != nil {
		id = *msg.EUTRANTraceID
	}

	if msg.Cause != nil {
		cause = *msg.Cause
	}

	ueConn.TraceFailed(ctx, id, cause)
}
