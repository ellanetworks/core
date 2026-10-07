// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/diameternode"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/smsf"
	"github.com/google/uuid"
)

type SMSCPeer struct {
	ID             string          `json:"id"`
	Address        string          `json:"address"`
	Port           int             `json:"port"`
	ServiceCentres []string        `json:"serviceCentres"`
	Status         *SMSCPeerStatus `json:"status,omitempty"`
}

type SMSCPeerStatus struct {
	State string `json:"state"`
	Host  string `json:"host,omitempty"`
	Realm string `json:"realm,omitempty"`
	Since string `json:"since"`
}

type ListSMSCPeersResponse struct {
	Items []SMSCPeer `json:"items"`
}

type SMSCPeerParams struct {
	Address        string   `json:"address"`
	Port           int      `json:"port,omitempty"`
	ServiceCentres []string `json:"serviceCentres"`
}

const (
	CreateSMSCPeerAction = "create_smsc_peer"
	UpdateSMSCPeerAction = "update_smsc_peer"
	DeleteSMSCPeerAction = "delete_smsc_peer"
)

func smscPeerResponse(p db.SMSCPeer, statuses map[string]diameternode.PeerStatus) SMSCPeer {
	centres := make([]string, 0, len(p.ServiceCentres))
	for _, sc := range p.ServiceCentres {
		centres = append(centres, formatE164(sc))
	}

	out := SMSCPeer{
		ID:             p.ID,
		Address:        p.Address,
		Port:           p.Port,
		ServiceCentres: centres,
	}

	if st, ok := statuses[smsf.SMSCPeerID(p.ID)]; ok {
		out.Status = &SMSCPeerStatus{
			State: st.State.String(),
			Host:  st.Host,
			Realm: st.Realm,
			Since: st.Since.UTC().Format(time.RFC3339),
		}
	}

	return out
}

func smscPeerStatuses(node DiameterNode) map[string]diameternode.PeerStatus {
	statuses := map[string]diameternode.PeerStatus{}

	if node == nil {
		return statuses
	}

	for _, st := range node.Peers() {
		statuses[st.ID] = st
	}

	return statuses
}

func smscPeerFromParams(id string, params SMSCPeerParams) (db.SMSCPeer, string) {
	peer := db.SMSCPeer{
		ID:   id,
		Port: params.Port,
	}

	if peer.Port == 0 {
		peer.Port = db.DefaultSMSCPort
	}

	if peer.Port < 1 || peer.Port > 65535 {
		return db.SMSCPeer{}, "port must be between 1 and 65535"
	}

	addr, err := netip.ParseAddr(strings.TrimSpace(params.Address))
	if err != nil || addr.Zone() != "" || addr.IsUnspecified() {
		return db.SMSCPeer{}, "address must be an IPv4 or IPv6 address"
	}

	peer.Address = addr.Unmap().String()

	if len(params.ServiceCentres) == 0 {
		return db.SMSCPeer{}, "serviceCentres needs at least one E.164 number"
	}

	if len(params.ServiceCentres) > db.MaxSMSCPeerServiceCentres {
		return db.SMSCPeer{}, fmt.Sprintf("serviceCentres has at most %d numbers", db.MaxSMSCPeerServiceCentres)
	}

	for _, sc := range params.ServiceCentres {
		number, ok := parseE164(sc)
		if !ok || number == "" {
			return db.SMSCPeer{}, "serviceCentres must be E.164 numbers: + followed by 1 to 15 digits, for example +15550000000"
		}

		if slices.Contains(peer.ServiceCentres, number) {
			return db.SMSCPeer{}, fmt.Sprintf("serviceCentres lists %s twice", formatE164(number))
		}

		peer.ServiceCentres = append(peer.ServiceCentres, number)
	}

	return peer, ""
}

func smscPeerSummary(p db.SMSCPeer) string {
	centres := make([]string, 0, len(p.ServiceCentres))
	for _, sc := range p.ServiceCentres {
		centres = append(centres, formatE164(sc))
	}

	return fmt.Sprintf("address %s, service centres %s", net.JoinHostPort(p.Address, strconv.Itoa(p.Port)), strings.Join(centres, " "))
}

func writeSMSCPeerError(w http.ResponseWriter, r *http.Request, action string, err error) {
	switch {
	case errors.Is(err, db.ErrNotFound):
		writeError(r.Context(), w, http.StatusNotFound, "SMSC peer not found", nil, logger.APILog)
	case errors.Is(err, db.ErrSMSCPeerConflict):
		writeError(r.Context(), w, http.StatusConflict, conflictReason(err), nil, logger.APILog)
	case errors.Is(err, db.ErrAlreadyExists):
		writeError(r.Context(), w, http.StatusConflict, "SMSC peer already exists", nil, logger.APILog)
	default:
		writeError(r.Context(), w, http.StatusInternalServerError, "Failed to "+action+" SMSC peer", err, logger.APILog)
	}
}

func conflictReason(err error) string {
	msg := err.Error()
	prefix := db.ErrSMSCPeerConflict.Error() + ": "

	if i := strings.LastIndex(msg, prefix); i >= 0 {
		return msg[i+len(prefix):]
	}

	return msg
}

func ListSMSCPeers(dbInstance *db.Database, node DiameterNode) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		peers, err := dbInstance.ListSMSCPeers(r.Context())
		if err != nil {
			writeError(r.Context(), w, http.StatusInternalServerError, "Failed to list SMSC peers", err, logger.APILog)
			return
		}

		statuses := smscPeerStatuses(node)
		resp := ListSMSCPeersResponse{Items: make([]SMSCPeer, 0, len(peers))}

		for _, p := range peers {
			resp.Items = append(resp.Items, smscPeerResponse(p, statuses))
		}

		writeResponse(r.Context(), w, resp, http.StatusOK, logger.APILog)
	})
}

func GetSMSCPeer(dbInstance *db.Database, node DiameterNode) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		peer, err := dbInstance.GetSMSCPeer(r.Context(), r.PathValue("id"))
		if err != nil {
			writeSMSCPeerError(w, r, "get", err)
			return
		}

		writeResponse(r.Context(), w, smscPeerResponse(*peer, smscPeerStatuses(node)), http.StatusOK, logger.APILog)
	})
}

func CreateSMSCPeer(dbInstance *db.Database) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		email := getEmailFromContext(r)

		var params SMSCPeerParams
		if err := json.NewDecoder(r.Body).Decode(&params); err != nil {
			writeError(r.Context(), w, http.StatusBadRequest, "Invalid request data", err, logger.APILog)
			return
		}

		id, err := uuid.NewV7()
		if err != nil {
			writeError(r.Context(), w, http.StatusInternalServerError, "Failed to generate SMSC peer id", err, logger.APILog)
			return
		}

		peer, message := smscPeerFromParams(id.String(), params)
		if message != "" {
			writeError(r.Context(), w, http.StatusBadRequest, message, nil, logger.APILog)
			return
		}

		if err := dbInstance.CreateSMSCPeer(r.Context(), &peer); err != nil {
			writeSMSCPeerError(w, r, "create", err)
			return
		}

		writeResponse(r.Context(), w, smscPeerResponse(peer, nil), http.StatusCreated, logger.APILog)

		logger.LogAuditEvent(r.Context(), CreateSMSCPeerAction, email, getClientIP(r), "Created SMSC peer ("+smscPeerSummary(peer)+")")
	})
}

func UpdateSMSCPeer(dbInstance *db.Database) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		email := getEmailFromContext(r)

		var params SMSCPeerParams
		if err := json.NewDecoder(r.Body).Decode(&params); err != nil {
			writeError(r.Context(), w, http.StatusBadRequest, "Invalid request data", err, logger.APILog)
			return
		}

		if params.Port == 0 {
			writeError(r.Context(), w, http.StatusBadRequest, "port is required", nil, logger.APILog)
			return
		}

		peer, message := smscPeerFromParams(r.PathValue("id"), params)
		if message != "" {
			writeError(r.Context(), w, http.StatusBadRequest, message, nil, logger.APILog)
			return
		}

		if err := dbInstance.UpdateSMSCPeer(r.Context(), &peer); err != nil {
			writeSMSCPeerError(w, r, "update", err)
			return
		}

		writeResponse(r.Context(), w, SuccessResponse{Message: "SMSC peer updated successfully"}, http.StatusOK, logger.APILog)

		logger.LogAuditEvent(r.Context(), UpdateSMSCPeerAction, email, getClientIP(r), "Updated SMSC peer "+peer.ID+" ("+smscPeerSummary(peer)+")")
	})
}

func DeleteSMSCPeer(dbInstance *db.Database) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		email := getEmailFromContext(r)
		id := r.PathValue("id")

		peer, err := dbInstance.GetSMSCPeer(r.Context(), id)
		if err != nil {
			writeSMSCPeerError(w, r, "delete", err)
			return
		}

		if err := dbInstance.DeleteSMSCPeer(r.Context(), id); err != nil {
			writeSMSCPeerError(w, r, "delete", err)
			return
		}

		writeResponse(r.Context(), w, SuccessResponse{Message: "SMSC peer deleted successfully"}, http.StatusOK, logger.APILog)

		logger.LogAuditEvent(r.Context(), DeleteSMSCPeerAction, email, getClientIP(r), "Deleted SMSC peer "+id+" ("+smscPeerSummary(*peer)+")")
	})
}
