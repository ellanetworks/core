// Copyright 2026 Ella Networks

package listener

// ALPN protocol identifiers negotiated during the TLS handshake on the
// cluster port. Every intra-cluster connection must negotiate one of these;
// connections with an unknown or empty NegotiatedProtocol are closed.
const (
	ALPNRaft = "ella-raft-v1"
	ALPNHTTP = "ella-http-v1"
)
