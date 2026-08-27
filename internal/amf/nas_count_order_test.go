// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package amf

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/ellanetworks/core/internal/guard"
	"github.com/ellanetworks/core/internal/sctp"
	"github.com/ellanetworks/core/nas"
	"github.com/ellanetworks/core/nas/fgs"
	"github.com/ellanetworks/core/ngap"
	"go.uber.org/zap"
)

type downlinkOrderConn struct {
	mu    sync.Mutex
	seqs  []uint8
	shts  []uint8
	wrote chan struct{}
}

func (c *downlinkOrderConn) WriteMsg(b []byte, _ *sctp.SndRcvInfo) (int, error) {
	pdu, err := ngap.Unmarshal(b)
	if err != nil {
		panic(fmt.Sprintf("downlinkOrderConn: unmarshal NGAP PDU: %v", err))
	}

	m, ok := pdu.(*ngap.InitiatingMessage)
	if !ok || m.ProcedureCode != ngap.ProcDownlinkNASTransport {
		return len(b), nil
	}

	transport, err := ngap.ParseDownlinkNASTransport(m.Value)
	if err != nil {
		panic(fmt.Sprintf("downlinkOrderConn: parse Downlink NAS Transport: %v", err))
	}

	spm, err := fgs.ParseSecurityProtectedMessage(transport.NASPDU)
	if err != nil {
		panic(fmt.Sprintf("downlinkOrderConn: parse security protected message: %v", err))
	}

	c.mu.Lock()
	c.seqs = append(c.seqs, spm.SequenceNumber)
	c.shts = append(c.shts, uint8(spm.SecurityHeaderType))
	c.mu.Unlock()

	select {
	case c.wrote <- struct{}{}:
	default:
	}

	return len(b), nil
}

func (c *downlinkOrderConn) awaitWrites(t *testing.T, n int) (seqs, shts []uint8) {
	t.Helper()

	deadline := time.After(5 * time.Second)

	for {
		c.mu.Lock()
		if len(c.seqs) >= n {
			seqs = append([]uint8(nil), c.seqs[:n]...)
			shts = append([]uint8(nil), c.shts[:n]...)
			c.mu.Unlock()

			return seqs, shts
		}
		c.mu.Unlock()

		select {
		case <-c.wrote:
		case <-deadline:
			t.Fatalf("only %d of %d downlink messages were written", len(c.seqs), n)
		}
	}
}

func newDownlinkOrderUE(t *testing.T) (*UeContext, *downlinkOrderConn) {
	t.Helper()

	sender := &downlinkOrderConn{wrote: make(chan struct{}, 1)}

	ue := NewUeContext()
	ue.secured = true
	ue.integrityAlg = nas.IntegrityAES
	ue.knasInt = [16]uint8{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}

	ue.reinstallSecurityContextForTest()

	if ue.sc == nil {
		t.Fatal("install security context")
	}

	radio := &Radio{name: "test-gNB", Log: zap.NewNop()}
	radio.BindAMFForTest(New(nil, nil, nil))

	ueConn := &UeConn{
		conn:        sender,
		radioName:   radio.name,
		amf:         radio.amf,
		RanUeNgapID: 1,
		AmfUeNgapID: 1,
	}
	ueConn.setLog(zap.NewNop())
	ueConn.amf.AttachUeConn(ue, ueConn)

	return ue, sender
}

func TestDownlinkNASWrittenInCountOrder_TS24501_4_4_3_1(t *testing.T) {
	ue, sender := newDownlinkOrderUE(t)
	ueConn := ue.Conn()

	const (
		senders  = 8
		messages = 30
	)

	var (
		wg    sync.WaitGroup
		start = make(chan struct{})
	)

	for range senders {
		wg.Add(1)

		go func() {
			defer wg.Done()

			<-start

			for range messages {
				SendDLNASTransport(context.Background(), ueConn, fgs.PayloadContainerTypeN1SMInfo, []byte{0x2e, 0x01, 0x01}, 1, 0)
			}
		}()
	}

	close(start)
	wg.Wait()

	if len(sender.seqs) != senders*messages {
		t.Fatalf("wrote %d downlink messages, want %d", len(sender.seqs), senders*messages)
	}

	for i, seq := range sender.seqs {
		if int(seq) != i {
			t.Fatalf("downlink %d carries NAS sequence number %d: the AMF wrote the downlink NAS COUNTs out of the order it took them in, and a UE estimating the COUNT from the sequence number discards the message",
				i, seq)
		}
	}
}

// TS 24.501 §4.4.3.1
func TestRegistrationAcceptResendTakesTheNextNASCount_TS24501_4_4_3_1(t *testing.T) {
	ue, sender := newDownlinkOrderUE(t)

	conn := ue.Conn()
	conn.RegistrationAcceptPlain = []byte{0x7e, 0x00, 0x42, 0x01, 0x01}

	const resends = 3

	for range resends {
		ResendRegistrationAccept(context.Background(), conn.amf, ue)
	}

	seqs, _ := sender.awaitWrites(t, resends)

	for i, seq := range seqs {
		if int(seq) != i {
			t.Fatalf("resend %d carries NAS sequence number %d, want %d: a retransmission that replays a spent NAS COUNT reuses the key stream", i, seq, i)
		}
	}
}

// TS 24.501 §4.4.3.1
func TestNASGuardRetransmissionTakesTheNextNASCount_TS24501_4_4_3_1(t *testing.T) {
	ue, sender := newDownlinkOrderUE(t)

	conn := ue.Conn()
	sht := uint8(fgs.SHTIntegrityProtectedNewContext)
	cfg := guard.TimerValue{Enable: true, ExpireTime: time.Millisecond, MaxRetryTimes: 8}

	armNASGuard(conn, conn, cfg, "T3560 (Security Mode Command)", []byte{0x7e, 0x00, 0x5d, 0x02, 0x02}, sht, func() {})

	seqs, shts := sender.awaitWrites(t, 2)

	conn.StopNASGuard()

	if seqs[0] != 0 || seqs[1] != 1 {
		t.Fatalf("retransmissions carry NAS sequence numbers %v, want 0 then 1", seqs)
	}

	for i, got := range shts {
		if got != sht {
			t.Fatalf("retransmission %d carries security header type %d, want %d", i, got, sht)
		}
	}
}

func TestSecurityModeCommandIsTheFirstMessageUnderTheNewContext_TS24501_4_4_3_1(t *testing.T) {
	ue, sender := newDownlinkOrderUE(t)
	ueConn := ue.Conn()

	SendDLNASTransport(context.Background(), ueConn, fgs.PayloadContainerTypeN1SMInfo, []byte{0x2e, 0x01, 0x01}, 1, 0)

	if seqs, _ := sender.awaitWrites(t, 1); seqs[0] != 0 {
		t.Fatalf("first downlink carries sequence number %d, want 0", seqs[0])
	}

	ue.ueSecurityCapability = &fgs.UESecurityCapability{EA: 0xe0, IA: 0xe0}
	ue.kamf = make([]byte, 32)

	if err := ue.InstallNASSecurityContext(nas.CipheringAES, nas.IntegrityAES, MintAuthProofForSecurityMode()); err != nil {
		t.Fatalf("InstallNASSecurityContext: %v", err)
	}

	if err := SendSecurityModeCommand(context.Background(), ueConn.amf, ueConn); err != nil {
		t.Fatalf("SendSecurityModeCommand: %v", err)
	}

	seqs, shts := sender.awaitWrites(t, 2)

	if seqs[1] != 0 {
		t.Errorf("the Security Mode Command carries sequence number %d, want 0: it is the first message under the new context", seqs[1])
	}

	if shts[1] != uint8(fgs.SHTIntegrityProtectedNewContext) {
		t.Errorf("security header type = %#x, want integrity protected with a new 5G NAS security context", shts[1])
	}
}
