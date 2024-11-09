package server_test

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

type ListSubscribersResponseResult []int

type ListSubscribersResponse struct {
	Error  string                        `json:"error,omitempty"`
	Result ListSubscribersResponseResult `json:"result"`
}

type CreateSubscriberParams struct {
	IMSI           string `json:"imsi"`
	PLMNId         string `json:"plmn_id"`
	OPC            string `json:"opc"`
	Key            string `json:"key"`
	SequenceNumber string `json:"sequence_number"`
	DeviceGroupID  int64  `json:"device_group_id"`
}

type createSubscriberResponseResult struct {
	ID int64 `json:"id"`
}

type createSubscriberResponse struct {
	Error  string                         `json:"error,omitempty"`
	Result createSubscriberResponseResult `json:"result"`
}

type GetSubscriberResponseResult struct {
	ID             int64  `json:"id"`
	IMSI           string `json:"imsi"`
	PLMNId         string `json:"plmn_id"`
	OPC            string `json:"opc"`
	Key            string `json:"key"`
	SequenceNumber string `json:"sequence_number"`
	DeviceGroupID  int64  `json:"device_group_id"`
}

type GetSubscriberResponse struct {
	Error  string                      `json:"error,omitempty"`
	Result GetSubscriberResponseResult `json:"result"`
}

type DeleteSubscriberResponseResult struct {
	ID int64 `json:"id"`
}

type DeleteSubscriberResponse struct {
	Error  string                         `json:"error,omitempty"`
	Result DeleteSubscriberResponseResult `json:"result"`
}

func listSubscribers(url string, client *http.Client) (int, *ListSubscribersResponse, error) {
	req, err := http.NewRequest("GET", url+"/api/v1/subscribers", nil)
	if err != nil {
		return 0, nil, err
	}
	res, err := client.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer res.Body.Close()
	var listResponse ListSubscribersResponse
	if err := json.NewDecoder(res.Body).Decode(&listResponse); err != nil {
		return 0, nil, err
	}
	return res.StatusCode, &listResponse, nil
}

func getSubscriber(url string, client *http.Client, id string) (int, *GetSubscriberResponse, error) {
	req, err := http.NewRequest("GET", url+"/api/v1/subscribers/"+id, nil)
	if err != nil {
		return 0, nil, err
	}
	res, err := client.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer res.Body.Close()
	var getResponse GetSubscriberResponse
	if err := json.NewDecoder(res.Body).Decode(&getResponse); err != nil {
		return 0, nil, err
	}
	return res.StatusCode, &getResponse, nil
}

func createSubscriber(url string, client *http.Client, data *CreateSubscriberParams) (int, *createSubscriberResponse, error) {
	body, err := json.Marshal(data)
	if err != nil {
		return 0, nil, err
	}
	req, err := http.NewRequest("POST", url+"/api/v1/subscribers", strings.NewReader(string(body)))
	if err != nil {
		return 0, nil, err
	}
	res, err := client.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer res.Body.Close()
	var createResponse createSubscriberResponse
	if err := json.NewDecoder(res.Body).Decode(&createResponse); err != nil {
		return 0, nil, err
	}
	return res.StatusCode, &createResponse, nil
}

func deleteSubscriber(url string, client *http.Client, id string) (int, *DeleteSubscriberResponse, error) {
	req, err := http.NewRequest("DELETE", url+"/api/v1/subscribers/"+id, nil)
	if err != nil {
		return 0, nil, err
	}
	res, err := client.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer res.Body.Close()
	var deleteResponse DeleteSubscriberResponse
	if err := json.NewDecoder(res.Body).Decode(&deleteResponse); err != nil {
		return 0, nil, err
	}
	return res.StatusCode, &deleteResponse, nil
}

func TestSubscribersHandlers(t *testing.T) {
	ts, err := setupServer()
	if err != nil {
		t.Fatal(err)
	}
	defer ts.Close()
	client := ts.Client()

	t.Run("List subscribers - 0", func(t *testing.T) {
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

		if len(response.Result) != 0 {
			t.Fatalf("expected result %v, got %v", []int{}, response.Result)
		}
	})

	t.Run("Create subscriber with non-existent device group", func(t *testing.T) {
		data := CreateSubscriberParams{
			IMSI:           "IMSI2",
			PLMNId:         "PLMNId2",
			OPC:            "OPC2",
			Key:            "Key2",
			SequenceNumber: "SequenceNumber2",
			DeviceGroupID:  2,
		}
		statusCode, response, _ := createSubscriber(ts.URL, client, &data)

		if statusCode != http.StatusBadRequest {
			t.Fatalf("expected status %d, got %d", http.StatusBadRequest, statusCode)
		}

		if response.Error != "device group not found" {
			t.Fatalf("expected error %q, got %q", "device group not found", response.Error)
		}
	})

	t.Run("Create network slice - 1", func(t *testing.T) {
		data := CreateNetworkSliceParams{
			Name:     "Name1",
			Sst:      1,
			Sd:       "Sd1",
			SiteName: "SiteName1",
			Mcc:      "Mcc1",
			Mnc:      "Mnc1",
		}
		statusCode, response, err := createNetworkSlice(ts.URL, client, &data)
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

	t.Run("Create device group - 1", func(t *testing.T) {
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
			NetworkSliceId:   1,
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

	t.Run("Create subscriber (with device group) - 1", func(t *testing.T) {
		data := CreateSubscriberParams{
			IMSI:           "IMSI2",
			PLMNId:         "PLMNId2",
			OPC:            "OPC2",
			Key:            "Key2",
			SequenceNumber: "SequenceNumber2",
			DeviceGroupID:  1,
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

	t.Run("List subscribers - 1", func(t *testing.T) {
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

		if len(response.Result) != 1 {
			t.Fatalf("expected result %v, got %v", []int{1}, response.Result)
		}
	})

	t.Run("Get subscriber - 1", func(t *testing.T) {
		statusCode, response, err := getSubscriber(ts.URL, client, "1")
		if err != nil {
			t.Fatal(err)
		}

		if statusCode != http.StatusOK {
			t.Fatalf("expected status %d, got %d", http.StatusOK, statusCode)
		}

		if response.Error != "" {
			t.Fatalf("expected error %q, got %q", "", response.Error)
		}

		if response.Result.ID != 1 {
			t.Fatalf("expected id %d, got %d", 1, response.Result.ID)
		}

		if response.Result.IMSI != "IMSI2" {
			t.Fatalf("expected imsi %q, got %q", "IMSI2", response.Result.IMSI)
		}

		if response.Result.PLMNId != "PLMNId2" {
			t.Fatalf("expected plmn_id %q, got %q", "PLMNId2", response.Result.PLMNId)
		}

		if response.Result.OPC != "OPC2" {
			t.Fatalf("expected opc %q, got %q", "OPC2", response.Result.OPC)
		}

		if response.Result.Key != "Key2" {
			t.Fatalf("expected key %q, got %q", "Key2", response.Result.Key)
		}

		if response.Result.SequenceNumber != "SequenceNumber2" {
			t.Fatalf("expected sequence_number %q, got %q", "SequenceNumber2", response.Result.SequenceNumber)
		}

		if response.Result.DeviceGroupID != 1 {
			t.Fatalf("expected device_group_id %d, got %d", 1, response.Result.DeviceGroupID)
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

	t.Run("List subscribers - 0", func(t *testing.T) {
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

		if len(response.Result) != 0 {
			t.Fatalf("expected result %v, got %v", []int{}, response.Result)
		}
	})
}
