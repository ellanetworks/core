// Copyright 2026 Ella Networks

package raft

import (
	"fmt"
	"net"
	"time"

	"github.com/hashicorp/raft"
)

// TransportFactory builds the Raft network transport for a Manager. It is
// called once during NewManager with the full cluster configuration.
//
// The factory must return a transport whose LocalAddr() reflects the actual
// bound address — that address is used as the server entry in single-server
// bootstrap configurations, and binding to an ephemeral port (the standalone
// default) relies on this invariant.
//
// The default tcpTransportFactory produces a plain TCP transport. HA
// deployments plug in an mTLS-backed factory via WithTransportFactory
// without modifying NewManager.
type TransportFactory func(cfg ClusterConfig) (raft.Transport, error)

// tcpTransportFactory is the default plain-TCP builder. It applies the
// standalone bind-address default (127.0.0.1:0), validates that HA mode
// supplies an explicit bind address, and leaves the advertise address nil
// when binding to an ephemeral port so TCPStreamLayer falls back to the
// listener's concrete address.
func tcpTransportFactory(cfg ClusterConfig) (raft.Transport, error) {
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

	transport, err := raft.NewTCPTransport(bindAddress, advertise, 3, 10*time.Second, newZapIOWriter("transport"))
	if err != nil {
		return nil, fmt.Errorf("create TCP transport on %s: %w", bindAddress, err)
	}

	return transport, nil
}

// ManagerOption configures optional behaviour of NewManager.
type ManagerOption func(*managerOptions)

type managerOptions struct {
	transportFactory TransportFactory
}

// WithTransportFactory overrides the default TCP transport builder. HA
// deployments use this to plug in an mTLS-backed transport without
// modifying NewManager.
func WithTransportFactory(tf TransportFactory) ManagerOption {
	return func(o *managerOptions) { o.transportFactory = tf }
}
