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

	"github.com/ellanetworks/core/internal/bgp"
	"github.com/ellanetworks/core/internal/config"
	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/logger"
)

// BGP Settings types

type RejectedPrefix struct {
	Prefix      string `json:"prefix"`
	Source      string `json:"source"`
	Description string `json:"description"`
}

type GetBGPSettingsResponse struct {
	Enabled          bool             `json:"enabled"`
	LocalAS          int              `json:"localAS"`
	RouterID         string           `json:"routerID"`
	ListenAddress    string           `json:"listenAddress"`
	RejectedPrefixes []RejectedPrefix `json:"rejectedPrefixes"`
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
	HasPassword      bool              `json:"hasPassword"`
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
	Password       *string           `json:"password,omitempty"`
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

// Audit log action constants

const (
	UpdateBGPSettingsAction = "update_bgp_settings"
	CreateBGPPeerAction     = "create_bgp_peer"
	UpdateBGPPeerAction     = "update_bgp_peer"
	DeleteBGPPeerAction     = "delete_bgp_peer"
)

const (
	MaxNumBGPPeers           = 5
	MaxImportPrefixesPerPeer = 50
)

// BGP Settings handlers

func GetBGPSettings(dbInstance *db.Database, bgpService *bgp.BGPService, cfg config.Config) http.Handler {
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
			Enabled:          settings.Enabled,
			LocalAS:          settings.LocalAS,
			RouterID:         routerID,
			ListenAddress:    settings.ListenAddress,
			RejectedPrefixes: buildRejectedPrefixes(r.Context(), dbInstance, cfg),
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
			if _, err := netip.ParseAddr(params.RouterID); err != nil {
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
// Callers are responsible for setting PrefixesAccepted (learned route count).
func dbPeerToAPIPeer(ctx context.Context, dbInstance *db.Database, dbPeer db.BGPPeer, statusMap map[string]bgp.BGPPeerStatus) BGPPeer {
	peer := BGPPeer{
		ID:             dbPeer.ID,
		Address:        dbPeer.Address,
		RemoteAS:       dbPeer.RemoteAS,
		HoldTime:       dbPeer.HoldTime,
		HasPassword:    dbPeer.Password != "",
		Description:    dbPeer.Description,
		ImportPrefixes: loadImportPrefixesForPeer(ctx, dbInstance, dbPeer.ID),
	}

	if ps, ok := statusMap[dbPeer.Address]; ok {
		peer.State = ps.State
		peer.Uptime = ps.Uptime
		peer.PrefixesSent = ps.PrefixesSent
		peer.PrefixesReceived = ps.PrefixesReceived
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

// validatePeerParams validates common BGP peer parameters shared between
// create and update handlers. It applies the default holdTime if zero.
func validatePeerParams(address string, remoteAS int, holdTime *int, importPrefixes []BGPImportPrefix) error {
	if address == "" {
		return fmt.Errorf("address is required")
	}

	if addr, err := netip.ParseAddr(address); err != nil || !addr.Is4() {
		return fmt.Errorf("address must be a valid IPv4 address")
	}

	if remoteAS < 1 || remoteAS > 4294967295 {
		return fmt.Errorf("remoteAS must be between 1 and 4294967295")
	}

	if *holdTime == 0 {
		*holdTime = 90
	}

	if *holdTime < 3 || *holdTime > 65535 {
		return fmt.Errorf("holdTime must be between 3 and 65535")
	}

	return validateImportPrefixes(importPrefixes)
}

// validateImportPrefixes checks that all import prefix entries are valid.
// Note: maxLength == 0 is valid for a /0 prefix and means "default route only"
// (exact match on 0.0.0.0/0).
func validateImportPrefixes(prefixes []BGPImportPrefix) error {
	if len(prefixes) > MaxImportPrefixesPerPeer {
		return fmt.Errorf("too many import prefixes: maximum is %d", MaxImportPrefixesPerPeer)
	}

	for _, p := range prefixes {
		prefix, err := netip.ParsePrefix(p.Prefix)
		if err != nil {
			return fmt.Errorf("invalid prefix %q: must be valid CIDR notation", p.Prefix)
		}

		prefixLen := prefix.Bits()

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

		var learnedCounts map[string]int
		if bgpService != nil {
			learnedCounts = bgpService.LearnedRouteCountsByPeer()
		}

		items := make([]BGPPeer, 0, len(dbPeers))

		for _, dbPeer := range dbPeers {
			peer := dbPeerToAPIPeer(r.Context(), dbInstance, dbPeer, statusMap)
			if learnedCounts != nil {
				peer.PrefixesAccepted = learnedCounts[dbPeer.Address]
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
		peer := dbPeerToAPIPeer(r.Context(), dbInstance, *dbPeer, statusMap)

		if bgpService != nil {
			peer.PrefixesAccepted = bgpService.CountLearnedRoutesByPeer(dbPeer.Address)
		}

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

		if err := validatePeerParams(params.Address, params.RemoteAS, &params.HoldTime, params.ImportPrefixes); err != nil {
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

		if err := validatePeerParams(params.Address, params.RemoteAS, &params.HoldTime, params.ImportPrefixes); err != nil {
			writeError(r.Context(), w, http.StatusBadRequest, err.Error(), nil, logger.APILog)
			return
		}

		password := prevPeer.Password
		if params.Password != nil {
			password = *params.Password
		}

		dbPeer := &db.BGPPeer{
			ID:          id,
			Address:     params.Address,
			RemoteAS:    params.RemoteAS,
			HoldTime:    params.HoldTime,
			Password:    password,
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

		if _, err := dbInstance.GetBGPPeer(r.Context(), id); err != nil {
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

// buildRejectedPrefixes constructs the list of safety rejection prefixes
// from builtins, data networks, and interface configuration.
func buildRejectedPrefixes(ctx context.Context, dbInstance *db.Database, cfg config.Config) []RejectedPrefix {
	var filters []RejectedPrefix

	builtins := []struct {
		cidr string
		desc string
	}{
		{"169.254.0.0/16", "Link-local"},
		{"224.0.0.0/4", "Multicast"},
		{"127.0.0.0/8", "Loopback"},
	}
	for _, b := range builtins {
		filters = append(filters, RejectedPrefix{
			Prefix:      b.cidr,
			Source:      "builtin",
			Description: b.desc,
		})
	}

	dataNetworks, err := dbInstance.ListAllDataNetworks(ctx)
	if err == nil {
		for _, dn := range dataNetworks {
			if _, parseErr := netip.ParsePrefix(dn.IPPool); parseErr == nil {
				filters = append(filters, RejectedPrefix{
					Prefix:      dn.IPPool,
					Source:      "data_network",
					Description: "UE IP pool (" + dn.Name + ")",
				})
			}
		}
	}

	if n3Addr, err := netip.ParseAddr(cfg.Interfaces.N3.Address); err == nil {
		filters = append(filters, RejectedPrefix{
			Prefix:      n3Addr.String() + "/32",
			Source:      "interface",
			Description: "N3 interface address",
		})
	}

	n6Subnets := bgp.InterfaceIPv4Subnets(cfg.Interfaces.N6.Name)
	for _, s := range n6Subnets {
		filters = append(filters, RejectedPrefix{
			Prefix:      s.String(),
			Source:      "interface",
			Description: "N6 interface subnet",
		})
	}

	return filters
}
