// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package server

import (
	"crypto/hkdf"
	"crypto/sha256"
	"crypto/subtle"
	"errors"
	"fmt"
	"net/http"
	"net/mail"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/oidc"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

const (
	OIDCLoginCookieName = "__Host-ella_core_oidc_login"
	OIDCLoginAction     = "auth_oidc_login"
	LinkUserAction      = "link_user_oidc"
	oidcCookieKeyInfo   = "ella-core OIDC login cookie"
	maxErrorDetail      = 300
	maxListedGroups     = 10
	maxListedGroupBytes = 24
)

const (
	OIDCSuccess          = "success"
	OIDCRateLimited      = "rate_limited"
	OIDCUnavailable      = "unavailable"
	OIDCFailed           = "failed"
	OIDCDenied           = "denied"
	OIDCNotSetUp         = "not_set_up"
	OIDCEmailNotVerified = "email_not_verified"
	OIDCLinkedElsewhere  = "linked_elsewhere"
	OIDCNoGroup          = "no_group"
	OIDCGroupsOverage    = "groups_overage"
	OIDCTooManyUsers     = "too_many_users"
	OIDCLinkRequired     = "link_required"
	OIDCError            = "error"
)

type OIDCLoginResponse struct {
	Enabled     bool   `json:"enabled"`
	DisplayName string `json:"display_name,omitempty"`
}

type oidcRefusal struct {
	outcome string
	detail  string
}

func (e oidcRefusal) Error() string { return e.outcome + ": " + e.detail }

func refuse(outcome, format string, args ...any) error {
	return oidcRefusal{outcome: outcome, detail: clip(fmt.Sprintf(format, args...))}
}

func oidcCookieKey(jwtSecret *JWTSecret) ([]byte, error) {
	return hkdf.Key(sha256.New, jwtSecret.Get(), nil, oidcCookieKeyInfo, 32)
}

func GetOIDCLogin(oidcSettings OIDCSettings) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		o := oidcSettings.Get()

		resp := OIDCLoginResponse{Enabled: o.Enabled}
		if o.Enabled {
			resp.DisplayName = o.DisplayName
		}

		writeResponse(r.Context(), w, resp, http.StatusOK, logger.APILog)
	})
}

func StartOIDCLogin(dbInstance *db.Database, oidcSettings OIDCSettings, jwtSecret *JWTSecret, loginLimiter *ipRateLimiter) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")

		link := r.URL.Query().Get("link") == "1"

		if !loginLimiter.allow(getClientIP(r)) {
			oidcDone(w, r, "", link, refuse(OIDCRateLimited, "too many login attempts"))
			return
		}

		c, err := oidcSettings.Client(r.Context(), time.Now)
		if err != nil {
			oidcDone(w, r, "", link, refuse(OIDCUnavailable, "%v", err))
			return
		}

		key, err := oidcCookieKey(jwtSecret)
		if err != nil {
			oidcDone(w, r, "", link, err)
			return
		}

		l := c.NewLogin(time.Now())

		if link {
			s, err := sessionFromCookie(r, dbInstance)
			if err != nil {
				oidcDone(w, r, "", link, refuse(OIDCFailed, "no session to link: %v", err))
				return
			}

			l.LinkUserID = s.UserID
		}

		value, err := oidc.Seal(key, OIDCLoginCookieName, l)
		if err != nil {
			oidcDone(w, r, "", link, err)
			return
		}

		http.SetCookie(w, &http.Cookie{
			Name:     OIDCLoginCookieName,
			Value:    value,
			Path:     "/",
			MaxAge:   int(oidc.LoginLifetime.Seconds()),
			HttpOnly: true,
			Secure:   true,
			SameSite: http.SameSiteLaxMode,
		})

		http.Redirect(w, r, c.AuthCodeURL(l.State, l.Nonce, l.Verifier), http.StatusSeeOther)
	})
}

func OIDCCallback(dbInstance *db.Database, oidcSettings OIDCSettings, jwtSecret *JWTSecret, secureCookie bool) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		http.SetCookie(w, &http.Cookie{
			Name: OIDCLoginCookieName, Path: "/", MaxAge: -1, HttpOnly: true, Secure: true,
			SameSite: http.SameSiteLaxMode,
		})

		c, l, cl, err := oidcAuthenticate(r, dbInstance, oidcSettings, jwtSecret)
		link := l.LinkUserID != ""

		if err != nil {
			oidcDone(w, r, "", link, err)
			return
		}

		if link {
			email, err := oidcLink(r, dbInstance, c, l, cl)
			oidcDone(w, r, email, link, err)

			return
		}

		u, err := oidcLogIn(w, r, dbInstance, c, cl, secureCookie)
		if err != nil {
			oidcDone(w, r, actor(cl), link, err)
			return
		}

		oidcDone(w, r, u.Email, link, nil)
	})
}

func oidcAuthenticate(r *http.Request, dbInstance *db.Database, oidcSettings OIDCSettings, jwtSecret *JWTSecret) (*oidc.Client, oidc.Login, oidc.Claims, error) {
	var l oidc.Login

	cookie, err := r.Cookie(OIDCLoginCookieName)
	if err != nil {
		return nil, l, oidc.Claims{}, refuse(OIDCFailed, "no login in progress")
	}

	key, err := oidcCookieKey(jwtSecret)
	if err != nil {
		return nil, l, oidc.Claims{}, err
	}

	l, err = oidc.Open(key, OIDCLoginCookieName, cookie.Value, time.Now())
	if err != nil {
		return nil, l, oidc.Claims{}, refuse(OIDCFailed, "%v", err)
	}

	q := r.URL.Query()
	if subtle.ConstantTimeCompare([]byte(q.Get("state")), []byte(l.State)) != 1 {
		return nil, l, oidc.Claims{}, refuse(OIDCFailed, "the state does not match the login")
	}

	state := sha256.Sum256([]byte(l.State))

	err = dbInstance.UseOIDCState(r.Context(), state[:], l.ExpiresAt, time.Now().Unix())
	if errors.Is(err, db.ErrStateUsed) {
		return nil, l, oidc.Claims{}, refuse(OIDCFailed, "the login was already used")
	}

	if err != nil {
		return nil, l, oidc.Claims{}, err
	}

	c, err := oidcSettings.Client(r.Context(), time.Now)
	if err != nil {
		return nil, l, oidc.Claims{}, refuse(OIDCUnavailable, "%v", err)
	}

	if !c.Owns(l) {
		return nil, l, oidc.Claims{}, refuse(OIDCFailed, "the OIDC settings changed during the login")
	}

	if err := c.CheckIssuer(q.Get("iss"), q.Has("iss")); err != nil {
		return nil, l, oidc.Claims{}, refuse(OIDCFailed, "%v", err)
	}

	if e := q.Get("error"); e != "" {
		outcome := OIDCFailed
		if e == "access_denied" {
			outcome = OIDCDenied
		}

		return nil, l, oidc.Claims{}, refuse(outcome, "the identity provider returned %s: %s", e, q.Get("error_description"))
	}

	code := q.Get("code")
	if code == "" {
		return nil, l, oidc.Claims{}, refuse(OIDCFailed, "no authorization code")
	}

	cl, err := c.Exchange(r.Context(), code, l.Verifier, l.Nonce)
	if err != nil {
		return nil, l, oidc.Claims{}, refuse(OIDCFailed, "%v", err)
	}

	return c, l, cl, nil
}

func oidcLink(r *http.Request, dbInstance *db.Database, c *oidc.Client, l oidc.Login, cl oidc.Claims) (string, error) {
	s, err := sessionFromCookie(r, dbInstance)
	if err != nil || s.UserID != l.LinkUserID {
		return "", refuse(OIDCFailed, "the session that started the link has ended")
	}

	u, err := dbInstance.GetUserByID(r.Context(), s.UserID)
	if err != nil {
		return "", refuse(OIDCFailed, "the user that started the link no longer exists")
	}

	o := c.Settings()

	err = dbInstance.LinkOIDCIdentity(r.Context(), &db.OIDCLink{
		Issuer: o.Issuer, ClientID: o.ClientID, Subject: cl.Subject, Email: cl.Email, UserID: u.ID, IdentityID: newID(),
		At: time.Now().Unix(),
	})

	switch {
	case errors.Is(err, db.ErrOIDCDisabled):
		return u.Email, refuse(OIDCUnavailable, "%v", oidc.ErrDisabled)
	case errors.Is(err, db.ErrOIDCChanged):
		return u.Email, refuse(OIDCFailed, "the OIDC settings changed during the login")
	case errors.Is(err, db.ErrLinkedElsewhere):
		return u.Email, refuse(OIDCLinkedElsewhere, "the account at the identity provider, or %s, is already linked", u.Email)
	case errors.Is(err, db.ErrNotFound):
		return u.Email, refuse(OIDCFailed, "the user that started the link no longer exists")
	case err != nil:
		return u.Email, err
	}

	logger.LogAuditEvent(r.Context(), LinkUserAction, u.Email, getClientIP(r), "User linked to the identity provider account "+cl.Subject)

	return u.Email, nil
}

func oidcLogIn(w http.ResponseWriter, r *http.Request, dbInstance *db.Database, c *oidc.Client, cl oidc.Claims, secureCookie bool) (db.User, error) {
	o := c.Settings()
	role, groupErr := groupRole(c, cl)

	rawToken, tokenHash, err := newSessionToken()
	if err != nil {
		return db.User{}, err
	}

	now := time.Now()
	expiresAt := now.Add(o.SessionLifetime)

	l := &db.OIDCLogin{
		Issuer:             o.Issuer,
		ClientID:           o.ClientID,
		Subject:            cl.Subject,
		Email:              cl.Email,
		MapsGroups:         c.MapsGroups(),
		RoleID:             role,
		MaxUsers:           MaxNumUsers,
		MaxSessionsPerUser: MaxSessionsPerUser,
		NewUserID:          newID(),
		NewIdentityID:      newID(),
		Session: db.Session{
			ID:        newID(),
			TokenHash: tokenHash,
			CreatedAt: now.Unix(),
			ExpiresAt: expiresAt.Unix(),
		},
	}

	var unlinked error

	email, ok := normalEmail(cl.Email)

	switch {
	case !ok:
		unlinked = refuse(OIDCNotSetUp, "the identity provider gave no email address")
	case !c.TrustsEmail(cl):
		unlinked = refuse(OIDCEmailNotVerified, "the identity provider has not verified %s", email)
	default:
		l.LinkEmail = email
	}

	res, err := dbInstance.LogInWithOIDC(r.Context(), l)

	switch {
	case errors.Is(err, db.ErrOIDCDisabled):
		return db.User{}, refuse(OIDCUnavailable, "%v", oidc.ErrDisabled)
	case errors.Is(err, db.ErrOIDCChanged):
		return db.User{}, refuse(OIDCFailed, "the OIDC settings changed during the login")
	case errors.Is(err, db.ErrNotLinked):
		return db.User{}, unlinked
	case errors.Is(err, db.ErrNoUser):
		return db.User{}, refuse(OIDCNotSetUp, "no user has the email %s", email)
	case errors.Is(err, db.ErrAmbiguousEmail):
		return db.User{}, refuse(OIDCNotSetUp, "several users have the email %s in different cases", email)
	case errors.Is(err, db.ErrNoRole):
		return db.User{}, groupErr
	case errors.Is(err, db.ErrLinkedElsewhere):
		return db.User{}, refuse(OIDCLinkedElsewhere, "%s is linked to another account at the identity provider", email)
	case errors.Is(err, db.ErrLinkRequired):
		return db.User{}, refuse(OIDCLinkRequired, "%s has a password, so the account must be linked from the profile", email)
	case errors.Is(err, db.ErrUserLimit):
		return db.User{}, refuse(OIDCTooManyUsers, "%d users exist", MaxNumUsers)
	case err != nil:
		return db.User{}, err
	}

	u := res.User

	switch {
	case res.Created:
		logger.LogAuditEvent(r.Context(), CreateUserAction, u.Email, getClientIP(r),
			fmt.Sprintf("User %s created from the identity provider with role %d", u.Email, u.RoleID))
	case res.Linked:
		logger.LogAuditEvent(r.Context(), LinkUserAction, u.Email, getClientIP(r), "User linked to the identity provider account "+cl.Subject)
	}

	switch {
	case res.RoleKept:
		logger.LogAuditEvent(r.Context(), UpdateUserAction, u.Email, getClientIP(r),
			fmt.Sprintf("Role %d from the identity provider not applied to %s, the last admin with a password", role, u.Email))
	case res.RoleChanged():
		logger.LogAuditEvent(r.Context(), UpdateUserAction, u.Email, getClientIP(r),
			fmt.Sprintf("Role of %s set to %d from the identity provider", u.Email, u.RoleID))
	}

	setSessionCookie(w, rawToken, expiresAt, secureCookie)

	return u, nil
}

func groupRole(c *oidc.Client, cl oidc.Claims) (db.RoleID, error) {
	if !c.MapsGroups() {
		return 0, nil
	}

	if cl.GroupsOverage {
		return 0, refuse(OIDCGroupsOverage, "%v", oidc.ErrOverage)
	}

	role, ok := c.RoleFor(cl.Groups)
	if !ok {
		if len(cl.Groups) == 0 {
			return 0, refuse(OIDCNoGroup, "the user has no groups")
		}

		return 0, refuse(OIDCNoGroup, "none of the user's %d groups is mapped to a role: %s", len(cl.Groups), listGroups(cl.Groups))
	}

	return role, nil
}

func oidcDone(w http.ResponseWriter, r *http.Request, who string, link bool, err error) {
	outcome, target, failure := OIDCSuccess, "/", "/login?"
	if link {
		target, failure = "/profile", "/profile?"
	}

	var refusal oidcRefusal

	switch {
	case err == nil && link:
	case err == nil:
		logger.LogAuditEvent(r.Context(), OIDCLoginAction, who, getClientIP(r), "User logged in through OIDC")
	case errors.As(err, &refusal):
		outcome = refusal.outcome
		logger.LogAuditEvent(r.Context(), OIDCLoginAction, who, getClientIP(r), "User failed to log in through OIDC: "+refusal.Error())
	default:
		outcome = OIDCError

		logger.From(r.Context(), logger.APILog).Error("OIDC login failed", zap.Error(err))
		logger.LogAuditEvent(r.Context(), OIDCLoginAction, who, getClientIP(r), "User failed to log in through OIDC: internal error")
	}

	trackAuthAttempt("oidc", outcome == OIDCSuccess)

	if outcome != OIDCSuccess {
		target = failure + url.Values{"error": {outcome}}.Encode()
	}

	http.Redirect(w, r, target, http.StatusSeeOther)
}

func newID() string {
	id, err := uuid.NewV7()
	if err != nil {
		return uuid.NewString()
	}

	return id.String()
}

func normalEmail(s string) (string, bool) {
	a, err := mail.ParseAddress(s)
	if err != nil || a.Address != s {
		return "", false
	}

	return strings.Map(func(r rune) rune {
		if 'A' <= r && r <= 'Z' {
			return r + 'a' - 'A'
		}

		return r
	}, s), true
}

func actor(cl oidc.Claims) string {
	if cl.Email != "" {
		return cl.Email
	}

	return cl.Subject
}

func listGroups(groups []string) string {
	shown := groups[:min(len(groups), maxListedGroups)]
	quoted := make([]string, len(shown))

	for i, g := range shown {
		if len(g) > maxListedGroupBytes {
			g = strings.ToValidUTF8(g[:maxListedGroupBytes], "") + "…"
		}

		quoted[i] = strconv.Quote(g)
	}

	list := strings.Join(quoted, ", ")
	if more := len(groups) - len(shown); more > 0 {
		list += fmt.Sprintf(", and %d more", more)
	}

	return list
}

func clip(s string) string {
	s = strings.Map(func(r rune) rune {
		if r < 0x20 || r == 0x7f {
			return -1
		}

		return r
	}, s)

	if len(s) > maxErrorDetail {
		s = strings.ToValidUTF8(s[:maxErrorDetail], "")
	}

	return s
}
