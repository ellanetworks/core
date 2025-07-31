package server_test

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
)

const (
	DataNetworkName = "internet"
	DNS             = "8.8.8.8"
	IPPool          = "0.0.0.0/24"
	MTU             = 1500
)

type CreateDataNetworkResponseResult struct {
	Message string `json:"message"`
}

type GetDataNetworkResponseResult struct {
	Name   string `json:"name"`
	IPPool string `json:"ip-pool,omitempty"`
	DNS    string `json:"dns,omitempty"`
	MTU    int32  `json:"mtu,omitempty"`
}

type GetDataNetworkResponse struct {
	Result GetDataNetworkResponseResult `json:"result"`
	Error  string                       `json:"error,omitempty"`
}

type CreateDataNetworkParams struct {
	Name   string `json:"name"`
	IPPool string `json:"ip-pool,omitempty"`
	DNS    string `json:"dns,omitempty"`
	MTU    int32  `json:"mtu,omitempty"`
}

type CreateDataNetworkResponse struct {
	Result CreateDataNetworkResponseResult `json:"result"`
	Error  string                          `json:"error,omitempty"`
}

type DeleteDataNetworkResponseResult struct {
	Message string `json:"message"`
}

type DeleteDataNetworkResponse struct {
	Result DeleteDataNetworkResponseResult `json:"result"`
	Error  string                          `json:"error,omitempty"`
}

type ListDataNetworkResponse struct {
	Result []GetDataNetworkResponse `json:"result"`
	Error  string                   `json:"error,omitempty"`
}

func listDataNetworks(url string, client *http.Client, token string) (int, *ListDataNetworkResponse, error) {
	req, err := http.NewRequestWithContext(context.Background(), "GET", url+"/api/v1/data-networks", nil)
	if err != nil {
		return 0, nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	res, err := client.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer func() {
		if err := res.Body.Close(); err != nil {
			panic(err)
		}
	}()
	var dataNetworkResponse ListDataNetworkResponse
	if err := json.NewDecoder(res.Body).Decode(&dataNetworkResponse); err != nil {
		return 0, nil, err
	}
	return res.StatusCode, &dataNetworkResponse, nil
}

func getDataNetwork(url string, client *http.Client, token string, name string) (int, *GetDataNetworkResponse, error) {
	req, err := http.NewRequest("GET", url+"/api/v1/data-networks/"+name, nil)
	if err != nil {
		return 0, nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	res, err := client.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer func() {
		if err := res.Body.Close(); err != nil {
			panic(err)
		}
	}()
	var dataNetworkResponse GetDataNetworkResponse
	if err := json.NewDecoder(res.Body).Decode(&dataNetworkResponse); err != nil {
		return 0, nil, err
	}
	return res.StatusCode, &dataNetworkResponse, nil
}

func createDataNetwork(url string, client *http.Client, token string, data *CreateDataNetworkParams) (int, *CreateDataNetworkResponse, error) {
	body, err := json.Marshal(data)
	if err != nil {
		return 0, nil, err
	}
	req, err := http.NewRequestWithContext(context.Background(), "POST", url+"/api/v1/data-networks", strings.NewReader(string(body)))
	if err != nil {
		return 0, nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	res, err := client.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer func() {
		if err := res.Body.Close(); err != nil {
			panic(err)
		}
	}()
	var createResponse CreateDataNetworkResponse
	if err := json.NewDecoder(res.Body).Decode(&createResponse); err != nil {
		return 0, nil, err
	}
	return res.StatusCode, &createResponse, nil
}

func editDataNetwork(url string, client *http.Client, name string, token string, data *CreateDataNetworkParams) (int, *CreateDataNetworkResponse, error) {
	body, err := json.Marshal(data)
	if err != nil {
		return 0, nil, err
	}
	req, err := http.NewRequestWithContext(context.Background(), "PUT", url+"/api/v1/data-networks/"+name, strings.NewReader(string(body)))
	if err != nil {
		return 0, nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	res, err := client.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer func() {
		if err := res.Body.Close(); err != nil {
			panic(err)
		}
	}()
	var createResponse CreateDataNetworkResponse
	if err := json.NewDecoder(res.Body).Decode(&createResponse); err != nil {
		return 0, nil, err
	}
	return res.StatusCode, &createResponse, nil
}

func deleteDataNetwork(url string, client *http.Client, token, name string) (int, *DeleteDataNetworkResponse, error) {
	req, err := http.NewRequest("DELETE", url+"/api/v1/data-networks/"+name, nil)
	if err != nil {
		return 0, nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	res, err := client.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer func() {
		if err := res.Body.Close(); err != nil {
			panic(err)
		}
	}()
	var deleteDataNetworkResponse DeleteDataNetworkResponse
	if err := json.NewDecoder(res.Body).Decode(&deleteDataNetworkResponse); err != nil {
		return 0, nil, err
	}
	return res.StatusCode, &deleteDataNetworkResponse, nil
}
