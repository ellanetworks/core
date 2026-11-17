// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package server_test

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ellanetworks/core/internal/api/server"
	"github.com/go-jose/go-jose/v4"
)

const (
	ssoClientID     = "ella-core"
	ssoClientSecret = "s3cret-value"
	ssoExternalURL  = "https://core.example.com"
)

type rawResponse struct {
	Result json.RawMessage `json:"result"`
	Error  string          `json:"error,omitempty"`
}

type ssoUser struct {
	Email       string `json:"email"`
	RoleID      int    `json:"role_id"`
	PasswordSet bool   `json:"password_set"`
	OIDCLinked  bool   `json:"oidc_linked"`
}

type fakeIdP struct {
	server *httptest.Server
	issuer string
	signer jose.Signer
	keys   jose.JSONWebKeySet

	mu    sync.Mutex
	codes map[string]issuedCode
}

type issuedCode struct {
	challenge string
	idToken   string
}

func newFakeIdP(t *testing.T) *fakeIdP {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}

	signer, err := jose.NewSigner(jose.SigningKey{Algorithm: jose.ES256, Key: jose.JSONWebKey{Key: key, KeyID: "sig"}},
		(&jose.SignerOptions{}).WithType("JWT"))
	if err != nil {
		t.Fatal(err)
	}

	idp := &fakeIdP{signer: signer, codes: map[string]issuedCode{}}
	idp.keys = jose.JSONWebKeySet{Keys: []jose.JSONWebKey{{Key: key.Public(), KeyID: "sig", Algorithm: "ES256", Use: "sig"}}}
	idp.server = httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		switch r.URL.Path {
		case "/realms/ella/.well-known/openid-configuration":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"issuer":                                         idp.issuer,
				"authorization_endpoint":                         idp.issuer + "/auth",
				"token_endpoint":                                 idp.issuer + "/token",
				"jwks_uri":                                       idp.issuer + "/keys",
				"response_types_supported":                       []string{"code"},
				"id_token_signing_alg_values_supported":          []string{"ES256"},
				"code_challenge_methods_supported":               []string{"S256"},
				"authorization_response_iss_parameter_supported": true,
				"end_session_endpoint":                           idp.issuer + "/logout",
			})
		case "/realms/ella/keys":
			_ = json.NewEncoder(w).Encode(idp.keys)
		case "/realms/ella/token":
			idp.token(w, r)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(idp.server.Close)

	idp.issuer = idp.server.URL + "/realms/ella"

	return idp
}

func (idp *fakeIdP) ca() string {
	return string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: idp.server.Certificate().Raw}))
}

func (idp *fakeIdP) token(w http.ResponseWriter, r *http.Request) {
	user, pass, ok := r.BasicAuth()
	_ = r.ParseForm()

	idp.mu.Lock()
	issued, found := idp.codes[r.PostForm.Get("code")]
	delete(idp.codes, r.PostForm.Get("code"))
	idp.mu.Unlock()

	sum := sha256.Sum256([]byte(r.PostForm.Get("code_verifier")))

	switch {
	case !ok || user != ssoClientID || pass != ssoClientSecret:
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error": "invalid_client"}`))
	case !found || r.PostForm.Get("redirect_uri") != ssoExternalURL+"/api/v1/auth/oidc/callback" ||
		base64.RawURLEncoding.EncodeToString(sum[:]) != issued.challenge:
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error": "invalid_grant"}`))
	default:
		_ = json.NewEncoder(w).Encode(map[string]any{
			"access_token": rand.Text(), "token_type": "Bearer", "expires_in": 300, "id_token": issued.idToken,
		})
	}
}

func (idp *fakeIdP) sign(t *testing.T, claims map[string]any) string {
	t.Helper()

	payload, err := json.Marshal(claims)
	if err != nil {
		t.Fatal(err)
	}

	jws, err := idp.signer.Sign(payload)
	if err != nil {
		t.Fatal(err)
	}

	token, err := jws.CompactSerialize()
	if err != nil {
		t.Fatal(err)
	}

	return token
}

type ssoEnv struct {
	env   testEnv
	admin *http.Client
	token string
	idp   *fakeIdP
}

func newSSOEnv(t *testing.T) *ssoEnv {
	t.Helper()

	env, err := setupServer(filepath.Join(t.TempDir(), "db.sqlite3"))
	if err != nil {
		t.Fatalf("couldn't create test server: %s", err)
	}

	t.Cleanup(env.Server.Close)

	admin := newTestClient(env.Server)

	token, err := initializeAndRefresh(env.Server.URL, admin)
	if err != nil {
		t.Fatalf("couldn't initialize: %s", err)
	}

	return &ssoEnv{env: env, admin: admin, token: token, idp: newFakeIdP(t)}
}

func (e *ssoEnv) do(t *testing.T, client *http.Client, token, method, path string, payload any) (int, rawResponse) {
	t.Helper()

	code, resp, err := apiDo[rawResponse](client, method, e.env.Server.URL+path, token, payload)
	if err != nil {
		t.Fatalf("%s %s: %v", method, path, err)
	}

	return code, *resp
}

func (e *ssoEnv) settings(change func(map[string]any)) map[string]any {
	o := map[string]any{
		"enabled":                  true,
		"external_url":             ssoExternalURL + "/",
		"issuer":                   e.idp.issuer,
		"client_id":                ssoClientID,
		"client_secret":            ssoClientSecret,
		"display_name":             "Keycloak",
		"scopes":                   []string{"openid", "email", "profile"},
		"session_lifetime_seconds": 28800,
		"ca_certificate":           e.idp.ca(),
		"group_roles":              []map[string]any{},
	}

	if change != nil {
		change(o)
	}

	return o
}

func (e *ssoEnv) configure(t *testing.T, change func(map[string]any)) {
	t.Helper()

	if code, resp := e.do(t, e.admin, e.token, http.MethodPut, "/api/v1/oidc", e.settings(change)); code != http.StatusOK {
		t.Fatalf("PUT /api/v1/oidc = %d %q", code, resp.Error)
	}
}

func (e *ssoEnv) browser() *http.Client {
	jar, _ := cookiejar.New(nil)

	return &http.Client{
		Transport:     e.env.Server.Client().Transport,
		Jar:           jar,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
}

type ssoLogin struct {
	claims   map[string]any
	callback func(url.Values)
	browser  *http.Client
	link     bool
}

func (e *ssoEnv) login(t *testing.T, l ssoLogin) (string, *http.Client) {
	t.Helper()

	browser, start, done, failed := e.browser(), "/api/v1/auth/oidc/login", "/", "/login"
	if l.browser != nil {
		browser = l.browser
	}

	if l.link {
		start, done, failed = start+"?link=1", "/profile", "/profile"
	}

	location := e.redirect(t, browser, start)
	if strings.HasPrefix(location, failed) {
		u, _ := url.Parse(location)
		return u.Query().Get("error"), browser
	}

	u, err := url.Parse(location)
	if err != nil {
		t.Fatal(err)
	}

	q := u.Query()
	now := time.Now()

	claims := map[string]any{
		"iss": e.idp.issuer, "sub": "sub-nm", "aud": ssoClientID, "iat": now.Unix(), "exp": now.Add(5 * time.Minute).Unix(),
		"nonce": q.Get("nonce"), "email": "nm@example.com", "email_verified": true,
	}

	for k, v := range l.claims {
		if v == nil {
			delete(claims, k)
		} else {
			claims[k] = v
		}
	}

	e.idp.mu.Lock()
	e.idp.codes["code"] = issuedCode{challenge: q.Get("code_challenge"), idToken: e.idp.sign(t, claims)}
	e.idp.mu.Unlock()

	callback := url.Values{"code": {"code"}, "state": {q.Get("state")}, "iss": {e.idp.issuer}}
	if l.callback != nil {
		l.callback(callback)
	}

	location = e.redirect(t, browser, "/api/v1/auth/oidc/callback?"+callback.Encode())
	if location == done {
		return server.OIDCSuccess, browser
	}

	u, err = url.Parse(location)
	if err != nil || u.Path != failed {
		t.Fatalf("callback redirects to %q", location)
	}

	return u.Query().Get("error"), browser
}

func (e *ssoEnv) redirect(t *testing.T, client *http.Client, path string) string {
	t.Helper()

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, e.env.Server.URL+path, nil)
	if err != nil {
		t.Fatal(err)
	}

	res, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}

	_ = res.Body.Close()

	if res.StatusCode != http.StatusSeeOther {
		t.Fatalf("GET %s = %d", path, res.StatusCode)
	}

	return res.Header.Get("Location")
}

func (e *ssoEnv) accessToken(t *testing.T, browser *http.Client) string {
	t.Helper()

	code, resp, err := refresh(e.env.Server.URL, browser)
	if err != nil || code != http.StatusOK {
		t.Fatalf("refresh = %d, %v", code, err)
	}

	return resp.Result.Token
}

func (e *ssoEnv) user(t *testing.T, email string) ssoUser {
	t.Helper()

	code, resp := e.do(t, e.admin, e.token, http.MethodGet, "/api/v1/users/"+email, nil)
	if code != http.StatusOK {
		t.Fatalf("GET user %s = %d %q", email, code, resp.Error)
	}

	var u ssoUser
	if err := json.Unmarshal(resp.Result, &u); err != nil {
		t.Fatal(err)
	}

	return u
}

func (e *ssoEnv) createUser(t *testing.T, payload map[string]any) {
	t.Helper()

	if code, resp := e.do(t, e.admin, e.token, http.MethodPost, "/api/v1/users", payload); code != http.StatusCreated {
		t.Fatalf("create user = %d %q", code, resp.Error)
	}
}

func TestOIDCSettings(t *testing.T) {
	e := newSSOEnv(t)

	code, resp := e.do(t, e.admin, e.token, http.MethodGet, "/api/v1/oidc", nil)
	if code != http.StatusOK {
		t.Fatalf("GET = %d %q", code, resp.Error)
	}

	var got server.OIDCSettingsResponse
	if err := json.Unmarshal(resp.Result, &got); err != nil {
		t.Fatal(err)
	}

	if got.Enabled || got.SessionLifetimeSeconds != 86400 || strings.Join(got.Scopes, " ") != "openid email profile" ||
		got.RedirectURL != "" || got.ClientSecretSet {
		t.Fatalf("default settings = %+v", got)
	}

	code, resp = e.do(t, e.admin, e.token, http.MethodPut, "/api/v1/oidc", e.settings(nil))
	if code != http.StatusOK {
		t.Fatalf("PUT = %d %q", code, resp.Error)
	}

	if strings.Contains(string(resp.Result), ssoClientSecret) {
		t.Fatalf("PUT response holds the client secret: %s", resp.Result)
	}

	if err := json.Unmarshal(resp.Result, &got); err != nil {
		t.Fatal(err)
	}

	if !got.Enabled || got.ExternalURL != ssoExternalURL || got.RedirectURL != ssoExternalURL+"/api/v1/auth/oidc/callback" ||
		!got.ClientSecretSet {
		t.Fatalf("settings = %+v", got)
	}

	keep := e.settings(func(o map[string]any) {
		delete(o, "client_secret")
		o["display_name"] = "Corporate SSO"
	})

	if code, resp := e.do(t, e.admin, e.token, http.MethodPut, "/api/v1/oidc", keep); code != http.StatusOK {
		t.Fatalf("PUT without a client secret = %d %q", code, resp.Error)
	}

	newClient := e.settings(func(o map[string]any) {
		delete(o, "client_secret")
		o["client_id"] = "another"
	})

	if code, _ := e.do(t, e.admin, e.token, http.MethodPut, "/api/v1/oidc", newClient); code != http.StatusBadRequest {
		t.Fatalf("PUT with a new client and no secret = %d, want %d", code, http.StatusBadRequest)
	}

	code, login := e.do(t, e.browser(), "", http.MethodGet, "/api/v1/auth/oidc", nil)
	if code != http.StatusOK || !strings.Contains(string(login.Result), "Corporate SSO") {
		t.Fatalf("GET /api/v1/auth/oidc = %d %s", code, login.Result)
	}
}

func TestOIDCSettingsAreValidated(t *testing.T) {
	e := newSSOEnv(t)

	tests := map[string]func(map[string]any){
		"an http issuer":              func(o map[string]any) { o["issuer"] = "http://idp.example.com" },
		"an http external URL":        func(o map[string]any) { o["external_url"] = "http://core.example.com" },
		"an external URL with a path": func(o map[string]any) { o["external_url"] = "https://example.com/core" },
		"no client ID":                func(o map[string]any) { o["client_id"] = "" },
		"no openid scope":             func(o map[string]any) { o["scopes"] = []string{"email"} },
		"a short session":             func(o map[string]any) { o["session_lifetime_seconds"] = 299 },
		"a CA that is not PEM":        func(o map[string]any) { o["ca_certificate"] = "not a certificate" },
		"group roles without a claim": func(o map[string]any) { o["group_roles"] = []map[string]any{{"group": "a", "role_id": 2}} },
		"a group with an unknown role": func(o map[string]any) {
			o["groups_claim"] = "groups"
			o["group_roles"] = []map[string]any{{"group": "a", "role_id": 4}}
		},
	}

	for name, change := range tests {
		if code, resp := e.do(t, e.admin, e.token, http.MethodPut, "/api/v1/oidc", e.settings(change)); code != http.StatusBadRequest {
			t.Errorf("PUT with %s = %d %q, want %d", name, code, resp.Error, http.StatusBadRequest)
		}
	}
}

func TestOIDCSettingsNeedAnAdmin(t *testing.T) {
	e := newSSOEnv(t)

	code, resp, err := createAPIToken(e.env.Server.URL, e.admin, e.token, &CreateAPITokenParams{Name: "admin"})
	if err != nil || code != http.StatusCreated {
		t.Fatalf("create API token = %d, %v", code, err)
	}

	if code, _ := e.do(t, e.admin, resp.Result.Token, http.MethodGet, "/api/v1/oidc", nil); code != http.StatusOK {
		t.Fatalf("GET with an admin API token = %d, want %d", code, http.StatusOK)
	}

	nm, err := createUserAndLogin(e.env.Server.URL, e.token, "nm@example.com", RoleNetworkManager, newTestClient(e.env.Server))
	if err != nil {
		t.Fatal(err)
	}

	if code, _ := e.do(t, e.admin, nm, http.MethodGet, "/api/v1/oidc", nil); code != http.StatusForbidden {
		t.Fatalf("GET as a network manager = %d, want %d", code, http.StatusForbidden)
	}
}

func TestDiscoverOIDC(t *testing.T) {
	e := newSSOEnv(t)

	code, resp := e.do(t, e.admin, e.token, http.MethodPost, "/api/v1/oidc/discovery",
		map[string]string{"issuer": e.idp.issuer, "ca_certificate": e.idp.ca()})
	if code != http.StatusOK {
		t.Fatalf("discovery = %d %q", code, resp.Error)
	}

	var d server.OIDCDiscoveryResponse
	if err := json.Unmarshal(resp.Result, &d); err != nil {
		t.Fatal(err)
	}

	if d.Issuer != e.idp.issuer || d.Keys != 1 || !d.PKCE || d.TokenEndpointAuthMethod != "client_secret_basic" {
		t.Fatalf("discovery = %+v", d)
	}

	if code, _ := e.do(t, e.admin, e.token, http.MethodPost, "/api/v1/oidc/discovery",
		map[string]string{"issuer": e.idp.issuer}); code != http.StatusBadGateway {
		t.Fatalf("discovery without the CA = %d, want %d", code, http.StatusBadGateway)
	}
}

func TestOIDCLoginLinksByEmailThenBySubject(t *testing.T) {
	e := newSSOEnv(t)
	e.configure(t, nil)
	e.createUser(t, map[string]any{"email": "NM@example.com", "role_id": 3})

	outcome, browser := e.login(t, ssoLogin{})
	if outcome != server.OIDCSuccess {
		t.Fatalf("login = %q", outcome)
	}

	token := e.accessToken(t, browser)

	code, resp := e.do(t, browser, token, http.MethodGet, "/api/v1/users/me", nil)
	if code != http.StatusOK {
		t.Fatalf("GET /users/me = %d %q", code, resp.Error)
	}

	if u := e.user(t, "NM@example.com"); !u.OIDCLinked || u.RoleID != 3 || u.PasswordSet {
		t.Fatalf("user = %+v", u)
	}

	outcome, _ = e.login(t, ssoLogin{claims: map[string]any{"email": "renamed@example.com", "email_verified": false}})
	if outcome != server.OIDCSuccess {
		t.Fatalf("login by subject = %q", outcome)
	}
}

func TestOIDCLoginRefusals(t *testing.T) {
	e := newSSOEnv(t)
	e.configure(t, nil)
	e.createUser(t, map[string]any{"email": "nm@example.com", "role_id": 3})

	tests := map[string]struct {
		login ssoLogin
		want  string
	}{
		"an unknown email": {ssoLogin{claims: map[string]any{"email": "nobody@example.com"}}, server.OIDCNotSetUp},
		"an unverified email": {
			ssoLogin{claims: map[string]any{"email_verified": false}}, server.OIDCEmailNotVerified,
		},
		"an email of unknown verification": {
			ssoLogin{claims: map[string]any{"email_verified": nil}}, server.OIDCEmailNotVerified,
		},
		"a refusal at the provider": {
			ssoLogin{callback: func(q url.Values) {
				q.Del("code")
				q.Set("error", "access_denied")
			}},
			server.OIDCDenied,
		},
		"another state": {ssoLogin{callback: func(q url.Values) { q.Set("state", "other") }}, server.OIDCFailed},
		"another issuer": {
			ssoLogin{callback: func(q url.Values) { q.Set("iss", "https://evil.example.com") }}, server.OIDCFailed,
		},
		"another audience": {ssoLogin{claims: map[string]any{"aud": "other"}}, server.OIDCFailed},
		"another nonce":    {ssoLogin{claims: map[string]any{"nonce": "other"}}, server.OIDCFailed},
	}

	for name, tc := range tests {
		if outcome, _ := e.login(t, tc.login); outcome != tc.want {
			t.Errorf("login with %s = %q, want %q", name, outcome, tc.want)
		}
	}

	if u := e.user(t, "nm@example.com"); u.OIDCLinked {
		t.Fatalf("a refused login linked the user: %+v", u)
	}
}

func TestOIDCLoginIsSingleUse(t *testing.T) {
	e := newSSOEnv(t)
	e.configure(t, nil)
	e.createUser(t, map[string]any{"email": "nm@example.com", "role_id": 3})

	var replay string

	outcome, browser := e.login(t, ssoLogin{callback: func(q url.Values) { replay = q.Encode() }})
	if outcome != server.OIDCSuccess {
		t.Fatalf("login = %q", outcome)
	}

	location := e.redirect(t, browser, "/api/v1/auth/oidc/callback?"+replay)
	if location == "/" {
		t.Fatal("the callback was accepted twice")
	}
}

func TestOIDCLoginRolesFromGroups(t *testing.T) {
	e := newSSOEnv(t)
	e.configure(t, func(o map[string]any) {
		o["groups_claim"] = "groups"
		o["group_roles"] = []map[string]any{{"group": "noc", "role_id": 3}, {"group": "admins", "role_id": 1}}
	})

	if outcome, _ := e.login(t, ssoLogin{claims: map[string]any{"groups": []string{"guests"}}}); outcome != server.OIDCNoGroup {
		t.Fatalf("login in no mapped group = %q, want %q", outcome, server.OIDCNoGroup)
	}

	outcome, _ := e.login(t, ssoLogin{claims: map[string]any{"groups": []string{"noc", "guests"}}})
	if outcome != server.OIDCSuccess {
		t.Fatalf("login in a mapped group = %q", outcome)
	}

	if u := e.user(t, "nm@example.com"); u.RoleID != 3 || u.PasswordSet || !u.OIDCLinked {
		t.Fatalf("created user = %+v", u)
	}

	if code, _ := e.do(t, e.admin, e.token, http.MethodPut, "/api/v1/users/nm@example.com", map[string]any{"role_id": 2}); code != http.StatusConflict {
		t.Fatalf("change a role from groups = %d, want %d", code, http.StatusConflict)
	}

	if outcome, _ := e.login(t, ssoLogin{claims: map[string]any{"groups": []string{"noc", "admins"}}}); outcome != server.OIDCSuccess {
		t.Fatalf("login in two mapped groups = %q", outcome)
	}

	if u := e.user(t, "nm@example.com"); u.RoleID != 1 {
		t.Fatalf("role in two mapped groups = %d, want the highest, 1", u.RoleID)
	}

	if outcome, _ := e.login(t, ssoLogin{claims: map[string]any{"groups": []string{}}}); outcome != server.OIDCNoGroup {
		t.Fatalf("login after leaving every mapped group = %q, want %q", outcome, server.OIDCNoGroup)
	}
}

func TestOIDCLinkFromTheProfile(t *testing.T) {
	e := newSSOEnv(t)
	e.configure(t, nil)
	e.createUser(t, map[string]any{"email": "nm@example.com", "password": "password123", "role_id": 3})

	if outcome, _ := e.login(t, ssoLogin{}); outcome != server.OIDCLinkRequired {
		t.Fatalf("login of a user with a password = %q, want %q", outcome, server.OIDCLinkRequired)
	}

	if outcome, _ := e.login(t, ssoLogin{link: true}); outcome != server.OIDCFailed {
		t.Fatalf("link without a session = %q, want %q", outcome, server.OIDCFailed)
	}

	browser := e.browser()

	if code, _, err := login(e.env.Server.URL, browser, &LoginParams{Email: "nm@example.com", Password: "password123"}); err != nil || code != http.StatusOK {
		t.Fatalf("password login = %d, %v", code, err)
	}

	if outcome, _ := e.login(t, ssoLogin{browser: browser, link: true, claims: map[string]any{"email": "other@example.com"}}); outcome != server.OIDCSuccess {
		t.Fatalf("link = %q", outcome)
	}

	if u := e.user(t, "nm@example.com"); !u.OIDCLinked {
		t.Fatalf("user after linking = %+v", u)
	}

	if outcome, _ := e.login(t, ssoLogin{}); outcome != server.OIDCSuccess {
		t.Fatalf("login after linking = %q", outcome)
	}
}

func TestLogoutEndsTheIdentityProviderSession(t *testing.T) {
	e := newSSOEnv(t)
	e.configure(t, nil)
	e.createUser(t, map[string]any{"email": "nm@example.com", "role_id": 3})

	outcome, browser := e.login(t, ssoLogin{})
	if outcome != server.OIDCSuccess {
		t.Fatalf("login = %q", outcome)
	}

	code, resp := e.do(t, browser, "", http.MethodPost, "/api/v1/auth/logout", nil)
	if code != http.StatusOK {
		t.Fatalf("logout = %d", code)
	}

	var out server.LogoutResponse
	if err := json.Unmarshal(resp.Result, &out); err != nil {
		t.Fatal(err)
	}

	u, err := url.Parse(out.RedirectURL)
	if err != nil || u.Path != "/realms/ella/logout" || u.Query().Get("client_id") != ssoClientID ||
		u.Query().Get("post_logout_redirect_uri") != ssoExternalURL+"/login" {
		t.Fatalf("redirect_url = %q", out.RedirectURL)
	}

	password := e.browser()

	if code, _, err := login(e.env.Server.URL, password, &LoginParams{Email: FirstUserEmail, Password: "password123"}); err != nil || code != http.StatusOK {
		t.Fatalf("password login = %d, %v", code, err)
	}

	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, e.env.Server.URL+"/api/v1/auth/logout", nil)
	if err != nil {
		t.Fatal(err)
	}

	res, err := password.Do(req)
	if err != nil {
		t.Fatal(err)
	}

	_ = res.Body.Close()

	if res.StatusCode != http.StatusNoContent {
		t.Fatalf("logout of a password session = %d, want %d", res.StatusCode, http.StatusNoContent)
	}
}

func TestOIDCSessionsEndWithTheSettings(t *testing.T) {
	e := newSSOEnv(t)
	e.configure(t, nil)
	e.createUser(t, map[string]any{"email": "nm@example.com", "role_id": 3})

	outcome, browser := e.login(t, ssoLogin{})
	if outcome != server.OIDCSuccess {
		t.Fatalf("login = %q", outcome)
	}

	token := e.accessToken(t, browser)

	e.configure(t, func(o map[string]any) { o["enabled"] = false })

	if code, _ := e.do(t, browser, token, http.MethodGet, "/api/v1/users/me", nil); code != http.StatusUnauthorized {
		t.Fatalf("access token after disabling = %d, want %d", code, http.StatusUnauthorized)
	}

	if code, _, _ := refresh(e.env.Server.URL, browser); code != http.StatusUnauthorized {
		t.Fatalf("refresh after disabling = %d, want %d", code, http.StatusUnauthorized)
	}

	if outcome, _ := e.login(t, ssoLogin{}); outcome != server.OIDCUnavailable {
		t.Fatalf("login when disabled = %q, want %q", outcome, server.OIDCUnavailable)
	}
}

func TestUnlinkEndsOIDCSessions(t *testing.T) {
	e := newSSOEnv(t)
	e.configure(t, nil)
	e.createUser(t, map[string]any{"email": "nm@example.com", "role_id": 3})

	if code, _ := e.do(t, e.admin, e.token, http.MethodDelete, "/api/v1/users/nm@example.com/oidc", nil); code != http.StatusNotFound {
		t.Fatalf("unlink a user that is not linked = %d, want %d", code, http.StatusNotFound)
	}

	outcome, browser := e.login(t, ssoLogin{})
	if outcome != server.OIDCSuccess {
		t.Fatalf("login = %q", outcome)
	}

	token := e.accessToken(t, browser)

	if code, resp := e.do(t, e.admin, e.token, http.MethodDelete, "/api/v1/users/nm@example.com/oidc", nil); code != http.StatusOK {
		t.Fatalf("unlink = %d %q", code, resp.Error)
	}

	if code, _ := e.do(t, browser, token, http.MethodGet, "/api/v1/users/me", nil); code != http.StatusUnauthorized {
		t.Fatalf("access token after unlinking = %d, want %d", code, http.StatusUnauthorized)
	}

	if u := e.user(t, "nm@example.com"); u.OIDCLinked {
		t.Fatalf("user after unlinking = %+v", u)
	}
}

func TestUsersWithoutAPassword(t *testing.T) {
	e := newSSOEnv(t)

	e.createUser(t, map[string]any{"email": "sso@example.com", "role_id": 1})

	if u := e.user(t, "sso@example.com"); u.PasswordSet || u.RoleID != 1 {
		t.Fatalf("user = %+v", u)
	}

	if code, _, _ := login(e.env.Server.URL, e.browser(), &LoginParams{Email: "sso@example.com", Password: "anything"}); code != http.StatusUnauthorized {
		t.Fatalf("password login without a password = %d, want %d", code, http.StatusUnauthorized)
	}

	if code, _ := e.do(t, e.admin, e.token, http.MethodDelete, "/api/v1/users/sso@example.com/password", nil); code != http.StatusConflict {
		t.Fatalf("remove a missing password = %d, want %d", code, http.StatusConflict)
	}

	other := newTestClient(e.env.Server)

	second, err := createUserAndLogin(e.env.Server.URL, e.token, "second@example.com", RoleAdmin, other)
	if err != nil {
		t.Fatal(err)
	}

	for _, tc := range []struct {
		method, path string
		payload      any
	}{
		{http.MethodDelete, "/api/v1/users/" + FirstUserEmail + "/password", nil},
		{http.MethodPut, "/api/v1/users/" + FirstUserEmail, map[string]any{"role_id": 2}},
	} {
		if code, resp := e.do(t, other, second, tc.method, tc.path, tc.payload); code != http.StatusOK {
			t.Fatalf("%s %s with another password admin = %d %q", tc.method, tc.path, code, resp.Error)
		}
	}

	e.admin, e.token = other, second

	e.configure(t, func(o map[string]any) {
		o["groups_claim"] = "groups"
		o["group_roles"] = []map[string]any{{"group": "admins", "role_id": 1}}
	})

	outcome, browser := e.login(t, ssoLogin{claims: map[string]any{"email": "boss@example.com", "sub": "sub-boss", "groups": []string{"admins"}}})
	if outcome != server.OIDCSuccess {
		t.Fatalf("login = %q", outcome)
	}

	boss := e.accessToken(t, browser)

	for _, tc := range []struct {
		method, path string
		payload      any
	}{
		{http.MethodDelete, "/api/v1/users/second@example.com/password", nil},
		{http.MethodPut, "/api/v1/users/second@example.com", map[string]any{"role_id": 2}},
		{http.MethodDelete, "/api/v1/users/second@example.com", nil},
	} {
		if code, resp := e.do(t, browser, boss, tc.method, tc.path, tc.payload); code != http.StatusConflict {
			t.Errorf("%s %s of the last admin with a password = %d %q, want %d", tc.method, tc.path, code, resp.Error, http.StatusConflict)
		}
	}

	if code, _ := e.do(t, browser, boss, http.MethodPut, "/api/v1/users/me/password",
		map[string]any{"current_password": "", "password": "a new password"}); code != http.StatusConflict {
		t.Fatalf("change my password without one = %d, want %d", code, http.StatusConflict)
	}
}
