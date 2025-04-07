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

// func TestE2EReal(t *testing.T) {
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
// 	err = ella.Login(loginOpts)
// 	if err != nil {
// 		t.Fatalf("Failed to login: %v", err)
// 	}

// 	token := ella.GetToken()
// 	if token == "" {
// 		t.Fatalf("Expected token, got empty string")
// 	}

// 	metrics, err := ella.GetMetrics()
// 	if err != nil {
// 		t.Fatalf("Failed to get metrics: %v", err)
// 	}

// 	if metrics == nil {
// 		t.Fatalf("Expected metrics, got nil")
// 	}

// 	appDownlinkBytes := metrics["app_downlink_bytes"]
// 	if appDownlinkBytes == 0 {
// 		t.Fatalf("Expected app_downlink_bytes, got 0")
// 	}

// }
