// Copyright 2026 Ella Networks

// Handlers for /cluster/pki/register on the cluster HTTP port.
//
// A joining node POSTs (cert, token) on the bootstrap ALPN. A node
// rotating its cert POSTs (cert) over the regular mTLS ALPN; the
// presenting cert's nodeID must match the new cert's URI nodeID
// so a member cannot overwrite another member's pin.

package server

import (
	"bufio"
	"bytes"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"time"

	"github.com/ellanetworks/core/internal/cluster/listener"
	"github.com/ellanetworks/core/internal/cluster/pkiagent"
	"github.com/ellanetworks/core/internal/cluster/pkiissuer"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/pki"
	"go.uber.org/zap"
)

const clusterPKIMaxBody = 32 * 1024

// ClusterPKIRegister handles POST /cluster/pki/register. On the
// bootstrap ALPN it requires a valid join token; on the mTLS ALPN
// the presenting peer cert's nodeID must equal the requested
// nodeID, restricting re-pinning to the cert's owner.
func ClusterPKIRegister(svc *pkiissuer.Service) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(io.LimitReader(r.Body, clusterPKIMaxBody))
		if err != nil {
			writeError(r.Context(), w, http.StatusBadRequest, "read body", err, logger.APILog)
			return
		}

		var req pkiagent.RegisterRequest
		if err := json.Unmarshal(body, &req); err != nil {
			writeError(r.Context(), w, http.StatusBadRequest, "decode body", err, logger.APILog)
			return
		}

		if req.NodeID < pki.MinNodeID || req.NodeID > pki.MaxNodeID {
			writeError(r.Context(), w, http.StatusBadRequest, "node-id out of range", nil, logger.APILog)
			return
		}

		peerNodeID, hasPeer := peerNodeIDFromContext(r.Context())

		switch {
		case hasPeer:
			// mTLS path: the cert's owner is the only caller who
			// may re-pin its nodeID.
			if peerNodeID != req.NodeID {
				writeError(r.Context(), w, http.StatusForbidden,
					"node-id in body does not match presenting peer cert", nil, logger.APILog)

				return
			}

		default:
			if req.Token == "" {
				writeError(r.Context(), w, http.StatusUnauthorized,
					"join token required on bootstrap path", nil, logger.APILog)

				return
			}

			claims, err := svc.VerifyAndConsumeJoinToken(r.Context(), req.Token)
			if err != nil {
				writeError(r.Context(), w, http.StatusUnauthorized, "verify join token", err, logger.APILog)
				return
			}

			if claims.NodeID != req.NodeID {
				writeError(r.Context(), w, http.StatusForbidden,
					"node-id in body does not match token claims", nil, logger.APILog)

				return
			}
		}

		fp, err := svc.RegisterCert(r.Context(), req.NodeID, []byte(req.CertPEM))
		if err != nil {
			writeError(r.Context(), w, http.StatusBadRequest, "register cert", err, logger.APILog)
			return
		}

		// Refresh the local pin cache so handshakes from the new
		// peer succeed without waiting for the periodic refresher.
		nudgePinCache(r.Context())

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(pkiagent.RegisterResponse{Fingerprint: fp})
	})
}

// RegisterBootstrapALPN dispatches POST /cluster/pki/register on
// the bootstrap ALPN (no client cert) and closes the connection
// after one request.
func RegisterBootstrapALPN(ln *listener.Listener, svc *pkiissuer.Service) {
	register := ClusterPKIRegister(svc)

	ln.Register(listener.ALPNPKIBootstrap, func(conn net.Conn) {
		defer func() { _ = conn.Close() }()

		_ = conn.SetDeadline(time.Now().Add(30 * time.Second))

		br := bufio.NewReader(conn)

		req, err := http.ReadRequest(br)
		if err != nil {
			logger.APILog.Warn("bootstrap: read request", zap.Error(err))
			return
		}

		if req.Method != http.MethodPost || req.URL.Path != "/cluster/pki/register" {
			writeInlineResponse(conn, http.StatusNotFound, "only POST /cluster/pki/register is served on the bootstrap ALPN")
			return
		}

		bw := &bufferedResponseWriter{header: http.Header{}}
		register.ServeHTTP(bw, req)
		bw.writeTo(conn)
	})
}

type bufferedResponseWriter struct {
	header http.Header
	status int
	body   []byte
}

func (b *bufferedResponseWriter) Header() http.Header { return b.header }
func (b *bufferedResponseWriter) WriteHeader(status int) {
	if b.status == 0 {
		b.status = status
	}
}

func (b *bufferedResponseWriter) Write(p []byte) (int, error) {
	if b.status == 0 {
		b.status = http.StatusOK
	}

	b.body = append(b.body, p...)

	return len(p), nil
}

func (b *bufferedResponseWriter) writeTo(conn net.Conn) {
	if b.status == 0 {
		b.status = http.StatusOK
	}

	resp := &http.Response{
		StatusCode:    b.status,
		ProtoMajor:    1,
		ProtoMinor:    1,
		Header:        b.header,
		Body:          io.NopCloser(bytes.NewReader(b.body)),
		ContentLength: int64(len(b.body)),
	}

	_ = resp.Write(conn)
}

func writeInlineResponse(conn net.Conn, status int, msg string) {
	resp := &http.Response{
		StatusCode:    status,
		ProtoMajor:    1,
		ProtoMinor:    1,
		Header:        http.Header{"Content-Type": []string{"text/plain"}},
		Body:          io.NopCloser(bytes.NewReader([]byte(msg))),
		ContentLength: int64(len(msg)),
	}

	_ = resp.Write(conn)
}
