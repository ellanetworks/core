package client_test

import (
	"errors"
	"net/http"
	"testing"

	"github.com/ellanetworks/core/client"
)

func TestCreateDataNetwork_Success(t *testing.T) {
	fake := &fakeRequester{
		response: &client.RequestResponse{
			StatusCode: 200,
			Headers:    http.Header{},
			Result:     []byte(`{"message": "Data Network created successfully"}`),
		},
		err: nil,
	}
	clientObj := &client.Client{
		Requester: fake,
	}

	createDataNetworkOpts := &client.CreateDataNetworkOptions{
		Name:   "testDataNetwork",
		IPPool: "10.45.0.0/16",
		DNS:    "8.8.8.8",
		Mtu:    1400,
	}

	err := clientObj.CreateDataNetwork(createDataNetworkOpts)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}

func TestCreateDataNetwork_Failure(t *testing.T) {
	fake := &fakeRequester{
		response: &client.RequestResponse{
			StatusCode: 400,
			Headers:    http.Header{},
			Result:     []byte(`{"error": "Invalid UE IP Pool"}`),
		},
		err: errors.New("requester error"),
	}
	clientObj := &client.Client{
		Requester: fake,
	}
	createDataNetworkOpts := &client.CreateDataNetworkOptions{
		Name:   "testDataNetwork",
		IPPool: "12312312312",
		DNS:    "8.8.8.8",
		Mtu:    1400,
	}

	err := clientObj.CreateDataNetwork(createDataNetworkOpts)
	if err == nil {
		t.Fatalf("expected error, got none")
	}
}

func TestGetDataNetwork_Success(t *testing.T) {
	fake := &fakeRequester{
		response: &client.RequestResponse{
			StatusCode: 200,
			Headers:    http.Header{},
			Result:     []byte(`{"name": "my-data-network", "ip-pool": "1.2.3.0/24"}`),
		},
		err: nil,
	}
	clientObj := &client.Client{
		Requester: fake,
	}
	name := "my-data-network"

	getRouteOpts := &client.GetDataNetworkOptions{
		Name: name,
	}

	dataNetwork, err := clientObj.GetDataNetwork(getRouteOpts)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if dataNetwork.Name != name {
		t.Fatalf("expected ID %v, got %v", name, dataNetwork.Name)
	}

	if dataNetwork.IPPool != "1.2.3.0/24" {
		t.Fatalf("expected ID %v, got %v", "1.2.3.0/24", dataNetwork.IPPool)
	}
}

func TestGetDataNetwork_Failure(t *testing.T) {
	fake := &fakeRequester{
		response: &client.RequestResponse{
			StatusCode: 404,
			Headers:    http.Header{},
			Result:     []byte(`{"error": "Data Network not found"}`),
		},
		err: errors.New("requester error"),
	}
	clientObj := &client.Client{
		Requester: fake,
	}

	name := "non-existent-data-network"
	getDataNetworkOpts := &client.GetDataNetworkOptions{
		Name: name,
	}
	_, err := clientObj.GetDataNetwork(getDataNetworkOpts)
	if err == nil {
		t.Fatalf("expected error, got none")
	}
}

func TestDeleteDataNetwork_Success(t *testing.T) {
	fake := &fakeRequester{
		response: &client.RequestResponse{
			StatusCode: 200,
			Headers:    http.Header{},
			Result:     []byte(`{"message": "Data Network deleted successfully"}`),
		},
		err: nil,
	}
	clientObj := &client.Client{
		Requester: fake,
	}
	name := "testDataNetwork"

	deleteDataNetworkOpts := &client.DeleteDataNetworkOptions{
		Name: name,
	}
	err := clientObj.DeleteDataNetwork(deleteDataNetworkOpts)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}

func TestDeleteDataNetwork_Failure(t *testing.T) {
	fake := &fakeRequester{
		response: &client.RequestResponse{
			StatusCode: 404,
			Headers:    http.Header{},
			Result:     []byte(`{"error": "Data Network not found"}`),
		},
		err: errors.New("requester error"),
	}
	clientObj := &client.Client{
		Requester: fake,
	}

	name := "non-existent-data-network"

	deleteDataNetworkOpts := &client.DeleteDataNetworkOptions{
		Name: name,
	}
	err := clientObj.DeleteDataNetwork(deleteDataNetworkOpts)
	if err == nil {
		t.Fatalf("expected error, got none")
	}
}

func TestListDataNetworks_Success(t *testing.T) {
	fake := &fakeRequester{
		response: &client.RequestResponse{
			StatusCode: 200,
			Headers:    http.Header{},
			Result:     []byte(`[{"imsi": "001010100000022", "dataNetworkName": "default"}]`),
		},
		err: nil,
	}
	clientObj := &client.Client{
		Requester: fake,
	}

	dataNetworks, err := clientObj.ListDataNetworks()
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if len(dataNetworks) != 1 {
		t.Fatalf("expected 1 data network, got %d", len(dataNetworks))
	}
}

func TestListDataNetworks_Failure(t *testing.T) {
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

	_, err := clientObj.ListDataNetworks()
	if err == nil {
		t.Fatalf("expected error, got none")
	}
}
