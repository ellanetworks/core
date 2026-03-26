package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strconv"

	"github.com/ellanetworks/core/internal/bgp"
	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/logger"
	"go.uber.org/zap"
)

// BGP Settings types

type GetBGPSettingsResponse struct {
	Enabled       bool   `json:"enabled"`
	LocalAS       int    `json:"localAS"`
	RouterID      string `json:"routerID"`
	ListenAddress string `json:"listenAddress"`
}

type UpdateBGPSettingsParams struct {
	Enabled       bool   `json:"enabled"`
	LocalAS       int    `json:"localAS"`
	RouterID      string `json:"routerID"`
	ListenAddress string `json:"listenAddress"`
}

// BGP Peers types

type BGPPeer struct {
	ID           int    `json:"id"`
	Address      string `json:"address"`
	RemoteAS     int    `json:"remoteAS"`
	HoldTime     int    `json:"holdTime"`
	Password     string `json:"password"`
	Description  string `json:"description"`
	State        string `json:"state,omitempty"`
	Uptime       string `json:"uptime,omitempty"`
	PrefixesSent int    `json:"prefixesSent,omitempty"`
}

type CreateBGPPeerParams struct {
	Address     string `json:"address"`
	RemoteAS    int    `json:"remoteAS"`
	HoldTime    int    `json:"holdTime"`
	Password    string `json:"password"`
	Description string `json:"description"`
}

type ListBGPPeersResponse struct {
	Items      []BGPPeer `json:"items"`
	Page       int       `json:"page"`
	PerPage    int       `json:"per_page"`
	TotalCount int       `json:"total_count"`
}

type BGPRoutesResponse struct {
	Routes []bgp.BGPRoute `json:"routes"`
}

// Audit log action constants

const (
	UpdateBGPSettingsAction = "update_bgp_settings"
	CreateBGPPeerAction     = "create_bgp_peer"
	DeleteBGPPeerAction     = "delete_bgp_peer"
)

const MaxNumBGPPeers = 5

// maskedPassword returns a masked representation if the password is set.
const maskedPassword = "********"

// BGP Settings handlers

func GetBGPSettings(dbInstance *db.Database, bgpService *bgp.BGPService) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		settings, err := dbInstance.GetBGPSettings(r.Context())
		if err != nil {
			writeError(r.Context(), w, http.StatusInternalServerError, "Failed to get BGP settings", err, logger.APILog)
			return
		}

		routerID := settings.RouterID
		if routerID == "" && bgpService != nil {
			routerID = bgpService.GetEffectiveRouterID("")
		}

		resp := GetBGPSettingsResponse{
			Enabled:       settings.Enabled,
			LocalAS:       settings.LocalAS,
			RouterID:      routerID,
			ListenAddress: settings.ListenAddress,
		}

		writeResponse(r.Context(), w, resp, http.StatusOK, logger.APILog)
	})
}

func UpdateBGPSettings(dbInstance *db.Database, bgpService *bgp.BGPService) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		email, ok := r.Context().Value(contextKeyEmail).(string)
		if !ok {
			writeError(r.Context(), w, http.StatusInternalServerError, "Failed to get email", nil, logger.APILog)
			return
		}

		var params UpdateBGPSettingsParams
		if err := json.NewDecoder(r.Body).Decode(&params); err != nil {
			writeError(r.Context(), w, http.StatusBadRequest, "Invalid request data", err, logger.APILog)
			return
		}

		if params.LocalAS < 1 || params.LocalAS > 4294967295 {
			writeError(r.Context(), w, http.StatusBadRequest, "localAS must be between 1 and 4294967295", nil, logger.APILog)
			return
		}

		if params.RouterID != "" {
			if net.ParseIP(params.RouterID) == nil {
				writeError(r.Context(), w, http.StatusBadRequest, "routerID must be a valid IPv4 address or empty", nil, logger.APILog)
				return
			}
		} else if bgpService != nil {
			params.RouterID = bgpService.GetEffectiveRouterID("")
		}

		if params.ListenAddress == "" {
			params.ListenAddress = ":179"
		}

		if _, _, err := net.SplitHostPort(params.ListenAddress); err != nil {
			writeError(r.Context(), w, http.StatusBadRequest, "listenAddress must be a valid host:port or :port string", nil, logger.APILog)
			return
		}

		if params.Enabled {
			natEnabled, err := dbInstance.IsNATEnabled(r.Context())
			if err != nil {
				writeError(r.Context(), w, http.StatusInternalServerError, "Failed to check NAT settings", err, logger.APILog)
				return
			}

			if natEnabled {
				writeError(r.Context(), w, http.StatusConflict, "BGP and NAT cannot be enabled simultaneously. Disable NAT first.", nil, logger.APILog)
				return
			}
		}

		// Get previous settings to determine what changed
		prevSettings, err := dbInstance.GetBGPSettings(r.Context())
		if err != nil {
			writeError(r.Context(), w, http.StatusInternalServerError, "Failed to get current BGP settings", err, logger.APILog)
			return
		}

		settings := &db.BGPSettings{
			Enabled:       params.Enabled,
			LocalAS:       params.LocalAS,
			RouterID:      params.RouterID,
			ListenAddress: params.ListenAddress,
		}

		if err := dbInstance.UpdateBGPSettings(r.Context(), settings); err != nil {
			writeError(r.Context(), w, http.StatusInternalServerError, "Failed to update BGP settings", err, logger.APILog)
			return
		}

		// Apply changes to the live BGP speaker
		if bgpService != nil {
			if err := applyBGPSettingsChange(r.Context(), dbInstance, bgpService, prevSettings.Enabled, params.Enabled); err != nil {
				// Rollback DB on failure
				rollbackSettings := &db.BGPSettings{
					Enabled:       prevSettings.Enabled,
					LocalAS:       prevSettings.LocalAS,
					RouterID:      prevSettings.RouterID,
					ListenAddress: prevSettings.ListenAddress,
				}
				_ = dbInstance.UpdateBGPSettings(r.Context(), rollbackSettings)

				writeError(r.Context(), w, http.StatusInternalServerError, "Failed to apply BGP settings: "+err.Error(), err, logger.APILog)

				return
			}
		}

		writeResponse(r.Context(), w, SuccessResponse{Message: "BGP settings updated successfully"}, http.StatusOK, logger.APILog)

		logger.LogAuditEvent(
			r.Context(),
			UpdateBGPSettingsAction,
			email,
			getClientIP(r),
			fmt.Sprintf("BGP settings updated: enabled=%t, localAS=%d, routerID=%s, listenAddress=%s", params.Enabled, params.LocalAS, params.RouterID, params.ListenAddress),
		)
	})
}

// DBSettingsToBGPSettings converts database BGP settings to BGP service settings.
func DBSettingsToBGPSettings(s *db.BGPSettings) bgp.BGPSettings {
	return bgp.BGPSettings{
		Enabled:       s.Enabled,
		LocalAS:       s.LocalAS,
		RouterID:      s.RouterID,
		ListenAddress: s.ListenAddress,
	}
}

func applyBGPSettingsChange(ctx context.Context, dbInstance *db.Database, bgpService *bgp.BGPService, wasEnabled, nowEnabled bool) error {
	switch {
	case !wasEnabled && nowEnabled:
		// Start the BGP speaker
		settings, err := dbInstance.GetBGPSettings(ctx)
		if err != nil {
			return fmt.Errorf("failed to read BGP settings: %w", err)
		}

		dbPeers, err := dbInstance.ListAllBGPPeers(ctx)
		if err != nil {
			return fmt.Errorf("failed to list BGP peers: %w", err)
		}

		allocatedIPs, err := dbInstance.ListAllocatedIPs(ctx)
		if err != nil {
			return fmt.Errorf("failed to list allocated IPs: %w", err)
		}

		bgpPeers := DBPeersToBGPPeers(dbPeers)

		return bgpService.Start(ctx, DBSettingsToBGPSettings(settings), bgpPeers, allocatedIPs)

	case wasEnabled && !nowEnabled:
		// Stop the BGP speaker
		return bgpService.Stop()

	case wasEnabled && nowEnabled:
		// Reconfigure (AS/RouterID/ListenAddress may have changed)
		settings, err := dbInstance.GetBGPSettings(ctx)
		if err != nil {
			return fmt.Errorf("failed to read BGP settings: %w", err)
		}

		dbPeers, err := dbInstance.ListAllBGPPeers(ctx)
		if err != nil {
			return fmt.Errorf("failed to list BGP peers: %w", err)
		}

		bgpPeers := DBPeersToBGPPeers(dbPeers)

		return bgpService.Reconfigure(ctx, DBSettingsToBGPSettings(settings), bgpPeers)
	}

	return nil
}

// DBPeersToBGPPeers converts database BGP peer records to BGP service peer configs.
func DBPeersToBGPPeers(dbPeers []db.BGPPeer) []bgp.BGPPeer {
	peers := make([]bgp.BGPPeer, len(dbPeers))
	for i, p := range dbPeers {
		peers[i] = bgp.BGPPeer{
			ID:          p.ID,
			Address:     p.Address,
			RemoteAS:    p.RemoteAS,
			HoldTime:    p.HoldTime,
			Password:    p.Password,
			Description: p.Description,
		}
	}

	return peers
}

func reconfigureBGPPeers(ctx context.Context, dbInstance *db.Database, bgpService *bgp.BGPService) {
	if bgpService == nil || !bgpService.IsRunning() {
		return
	}

	settings, err := dbInstance.GetBGPSettings(ctx)
	if err != nil {
		logger.APILog.Warn("failed to read BGP settings for peer reconfigure", zap.Error(err))
		return
	}

	dbPeers, err := dbInstance.ListAllBGPPeers(ctx)
	if err != nil {
		logger.APILog.Warn("failed to list BGP peers for reconfigure", zap.Error(err))
		return
	}

	bgpPeers := DBPeersToBGPPeers(dbPeers)

	if err := bgpService.Reconfigure(ctx, DBSettingsToBGPSettings(settings), bgpPeers); err != nil {
		logger.APILog.Warn("failed to reconfigure BGP after peer change", zap.Error(err))
	}
}

// BGP Peers handlers

func ListBGPPeers(dbInstance *db.Database, bgpService *bgp.BGPService) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		page := atoiDefault(q.Get("page"), 1)
		perPage := atoiDefault(q.Get("per_page"), 25)

		if page < 1 {
			writeError(r.Context(), w, http.StatusBadRequest, "page must be >= 1", nil, logger.APILog)
			return
		}

		if perPage < 1 || perPage > 100 {
			writeError(r.Context(), w, http.StatusBadRequest, "per_page must be between 1 and 100", nil, logger.APILog)
			return
		}

		dbPeers, total, err := dbInstance.ListBGPPeersPage(r.Context(), page, perPage)
		if err != nil {
			writeError(r.Context(), w, http.StatusInternalServerError, "Failed to list BGP peers", err, logger.APILog)
			return
		}

		// Build a map of live peer statuses by address
		peerStatusMap := make(map[string]bgp.BGPPeerStatus)

		if bgpService != nil && bgpService.IsRunning() {
			status, err := bgpService.GetStatus(r.Context())
			if err == nil {
				for _, ps := range status.Peers {
					peerStatusMap[ps.Address] = ps
				}
			}
		}

		items := make([]BGPPeer, 0)

		for _, dbPeer := range dbPeers {
			pw := ""
			if dbPeer.Password != "" {
				pw = maskedPassword
			}

			peer := BGPPeer{
				ID:          dbPeer.ID,
				Address:     dbPeer.Address,
				RemoteAS:    dbPeer.RemoteAS,
				HoldTime:    dbPeer.HoldTime,
				Password:    pw,
				Description: dbPeer.Description,
			}

			if ps, ok := peerStatusMap[dbPeer.Address]; ok {
				peer.State = ps.State
				peer.Uptime = ps.Uptime
				peer.PrefixesSent = ps.PrefixesSent
			}

			items = append(items, peer)
		}

		resp := ListBGPPeersResponse{
			Items:      items,
			Page:       page,
			PerPage:    perPage,
			TotalCount: total,
		}

		writeResponse(r.Context(), w, resp, http.StatusOK, logger.APILog)
	})
}

func CreateBGPPeer(dbInstance *db.Database, bgpService *bgp.BGPService) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		email, ok := r.Context().Value(contextKeyEmail).(string)
		if !ok {
			writeError(r.Context(), w, http.StatusInternalServerError, "Failed to get email", nil, logger.APILog)
			return
		}

		var params CreateBGPPeerParams
		if err := json.NewDecoder(r.Body).Decode(&params); err != nil {
			writeError(r.Context(), w, http.StatusBadRequest, "Invalid request data", err, logger.APILog)
			return
		}

		if params.Address == "" {
			writeError(r.Context(), w, http.StatusBadRequest, "address is required", nil, logger.APILog)
			return
		}

		if ip := net.ParseIP(params.Address); ip == nil || ip.To4() == nil {
			writeError(r.Context(), w, http.StatusBadRequest, "address must be a valid IPv4 address", nil, logger.APILog)
			return
		}

		if params.RemoteAS < 1 || params.RemoteAS > 4294967295 {
			writeError(r.Context(), w, http.StatusBadRequest, "remoteAS must be between 1 and 4294967295", nil, logger.APILog)
			return
		}

		if params.HoldTime == 0 {
			params.HoldTime = 90
		}

		if params.HoldTime < 3 || params.HoldTime > 65535 {
			writeError(r.Context(), w, http.StatusBadRequest, "holdTime must be between 3 and 65535", nil, logger.APILog)
			return
		}

		numPeers, err := dbInstance.CountBGPPeers(r.Context())
		if err != nil {
			writeError(r.Context(), w, http.StatusInternalServerError, "Failed to count BGP peers", err, logger.APILog)
			return
		}

		if numPeers >= MaxNumBGPPeers {
			writeError(r.Context(), w, http.StatusBadRequest, "Maximum number of BGP peers reached ("+strconv.Itoa(MaxNumBGPPeers)+")", nil, logger.APILog)
			return
		}

		dbPeer := &db.BGPPeer{
			Address:     params.Address,
			RemoteAS:    params.RemoteAS,
			HoldTime:    params.HoldTime,
			Password:    params.Password,
			Description: params.Description,
		}

		if err := dbInstance.CreateBGPPeer(r.Context(), dbPeer); err != nil {
			if errors.Is(err, db.ErrAlreadyExists) {
				writeError(r.Context(), w, http.StatusConflict, "A BGP peer with this address already exists", nil, logger.APILog)

				return
			}

			writeError(r.Context(), w, http.StatusInternalServerError, "Failed to create BGP peer", err, logger.APILog)

			return
		}

		reconfigureBGPPeers(r.Context(), dbInstance, bgpService)

		writeResponse(r.Context(), w, SuccessResponse{Message: "BGP peer created successfully"}, http.StatusCreated, logger.APILog)

		logger.LogAuditEvent(
			r.Context(),
			CreateBGPPeerAction,
			email,
			getClientIP(r),
			fmt.Sprintf("BGP peer created: address=%s, remoteAS=%d", params.Address, params.RemoteAS),
		)
	})
}

func DeleteBGPPeer(dbInstance *db.Database, bgpService *bgp.BGPService) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		email, ok := r.Context().Value(contextKeyEmail).(string)
		if !ok {
			writeError(r.Context(), w, http.StatusInternalServerError, "Failed to get email", nil, logger.APILog)
			return
		}

		idStr := r.PathValue("id")

		id, err := strconv.Atoi(idStr)
		if err != nil {
			writeError(r.Context(), w, http.StatusBadRequest, "Invalid id format", err, logger.APILog)
			return
		}

		if err := dbInstance.DeleteBGPPeer(r.Context(), id); err != nil {
			if errors.Is(err, db.ErrNotFound) {
				writeError(r.Context(), w, http.StatusNotFound, "BGP peer not found", nil, logger.APILog)

				return
			}

			writeError(r.Context(), w, http.StatusInternalServerError, "Failed to delete BGP peer", err, logger.APILog)

			return
		}

		reconfigureBGPPeers(r.Context(), dbInstance, bgpService)

		writeResponse(r.Context(), w, SuccessResponse{Message: "BGP peer deleted successfully"}, http.StatusOK, logger.APILog)

		logger.LogAuditEvent(
			r.Context(),
			DeleteBGPPeerAction,
			email,
			getClientIP(r),
			"BGP peer deleted: id="+idStr,
		)
	})
}

// BGP Routes handler

func GetBGPRoutes(bgpService *bgp.BGPService) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if bgpService == nil || !bgpService.IsRunning() {
			writeResponse(r.Context(), w, BGPRoutesResponse{Routes: []bgp.BGPRoute{}}, http.StatusOK, logger.APILog)
			return
		}

		routes, err := bgpService.GetRoutes()
		if err != nil {
			writeError(r.Context(), w, http.StatusInternalServerError, "Failed to get BGP routes", err, logger.APILog)
			return
		}

		if routes == nil {
			routes = []bgp.BGPRoute{}
		}

		writeResponse(r.Context(), w, BGPRoutesResponse{Routes: routes}, http.StatusOK, logger.APILog)
	})
}
