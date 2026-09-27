// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"

	"github.com/ellanetworks/core/etsi"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/tracing/attrs"
	"github.com/ellanetworks/core/nas/eps"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

type LPPHandler interface {
	ForwardLPP(ctx context.Context, supi etsi.SUPI, correlationID, lppData []byte) error
}

type LPPBuffered struct {
	CorrelationID []byte
	Payload       []byte
}

func (m *MME) AllocateLCSCorrelationID() []byte {
	id := make([]byte, 4)
	binary.BigEndian.PutUint32(id, m.lcsCorrelationSeq.Add(1))

	return id
}

func (m *MME) TransferLPPMsg(ctx context.Context, supi etsi.SUPI, correlationID, lppMsg []byte) error {
	ctx, span := Tracer.Start(ctx, "mme/transfer_lpp_message",
		trace.WithAttributes(attrs.SUPI(supi.String())),
	)
	defer span.End()

	ue, ok := m.LookupUeBySupi(supi)
	if !ok {
		return fmt.Errorf("UE not found: %s", supi)
	}

	if ue.EMMState() != EMMRegistered {
		return fmt.Errorf("UE is not in registered state")
	}

	if len(correlationID) == 0 {
		correlationID = m.AllocateLCSCorrelationID()
	}

	if ueConn := ue.Conn(); ueConn != nil {
		return sendLPP(ctx, ueConn, correlationID, lppMsg)
	}

	return m.pageForLPP(ctx, ue, correlationID, lppMsg)
}

func (m *MME) pageForLPP(ctx context.Context, ue *UeContext, correlationID, lppMsg []byte) error {
	buf := LPPBuffered{CorrelationID: bytes.Clone(correlationID), Payload: bytes.Clone(lppMsg)}

	err := m.pageToDeliver(ctx, ue, func() { ue.lppBuf.set(buf) })
	if errors.Is(err, errPagingSkipped) {
		if ueConn := ue.Conn(); ueConn != nil {
			return sendLPP(ctx, ueConn, correlationID, lppMsg)
		}

		if m.pagingInProgress(ue) {
			ue.lppBuf.set(buf)

			if ueConn := ue.Conn(); ueConn != nil {
				m.DeliverBufferedLPP(ctx, ue, ueConn)
			}

			logger.From(ctx, logger.MmeLog).Info("LPP message buffered on the paging procedure in progress",
				logger.SUPI(ue.Supi().String()))

			return nil
		}
	}

	if err != nil {
		return err
	}

	logger.From(ctx, logger.MmeLog).Info("LPP message buffered, paging ECM-IDLE UE",
		logger.SUPI(ue.Supi().String()),
		zap.Int("lpp_len", len(lppMsg)),
	)

	return nil
}

func (m *MME) CancelBufferedLPP(supi etsi.SUPI, correlationID []byte) {
	ue, ok := m.LookupUeBySupi(supi)
	if !ok {
		return
	}

	ue.ClearLPPBufferedIf(correlationID)
}

func (m *MME) DeliverBufferedLPP(ctx context.Context, ue *UeContext, ueConn *UeConn) {
	if ueConn == nil {
		return
	}

	buf := ue.PopLPPBuffered()
	if buf == nil {
		return
	}

	if err := sendLPP(ctx, ueConn, buf.CorrelationID, buf.Payload); err != nil {
		logger.From(ctx, logger.MmeLog).Error("failed to deliver buffered LPP", zap.Error(err))
		return
	}

	logger.From(ctx, logger.MmeLog).Info("delivered buffered LPP after paging")
}

func sendLPP(ctx context.Context, ueConn *UeConn, correlationID, lppMsg []byte) error {
	plain, err := (&eps.DownlinkGenericNASTransport{
		ContainerType:         eps.GenericMessageContainerTypeLPP,
		Container:             lppMsg,
		AdditionalInformation: correlationID,
	}).MarshalBinary()
	if err != nil {
		return fmt.Errorf("build Downlink Generic NAS Transport: %w", err)
	}

	if err := ueConn.SendProtectedNASTransport(ctx, plain, eps.SHTIntegrityProtectedCiphered); err != nil {
		return fmt.Errorf("send Downlink Generic NAS Transport: %w", err)
	}

	return nil
}

func (ue *UeContext) SetLPPBuffered(correlationID, payload []byte) {
	ue.lppBuf.set(LPPBuffered{
		CorrelationID: bytes.Clone(correlationID),
		Payload:       bytes.Clone(payload),
	})
}

func (ue *UeContext) PopLPPBuffered() *LPPBuffered {
	return ue.lppBuf.pop()
}

func (ue *UeContext) ClearLPPBufferedIf(correlationID []byte) {
	ue.lppBuf.clearIf(func(b *LPPBuffered) bool { return bytes.Equal(b.CorrelationID, correlationID) })
}
