// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/netip"
	"strings"

	"github.com/ellanetworks/core/diameter"
	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/diameternode"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

const (
	UpdateSMSSettingsAction    = "update_sms_settings"
	UpdateIMSSettingsAction    = "update_ims_settings"
	CreateAllowedIMSPeerAction = "create_allowed_ims_peer"
	UpdateAllowedIMSPeerAction = "update_allowed_ims_peer"
	DeleteAllowedIMSPeerAction = "delete_allowed_ims_peer"
)

type SMSSettings struct {
	ServingNodeNumber string `json:"servingNodeNumber"`
}

type IMSSettings struct {
	PCSCFAddresses    []string `json:"pcscfAddresses"`
	VoiceStatus       string   `json:"voiceStatus"`
	HomeDomain        string   `json:"homeDomain"`
	DerivedHomeDomain string   `json:"derivedHomeDomain"`
	AdvertiseVoice    bool     `json:"advertiseVoice"`
	RestrictPeers     bool     `json:"restrictPeers"`
}

type UpdateIMSSettingsParams struct {
	PCSCFAddresses []string `json:"pcscfAddresses"`
	HomeDomain     string   `json:"homeDomain"`
	AdvertiseVoice bool     `json:"advertiseVoice"`
	RestrictPeers  bool     `json:"restrictPeers"`
}

type AllowedIMSPeer struct {
	Name        string               `json:"name"`
	OriginHost  string               `json:"originHost,omitempty"`
	Realm       string               `json:"realm,omitempty"`
	Addresses   []string             `json:"addresses"`
	Connections []DiameterPeerStatus `json:"connections"`
}

type AllowedIMSPeerParams struct {
	Name       string   `json:"name,omitempty"`
	OriginHost string   `json:"originHost"`
	Realm      string   `json:"realm"`
	Addresses  []string `json:"addresses"`
}

type ListAllowedIMSPeersResponse struct {
	Items []AllowedIMSPeer `json:"items"`
}

func GetSMSSettings(dbInstance *db.Database) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		settings, err := dbInstance.GetSMSSettings(r.Context())
		if err != nil {
			writeError(r.Context(), w, http.StatusInternalServerError, "Failed to get SMS settings", err, logger.APILog)
			return
		}

		writeResponse(r.Context(), w, SMSSettings{ServingNodeNumber: formatE164(settings.SMSNumber)}, http.StatusOK, logger.APILog)
	})
}

func UpdateSMSSettings(dbInstance *db.Database) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		email := getEmailFromContext(r)

		var params SMSSettings
		if err := json.NewDecoder(r.Body).Decode(&params); err != nil {
			writeError(r.Context(), w, http.StatusBadRequest, "Invalid request data", err, logger.APILog)
			return
		}

		number, ok := parseE164(params.ServingNodeNumber)
		if !ok {
			writeError(r.Context(), w, http.StatusBadRequest, "servingNodeNumber must be an E.164 number: + followed by 1 to 15 digits, for example +15550001111", nil, logger.APILog)
			return
		}

		settings := db.SMSSettings{SMSNumber: number}

		if err := settings.Validate(); err != nil {
			writeError(r.Context(), w, http.StatusBadRequest, "Invalid SMS settings", err, logger.APILog)
			return
		}

		if err := dbInstance.UpdateSMSSettings(r.Context(), &settings); err != nil {
			writeError(r.Context(), w, http.StatusInternalServerError, "Failed to update SMS settings", err, logger.APILog)
			return
		}

		writeResponse(r.Context(), w, SuccessResponse{Message: "SMS settings updated successfully"}, http.StatusOK, logger.APILog)

		logger.LogAuditEvent(r.Context(), UpdateSMSSettingsAction, email, getClientIP(r), fmt.Sprintf("User updated SMS settings (serving node number %s)", formatE164(number)))
	})
}

func imsSettingsResponse(ctx context.Context, dbInstance *db.Database) (*IMSSettings, error) {
	pcscfAddresses, err := dbInstance.ListPCSCFAddresses(ctx)
	if err != nil {
		return nil, fmt.Errorf("list P-CSCF addresses: %w", err)
	}

	voiceStatus, err := dbInstance.IMSVoiceStatus(ctx)
	if err != nil {
		return nil, fmt.Errorf("get the voice status: %w", err)
	}

	settings, err := dbInstance.GetIMSSettings(ctx)
	if err != nil {
		return nil, fmt.Errorf("get IMS settings: %w", err)
	}

	domains, err := dbInstance.GetIMSHomeDomains(ctx)
	if err != nil {
		return nil, fmt.Errorf("get IMS home domains: %w", err)
	}

	if pcscfAddresses == nil {
		pcscfAddresses = []string{}
	}

	return &IMSSettings{
		PCSCFAddresses:    pcscfAddresses,
		VoiceStatus:       string(voiceStatus),
		HomeDomain:        settings.HomeDomain,
		DerivedHomeDomain: domains.Derived,
		AdvertiseVoice:    settings.AdvertiseVoice,
		RestrictPeers:     settings.RestrictPeers,
	}, nil
}

func GetIMSSettings(dbInstance *db.Database) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp, err := imsSettingsResponse(r.Context(), dbInstance)
		if err != nil {
			writeError(r.Context(), w, http.StatusInternalServerError, "Failed to get IMS settings", err, logger.APILog)
			return
		}

		writeResponse(r.Context(), w, resp, http.StatusOK, logger.APILog)
	})
}

func pcscfAddressesFromParams(params []string) ([]string, string) {
	addresses := make([]string, 0, len(params))

	for _, s := range params {
		address := strings.TrimSpace(s)

		if addr, err := netip.ParseAddr(address); err == nil {
			address = addr.String()
		} else if diameter.IsValidHostname(address) {
			address = strings.ToLower(address)
		} else {
			return nil, fmt.Sprintf("pcscfAddresses: %q is not an IPv4 or IPv6 address or a host name", s)
		}

		addresses = append(addresses, address)
	}

	if err := db.ValidatePCSCFAddresses(addresses); err != nil {
		return nil, err.Error()
	}

	return addresses, ""
}

func UpdateIMSSettings(dbInstance *db.Database, imsProfiles IMSProfileNotifier) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		email := getEmailFromContext(r)

		var raw map[string]json.RawMessage
		if err := json.NewDecoder(r.Body).Decode(&raw); err != nil {
			writeError(r.Context(), w, http.StatusBadRequest, "Invalid request data", err, logger.APILog)
			return
		}

		_, hasAddresses := raw["pcscfAddresses"]
		_, hasDomain := raw["homeDomain"]
		_, hasVoice := raw["advertiseVoice"]
		_, hasRestrict := raw["restrictPeers"]

		if !hasAddresses || !hasDomain || !hasVoice || !hasRestrict {
			writeError(r.Context(), w, http.StatusBadRequest, "pcscfAddresses, homeDomain, advertiseVoice and restrictPeers are required", nil, logger.APILog)
			return
		}

		var params UpdateIMSSettingsParams
		if err := json.Unmarshal(raw["pcscfAddresses"], &params.PCSCFAddresses); err != nil {
			writeError(r.Context(), w, http.StatusBadRequest, "pcscfAddresses must be a list of strings", err, logger.APILog)
			return
		}

		if err := json.Unmarshal(raw["homeDomain"], &params.HomeDomain); err != nil {
			writeError(r.Context(), w, http.StatusBadRequest, "homeDomain must be a string", err, logger.APILog)
			return
		}

		if err := json.Unmarshal(raw["advertiseVoice"], &params.AdvertiseVoice); err != nil || string(raw["advertiseVoice"]) == "null" {
			writeError(r.Context(), w, http.StatusBadRequest, "advertiseVoice must be true or false", err, logger.APILog)
			return
		}

		if err := json.Unmarshal(raw["restrictPeers"], &params.RestrictPeers); err != nil || string(raw["restrictPeers"]) == "null" {
			writeError(r.Context(), w, http.StatusBadRequest, "restrictPeers must be true or false", err, logger.APILog)
			return
		}

		addresses, message := pcscfAddressesFromParams(params.PCSCFAddresses)
		if message != "" {
			writeError(r.Context(), w, http.StatusBadRequest, message, nil, logger.APILog)
			return
		}

		op, err := dbInstance.GetOperator(r.Context())
		if err != nil {
			writeError(r.Context(), w, http.StatusInternalServerError, "Failed to get operator", err, logger.APILog)
			return
		}

		homeDomain, err := db.NormalizeIMSHomeDomain(params.HomeDomain, op.Mcc, op.Mnc)
		if err != nil {
			writeError(r.Context(), w, http.StatusBadRequest, "homeDomain: "+err.Error(), nil, logger.APILog)
			return
		}

		before, err := imsSettingsResponse(r.Context(), dbInstance)
		if err != nil {
			writeError(r.Context(), w, http.StatusInternalServerError, "Failed to get IMS settings", err, logger.APILog)
			return
		}

		var changes []string

		if strings.Join(before.PCSCFAddresses, ",") != strings.Join(addresses, ",") {
			pcscf := "none"
			if len(addresses) > 0 {
				pcscf = strings.Join(addresses, ", ")
			}

			changes = append(changes, "P-CSCF addresses "+pcscf)
		}

		domainChanged := before.HomeDomain != homeDomain

		if domainChanged {
			shown := homeDomain
			if shown == "" {
				shown = "default (" + before.DerivedHomeDomain + ")"
			}

			changes = append(changes, "home domain "+shown)
		}

		if before.AdvertiseVoice != params.AdvertiseVoice {
			changes = append(changes, fmt.Sprintf("advertise IMS voice %t", params.AdvertiseVoice))
		}

		if before.RestrictPeers != params.RestrictPeers {
			changes = append(changes, fmt.Sprintf("restrict to allowed peers %t", params.RestrictPeers))
		}

		if len(changes) > 0 {
			if err := dbInstance.UpdateIMSConfiguration(r.Context(), &db.IMSSettings{HomeDomain: homeDomain, AdvertiseVoice: params.AdvertiseVoice, RestrictPeers: params.RestrictPeers}, addresses); err != nil {
				logger.APILog.Warn("Failed to update IMS settings", zap.Error(err))
				writeError(r.Context(), w, http.StatusInternalServerError, "Failed to update IMS settings", err, logger.APILog)

				return
			}
		}

		if domainChanged && imsProfiles != nil {
			imsProfiles.AllSubscribersChanged(context.WithoutCancel(r.Context()))
		}

		writeResponse(r.Context(), w, SuccessResponse{Message: "IMS settings updated successfully"}, http.StatusOK, logger.APILog)

		if len(changes) > 0 {
			logger.LogAuditEvent(r.Context(), UpdateIMSSettingsAction, email, getClientIP(r), "User updated IMS settings ("+strings.Join(changes, "; ")+")")
		}
	})
}

func allowedIMSPeerResponse(p db.IMSPeer, node DiameterNode) AllowedIMSPeer {
	resp := AllowedIMSPeer{Name: p.Name, OriginHost: p.OriginHost, Realm: p.Realm, Addresses: p.Addresses, Connections: []DiameterPeerStatus{}}
	rule := p.Rule()

	for _, st := range inboundDiameterPeers(node) {
		if rule.Matches(st.Host, st.Realm, peerRemoteAddr(st)) {
			resp.Connections = append(resp.Connections, *diameterPeerStatus(st))
		}
	}

	return resp
}

func peerRemoteAddr(st diameternode.PeerStatus) netip.Addr {
	addr, err := netip.ParseAddr(st.Address)
	if err != nil {
		return netip.Addr{}
	}

	return addr
}

func allowedIMSPeerFromParams(id, name string, params AllowedIMSPeerParams) (db.IMSPeer, string) {
	peer := db.IMSPeer{
		ID:         id,
		Name:       name,
		OriginHost: strings.ToLower(strings.TrimSpace(params.OriginHost)),
		Realm:      strings.ToLower(strings.TrimSpace(params.Realm)),
	}

	if peer.OriginHost == "" && peer.Realm == "" {
		return db.IMSPeer{}, "originHost or realm is required"
	}

	if _, err := netip.ParseAddr(peer.OriginHost); err == nil || peer.OriginHost != "" && !db.IsValidDiameterIdentity(peer.OriginHost) {
		return db.IMSPeer{}, "originHost must be a fully qualified domain name, for example scscf.ims.mnc001.mcc001.3gppnetwork.org"
	}

	if _, err := netip.ParseAddr(peer.Realm); err == nil || peer.Realm != "" && !db.IsValidDiameterIdentity(peer.Realm) {
		return db.IMSPeer{}, "realm must be a fully qualified domain name, for example ims.mnc001.mcc001.3gppnetwork.org"
	}

	addresses, err := db.NormalizeIMSPeerAddresses(params.Addresses)
	if err != nil {
		return db.IMSPeer{}, "addresses: " + err.Error()
	}

	if len(addresses) == 0 {
		return db.IMSPeer{}, "addresses must list at least one source IP address or CIDR prefix, for example 192.0.2.10 or 192.0.2.0/24"
	}

	peer.Addresses = addresses

	return peer, ""
}

func writeAllowedIMSPeerError(w http.ResponseWriter, r *http.Request, action string, err error) {
	switch {
	case errors.Is(err, db.ErrNotFound):
		writeError(r.Context(), w, http.StatusNotFound, "Allowed IMS peer not found", nil, logger.APILog)
	case errors.Is(err, db.ErrAlreadyExists):
		writeError(r.Context(), w, http.StatusConflict, "Allowed IMS peer already exists", nil, logger.APILog)
	case errors.Is(err, db.ErrIMSPeerConflict):
		msg := err.Error()
		if i := strings.LastIndex(msg, db.ErrIMSPeerConflict.Error()+": "); i >= 0 {
			msg = msg[i+len(db.ErrIMSPeerConflict.Error())+2:]
		}

		writeError(r.Context(), w, http.StatusConflict, msg, nil, logger.APILog)
	default:
		writeError(r.Context(), w, http.StatusInternalServerError, "Failed to "+action+" allowed IMS peer", err, logger.APILog)
	}
}

func allowedIMSPeerSummary(p db.IMSPeer) string {
	var parts []string

	if p.OriginHost != "" {
		parts = append(parts, "Origin-Host "+p.OriginHost)
	}

	if p.Realm != "" {
		parts = append(parts, "realm "+p.Realm)
	}

	parts = append(parts, "addresses "+strings.Join(p.Addresses, " "))

	return strings.Join(parts, ", ")
}

func ListAllowedIMSPeers(dbInstance *db.Database, node DiameterNode) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		peers, err := dbInstance.ListIMSPeers(r.Context())
		if err != nil {
			writeError(r.Context(), w, http.StatusInternalServerError, "Failed to list allowed IMS peers", err, logger.APILog)
			return
		}

		resp := ListAllowedIMSPeersResponse{Items: make([]AllowedIMSPeer, 0, len(peers))}

		for _, p := range peers {
			resp.Items = append(resp.Items, allowedIMSPeerResponse(p, node))
		}

		writeResponse(r.Context(), w, resp, http.StatusOK, logger.APILog)
	})
}

func GetAllowedIMSPeer(dbInstance *db.Database, node DiameterNode) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		peer, err := dbInstance.GetIMSPeer(r.Context(), r.PathValue("name"))
		if err != nil {
			writeAllowedIMSPeerError(w, r, "get", err)
			return
		}

		writeResponse(r.Context(), w, allowedIMSPeerResponse(*peer, node), http.StatusOK, logger.APILog)
	})
}

func CreateAllowedIMSPeer(dbInstance *db.Database) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		email := getEmailFromContext(r)

		var params AllowedIMSPeerParams
		if err := json.NewDecoder(r.Body).Decode(&params); err != nil {
			writeError(r.Context(), w, http.StatusBadRequest, "Invalid request data", err, logger.APILog)
			return
		}

		if !isResourceNameValid(params.Name) || params.Name == "." || params.Name == ".." {
			writeError(r.Context(), w, http.StatusBadRequest, "name is required, must be less than 256 characters, and cannot be . or ..", nil, logger.APILog)
			return
		}

		id, err := uuid.NewV7()
		if err != nil {
			writeError(r.Context(), w, http.StatusInternalServerError, "Failed to generate allowed IMS peer id", err, logger.APILog)
			return
		}

		peer, message := allowedIMSPeerFromParams(id.String(), params.Name, params)
		if message != "" {
			writeError(r.Context(), w, http.StatusBadRequest, message, nil, logger.APILog)
			return
		}

		if err := dbInstance.CreateIMSPeer(r.Context(), &peer); err != nil {
			writeAllowedIMSPeerError(w, r, "create", err)
			return
		}

		writeResponse(r.Context(), w, allowedIMSPeerResponse(peer, nil), http.StatusCreated, logger.APILog)

		logger.LogAuditEvent(r.Context(), CreateAllowedIMSPeerAction, email, getClientIP(r), "Created allowed IMS peer "+peer.Name+" ("+allowedIMSPeerSummary(peer)+")")
	})
}

func UpdateAllowedIMSPeer(dbInstance *db.Database) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		email := getEmailFromContext(r)

		var params AllowedIMSPeerParams
		if err := json.NewDecoder(r.Body).Decode(&params); err != nil {
			writeError(r.Context(), w, http.StatusBadRequest, "Invalid request data", err, logger.APILog)
			return
		}

		existing, err := dbInstance.GetIMSPeer(r.Context(), r.PathValue("name"))
		if err != nil {
			writeAllowedIMSPeerError(w, r, "update", err)
			return
		}

		peer, message := allowedIMSPeerFromParams(existing.ID, existing.Name, params)
		if message != "" {
			writeError(r.Context(), w, http.StatusBadRequest, message, nil, logger.APILog)
			return
		}

		if err := dbInstance.UpdateIMSPeer(r.Context(), &peer); err != nil {
			writeAllowedIMSPeerError(w, r, "update", err)
			return
		}

		writeResponse(r.Context(), w, SuccessResponse{Message: "Allowed IMS peer updated successfully"}, http.StatusOK, logger.APILog)

		logger.LogAuditEvent(r.Context(), UpdateAllowedIMSPeerAction, email, getClientIP(r), "Updated allowed IMS peer "+peer.Name+" ("+allowedIMSPeerSummary(peer)+")")
	})
}

func DeleteAllowedIMSPeer(dbInstance *db.Database) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		email := getEmailFromContext(r)

		existing, err := dbInstance.GetIMSPeer(r.Context(), r.PathValue("name"))
		if err != nil {
			writeAllowedIMSPeerError(w, r, "delete", err)
			return
		}

		if err := dbInstance.DeleteIMSPeer(r.Context(), existing.ID); err != nil {
			writeAllowedIMSPeerError(w, r, "delete", err)
			return
		}

		writeResponse(r.Context(), w, SuccessResponse{Message: "Allowed IMS peer deleted successfully"}, http.StatusOK, logger.APILog)

		logger.LogAuditEvent(r.Context(), DeleteAllowedIMSPeerAction, email, getClientIP(r), "Deleted allowed IMS peer "+existing.Name)
	})
}
