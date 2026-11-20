// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/oidc"
	"go.uber.org/zap"
)

type OIDCSettings interface {
	Get() db.OIDC
	Update(ctx context.Context, o db.OIDC) error
	Client(ctx context.Context, now func() time.Time) (*oidc.Client, error)
}

type OIDCSettingsResponse struct {
	Enabled                bool            `json:"enabled"`
	ExternalURL            string          `json:"external_url"`
	RedirectURL            string          `json:"redirect_url"`
	PostLogoutRedirectURL  string          `json:"post_logout_redirect_url"`
	Issuer                 string          `json:"issuer"`
	ClientID               string          `json:"client_id"`
	ClientSecretSet        bool            `json:"client_secret_set"`
	DisplayName            string          `json:"display_name"`
	Scopes                 []string        `json:"scopes"`
	SessionLifetimeSeconds int64           `json:"session_lifetime_seconds"`
	TrustUnverifiedEmail   bool            `json:"trust_unverified_email"`
	CACertificate          string          `json:"ca_certificate"`
	CACertificates         []OIDCCACert    `json:"ca_certificates"`
	GroupsClaim            string          `json:"groups_claim"`
	GroupRoles             []OIDCGroupRole `json:"group_roles"`
}

type OIDCCACert struct {
	Subject  string `json:"subject"`
	NotAfter string `json:"not_after"`
}

type OIDCGroupRole struct {
	Group  string `json:"group"`
	RoleID RoleID `json:"role_id"`
}

type UpdateOIDCParams struct {
	Enabled                bool            `json:"enabled"`
	ExternalURL            string          `json:"external_url"`
	Issuer                 string          `json:"issuer"`
	ClientID               string          `json:"client_id"`
	ClientSecret           string          `json:"client_secret"`
	DisplayName            string          `json:"display_name"`
	Scopes                 []string        `json:"scopes"`
	SessionLifetimeSeconds int64           `json:"session_lifetime_seconds"`
	TrustUnverifiedEmail   bool            `json:"trust_unverified_email"`
	CACertificate          string          `json:"ca_certificate"`
	GroupsClaim            string          `json:"groups_claim"`
	GroupRoles             []OIDCGroupRole `json:"group_roles"`
}

const (
	UpdateOIDCAction = "update_oidc"
)

func newOIDCSettings(ctx context.Context, dbInstance *db.Database) *oidc.Live {
	if ctx == nil {
		ctx = context.Background()
	}

	wakeup, stop := dbInstance.Changefeed().Wakeup(db.TopicOIDC)

	initial, err := dbInstance.GetOIDC(ctx)
	if err != nil {
		logger.APILog.Warn("couldn't load the OIDC settings", zap.Error(err))

		initial = db.DefaultOIDC()
	}

	live := oidc.NewLive(dbInstance, initial)

	go func() {
		defer stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-wakeup:
				o, err := dbInstance.GetOIDC(ctx)
				if err != nil {
					logger.APILog.Warn("couldn't reload the OIDC settings after a replicated change", zap.Error(err))
					continue
				}

				live.Reload(o)
			}
		}
	}()

	return live
}

func GetOIDC(oidcSettings OIDCSettings) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeResponse(r.Context(), w, oidcResponse(oidcSettings.Get()), http.StatusOK, logger.APILog)
	})
}

func UpdateOIDC(oidcSettings OIDCSettings) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var params UpdateOIDCParams
		if err := json.NewDecoder(r.Body).Decode(&params); err != nil {
			writeError(r.Context(), w, http.StatusBadRequest, "Invalid request data", err, logger.APILog)
			return
		}

		minLifetime, maxLifetime := int64(oidc.MinSessionLifetime/time.Second), int64(oidc.MaxSessionLifetime/time.Second)
		if params.SessionLifetimeSeconds < minLifetime || params.SessionLifetimeSeconds > maxLifetime {
			writeError(r.Context(), w, http.StatusBadRequest,
				fmt.Sprintf("session_lifetime_seconds must be between %d and %d", minLifetime, maxLifetime), nil, logger.APILog)

			return
		}

		current := oidcSettings.Get()

		o := db.OIDC{
			Enabled:              params.Enabled,
			ExternalURL:          params.ExternalURL,
			Issuer:               params.Issuer,
			ClientID:             params.ClientID,
			ClientSecret:         params.ClientSecret,
			DisplayName:          params.DisplayName,
			Scopes:               params.Scopes,
			SessionLifetime:      time.Duration(params.SessionLifetimeSeconds) * time.Second,
			TrustUnverifiedEmail: params.TrustUnverifiedEmail,
			CACertificate:        params.CACertificate,
			GroupsClaim:          params.GroupsClaim,
			GroupRoles:           []db.GroupRole{},
		}

		if o.ClientSecret == "" && current.ClientSecret != "" {
			if o.Issuer != current.Issuer || o.ClientID != current.ClientID {
				writeError(r.Context(), w, http.StatusBadRequest, "client_secret is required when the issuer or the client ID changes", nil, logger.APILog)
				return
			}

			o.ClientSecret = current.ClientSecret
		}

		for _, g := range params.GroupRoles {
			o.GroupRoles = append(o.GroupRoles, db.GroupRole{Group: g.Group, RoleID: db.RoleID(g.RoleID)})
		}

		if o.Enabled {
			_, err := oidc.Discover(r.Context(), o.Issuer, o.CACertificate)
			if errors.Is(err, oidc.ErrInvalid) {
				writeError(r.Context(), w, http.StatusBadRequest, err.Error(), nil, logger.APILog)
				return
			}

			if err != nil {
				writeError(r.Context(), w, http.StatusBadGateway, clip(err.Error()), nil, logger.APILog)
				return
			}
		}

		err := oidcSettings.Update(r.Context(), o)
		if errors.Is(err, oidc.ErrInvalid) {
			writeError(r.Context(), w, http.StatusBadRequest, err.Error(), nil, logger.APILog)
			return
		}

		if err != nil {
			writeError(r.Context(), w, http.StatusInternalServerError, "Failed to update the OIDC settings", err, logger.APILog)
			return
		}

		updated := oidcSettings.Get()

		writeResponse(r.Context(), w, oidcResponse(updated), http.StatusOK, logger.APILog)

		logger.LogAuditEvent(r.Context(), UpdateOIDCAction, getEmailFromContext(r), getClientIP(r), oidcChanges(current, updated))
	})
}

func DeleteUserPassword(dbInstance *db.Database) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		emailParam, ok := otherUser(w, r)
		if !ok {
			return
		}

		err := dbInstance.UpdateUserPassword(r.Context(), emailParam, "")
		if err != nil {
			switch {
			case errors.Is(err, db.ErrNotFound):
				writeError(r.Context(), w, http.StatusNotFound, "User not found", nil, logger.APILog)
			case errors.Is(err, db.ErrLastPasswordAdmin):
				writeError(r.Context(), w, http.StatusConflict, lastPasswordAdmin, nil, logger.APILog)
			case errors.Is(err, db.ErrNoPassword):
				writeError(r.Context(), w, http.StatusConflict, "The user has no password", nil, logger.APILog)
			default:
				writeError(r.Context(), w, http.StatusInternalServerError, "Failed to remove the password", err, logger.APILog)
			}

			return
		}

		if u, err := dbInstance.GetUser(r.Context(), emailParam); err == nil {
			if err := dbInstance.DeleteAllSessionsForUser(r.Context(), u.ID); err != nil {
				logger.APILog.Warn("Failed to invalidate sessions after password removal", zap.Error(err))
			}
		}

		writeResponse(r.Context(), w, SuccessResponse{Message: "Password removed successfully"}, http.StatusOK, logger.APILog)

		logger.LogAuditEvent(r.Context(), DeleteUserPasswordAction, getEmailFromContext(r), getClientIP(r), "Password of "+emailParam+" removed")
	})
}

func UnlinkUser(dbInstance *db.Database) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		emailParam, ok := otherUser(w, r)
		if !ok {
			return
		}

		err := dbInstance.UnlinkUser(r.Context(), emailParam)
		if errors.Is(err, db.ErrNotFound) {
			writeError(r.Context(), w, http.StatusNotFound, "The user is not linked to the identity provider", nil, logger.APILog)
			return
		}

		if err != nil {
			writeError(r.Context(), w, http.StatusInternalServerError, "Failed to unlink the user", err, logger.APILog)
			return
		}

		writeResponse(r.Context(), w, SuccessResponse{Message: "User unlinked successfully"}, http.StatusOK, logger.APILog)

		logger.LogAuditEvent(r.Context(), UnlinkUserAction, getEmailFromContext(r), getClientIP(r), "User "+emailParam+" unlinked from the identity provider")
	})
}

func otherUser(w http.ResponseWriter, r *http.Request) (string, bool) {
	emailParam := r.PathValue("email")
	if emailParam == "" {
		writeError(r.Context(), w, http.StatusBadRequest, "Missing email parameter", errors.New("missing param"), logger.APILog)
		return "", false
	}

	if getEmailFromContext(r) == emailParam {
		writeError(r.Context(), w, http.StatusForbidden, "Cannot modify your own account", errors.New("self-modification not allowed"), logger.APILog)
		return "", false
	}

	return emailParam, true
}

func userResponse(u db.User) User {
	return User{
		Email:       u.Email,
		RoleID:      RoleID(u.RoleID),
		PasswordSet: u.HashedPassword != "",
		OIDCLinked:  u.HasIdentity,
	}
}

func roleFromGroups(oidcSettings OIDCSettings, u *db.User) bool {
	o := oidcSettings.Get()

	return o.Enabled && len(o.GroupRoles) > 0 && u.HasIdentity
}

func oidcResponse(o db.OIDC) OIDCSettingsResponse {
	resp := OIDCSettingsResponse{
		Enabled:                o.Enabled,
		ExternalURL:            o.ExternalURL,
		Issuer:                 o.Issuer,
		ClientID:               o.ClientID,
		ClientSecretSet:        o.ClientSecret != "",
		DisplayName:            o.DisplayName,
		Scopes:                 o.Scopes,
		SessionLifetimeSeconds: int64(o.SessionLifetime / time.Second),
		TrustUnverifiedEmail:   o.TrustUnverifiedEmail,
		CACertificate:          o.CACertificate,
		GroupsClaim:            o.GroupsClaim,
		CACertificates:         []OIDCCACert{},
		GroupRoles:             []OIDCGroupRole{},
	}

	if o.CACertificate != "" {
		certs, _ := oidc.CACertificates(o.CACertificate)
		for _, cert := range certs {
			resp.CACertificates = append(resp.CACertificates, OIDCCACert{
				Subject:  cert.Subject.String(),
				NotAfter: cert.NotAfter.UTC().Format(time.RFC3339),
			})
		}
	}

	if resp.Scopes == nil {
		resp.Scopes = []string{}
	}

	if o.ExternalURL != "" {
		resp.RedirectURL = oidc.RedirectURL(o)
		resp.PostLogoutRedirectURL = oidc.PostLogoutRedirectURL(o)
	}

	for _, g := range o.GroupRoles {
		resp.GroupRoles = append(resp.GroupRoles, OIDCGroupRole{Group: g.Group, RoleID: RoleID(g.RoleID)})
	}

	return resp
}

func oidcChanges(before, after db.OIDC) string {
	var changed []string

	for name, differs := range map[string]bool{
		"enabled":                before.Enabled != after.Enabled,
		"external_url":           before.ExternalURL != after.ExternalURL,
		"issuer":                 before.Issuer != after.Issuer,
		"client_id":              before.ClientID != after.ClientID,
		"client_secret":          before.ClientSecret != after.ClientSecret,
		"display_name":           before.DisplayName != after.DisplayName,
		"scopes":                 strings.Join(before.Scopes, " ") != strings.Join(after.Scopes, " "),
		"session_lifetime":       before.SessionLifetime != after.SessionLifetime,
		"trust_unverified_email": before.TrustUnverifiedEmail != after.TrustUnverifiedEmail,
		"ca_certificate":         before.CACertificate != after.CACertificate,
		"groups_claim":           before.GroupsClaim != after.GroupsClaim,
		"group_roles":            fmt.Sprint(before.GroupRoles) != fmt.Sprint(after.GroupRoles),
	} {
		if differs {
			changed = append(changed, name)
		}
	}

	if len(changed) == 0 {
		return fmt.Sprintf("OIDC settings saved unchanged, enabled %t", after.Enabled)
	}

	slices.Sort(changed)

	return fmt.Sprintf("OIDC settings changed (%s), enabled %t", strings.Join(changed, ", "), after.Enabled)
}
