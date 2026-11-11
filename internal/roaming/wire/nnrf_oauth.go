// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package wire

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"time"
)

const (
	ContentTypeForm         = "application/x-www-form-urlencoded"
	GrantTypeClientCreds    = "client_credentials"
	TokenTypeBearer         = "Bearer"
	maxAccessTokenLength    = 16 << 10
	accessTokenEndpointPath = "/oauth2/token" // #nosec G101 -- URL path of the NRF token endpoint, not a credential
)

var scopePattern = regexp.MustCompile(`^([a-zA-Z0-9_:-]+)( [a-zA-Z0-9_:-]+)*$`)

func AccessTokenPath() string {
	return accessTokenEndpointPath
}

type AccessTokenReq struct {
	NfInstanceID  string
	NfType        NFType
	TargetNfType  NFType
	Scope         string
	RequesterPlmn *PlmnID
	TargetPlmn    *PlmnID
}

func (r AccessTokenReq) Values() (url.Values, error) {
	if !ValidNfInstanceID(r.NfInstanceID) {
		return nil, Incorrect("/nfInstanceId", false, "invalid NF instance ID %q", r.NfInstanceID)
	}

	if !scopePattern.MatchString(r.Scope) {
		return nil, Incorrect("/scope", false, "invalid scope %q", r.Scope)
	}

	v := url.Values{}
	v.Set("grant_type", GrantTypeClientCreds)
	v.Set("nfInstanceId", r.NfInstanceID)
	v.Set("scope", r.Scope)

	for name, t := range map[string]NFType{"nfType": r.NfType, "targetNfType": r.TargetNfType} {
		if t == "" {
			continue
		}

		if !t.Known() {
			return nil, Incorrect("/"+name, false, "invalid NF type %q", t)
		}

		v.Set(name, string(t))
	}

	for name, p := range map[string]*PlmnID{"requesterPlmn": r.RequesterPlmn, "targetPlmn": r.TargetPlmn} {
		if p == nil {
			continue
		}

		if err := p.Check(Strict); err != nil {
			return nil, Nest("/"+name, err)
		}

		b, err := json.Marshal(p)
		if err != nil {
			return nil, err
		}

		v.Set(name, string(b))
	}

	return v, nil
}

type AccessTokenRsp struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   *int   `json:"expires_in,omitempty"`
	Scope       string `json:"scope,omitempty"`
}

func (r AccessTokenRsp) Check(Policy) error {
	if err := firstError(
		present("/access_token", r.AccessToken != ""),
		present("/token_type", r.TokenType != ""),
	); err != nil {
		return err
	}

	if !strings.EqualFold(r.TokenType, TokenTypeBearer) {
		return Incorrect("/token_type", false, "token type %q is not Bearer", r.TokenType)
	}

	if len(r.AccessToken) > maxAccessTokenLength || strings.ContainsAny(r.AccessToken, " \t\r\n") {
		return Incorrect("/access_token", false, "malformed access token")
	}

	if r.ExpiresIn != nil && *r.ExpiresIn < 0 {
		return Incorrect("/expires_in", true, "negative lifetime %d", *r.ExpiresIn)
	}

	return nil
}

func (r AccessTokenRsp) Lifetime(now time.Time) (time.Duration, bool) {
	if r.ExpiresIn != nil {
		return time.Duration(*r.ExpiresIn) * time.Second, true
	}

	parts := strings.Split(r.AccessToken, ".")
	if len(parts) != 3 {
		return 0, false
	}

	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return 0, false
	}

	var claims struct {
		Exp *int64 `json:"exp"`
	}

	if json.Unmarshal(payload, &claims) != nil || claims.Exp == nil {
		return 0, false
	}

	return time.Unix(*claims.Exp, 0).Sub(now), true
}

type AccessTokenErr struct {
	Error            string `json:"error"`
	ErrorDescription string `json:"error_description,omitempty"`
}

func (e AccessTokenErr) Check(Policy) error {
	return present("/error", e.Error != "")
}

func (e AccessTokenErr) String() string {
	if e.ErrorDescription == "" {
		return e.Error
	}

	return fmt.Sprintf("%s: %s", e.Error, e.ErrorDescription)
}
