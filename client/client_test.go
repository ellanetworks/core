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

// 	routes, err := ella.ListRoutes()
// 	if err != nil {
// 		t.Fatalf("Failed to list routes: %v", err)
// 	}
// 	if len(routes) != 0 {
// 		t.Fatalf("Expected no routes, got %d", len(routes))
// 	}

// 	profiles, err := ella.ListProfiles()
// 	if err != nil {
// 		t.Fatalf("Failed to list profiles: %v", err)
// 	}
// 	if len(profiles) == 0 {
// 		t.Fatalf("Expected profiles, got empty list")
// 	}

// 	newProfile := &client.CreateProfileOptions{
// 		Name:            "testProfile",
// 		UeIPPool:        "10.45.0.0/16",
// 		DNS:             "8.8.8.8",
// 		Mtu:             1400,
// 		BitrateUplink:   "100 Mbps",
// 		BitrateDownlink: "100 Mbps",
// 		Var5qi:          9,
// 		PriorityLevel:   1,
// 	}
// 	err = ella.CreateProfile(newProfile)
// 	if err != nil {
// 		t.Fatalf("Failed to create profile: %v", err)
// 	}
// 	_, err = ella.ListProfiles()
// 	if err != nil {
// 		t.Fatalf("Failed to list profiles: %v", err)
// 	}
// 	getProfileOpt := &client.GetProfileOptions{
// 		Name: "testProfile",
// 	}
// 	profile, err := ella.GetProfile(getProfileOpt)
// 	if err != nil {
// 		t.Fatalf("Failed to get profile: %v", err)
// 	}
// 	if profile == nil {
// 		t.Fatalf("Expected profile, got nil")
// 	}
// 	deleteProfileOpts := &client.DeleteProfileOptions{
// 		Name: "testProfile",
// 	}
// 	err = ella.DeleteProfile(deleteProfileOpts)
// 	if err != nil {
// 		t.Fatalf("Failed to delete profile: %v", err)
// 	}
// 	profile, err = ella.GetProfile(getProfileOpt)
// 	if err == nil {
// 		t.Fatalf("Expected error, got nil")
// 	}
// 	if profile != nil {
// 		t.Fatalf("Expected nil, got profile")
// 	}

// 	status, err := ella.GetStatus()
// 	if err != nil {
// 		t.Fatalf("Failed to get status: %v", err)
// 	}
// 	if status == nil {
// 		t.Fatalf("Expected status, got nil")
// 	}
// 	if status.Initialized != true {
// 		t.Fatalf("Expected initialized status, got false")
// 	}
// 	if status.Version != "v0.0.14" {
// 		t.Fatalf("Expected version v0.0.14, got %s", status.Version)
// 	}
// }
