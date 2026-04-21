// Copyright 2026 Ella Networks

// Handlers for the /cluster/pki/* surface on the cluster HTTP port.
//
//	POST /cluster/pki/issue  — join-token OR mTLS leaf. Sign a CSR.
//	POST /cluster/pki/renew  — mTLS leaf. Renew the presenting node's leaf.

package server

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"

	"github.com/ellanetworks/core/internal/cluster/listener"
	"github.com/ellanetworks/core/internal/cluster/pkiagent"
	"github.com/ellanetworks/core/internal/cluster/pkiissuer"
	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/pki"
	"go.uber.org/zap"
)

// clusterPKIMaxBody caps the request body we decode. IssueRequest's
// fields (CSR PEM + token + ids) never need more than a few KB.
const clusterPKIMaxBody = 32 * 1024

// ClusterPKIIssue handles POST /cluster/pki/issue.
//
// If the request arrives without an mTLS peer cert (bootstrap ALPN),
// the caller MUST supply a valid join token. Otherwise the caller
// MUST present a leaf whose node-id matches the CSR's (mTLS renewal).
func ClusterPKIIssue(svc *pkiissuer.Service) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !svc.Ready() {
			writeError(r.Context(), w, http.StatusServiceUnavailable, "pki issuer not ready", nil, logger.APILog)
			return
		}

		body, err := io.ReadAll(io.LimitReader(r.Body, clusterPKIMaxBody))
		if err != nil {
			writeError(r.Context(), w, http.StatusBadRequest, "read body", err, logger.APILog)
			return
		}

		var req pkiagent.IssueRequest
		if err := json.Unmarshal(body, &req); err != nil {
			writeError(r.Context(), w, http.StatusBadRequest, "decode body", err, logger.APILog)
			return
		}

		if req.NodeID < pki.MinNodeID || req.NodeID > pki.MaxNodeID {
			writeError(r.Context(), w, http.StatusBadRequest, "node-id out of range", nil, logger.APILog)
			return
		}

		csr, err := pki.ParseCSRPEM([]byte(req.CSRPEM))
		if err != nil {
			writeError(r.Context(), w, http.StatusBadRequest, "parse csr", err, logger.APILog)
			return
		}

		peerNodeID, hasPeer := peerNodeIDFromContext(r.Context())

		switch {
		case hasPeer:
			if peerNodeID != req.NodeID {
				writeError(r.Context(), w, http.StatusForbidden,
					"node-id in CSR does not match presenting leaf", nil, logger.APILog)

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
					"node-id in CSR does not match token claims", nil, logger.APILog)

				return
			}
		}

		leafPEM, err := svc.Issue(r.Context(), csr, req.NodeID, pki.DefaultLeafTTL)
		if err != nil {
			writeError(r.Context(), w, http.StatusInternalServerError, "issue leaf", err, logger.APILog)
			return
		}

		bundle, err := svc.CurrentBundle(r.Context())
		if err != nil {
			writeError(r.Context(), w, http.StatusInternalServerError, "get bundle", err, logger.APILog)
			return
		}

		writeIssueResponse(w, leafPEM, bundle)
	})
}

// ClusterPKIRenew handles POST /cluster/pki/renew (mTLS only). In
// addition to the base Issue flow, it verifies the presenting node is
// still a cluster member — a removed node's cert may still be within
// its TTL but should not renew.
func ClusterPKIRenew(dbInstance *db.Database, svc *pkiissuer.Service) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		peerNodeID, ok := peerNodeIDFromContext(r.Context())
		if !ok {
			writeError(r.Context(), w, http.StatusUnauthorized, "mTLS required for renew", nil, logger.APILog)
			return
		}

		if _, err := dbInstance.GetClusterMember(r.Context(), peerNodeID); err != nil {
			if errors.Is(err, db.ErrNotFound) {
				writeError(r.Context(), w, http.StatusForbidden,
					"node is no longer a cluster member", nil, logger.APILog)

				return
			}

			writeError(r.Context(), w, http.StatusInternalServerError, "verify membership", err, logger.APILog)

			return
		}

		ClusterPKIIssue(svc).ServeHTTP(w, r)
	})
}

func writeIssueResponse(w http.ResponseWriter, leafPEM []byte, bundle *pki.TrustBundle) {
	var bundlePEM []byte

	for _, r := range bundle.Roots {
		bundlePEM = append(bundlePEM, pki.EncodeCertPEM(r)...)
	}

	for _, i := range bundle.Intermediates {
		bundlePEM = append(bundlePEM, pki.EncodeCertPEM(i)...)
	}

	resp := pkiagent.IssueResponse{
		LeafPEM:   string(leafPEM),
		BundlePEM: string(bundlePEM),
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

// RegisterBootstrapALPN registers a listener handler for the bootstrap
// ALPN. Connections on this ALPN have no mTLS peer cert; the handler
// serves exactly one POST /cluster/pki/issue on the raw conn and then
// closes it.
func RegisterBootstrapALPN(ln *listener.Listener, svc *pkiissuer.Service) {
	issue := ClusterPKIIssue(svc)

	ln.Register(listener.ALPNPKIBootstrap, func(conn net.Conn) {
		defer func() { _ = conn.Close() }()

		br := bufio.NewReader(conn)

		req, err := http.ReadRequest(br)
		if err != nil {
			logger.APILog.Warn("bootstrap: read request", zap.Error(err))
			return
		}

		if req.Method != http.MethodPost || req.URL.Path != "/cluster/pki/issue" {
			writeInlineResponse(conn, http.StatusNotFound, "only POST /cluster/pki/issue is served on the bootstrap ALPN")
			return
		}

		bw := &bufferedResponseWriter{header: http.Header{}}
		issue.ServeHTTP(bw, req)
		bw.writeTo(conn)
	})
}

// bufferedResponseWriter is a minimal http.ResponseWriter that captures
// status, headers, and body so we can serialize one HTTP response on a
// raw conn without running an http.Server.
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
