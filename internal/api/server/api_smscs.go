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

type SMSC struct {
	Name           string     `json:"name"`
	ServiceCentres []string   `json:"serviceCentres"`
	Nodes          []SMSCNode `json:"nodes"`
}

type SMSCNode struct {
	DiameterIdentity string          `json:"diameterIdentity"`
	Address          string          `json:"address"`
	Port             int             `json:"port"`
	Status           *SMSCNodeStatus `json:"status,omitempty"`
}

type SMSCNodeStatus struct {
	State string `json:"state"`
	Host  string `json:"host,omitempty"`
	Realm string `json:"realm,omitempty"`
	Since string `json:"since"`
	Error string `json:"error,omitempty"`
}

type ListSMSCsResponse struct {
	Items []SMSC `json:"items"`
}

type SMSCNodeParams struct {
	DiameterIdentity string `json:"diameterIdentity"`
	Address          string `json:"address"`
	Port             int    `json:"port,omitempty"`
}

type CreateSMSCParams struct {
	Name           string           `json:"name"`
	ServiceCentres []string         `json:"serviceCentres"`
	Nodes          []SMSCNodeParams `json:"nodes"`
}

type UpdateSMSCParams struct {
	ServiceCentres []string         `json:"serviceCentres"`
	Nodes          []SMSCNodeParams `json:"nodes"`
}

const (
	CreateSMSCAction = "create_smsc"
	UpdateSMSCAction = "update_smsc"
	DeleteSMSCAction = "delete_smsc"
)

func smscResponse(s db.SMSC, statuses map[string]diameternode.PeerStatus) SMSC {
	centres := make([]string, 0, len(s.ServiceCentres))
	for _, sc := range s.ServiceCentres {
		centres = append(centres, formatE164(sc))
	}

	nodes := make([]SMSCNode, 0, len(s.Nodes))

	for _, n := range s.Nodes {
		node := SMSCNode{DiameterIdentity: n.DiameterIdentity, Address: n.Address, Port: n.Port}

		if st, ok := statuses[smsf.SMSCPeerID(n.ID)]; ok {
			node.Status = &SMSCNodeStatus{
				State: st.State.String(),
				Host:  st.Host,
				Realm: st.Realm,
				Since: st.Since.UTC().Format(time.RFC3339),
				Error: st.Error,
			}
		}

		nodes = append(nodes, node)
	}

	return SMSC{Name: s.Name, ServiceCentres: centres, Nodes: nodes}
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

func smscFromParams(smsc *db.SMSC, centres []string, nodes []SMSCNodeParams, existing []db.SMSCNode) string {
	if len(centres) == 0 {
		return "serviceCentres needs at least one E.164 number"
	}

	if len(centres) > db.MaxSMSCServiceCentres {
		return fmt.Sprintf("serviceCentres has at most %d numbers", db.MaxSMSCServiceCentres)
	}

	for _, sc := range centres {
		number, ok := parseE164(sc)
		if !ok || number == "" {
			return "serviceCentres must be E.164 numbers: + followed by 1 to 15 digits, for example +15550000000"
		}

		if slices.Contains(smsc.ServiceCentres, number) {
			return fmt.Sprintf("serviceCentres lists %s twice", formatE164(number))
		}

		smsc.ServiceCentres = append(smsc.ServiceCentres, number)
	}

	if len(nodes) == 0 {
		return "nodes needs at least one SMSC node"
	}

	if len(nodes) > db.MaxSMSCNodes {
		return fmt.Sprintf("nodes has at most %d SMSC nodes", db.MaxSMSCNodes)
	}

	for i, params := range nodes {
		node, message := smscNodeFromParams(params)
		if message != "" {
			return fmt.Sprintf("nodes[%d]: %s", i, message)
		}

		if j := slices.IndexFunc(existing, func(n db.SMSCNode) bool { return n.Address == node.Address && n.Port == node.Port }); j >= 0 {
			node.ID = existing[j].ID
		} else {
			id, err := uuid.NewV7()
			if err != nil {
				return "could not generate an SMSC node id"
			}

			node.ID = id.String()
		}

		smsc.Nodes = append(smsc.Nodes, node)
	}

	return ""
}

func smscNodeFromParams(params SMSCNodeParams) (db.SMSCNode, string) {
	node := db.SMSCNode{
		DiameterIdentity: strings.TrimSpace(params.DiameterIdentity),
		Port:             params.Port,
	}

	if node.DiameterIdentity != "" && !db.IsValidDiameterIdentity(node.DiameterIdentity) {
		return db.SMSCNode{}, "diameterIdentity must be a fully qualified domain name, for example smsc.example.org"
	}

	if node.Port == 0 {
		node.Port = db.DefaultSMSCPort
	}

	if node.Port < 1 || node.Port > 65535 {
		return db.SMSCNode{}, "port must be between 1 and 65535"
	}

	addr, err := netip.ParseAddr(strings.TrimSpace(params.Address))
	if err != nil || addr.Zone() != "" || addr.IsUnspecified() {
		return db.SMSCNode{}, "address must be an IPv4 or IPv6 address"
	}

	node.Address = addr.Unmap().String()

	return node, ""
}

func smscSummary(s db.SMSC) string {
	centres := make([]string, 0, len(s.ServiceCentres))
	for _, sc := range s.ServiceCentres {
		centres = append(centres, formatE164(sc))
	}

	nodes := make([]string, 0, len(s.Nodes))
	for _, n := range s.Nodes {
		nodes = append(nodes, strings.TrimSpace(n.DiameterIdentity+" "+net.JoinHostPort(n.Address, strconv.Itoa(n.Port))))
	}

	return fmt.Sprintf("service centres %s, nodes %s", strings.Join(centres, " "), strings.Join(nodes, ", "))
}

func writeSMSCError(w http.ResponseWriter, r *http.Request, action string, err error) {
	switch {
	case errors.Is(err, db.ErrNotFound):
		writeError(r.Context(), w, http.StatusNotFound, "SMSC not found", nil, logger.APILog)
	case errors.Is(err, db.ErrSMSCConflict):
		writeError(r.Context(), w, http.StatusConflict, conflictReason(err), nil, logger.APILog)
	case errors.Is(err, db.ErrAlreadyExists):
		writeError(r.Context(), w, http.StatusConflict, "an SMSC with this name already exists", nil, logger.APILog)
	default:
		writeError(r.Context(), w, http.StatusInternalServerError, "Failed to "+action+" SMSC", err, logger.APILog)
	}
}

func conflictReason(err error) string {
	msg := err.Error()
	prefix := db.ErrSMSCConflict.Error() + ": "

	if i := strings.LastIndex(msg, prefix); i >= 0 {
		return msg[i+len(prefix):]
	}

	return msg
}

func ListSMSCs(dbInstance *db.Database, node DiameterNode) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		smscs, err := dbInstance.ListSMSCs(r.Context())
		if err != nil {
			writeError(r.Context(), w, http.StatusInternalServerError, "Failed to list SMSCs", err, logger.APILog)
			return
		}

		statuses := smscPeerStatuses(node)
		resp := ListSMSCsResponse{Items: make([]SMSC, 0, len(smscs))}

		for _, s := range smscs {
			resp.Items = append(resp.Items, smscResponse(s, statuses))
		}

		writeResponse(r.Context(), w, resp, http.StatusOK, logger.APILog)
	})
}

func GetSMSC(dbInstance *db.Database, node DiameterNode) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		smsc, err := dbInstance.GetSMSC(r.Context(), r.PathValue("name"))
		if err != nil {
			writeSMSCError(w, r, "get", err)
			return
		}

		writeResponse(r.Context(), w, smscResponse(*smsc, smscPeerStatuses(node)), http.StatusOK, logger.APILog)
	})
}

func CreateSMSC(dbInstance *db.Database) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		email := getEmailFromContext(r)

		var params CreateSMSCParams
		if err := json.NewDecoder(r.Body).Decode(&params); err != nil {
			writeError(r.Context(), w, http.StatusBadRequest, "Invalid request data", err, logger.APILog)
			return
		}

		if !isResourceNameValid(params.Name) {
			writeError(r.Context(), w, http.StatusBadRequest, "name is required and must be less than 256 characters", nil, logger.APILog)
			return
		}

		id, err := uuid.NewV7()
		if err != nil {
			writeError(r.Context(), w, http.StatusInternalServerError, "Failed to generate SMSC id", err, logger.APILog)
			return
		}

		smsc := db.SMSC{ID: id.String(), Name: params.Name}

		if message := smscFromParams(&smsc, params.ServiceCentres, params.Nodes, nil); message != "" {
			writeError(r.Context(), w, http.StatusBadRequest, message, nil, logger.APILog)
			return
		}

		if err := dbInstance.CreateSMSC(r.Context(), &smsc); err != nil {
			writeSMSCError(w, r, "create", err)
			return
		}

		writeResponse(r.Context(), w, smscResponse(smsc, nil), http.StatusCreated, logger.APILog)

		logger.LogAuditEvent(r.Context(), CreateSMSCAction, email, getClientIP(r), "Created SMSC "+smsc.Name+" ("+smscSummary(smsc)+")")
	})
}

func UpdateSMSC(dbInstance *db.Database) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		email := getEmailFromContext(r)

		var params UpdateSMSCParams
		if err := json.NewDecoder(r.Body).Decode(&params); err != nil {
			writeError(r.Context(), w, http.StatusBadRequest, "Invalid request data", err, logger.APILog)
			return
		}

		existing, err := dbInstance.GetSMSC(r.Context(), r.PathValue("name"))
		if err != nil {
			writeSMSCError(w, r, "update", err)
			return
		}

		smsc := db.SMSC{ID: existing.ID, Name: existing.Name}

		if message := smscFromParams(&smsc, params.ServiceCentres, params.Nodes, existing.Nodes); message != "" {
			writeError(r.Context(), w, http.StatusBadRequest, message, nil, logger.APILog)
			return
		}

		if err := dbInstance.UpdateSMSC(r.Context(), &smsc); err != nil {
			writeSMSCError(w, r, "update", err)
			return
		}

		writeResponse(r.Context(), w, SuccessResponse{Message: "SMSC updated successfully"}, http.StatusOK, logger.APILog)

		logger.LogAuditEvent(r.Context(), UpdateSMSCAction, email, getClientIP(r), "Updated SMSC "+smsc.Name+" ("+smscSummary(smsc)+")")
	})
}

func DeleteSMSC(dbInstance *db.Database) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		email := getEmailFromContext(r)

		smsc, err := dbInstance.GetSMSC(r.Context(), r.PathValue("name"))
		if err != nil {
			writeSMSCError(w, r, "delete", err)
			return
		}

		if err := dbInstance.DeleteSMSC(r.Context(), smsc.ID); err != nil {
			writeSMSCError(w, r, "delete", err)
			return
		}

		writeResponse(r.Context(), w, SuccessResponse{Message: "SMSC deleted successfully"}, http.StatusOK, logger.APILog)

		logger.LogAuditEvent(r.Context(), DeleteSMSCAction, email, getClientIP(r), "Deleted SMSC "+smsc.Name+" ("+smscSummary(*smsc)+")")
	})
}
