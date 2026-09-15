// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-FileCopyrightText: 2022-present Intel Corporation
// SPDX-FileCopyrightText: 2021 Open Networking Foundation <info@opennetworking.org>
// Copyright 2019 free5GC.org
//
// Modified by Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"context"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/metrics"
	"github.com/ellanetworks/core/internal/sctp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

var tracer = otel.Tracer("ella-core/amf/ngap")

func Dispatch(ctx context.Context, amfInstance *amf.AMF, conn *sctp.SCTPConn, msg []byte) {
	remoteAddress := conn.RemoteAddr()
	localAddress := conn.LocalAddr()

	ctx, span := tracer.Start(ctx, "ngap/receive",
		trace.WithSpanKind(trace.SpanKindServer),
		trace.WithAttributes(
			attribute.Int("ngap.message_size", len(msg)),
			attribute.String("network.protocol.name", "ngap"),
			attribute.String("network.transport", "sctp"),
			attribute.String("network.peer.address", amf.AddrString(remoteAddress)),
			attribute.String("network.local.address", amf.AddrString(localAddress)),
		),
	)
	defer span.End()

	ran, ok := amfInstance.FindRadioByConn(conn)
	if !ok {
		var err error

		ran, err = amfInstance.NewRadio(conn)
		if err != nil {
			logger.From(ctx, logger.AmfLog).Error("Failed to add a new radio", zap.Error(err))
			return
		}

		ran.Log(ctx).Info("Radio connected", logger.RAT(metrics.RAT5G))
	}

	if len(msg) == 0 {
		amfInstance.DisconnectRadio(ctx, ran)

		return
	}

	ran.TouchLastSeen()

	// Every NGAP procedure this AMF supports is decoded by the in-house library.
	// A message it does not consume either failed to decode or names a procedure
	// the AMF does not implement, and route.go answers both per §10.3.
	route(ctx, amfInstance, ran, msg, span, remoteAddress, localAddress)
}
