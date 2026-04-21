// Copyright 2026 Ella Networks

package raft

import (
	"context"
	"errors"
	"fmt"
	"net"
	"time"

	"github.com/ellanetworks/core/internal/cluster/listener"
	hraft "github.com/hashicorp/raft"
)

// raftStreamLayer adapts a cluster Listener into a raft.StreamLayer.
// Inbound Raft connections arrive via the listener's ALPNRaft handler
// and are fed to Accept through a buffered channel. Outbound connections
// dial through the listener's Dial method, which presents this node's
// leaf certificate and negotiates ALPNRaft.
type raftStreamLayer struct {
	ln      *listener.Listener
	accepts chan net.Conn
	closeCh chan struct{}
	addr    net.Addr
}

var _ hraft.StreamLayer = (*raftStreamLayer)(nil)

func newRaftStreamLayer(ln *listener.Listener, advertiseAddr string) (*raftStreamLayer, error) {
	addr, err := net.ResolveTCPAddr("tcp", advertiseAddr)
	if err != nil {
		return nil, fmt.Errorf("resolve advertise address %s: %w", advertiseAddr, err)
	}

	sl := &raftStreamLayer{
		ln:      ln,
		accepts: make(chan net.Conn, 16),
		closeCh: make(chan struct{}),
		addr:    addr,
	}

	ln.Register(listener.ALPNRaft, sl.handleConn)

	return sl, nil
}

func (s *raftStreamLayer) handleConn(conn net.Conn) {
	select {
	case s.accepts <- conn:
	case <-s.closeCh:
		_ = conn.Close()
	}
}

func (s *raftStreamLayer) Accept() (net.Conn, error) {
	select {
	case conn := <-s.accepts:
		return conn, nil
	case <-s.closeCh:
		return nil, errors.New("raft stream layer closed")
	}
}

func (s *raftStreamLayer) Close() error {
	select {
	case <-s.closeCh:
	default:
		close(s.closeCh)
	}

	return nil
}

func (s *raftStreamLayer) Addr() net.Addr {
	return s.addr
}

// Dial opens an mTLS connection to a peer's Raft stream layer via the
// cluster listener. context.Background is used because raft.StreamLayer
// does not accept a context; the timeout parameter provides the deadline.
//
// The expected-peer check is intentionally skipped on the Raft channel:
// hashicorp/raft.StreamLayer.Dial hands us only a server address, not
// the target ServerID, and the raft instance that could resolve the ID
// from configuration is constructed *after* this stream layer. The Raft
// protocol itself bounds the impact of a misrouted connection — an
// endpoint that cannot forge valid log state cannot make meaningful
// replies, and an on-path actor that merely drops traffic is already
// within the attacker model of the transport layer. Callers that need
// stronger per-peer binding (leader proxy, discovery joins) use the
// cluster HTTP ALPN and pass an explicit expectedPeerID to Dial.
func (s *raftStreamLayer) Dial(address hraft.ServerAddress, timeout time.Duration) (net.Conn, error) {
	conn, err := s.ln.DialAnyPeer(context.Background(), string(address), listener.ALPNRaft, timeout)
	if err != nil {
		return nil, err
	}

	return conn, nil
}
