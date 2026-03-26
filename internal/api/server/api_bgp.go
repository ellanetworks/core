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
	"github.com/ellanetworks/core/internal/config"
	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/logger"
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

type BGPImportPrefix struct {
	Prefix    string `json:"prefix"`
	MaxLength int    `json:"maxLength"`
}

type BGPPeer struct {
	ID               int               `json:"id"`
	Address          string            `json:"address"`
	RemoteAS         int               `json:"remoteAS"`
	HoldTime         int               `json:"holdTime"`
	Password         string            `json:"password"`
	Description      string            `json:"description"`
	ImportPrefixes   []BGPImportPrefix `json:"importPrefixes"`
	State            string            `json:"state,omitempty"`
	Uptime           string            `json:"uptime,omitempty"`
	PrefixesSent     int               `json:"prefixesSent,omitempty"`
	PrefixesReceived int               `json:"prefixesReceived,omitempty"`
	PrefixesAccepted int               `json:"prefixesAccepted,omitempty"`
}

type CreateBGPPeerParams struct {
	Address        string            `json:"address"`
	RemoteAS       int               `json:"remoteAS"`
	HoldTime       int               `json:"holdTime"`
	Password       string            `json:"password"`
	Description    string            `json:"description"`
	ImportPrefixes []BGPImportPrefix `json:"importPrefixes"`
}

type UpdateBGPPeerParams struct {
	Address        string            `json:"address"`
	RemoteAS       int               `json:"remoteAS"`
	HoldTime       int               `json:"holdTime"`
	Password       string            `json:"password"`
	Description    string            `json:"description"`
	ImportPrefixes []BGPImportPrefix `json:"importPrefixes"`
}

type ListBGPPeersResponse struct {
	Items      []BGPPeer `json:"items"`
	Page       int       `json:"page"`
	PerPage    int       `json:"per_page"`
	TotalCount int       `json:"total_count"`
}

type BGPAdvertisedRoutesResponse struct {
	Routes []bgp.BGPRoute `json:"routes"`
}

type BGPLearnedRoutesResponse struct {
	Routes []bgp.LearnedRoute `json:"routes"`
}

// System (safety) filter types

type BGPSystemFilter struct {
	Prefix      string `json:"prefix"`
	Source      string `json:"source"`
	Description string `json:"description"`
}

type BGPSystemFiltersResponse struct {
	Filters []BGPSystemFilter `json:"filters"`
}

// Audit log action constants

const (
	UpdateBGPSettingsAction = "update_bgp_settings"
	CreateBGPPeerAction     = "create_bgp_peer"
	UpdateBGPPeerAction     = "update_bgp_peer"
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

		allocatedIPs, err := dbInstance.ListAllocatedIPMappings(ctx)
		if err != nil {
			return fmt.Errorf("failed to list allocated IPs: %w", err)
		}

		natEnabled, err := dbInstance.IsNATEnabled(ctx)
		if err != nil {
			return fmt.Errorf("failed to check NAT settings: %w", err)
		}

		bgpPeers := DBPeersToBGPPeers(dbPeers)

		return bgpService.Start(ctx, DBSettingsToBGPSettings(settings), bgpPeers, allocatedIPs, !natEnabled)

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

func reconfigureBGPPeers(ctx context.Context, dbInstance *db.Database, bgpService *bgp.BGPService) error {
	if bgpService == nil || !bgpService.IsRunning() {
		return nil
	}

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

// loadImportPrefixesForPeer loads import prefix entries from the DB for a single peer.
func loadImportPrefixesForPeer(ctx context.Context, dbInstance *db.Database, peerID int) []BGPImportPrefix {
	dbPrefixes, err := dbInstance.ListImportPrefixesByPeer(ctx, peerID)
	if err != nil || len(dbPrefixes) == 0 {
		return []BGPImportPrefix{}
	}

	result := make([]BGPImportPrefix, len(dbPrefixes))

	for i, p := range dbPrefixes {
		result[i] = BGPImportPrefix{
			Prefix:    p.Prefix,
			MaxLength: p.MaxLength,
		}
	}

	return result
}

// saveImportPrefixesForPeer persists import prefix entries for a peer.
func saveImportPrefixesForPeer(ctx context.Context, dbInstance *db.Database, peerID int, prefixes []BGPImportPrefix) error {
	dbPrefixes := make([]db.BGPImportPrefix, len(prefixes))

	for i, p := range prefixes {
		dbPrefixes[i] = db.BGPImportPrefix{
			Prefix:    p.Prefix,
			MaxLength: p.MaxLength,
		}
	}

	return dbInstance.SetImportPrefixesForPeer(ctx, peerID, dbPrefixes)
}

// dbPeerToAPIPeer converts a DB peer to an API peer, enriched with live status and import prefixes.
func dbPeerToAPIPeer(ctx context.Context, dbInstance *db.Database, dbPeer db.BGPPeer, statusMap map[string]bgp.BGPPeerStatus, bgpService *bgp.BGPService) BGPPeer {
	pw := ""
	if dbPeer.Password != "" {
		pw = maskedPassword
	}

	peer := BGPPeer{
		ID:             dbPeer.ID,
		Address:        dbPeer.Address,
		RemoteAS:       dbPeer.RemoteAS,
		HoldTime:       dbPeer.HoldTime,
		Password:       pw,
		Description:    dbPeer.Description,
		ImportPrefixes: loadImportPrefixesForPeer(ctx, dbInstance, dbPeer.ID),
	}

	if ps, ok := statusMap[dbPeer.Address]; ok {
		peer.State = ps.State
		peer.Uptime = ps.Uptime
		peer.PrefixesSent = ps.PrefixesSent
		peer.PrefixesReceived = ps.PrefixesReceived
	}

	if bgpService != nil {
		peer.PrefixesAccepted = bgpService.CountLearnedRoutesByPeer(dbPeer.Address)
	}

	return peer
}

// getPeerStatusMap builds a map of live peer statuses by address.
func getPeerStatusMap(ctx context.Context, bgpService *bgp.BGPService) map[string]bgp.BGPPeerStatus {
	statusMap := make(map[string]bgp.BGPPeerStatus)

	if bgpService != nil && bgpService.IsRunning() {
		status, err := bgpService.GetStatus(ctx)
		if err == nil {
			for _, ps := range status.Peers {
				statusMap[ps.Address] = ps
			}
		}
	}

	return statusMap
}

// validateImportPrefixes checks that all import prefix entries are valid.
func validateImportPrefixes(prefixes []BGPImportPrefix) error {
	for _, p := range prefixes {
		_, ipNet, err := net.ParseCIDR(p.Prefix)
		if err != nil {
			return fmt.Errorf("invalid prefix %q: must be valid CIDR notation", p.Prefix)
		}

		prefixLen, _ := ipNet.Mask.Size()

		if p.MaxLength < prefixLen || p.MaxLength > 32 {
			return fmt.Errorf("invalid maxLength %d for prefix %q: must be between %d and 32", p.MaxLength, p.Prefix, prefixLen)
		}
	}

	return nil
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

		statusMap := getPeerStatusMap(r.Context(), bgpService)

		items := make([]BGPPeer, 0, len(dbPeers))

		for _, dbPeer := range dbPeers {
			items = append(items, dbPeerToAPIPeer(r.Context(), dbInstance, dbPeer, statusMap, bgpService))
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

func GetBGPPeer(dbInstance *db.Database, bgpService *bgp.BGPService) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		idStr := r.PathValue("id")

		id, err := strconv.Atoi(idStr)
		if err != nil {
			writeError(r.Context(), w, http.StatusBadRequest, "Invalid id format", err, logger.APILog)
			return
		}

		dbPeer, err := dbInstance.GetBGPPeer(r.Context(), id)
		if err != nil {
			if errors.Is(err, db.ErrNotFound) {
				writeError(r.Context(), w, http.StatusNotFound, "BGP peer not found", nil, logger.APILog)

				return
			}

			writeError(r.Context(), w, http.StatusInternalServerError, "Failed to get BGP peer", err, logger.APILog)

			return
		}

		statusMap := getPeerStatusMap(r.Context(), bgpService)
		peer := dbPeerToAPIPeer(r.Context(), dbInstance, *dbPeer, statusMap, bgpService)

		writeResponse(r.Context(), w, peer, http.StatusOK, logger.APILog)
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

		if err := validateImportPrefixes(params.ImportPrefixes); err != nil {
			writeError(r.Context(), w, http.StatusBadRequest, err.Error(), nil, logger.APILog)
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

		if len(params.ImportPrefixes) > 0 {
			if err := saveImportPrefixesForPeer(r.Context(), dbInstance, dbPeer.ID, params.ImportPrefixes); err != nil {
				_ = dbInstance.DeleteBGPPeer(r.Context(), dbPeer.ID)

				writeError(r.Context(), w, http.StatusInternalServerError, "Failed to save import prefixes", err, logger.APILog)

				return
			}
		}

		if err := reconfigureBGPPeers(r.Context(), dbInstance, bgpService); err != nil {
			_ = dbInstance.DeleteBGPPeer(r.Context(), dbPeer.ID)

			writeError(r.Context(), w, http.StatusInternalServerError, "Failed to apply BGP peer: "+err.Error(), err, logger.APILog)

			return
		}

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

func UpdateBGPPeer(dbInstance *db.Database, bgpService *bgp.BGPService) http.Handler {
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

		prevPeer, err := dbInstance.GetBGPPeer(r.Context(), id)
		if err != nil {
			if errors.Is(err, db.ErrNotFound) {
				writeError(r.Context(), w, http.StatusNotFound, "BGP peer not found", nil, logger.APILog)

				return
			}

			writeError(r.Context(), w, http.StatusInternalServerError, "Failed to get BGP peer", err, logger.APILog)

			return
		}

		var params UpdateBGPPeerParams
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

		if err := validateImportPrefixes(params.ImportPrefixes); err != nil {
			writeError(r.Context(), w, http.StatusBadRequest, err.Error(), nil, logger.APILog)
			return
		}

		dbPeer := &db.BGPPeer{
			ID:          id,
			Address:     params.Address,
			RemoteAS:    params.RemoteAS,
			HoldTime:    params.HoldTime,
			Password:    params.Password,
			Description: params.Description,
		}

		if err := dbInstance.UpdateBGPPeer(r.Context(), dbPeer); err != nil {
			if errors.Is(err, db.ErrAlreadyExists) {
				writeError(r.Context(), w, http.StatusConflict, "A BGP peer with this address already exists", nil, logger.APILog)

				return
			}

			writeError(r.Context(), w, http.StatusInternalServerError, "Failed to update BGP peer", err, logger.APILog)

			return
		}

		if err := saveImportPrefixesForPeer(r.Context(), dbInstance, id, params.ImportPrefixes); err != nil {
			// Rollback peer update
			_ = dbInstance.UpdateBGPPeer(r.Context(), prevPeer)

			writeError(r.Context(), w, http.StatusInternalServerError, "Failed to save import prefixes", err, logger.APILog)

			return
		}

		if err := reconfigureBGPPeers(r.Context(), dbInstance, bgpService); err != nil {
			// Rollback peer update
			_ = dbInstance.UpdateBGPPeer(r.Context(), prevPeer)

			writeError(r.Context(), w, http.StatusInternalServerError, "Failed to apply BGP peer update: "+err.Error(), err, logger.APILog)

			return
		}

		writeResponse(r.Context(), w, SuccessResponse{Message: "BGP peer updated successfully"}, http.StatusOK, logger.APILog)

		logger.LogAuditEvent(
			r.Context(),
			UpdateBGPPeerAction,
			email,
			getClientIP(r),
			fmt.Sprintf("BGP peer updated: id=%d, address=%s, remoteAS=%d", id, params.Address, params.RemoteAS),
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

		prevPeer, err := dbInstance.GetBGPPeer(r.Context(), id)
		if err != nil {
			if errors.Is(err, db.ErrNotFound) {
				writeError(r.Context(), w, http.StatusNotFound, "BGP peer not found", nil, logger.APILog)

				return
			}

			writeError(r.Context(), w, http.StatusInternalServerError, "Failed to get BGP peer", err, logger.APILog)

			return
		}

		if err := dbInstance.DeleteBGPPeer(r.Context(), id); err != nil {
			writeError(r.Context(), w, http.StatusInternalServerError, "Failed to delete BGP peer", err, logger.APILog)

			return
		}

		if err := reconfigureBGPPeers(r.Context(), dbInstance, bgpService); err != nil {
			_ = dbInstance.CreateBGPPeer(r.Context(), prevPeer)

			writeError(r.Context(), w, http.StatusInternalServerError, "Failed to apply BGP peer removal: "+err.Error(), err, logger.APILog)

			return
		}

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

// BGP Routes handlers

func GetBGPAdvertisedRoutes(bgpService *bgp.BGPService) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if bgpService == nil || !bgpService.IsRunning() {
			writeResponse(r.Context(), w, BGPAdvertisedRoutesResponse{Routes: []bgp.BGPRoute{}}, http.StatusOK, logger.APILog)
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

		writeResponse(r.Context(), w, BGPAdvertisedRoutesResponse{Routes: routes}, http.StatusOK, logger.APILog)
	})
}

func GetBGPLearnedRoutes(bgpService *bgp.BGPService) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if bgpService == nil || !bgpService.IsRunning() {
			writeResponse(r.Context(), w, BGPLearnedRoutesResponse{Routes: []bgp.LearnedRoute{}}, http.StatusOK, logger.APILog)
			return
		}

		routes := bgpService.GetLearnedRoutes()
		if routes == nil {
			routes = []bgp.LearnedRoute{}
		}

		writeResponse(r.Context(), w, BGPLearnedRoutesResponse{Routes: routes}, http.StatusOK, logger.APILog)
	})
}

func GetBGPSystemFilters(dbInstance *db.Database, cfg config.Config) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var filters []BGPSystemFilter

		// Hard-coded safety prefixes (RFC 5765 / RFC 7454).
		builtins := []struct {
			cidr string
			desc string
		}{
			{"169.254.0.0/16", "Link-local"},
			{"224.0.0.0/4", "Multicast"},
			{"127.0.0.0/8", "Loopback"},
		}
		for _, b := range builtins {
			filters = append(filters, BGPSystemFilter{
				Prefix:      b.cidr,
				Source:      "builtin",
				Description: b.desc,
			})
		}

		// Data network UE IP pools.
		dataNetworks, _, err := dbInstance.ListDataNetworksPage(r.Context(), 1, 100)
		if err == nil {
			for _, dn := range dataNetworks {
				if _, _, parseErr := net.ParseCIDR(dn.IPPool); parseErr == nil {
					filters = append(filters, BGPSystemFilter{
						Prefix:      dn.IPPool,
						Source:      "data_network",
						Description: "UE IP pool (" + dn.Name + ")",
					})
				}
			}
		}

		// N3 interface address.
		if n3IP := net.ParseIP(cfg.Interfaces.N3.Address); n3IP != nil {
			filters = append(filters, BGPSystemFilter{
				Prefix:      n3IP.String() + "/32",
				Source:      "interface",
				Description: "N3 interface address",
			})
		}

		// N6 interface subnets.
		n6Subnets := interfaceSubnets(cfg.Interfaces.N6.Name)
		for _, s := range n6Subnets {
			filters = append(filters, BGPSystemFilter{
				Prefix:      s.String(),
				Source:      "interface",
				Description: "N6 interface subnet",
			})
		}

		writeResponse(r.Context(), w, BGPSystemFiltersResponse{Filters: filters}, http.StatusOK, logger.APILog)
	})
}
