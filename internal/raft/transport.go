// Copyright 2026 Ella Networks

package raft

import (
	"fmt"
	"net"
	"time"

	"github.com/ellanetworks/core/internal/cluster/listener"
	hraft "github.com/hashicorp/raft"
)

// tcpTransportFactory is the plain-TCP builder used in standalone mode.
// It applies the standalone bind-address default (127.0.0.1:0), validates
// that HA mode supplies an explicit bind address, and leaves the advertise
// address nil when binding to an ephemeral port so TCPStreamLayer falls
// back to the listener's concrete address.
func tcpTransportFactory(cfg ClusterConfig) (hraft.Transport, error) {
	singleServer := !cfg.Enabled

	bindAddress := cfg.BindAddress
	if bindAddress == "" {
		if !singleServer {
			return nil, fmt.Errorf("cluster.bind-address is required when HA is enabled")
		}

		bindAddress = defaultStandaloneBindAddress
	}

	var advertise net.Addr

	if cfg.BindAddress != "" {
		resolved, err := net.ResolveTCPAddr("tcp", cfg.BindAddress)
		if err != nil {
			return nil, fmt.Errorf("resolve bind address %s: %w", cfg.BindAddress, err)
		}

		advertise = resolved
	}

	transport, err := hraft.NewTCPTransport(bindAddress, advertise, 3, 10*time.Second, newZapIOWriter("transport"))
	if err != nil {
		return nil, fmt.Errorf("create TCP transport on %s: %w", bindAddress, err)
	}

	return transport, nil
}

// clusterTransportFactory builds a Raft transport backed by the cluster
// listener's ALPNRaft handler. Used in HA mode where all Raft traffic
// rides the mTLS-protected cluster port.
func clusterTransportFactory(ln *listener.Listener, cfg ClusterConfig) (hraft.Transport, error) {
	sl, err := newRaftStreamLayer(ln, cfg.AdvertiseAddress)
	if err != nil {
		return nil, err
	}

	return hraft.NewNetworkTransport(sl, 3, 10*time.Second, newZapIOWriter("transport")), nil
}

// ManagerOption configures optional behaviour of NewManager.
type ManagerOption func(*managerOptions)

type managerOptions struct {
	clusterListener *listener.Listener
}

// WithClusterListener provides the cluster listener for HA mode. The
// Raft stream layer registers ALPNRaft on this listener. Standalone
// mode ignores the option.
func WithClusterListener(ln *listener.Listener) ManagerOption {
	return func(o *managerOptions) { o.clusterListener = ln }
}
