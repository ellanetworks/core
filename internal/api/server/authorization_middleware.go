package server

import (
	"context"
	"errors"
	"net/http"

	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/logger"
)

type RoleID int

const (
	RoleAdmin          RoleID = 1
	RoleReadOnly       RoleID = 2
	RoleNetworkManager RoleID = 3
)

var PermissionsByRole = map[RoleID][]string{
	RoleAdmin: {"*"},

	RoleReadOnly: {
		PermReadMyUser,
		PermReadOperator, PermGetOperatorSlice, PermGetOperatorTracking,
		PermListSubscribers, PermReadSubscriber,
		PermListDataNetworks, PermReadDataNetwork,
		PermListPolicies, PermReadPolicy,
		PermListRoutes, PermReadRoute,
		PermListRadios, PermReadRadio,
	},

	RoleNetworkManager: {
		PermUpdateUserPassword, PermReadUser, PermReadMyUser,
		PermReadOperator, PermUpdateOperatorSlice, PermGetOperatorSlice, PermUpdateOperatorTracking, PermGetOperatorTracking,
		PermListDataNetworks, PermCreateDataNetwork, PermUpdateDataNetwork, PermReadDataNetwork, PermDeleteDataNetwork,
		PermListSubscribers, PermCreateSubscriber, PermUpdateSubscriber, PermReadSubscriber, PermDeleteSubscriber,
		PermListPolicies, PermCreatePolicy, PermUpdatePolicy, PermReadPolicy, PermDeletePolicy,
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

	// Data Network permissions
	PermListDataNetworks  = "data_network:list"
	PermCreateDataNetwork = "data_network:create"
	PermUpdateDataNetwork = "data_network:update"
	PermReadDataNetwork   = "data_network:read"
	PermDeleteDataNetwork = "data_network:delete"

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

	// Policy permissions
	PermListPolicies = "policy:list"
	PermCreatePolicy = "policy:create"
	PermUpdatePolicy = "policy:update"
	PermReadPolicy   = "policy:read"
	PermDeletePolicy = "policy:delete"

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

	// Audit Log permissions
	PermGetAuditLogRetentionPolicy = "audit_logs:get_retention"
	PermSetAuditLogRetentionPolicy = "audit_logs:set_retention"
	PermListAuditLogs              = "audit_logs:list"
	PermDeleteAuditLogs            = "audit_logs:delete"
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

		ctx = context.WithValue(ctx, contextKeyUserID, claims.ID)
		ctx = context.WithValue(ctx, contextKeyEmail, claims.Email)
		ctx = context.WithValue(ctx, contextKeyRoleID, claims.RoleID)
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

func RequirePermission(permission string, jwtSecret []byte, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		claims, err := getClaimsFromAuthorizationHeader(r.Header.Get("Authorization"), jwtSecret)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "Unauthorized", err, logger.APILog)
			return
		}

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
