package server_test

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

type ListDeviceGroupsResponseResult []int

type ListDeviceGroupsResponse struct {
	Error  string                         `json:"error,omitempty"`
	Result ListDeviceGroupsResponseResult `json:"result"`
}

type CreateDeviceGroupParams struct {
	Name             string `json:"name"`
	SiteInfo         string `json:"site_info"`
	IpDomainName     string `json:"ip_domain_name"`
	Dnn              string `json:"dnn"`
	UeIpPool         string `json:"ue_ip_pool"`
	DnsPrimary       string `json:"dns_primary"`
	Mtu              int64  `json:"mtu"`
	DnnMbrUplink     int64  `json:"dnn_mbr_uplink"`
	DnnMbrDownlink   int64  `json:"dnn_mbr_downlink"`
	TrafficClassName string `json:"traffic_class_name"`
	TrafficClassArp  int64  `json:"traffic_class_arp"`
	TrafficClassPdb  int64  `json:"traffic_class_pdb"`
	TrafficClassPelr int64  `json:"traffic_class_pelr"`
	TrafficClassQci  int64  `json:"traffic_class_qci"`
	NetworkSliceId   int64  `json:"network_slice_id"`
}

type createDeviceGroupResponseResult struct {
	ID int64 `json:"id"`
}

type createDeviceGroupResponse struct {
	Error  string                          `json:"error,omitempty"`
	Result createDeviceGroupResponseResult `json:"result"`
}

type GetDeviceGroupResponseResult struct {
	ID               int64  `json:"id"`
	Name             string `json:"name"`
	SiteInfo         string `json:"site_info"`
	IpDomainName     string `json:"ip_domain_name"`
	Dnn              string `json:"dnn"`
	UeIpPool         string `json:"ue_ip_pool"`
	DnsPrimary       string `json:"dns_primary"`
	Mtu              int64  `json:"mtu"`
	DnnMbrUplink     int64  `json:"dnn_mbr_uplink"`
	DnnMbrDownlink   int64  `json:"dnn_mbr_downlink"`
	TrafficClassName string `json:"traffic_class_name"`
	TrafficClassArp  int64  `json:"traffic_class_arp"`
	TrafficClassPdb  int64  `json:"traffic_class_pdb"`
	TrafficClassPelr int64  `json:"traffic_class_pelr"`
	TrafficClassQci  int64  `json:"traffic_class_qci"`
	NetworkSliceId   int64  `json:"network_slice_id"`
}

type GetDeviceGroupResponse struct {
	Error  string                       `json:"error,omitempty"`
	Result GetDeviceGroupResponseResult `json:"result"`
}

type DeleteDeviceGroupResponseResult struct {
	ID int64 `json:"id"`
}

type DeleteDeviceGroupResponse struct {
	Error  string                          `json:"error,omitempty"`
	Result DeleteDeviceGroupResponseResult `json:"result"`
}

func listDeviceGroups(url string, client *http.Client) (int, *ListDeviceGroupsResponse, error) {
	req, err := http.NewRequest("GET", url+"/api/v1/device-groups", nil)
	if err != nil {
		return 0, nil, err
	}
	res, err := client.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer res.Body.Close()
	var listResponse ListDeviceGroupsResponse
	if err := json.NewDecoder(res.Body).Decode(&listResponse); err != nil {
		return 0, nil, err
	}
	return res.StatusCode, &listResponse, nil
}

func getDeviceGroup(url string, client *http.Client, id string) (int, *GetDeviceGroupResponse, error) {
	req, err := http.NewRequest("GET", url+"/api/v1/device-groups/"+id, nil)
	if err != nil {
		return 0, nil, err
	}
	res, err := client.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer res.Body.Close()
	var getResponse GetDeviceGroupResponse
	if err := json.NewDecoder(res.Body).Decode(&getResponse); err != nil {
		return 0, nil, err
	}
	return res.StatusCode, &getResponse, nil
}

func createDeviceGroup(url string, client *http.Client, data *CreateDeviceGroupParams) (int, *createDeviceGroupResponse, error) {
	body, err := json.Marshal(data)
	if err != nil {
		return 0, nil, err
	}
	req, err := http.NewRequest("POST", url+"/api/v1/device-groups", strings.NewReader(string(body)))
	if err != nil {
		return 0, nil, err
	}
	res, err := client.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer res.Body.Close()
	var createResponse createDeviceGroupResponse
	if err := json.NewDecoder(res.Body).Decode(&createResponse); err != nil {
		return 0, nil, err
	}
	return res.StatusCode, &createResponse, nil
}

func deleteDeviceGroup(url string, client *http.Client, id string) (int, *DeleteDeviceGroupResponse, error) {
	req, err := http.NewRequest("DELETE", url+"/api/v1/device-groups/"+id, nil)
	if err != nil {
		return 0, nil, err
	}
	res, err := client.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer res.Body.Close()
	var deleteResponse DeleteDeviceGroupResponse
	if err := json.NewDecoder(res.Body).Decode(&deleteResponse); err != nil {
		return 0, nil, err
	}
	return res.StatusCode, &deleteResponse, nil
}

func TestDeviceGroupsHandlers(t *testing.T) {
	ts, err := setupServer()
	if err != nil {
		t.Fatal(err)
	}
	defer ts.Close()
	client := ts.Client()

	t.Run("List device groups - 0", func(t *testing.T) {
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

		if len(response.Result) != 0 {
			t.Fatalf("expected result %v, got %v", []int{}, response.Result)
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

	t.Run("Create device group with non-existent network slice", func(t *testing.T) {
		data := CreateDeviceGroupParams{
			Name:             "Name2",
			SiteInfo:         "SiteInfo2",
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
			NetworkSliceId:   2,
		}
		statusCode, response, _ := createDeviceGroup(ts.URL, client, &data)

		if statusCode != http.StatusBadRequest {
			t.Fatalf("expected status %d, got %d", http.StatusBadRequest, statusCode)
		}

		if response.Error != "network slice not found" {
			t.Fatalf("expected error %q, got %q", "network slice not found", response.Error)
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

	t.Run("Create device group (with network slice) - 2", func(t *testing.T) {
		data := CreateDeviceGroupParams{
			Name:             "Name2",
			SiteInfo:         "SiteInfo2",
			IpDomainName:     "IpDomainName2",
			Dnn:              "Dnn2",
			UeIpPool:         "10.0.0.0/24",
			DnsPrimary:       "9.9.9.9",
			Mtu:              1500,
			DnnMbrUplink:     10000000,
			DnnMbrDownlink:   10000000,
			TrafficClassName: "gold",
			TrafficClassArp:  5,
			TrafficClassPdb:  200,
			TrafficClassPelr: 5,
			TrafficClassQci:  7,
			NetworkSliceId:   1,
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
			t.Fatalf("expected result %v, got %v", []int{1, 2}, response.Result)
		}
	})

	t.Run("Get device group - 1", func(t *testing.T) {
		statusCode, response, err := getDeviceGroup(ts.URL, client, "1")
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

		if response.Result.Name != "Name1" {
			t.Fatalf("expected name %q, got %q", "Name1", response.Result.Name)
		}

		if response.Result.SiteInfo != "SiteInfo1" {
			t.Fatalf("expected site_info %q, got %q", "SiteInfo1", response.Result.SiteInfo)
		}

		if response.Result.IpDomainName != "IpDomainName1" {
			t.Fatalf("expected ip_domain_name %q, got %q", "IpDomainName1", response.Result.IpDomainName)
		}

		if response.Result.Dnn != "internet" {
			t.Fatalf("expected dnn %q, got %q", "internet", response.Result.Dnn)
		}

		if response.Result.UeIpPool != "11.0.0.0/24" {
			t.Fatalf("expected ue_ip_pool %q, got %q", "10.0.0.0/24", response.Result.UeIpPool)
		}

		if response.Result.DnsPrimary != "8.8.8.8" {
			t.Fatalf("expected dns_primary %q, got %q", "8.8.8.8", response.Result.DnsPrimary)
		}

		if response.Result.Mtu != 1460 {
			t.Fatalf("expected mtu %d, got %d", 1460, response.Result.Mtu)
		}

		if response.Result.DnnMbrUplink != 20000000 {
			t.Fatalf("expected dnn_mbr_uplink %d, got %d", 20000000, response.Result.DnnMbrUplink)
		}

		if response.Result.DnnMbrDownlink != 20000000 {
			t.Fatalf("expected dnn_mbr_downlink %d, got %d", 20000000, response.Result.DnnMbrDownlink)
		}

		if response.Result.TrafficClassName != "platinum" {
			t.Fatalf("expected traffic_class_name %q, got %q", "platinum", response.Result.TrafficClassName)
		}

		if response.Result.TrafficClassArp != 6 {
			t.Fatalf("expected traffic_class_arp %d, got %d", 6, response.Result.TrafficClassArp)
		}

		if response.Result.TrafficClassPdb != 300 {
			t.Fatalf("expected traffic_class_pdb %d, got %d", 300, response.Result.TrafficClassPdb)
		}

		if response.Result.TrafficClassPelr != 6 {
			t.Fatalf("expected traffic_class_pelr %d, got %d", 6, response.Result.TrafficClassPelr)
		}

		if response.Result.TrafficClassQci != 8 {
			t.Fatalf("expected traffic_class_qci %d, got %d", 8, response.Result.TrafficClassQci)
		}

		if response.Result.NetworkSliceId != 0 {
			t.Fatalf("expected network_slice_id %d, got %d", 0, response.Result.NetworkSliceId)
		}
	})

	t.Run("Get device group - 2", func(t *testing.T) {
		statusCode, response, err := getDeviceGroup(ts.URL, client, "2")
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

		if response.Result.Name != "Name2" {
			t.Fatalf("expected name %q, got %q", "Name2", response.Result.Name)
		}

		if response.Result.SiteInfo != "SiteInfo2" {
			t.Fatalf("expected site_info %q, got %q", "SiteInfo2", response.Result.SiteInfo)
		}

		if response.Result.IpDomainName != "IpDomainName2" {
			t.Fatalf("expected ip_domain_name %q, got %q", "IpDomainName2", response.Result.IpDomainName)
		}

		if response.Result.Dnn != "Dnn2" {
			t.Fatalf("expected dnn %q, got %q", "Dnn2", response.Result.Dnn)
		}

		if response.Result.UeIpPool != "10.0.0.0/24" {
			t.Fatalf("expected ue_ip_pool %q, got %q", "10.0.0.0/24", response.Result.UeIpPool)
		}

		if response.Result.DnsPrimary != "9.9.9.9" {
			t.Fatalf("expected dns_primary %q, got %q", "9.9.9.9", response.Result.DnsPrimary)
		}

		if response.Result.Mtu != 1500 {
			t.Fatalf("expected mtu %d, got %d", 1500, response.Result.Mtu)
		}

		if response.Result.DnnMbrUplink != 10000000 {
			t.Fatalf("expected dnn_mbr_uplink %d, got %d", 10000000, response.Result.DnnMbrUplink)
		}

		if response.Result.DnnMbrDownlink != 10000000 {
			t.Fatalf("expected dnn_mbr_downlink %d, got %d", 10000000, response.Result.DnnMbrDownlink)
		}

		if response.Result.TrafficClassName != "gold" {
			t.Fatalf("expected traffic_class_name %q, got %q", "gold", response.Result.TrafficClassName)
		}

		if response.Result.TrafficClassArp != 5 {
			t.Fatalf("expected traffic_class_arp %d, got %d", 5, response.Result.TrafficClassArp)
		}

		if response.Result.TrafficClassPdb != 200 {
			t.Fatalf("expected traffic_class_pdb %d, got %d", 200, response.Result.TrafficClassPdb)
		}

		if response.Result.TrafficClassPelr != 5 {
			t.Fatalf("expected traffic_class_pelr %d, got %d", 5, response.Result.TrafficClassPelr)
		}

		if response.Result.TrafficClassQci != 7 {
			t.Fatalf("expected traffic_class_qci %d, got %d", 7, response.Result.TrafficClassQci)
		}

		if response.Result.NetworkSliceId != 1 {
			t.Fatalf("expected network_slice_id %d, got %d", 1, response.Result.NetworkSliceId)
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

	t.Run("List device groups - 1", func(t *testing.T) {
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

		if len(response.Result) != 1 {
			t.Fatalf("expected result %v, got %v", []int{2}, response.Result)
		}
	})
}
