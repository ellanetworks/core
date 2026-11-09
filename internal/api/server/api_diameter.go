// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package server

import (
	"context"
	"net/http"
	"time"

	"github.com/ellanetworks/core/internal/config"
	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/diameternode"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/smsf"
	"go.uber.org/zap"
)

const (
	DiameterPeerOwnerSMSC                = "smsc"
	DiameterPeerOwnerIMSPeer             = "imsPeer"
	DiameterPeerOwnerRoamingDiameterPeer = "roamingDiameterPeer"
)

type DiameterNode interface {
	Identity(ctx context.Context) (diameternode.Identity, error)
	Peers() []diameternode.PeerStatus
}

type DiameterPeerStatus struct {
	State      string `json:"state"`
	OriginHost string `json:"originHost,omitempty"`
	Realm      string `json:"realm,omitempty"`
	Since      string `json:"since"`
	Error      string `json:"error,omitempty"`
}

type DiameterNodeIdentity struct {
	NodeID      string `json:"nodeId"`
	DisplayName string `json:"displayName,omitempty"`
	OriginHost  string `json:"originHost"`
	Local       bool   `json:"local"`
}

type DiameterListener struct {
	Enabled   bool     `json:"enabled"`
	Addresses []string `json:"addresses"`
	Port      int      `json:"port,omitempty"`
}

type DiameterPeerOwner struct {
	Kind string `json:"kind"`
	Name string `json:"name"`
}

type DiameterPeer struct {
	ID         string             `json:"id"`
	Role       string             `json:"role"`
	Owner      *DiameterPeerOwner `json:"owner,omitempty"`
	OriginHost string             `json:"originHost,omitempty"`
	Realm      string             `json:"realm,omitempty"`
	Address    string             `json:"address"`
	Port       int                `json:"port,omitempty"`
	Direction  string             `json:"direction"`
	State      string             `json:"state"`
	Since      string             `json:"since"`
	Error      string             `json:"error,omitempty"`
}

type DiameterStatus struct {
	Realm        string                 `json:"realm,omitempty"`
	ServedRealms []string               `json:"servedRealms"`
	Nodes        []DiameterNodeIdentity `json:"nodes"`
	Listener     DiameterListener       `json:"listener"`
	Peers        []DiameterPeer         `json:"peers"`
}

func diameterPeerStatus(st diameternode.PeerStatus) *DiameterPeerStatus {
	return &DiameterPeerStatus{
		State:      st.State.String(),
		OriginHost: st.Host,
		Realm:      st.Realm,
		Since:      st.Since.UTC().Format(time.RFC3339),
		Error:      st.Error,
	}
}

func diameterPeerStatuses(node DiameterNode) map[string]diameternode.PeerStatus {
	statuses := map[string]diameternode.PeerStatus{}

	if node == nil {
		return statuses
	}

	for _, st := range node.Peers() {
		statuses[st.ID] = st
	}

	return statuses
}

func inboundDiameterPeers(node DiameterNode) []diameternode.PeerStatus {
	if node == nil {
		return nil
	}

	var out []diameternode.PeerStatus

	for _, st := range node.Peers() {
		if st.Inbound {
			out = append(out, st)
		}
	}

	return out
}

func GetDiameterStatus(dbInstance *db.Database, cfg config.Config, node DiameterNode) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		resp := DiameterStatus{ServedRealms: []string{}, Nodes: []DiameterNodeIdentity{}, Peers: []DiameterPeer{}, Listener: DiameterListener{Addresses: []string{}}}

		if cfg.Interfaces.Diameter.Enabled {
			resp.Listener = DiameterListener{Enabled: true, Addresses: []string{cfg.Interfaces.Diameter.Address}, Port: cfg.Interfaces.Diameter.Port}

			if cfg.Interfaces.Diameter.Name != "" {
				ips, err := config.GetInterfaceIPs(cfg.Interfaces.Diameter.Name)
				if err != nil {
					logger.APILog.Warn("Failed to get Diameter interface IPs", zap.String("interface", cfg.Interfaces.Diameter.Name), zap.Error(err))

					ips = []string{}
				}

				resp.Listener.Addresses = ips
			}
		}

		op, err := dbInstance.GetOperator(ctx)
		if err != nil {
			writeError(ctx, w, http.StatusInternalServerError, "Failed to get operator", err, logger.APILog)
			return
		}

		imsSettings, err := dbInstance.GetIMSSettings(ctx)
		if err != nil {
			writeError(ctx, w, http.StatusInternalServerError, "Failed to get IMS settings", err, logger.APILog)
			return
		}

		if realm, err := diameternode.DiameterRealm(op.Mcc, op.Mnc); err == nil {
			resp.Realm = realm
		}

		if realms, err := diameternode.ServedRealms(op.Mcc, op.Mnc, imsSettings.HomeDomain); err == nil {
			resp.ServedRealms = realms
		}

		nodes, err := diameterNodeIdentities(ctx, dbInstance, op)
		if err != nil {
			writeError(ctx, w, http.StatusInternalServerError, "Failed to list cluster members", err, logger.APILog)
			return
		}

		resp.Nodes = nodes

		if node == nil {
			writeResponse(ctx, w, resp, http.StatusOK, logger.APILog)
			return
		}

		owners, err := diameterPeerOwners(ctx, dbInstance)
		if err != nil {
			writeError(ctx, w, http.StatusInternalServerError, "Failed to list Diameter peers", err, logger.APILog)
			return
		}

		imsPeers, err := dbInstance.ListIMSPeers(ctx)
		if err != nil {
			writeError(ctx, w, http.StatusInternalServerError, "Failed to list allowed IMS peers", err, logger.APILog)
			return
		}

		for _, p := range node.Peers() {
			peer := DiameterPeer{
				ID:         p.ID,
				Role:       p.Role,
				OriginHost: p.Host,
				Realm:      p.Realm,
				Address:    p.Address,
				Port:       int(p.Port),
				Direction:  "outbound",
				State:      p.State.String(),
				Since:      p.Since.UTC().Format(time.RFC3339),
				Error:      p.Error,
			}

			if p.Inbound {
				peer.Direction = "inbound"

				for _, ip := range imsPeers {
					if ip.Rule().Matches(p.Host, p.Realm, peerRemoteAddr(p)) {
						peer.Owner = &DiameterPeerOwner{Kind: DiameterPeerOwnerIMSPeer, Name: ip.Name}
						break
					}
				}
			} else if owner, ok := owners[p.ID]; ok {
				peer.Owner = &owner
			}

			resp.Peers = append(resp.Peers, peer)
		}

		writeResponse(ctx, w, resp, http.StatusOK, logger.APILog)
	})
}

func diameterNodeIdentities(ctx context.Context, dbInstance *db.Database, op *db.Operator) ([]DiameterNodeIdentity, error) {
	members, err := dbInstance.ListClusterMembers(ctx)
	if err != nil {
		return nil, err
	}

	localID := dbInstance.RaftID()

	if len(members) == 0 && dbInstance.AMFPointer() > 0 {
		members = []db.ClusterMember{{NodeID: localID, AMFPointer: dbInstance.AMFPointer()}}
	}

	out := make([]DiameterNodeIdentity, 0, len(members))

	for _, m := range members {
		if m.AMFPointer < 1 {
			continue
		}

		identity, err := diameternode.IdentityOf(diameternode.NodeSettingsOf(op.Mcc, op.Mnc, op.GUAMIRegionID(), uint16(op.AmfSetID), uint8(m.AMFPointer))) // #nosec G115 -- AMF Set ID bounded to 10 bits by the operator API, pointer to [1, 63]
		if err != nil {
			continue
		}

		out = append(out, DiameterNodeIdentity{NodeID: m.NodeID, DisplayName: m.DisplayName, OriginHost: identity.Host, Local: m.NodeID == localID})
	}

	return out, nil
}

func diameterPeerOwners(ctx context.Context, dbInstance *db.Database) (map[string]DiameterPeerOwner, error) {
	owners := map[string]DiameterPeerOwner{}

	smscs, err := dbInstance.ListSMSCs(ctx)
	if err != nil {
		return nil, err
	}

	for _, s := range smscs {
		for _, n := range s.Nodes {
			owners[smsf.SMSCPeerID(n.ID)] = DiameterPeerOwner{Kind: DiameterPeerOwnerSMSC, Name: s.Name}
		}
	}

	roamingPeers, err := dbInstance.ListRoamingDiameterPeers(ctx)
	if err != nil {
		return nil, err
	}

	for _, p := range roamingPeers {
		name := p.DiameterIdentity
		if name == "" {
			name = p.Address
		}

		owners[roamingDiameterPeerID(p.ID)] = DiameterPeerOwner{Kind: DiameterPeerOwnerRoamingDiameterPeer, Name: name}
	}

	return owners, nil
}

func roamingDiameterPeerID(id string) string {
	return diameternode.PeerRoleRoaming + "-" + id
}
