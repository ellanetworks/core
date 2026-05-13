package client_test

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/ellanetworks/core/client"
)

func TestGetBGPSettings_Success(t *testing.T) {
	fake := &fakeRequester{
		response: &client.RequestResponse{
			StatusCode: 200,
			Headers:    http.Header{},
			Result:     []byte(`{"enabled": true, "localAS": 65000, "routerID": "10.0.0.1", "listenAddress": ":179", "rejectedPrefixes": []}`),
		},
		err: nil,
	}
	clientObj := &client.Client{
		Requester: fake,
	}

	ctx := context.Background()

	settings, err := clientObj.GetBGPSettings(ctx)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if !settings.Enabled {
		t.Errorf("expected enabled=true, got: %v", settings.Enabled)
	}

	if settings.LocalAS != 65000 {
		t.Errorf("expected localAS 65000, got: %d", settings.LocalAS)
	}
}

func TestGetBGPSettings_Failure(t *testing.T) {
	fake := &fakeRequester{
		response: &client.RequestResponse{
			StatusCode: 500,
			Headers:    http.Header{},
			Result:     []byte(`{"error": "Failed to get BGP settings"}`),
		},
		err: errors.New("requester error"),
	}
	clientObj := &client.Client{
		Requester: fake,
	}

	ctx := context.Background()

	_, err := clientObj.GetBGPSettings(ctx)
	if err == nil {
		t.Fatalf("expected error, got none")
	}
}

func TestUpdateBGPSettings_Success(t *testing.T) {
	fake := &fakeRequester{
		response: &client.RequestResponse{
			StatusCode: 200,
			Headers:    http.Header{},
			Result:     []byte(`{"message": "BGP settings updated successfully"}`),
		},
		err: nil,
	}
	clientObj := &client.Client{
		Requester: fake,
	}

	opts := &client.UpdateBGPSettingsOptions{
		Enabled:       true,
		LocalAS:       65000,
		RouterID:      "10.0.0.1",
		ListenAddress: ":179",
	}

	ctx := context.Background()

	err := clientObj.UpdateBGPSettings(ctx, opts)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}

func TestUpdateBGPSettings_Failure(t *testing.T) {
	fake := &fakeRequester{
		response: &client.RequestResponse{
			StatusCode: 400,
			Headers:    http.Header{},
			Result:     []byte(`{"error": "localAS must be between 1 and 4294967295"}`),
		},
		err: errors.New("requester error"),
	}
	clientObj := &client.Client{
		Requester: fake,
	}

	opts := &client.UpdateBGPSettingsOptions{
		LocalAS: 0,
	}

	ctx := context.Background()

	err := clientObj.UpdateBGPSettings(ctx, opts)
	if err == nil {
		t.Fatalf("expected error, got none")
	}
}

func TestListBGPPeers_Success(t *testing.T) {
	fake := &fakeRequester{
		response: &client.RequestResponse{
			StatusCode: 200,
			Headers:    http.Header{},
			Result:     []byte(`{"items": [{"id": 1, "address": "10.0.0.2", "remoteAS": 65001, "holdTime": 90, "importPrefixes": []}], "page": 1, "per_page": 25, "total_count": 1}`),
		},
		err: nil,
	}
	clientObj := &client.Client{
		Requester: fake,
	}

	ctx := context.Background()

	params := &client.ListParams{Page: 1, PerPage: 25}

	peers, err := clientObj.ListBGPPeers(ctx, params)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if len(peers.Items) != 1 {
		t.Fatalf("expected 1 peer, got %d", len(peers.Items))
	}

	if peers.Items[0].Address != "10.0.0.2" {
		t.Errorf("expected address 10.0.0.2, got %s", peers.Items[0].Address)
	}
}

func TestListBGPPeers_Failure(t *testing.T) {
	fake := &fakeRequester{
		response: &client.RequestResponse{
			StatusCode: 500,
			Headers:    http.Header{},
			Result:     []byte(`{"error": "Internal server error"}`),
		},
		err: errors.New("requester error"),
	}
	clientObj := &client.Client{
		Requester: fake,
	}

	ctx := context.Background()

	params := &client.ListParams{Page: 1, PerPage: 25}

	_, err := clientObj.ListBGPPeers(ctx, params)
	if err == nil {
		t.Fatalf("expected error, got none")
	}
}

func TestGetBGPPeer_Success(t *testing.T) {
	fake := &fakeRequester{
		response: &client.RequestResponse{
			StatusCode: 200,
			Headers:    http.Header{},
			Result:     []byte(`{"id": 7, "address": "10.0.0.2", "remoteAS": 65001, "holdTime": 90, "hasPassword": true, "importPrefixes": [{"prefix": "0.0.0.0/0", "maxLength": 0}]}`),
		},
		err: nil,
	}
	clientObj := &client.Client{
		Requester: fake,
	}

	ctx := context.Background()

	peer, err := clientObj.GetBGPPeer(ctx, &client.GetBGPPeerOptions{ID: 7})
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if peer.ID != 7 {
		t.Errorf("expected ID 7, got %d", peer.ID)
	}

	if !peer.HasPassword {
		t.Errorf("expected HasPassword=true")
	}
}

func TestGetBGPPeer_Failure(t *testing.T) {
	fake := &fakeRequester{
		response: &client.RequestResponse{
			StatusCode: 404,
			Headers:    http.Header{},
			Result:     []byte(`{"error": "BGP peer not found"}`),
		},
		err: errors.New("requester error"),
	}
	clientObj := &client.Client{
		Requester: fake,
	}

	ctx := context.Background()

	_, err := clientObj.GetBGPPeer(ctx, &client.GetBGPPeerOptions{ID: 999})
	if err == nil {
		t.Fatalf("expected error, got none")
	}
}

func TestCreateBGPPeer_Success(t *testing.T) {
	fake := &fakeRequester{
		response: &client.RequestResponse{
			StatusCode: 201,
			Headers:    http.Header{},
			Result:     []byte(`{"message": "BGP peer created successfully"}`),
		},
		err: nil,
	}
	clientObj := &client.Client{
		Requester: fake,
	}

	opts := &client.CreateBGPPeerOptions{
		Address:  "10.0.0.2",
		RemoteAS: 65001,
		HoldTime: 90,
	}

	ctx := context.Background()

	err := clientObj.CreateBGPPeer(ctx, opts)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}

func TestCreateBGPPeer_Failure(t *testing.T) {
	fake := &fakeRequester{
		response: &client.RequestResponse{
			StatusCode: 409,
			Headers:    http.Header{},
			Result:     []byte(`{"error": "A BGP peer with this address already exists"}`),
		},
		err: errors.New("requester error"),
	}
	clientObj := &client.Client{
		Requester: fake,
	}

	opts := &client.CreateBGPPeerOptions{
		Address:  "10.0.0.2",
		RemoteAS: 65001,
	}

	ctx := context.Background()

	err := clientObj.CreateBGPPeer(ctx, opts)
	if err == nil {
		t.Fatalf("expected error, got none")
	}
}

func TestUpdateBGPPeer_Success(t *testing.T) {
	fake := &fakeRequester{
		response: &client.RequestResponse{
			StatusCode: 200,
			Headers:    http.Header{},
			Result:     []byte(`{"message": "BGP peer updated successfully"}`),
		},
		err: nil,
	}
	clientObj := &client.Client{
		Requester: fake,
	}

	opts := &client.UpdateBGPPeerOptions{
		ID:       7,
		Address:  "10.0.0.2",
		RemoteAS: 65001,
		HoldTime: 90,
	}

	ctx := context.Background()

	err := clientObj.UpdateBGPPeer(ctx, opts)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}

func TestUpdateBGPPeer_Failure(t *testing.T) {
	fake := &fakeRequester{
		response: &client.RequestResponse{
			StatusCode: 400,
			Headers:    http.Header{},
			Result:     []byte(`{"error": "Invalid request data"}`),
		},
		err: errors.New("requester error"),
	}
	clientObj := &client.Client{
		Requester: fake,
	}

	opts := &client.UpdateBGPPeerOptions{ID: 7}

	ctx := context.Background()

	err := clientObj.UpdateBGPPeer(ctx, opts)
	if err == nil {
		t.Fatalf("expected error, got none")
	}
}

func TestDeleteBGPPeer_Success(t *testing.T) {
	fake := &fakeRequester{
		response: &client.RequestResponse{
			StatusCode: 200,
			Headers:    http.Header{},
			Result:     []byte(`{"message": "BGP peer deleted successfully"}`),
		},
		err: nil,
	}
	clientObj := &client.Client{
		Requester: fake,
	}

	ctx := context.Background()

	err := clientObj.DeleteBGPPeer(ctx, &client.DeleteBGPPeerOptions{ID: 7})
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}

func TestDeleteBGPPeer_Failure(t *testing.T) {
	fake := &fakeRequester{
		response: &client.RequestResponse{
			StatusCode: 404,
			Headers:    http.Header{},
			Result:     []byte(`{"error": "BGP peer not found"}`),
		},
		err: errors.New("requester error"),
	}
	clientObj := &client.Client{
		Requester: fake,
	}

	ctx := context.Background()

	err := clientObj.DeleteBGPPeer(ctx, &client.DeleteBGPPeerOptions{ID: 999})
	if err == nil {
		t.Fatalf("expected error, got none")
	}
}

func TestGetBGPAdvertisedRoutes_Success(t *testing.T) {
	fake := &fakeRequester{
		response: &client.RequestResponse{
			StatusCode: 200,
			Headers:    http.Header{},
			Result:     []byte(`{"routes": [{"subscriber": "001010000000001", "prefix": "10.45.0.1/32", "nextHop": "10.0.0.1"}]}`),
		},
		err: nil,
	}
	clientObj := &client.Client{
		Requester: fake,
	}

	ctx := context.Background()

	routes, err := clientObj.GetBGPAdvertisedRoutes(ctx)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if len(routes.Routes) != 1 {
		t.Fatalf("expected 1 route, got %d", len(routes.Routes))
	}

	if routes.Routes[0].Prefix != "10.45.0.1/32" {
		t.Errorf("expected prefix 10.45.0.1/32, got %s", routes.Routes[0].Prefix)
	}
}

func TestGetBGPAdvertisedRoutes_Failure(t *testing.T) {
	fake := &fakeRequester{
		response: &client.RequestResponse{
			StatusCode: 500,
			Headers:    http.Header{},
			Result:     []byte(`{"error": "Failed to get BGP routes"}`),
		},
		err: errors.New("requester error"),
	}
	clientObj := &client.Client{
		Requester: fake,
	}

	ctx := context.Background()

	_, err := clientObj.GetBGPAdvertisedRoutes(ctx)
	if err == nil {
		t.Fatalf("expected error, got none")
	}
}

func TestGetBGPLearnedRoutes_Success(t *testing.T) {
	fake := &fakeRequester{
		response: &client.RequestResponse{
			StatusCode: 200,
			Headers:    http.Header{},
			Result:     []byte(`{"routes": [{"prefix": "192.168.1.0/24", "nextHop": "10.0.0.2", "peer": "10.0.0.2"}]}`),
		},
		err: nil,
	}
	clientObj := &client.Client{
		Requester: fake,
	}

	ctx := context.Background()

	routes, err := clientObj.GetBGPLearnedRoutes(ctx)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if len(routes.Routes) != 1 {
		t.Fatalf("expected 1 route, got %d", len(routes.Routes))
	}

	if routes.Routes[0].Peer != "10.0.0.2" {
		t.Errorf("expected peer 10.0.0.2, got %s", routes.Routes[0].Peer)
	}
}

func TestGetBGPLearnedRoutes_Failure(t *testing.T) {
	fake := &fakeRequester{
		response: &client.RequestResponse{
			StatusCode: 500,
			Headers:    http.Header{},
			Result:     []byte(`{"error": "Internal server error"}`),
		},
		err: errors.New("requester error"),
	}
	clientObj := &client.Client{
		Requester: fake,
	}

	ctx := context.Background()

	_, err := clientObj.GetBGPLearnedRoutes(ctx)
	if err == nil {
		t.Fatalf("expected error, got none")
	}
}
