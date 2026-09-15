// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package dbwriter

import (
	"context"
	"sync"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

var tracer = otel.Tracer("ella-core/dbwriter")

type queuedEvent struct {
	event *RadioEvent
	link  trace.SpanContext
}

// BufferedDBWriter wraps a DBWriter and performs InsertRadioEvent calls
// asynchronously via a buffered channel. This prevents synchronous SQLite
// writes from blocking the NGAP processing path.
//
// InsertAuditLog is passed through synchronously since audit log inserts
// are infrequent (API mutations only) and not on the NGAP hot path.
type BufferedDBWriter struct {
	delegate DBWriter
	logger   *zap.Logger
	eventCh  chan queuedEvent
	wg       sync.WaitGroup
}

// NewBufferedDBWriter creates a BufferedDBWriter that queues radio events
// in a channel of the given size and writes them in a background goroutine.
func NewBufferedDBWriter(delegate DBWriter, bufferSize int, logger *zap.Logger) *BufferedDBWriter {
	b := &BufferedDBWriter{
		delegate: delegate,
		logger:   logger,
		eventCh:  make(chan queuedEvent, bufferSize),
	}
	b.wg.Add(1)

	go b.drainLoop()

	return b
}

// InsertRadioEvent enqueues the event for asynchronous insertion.
// If the buffer is full the event is dropped and a warning is logged.
func (b *BufferedDBWriter) InsertRadioEvent(ctx context.Context, radioEvent *RadioEvent) error {
	queued := queuedEvent{
		event: radioEvent,
		link:  trace.SpanContextFromContext(ctx),
	}

	select {
	case b.eventCh <- queued:
	default:
		b.logger.Warn("radio event buffer full, dropping event",
			zap.String("message_type", radioEvent.MessageType),
		)
	}

	return nil
}

// InsertAuditLog is forwarded synchronously to the underlying writer.
func (b *BufferedDBWriter) InsertAuditLog(ctx context.Context, auditLog *AuditLog) error {
	return b.delegate.InsertAuditLog(ctx, auditLog)
}

// InsertFlowReports is forwarded synchronously to the underlying writer.
func (b *BufferedDBWriter) InsertFlowReports(ctx context.Context, flowReports []*FlowReport) error {
	return b.delegate.InsertFlowReports(ctx, flowReports)
}

// Stop closes the event channel and blocks until all queued events have
// been written or the context expires. Call this during graceful shutdown
// before closing the DB.
func (b *BufferedDBWriter) Stop(ctx context.Context) {
	close(b.eventCh)

	done := make(chan struct{})

	go func() {
		b.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-ctx.Done():
		b.logger.Warn("buffered writer drain timed out, some events may be lost")
	}
}

// drainLoop reads events from the channel and writes them to the database.
func (b *BufferedDBWriter) drainLoop() {
	defer b.wg.Done()

	for queued := range b.eventCh {
		b.write(queued)
	}
}

func (b *BufferedDBWriter) write(queued queuedEvent) {
	ctx := context.Background()
	span := trace.SpanFromContext(ctx)

	if queued.link.IsValid() {
		ctx, span = tracer.Start(ctx, "dbwriter/insert_radio_event",
			trace.WithSpanKind(trace.SpanKindInternal),
			trace.WithLinks(trace.Link{SpanContext: queued.link}),
		)
		defer span.End()
	}

	err := b.delegate.InsertRadioEvent(ctx, queued.event)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to insert radio event")

		b.logger.Warn("failed to insert buffered radio event",
			zap.String("message_type", queued.event.MessageType),
			zap.Error(err),
		)
	}
}
