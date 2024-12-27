// Contains helper functions for testing the server
package server_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"

	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/nms/server"
)

func setupServer(filepath string) (*httptest.Server, []byte, error) {
	testdb, err := db.NewDatabase(filepath)
	if err != nil {
		return nil, nil, err
	}
	err = testdb.InitializeNetwork()
	if err != nil {
		return nil, nil, err
	}
	jwtSecret := []byte("testsecret")
	ts := httptest.NewTLSServer(server.NewHandler(testdb, jwtSecret))
	return ts, jwtSecret, nil
}

func createFirstUserAndLogin(url string, client *http.Client) (string, error) {
	user := &CreateUserParams{
		Username: "testuser",
		Password: "password123",
	}
	statusCode, _, err := createUser(url, client, "", user)
	if err != nil {
		return "", fmt.Errorf("couldn't create user: %s", err)
	}
	if statusCode != http.StatusCreated {
		return "", fmt.Errorf("expected status %d, got %d", http.StatusCreated, statusCode)
	}

	loginParams := &LoginParams{
		Username: "testuser",
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
