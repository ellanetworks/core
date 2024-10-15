package server_test

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

type ListNetworkSliceDeviceGroupsResponseResult []int

type ListNetworkSliceDeviceGroupsResponse struct {
	Error  string                                     `json:"error,omitempty"`
	Result ListNetworkSliceDeviceGroupsResponseResult `json:"result"`
}

type CreateNetworkSliceDeviceGroupParams struct {
	DeviceGroupID int `json:"device_group_id"`
}

type CreateNetworkSliceDeviceGroupResponseResult struct {
	DeviceGroupID int `json:"device_group_id"`
}

type CreateNetworkSliceDeviceGroupResponse struct {
	Error  string                                      `json:"error,omitempty"`
	Result CreateNetworkSliceDeviceGroupResponseResult `json:"result"`
}

type DeleteNetworkSliceDeviceGroupResponseResult struct {
	DeviceGroupID int `json:"device_group_id"`
}

type DeleteNetworkSliceDeviceGroupResponse struct {
	Error  string                                      `json:"error,omitempty"`
	Result DeleteNetworkSliceDeviceGroupResponseResult `json:"result"`
}

func listNetworkSliceDeviceGroups(url string, client *http.Client) (int, *ListNetworkSliceDeviceGroupsResponse, error) {
	req, err := http.NewRequest("GET", url+"/api/v1/network-slices/1/device-groups", nil)
	if err != nil {
		return 0, nil, err
	}
	res, err := client.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer res.Body.Close()
	var listResponse ListNetworkSliceDeviceGroupsResponse
	if err := json.NewDecoder(res.Body).Decode(&listResponse); err != nil {
		return 0, nil, err
	}
	return res.StatusCode, &listResponse, nil
}

func createNetworkSliceDeviceGroup(url string, client *http.Client, data *CreateNetworkSliceDeviceGroupParams) (int, *CreateNetworkSliceDeviceGroupResponse, error) {
	body, err := json.Marshal(data)
	if err != nil {
		return 0, nil, err
	}
	req, err := http.NewRequest("POST", url+"/api/v1/network-slices/1/device-groups", strings.NewReader(string(body)))
	if err != nil {
		return 0, nil, err
	}
	res, err := client.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer res.Body.Close()
	var createResponse CreateNetworkSliceDeviceGroupResponse
	if err := json.NewDecoder(res.Body).Decode(&createResponse); err != nil {
		return 0, nil, err
	}
	return res.StatusCode, &createResponse, nil
}

func deleteNetworkSliceDeviceGroup(url string, client *http.Client, id, deviceGroupID string) (int, *DeleteNetworkSliceDeviceGroupResponse, error) {
	req, err := http.NewRequest("DELETE", url+"/api/v1/network-slices/"+id+"/device-groups/"+deviceGroupID, nil)
	if err != nil {
		return 0, nil, err
	}
	res, err := client.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer res.Body.Close()
	var deleteResponse DeleteNetworkSliceDeviceGroupResponse
	if err := json.NewDecoder(res.Body).Decode(&deleteResponse); err != nil {
		return 0, nil, err
	}
	return res.StatusCode, &deleteResponse, nil
}

func TestNetworkSliceDeviceGroupsHandlers(t *testing.T) {
	ts, err := setupServer()
	if err != nil {
		t.Fatal(err)
	}
	defer ts.Close()
	client := ts.Client()

	t.Run("List network slice device groups - not found", func(t *testing.T) {
		statusCode, response, err := listNetworkSliceDeviceGroups(ts.URL, client)
		if err != nil {
			t.Fatal(err)
		}

		if statusCode != http.StatusNotFound {
			t.Fatalf("expected status %d, got %d", http.StatusNotFound, statusCode)
		}

		if response.Error != "network slice not found" {
			t.Fatalf("expected error %q, got %q", "network slice not found", response.Error)
		}

		if len(response.Result) != 0 {
			t.Fatalf("expected result %v, got %v", []int{}, response.Result)
		}
	})

	t.Run("Create network slice", func(t *testing.T) {
		data := CreateNetworkSliceParams{
			Name:     "Name1",
			Sst:      "Sst1",
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

	t.Run("List network slice device groups - 0", func(t *testing.T) {
		statusCode, response, err := listNetworkSliceDeviceGroups(ts.URL, client)
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

	t.Run("Create network slice device group - device group does not exist", func(t *testing.T) {
		data := CreateNetworkSliceDeviceGroupParams{
			DeviceGroupID: 1,
		}
		statusCode, response, err := createNetworkSliceDeviceGroup(ts.URL, client, &data)
		if err != nil {
			t.Fatal(err)
		}

		if statusCode != http.StatusNotFound {
			t.Fatalf("expected status %d, got %d", http.StatusNotFound, statusCode)
		}

		if response.Error != "device group not found" {
			t.Fatalf("expected error %q, got %q", "device group not found", response.Error)
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

	t.Run("Create device group - 2", func(t *testing.T) {
		data := CreateDeviceGroupParams{
			Name:             "Name2",
			SiteInfo:         "SiteInfo2",
			IpDomainName:     "IpDomainName2",
			Dnn:              "internet2",
			UeIpPool:         "1.0.0.0/24",
			DnsPrimary:       "9.9.9.9",
			Mtu:              1500,
			DnnMbrUplink:     20000000,
			DnnMbrDownlink:   20000000,
			TrafficClassName: "gold",
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

		if response.Result.ID != 2 {
			t.Fatalf("expected id %d, got %d", 2, response.Result.ID)
		}
	})

	t.Run("Create device group - 3", func(t *testing.T) {
		data := CreateDeviceGroupParams{
			Name:             "Name3",
			SiteInfo:         "SiteInfo3",
			IpDomainName:     "IpDomainName3",
			Dnn:              "internet3",
			UeIpPool:         "4.0.0.0/24",
			DnsPrimary:       "3.3.3.3",
			Mtu:              1500,
			DnnMbrUplink:     20000000,
			DnnMbrDownlink:   20000000,
			TrafficClassName: "silver",
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

		if response.Result.ID != 3 {
			t.Fatalf("expected id %d, got %d", 3, response.Result.ID)
		}
	})

	t.Run("Create network slice device group - 1", func(t *testing.T) {
		data := CreateNetworkSliceDeviceGroupParams{
			DeviceGroupID: 1,
		}
		statusCode, response, err := createNetworkSliceDeviceGroup(ts.URL, client, &data)
		if err != nil {
			t.Fatal(err)
		}

		if statusCode != http.StatusCreated {
			t.Fatalf("expected status %d, got %d", http.StatusCreated, statusCode)
		}

		if response.Result.DeviceGroupID != 1 {
			t.Fatalf("expected device_group_id %d, got %d", 2, response.Result.DeviceGroupID)
		}
	})

	t.Run("Create network slice device group - 2", func(t *testing.T) {
		data := CreateNetworkSliceDeviceGroupParams{
			DeviceGroupID: 2,
		}
		statusCode, response, err := createNetworkSliceDeviceGroup(ts.URL, client, &data)
		if err != nil {
			t.Fatal(err)
		}

		if statusCode != http.StatusCreated {
			t.Fatalf("expected status %d, got %d", http.StatusCreated, statusCode)
		}

		if response.Result.DeviceGroupID != 2 {
			t.Fatalf("expected device_group_id %d, got %d", 2, response.Result.DeviceGroupID)
		}
	})

	t.Run("Create network slice device group - 3", func(t *testing.T) {
		data := CreateNetworkSliceDeviceGroupParams{
			DeviceGroupID: 3,
		}
		statusCode, response, err := createNetworkSliceDeviceGroup(ts.URL, client, &data)
		if err != nil {
			t.Fatal(err)
		}

		if statusCode != http.StatusCreated {
			t.Fatalf("expected status %d, got %d", http.StatusCreated, statusCode)
		}

		if response.Result.DeviceGroupID != 3 {
			t.Fatalf("expected device_group_id %d, got %d", 3, response.Result.DeviceGroupID)
		}
	})

	t.Run("Create network slice device group - device group already exists", func(t *testing.T) {
		data := CreateNetworkSliceDeviceGroupParams{
			DeviceGroupID: 1,
		}
		statusCode, response, err := createNetworkSliceDeviceGroup(ts.URL, client, &data)
		if err != nil {
			t.Fatal(err)
		}

		if statusCode != http.StatusConflict {
			t.Fatalf("expected status %d, got %d", http.StatusConflict, statusCode)
		}

		if response.Error != "device group already in network slice" {
			t.Fatalf("expected error %q, got %q", "device group already in network slice", response.Error)
		}
	})

	t.Run("List network slice device groups - 3", func(t *testing.T) {
		statusCode, response, err := listNetworkSliceDeviceGroups(ts.URL, client)
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

	t.Run("Delete device group - 1", func(t *testing.T) {
		statusCode, response, err := deleteDeviceGroup(ts.URL, client, "1")
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

	t.Run("List network slice device groups - 1", func(t *testing.T) {
		statusCode, response, err := listNetworkSliceDeviceGroups(ts.URL, client)
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

	t.Run("Delete network slice device group - 2", func(t *testing.T) {
		statusCode, response, err := deleteNetworkSliceDeviceGroup(ts.URL, client, "1", "2")
		if err != nil {
			t.Fatal(err)
		}

		if statusCode != http.StatusAccepted {
			t.Fatalf("expected status %d, got %d", http.StatusAccepted, statusCode)
		}

		if response.Error != "" {
			t.Fatalf("expected error %q, got %q", "", response.Error)
		}

		if response.Result.DeviceGroupID != 2 {
			t.Fatalf("expected device_group_id %d, got %d", 2, response.Result.DeviceGroupID)
		}
	})

	t.Run("List network slice device groups - 1", func(t *testing.T) {
		statusCode, response, err := listNetworkSliceDeviceGroups(ts.URL, client)
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

	t.Run("List device groups - 2", func(t *testing.T) {
		statusCode, response, err := listDeviceGroups(ts.URL, client)
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

	t.Run("Get device group - 2 - should still exist", func(t *testing.T) {
		statusCode, _, err := getDeviceGroup(ts.URL, client, "2")
		if err != nil {
			t.Fatal(err)
		}

		if statusCode != http.StatusOK {
			t.Fatalf("expected status %d, got %d", http.StatusOK, statusCode)
		}
	})
}
