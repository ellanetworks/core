package dbwriter

import (
	"context"
	"sync"

	"go.uber.org/zap"
)

// BufferedDBWriter wraps a DBWriter and performs InsertRadioEvent calls
// asynchronously via a buffered channel. This prevents synchronous SQLite
// writes from blocking the NGAP processing path.
//
// InsertAuditLog is passed through synchronously since audit log inserts
// are infrequent (API mutations only) and not on the NGAP hot path.
type BufferedDBWriter struct {
	delegate DBWriter
	logger   *zap.Logger
	eventCh  chan *RadioEvent
	wg       sync.WaitGroup
}

// NewBufferedDBWriter creates a BufferedDBWriter that queues radio events
// in a channel of the given size and writes them in a background goroutine.
func NewBufferedDBWriter(delegate DBWriter, bufferSize int, logger *zap.Logger) *BufferedDBWriter {
	b := &BufferedDBWriter{
		delegate: delegate,
		logger:   logger,
		eventCh:  make(chan *RadioEvent, bufferSize),
	}
	b.wg.Add(1)

	go b.drainLoop()

	return b
}

// InsertRadioEvent enqueues the event for asynchronous insertion.
// If the buffer is full the event is dropped and a warning is logged.
func (b *BufferedDBWriter) InsertRadioEvent(_ context.Context, radioEvent *RadioEvent) error {
	select {
	case b.eventCh <- radioEvent:
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

// Stop closes the event channel and blocks until all queued events have
// been written. Call this during graceful shutdown before closing the DB.
func (b *BufferedDBWriter) Stop() {
	close(b.eventCh)
	b.wg.Wait()
}

// drainLoop reads events from the channel and writes them to the database.
func (b *BufferedDBWriter) drainLoop() {
	defer b.wg.Done()

	for event := range b.eventCh {
		err := b.delegate.InsertRadioEvent(context.Background(), event)
		if err != nil {
			b.logger.Warn("failed to insert buffered radio event",
				zap.String("message_type", event.MessageType),
				zap.Error(err),
			)
		}
	}
}
