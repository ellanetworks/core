// Copyright 2026 Ella Networks

package listener

// ALPN protocol identifiers negotiated during the TLS handshake on the
// cluster port. Every intra-cluster connection must negotiate one of these;
// connections with an unknown or empty NegotiatedProtocol are closed.
const (
	ALPNRaft = "ella-raft-v1"
	ALPNHTTP = "ella-http-v1"
	// ALPNPKIBootstrap is the join-flow ALPN. Unlike every other
	// protocol on this port it does not require a client certificate;
	// the caller authenticates via a join-token HMAC carried in the
	// request body, and pins the server via root-fingerprint. The
	// listener's verifyConnection hook lets connections through without
	// client certs iff this ALPN is negotiated.
	ALPNPKIBootstrap = "ella-pki-bootstrap-v1"
)

// RequiresClientCert reports whether the negotiated ALPN requires the
// peer to present a client certificate. Everything but the bootstrap
// ALPN does.
func RequiresClientCert(alpn string) bool {
	return alpn != ALPNPKIBootstrap
}
