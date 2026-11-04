// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"strconv"
	"strings"

	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/pki"
	"github.com/google/uuid"
)

const (
	CreateRoamingPartnerAction      = "create_roaming_partner"
	UpdateRoamingPartnerAction      = "update_roaming_partner"
	DeleteRoamingPartnerAction      = "delete_roaming_partner"
	CreateRoamingDiameterPeerAction = "create_roaming_diameter_peer"
	UpdateRoamingDiameterPeerAction = "update_roaming_diameter_peer"
	DeleteRoamingDiameterPeerAction = "delete_roaming_diameter_peer"
)

type RoamingPartnerSEPP struct {
	FQDN           string `json:"fqdn"`
	N32cURI        string `json:"n32cURI"`
	N32fURI        string `json:"n32fURI,omitempty"`
	TrustAnchorPEM string `json:"trustAnchorPEM"`
	NRFAPIRoot     string `json:"nrfAPIRoot,omitempty"`
}

type RoamingPLMN struct {
	Mcc string `json:"mcc"`
	Mnc string `json:"mnc"`
}

type RoamingPartner struct {
	ID             string              `json:"id"`
	Name           string              `json:"name"`
	PLMNs          []RoamingPLMN       `json:"plmns"`
	ProfileName    string              `json:"profileName"`
	SEPP           *RoamingPartnerSEPP `json:"sepp,omitempty"`
	DiameterPeerID string              `json:"diameterPeerID,omitempty"`
	N32            *RoamingPartnerN32  `json:"n32,omitempty"`
}

type RoamingPartnerParams struct {
	Name           string              `json:"name"`
	PLMNs          []RoamingPLMN       `json:"plmns"`
	ProfileName    string              `json:"profileName"`
	SEPP           *RoamingPartnerSEPP `json:"sepp,omitempty"`
	DiameterPeerID string              `json:"diameterPeerID,omitempty"`
}

type ListRoamingPartnersResponse struct {
	Items []RoamingPartner `json:"items"`
}

type RoamingDiameterPeer struct {
	ID               string `json:"id"`
	DiameterIdentity string `json:"diameterIdentity"`
	Address          string `json:"address"`
	Port             int    `json:"port"`
}

type RoamingDiameterPeerParams struct {
	DiameterIdentity string `json:"diameterIdentity"`
	Address          string `json:"address"`
	Port             int    `json:"port,omitempty"`
}

type ListRoamingDiameterPeersResponse struct {
	Items []RoamingDiameterPeer `json:"items"`
}

func writeRoamingError(w http.ResponseWriter, r *http.Request, resource, action string, err error) {
	title := strings.ToUpper(resource[:1]) + resource[1:]

	switch {
	case errors.Is(err, db.ErrNotFound):
		writeError(r.Context(), w, http.StatusNotFound, title+" not found", nil, logger.APILog)
	case errors.Is(err, db.ErrRoamingConflict):
		writeError(r.Context(), w, http.StatusConflict, roamingConflictReason(err), nil, logger.APILog)
	case errors.Is(err, db.ErrAlreadyExists):
		writeError(r.Context(), w, http.StatusConflict, title+" already exists", nil, logger.APILog)
	case errors.Is(err, db.ErrInUse):
		writeError(r.Context(), w, http.StatusConflict, title+" is used by a roaming partner", nil, logger.APILog)
	default:
		writeError(r.Context(), w, http.StatusInternalServerError, "Failed to "+action+" "+resource, err, logger.APILog)
	}
}

func roamingConflictReason(err error) string {
	msg := err.Error()
	prefix := db.ErrRoamingConflict.Error() + ": "

	if i := strings.LastIndex(msg, prefix); i >= 0 {
		return msg[i+len(prefix):]
	}

	return msg
}

func operatorPLMN(ctx context.Context, dbInstance *db.Database) (pki.PLMN, error) {
	op, err := dbInstance.GetOperator(ctx)
	if err != nil {
		return pki.PLMN{}, err
	}

	return pki.PLMN{MCC: op.Mcc, MNC: op.Mnc}, nil
}

func roamingPartnerResponse(p db.RoamingPartner, profileName string) RoamingPartner {
	out := RoamingPartner{
		ID:          p.ID,
		Name:        p.Name,
		PLMNs:       make([]RoamingPLMN, 0, len(p.PLMNs)),
		ProfileName: profileName,
	}

	for _, plmn := range p.PLMNs {
		out.PLMNs = append(out.PLMNs, RoamingPLMN{Mcc: plmn.Mcc, Mnc: plmn.Mnc})
	}

	if p.SEPPFQDN != nil {
		out.SEPP = &RoamingPartnerSEPP{
			FQDN:           *p.SEPPFQDN,
			N32cURI:        derefString(p.SEPPN32cURI),
			N32fURI:        derefString(p.SEPPN32fURI),
			TrustAnchorPEM: derefString(p.SEPPTrustAnchorPEM),
			NRFAPIRoot:     derefString(p.NRFAPIRoot),
		}
	}

	if p.DiameterPeerID != nil {
		out.DiameterPeerID = *p.DiameterPeerID
	}

	return out
}

func derefString(s *string) string {
	if s == nil {
		return ""
	}

	return *s
}

func optionalString(s string) *string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}

	return &s
}

type profileNames struct {
	dbInstance *db.Database
	byID       map[string]string
}

func (n *profileNames) name(ctx context.Context, id string) (string, error) {
	if name, ok := n.byID[id]; ok {
		return name, nil
	}

	profile, err := n.dbInstance.GetProfileByID(ctx, id)
	if err != nil {
		return "", err
	}

	n.byID[id] = profile.Name

	return profile.Name, nil
}

func roamingPartnerFromParams(ctx context.Context, dbInstance *db.Database, id string, params RoamingPartnerParams) (db.RoamingPartner, int, string) {
	if strings.TrimSpace(params.ProfileName) == "" {
		return db.RoamingPartner{}, http.StatusBadRequest, "profileName is required"
	}

	partner := db.RoamingPartner{
		ID:             id,
		Name:           strings.TrimSpace(params.Name),
		DiameterPeerID: optionalString(params.DiameterPeerID),
	}

	for _, plmn := range params.PLMNs {
		partner.PLMNs = append(partner.PLMNs, db.RoamingPLMN{Mcc: strings.TrimSpace(plmn.Mcc), Mnc: strings.TrimSpace(plmn.Mnc)})
	}

	if params.SEPP != nil {
		partner.SEPPFQDN = optionalString(strings.ToLower(params.SEPP.FQDN))
		partner.SEPPN32cURI = optionalString(db.NormalizeAPIRoot(params.SEPP.N32cURI))
		partner.SEPPN32fURI = optionalString(db.NormalizeAPIRoot(params.SEPP.N32fURI))
		partner.NRFAPIRoot = optionalString(db.NormalizeAPIRoot(params.SEPP.NRFAPIRoot))

		if strings.TrimSpace(params.SEPP.TrustAnchorPEM) != "" {
			partner.SEPPTrustAnchorPEM = &params.SEPP.TrustAnchorPEM
		}

		if partner.SEPPFQDN == nil || partner.SEPPN32cURI == nil || partner.SEPPTrustAnchorPEM == nil {
			return db.RoamingPartner{}, http.StatusBadRequest, "sepp needs fqdn, n32cURI and trustAnchorPEM"
		}
	}

	profile, err := dbInstance.GetProfile(ctx, params.ProfileName)
	if err != nil {
		return db.RoamingPartner{}, http.StatusNotFound, "Profile not found"
	}

	partner.ProfileID = profile.ID

	if err := partner.Validate(); err != nil {
		return db.RoamingPartner{}, http.StatusBadRequest, err.Error()
	}

	if partner.DiameterPeerID != nil {
		if _, err := dbInstance.GetRoamingDiameterPeer(ctx, *partner.DiameterPeerID); err != nil {
			return db.RoamingPartner{}, http.StatusNotFound, "Roaming Diameter peer not found"
		}
	}

	return partner, 0, ""
}

func roamingPLMNList(p db.RoamingPartner) string {
	plmns := make([]string, 0, len(p.PLMNs))
	for _, plmn := range p.PLMNs {
		plmns = append(plmns, plmn.String())
	}

	return strings.Join(plmns, " ")
}

func roamingPartnerSummary(p db.RoamingPartner, profileName string) string {
	parts := []string{"name " + p.Name, "PLMNs " + roamingPLMNList(p), "profile " + profileName}

	if p.SEPPFQDN != nil {
		parts = append(parts, "SEPP "+*p.SEPPFQDN+" at "+derefString(p.SEPPN32cURI))
	}

	if p.DiameterPeerID != nil {
		parts = append(parts, "Diameter peer "+*p.DiameterPeerID)
	}

	return strings.Join(parts, ", ")
}

func ListRoamingPartners(dbInstance *db.Database, seppRuntime SEPPRuntime) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		partners, err := dbInstance.ListRoamingPartners(r.Context())
		if err != nil {
			writeError(r.Context(), w, http.StatusInternalServerError, "Failed to list roaming partners", err, logger.APILog)
			return
		}

		names := &profileNames{dbInstance: dbInstance, byID: map[string]string{}}
		resp := ListRoamingPartnersResponse{Items: make([]RoamingPartner, 0, len(partners))}

		st := localSEPPStatus(seppRuntime)

		for _, p := range partners {
			name, err := names.name(r.Context(), p.ProfileID)
			if err != nil {
				writeError(r.Context(), w, http.StatusInternalServerError, "Failed to list roaming partners", err, logger.APILog)
				return
			}

			resp.Items = append(resp.Items, withN32(roamingPartnerResponse(p, name), p, st))
		}

		writeResponse(r.Context(), w, resp, http.StatusOK, logger.APILog)
	})
}

func GetRoamingPartner(dbInstance *db.Database, seppRuntime SEPPRuntime) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		partner, err := dbInstance.GetRoamingPartner(r.Context(), r.PathValue("id"))
		if err != nil {
			writeRoamingError(w, r, "roaming partner", "get", err)
			return
		}

		profile, err := dbInstance.GetProfileByID(r.Context(), partner.ProfileID)
		if err != nil {
			writeError(r.Context(), w, http.StatusInternalServerError, "Failed to get roaming partner", err, logger.APILog)
			return
		}

		writeResponse(r.Context(), w, withN32(roamingPartnerResponse(*partner, profile.Name), *partner, localSEPPStatus(seppRuntime)), http.StatusOK, logger.APILog)
	})
}

func CreateRoamingPartner(dbInstance *db.Database) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		email := getEmailFromContext(r)

		var params RoamingPartnerParams
		if err := json.NewDecoder(r.Body).Decode(&params); err != nil {
			writeError(r.Context(), w, http.StatusBadRequest, "Invalid request data", err, logger.APILog)
			return
		}

		id, err := uuid.NewV7()
		if err != nil {
			writeError(r.Context(), w, http.StatusInternalServerError, "Failed to generate roaming partner id", err, logger.APILog)
			return
		}

		partner, status, message := roamingPartnerFromParams(r.Context(), dbInstance, id.String(), params)
		if message != "" {
			writeError(r.Context(), w, status, message, nil, logger.APILog)
			return
		}

		if err := dbInstance.CreateRoamingPartner(r.Context(), &partner); err != nil {
			writeRoamingError(w, r, "roaming partner", "create", err)
			return
		}

		writeResponse(r.Context(), w, roamingPartnerResponse(partner, params.ProfileName), http.StatusCreated, logger.APILog)

		logger.LogAuditEvent(r.Context(), CreateRoamingPartnerAction, email, getClientIP(r), "Created roaming partner "+partner.ID+" ("+roamingPartnerSummary(partner, params.ProfileName)+")")
	})
}

func UpdateRoamingPartner(dbInstance *db.Database) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		email := getEmailFromContext(r)

		var params RoamingPartnerParams
		if err := json.NewDecoder(r.Body).Decode(&params); err != nil {
			writeError(r.Context(), w, http.StatusBadRequest, "Invalid request data", err, logger.APILog)
			return
		}

		existing, err := dbInstance.GetRoamingPartner(r.Context(), r.PathValue("id"))
		if err != nil {
			writeRoamingError(w, r, "roaming partner", "update", err)
			return
		}

		partner, status, message := roamingPartnerFromParams(r.Context(), dbInstance, r.PathValue("id"), params)
		if message != "" {
			writeError(r.Context(), w, status, message, nil, logger.APILog)
			return
		}

		if err := applyWithdrawingRoamers(r.Context(), dbInstance, *existing, &partner, func(ctx context.Context) error {
			return dbInstance.UpdateRoamingPartner(ctx, &partner)
		}); err != nil {
			writeRoamingError(w, r, "roaming partner", "update", err)
			return
		}

		writeResponse(r.Context(), w, SuccessResponse{Message: "Roaming partner updated successfully"}, http.StatusOK, logger.APILog)

		logger.LogAuditEvent(r.Context(), UpdateRoamingPartnerAction, email, getClientIP(r), "Updated roaming partner "+partner.ID+" ("+roamingPartnerSummary(partner, params.ProfileName)+")")
	})
}

func DeleteRoamingPartner(dbInstance *db.Database) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		email := getEmailFromContext(r)
		id := r.PathValue("id")

		partner, err := dbInstance.GetRoamingPartner(r.Context(), id)
		if err != nil {
			writeRoamingError(w, r, "roaming partner", "delete", err)
			return
		}

		if err := applyWithdrawingRoamers(r.Context(), dbInstance, *partner, nil, func(ctx context.Context) error {
			return dbInstance.DeleteRoamingPartner(ctx, id)
		}); err != nil {
			writeRoamingError(w, r, "roaming partner", "delete", err)
			return
		}

		writeResponse(r.Context(), w, SuccessResponse{Message: "Roaming partner deleted successfully"}, http.StatusOK, logger.APILog)

		logger.LogAuditEvent(r.Context(), DeleteRoamingPartnerAction, email, getClientIP(r), "Deleted roaming partner "+id+" ("+partner.Name+", PLMNs "+roamingPLMNList(*partner)+")")
	})
}

func roamingDiameterPeerResponse(p db.RoamingDiameterPeer) RoamingDiameterPeer {
	return RoamingDiameterPeer{ID: p.ID, DiameterIdentity: p.DiameterIdentity, Address: p.Address, Port: p.Port}
}

func roamingDiameterPeerFromParams(id string, params RoamingDiameterPeerParams) (db.RoamingDiameterPeer, string) {
	peer := db.RoamingDiameterPeer{
		ID:               id,
		DiameterIdentity: strings.TrimSpace(params.DiameterIdentity),
		Port:             params.Port,
	}

	if peer.DiameterIdentity != "" && !db.IsValidDiameterIdentity(peer.DiameterIdentity) {
		return db.RoamingDiameterPeer{}, "diameterIdentity must be a fully qualified domain name, for example hss.epc.mnc001.mcc001.3gppnetwork.org"
	}

	if peer.Port == 0 {
		peer.Port = db.DefaultSMSCPort
	}

	if peer.Port < 1 || peer.Port > 65535 {
		return db.RoamingDiameterPeer{}, "port must be between 1 and 65535"
	}

	addr, err := netip.ParseAddr(strings.TrimSpace(params.Address))
	if err != nil || addr.Zone() != "" || addr.IsUnspecified() {
		return db.RoamingDiameterPeer{}, "address must be an IPv4 or IPv6 address"
	}

	peer.Address = addr.Unmap().String()

	return peer, ""
}

func roamingDiameterPeerSummary(p db.RoamingDiameterPeer) string {
	return fmt.Sprintf("Diameter identity %s, address %s", p.DiameterIdentity, net.JoinHostPort(p.Address, strconv.Itoa(p.Port)))
}

func ListRoamingDiameterPeers(dbInstance *db.Database) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		peers, err := dbInstance.ListRoamingDiameterPeers(r.Context())
		if err != nil {
			writeError(r.Context(), w, http.StatusInternalServerError, "Failed to list roaming Diameter peers", err, logger.APILog)
			return
		}

		resp := ListRoamingDiameterPeersResponse{Items: make([]RoamingDiameterPeer, 0, len(peers))}

		for _, p := range peers {
			resp.Items = append(resp.Items, roamingDiameterPeerResponse(p))
		}

		writeResponse(r.Context(), w, resp, http.StatusOK, logger.APILog)
	})
}

func GetRoamingDiameterPeer(dbInstance *db.Database) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		peer, err := dbInstance.GetRoamingDiameterPeer(r.Context(), r.PathValue("id"))
		if err != nil {
			writeRoamingError(w, r, "roaming Diameter peer", "get", err)
			return
		}

		writeResponse(r.Context(), w, roamingDiameterPeerResponse(*peer), http.StatusOK, logger.APILog)
	})
}

func CreateRoamingDiameterPeer(dbInstance *db.Database) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		email := getEmailFromContext(r)

		var params RoamingDiameterPeerParams
		if err := json.NewDecoder(r.Body).Decode(&params); err != nil {
			writeError(r.Context(), w, http.StatusBadRequest, "Invalid request data", err, logger.APILog)
			return
		}

		id, err := uuid.NewV7()
		if err != nil {
			writeError(r.Context(), w, http.StatusInternalServerError, "Failed to generate roaming Diameter peer id", err, logger.APILog)
			return
		}

		peer, message := roamingDiameterPeerFromParams(id.String(), params)
		if message != "" {
			writeError(r.Context(), w, http.StatusBadRequest, message, nil, logger.APILog)
			return
		}

		if err := dbInstance.CreateRoamingDiameterPeer(r.Context(), &peer); err != nil {
			writeRoamingError(w, r, "roaming Diameter peer", "create", err)
			return
		}

		writeResponse(r.Context(), w, roamingDiameterPeerResponse(peer), http.StatusCreated, logger.APILog)

		logger.LogAuditEvent(r.Context(), CreateRoamingDiameterPeerAction, email, getClientIP(r), "Created roaming Diameter peer "+peer.ID+" ("+roamingDiameterPeerSummary(peer)+")")
	})
}

func UpdateRoamingDiameterPeer(dbInstance *db.Database) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		email := getEmailFromContext(r)

		var params RoamingDiameterPeerParams
		if err := json.NewDecoder(r.Body).Decode(&params); err != nil {
			writeError(r.Context(), w, http.StatusBadRequest, "Invalid request data", err, logger.APILog)
			return
		}

		if params.Port == 0 {
			writeError(r.Context(), w, http.StatusBadRequest, "port is required", nil, logger.APILog)
			return
		}

		peer, message := roamingDiameterPeerFromParams(r.PathValue("id"), params)
		if message != "" {
			writeError(r.Context(), w, http.StatusBadRequest, message, nil, logger.APILog)
			return
		}

		if err := dbInstance.UpdateRoamingDiameterPeer(r.Context(), &peer); err != nil {
			writeRoamingError(w, r, "roaming Diameter peer", "update", err)
			return
		}

		writeResponse(r.Context(), w, SuccessResponse{Message: "Roaming Diameter peer updated successfully"}, http.StatusOK, logger.APILog)

		logger.LogAuditEvent(r.Context(), UpdateRoamingDiameterPeerAction, email, getClientIP(r), "Updated roaming Diameter peer "+peer.ID+" ("+roamingDiameterPeerSummary(peer)+")")
	})
}

func DeleteRoamingDiameterPeer(dbInstance *db.Database) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		email := getEmailFromContext(r)
		id := r.PathValue("id")

		peer, err := dbInstance.GetRoamingDiameterPeer(r.Context(), id)
		if err != nil {
			writeRoamingError(w, r, "roaming Diameter peer", "delete", err)
			return
		}

		if err := dbInstance.DeleteRoamingDiameterPeer(r.Context(), id); err != nil {
			writeRoamingError(w, r, "roaming Diameter peer", "delete", err)
			return
		}

		writeResponse(r.Context(), w, SuccessResponse{Message: "Roaming Diameter peer deleted successfully"}, http.StatusOK, logger.APILog)

		logger.LogAuditEvent(r.Context(), DeleteRoamingDiameterPeerAction, email, getClientIP(r), "Deleted roaming Diameter peer "+id+" ("+roamingDiameterPeerSummary(*peer)+")")
	})
}
