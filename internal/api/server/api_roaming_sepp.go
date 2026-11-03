// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package server

import (
	"bytes"
	"context"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/ellanetworks/core/internal/cluster/listener"
	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/pki"
	"github.com/ellanetworks/core/internal/sepp"
)

const (
	RehandshakeRoamingPartnerAction = "rehandshake_roaming_partner"
	GenerateSEPPCertificateAction   = "generate_sepp_certificate"
	ImportSEPPCertificateAction     = "import_sepp_certificate"
	RegenerateSEPPCAAction          = "regenerate_sepp_ca"

	seppNodeStatusTimeout = 2 * time.Second

	seppTrustAnchorChangedWarning = "The trust anchor of this node changed: every roaming partner must import the new trust anchor before N32 with this node works again"
	seppGeneratedWarning          = "Ella Core issued this certificate with the cluster SEPP CA, which has no CRL distribution point: it suits labs and bilateral agreements, while TS 33.310 clause 6.1.3c.3 expects production certificates from the operator's CA (import one per node)"
	seppCARegeneratedWarning      = "The cluster SEPP CA changed: every roaming partner must import the new trust anchor before N32 with the nodes that use generated certificates works again"
)

type SEPPRuntime interface {
	Status() sepp.NodeStatus
	Rehandshake(partnerID string) (sepp.ContextStatus, error)
}

type RoamingPartnerN32 struct {
	NodeID     string                  `json:"nodeID,omitempty"`
	SEPPID     int                     `json:"seppID,omitempty"`
	State      string                  `json:"state"`
	Since      string                  `json:"since,omitempty"`
	Initiator  string                  `json:"initiator,omitempty"`
	NFTLST     bool                    `json:"nftlst"`
	Peer       string                  `json:"peer,omitempty"`
	LastError  string                  `json:"lastError,omitempty"`
	OtherPeers []RoamingPartnerN32Peer `json:"otherPeers,omitempty"`
}

type RoamingPartnerN32Peer struct {
	Peer   string `json:"peer"`
	Since  string `json:"since"`
	NFTLST bool   `json:"nftlst"`
}

type RoamingPartnerHandshake struct {
	N32 []RoamingPartnerN32 `json:"n32"`
}

type SEPPCertificateSummary struct {
	Subject                string   `json:"subject"`
	SANs                   []string `json:"sans"`
	Issuer                 string   `json:"issuer"`
	NotBefore              string   `json:"notBefore"`
	NotAfter               string   `json:"notAfter"`
	Fingerprint            string   `json:"fingerprint"`
	TrustAnchorFingerprint string   `json:"trustAnchorFingerprint"`
}

type SEPPTrustAnchor struct {
	Source      string `json:"source"`
	Subject     string `json:"subject"`
	Fingerprint string `json:"fingerprint"`
	NotAfter    string `json:"notAfter"`
	PEM         string `json:"pem"`
	SEPPIDs     []int  `json:"seppIDs"`
}

type SEPPNodeStatus struct {
	NodeID        string                  `json:"nodeID,omitempty"`
	SEPPID        int                     `json:"seppID,omitempty"`
	FQDN          string                  `json:"fqdn,omitempty"`
	Source        string                  `json:"source,omitempty"`
	Reachable     bool                    `json:"reachable"`
	N32Configured bool                    `json:"n32Configured"`
	Address       string                  `json:"address,omitempty"`
	Port          int                     `json:"port,omitempty"`
	Listening     bool                    `json:"listening"`
	Certificate   *SEPPCertificateSummary `json:"certificate,omitempty"`
	Error         string                  `json:"error,omitempty"`
}

type SEPPStatus struct {
	TrustAnchors []SEPPTrustAnchor `json:"trustAnchors"`
	Nodes        []SEPPNodeStatus  `json:"nodes"`
}

type SEPPCertificate struct {
	SEPPID              int      `json:"seppID"`
	FQDN                string   `json:"fqdn"`
	Source              string   `json:"source"`
	CertificateChainPEM string   `json:"certificateChainPEM"`
	TrustAnchorPEM      string   `json:"trustAnchorPEM"`
	Fingerprint         string   `json:"fingerprint"`
	NotAfter            string   `json:"notAfter"`
	Warnings            []string `json:"warnings,omitempty"`
}

type ImportSEPPCertificateParams struct {
	CertificateChainPEM string `json:"certificateChainPEM"`
	PrivateKeyPEM       string `json:"privateKeyPEM"`
	FQDN                string `json:"fqdn,omitempty"`
}

type SEPPCARegenerated struct {
	TrustAnchor SEPPTrustAnchor `json:"trustAnchor"`
	Reissued    []int           `json:"reissued"`
	Warnings    []string        `json:"warnings,omitempty"`
}

type seppNode struct {
	nodeID  string
	seppID  int
	address string
	self    bool
}

type seppNodeView struct {
	seppNode
	status *sepp.NodeStatus
	err    error
}

func roamingPartnerN32(v seppNodeView, st sepp.ContextStatus) RoamingPartnerN32 {
	out := RoamingPartnerN32{
		NodeID:    v.nodeID,
		SEPPID:    v.seppID,
		State:     string(st.State),
		Initiator: string(st.Initiator),
		NFTLST:    st.NFTLST,
		Peer:      st.Peer,
		LastError: st.LastError,
	}

	if !st.Since.IsZero() {
		out.Since = st.Since.UTC().Format(time.RFC3339)
	}

	for _, pc := range st.OtherPeers {
		out.OtherPeers = append(out.OtherPeers, RoamingPartnerN32Peer{Peer: pc.Peer, Since: pc.Since.UTC().Format(time.RFC3339), NFTLST: pc.NFTLST})
	}

	return out
}

func partnerN32(views []seppNodeView, partnerID string) []RoamingPartnerN32 {
	out := make([]RoamingPartnerN32, 0, len(views))

	for _, v := range views {
		switch {
		case v.err != nil:
			out = append(out, RoamingPartnerN32{NodeID: v.nodeID, SEPPID: v.seppID, State: "unknown", LastError: v.err.Error()})
		case !v.status.Listener.Configured:
			continue
		default:
			st, ok := v.status.Contexts[partnerID]
			if !ok {
				st = sepp.ContextStatus{State: sepp.StateIdle}
			}

			out = append(out, roamingPartnerN32(v, st))
		}
	}

	return out
}

func withN32(resp RoamingPartner, p db.RoamingPartner, views []seppNodeView) RoamingPartner {
	if p.SEPPFQDN != nil && views != nil {
		resp.N32 = partnerN32(views, p.ID)
	}

	return resp
}

func seppNodes(ctx context.Context, dbInstance *db.Database) []seppNode {
	self := seppNode{nodeID: dbInstance.RaftID(), seppID: dbInstance.AMFPointer(), self: true}

	if !dbInstance.ClusterEnabled() {
		return []seppNode{self}
	}

	members, err := dbInstance.ListClusterMembers(ctx)
	if err != nil || len(members) == 0 {
		return []seppNode{self}
	}

	out := make([]seppNode, 0, len(members))

	for _, m := range members {
		n := seppNode{nodeID: m.NodeID, seppID: m.AMFPointer, self: m.NodeID == self.nodeID}
		if srv := dbInstance.RaftServer(m.NodeID); srv != nil {
			n.address = srv.Address
		}

		out = append(out, n)
	}

	slices.SortFunc(out, func(a, b seppNode) int { return a.seppID - b.seppID })

	return out
}

func gatherSEPPNodes(ctx context.Context, dbInstance *db.Database, rt SEPPRuntime, ln *listener.Listener) []seppNodeView {
	nodes := seppNodes(ctx, dbInstance)
	out := make([]seppNodeView, len(nodes))

	var wg sync.WaitGroup

	for i, n := range nodes {
		out[i].seppNode = n

		if n.self {
			if rt == nil {
				out[i].err = errors.New("the SEPP is not running")
				continue
			}

			st := rt.Status()
			out[i].status = &st

			continue
		}

		wg.Go(func() {
			status, body, err := clusterNodeDo(ctx, ln, n.nodeID, n.address, http.MethodGet, sepp.ClusterStatusPath, nil)

			switch {
			case err != nil:
				out[i].err = fmt.Errorf("unreachable: %w", err)
			case status != http.StatusOK:
				out[i].err = fmt.Errorf("the node answered %d", status)
			default:
				st, err := sepp.DecodeNodeStatus(body)
				if err != nil {
					out[i].err = err
				} else {
					out[i].status = &st
				}
			}
		})
	}

	wg.Wait()

	return out
}

func clusterNodeDo(ctx context.Context, ln *listener.Listener, nodeID, addr, method, path string, payload []byte) (int, []byte, error) {
	if ln == nil {
		return 0, nil, errors.New("no cluster listener")
	}

	if addr == "" {
		return 0, nil, errors.New("no cluster address")
	}

	ctx, cancel := context.WithTimeout(ctx, seppNodeStatusTimeout)
	defer cancel()

	conn, err := ln.Dial(ctx, addr, nodeID, listener.ALPNHTTP, seppNodeStatusTimeout)
	if err != nil {
		return 0, nil, err
	}

	var once sync.Once

	tr := &http.Transport{
		DialTLSContext: func(context.Context, string, string) (net.Conn, error) {
			var c net.Conn

			once.Do(func() { c = conn })

			if c == nil {
				return nil, errors.New("cluster connection already used")
			}

			return c, nil
		},
		DisableKeepAlives: true,
	}
	defer tr.CloseIdleConnections()

	req, err := http.NewRequestWithContext(ctx, method, "https://"+addr+path, bytes.NewReader(payload))
	if err != nil {
		_ = conn.Close()
		return 0, nil, err
	}

	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := (&http.Client{Transport: tr}).Do(req)
	if err != nil {
		_ = conn.Close()
		return 0, nil, err
	}

	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 256<<10))
	if err != nil {
		return 0, nil, err
	}

	return resp.StatusCode, body, nil
}

func RehandshakeRoamingPartner(dbInstance *db.Database, rt SEPPRuntime, ln *listener.Listener) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		email := getEmailFromContext(r)
		id := r.PathValue("id")

		partner, err := dbInstance.GetRoamingPartner(r.Context(), id)
		if err != nil {
			writeRoamingError(w, r, "roaming partner", "get", err)
			return
		}

		if partner.SEPPFQDN == nil {
			writeError(r.Context(), w, http.StatusConflict, "Roaming partner has no 5G agreement", nil, logger.APILog)
			return
		}

		views := gatherSEPPNodes(r.Context(), dbInstance, rt, ln)
		resp := RoamingPartnerHandshake{N32: []RoamingPartnerN32{}}

		payload, err := json.Marshal(sepp.HandshakeRequest{PartnerID: id})
		if err != nil {
			writeError(r.Context(), w, http.StatusInternalServerError, "Failed to encode the handshake request", err, logger.APILog)
			return
		}

		var mu sync.Mutex

		var wg sync.WaitGroup

		for _, v := range views {
			if v.err == nil && !v.status.Listener.Configured {
				continue
			}

			wg.Go(func() {
				out := rehandshakeOnNode(r.Context(), rt, ln, v, id, payload)

				mu.Lock()

				resp.N32 = append(resp.N32, out)
				mu.Unlock()
			})
		}

		wg.Wait()

		slices.SortFunc(resp.N32, func(a, b RoamingPartnerN32) int { return a.SEPPID - b.SEPPID })

		if len(resp.N32) == 0 {
			writeError(r.Context(), w, http.StatusConflict, "No node has interfaces.n32 configured", nil, logger.APILog)
			return
		}

		writeResponse(r.Context(), w, resp, http.StatusAccepted, logger.APILog)

		logger.LogAuditEvent(r.Context(), RehandshakeRoamingPartnerAction, email, getClientIP(r), "Requested an N32-c handshake with roaming partner "+id+" ("+partner.Name+") on every SEPP node")
	})
}

func rehandshakeOnNode(ctx context.Context, rt SEPPRuntime, ln *listener.Listener, v seppNodeView, partnerID string, payload []byte) RoamingPartnerN32 {
	failed := func(err error) RoamingPartnerN32 {
		return RoamingPartnerN32{NodeID: v.nodeID, SEPPID: v.seppID, State: "unknown", LastError: err.Error()}
	}

	if v.self {
		if rt == nil {
			return failed(errors.New("the SEPP is not running"))
		}

		st, err := rt.Rehandshake(partnerID)
		if err != nil {
			return failed(err)
		}

		return roamingPartnerN32(v, st)
	}

	status, body, err := clusterNodeDo(ctx, ln, v.nodeID, v.address, http.MethodPost, sepp.ClusterHandshakePath, payload)
	if err != nil {
		return failed(fmt.Errorf("unreachable: %w", err))
	}

	if status != http.StatusOK {
		return failed(fmt.Errorf("the node answered %d: %s", status, strings.TrimSpace(string(body))))
	}

	st, err := sepp.DecodeContextStatus(body)
	if err != nil {
		return failed(err)
	}

	return roamingPartnerN32(v, st)
}

func certificateSummary(cert pki.SEPPCertificate) *SEPPCertificateSummary {
	leaf := cert.Leaf()

	return &SEPPCertificateSummary{
		Subject:                leaf.Subject.String(),
		SANs:                   append([]string{}, leaf.DNSNames...),
		Issuer:                 leaf.Issuer.String(),
		NotBefore:              leaf.NotBefore.UTC().Format(time.RFC3339),
		NotAfter:               leaf.NotAfter.UTC().Format(time.RFC3339),
		Fingerprint:            pki.Fingerprint(leaf),
		TrustAnchorFingerprint: pki.Fingerprint(cert.TrustAnchor()),
	}
}

func trustAnchorOf(source string, anchor *x509.Certificate) SEPPTrustAnchor {
	return SEPPTrustAnchor{
		Source:      source,
		Subject:     anchor.Subject.String(),
		Fingerprint: pki.Fingerprint(anchor),
		NotAfter:    anchor.NotAfter.UTC().Format(time.RFC3339),
		PEM:         string(pki.EncodeCertPEM(anchor)),
		SEPPIDs:     []int{},
	}
}

func GetSEPPStatus(dbInstance *db.Database, rt SEPPRuntime, ln *listener.Listener) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		identities, err := dbInstance.ListSEPPIdentities(r.Context())
		if err != nil {
			writeError(r.Context(), w, http.StatusInternalServerError, "Failed to get the SEPP status", err, logger.APILog)
			return
		}

		resp := SEPPStatus{TrustAnchors: []SEPPTrustAnchor{}, Nodes: []SEPPNodeStatus{}}
		anchors := map[string]int{}

		addAnchor := func(source string, anchor *x509.Certificate, seppID int) {
			fp := pki.Fingerprint(anchor)

			i, ok := anchors[fp]
			if !ok {
				i = len(resp.TrustAnchors)
				anchors[fp] = i

				resp.TrustAnchors = append(resp.TrustAnchors, trustAnchorOf(source, anchor))
			}

			if seppID != 0 {
				resp.TrustAnchors[i].SEPPIDs = append(resp.TrustAnchors[i].SEPPIDs, seppID)
			}
		}

		ca, err := dbInstance.GetSEPPCA(r.Context())
		if err != nil && !errors.Is(err, db.ErrNotFound) {
			writeError(r.Context(), w, http.StatusInternalServerError, "Failed to get the SEPP CA", err, logger.APILog)
			return
		}

		if ca != nil {
			if cert, err := ca.Certificate(); err == nil {
				addAnchor(db.SEPPIdentityGenerated, cert, 0)
			}
		}

		byID := map[int]db.SEPPIdentity{}
		certs := map[int]pki.SEPPCertificate{}

		for _, identity := range identities {
			byID[identity.SEPPID] = identity

			cert, err := pki.ParseSEPPCertificate([]byte(identity.CertChainPEM), identity.FQDN)
			if err != nil {
				continue
			}

			certs[identity.SEPPID] = cert
			addAnchor(identity.Source, cert.TrustAnchor(), identity.SEPPID)
		}

		seen := map[int]bool{}

		for _, v := range gatherSEPPNodes(r.Context(), dbInstance, rt, ln) {
			node := SEPPNodeStatus{NodeID: v.nodeID, SEPPID: v.seppID}

			if identity, ok := byID[v.seppID]; ok {
				node.FQDN, node.Source = identity.FQDN, identity.Source
			}

			if cert, ok := certs[v.seppID]; ok {
				node.Certificate = certificateSummary(cert)
			}

			if v.err != nil {
				node.Error = v.err.Error()
			} else {
				node.Reachable = true
				node.N32Configured = v.status.Listener.Configured
				node.Address = v.status.Listener.Address
				node.Port = v.status.Listener.Port
				node.Listening = v.status.Listener.Listening
				node.Error = v.status.Listener.Error
			}

			seen[v.seppID] = true

			resp.Nodes = append(resp.Nodes, node)
		}

		for _, identity := range identities {
			if seen[identity.SEPPID] {
				continue
			}

			node := SEPPNodeStatus{SEPPID: identity.SEPPID, FQDN: identity.FQDN, Source: identity.Source, Error: "no cluster node has this SEPP ID"}
			if cert, ok := certs[identity.SEPPID]; ok {
				node.Certificate = certificateSummary(cert)
			}

			resp.Nodes = append(resp.Nodes, node)
		}

		writeResponse(r.Context(), w, resp, http.StatusOK, logger.APILog)
	})
}

func seppIDParam(w http.ResponseWriter, r *http.Request) (int, bool) {
	id, err := strconv.Atoi(r.PathValue("seppID"))
	if err != nil || !db.ValidSEPPID(id) {
		writeError(r.Context(), w, http.StatusBadRequest, fmt.Sprintf("SEPP ID must be an integer between %d and %d", db.MinSEPPID, db.MaxSEPPID), nil, logger.APILog)
		return 0, false
	}

	return id, true
}

func seppCertificateResponse(identity db.SEPPIdentity, cert pki.SEPPCertificate, warnings ...string) SEPPCertificate {
	return SEPPCertificate{
		SEPPID:              identity.SEPPID,
		FQDN:                cert.FQDN,
		Source:              identity.Source,
		CertificateChainPEM: cert.ChainPEM(),
		TrustAnchorPEM:      string(pki.EncodeCertPEM(cert.TrustAnchor())),
		Fingerprint:         pki.Fingerprint(cert.Leaf()),
		NotAfter:            cert.Leaf().NotAfter.UTC().Format(time.RFC3339),
		Warnings:            append(cert.ProfileWarnings(), warnings...),
	}
}

func storedSEPPCertificate(ctx context.Context, dbInstance *db.Database, seppID int) (*db.SEPPIdentity, *pki.SEPPCertificate, error) {
	identity, err := dbInstance.GetSEPPIdentity(ctx, seppID)
	if err != nil {
		return nil, nil, err
	}

	cert, err := pki.ParseSEPPCertificate([]byte(identity.CertChainPEM), identity.FQDN)
	if err != nil {
		return nil, nil, fmt.Errorf("parse the stored SEPP certificate: %w", err)
	}

	return identity, &cert, nil
}

func GetSEPPCertificate(dbInstance *db.Database) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seppID, ok := seppIDParam(w, r)
		if !ok {
			return
		}

		identity, cert, err := storedSEPPCertificate(r.Context(), dbInstance, seppID)
		if err != nil {
			writeRoamingError(w, r, "SEPP certificate", "get", err)
			return
		}

		writeResponse(r.Context(), w, seppCertificateResponse(*identity, *cert), http.StatusOK, logger.APILog)
	})
}

func anchorChanged(ctx context.Context, dbInstance *db.Database, seppID int, next pki.SEPPCertificate) (bool, error) {
	_, previous, err := storedSEPPCertificate(ctx, dbInstance, seppID)
	if errors.Is(err, db.ErrNotFound) {
		return false, nil
	}

	if err != nil {
		return false, err
	}

	return !previous.TrustAnchor().Equal(next.TrustAnchor()), nil
}

func ImportSEPPCertificate(dbInstance *db.Database) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		email := getEmailFromContext(r)

		seppID, ok := seppIDParam(w, r)
		if !ok {
			return
		}

		var params ImportSEPPCertificateParams
		if err := json.NewDecoder(r.Body).Decode(&params); err != nil {
			writeError(r.Context(), w, http.StatusBadRequest, "Invalid request data", err, logger.APILog)
			return
		}

		plmn, err := operatorPLMN(r.Context(), dbInstance)
		if err != nil {
			writeError(r.Context(), w, http.StatusInternalServerError, "Failed to get the operator PLMN", err, logger.APILog)
			return
		}

		cert, err := pki.ValidateSEPPCertificate([]byte(params.CertificateChainPEM), []byte(params.PrivateKeyPEM), strings.TrimSpace(params.FQDN), plmn, time.Now())
		if err != nil {
			writeError(r.Context(), w, http.StatusBadRequest, "Invalid SEPP certificate: "+err.Error(), nil, logger.APILog)
			return
		}

		var warnings []string

		if cert.FQDN != plmn.SEPPFQDN(seppID) {
			warnings = append(warnings, "The FQDN "+cert.FQDN+" differs from "+plmn.SEPPFQDN(seppID)+", the GSMA FS.34 clause 5.1.1 name of SEPP "+strconv.Itoa(seppID))
		}

		changed, err := anchorChanged(r.Context(), dbInstance, seppID, cert)
		if err != nil {
			writeError(r.Context(), w, http.StatusInternalServerError, "Failed to read the current SEPP certificate", err, logger.APILog)
			return
		}

		if changed {
			warnings = append(warnings, seppTrustAnchorChangedWarning)
		}

		identity := db.SEPPIdentity{SEPPID: seppID, FQDN: cert.FQDN, Source: db.SEPPIdentityImported, CertChainPEM: cert.ChainPEM(), KeyPEM: params.PrivateKeyPEM}
		if err := dbInstance.PutSEPPIdentity(r.Context(), &identity); err != nil {
			writeRoamingError(w, r, "SEPP certificate", "import", err)
			return
		}

		resp := seppCertificateResponse(identity, cert, warnings...)
		writeResponse(r.Context(), w, resp, http.StatusOK, logger.APILog)

		logger.LogAuditEvent(r.Context(), ImportSEPPCertificateAction, email, getClientIP(r), "Imported the certificate of SEPP "+strconv.Itoa(seppID)+" for "+cert.FQDN+" ("+resp.Fingerprint+", expires "+resp.NotAfter+")")
	})
}

func GenerateSEPPCertificate(dbInstance *db.Database) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		email := getEmailFromContext(r)

		seppID, ok := seppIDParam(w, r)
		if !ok {
			return
		}

		previous, _, err := storedSEPPCertificate(r.Context(), dbInstance, seppID)
		if err != nil && !errors.Is(err, db.ErrNotFound) {
			writeError(r.Context(), w, http.StatusInternalServerError, "Failed to read the current SEPP certificate", err, logger.APILog)
			return
		}

		identity, err := dbInstance.GenerateSEPPIdentity(r.Context(), seppID)
		if err != nil {
			writeRoamingError(w, r, "SEPP certificate", "generate", err)
			return
		}

		cert, err := pki.ParseSEPPCertificate([]byte(identity.CertChainPEM), identity.FQDN)
		if err != nil {
			writeError(r.Context(), w, http.StatusInternalServerError, "Failed to parse the generated SEPP certificate", err, logger.APILog)
			return
		}

		warnings := []string{seppGeneratedWarning}
		if previous != nil && previous.Source == db.SEPPIdentityImported {
			warnings = append(warnings, seppTrustAnchorChangedWarning)
		}

		resp := seppCertificateResponse(*identity, cert, warnings...)
		writeResponse(r.Context(), w, resp, http.StatusCreated, logger.APILog)

		logger.LogAuditEvent(r.Context(), GenerateSEPPCertificateAction, email, getClientIP(r), "Generated the certificate of SEPP "+strconv.Itoa(seppID)+" for "+cert.FQDN+" ("+resp.Fingerprint+", expires "+resp.NotAfter+")")
	})
}

func RegenerateSEPPCA(dbInstance *db.Database) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		email := getEmailFromContext(r)

		identities, err := dbInstance.ListSEPPIdentities(r.Context())
		if err != nil {
			writeError(r.Context(), w, http.StatusInternalServerError, "Failed to list the SEPP certificates", err, logger.APILog)
			return
		}

		ca, err := dbInstance.RegenerateSEPPCA(r.Context())
		if err != nil {
			writeRoamingError(w, r, "SEPP CA", "regenerate", err)
			return
		}

		caCert, err := ca.Certificate()
		if err != nil {
			writeError(r.Context(), w, http.StatusInternalServerError, "Failed to parse the SEPP CA", err, logger.APILog)
			return
		}

		resp := SEPPCARegenerated{TrustAnchor: trustAnchorOf(db.SEPPIdentityGenerated, caCert), Reissued: []int{}, Warnings: []string{seppCARegeneratedWarning}}

		for _, identity := range identities {
			if identity.Source != db.SEPPIdentityGenerated {
				continue
			}

			if _, err := dbInstance.GenerateSEPPIdentity(r.Context(), identity.SEPPID); err != nil {
				resp.Warnings = append(resp.Warnings, "SEPP "+strconv.Itoa(identity.SEPPID)+" has no certificate yet: "+err.Error())
				continue
			}

			resp.Reissued = append(resp.Reissued, identity.SEPPID)
			resp.TrustAnchor.SEPPIDs = append(resp.TrustAnchor.SEPPIDs, identity.SEPPID)
		}

		writeResponse(r.Context(), w, resp, http.StatusCreated, logger.APILog)

		logger.LogAuditEvent(r.Context(), RegenerateSEPPCAAction, email, getClientIP(r), "Regenerated the cluster SEPP CA ("+resp.TrustAnchor.Fingerprint+") and reissued the generated certificates of SEPPs "+fmt.Sprint(resp.Reissued))
	})
}
