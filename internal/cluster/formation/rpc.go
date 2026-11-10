// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package formation

import (
	"bufio"
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"time"

	"github.com/ellanetworks/core/internal/cluster/listener"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/pki"
	"go.uber.org/zap"
)

const (
	PathHello    = "/cluster/pki/hello"
	PathRegister = "/cluster/pki/register"

	HeaderNode    = "Ella-Cluster-Node"
	HeaderAddress = "Ella-Cluster-Address"
	HeaderProof   = "Ella-Cluster-Proof"

	maxBody     = 256 << 10
	callTimeout = 5 * time.Second
)

var (
	ErrWrongSecret = errors.New("the peer refused this node's cluster secret proof: check that every node has the same cluster secret")
	errBadProof    = errors.New("the peer's answer does not carry a valid cluster secret proof")
	errUnreachable = errors.New("peer unreachable")
)

type rpcError struct {
	Error string `json:"error"`
}

func IsSecretRequest(r *http.Request) bool {
	return r.URL.Path == PathHello || (r.URL.Path == PathRegister && r.Header.Get(HeaderProof) != "")
}

type reply struct {
	status int
	nodeID string
	body   []byte
}

func (n *Node) dial(ctx context.Context, addr string) (*tls.Conn, error) {
	if n.cfg.Dial != nil {
		return n.cfg.Dial(ctx, addr)
	}

	device, err := listener.ResolveBindDevice(ctx, n.cfg.BindAddress)
	if err != nil {
		return nil, fmt.Errorf("resolve cluster bind %s: %w", n.cfg.BindAddress, err)
	}

	raw, err := listener.DialContextTCPFromDevice(device)(ctx, "tcp", addr)
	if err != nil {
		return nil, err
	}

	conn := tls.Client(raw, &tls.Config{
		MinVersion:         tls.VersionTLS13,
		NextProtos:         []string{listener.ALPNPKIBootstrap},
		InsecureSkipVerify: true, // #nosec G402 -- the peer proves the cluster secret over this session's exporter
	})

	if err := conn.HandshakeContext(ctx); err != nil {
		_ = raw.Close()
		return nil, err
	}

	return conn, nil
}

func (n *Node) call(ctx context.Context, addr, path string, in any) (reply, error) {
	body, err := json.Marshal(in)
	if err != nil {
		return reply{}, err
	}

	ctx, cancel := context.WithTimeout(ctx, callTimeout)
	defer cancel()

	conn, err := n.dial(ctx, addr)
	if err != nil {
		return reply{}, fmt.Errorf("%w: %w", errUnreachable, err)
	}

	defer func() { _ = conn.Close() }()

	if deadline, ok := ctx.Deadline(); ok {
		_ = conn.SetDeadline(deadline)
	}

	ekm, err := pki.Exporter(conn.ConnectionState())
	if err != nil {
		return reply{}, err
	}

	proof, err := n.cfg.Auth.prove(pki.ProofInput{
		Role: pki.RoleClient, Exporter: ekm, NodeID: n.cfg.NodeID, Address: n.cfg.Advertise, BodyHash: requestHash(path, body),
	})
	if err != nil {
		return reply{}, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://"+addr+path, bytes.NewReader(body))
	if err != nil {
		return reply{}, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(HeaderNode, n.cfg.NodeID)
	req.Header.Set(HeaderAddress, n.cfg.Advertise)
	req.Header.Set(HeaderProof, proof)
	req.Close = true

	if err := req.Write(conn); err != nil {
		return reply{}, fmt.Errorf("%w: %w", errUnreachable, err)
	}

	resp, err := http.ReadResponse(bufio.NewReader(conn), req)
	if err != nil {
		return reply{}, fmt.Errorf("%w: %w", errUnreachable, err)
	}

	defer func() { _ = resp.Body.Close() }()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxBody))
	if err != nil {
		return reply{}, fmt.Errorf("%w: %w", errUnreachable, err)
	}

	switch resp.StatusCode {
	case http.StatusUnauthorized:
		return reply{}, ErrWrongSecret
	case http.StatusTooManyRequests:
		return reply{}, fmt.Errorf("%w: %w", errUnreachable, ErrWrongSecret)
	case http.StatusNotFound:
		return reply{}, fmt.Errorf("%w: the peer does not serve %s", errUnreachable, path)
	}

	peer := resp.Header.Get(HeaderNode)

	if peer == "" || !n.cfg.Auth.verify(pki.ProofInput{
		Role: pki.RoleServer, Exporter: ekm, NodeID: peer, Address: resp.Header.Get(HeaderAddress), BodyHash: responseHash(path, resp.StatusCode, raw),
	}, resp.Header.Get(HeaderProof)) {
		return reply{}, errBadProof
	}

	return reply{status: resp.StatusCode, nodeID: peer, body: raw}, nil
}

func (n *Node) ServeSecret(conn net.Conn, w http.ResponseWriter, r *http.Request) {
	ip, _, _ := net.SplitHostPort(conn.RemoteAddr().String())

	tlsConn, ok := listener.TLSConn(conn)
	if !ok || !n.cfg.Auth.Enabled() {
		writeJSON(w, http.StatusNotFound, rpcError{Error: "this node has no cluster secret"})
		return
	}

	if n.cfg.Auth.limited(ip) {
		writeJSON(w, http.StatusTooManyRequests, rpcError{Error: "too many failed proofs"})
		return
	}

	body, err := io.ReadAll(io.LimitReader(r.Body, maxBody))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, rpcError{Error: err.Error()})
		return
	}

	ekm, err := pki.Exporter(tlsConn.ConnectionState())
	callerID := r.Header.Get(HeaderNode)

	if err != nil || callerID == "" || !n.cfg.Auth.verify(pki.ProofInput{
		Role: pki.RoleClient, Exporter: ekm, NodeID: callerID, Address: r.Header.Get(HeaderAddress), BodyHash: requestHash(r.URL.Path, body),
	}, r.Header.Get(HeaderProof)) {
		n.cfg.Auth.fail(ip)

		logger.EllaLog.Warn("refused a cluster request with a wrong secret proof",
			zap.String("remote", conn.RemoteAddr().String()), zap.String("node", callerID))
		writeJSON(w, http.StatusUnauthorized, rpcError{Error: "invalid cluster secret proof"})

		return
	}

	status, v := n.route(r, callerID, body)

	out, err := json.Marshal(v)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, rpcError{Error: err.Error()})
		return
	}

	proof, err := n.cfg.Auth.prove(pki.ProofInput{
		Role: pki.RoleServer, Exporter: ekm, NodeID: n.cfg.NodeID, Address: n.cfg.Advertise, BodyHash: responseHash(r.URL.Path, status, out),
	})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, rpcError{Error: err.Error()})
		return
	}

	w.Header().Set(HeaderNode, n.cfg.NodeID)
	w.Header().Set(HeaderAddress, n.cfg.Advertise)
	w.Header().Set(HeaderProof, proof)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write(out)
}

func (n *Node) route(r *http.Request, callerID string, body []byte) (int, any) {
	switch r.URL.Path {
	case PathHello:
		var in Hello
		if err := json.Unmarshal(body, &in); err != nil || in.NodeID != callerID {
			return http.StatusBadRequest, rpcError{Error: "invalid hello"}
		}

		out := n.selfHello(r.Context())

		if out.ClusterID != "" && in.ClusterID != "" && in.ClusterID != out.ClusterID {
			logger.EllaLog.Warn("a node of another cluster sent a hello: check the cluster.join addresses of both clusters",
				zap.String("node", in.NodeID), zap.String("address", in.Address), zap.String("cluster", in.ClusterID))
		}

		return http.StatusOK, out
	case PathRegister:
		var in RegisterRequest
		if err := json.Unmarshal(body, &in); err != nil || in.NodeID != callerID {
			return http.StatusBadRequest, rpcError{Error: "invalid registration"}
		}

		if self := n.selfHello(r.Context()); self.State != StateMember {
			return http.StatusServiceUnavailable, rpcError{Error: "this node is not a cluster member yet"}
		}

		out, err := n.cfg.Register(r.Context(), in.NodeID, []byte(in.CertPEM))
		if err != nil {
			logger.EllaLog.Warn("refused a cluster registration", zap.String("node", in.NodeID), zap.Error(err))
			return http.StatusConflict, rpcError{Error: err.Error()}
		}

		logger.EllaLog.Info("registered a node joining with the cluster secret", zap.String("node", in.NodeID))

		return http.StatusCreated, out
	}

	return http.StatusNotFound, rpcError{Error: "not found"}
}

func requestHash(path string, body []byte) []byte {
	return pki.BodyHash(fmt.Appendf(nil, "%s\n%s", path, body))
}

func responseHash(path string, status int, body []byte) []byte {
	return pki.BodyHash(fmt.Appendf(nil, "%s\n%d\n%s", path, status, body))
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
