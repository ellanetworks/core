// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

//go:build linux && !386

package gnb_test

import (
	"context"
	"net"
	"net/netip"
	"os"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/ellanetworks/core/internal/sctp"
	"github.com/ellanetworks/core/internal/tester/gnb"
	"github.com/ellanetworks/core/internal/tester/logger"
	"github.com/ellanetworks/core/ngap"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func TestMain(m *testing.M) {
	logger.Init(zapcore.ErrorLevel)

	os.Exit(m.Run())
}

// skipIfNoSCTP skips the test when the SCTP kernel module is not loaded.
func skipIfNoSCTP(t *testing.T) {
	t.Helper()

	fd, err := syscall.Socket(syscall.AF_INET, syscall.SOCK_STREAM, syscall.IPPROTO_SCTP)
	if err != nil {
		t.Skipf("SCTP not available: %v", err)
	}

	if err := syscall.Close(fd); err != nil {
		t.Fatalf("close probe socket: %v", err)
	}
}

// recordingDownlinkSender records the NAS PDUs the gNB delivers to one UE.
// When hold is non-nil the first delivery blocks on it after recording, so a
// test can observe whether a second delivery starts while the first is still
// running. The mutex is released before blocking: a recorder that held it
// would serialize the deliveries itself and hide the concurrency the test is
// looking for.
type recordingDownlinkSender struct {
	mu       sync.Mutex
	payloads [][]byte

	started  atomic.Int32
	finished atomic.Int32
	hold     chan struct{}
}

func (r *recordingDownlinkSender) SendDownlinkNAS(nasPDU []byte, _, _ int64) error {
	nth := r.started.Add(1)

	r.mu.Lock()
	r.payloads = append(r.payloads, append([]byte(nil), nasPDU...))
	r.mu.Unlock()

	if nth == 1 && r.hold != nil {
		<-r.hold
	}

	r.finished.Add(1)

	return nil
}

func (r *recordingDownlinkSender) RRCRelease() {}

func (r *recordingDownlinkSender) snapshot() [][]byte {
	r.mu.Lock()
	defer r.mu.Unlock()

	return append([][]byte(nil), r.payloads...)
}

// amfUEID is the AMF UE NGAP ID every frame these tests build carries. The gNB
// keys its queues on the RAN UE NGAP ID, so the AMF side never has to vary.
const amfUEID = uint32(1)

// marshalDownlinkNASTransport builds a DownlinkNASTransport NGAP PDU.
func marshalDownlinkNASTransport(t *testing.T, ranUEID uint64, nas []byte) []byte {
	t.Helper()

	pdu, err := (&ngap.DownlinkNASTransport{
		AMFUENGAPID: ngap.AMFUENGAPID(amfUEID),
		RANUENGAPID: ngap.RANUENGAPID(ranUEID),
		NASPDU:      ngap.NASPDU(nas),
	}).Marshal()
	if err != nil {
		t.Fatalf("could not marshal DownlinkNASTransport: %v", err)
	}

	return pdu
}

// marshalNGSetupRequest builds an NG Setup Request NGAP PDU.
func marshalNGSetupRequest(t *testing.T) []byte {
	t.Helper()

	opts := &gnb.NGSetupRequestOpts{
		GnbID: "000102",
		Mcc:   "001",
		Mnc:   "01",
		Sst:   1,
		Tac:   "000001",
		Name:  "test",
	}

	pkt, err := gnb.BuildNGSetupRequest(opts)
	if err != nil {
		t.Fatalf("could not build NGSetupRequest: %v", err)
	}

	return pkt
}

// TestRunReceiverDeliversFramesInOrder pins the invariant that no two frames
// of one UE are handled concurrently. The recorder's first SendDownlinkNAS
// publishes "in flight", waits up to 250 ms for a second delivery to start,
// and fails the test if one does.
//
// With per-UE FIFO the second delivery cannot start (the worker is busy with
// the first), the wait expires, and the test passes. With goroutine-per-frame
// the second delivery starts immediately and the test fails every run.
func TestRunReceiverDeliversFramesInOrder(t *testing.T) {
	skipIfNoSCTP(t)

	const (
		port    = 29421
		ranUEID = uint64(2)
	)

	const (
		firstNAS  = "service-accept"
		secondNAS = "configuration-update-command"
	)

	acceptedCh := make(chan *sctp.SCTPConn, 1)

	srv := sctp.NewServer(sctp.Config{
		PPID:   60,
		Name:   "TEST",
		Logger: zap.NewNop(),
	}, sctp.Callbacks{
		Dispatch: func(_ context.Context, conn *sctp.SCTPConn, _ []byte) {
			select {
			case acceptedCh <- conn:
			default:
			}
		},
	})

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	if err := srv.ListenAndServe(ctx, "127.0.0.1", port, ""); err != nil {
		t.Fatalf("ListenAndServe: %v", err)
	}

	t.Cleanup(func() {
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer shutdownCancel()

		srv.Shutdown(shutdownCtx)
	})

	loopback := &sctp.SCTPAddr{IPAddrs: []net.IPAddr{{IP: net.ParseIP("127.0.0.1")}}}

	gnbConn, err := sctp.Dial(ctx, "sctp", loopback, &sctp.SCTPAddr{IPAddrs: loopback.IPAddrs, Port: port}, sctp.InitMsg{NumOstreams: 2, MaxInstreams: 2})
	if err != nil {
		t.Fatalf("dial: %v", err)
	}

	// Use NewGnodeB with pre-dialed conn so the gNB's receiver is on the same
	// connection the test uses to send downlink frames.
	g := gnb.NewGnodeB("000102", "001", "01", 1, "", "internet", "000001", "test", gnbConn, nil, netip.Addr{})
	t.Cleanup(g.Close)

	recorder := &recordingDownlinkSender{hold: make(chan struct{})}

	var holdClosed sync.Once

	t.Cleanup(func() {
		holdClosed.Do(func() { close(recorder.hold) })
	})
	g.AddUE(int64(ranUEID), recorder)

	// Send NG Setup Request to trigger the server's response and establish
	// the association.
	ngSetup := marshalNGSetupRequest(t)
	if _, err := gnbConn.WriteMsg(ngSetup, &sctp.SndRcvInfo{PPID: sctp.PPIDWireOrder(60), Stream: 0}); err != nil {
		t.Fatalf("NG Setup trigger write: %v", err)
	}

	// Wait for the server to dispatch the association (which is gnbConn).
	var amfConn *sctp.SCTPConn
	select {
	case amfConn = <-acceptedCh:
	case <-time.After(8 * time.Second):
		t.Fatal("server never dispatched the association; cannot capture it")
	}

	// Push the first frame. The recorder blocks after recording it.
	p1 := marshalDownlinkNASTransport(t, ranUEID, []byte(firstNAS))
	if _, err := amfConn.WriteMsg(p1, &sctp.SndRcvInfo{PPID: sctp.PPIDWireOrder(60), Stream: 0}); err != nil {
		t.Fatalf("write first frame: %v", err)
	}

	// Wait for the first delivery to be recorded.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if recorder.started.Load() >= 1 {
			break
		}

		time.Sleep(10 * time.Millisecond)
	}

	if recorder.started.Load() < 1 {
		t.Fatal("first frame never delivered")
	}

	// Now push the second frame. With per-UE FIFO it cannot be handled until
	// the first completes (which is blocked). With goroutine-per-frame it
	// would start immediately.
	p2 := marshalDownlinkNASTransport(t, ranUEID, []byte(secondNAS))
	if _, err := amfConn.WriteMsg(p2, &sctp.SndRcvInfo{PPID: sctp.PPIDWireOrder(60), Stream: 0}); err != nil {
		t.Fatalf("write second frame: %v", err)
	}

	// The assertion: poll for 250 ms and fail if a second frame starts.
	deadline = time.Now().Add(250 * time.Millisecond)
	for time.Now().Before(deadline) {
		if recorder.started.Load() > 1 {
			t.Fatal("a second frame of the same UE started while the first was still being handled; per-UE FIFO is violated")
		}

		time.Sleep(5 * time.Millisecond)
	}

	// Let the first frame through.
	holdClosed.Do(func() { close(recorder.hold) })

	// Wait for both deliveries.
	deadline = time.Now().Add(8 * time.Second)
	for time.Now().Before(deadline) {
		if recorder.finished.Load() >= 2 {
			break
		}

		time.Sleep(10 * time.Millisecond)
	}

	if recorder.finished.Load() < 2 {
		t.Fatalf("timed out waiting for the second downlink NAS message; received %d", recorder.finished.Load())
	}

	// Verify payload order.
	got := recorder.snapshot()
	if len(got) < 2 {
		t.Fatalf("expected at least 2 deliveries, got %d", len(got))
	}

	if string(got[0]) != firstNAS || string(got[1]) != secondNAS {
		t.Fatalf("frames handled out of order: got %q, %q; want %q, %q", got[0], got[1], firstNAS, secondNAS)
	}
}

// TestRunReceiverDoesNotBlockOtherUEs asserts that a wedged handler for one UE
// does not block delivery to another UE on the same association.
func TestRunReceiverDoesNotBlockOtherUEs(t *testing.T) {
	skipIfNoSCTP(t)

	const (
		port = 29422
		ueA  = uint64(10)
		ueB  = uint64(11)
	)

	acceptedCh := make(chan *sctp.SCTPConn, 1)

	srv := sctp.NewServer(sctp.Config{
		PPID:   60,
		Name:   "TEST",
		Logger: zap.NewNop(),
	}, sctp.Callbacks{
		Dispatch: func(_ context.Context, conn *sctp.SCTPConn, _ []byte) {
			select {
			case acceptedCh <- conn:
			default:
			}
		},
	})

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	if err := srv.ListenAndServe(ctx, "127.0.0.1", port, ""); err != nil {
		t.Fatalf("ListenAndServe: %v", err)
	}

	t.Cleanup(func() {
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer shutdownCancel()

		srv.Shutdown(shutdownCtx)
	})

	loopback := &sctp.SCTPAddr{IPAddrs: []net.IPAddr{{IP: net.ParseIP("127.0.0.1")}}}

	gnbConn, err := sctp.Dial(ctx, "sctp", loopback, &sctp.SCTPAddr{IPAddrs: loopback.IPAddrs, Port: port}, sctp.InitMsg{NumOstreams: 2, MaxInstreams: 2})
	if err != nil {
		t.Fatalf("dial: %v", err)
	}

	// Use NewGnodeB with pre-dialed conn so the gNB's receiver is on the same
	// connection the test uses to send downlink frames.
	g := gnb.NewGnodeB("000102", "001", "01", 1, "", "internet", "000001", "test", gnbConn, nil, netip.Addr{})
	t.Cleanup(g.Close)

	ueARecorder := &recordingDownlinkSender{hold: make(chan struct{})}
	ueBRecorder := &recordingDownlinkSender{}

	var ueAHoldClosed sync.Once

	t.Cleanup(func() {
		ueAHoldClosed.Do(func() { close(ueARecorder.hold) })
	})
	g.AddUE(int64(ueA), ueARecorder)
	g.AddUE(int64(ueB), ueBRecorder)

	// Send NG Setup Request to trigger the server's response and establish
	// the association.
	ngSetup := marshalNGSetupRequest(t)
	if _, err := gnbConn.WriteMsg(ngSetup, &sctp.SndRcvInfo{PPID: sctp.PPIDWireOrder(60), Stream: 0}); err != nil {
		t.Fatalf("NG Setup trigger write: %v", err)
	}

	// Wait for the server to dispatch the association (which is gnbConn).
	var amfConn *sctp.SCTPConn
	select {
	case amfConn = <-acceptedCh:
	case <-time.After(8 * time.Second):
		t.Fatal("server never dispatched the association")
	}

	// Block UE A's handler and push its frame.
	pA := marshalDownlinkNASTransport(t, ueA, []byte("ue-a"))
	if _, err := amfConn.WriteMsg(pA, &sctp.SndRcvInfo{PPID: sctp.PPIDWireOrder(60), Stream: 0}); err != nil {
		t.Fatalf("write UE A frame: %v", err)
	}

	// Push UE B's frame immediately after.
	pB := marshalDownlinkNASTransport(t, ueB, []byte("ue-b"))
	if _, err := amfConn.WriteMsg(pB, &sctp.SndRcvInfo{PPID: sctp.PPIDWireOrder(60), Stream: 0}); err != nil {
		t.Fatalf("write UE B frame: %v", err)
	}

	// UE B should be delivered within ~200 ms (not blocked by UE A).
	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		if ueBRecorder.started.Load() >= 1 {
			// Let UE A through so the test can clean up.
			ueAHoldClosed.Do(func() { close(ueARecorder.hold) })
			return
		}

		time.Sleep(10 * time.Millisecond)
	}

	ueAHoldClosed.Do(func() { close(ueARecorder.hold) })
	t.Fatal("UE B was not delivered within 500 ms; cross-UE HOL blocking detected")
}
