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

// 	createUserOpts := &client.CreateUserOptions{
// 		Email:    "nonAdmin4@ellanetworks.co",
// 		Role:     "admin",
// 		Password: "password",
// 	}
// 	err = ella.CreateUser(createUserOpts)
// 	if err != nil {
// 		t.Fatalf("Failed to create user: %v", err)
// 	}
// 	users, err := ella.ListUsers()
// 	if err != nil {
// 		t.Fatalf("Failed to list users: %v", err)
// 	}
// 	if len(users) == 0 {
// 		t.Fatalf("Expected users, got empty list")
// 	}

// 	deleteUserOptions := &client.DeleteUserOptions{
// 		Email: "nonAdmin4@ellanetworks.co",
// 	}
// 	err = ella.DeleteUser(deleteUserOptions)
// 	if err != nil {
// 		t.Fatalf("Failed to delete user: %v", err)
// 	}

// 	createSubscriberOpts := &client.CreateSubscriberOptions{
// 		Imsi:           "001010100000033",
// 		Key:            "5122250214c33e723a5dd523fc145fc0",
// 		SequenceNumber: "000000000022",
// 		ProfileName:    "default",
// 	}
// 	err = ella.CreateSubscriber(createSubscriberOpts)
// 	if err != nil {
// 		t.Fatalf("Failed to create subscriber: %v", err)
// 	}

// 	subscribers, err := ella.ListSubscribers()
// 	if err != nil {
// 		t.Fatalf("Failed to list subscribers: %v", err)
// 	}
// 	if len(subscribers) == 0 {
// 		t.Fatalf("Expected subscribers, got empty list")
// 	}

// 	getSubOpts := &client.GetSubscriberOptions{
// 		ID: "001010100000033",
// 	}
// 	sub, err := ella.GetSubscriber(getSubOpts)
// 	if err != nil {
// 		t.Fatalf("Failed to get subscriber: %v", err)
// 	}
// 	if sub == nil {
// 		t.Fatalf("Expected subscriber, got nil")
// 	}

// 	deleteSubOpts := &client.DeleteSubscriberOptions{
// 		ID: "001010100000033",
// 	}
// 	err = ella.DeleteSubscriber(deleteSubOpts)
// 	if err != nil {
// 		t.Fatalf("Failed to delete subscriber: %v", err)
// 	}

// }
