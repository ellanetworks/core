// Contains helper functions for testing the server
package server_test

import (
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"

	"github.com/ellanetworks/core/internal/api/server"
	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/kernel"
)

var initialOperator = db.Operator{
	Mcc:                   "001",
	Mnc:                   "01",
	OperatorCode:          "0123456789ABCDEF0123456789ABCDEF",
	Sst:                   1,
	Sd:                    1056816,
	SupportedTACs:         `["001"]`,
	HomeNetworkPrivateKey: "c09c17bddf23357f614f492075b970d825767718114f59554ce2f345cf8c4b6a",
}

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

func setupServer(filepath string, mode string) (*httptest.Server, []byte, error) {
	testdb, err := db.NewDatabase(filepath, initialOperator)
	if err != nil {
		return nil, nil, err
	}
	jwtSecret := []byte("testsecret")
	fakeKernel := FakeKernel{}
	ts := httptest.NewTLSServer(server.NewHandler(testdb, fakeKernel, jwtSecret, mode))
	return ts, jwtSecret, nil
}

func createFirstUserAndLogin(url string, client *http.Client) (string, error) {
	user := &CreateUserParams{
		Email:    "my.user123@ellanetworks.com",
		Password: "password123",
		Role:     "admin",
	}
	statusCode, _, err := createUser(url, client, "", user)
	if err != nil {
		return "", fmt.Errorf("couldn't create user: %s", err)
	}
	if statusCode != http.StatusCreated {
		return "", fmt.Errorf("expected status %d, got %d", http.StatusCreated, statusCode)
	}

	loginParams := &LoginParams{
		Email:    "my.user123@ellanetworks.com",
		Password: "password123",
	}

	statusCode, response, err := login(url, client, loginParams)
	if err != nil {
		return "", fmt.Errorf("couldn't login: %s", err)
	}

	if statusCode != http.StatusOK {
		return "", fmt.Errorf("expected status %d, got %d", http.StatusOK, statusCode)
	}

	return response.Result.Token, nil
}

func createUserAndLogin(url string, token string, email string, role string, client *http.Client) (string, error) {
	user := &CreateUserParams{
		Email:    email,
		Password: "password123",
		Role:     role,
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

	statusCode, response, err := login(url, client, loginParams)
	if err != nil {
		return "", fmt.Errorf("couldn't login: %s", err)
	}

	if statusCode != http.StatusOK {
		return "", fmt.Errorf("expected status %d, got %d", http.StatusOK, statusCode)
	}

	return response.Result.Token, nil
}
