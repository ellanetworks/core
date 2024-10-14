package server_test

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

type ListDeviceGroupSubscribersResponseResult []int

type ListDeviceGroupSubscribersResponse struct {
	Error  string                                   `json:"error,omitempty"`
	Result ListDeviceGroupSubscribersResponseResult `json:"result"`
}

type CreateDeviceGroupSubscriberParams struct {
	SubscriberID int `json:"subscriber_id"`
}

type CreateDeviceGroupSubscriberResponseResult struct {
	SubscriberID int `json:"subscriber_id"`
}

type CreateDeviceGroupSubscriberResponse struct {
	Error  string                                    `json:"error,omitempty"`
	Result CreateDeviceGroupSubscriberResponseResult `json:"result"`
}

type DeleteDeviceGroupSubscriberResponseResult struct {
	SubscriberID int `json:"subscriber_id"`
}

type DeleteDeviceGroupSubscriberResponse struct {
	Error  string                                    `json:"error,omitempty"`
	Result DeleteDeviceGroupSubscriberResponseResult `json:"result"`
}

func listDeviceGroupSubscribers(url string, client *http.Client, id string) (int, *ListDeviceGroupSubscribersResponse, error) {
	req, err := http.NewRequest("GET", url+"/api/v1/device-groups/"+id+"/subscribers", nil)
	if err != nil {
		return 0, nil, err
	}
	res, err := client.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer res.Body.Close()
	var listResponse ListDeviceGroupSubscribersResponse
	if err := json.NewDecoder(res.Body).Decode(&listResponse); err != nil {
		return 0, nil, err
	}
	return res.StatusCode, &listResponse, nil
}

func createDeviceGroupSubscriber(url string, client *http.Client, id string, data *CreateDeviceGroupSubscriberParams) (int, *CreateDeviceGroupSubscriberResponse, error) {
	body, err := json.Marshal(data)
	if err != nil {
		return 0, nil, err
	}
	req, err := http.NewRequest("POST", url+"/api/v1/device-groups/"+id+"/subscribers", strings.NewReader(string(body)))
	if err != nil {
		return 0, nil, err
	}
	res, err := client.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer res.Body.Close()
	var createResponse CreateDeviceGroupSubscriberResponse
	if err := json.NewDecoder(res.Body).Decode(&createResponse); err != nil {
		return 0, nil, err
	}
	return res.StatusCode, &createResponse, nil
}

func deleteDeviceGroupSubscriber(url string, client *http.Client, id, subscriberID string) (int, *DeleteDeviceGroupSubscriberResponse, error) {
	req, err := http.NewRequest("DELETE", url+"/api/v1/device-groups/"+id+"/subscribers/"+subscriberID, nil)
	if err != nil {
		return 0, nil, err
	}
	res, err := client.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer res.Body.Close()
	var deleteResponse DeleteDeviceGroupSubscriberResponse
	if err := json.NewDecoder(res.Body).Decode(&deleteResponse); err != nil {
		return 0, nil, err
	}
	return res.StatusCode, &deleteResponse, nil
}

func TestDeviceGroupSubscribersHandlers(t *testing.T) {
	ts, err := setupServer()
	if err != nil {
		t.Fatal(err)
	}
	defer ts.Close()
	client := ts.Client()

	t.Run("List device group subscribers - not found", func(t *testing.T) {
		statusCode, response, err := listDeviceGroupSubscribers(ts.URL, client, "1")
		if err != nil {
			t.Fatal(err)
		}

		if statusCode != http.StatusNotFound {
			t.Fatalf("expected status %d, got %d", http.StatusNotFound, statusCode)
		}

		if response.Error != "device group not found" {
			t.Fatalf("expected error %q, got %q", "device group not found", response.Error)
		}

		if len(response.Result) != 0 {
			t.Fatalf("expected result %v, got %v", []int{}, response.Result)
		}
	})

	t.Run("Create device group", func(t *testing.T) {
		data := CreateDeviceGroupParams{
			Name:             "Name1",
			SiteInfo:         "SiteInfo1",
			IpDomainName:     "IpDomainName1",
			Dnn:              "internet",
			UeIpPool:         "11.0.0.0/24",
			DnsPrimary:       "8.8.8.8",
			Mtu:              1460,
			DnnMbrUplink:     20000000,
			DnnMbrDownlink:   20000000,
			TrafficClassName: "platinum",
			TrafficClassArp:  6,
			TrafficClassPdb:  300,
			TrafficClassPelr: 6,
			TrafficClassQci:  8,
		}
		statusCode, response, err := createDeviceGroup(ts.URL, client, &data)
		if err != nil {
			t.Fatal(err)
		}

		if statusCode != http.StatusCreated {
			t.Fatalf("expected status %d, got %d", http.StatusCreated, statusCode)
		}

		if response.Result.ID != 1 {
			t.Fatalf("expected id %d, got %d", 1, response.Result.ID)
		}
	})

	t.Run("List device group subscribers - 0", func(t *testing.T) {
		statusCode, response, err := listDeviceGroupSubscribers(ts.URL, client, "1")
		if err != nil {
			t.Fatal(err)
		}

		if statusCode != http.StatusOK {
			t.Fatalf("expected status %d, got %d", http.StatusOK, statusCode)
		}

		if response.Error != "" {
			t.Fatalf("expected error %q, got %q", "", response.Error)
		}

		if len(response.Result) != 0 {
			t.Fatalf("expected result %v, got %v", []int{}, response.Result)
		}
	})

	t.Run("Create device group subscriber - subscriber does not exist", func(t *testing.T) {
		data := CreateDeviceGroupSubscriberParams{
			SubscriberID: 1,
		}
		statusCode, response, err := createDeviceGroupSubscriber(ts.URL, client, "1", &data)
		if err != nil {
			t.Fatal(err)
		}

		if statusCode != http.StatusNotFound {
			t.Fatalf("expected status %d, got %d", http.StatusNotFound, statusCode)
		}

		if response.Error != "subscriber not found" {
			t.Fatalf("expected error %q, got %q", "subscriber not found", response.Error)
		}
	})

	t.Run("Create subscriber - 1", func(t *testing.T) {
		data := CreateSubscriberParams{
			IMSI:           "IMSI1",
			PLMNId:         "PLMNId1",
			OPC:            "OPC1",
			Key:            "Key1",
			SequenceNumber: "SequenceNumber1",
		}
		statusCode, response, err := createSubscriber(ts.URL, client, &data)
		if err != nil {
			t.Fatal(err)
		}

		if statusCode != http.StatusCreated {
			t.Fatalf("expected status %d, got %d", http.StatusCreated, statusCode)
		}

		if response.Result.ID != 1 {
			t.Fatalf("expected id %d, got %d", 1, response.Result.ID)
		}
	})

	t.Run("Create subscriber - 2", func(t *testing.T) {
		data := CreateSubscriberParams{
			IMSI:           "IMSI2",
			PLMNId:         "PLMNId2",
			OPC:            "OPC2",
			Key:            "Key2",
			SequenceNumber: "SequenceNumber2",
		}
		statusCode, response, err := createSubscriber(ts.URL, client, &data)
		if err != nil {
			t.Fatal(err)
		}

		if statusCode != http.StatusCreated {
			t.Fatalf("expected status %d, got %d", http.StatusCreated, statusCode)
		}

		if response.Result.ID != 2 {
			t.Fatalf("expected id %d, got %d", 2, response.Result.ID)
		}
	})

	t.Run("Create subscriber - 3", func(t *testing.T) {
		data := CreateSubscriberParams{
			IMSI:           "IMSI3",
			PLMNId:         "PLMNId3",
			OPC:            "OPC3",
			Key:            "Key3",
			SequenceNumber: "SequenceNumber3",
		}
		statusCode, response, err := createSubscriber(ts.URL, client, &data)
		if err != nil {
			t.Fatal(err)
		}

		if statusCode != http.StatusCreated {
			t.Fatalf("expected status %d, got %d", http.StatusCreated, statusCode)
		}

		if response.Result.ID != 3 {
			t.Fatalf("expected id %d, got %d", 3, response.Result.ID)
		}
	})

	t.Run("Create device group subscriber - 1", func(t *testing.T) {
		data := CreateDeviceGroupSubscriberParams{
			SubscriberID: 1,
		}
		statusCode, response, err := createDeviceGroupSubscriber(ts.URL, client, "1", &data)
		if err != nil {
			t.Fatal(err)
		}

		if statusCode != http.StatusCreated {
			t.Fatalf("expected status %d, got %d", http.StatusCreated, statusCode)
		}

		if response.Result.SubscriberID != 1 {
			t.Fatalf("expected subscriber_id %d, got %d", 2, response.Result.SubscriberID)
		}
	})

	t.Run("Create device group subscriber - 2", func(t *testing.T) {
		data := CreateDeviceGroupSubscriberParams{
			SubscriberID: 2,
		}
		statusCode, response, err := createDeviceGroupSubscriber(ts.URL, client, "1", &data)
		if err != nil {
			t.Fatal(err)
		}

		if statusCode != http.StatusCreated {
			t.Fatalf("expected status %d, got %d", http.StatusCreated, statusCode)
		}

		if response.Result.SubscriberID != 2 {
			t.Fatalf("expected subscriber_id %d, got %d", 2, response.Result.SubscriberID)
		}
	})

	t.Run("Create device group subscriber - 3", func(t *testing.T) {
		data := CreateDeviceGroupSubscriberParams{
			SubscriberID: 3,
		}
		statusCode, response, err := createDeviceGroupSubscriber(ts.URL, client, "1", &data)
		if err != nil {
			t.Fatal(err)
		}

		if statusCode != http.StatusCreated {
			t.Fatalf("expected status %d, got %d", http.StatusCreated, statusCode)
		}

		if response.Result.SubscriberID != 3 {
			t.Fatalf("expected subscriber_id %d, got %d", 3, response.Result.SubscriberID)
		}
	})

	t.Run("Create device group subscriber - subscriber already exists", func(t *testing.T) {
		data := CreateDeviceGroupSubscriberParams{
			SubscriberID: 1,
		}
		statusCode, response, err := createDeviceGroupSubscriber(ts.URL, client, "1", &data)
		if err != nil {
			t.Fatal(err)
		}

		if statusCode != http.StatusConflict {
			t.Fatalf("expected status %d, got %d", http.StatusConflict, statusCode)
		}

		if response.Error != "subscriber already in device group" {
			t.Fatalf("expected error %q, got %q", "subscriber already in device group", response.Error)
		}
	})

	t.Run("List device group subscribers - 3", func(t *testing.T) {
		statusCode, response, err := listDeviceGroupSubscribers(ts.URL, client, "1")
		if err != nil {
			t.Fatal(err)
		}

		if statusCode != http.StatusOK {
			t.Fatalf("expected status %d, got %d", http.StatusOK, statusCode)
		}

		if response.Error != "" {
			t.Fatalf("expected error %q, got %q", "", response.Error)
		}

		if len(response.Result) != 3 {
			t.Fatalf("expected result %v, got %v", []int{1, 2, 3}, response.Result)
		}
	})

	t.Run("Delete subscriber - 1", func(t *testing.T) {
		statusCode, response, err := deleteSubscriber(ts.URL, client, "1")
		if err != nil {
			t.Fatal(err)
		}

		if statusCode != http.StatusAccepted {
			t.Fatalf("expected status %d, got %d", http.StatusAccepted, statusCode)
		}

		if response.Error != "" {
			t.Fatalf("expected error %q, got %q", "", response.Error)
		}

		if response.Result.ID != 1 {
			t.Fatalf("expected id %d, got %d", 1, response.Result.ID)
		}
	})

	t.Run("List device group subscribers - 1", func(t *testing.T) {
		statusCode, response, err := listDeviceGroupSubscribers(ts.URL, client, "1")
		if err != nil {
			t.Fatal(err)
		}

		if statusCode != http.StatusOK {
			t.Fatalf("expected status %d, got %d", http.StatusOK, statusCode)
		}

		if response.Error != "" {
			t.Fatalf("expected error %q, got %q", "", response.Error)
		}

		if len(response.Result) != 2 {
			t.Fatalf("expected result %v, got %v", []int{2, 3}, response.Result)
		}
	})

	t.Run("Delete device group subscriber - 2", func(t *testing.T) {
		statusCode, response, err := deleteDeviceGroupSubscriber(ts.URL, client, "1", "2")
		if err != nil {
			t.Fatal(err)
		}

		if statusCode != http.StatusAccepted {
			t.Fatalf("expected status %d, got %d", http.StatusAccepted, statusCode)
		}

		if response.Error != "" {
			t.Fatalf("expected error %q, got %q", "", response.Error)
		}

		if response.Result.SubscriberID != 2 {
			t.Fatalf("expected subscriber_id %d, got %d", 2, response.Result.SubscriberID)
		}
	})

	t.Run("List device group subscribers - 1", func(t *testing.T) {
		statusCode, response, err := listDeviceGroupSubscribers(ts.URL, client, "1")
		if err != nil {
			t.Fatal(err)
		}

		if statusCode != http.StatusOK {
			t.Fatalf("expected status %d, got %d", http.StatusOK, statusCode)
		}

		if response.Error != "" {
			t.Fatalf("expected error %q, got %q", "", response.Error)
		}

		if len(response.Result) != 1 {
			t.Fatalf("expected result %v, got %v", []int{3}, response.Result)
		}
	})

	t.Run("List subscribers - 2", func(t *testing.T) {
		statusCode, response, err := listSubscribers(ts.URL, client)
		if err != nil {
			t.Fatal(err)
		}

		if statusCode != http.StatusOK {
			t.Fatalf("expected status %d, got %d", http.StatusOK, statusCode)
		}

		if response.Error != "" {
			t.Fatalf("expected error %q, got %q", "", response.Error)
		}

		if len(response.Result) != 2 {
			t.Fatalf("expected result %v, got %v", []int{2, 3}, response.Result)
		}
	})

	t.Run("Get subscriber - 2 - should still exist", func(t *testing.T) {
		statusCode, _, err := getSubscriber(ts.URL, client, "2")
		if err != nil {
			t.Fatal(err)
		}

		if statusCode != http.StatusOK {
			t.Fatalf("expected status %d, got %d", http.StatusOK, statusCode)
		}
	})
}
