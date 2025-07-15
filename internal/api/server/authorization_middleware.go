package server

import (
	"context"
	"errors"
	"net/http"

	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/logger"
)

var roleNameToID = map[string]int{
	"admin":           1,
	"readonly":        2,
	"network-manager": 3,
}

// map role ID to role name
var roleIDToName = map[int]string{
	1: "admin",
	2: "readonly",
	3: "network-manager",
}

var PermissionsByRole = map[int][]string{
	1: {"*"}, // Admin

	2: { // Read Only
		PermReadMyUser,
		PermReadOperator, PermGetOperatorSlice, PermGetOperatorTracking,
		PermListSubscribers, PermReadSubscriber,
		PermListProfiles, PermReadProfile,
		PermListRoutes, PermReadRoute,
		PermListRadios, PermReadRadio,
	},

	3: { // Network Manager
		PermUpdateUserPassword, PermReadUser, PermReadMyUser,
		PermReadOperator, PermUpdateOperatorSlice, PermGetOperatorSlice, PermUpdateOperatorTracking, PermGetOperatorTracking,
		PermListSubscribers, PermCreateSubscriber, PermUpdateSubscriber, PermReadSubscriber, PermDeleteSubscriber,
		PermListProfiles, PermCreateProfile, PermUpdateProfile, PermReadProfile, PermDeleteProfile,
		PermListRoutes, PermCreateRoute, PermReadRoute, PermDeleteRoute,
		PermListRadios, PermReadRadio,
	},
}

const (
	// User permissions
	PermListUsers          = "user:list"
	PermCreateUser         = "user:create"
	PermUpdateUser         = "user:update"
	PermUpdateUserPassword = "user:update_password"
	PermReadUser           = "user:read"
	PermDeleteUser         = "user:delete"
	PermReadMyUser         = "user:read_my_user"

	// Operator permissions
	PermReadOperator              = "operator:read"
	PermUpdateOperatorSlice       = "operator:update_slice"
	PermGetOperatorSlice          = "operator:get_slice"
	PermUpdateOperatorTracking    = "operator:update_tracking"
	PermGetOperatorTracking       = "operator:get_tracking"
	PermUpdateOperatorID          = "operator:update_id"
	PermGetOperatorID             = "operator:get_id"
	PermUpdateOperatorCode        = "operator:update_code"
	PermUpdateOperatorHomeNetwork = "operator:update_home_network"

	// Subscriber permissions
	PermListSubscribers  = "subscriber:list"
	PermCreateSubscriber = "subscriber:create"
	PermUpdateSubscriber = "subscriber:update"
	PermReadSubscriber   = "subscriber:read"
	PermDeleteSubscriber = "subscriber:delete"

	// Profile permissions
	PermListProfiles  = "profile:list"
	PermCreateProfile = "profile:create"
	PermUpdateProfile = "profile:update"
	PermReadProfile   = "profile:read"
	PermDeleteProfile = "profile:delete"

	// Route permissions
	PermListRoutes  = "route:list"
	PermCreateRoute = "route:create"
	PermReadRoute   = "route:read"
	PermDeleteRoute = "route:delete"

	// Radio permissions
	PermListRadios = "radio:list"
	PermReadRadio  = "radio:read"

	// Backup and Restore permissions
	PermBackup  = "backup:create"
	PermRestore = "backup:restore"
)

func RequirePermissionOrFirstUser(permission string, db *db.Database, jwtSecret []byte, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		// Allow unauthenticated creation if first user
		if permission == PermCreateUser && r.Method == http.MethodPost {
			userCount, err := db.NumUsers(ctx)
			if err != nil {
				writeError(w, http.StatusInternalServerError, "Failed to count users", err, logger.APILog)
				return
			}
			if userCount == 0 {
				next.ServeHTTP(w, r)
				return
			}
		}

		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			writeError(w, http.StatusUnauthorized, "Authorization header not found", errors.New("missing header"), logger.APILog)
			return
		}

		claims, err := getClaimsFromAuthorizationHeader(authHeader, jwtSecret)
		if err != nil {
			logger.LogAuditEvent("auth_fail", "", getClientIP(r), "unauthorized")
			writeError(w, http.StatusUnauthorized, "Invalid token", err, logger.APILog)
			return
		}

		// Inject claims into context
		ctx = context.WithValue(ctx, "userID", claims.ID)
		ctx = context.WithValue(ctx, "email", claims.Email)
		ctx = context.WithValue(ctx, "roleID", claims.RoleID)
		r = r.WithContext(ctx)

		// Check permission
		allowedPerms := PermissionsByRole[claims.RoleID]
		for _, p := range allowedPerms {
			if p == permission || p == "*" {
				next.ServeHTTP(w, r)
				return
			}
		}

		writeError(w, http.StatusForbidden, "Forbidden", errors.New("permission denied"), logger.APILog)
	})
}

func RequirePermission(permission string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		roleIDAny := r.Context().Value("roleID")
		roleID, ok := roleIDAny.(int)
		if !ok {
			writeError(w, http.StatusForbidden, "Invalid or missing role ID", errors.New("role ID missing in context"), logger.APILog)
			return
		}

		allowedPerms := PermissionsByRole[roleID]
		for _, p := range allowedPerms {
			if p == permission || p == "*" {
				next.ServeHTTP(w, r)
				return
			}
		}

		writeError(w, http.StatusForbidden, "Forbidden", errors.New("permission denied"), logger.APILog)
	})
}
