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

	"github.com/ellanetworks/core/internal/api/server"
	"github.com/ellanetworks/core/internal/config"
	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/kernel"
	"github.com/ellanetworks/core/internal/logger"
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

func (fk FakeKernel) EnsureGatewaysOnInterfaceInNeighTable(ifKey kernel.NetworkInterface) error {
	return nil
}

type dummyFS struct{}

func (dummyFS) Open(name string) (fs.File, error) {
	return nil, fs.ErrNotExist
}

type FakeUPF struct{}

func (f FakeUPF) Reload(natEnabled bool) error {
	return nil
}

func (f FakeUPF) UpdateAdvertisedN3Address(ip net.IP) {
}

func setupServer(filepath string) (*httptest.Server, []byte, *db.Database, error) {
	testdb, err := db.NewDatabase(context.Background(), filepath)
	if err != nil {
		return nil, nil, nil, err
	}

	logger.SetDb(testdb)

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

	ts := httptest.NewTLSServer(server.NewHandler(testdb, cfg, fakeUPF, fakeKernel, jwtSecret, false, dummyfs, nil))

	client := ts.Client()

	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, nil, nil, err
	}

	client.Jar = jar

	return ts, jwtSecret, testdb, nil
}

func initializeAndRefresh(url string, client *http.Client) (string, error) {
	initParams := &InitializeParams{
		Email:    FirstUserEmail,
		Password: "password123",
	}

	statusCode, _, err := initialize(url, client, initParams)
	if err != nil {
		return "", fmt.Errorf("couldn't create user: %s", err)
	}

	if statusCode != http.StatusCreated {
		return "", fmt.Errorf("expected status %d, got %d", http.StatusCreated, statusCode)
	}

	statusCode, refreshResponse, err := refresh(url, client)
	if err != nil {
		return "", fmt.Errorf("couldn't refresh: %s", err)
	}

	if statusCode != http.StatusOK {
		return "", fmt.Errorf("expected refresh status %d, got %d", http.StatusOK, statusCode)
	}

	return refreshResponse.Result.Token, nil
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

	statusCode, _, err = login(url, client, loginParams)
	if err != nil {
		return "", fmt.Errorf("couldn't login: %s", err)
	}

	if statusCode != http.StatusOK {
		return "", fmt.Errorf("expected status %d, got %d", http.StatusOK, statusCode)
	}

	statusCode, refreshResp, err := refresh(url, client)
	if err != nil {
		return "", fmt.Errorf("couldn't refresh: %s", err)
	}

	if statusCode != http.StatusOK {
		return "", fmt.Errorf("expected status %d, got %d", http.StatusOK, statusCode)
	}

	if refreshResp.Result.Token == "" {
		return "", fmt.Errorf("expected non-empty token from refresh")
	}

	return refreshResp.Result.Token, nil
}
