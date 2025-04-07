package client_test

// func TestClient(t *testing.T) {
// 	config := &client.Config{
// 		BaseURL: "http://127.0.0.1:32308",
// 	}
// 	ellaClient, err := client.New(config)
// 	if err != nil {
// 		t.Fatalf("Failed to create client: %v", err)
// 	}
// 	loginOpts := &client.LoginOptions{
// 		Email:    "admin@ellanetworks.com",
// 		Password: "aa",
// 	}
// 	loginResponse, err := ellaClient.Login(loginOpts)
// 	if err != nil {
// 		t.Fatalf("Failed to login: %v", err)
// 	}
// 	if loginResponse == nil {
// 		t.Fatalf("Login response is nil")
// 	}
// 	if loginResponse.Token == "" {
// 		t.Fatalf("Login response token is empty")
// 	}
// }
