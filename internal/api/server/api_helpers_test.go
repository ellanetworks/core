// Contains helper functions for testing the server
package server_test

import (
	"context"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"

	"github.com/ellanetworks/core/etsi"
	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/api/server"
	"github.com/ellanetworks/core/internal/config"
	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/kernel"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/smf"
	"github.com/ellanetworks/core/internal/supportbundle"
)

const (
	FirstUserEmail = "my.user123@ellanetworks.com"
)

type FakeKernel struct{}

func (fk FakeKernel) CreateRoute(destination *net.IPNet, gateway net.IP, priority int, networkInterface kernel.NetworkInterface) error {
	return nil
}

func (fk FakeKernel) DeleteRoute(destination *net.IPNet, gateway net.IP, priority int, networkInterface kernel.NetworkInterface) error {
	return nil
}

func (fk FakeKernel) InterfaceExists(networkInterface kernel.NetworkInterface) (bool, error) {
	return true, nil
}

func (fk FakeKernel) RouteExists(destination *net.IPNet, gateway net.IP, priority int, networkInterface kernel.NetworkInterface) (bool, error) {
	return false, nil
}

func (fk FakeKernel) EnableIPForwarding() error {
	return nil
}

func (fk FakeKernel) IsIPForwardingEnabled() (bool, error) {
	return true, nil
}

func (fk FakeKernel) ReplaceRoute(destination *net.IPNet, gateway net.IP, priority int, networkInterface kernel.NetworkInterface) error {
	return nil
}

func (fk FakeKernel) ListRoutesByPriority(priority int, networkInterface kernel.NetworkInterface) ([]net.IPNet, error) {
	return nil, nil
}

func (fk FakeKernel) EnsureGatewaysOnInterfaceInNeighTable(ifKey kernel.NetworkInterface) error {
	return nil
}

type dummyFS struct{}

func (dummyFS) Open(name string) (fs.File, error) {
	return nil, fs.ErrNotExist
}

type FakeUPF struct{}

func (f FakeUPF) ReloadNAT(natEnabled bool) error {
	return nil
}

func (f FakeUPF) ReloadFlowAccounting(flowAccountingEnabled bool) error {
	return nil
}

func (f FakeUPF) UpdateAdvertisedN3Address(ip net.IP) {
}

// testEnv holds the components created by setupServer.
type testEnv struct {
	Server    *httptest.Server
	JWTSecret []byte
	DB        *db.Database
	SMF       *smf.SMF
	AMF       *amf.AMF
}

func setupServer(filepath string) (testEnv, error) {
	testdb, err := db.NewDatabase(context.Background(), filepath)
	if err != nil {
		return testEnv{}, err
	}

	logger.SetDb(testdb)

	// Initialize SMF context with test stubs
	smfInstance := smf.New(&fakeSessionStore{db: testdb}, &fakeUPFClient{}, &fakeAMFCallback{})

	jwtSecret := []byte("testsecret")
	fakeKernel := FakeKernel{}
	dummyfs := dummyFS{}
	fakeUPF := FakeUPF{}

	cfg := config.Config{
		Interfaces: config.Interfaces{
			N2: config.N2Interface{
				Address: "12.12.12.12",
				Port:    2152,
			},
			N3: config.N3Interface{
				Name:    "eth0",
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

	amfInstance := amf.New(testdb, nil, nil)
	ts := httptest.NewTLSServer(server.NewHandler(testdb, cfg, fakeUPF, fakeKernel, jwtSecret, false, dummyfs, smfInstance, amfInstance, nil, nil))

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

type fakeSessionStore struct {
	db *db.Database
}

func (f *fakeSessionStore) AllocateIP(ctx context.Context, supi string) (net.IP, error) {
	return f.db.AllocateIP(ctx, supi)
}

func (f *fakeSessionStore) ReleaseIP(ctx context.Context, supi string, ip net.IP) error {
	return f.db.ReleaseIP(ctx, supi, ip)
}

func (f *fakeSessionStore) GetSubscriberPolicy(ctx context.Context, imsi string) (*smf.Policy, error) {
	return nil, fmt.Errorf("not implemented in test")
}

func (f *fakeSessionStore) GetDataNetwork(ctx context.Context, snssai *models.Snssai, dnn string) (*smf.DataNetworkInfo, error) {
	return nil, fmt.Errorf("not implemented in test")
}

func (f *fakeSessionStore) IncrementDailyUsage(ctx context.Context, imsi string, uplinkBytes, downlinkBytes uint64) error {
	return nil
}

func (f *fakeSessionStore) InsertFlowReport(ctx context.Context, report *smf.FlowReport) error {
	return nil
}

type fakeUPFClient struct{}

func (f *fakeUPFClient) EstablishSession(ctx context.Context, req *smf.PFCPEstablishmentRequest) (*smf.PFCPEstablishmentResponse, error) {
	return nil, fmt.Errorf("not implemented in test")
}

func (f *fakeUPFClient) ModifySession(ctx context.Context, req *smf.PFCPModificationRequest) error {
	return nil
}

func (f *fakeUPFClient) DeleteSession(ctx context.Context, localSEID, remoteSEID uint64) error {
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
