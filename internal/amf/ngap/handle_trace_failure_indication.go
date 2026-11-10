// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"context"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/ngap"
)

func HandleTraceFailureIndication(ctx context.Context, amfInstance *amf.AMF, ran *amf.Radio, msg *ngap.TraceFailureIndication) {
	ueConn, ok := resolveUE(ctx, amfInstance, ran, msg.AMFUENGAPID, msg.RANUENGAPID)
	if !ok {
		return
	}

	ueConn.TouchLastSeen()

	var (
		id    ngap.NGRANTraceID
		cause ngap.Cause
	)

	if msg.NGRANTraceID != nil {
		id = *msg.NGRANTraceID
	}

	if msg.Cause != nil {
		cause = *msg.Cause
	}

	ueConn.TraceFailed(ctx, id, cause)
}
