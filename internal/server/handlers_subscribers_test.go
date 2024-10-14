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
			t.Fatalf("expected result %v, got %v", []int{1, 2}, response.Result)
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

		if response.Result.IMSI != "IMSI1" {
			t.Fatalf("expected imsi %q, got %q", "IMSI1", response.Result.IMSI)
		}

		if response.Result.PLMNId != "PLMNId1" {
			t.Fatalf("expected plmn_id %q, got %q", "PLMNId1", response.Result.PLMNId)
		}

		if response.Result.OPC != "OPC1" {
			t.Fatalf("expected opc %q, got %q", "OPC1", response.Result.OPC)
		}

		if response.Result.Key != "Key1" {
			t.Fatalf("expected key %q, got %q", "Key1", response.Result.Key)
		}

		if response.Result.SequenceNumber != "SequenceNumber1" {
			t.Fatalf("expected sequence_number %q, got %q", "SequenceNumber1", response.Result.SequenceNumber)
		}
	})

	t.Run("Get subscriber - 2", func(t *testing.T) {
		statusCode, response, err := getSubscriber(ts.URL, client, "2")
		if err != nil {
			t.Fatal(err)
		}

		if statusCode != http.StatusOK {
			t.Fatalf("expected status %d, got %d", http.StatusOK, statusCode)
		}

		if response.Error != "" {
			t.Fatalf("expected error %q, got %q", "", response.Error)
		}

		if response.Result.ID != 2 {
			t.Fatalf("expected id %d, got %d", 2, response.Result.ID)
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
			t.Fatalf("expected result %v, got %v", []int{2}, response.Result)
		}
	})
}
