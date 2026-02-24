package dbwriter_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/ellanetworks/core/internal/dbwriter"
	"go.uber.org/zap"
)

type fakeDBWriter struct {
	mu          sync.Mutex
	radioEvents []*dbwriter.RadioEvent
	auditLogs   []*dbwriter.AuditLog
	flowReports []*dbwriter.FlowReport
	insertErr   error
	insertDelay time.Duration
}

func (f *fakeDBWriter) InsertRadioEvent(_ context.Context, event *dbwriter.RadioEvent) error {
	if f.insertDelay > 0 {
		time.Sleep(f.insertDelay)
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	if f.insertErr != nil {
		return f.insertErr
	}

	f.radioEvents = append(f.radioEvents, event)

	return nil
}

func (f *fakeDBWriter) InsertAuditLog(_ context.Context, log *dbwriter.AuditLog) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.auditLogs = append(f.auditLogs, log)

	return nil
}

func (f *fakeDBWriter) InsertFlowReport(_ context.Context, flow *dbwriter.FlowReport) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.flowReports = append(f.flowReports, flow)

	return nil
}

func (f *fakeDBWriter) radioEventCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()

	return len(f.radioEvents)
}

func TestBufferedDBWriter_EventsAreWrittenAsynchronously(t *testing.T) {
	fake := &fakeDBWriter{}
	buf := dbwriter.NewBufferedDBWriter(fake, 100, zap.NewNop())

	for i := 0; i < 10; i++ {
		err := buf.InsertRadioEvent(context.Background(), &dbwriter.RadioEvent{
			MessageType: "InitialUEMessage",
			Protocol:    "NGAP",
		})
		if err != nil {
			t.Fatalf("InsertRadioEvent returned error: %v", err)
		}
	}

	buf.Stop()

	if got := fake.radioEventCount(); got != 10 {
		t.Fatalf("expected 10 events written, got %d", got)
	}
}

func TestBufferedDBWriter_StopDrainsRemainingEvents(t *testing.T) {
	fake := &fakeDBWriter{insertDelay: 1 * time.Millisecond}
	buf := dbwriter.NewBufferedDBWriter(fake, 100, zap.NewNop())

	for i := 0; i < 50; i++ {
		_ = buf.InsertRadioEvent(context.Background(), &dbwriter.RadioEvent{
			MessageType: "UplinkNASTransport",
		})
	}

	buf.Stop()

	if got := fake.radioEventCount(); got != 50 {
		t.Fatalf("expected all 50 events flushed on Stop, got %d", got)
	}
}

func TestBufferedDBWriter_DropsWhenBufferFull(t *testing.T) {
	fake := &fakeDBWriter{insertDelay: 50 * time.Millisecond}
	buf := dbwriter.NewBufferedDBWriter(fake, 5, zap.NewNop())

	for i := 0; i < 20; i++ {
		_ = buf.InsertRadioEvent(context.Background(), &dbwriter.RadioEvent{
			MessageType: "UEContextReleaseRequest",
		})
	}

	buf.Stop()

	got := fake.radioEventCount()
	if got >= 20 {
		t.Fatalf("expected some events to be dropped, but all %d were written", got)
	}

	if got == 0 {
		t.Fatal("expected at least some events to be written")
	}
}

func TestBufferedDBWriter_InsertErrorDoesNotBlock(t *testing.T) {
	fake := &fakeDBWriter{insertErr: errors.New("disk full")}
	buf := dbwriter.NewBufferedDBWriter(fake, 10, zap.NewNop())

	_ = buf.InsertRadioEvent(context.Background(), &dbwriter.RadioEvent{
		MessageType: "NGSetupRequest",
	})

	buf.Stop()
}

func TestBufferedDBWriter_AuditLogIsSynchronous(t *testing.T) {
	fake := &fakeDBWriter{}
	buf := dbwriter.NewBufferedDBWriter(fake, 10, zap.NewNop())

	err := buf.InsertAuditLog(context.Background(), &dbwriter.AuditLog{
		Action: "create_subscriber",
		Actor:  "admin",
	})
	if err != nil {
		t.Fatalf("InsertAuditLog returned error: %v", err)
	}

	if len(fake.auditLogs) != 1 {
		t.Fatalf("expected 1 audit log written synchronously, got %d", len(fake.auditLogs))
	}

	buf.Stop()
}

func TestBufferedDBWriter_ReturnsNilErrorOnInsert(t *testing.T) {
	fake := &fakeDBWriter{}

	buf := dbwriter.NewBufferedDBWriter(fake, 10, zap.NewNop())
	defer buf.Stop()

	err := buf.InsertRadioEvent(context.Background(), &dbwriter.RadioEvent{
		MessageType: "InitialUEMessage",
	})
	if err != nil {
		t.Fatalf("expected nil error from buffered InsertRadioEvent, got: %v", err)
	}
}
