// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package server

import (
	"context"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/pki"
	"github.com/ellanetworks/core/internal/sepp"
)

const (
	RehandshakeRoamingPartnerAction = "rehandshake_roaming_partner"
	DeleteSEPPCertificateAction     = "delete_sepp_certificate"
	ImportSEPPCertificateAction     = "import_sepp_certificate"
	RegenerateSEPPCAAction          = "regenerate_sepp_ca"

	seppTrustAnchorChangedWarning = "The trust anchor of this node changed: every roaming partner must import the new trust anchor before N32 with this node works again"
	seppCARegeneratedWarning      = "The cluster SEPP CA changed: every roaming partner must import the new trust anchor before N32 with the nodes that use generated certificates works again"
)

type SEPPRuntime interface {
	Status() sepp.NodeStatus
	Rehandshake(partnerID string) (sepp.ContextStatus, error)
}

type RoamingPartnerN32 struct {
	State     string `json:"state"`
	Peer      string `json:"peer,omitempty"`
	LastError string `json:"lastError,omitempty"`
}

type RoamingPartnerHandshake struct {
	N32 RoamingPartnerN32 `json:"n32"`
}

type SEPPCertificate struct {
	SEPPID                 int      `json:"seppID"`
	FQDN                   string   `json:"fqdn"`
	Source                 string   `json:"source"`
	Subject                string   `json:"subject,omitempty"`
	SANs                   []string `json:"sans,omitempty"`
	Issuer                 string   `json:"issuer,omitempty"`
	NotBefore              string   `json:"notBefore,omitempty"`
	NotAfter               string   `json:"notAfter,omitempty"`
	Fingerprint            string   `json:"fingerprint,omitempty"`
	TrustAnchorFingerprint string   `json:"trustAnchorFingerprint,omitempty"`
	Warnings               []string `json:"warnings,omitempty"`
}

type SEPPTrustAnchor struct {
	Source      string `json:"source"`
	Subject     string `json:"subject"`
	Fingerprint string `json:"fingerprint"`
	NotAfter    string `json:"notAfter"`
	PEM         string `json:"pem"`
	SEPPIDs     []int  `json:"seppIDs"`
}

type SEPPListener struct {
	SEPPID     int    `json:"seppID,omitempty"`
	Configured bool   `json:"configured"`
	Address    string `json:"address,omitempty"`
	Port       int    `json:"port,omitempty"`
	Listening  bool   `json:"listening"`
	Error      string `json:"error,omitempty"`
}

type SEPPStatus struct {
	Listener     SEPPListener      `json:"listener"`
	Certificates []SEPPCertificate `json:"certificates"`
	TrustAnchors []SEPPTrustAnchor `json:"trustAnchors"`
}

type ImportSEPPCertificateParams struct {
	CertificateChainPEM string `json:"certificateChainPEM"`
	PrivateKeyPEM       string `json:"privateKeyPEM"`
	FQDN                string `json:"fqdn,omitempty"`
}

type SEPPCARegenerated struct {
	TrustAnchor SEPPTrustAnchor `json:"trustAnchor"`
	Warnings    []string        `json:"warnings,omitempty"`
}

func roamingPartnerN32(st sepp.ContextStatus) RoamingPartnerN32 {
	return RoamingPartnerN32{State: string(st.State), Peer: st.Peer, LastError: st.LastError}
}

func localSEPPStatus(rt SEPPRuntime) *sepp.NodeStatus {
	if rt == nil {
		return nil
	}

	st := rt.Status()

	return &st
}

func withN32(resp RoamingPartner, p db.RoamingPartner, st *sepp.NodeStatus) RoamingPartner {
	if p.SEPPFQDN == nil || st == nil || !st.Listener.Configured {
		return resp
	}

	ctx, ok := st.Contexts[p.ID]
	if !ok {
		ctx = sepp.ContextStatus{State: sepp.StateIdle}
	}

	n32 := roamingPartnerN32(ctx)
	resp.N32 = &n32

	return resp
}

func RehandshakeRoamingPartner(dbInstance *db.Database, rt SEPPRuntime) http.Handler {
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

		if st := localSEPPStatus(rt); st == nil || !st.Listener.Configured {
			writeError(r.Context(), w, http.StatusConflict, "This node has no interfaces.n32 configured", nil, logger.APILog)
			return
		}

		var resp RoamingPartnerHandshake

		st, err := rt.Rehandshake(id)
		if err != nil {
			resp.N32 = RoamingPartnerN32{State: "unknown", LastError: err.Error()}
		} else {
			resp.N32 = roamingPartnerN32(st)
		}

		writeResponse(r.Context(), w, resp, http.StatusAccepted, logger.APILog)

		logger.LogAuditEvent(r.Context(), RehandshakeRoamingPartnerAction, email, getClientIP(r), "Requested an N32-c handshake with roaming partner "+id+" ("+partner.Name+")")
	})
}

func seppCertificate(identity db.SEPPIdentity, cert *pki.SEPPCertificate, warnings ...string) SEPPCertificate {
	out := SEPPCertificate{SEPPID: identity.SEPPID, FQDN: identity.FQDN, Source: identity.Source, Warnings: warnings}
	if cert == nil {
		return out
	}

	leaf := cert.Leaf()

	out.Subject = leaf.Subject.String()
	out.SANs = append([]string{}, leaf.DNSNames...)
	out.Issuer = leaf.Issuer.String()
	out.NotBefore = leaf.NotBefore.UTC().Format(time.RFC3339)
	out.NotAfter = leaf.NotAfter.UTC().Format(time.RFC3339)
	out.Fingerprint = pki.Fingerprint(leaf)
	out.TrustAnchorFingerprint = pki.Fingerprint(cert.TrustAnchor())

	return out
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

func GetSEPPStatus(dbInstance *db.Database, rt SEPPRuntime) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		identities, err := dbInstance.ListSEPPIdentities(r.Context())
		if err != nil {
			writeError(r.Context(), w, http.StatusInternalServerError, "Failed to get the SEPP status", err, logger.APILog)
			return
		}

		resp := SEPPStatus{Listener: SEPPListener{SEPPID: dbInstance.AMFPointer()}, TrustAnchors: []SEPPTrustAnchor{}, Certificates: []SEPPCertificate{}}
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

		for _, identity := range identities {
			cert, err := pki.ParseSEPPCertificate([]byte(identity.CertChainPEM), identity.FQDN)
			if err != nil {
				resp.Certificates = append(resp.Certificates, seppCertificate(identity, nil))
				continue
			}

			addAnchor(identity.Source, cert.TrustAnchor(), identity.SEPPID)
			resp.Certificates = append(resp.Certificates, seppCertificate(identity, &cert))
		}

		if st := localSEPPStatus(rt); st == nil {
			resp.Listener.Error = "the SEPP is not running"
		} else {
			resp.Listener.Configured = st.Listener.Configured
			resp.Listener.Address = st.Listener.Address
			resp.Listener.Port = st.Listener.Port
			resp.Listener.Listening = st.Listener.Listening
			resp.Listener.Error = st.Listener.Error
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

func anchorChanged(ctx context.Context, dbInstance *db.Database, seppID int, next pki.SEPPCertificate) (bool, error) {
	identity, err := dbInstance.GetSEPPIdentity(ctx, seppID)
	if errors.Is(err, db.ErrNotFound) {
		return false, nil
	}

	if err != nil {
		return false, err
	}

	previous, err := pki.ParseSEPPCertificate([]byte(identity.CertChainPEM), identity.FQDN)
	if err != nil {
		return true, nil
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

		warnings := cert.ProfileWarnings()

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

		resp := seppCertificate(identity, &cert, warnings...)
		writeResponse(r.Context(), w, resp, http.StatusOK, logger.APILog)

		logger.LogAuditEvent(r.Context(), ImportSEPPCertificateAction, email, getClientIP(r), "Imported the certificate of SEPP "+strconv.Itoa(seppID)+" for "+cert.FQDN+" ("+resp.Fingerprint+", expires "+resp.NotAfter+")")
	})
}

func DeleteSEPPCertificate(dbInstance *db.Database) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		email := getEmailFromContext(r)

		seppID, ok := seppIDParam(w, r)
		if !ok {
			return
		}

		if err := dbInstance.DeleteSEPPIdentity(r.Context(), seppID); err != nil {
			writeRoamingError(w, r, "SEPP certificate", "delete", err)
			return
		}

		writeResponse(r.Context(), w, SuccessResponse{Message: "SEPP certificate deleted: the node generates a new one with the cluster SEPP CA"}, http.StatusOK, logger.APILog)

		logger.LogAuditEvent(r.Context(), DeleteSEPPCertificateAction, email, getClientIP(r), "Deleted the certificate of SEPP "+strconv.Itoa(seppID)+": the node generates a new one with the cluster SEPP CA")
	})
}

func RegenerateSEPPCA(dbInstance *db.Database) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		email := getEmailFromContext(r)

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

		resp := SEPPCARegenerated{TrustAnchor: trustAnchorOf(db.SEPPIdentityGenerated, caCert), Warnings: []string{seppCARegeneratedWarning}}

		writeResponse(r.Context(), w, resp, http.StatusCreated, logger.APILog)

		logger.LogAuditEvent(r.Context(), RegenerateSEPPCAAction, email, getClientIP(r), "Regenerated the cluster SEPP CA ("+resp.TrustAnchor.Fingerprint+"): the nodes with generated certificates reissue them")
	})
}
