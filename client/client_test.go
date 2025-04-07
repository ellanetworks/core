package client_test

import (
	"context"

	"github.com/ellanetworks/core/client"
)

type fakeRequester struct {
	response *client.RequestResponse
	err      error
	// lastOpts holds the most recent RequestOptions passed in, so that we can verify
	// that the Login method constructs the request correctly.
	lastOpts *client.RequestOptions
}

func (f *fakeRequester) Do(ctx context.Context, opts *client.RequestOptions) (*client.RequestResponse, error) {
	f.lastOpts = opts
	return f.response, f.err
}

// func TestLoginReal(t *testing.T) {
// 	clientConfig := &client.Config{
// 		BaseURL: "http://127.0.0.1:32308",
// 	}
// 	ella, err := client.New(clientConfig)

// 	if err != nil {
// 		t.Fatalf("Failed to create client: %v", err)
// 	}

// 	loginOpts := &client.LoginOptions{
// 		Email:    "admin@ellanetworks.com",
// 		Password: "admin",
// 	}
// 	response, err := ella.Login(loginOpts)
// 	if err != nil {
// 		t.Fatalf("Failed to login: %v", err)
// 	}
// 	if response.Token == "" {
// 		t.Fatalf("Expected token, got empty string")
// 	}
// }
