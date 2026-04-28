// Contains helper functions for testing the server
package server_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/netip"
	"strings"

	"github.com/ellanetworks/core/etsi"
	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/api/server"
	"github.com/ellanetworks/core/internal/config"
	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/kernel"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/models"
	ellaraft "github.com/ellanetworks/core/internal/raft"
	"github.com/ellanetworks/core/internal/smf"
	"github.com/ellanetworks/core/internal/supportbundle"
	"golang.org/x/crypto/bcrypt"
)

const (
	FirstUserEmail = "my.user123@ellanetworks.com"
)

type FakeKernel struct{}

func (fk FakeKernel) CreateRoute(destination netip.Prefix, gateway netip.Addr, priority int, networkInterface kernel.NetworkInterface) error {
	return nil
}

func (fk FakeKernel) DeleteRoute(destination netip.Prefix, gateway netip.Addr, priority int, networkInterface kernel.NetworkInterface) error {
	return nil
}

func (fk FakeKernel) InterfaceExists(networkInterface kernel.NetworkInterface) (bool, error) {
	return true, nil
}

func (fk FakeKernel) RouteExists(destination netip.Prefix, gateway netip.Addr, priority int, networkInterface kernel.NetworkInterface) (bool, error) {
	return false, nil
}

func (fk FakeKernel) EnableIPForwarding() error {
	return nil
}

func (fk FakeKernel) IsIPForwardingEnabled() (bool, error) {
	return true, nil
}

func (fk FakeKernel) ReplaceRoute(destination netip.Prefix, gateway netip.Addr, priority int, networkInterface kernel.NetworkInterface) error {
	return nil
}

func (fk FakeKernel) ListRoutesByPriority(priority int, networkInterface kernel.NetworkInterface) ([]netip.Prefix, error) {
	return nil, nil
}

func (fk FakeKernel) ListManagedRoutes(networkInterface kernel.NetworkInterface) ([]kernel.ManagedRoute, error) {
	return nil, nil
}

func (fk FakeKernel) EnsureGatewaysOnInterfaceInNeighTable(ifKey kernel.NetworkInterface) error {
	return nil
}

type dummyFS struct{}

func (dummyFS) Open(name string) (fs.File, error) {
	return nil, fs.ErrNotExist
}

// testEnv holds the components created by setupServer.
type testEnv struct {
	Server    *httptest.Server
	JWTSecret *server.JWTSecret
	DB        *db.Database
	SMF       *smf.SMF
	AMF       *amf.AMF
}

func setupServer(filepath string) (testEnv, error) {
	testdb, err := db.NewDatabaseWithoutRaft(context.Background(), filepath)
	if err != nil {
		return testEnv{}, err
	}

	return buildTestEnv(testdb)
}

// setupServerWithRaft is the slow path for tests that exercise cluster /
// restore behavior and therefore need a live Raft manager.
func setupServerWithRaft(filepath string) (testEnv, error) {
	testdb, err := db.NewDatabase(context.Background(), filepath, ellaraft.ClusterConfig{})
	if err != nil {
		return testEnv{}, err
	}

	return buildTestEnv(testdb)
}

func buildTestEnv(testdb *db.Database) (testEnv, error) {
	logger.SetDb(testdb)

	// Initialize SMF context with test stubs
	smfInstance := smf.New(&fakePCF{}, &fakeSessionStore{}, &fakeUPFClient{}, &fakeAMFCallback{})

	jwtSecret := server.NewJWTSecret([]byte("testsecret"))
	dummyfs := dummyFS{}

	cfg := config.Config{
		Interfaces: config.Interfaces{
			N2: config.N2Interface{
				Address: "12.12.12.12",
				Port:    2152,
			},
			N3: config.N3Interface{
				Address: "13.13.13.13",
			},
			N6: config.N6Interface{
				Name: "eth1",
			},
			API: config.APIInterface{
				Port: 8443,
			},
		},
	}

	amfInstance := amf.New(testdb, nil, smfInstance)
	ts := httptest.NewTLSServer(server.NewHandler(server.HandlerConfig{
		DB:           testdb,
		Config:       cfg,
		JWTSecret:    jwtSecret,
		SecureCookie: false,
		FrontendFS:   dummyfs,
		Sessions:     smfInstance,
		AMF:          amfInstance,
		BcryptCost:   bcrypt.MinCost,
	}))

	supportbundle.ConfigProvider = func(ctx context.Context) ([]byte, error) {
		return []byte("fake test config"), nil
	}

	return testEnv{
		Server:    ts,
		JWTSecret: jwtSecret,
		DB:        testdb,
		SMF:       smfInstance,
		AMF:       amfInstance,
	}, nil
}

// newTestClient returns an independent HTTP client for the given test server.
// Each call creates a fresh cookie jar, so different users/sessions don't
// interfere with each other. The TLS transport is shared from the server.
func newTestClient(ts *httptest.Server) *http.Client {
	jar, _ := cookiejar.New(nil)

	return &http.Client{
		Transport: ts.Client().Transport,
		Jar:       jar,
	}
}

func initializeAndRefresh(url string, client *http.Client) (string, error) {
	initParams := &InitializeParams{
		Email:    FirstUserEmail,
		Password: "password123",
	}

	statusCode, initResponse, err := initialize(url, client, initParams)
	if err != nil {
		return "", fmt.Errorf("couldn't create user: %s", err)
	}

	if statusCode != http.StatusCreated {
		return "", fmt.Errorf("expected status %d, got %d", http.StatusCreated, statusCode)
	}

	if initResponse.Result.Token == "" {
		return "", fmt.Errorf("expected non-empty token from initialize")
	}

	return initResponse.Result.Token, nil
}

func createUserAndLogin(url string, token string, email string, roleID RoleID, client *http.Client) (string, error) {
	user := &CreateUserParams{
		Email:    email,
		Password: "password123",
		RoleID:   roleID,
	}

	statusCode, _, err := createUser(url, client, token, user)
	if err != nil {
		return "", fmt.Errorf("couldn't create user: %s", err)
	}

	if statusCode != http.StatusCreated {
		return "", fmt.Errorf("expected status %d, got %d", http.StatusCreated, statusCode)
	}

	loginParams := &LoginParams{
		Email:    email,
		Password: "password123",
	}

	statusCode, loginResp, err := login(url, client, loginParams)
	if err != nil {
		return "", fmt.Errorf("couldn't login: %s", err)
	}

	if statusCode != http.StatusOK {
		return "", fmt.Errorf("expected status %d, got %d", http.StatusOK, statusCode)
	}

	if loginResp.Result.Token == "" {
		return "", fmt.Errorf("expected non-empty token from login")
	}

	return loginResp.Result.Token, nil
}

// Stub adapters for SMF initialization in tests.

type fakePCF struct{}

func (f *fakePCF) GetSessionPolicy(_ context.Context, _ string, _ *models.Snssai, _ string) (*smf.Policy, error) {
	return nil, fmt.Errorf("not implemented in test")
}

type fakeSessionStore struct{}

func (f *fakeSessionStore) AllocateIP(_ context.Context, _ string, _ string, _ uint8) (netip.Addr, error) {
	return netip.Addr{}, fmt.Errorf("not implemented in test")
}

func (f *fakeSessionStore) ReleaseIP(_ context.Context, _ string, _ string, _ uint8) (netip.Addr, error) {
	return netip.Addr{}, fmt.Errorf("not implemented in test")
}

func (f *fakeSessionStore) IncrementDailyUsage(_ context.Context, _ string, _, _ uint64) error {
	return nil
}

func (f *fakeSessionStore) InsertFlowReports(_ context.Context, _ []*models.FlowReportRequest) error {
	return nil
}

type fakeUPFClient struct{}

func (f *fakeUPFClient) EstablishSession(ctx context.Context, req *models.EstablishRequest) (*models.EstablishResponse, error) {
	return nil, fmt.Errorf("not implemented in test")
}

func (f *fakeUPFClient) ModifySession(ctx context.Context, req *models.ModifyRequest) error {
	return nil
}

func (f *fakeUPFClient) DeleteSession(ctx context.Context, remoteSEID uint64) error {
	return nil
}

func (f *fakeUPFClient) FlushUsage(ctx context.Context, remoteSEID uint64) {}

func (f *fakeUPFClient) UpdateFilters(ctx context.Context, policyID int64, direction models.Direction, rules []models.FilterRule) error {
	return nil
}

type fakeAMFCallback struct{}

func (f *fakeAMFCallback) TransferN1(ctx context.Context, supi etsi.SUPI, n1Msg []byte, pduSessionID uint8) error {
	return nil
}

func (f *fakeAMFCallback) TransferN1N2(ctx context.Context, supi etsi.SUPI, pduSessionID uint8, snssai *models.Snssai, n1Msg, n2Msg []byte) error {
	return nil
}

func (f *fakeAMFCallback) N2TransferOrPage(ctx context.Context, supi etsi.SUPI, pduSessionID uint8, snssai *models.Snssai, n2Msg []byte) error {
	return nil
}

// ── Profile test helpers ────────────────────────────────────────────────

type CreateProfileParams struct {
	Name           string `json:"name"`
	UeAmbrUplink   string `json:"ue_ambr_uplink"`
	UeAmbrDownlink string `json:"ue_ambr_downlink"`
}

type ProfileResponse struct {
	Name           string `json:"name"`
	UeAmbrUplink   string `json:"ue_ambr_uplink"`
	UeAmbrDownlink string `json:"ue_ambr_downlink"`
}

type CreateProfileResponseResult struct {
	Message string `json:"message"`
}

type CreateProfileResponse struct {
	Result CreateProfileResponseResult `json:"result"`
	Error  string                      `json:"error,omitempty"`
}

type DeleteProfileResponseResult struct {
	Message string `json:"message"`
}

type DeleteProfileResponse struct {
	Result DeleteProfileResponseResult `json:"result"`
	Error  string                      `json:"error,omitempty"`
}

type GetProfileResponse struct {
	Result ProfileResponse `json:"result"`
	Error  string          `json:"error,omitempty"`
}

type ListProfilesResponseResult struct {
	Items      []ProfileResponse `json:"items"`
	Page       int               `json:"page"`
	PerPage    int               `json:"per_page"`
	TotalCount int               `json:"total_count"`
}

type ListProfilesResponse struct {
	Result ListProfilesResponseResult `json:"result"`
	Error  string                     `json:"error,omitempty"`
}

func createProfile(url string, client *http.Client, token string, data *CreateProfileParams) (int, *CreateProfileResponse, error) {
	body, err := json.Marshal(data)
	if err != nil {
		return 0, nil, err
	}

	req, err := http.NewRequestWithContext(context.Background(), "POST", url+"/api/v1/profiles", strings.NewReader(string(body)))
	if err != nil {
		return 0, nil, err
	}

	req.Header.Set("Authorization", "Bearer "+token)

	res, err := client.Do(req)
	if err != nil {
		return 0, nil, err
	}

	defer func() {
		if err := res.Body.Close(); err != nil {
			panic(err)
		}
	}()

	var createResponse CreateProfileResponse
	if err := json.NewDecoder(res.Body).Decode(&createResponse); err != nil {
		return 0, nil, err
	}

	return res.StatusCode, &createResponse, nil
}

// ── Slice test helpers ──────────────────────────────────────────────────

type CreateSliceParams struct {
	Name string `json:"name"`
	Sst  int    `json:"sst"`
	Sd   string `json:"sd,omitempty"`
}

type SliceResponse struct {
	Name string `json:"name"`
	Sst  int    `json:"sst"`
	Sd   string `json:"sd,omitempty"`
}

type CreateSliceResponseResult struct {
	Message string `json:"message"`
}

type CreateSliceResponse struct {
	Result CreateSliceResponseResult `json:"result"`
	Error  string                    `json:"error,omitempty"`
}

type UpdateSliceParams struct {
	Sst int    `json:"sst"`
	Sd  string `json:"sd,omitempty"`
}

type UpdateSliceResponseResult struct {
	Message string `json:"message"`
}

type UpdateSliceResponse struct {
	Result UpdateSliceResponseResult `json:"result"`
	Error  string                    `json:"error,omitempty"`
}

type GetSliceResponse struct {
	Result SliceResponse `json:"result"`
	Error  string        `json:"error,omitempty"`
}

type ListSlicesResponseResult struct {
	Items      []SliceResponse `json:"items"`
	Page       int             `json:"page"`
	PerPage    int             `json:"per_page"`
	TotalCount int             `json:"total_count"`
}

type ListSlicesResponse struct {
	Result ListSlicesResponseResult `json:"result"`
	Error  string                   `json:"error,omitempty"`
}

type DeleteSliceResponseResult struct {
	Message string `json:"message"`
}

type DeleteSliceResponse struct {
	Result DeleteSliceResponseResult `json:"result"`
	Error  string                    `json:"error,omitempty"`
}

type UpdateProfileParams struct {
	UeAmbrUplink   string `json:"ue_ambr_uplink"`
	UeAmbrDownlink string `json:"ue_ambr_downlink"`
}

type UpdateProfileResponseResult struct {
	Message string `json:"message"`
}

type UpdateProfileResponse struct {
	Result UpdateProfileResponseResult `json:"result"`
	Error  string                      `json:"error,omitempty"`
}

// ── Profile CRUD helpers (continued) ────────────────────────────────────

func listProfiles(url string, client *http.Client, token string) (int, *ListProfilesResponse, error) {
	req, err := http.NewRequestWithContext(context.Background(), "GET", url+"/api/v1/profiles", nil)
	if err != nil {
		return 0, nil, err
	}

	req.Header.Set("Authorization", "Bearer "+token)

	res, err := client.Do(req)
	if err != nil {
		return 0, nil, err
	}

	defer func() {
		if err := res.Body.Close(); err != nil {
			panic(err)
		}
	}()

	var response ListProfilesResponse
	if err := json.NewDecoder(res.Body).Decode(&response); err != nil {
		return 0, nil, err
	}

	return res.StatusCode, &response, nil
}

func getProfile(url string, client *http.Client, token string, name string) (int, *GetProfileResponse, error) {
	req, err := http.NewRequestWithContext(context.Background(), "GET", url+"/api/v1/profiles/"+name, nil)
	if err != nil {
		return 0, nil, err
	}

	req.Header.Set("Authorization", "Bearer "+token)

	res, err := client.Do(req)
	if err != nil {
		return 0, nil, err
	}

	defer func() {
		if err := res.Body.Close(); err != nil {
			panic(err)
		}
	}()

	var response GetProfileResponse
	if err := json.NewDecoder(res.Body).Decode(&response); err != nil {
		return 0, nil, err
	}

	return res.StatusCode, &response, nil
}

func editProfile(url string, client *http.Client, name string, token string, data *UpdateProfileParams) (int, *UpdateProfileResponse, error) {
	body, err := json.Marshal(data)
	if err != nil {
		return 0, nil, err
	}

	req, err := http.NewRequestWithContext(context.Background(), "PUT", url+"/api/v1/profiles/"+name, strings.NewReader(string(body)))
	if err != nil {
		return 0, nil, err
	}

	req.Header.Set("Authorization", "Bearer "+token)

	res, err := client.Do(req)
	if err != nil {
		return 0, nil, err
	}

	defer func() {
		if err := res.Body.Close(); err != nil {
			panic(err)
		}
	}()

	var response UpdateProfileResponse
	if err := json.NewDecoder(res.Body).Decode(&response); err != nil {
		return 0, nil, err
	}

	return res.StatusCode, &response, nil
}

func deleteProfile(url string, client *http.Client, token string, name string) (int, *DeleteProfileResponse, error) {
	req, err := http.NewRequestWithContext(context.Background(), "DELETE", url+"/api/v1/profiles/"+name, nil)
	if err != nil {
		return 0, nil, err
	}

	req.Header.Set("Authorization", "Bearer "+token)

	res, err := client.Do(req)
	if err != nil {
		return 0, nil, err
	}

	defer func() {
		if err := res.Body.Close(); err != nil {
			panic(err)
		}
	}()

	var response DeleteProfileResponse
	if err := json.NewDecoder(res.Body).Decode(&response); err != nil {
		return 0, nil, err
	}

	return res.StatusCode, &response, nil
}

// ── Slice CRUD helpers ──────────────────────────────────────────────────

func createSlice(url string, client *http.Client, token string, data *CreateSliceParams) (int, *CreateSliceResponse, error) {
	body, err := json.Marshal(data)
	if err != nil {
		return 0, nil, err
	}

	req, err := http.NewRequestWithContext(context.Background(), "POST", url+"/api/v1/slices", strings.NewReader(string(body)))
	if err != nil {
		return 0, nil, err
	}

	req.Header.Set("Authorization", "Bearer "+token)

	res, err := client.Do(req)
	if err != nil {
		return 0, nil, err
	}

	defer func() {
		if err := res.Body.Close(); err != nil {
			panic(err)
		}
	}()

	var response CreateSliceResponse
	if err := json.NewDecoder(res.Body).Decode(&response); err != nil {
		return 0, nil, err
	}

	return res.StatusCode, &response, nil
}

func listSlices(url string, client *http.Client, token string) (int, *ListSlicesResponse, error) {
	req, err := http.NewRequestWithContext(context.Background(), "GET", url+"/api/v1/slices", nil)
	if err != nil {
		return 0, nil, err
	}

	req.Header.Set("Authorization", "Bearer "+token)

	res, err := client.Do(req)
	if err != nil {
		return 0, nil, err
	}

	defer func() {
		if err := res.Body.Close(); err != nil {
			panic(err)
		}
	}()

	var response ListSlicesResponse
	if err := json.NewDecoder(res.Body).Decode(&response); err != nil {
		return 0, nil, err
	}

	return res.StatusCode, &response, nil
}

func getSlice(url string, client *http.Client, token string, name string) (int, *GetSliceResponse, error) {
	req, err := http.NewRequestWithContext(context.Background(), "GET", url+"/api/v1/slices/"+name, nil)
	if err != nil {
		return 0, nil, err
	}

	req.Header.Set("Authorization", "Bearer "+token)

	res, err := client.Do(req)
	if err != nil {
		return 0, nil, err
	}

	defer func() {
		if err := res.Body.Close(); err != nil {
			panic(err)
		}
	}()

	var response GetSliceResponse
	if err := json.NewDecoder(res.Body).Decode(&response); err != nil {
		return 0, nil, err
	}

	return res.StatusCode, &response, nil
}

func editSlice(url string, client *http.Client, name string, token string, data *UpdateSliceParams) (int, *UpdateSliceResponse, error) {
	body, err := json.Marshal(data)
	if err != nil {
		return 0, nil, err
	}

	req, err := http.NewRequestWithContext(context.Background(), "PUT", url+"/api/v1/slices/"+name, strings.NewReader(string(body)))
	if err != nil {
		return 0, nil, err
	}

	req.Header.Set("Authorization", "Bearer "+token)

	res, err := client.Do(req)
	if err != nil {
		return 0, nil, err
	}

	defer func() {
		if err := res.Body.Close(); err != nil {
			panic(err)
		}
	}()

	var response UpdateSliceResponse
	if err := json.NewDecoder(res.Body).Decode(&response); err != nil {
		return 0, nil, err
	}

	return res.StatusCode, &response, nil
}

func deleteSlice(url string, client *http.Client, token string, name string) (int, *DeleteSliceResponse, error) {
	req, err := http.NewRequestWithContext(context.Background(), "DELETE", url+"/api/v1/slices/"+name, nil)
	if err != nil {
		return 0, nil, err
	}

	req.Header.Set("Authorization", "Bearer "+token)

	res, err := client.Do(req)
	if err != nil {
		return 0, nil, err
	}

	defer func() {
		if err := res.Body.Close(); err != nil {
			panic(err)
		}
	}()

	var response DeleteSliceResponse
	if err := json.NewDecoder(res.Body).Decode(&response); err != nil {
		return 0, nil, err
	}

	return res.StatusCode, &response, nil
}
