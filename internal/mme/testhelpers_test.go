// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"sync"
	"testing"

	"github.com/ellanetworks/core/internal/sctp"
	"github.com/ellanetworks/core/nas"
	"github.com/ellanetworks/core/s1ap"
)

type captureConn struct {
	mu   sync.Mutex
	sent [][]byte
}

func (c *captureConn) WriteMsg(b []byte, _ *sctp.SndRcvInfo) (int, error) {
	c.mu.Lock()
	c.sent = append(c.sent, append([]byte(nil), b...))
	c.mu.Unlock()

	return len(b), nil
}

func (c *captureConn) count() int {
	c.mu.Lock()
	defer c.mu.Unlock()

	return len(c.sent)
}

func (c *captureConn) snapshot() [][]byte {
	c.mu.Lock()
	defer c.mu.Unlock()

	return append([][]byte(nil), c.sent...)
}

func mobileIdentityDigits(b []byte) string {
	digits, err := nas.DecodeBCDIdentity(b)
	if err != nil {
		return ""
	}

	return digits
}

func decodeDownlinkNAS(t *testing.T, pdu []byte) []byte {
	t.Helper()

	msg, err := s1ap.Unmarshal(pdu)
	if err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	im, ok := msg.(*s1ap.InitiatingMessage)
	if !ok || im.ProcedureCode != s1ap.ProcDownlinkNASTransport {
		t.Fatalf("expected Downlink NAS Transport, got %T", msg)
	}

	dl, err := s1ap.ParseDownlinkNASTransport(im.Value)
	if err != nil {
		t.Fatalf("parse downlink: %v", err)
	}

	return []byte(dl.NASPDU)
}
