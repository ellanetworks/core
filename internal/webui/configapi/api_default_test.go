// SPDX-FileCopyrightText: 2023 Open Networking Foundation <info@opennetworking.org>
//
// SPDX-License-Identifier: Apache-2.0
//

package configapi

import (
	"encoding/json"
	"io"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/yeastengine/canard/internal/webui/configmodels"
	"github.com/yeastengine/canard/internal/webui/dbadapter"
	"go.mongodb.org/mongo-driver/bson"
)

type MockMongoClientNoDeviceGroups struct {
	dbadapter.DBInterface
}

type MockMongoClientOneDeviceGroups struct {
	dbadapter.DBInterface
}

type MockMongoClientManyDeviceGroups struct {
	dbadapter.DBInterface
}

type MockMongoClientNotFoundDeviceGroup struct {
	dbadapter.DBInterface
}

type MockMongoClientFoundDeviceGroup struct {
	dbadapter.DBInterface
}

type MockMongoClientNoNetworkSlice struct {
	dbadapter.DBInterface
}

type MockMongoClientNotFoundNetworkSlice struct {
	dbadapter.DBInterface
}

type MockMongoClientFoundNetworkSlice struct {
	dbadapter.DBInterface
}

type MockMongoClientOneNetworkSlice struct {
	dbadapter.DBInterface
}

type MockMongoClientManyNetworkSlices struct {
	dbadapter.DBInterface
}

func (m *MockMongoClientNoDeviceGroups) RestfulAPIGetMany(coll string, filter bson.M) ([]map[string]interface{}, error) {
	var results []map[string]interface{}
	return results, nil
}

func (m *MockMongoClientOneDeviceGroups) RestfulAPIGetMany(coll string, filter bson.M) ([]map[string]interface{}, error) {
	var results []map[string]interface{}
	dg := deviceGroup("group1")
	var dgbson bson.M
	tmp, _ := json.Marshal(dg)
	json.Unmarshal(tmp, &dgbson)

	results = append(results, dgbson)
	return results, nil
}

func (m *MockMongoClientManyDeviceGroups) RestfulAPIGetMany(coll string, filter bson.M) ([]map[string]interface{}, error) {
	var results []map[string]interface{}
	names := []string{"group1", "group2", "group3"}
	for _, name := range names {
		dg := deviceGroup(name)
		var dgbson bson.M
		tmp, _ := json.Marshal(dg)
		json.Unmarshal(tmp, &dgbson)

		results = append(results, dgbson)
	}
	return results, nil
}

func (m *MockMongoClientNotFoundDeviceGroup) RestfulAPIGetOne(coll string, filter bson.M) (map[string]interface{}, error) {
	return nil, nil
}

func (m *MockMongoClientFoundDeviceGroup) RestfulAPIGetOne(coll string, filter bson.M) (map[string]interface{}, error) {
	dg := deviceGroup("group1")
	var dgbson bson.M
	tmp, _ := json.Marshal(dg)
	json.Unmarshal(tmp, &dgbson)
	return dgbson, nil
}

func (m *MockMongoClientNoNetworkSlice) RestfulAPIGetMany(coll string, filter bson.M) ([]map[string]interface{}, error) {
	var results []map[string]interface{}
	return results, nil
}

func (m *MockMongoClientOneNetworkSlice) RestfulAPIGetMany(coll string, filter bson.M) ([]map[string]interface{}, error) {
	var results []map[string]interface{}
	ns := networkSlice("slice1")
	var slicebson bson.M
	tmp, _ := json.Marshal(ns)
	json.Unmarshal(tmp, &slicebson)
	results = append(results, slicebson)
	return results, nil
}

func (m *MockMongoClientManyNetworkSlices) RestfulAPIGetMany(coll string, filter bson.M) ([]map[string]interface{}, error) {
	var results []map[string]interface{}
	names := []string{"slice1", "slice2", "slice3"}
	for _, name := range names {
		ns := networkSlice(name)
		var slicebson bson.M
		tmp, _ := json.Marshal(ns)
		json.Unmarshal(tmp, &slicebson)

		results = append(results, slicebson)
	}
	return results, nil
}

func (m *MockMongoClientNotFoundNetworkSlice) RestfulAPIGetOne(coll string, filter bson.M) (map[string]interface{}, error) {
	return nil, nil
}

func (m *MockMongoClientFoundNetworkSlice) RestfulAPIGetOne(coll string, filter bson.M) (map[string]interface{}, error) {
	ns := networkSlice("slice1")
	var slicebson bson.M
	tmp, _ := json.Marshal(ns)
	json.Unmarshal(tmp, &slicebson)
	return slicebson, nil
}

func deviceGroup(name string) configmodels.DeviceGroups {
	traffic_class := configmodels.TrafficClassInfo{
		Name: "platinum",
		Qci:  8,
		Arp:  6,
		Pdb:  300,
		Pelr: 6,
	}
	qos := configmodels.DeviceGroupsIpDomainExpandedUeDnnQos{
		DnnMbrUplink:   10000000,
		DnnMbrDownlink: 10000000,
		BitrateUnit:    "kbps",
		TrafficClass:   &traffic_class,
	}
	ipdomain := configmodels.DeviceGroupsIpDomainExpanded{
		Dnn:          "internet",
		UeIpPool:     "172.250.1.0/16",
		DnsPrimary:   "1.1.1.1",
		DnsSecondary: "8.8.8.8",
		Mtu:          1460,
		UeDnnQos:     &qos,
	}
	deviceGroup := configmodels.DeviceGroups{
		DeviceGroupName:  name,
		Imsis:            []string{"1234", "5678"},
		SiteInfo:         "demo",
		IpDomainName:     "pool1",
		IpDomainExpanded: ipdomain,
	}
	return deviceGroup
}

func TestGetDeviceGroupsNoGroups(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	dbadapter.CommonDBClient = &MockMongoClientNoDeviceGroups{}
	GetDeviceGroups(c)
	resp := w.Result()

	if resp.StatusCode != 200 {
		t.Errorf("Expected StatusCode %d, got %d", 200, resp.StatusCode)
	}
	body_bytes, _ := io.ReadAll(resp.Body)
	body := string(body_bytes)
	if body != "[]" {
		t.Errorf("Expected empty JSON list, got %v", body)
	}
}

func TestGetDeviceGroupsOneGroup(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	dbadapter.CommonDBClient = &MockMongoClientOneDeviceGroups{}
	GetDeviceGroups(c)
	resp := w.Result()

	if resp.StatusCode != 200 {
		t.Errorf("Expected StatusCode %d, got %d", 200, resp.StatusCode)
	}
	body_bytes, _ := io.ReadAll(resp.Body)
	body := string(body_bytes)
	expected := `["group1"]`
	if body != expected {
		t.Errorf("Expected %v, got %v", expected, body)
	}
}

func TestGetDeviceGroupsManyGroup(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	dbadapter.CommonDBClient = &MockMongoClientManyDeviceGroups{}
	GetDeviceGroups(c)
	resp := w.Result()

	if resp.StatusCode != 200 {
		t.Errorf("Expected StatusCode %d, got %d", 200, resp.StatusCode)
	}
	body_bytes, _ := io.ReadAll(resp.Body)
	body := string(body_bytes)
	expected := `["group1","group2","group3"]`
	if body != expected {
		t.Errorf("Expected %v, got %v", expected, body)
	}
}

func TestGetDeviceGroupByNameDoesNotExist(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	dbadapter.CommonDBClient = &MockMongoClientNotFoundDeviceGroup{}
	c.Params = append(c.Params, gin.Param{Key: "device-name", Value: "group1"})
	GetDeviceGroupByName(c)
	resp := w.Result()

	if resp.StatusCode != 404 {
		t.Errorf("Expected StatusCode %d, got %d", 404, resp.StatusCode)
	}
	body_bytes, _ := io.ReadAll(resp.Body)
	body := string(body_bytes)
	if body != "null" {
		t.Errorf("Expected %v, got %v", "null", body)
	}
}

func TestGetDeviceGroupByNameDoesExists(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	dbadapter.CommonDBClient = &MockMongoClientFoundDeviceGroup{}
	c.Params = append(c.Params, gin.Param{Key: "device-name", Value: "group1"})
	GetDeviceGroupByName(c)
	resp := w.Result()

	if resp.StatusCode != 200 {
		t.Errorf("Expected StatusCode %d, got %d", 200, resp.StatusCode)
	}
	body_bytes, _ := io.ReadAll(resp.Body)
	body := string(body_bytes)
	expected := `{"group-name":"group1","imsis":["1234","5678"],"site-info":"demo","ip-domain-name":"pool1","ip-domain-expanded":{"dnn":"internet","ue-ip-pool":"172.250.1.0/16","dns-primary":"1.1.1.1","dns-secondary":"8.8.8.8","mtu":1460,"ue-dnn-qos":{"dnn-mbr-uplink":10000000,"dnn-mbr-downlink":10000000,"bitrate-unit":"kbps","traffic-class":{"name":"platinum","qci":8,"arp":6,"pdb":300,"pelr":6}}}}`
	if body != expected {
		t.Errorf("Expected %v, got %v", expected, body)
	}
}

func networkSlice(name string) configmodels.Slice {
	upf := make(map[string]interface{}, 0)
	upf["upf-name"] = "upf"
	upf["upf-port"] = "8805"
	plmn := configmodels.SliceSiteInfoPlmn{
		Mcc: "208",
		Mnc: "93",
	}
	gnodeb := configmodels.SliceSiteInfoGNodeBs{
		Name: "demo-gnb1",
		Tac:  1,
	}
	slice_id := configmodels.SliceSliceId{
		Sst: "1",
		Sd:  "010203",
	}
	site_info := configmodels.SliceSiteInfo{
		SiteName: "demo",
		Plmn:     plmn,
		GNodeBs:  []configmodels.SliceSiteInfoGNodeBs{gnodeb},
		Upf:      upf,
	}
	slice := configmodels.Slice{
		SliceName:       name,
		SliceId:         slice_id,
		SiteDeviceGroup: []string{"group1", "group2"},
		SiteInfo:        site_info,
	}
	return slice
}

func TestGetNetworkSlicesNoSlices(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	dbadapter.CommonDBClient = &MockMongoClientNoNetworkSlice{}
	GetNetworkSlices(c)
	resp := w.Result()

	if resp.StatusCode != 200 {
		t.Errorf("Expected StatusCode %d, got %d", 200, resp.StatusCode)
	}
	body_bytes, _ := io.ReadAll(resp.Body)
	body := string(body_bytes)
	if body != "[]" {
		t.Errorf("Expected empty JSON list, got %v", body)
	}
}

func TestGetNetworkSlicesOneSlice(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	dbadapter.CommonDBClient = &MockMongoClientOneNetworkSlice{}
	GetNetworkSlices(c)
	resp := w.Result()

	if resp.StatusCode != 200 {
		t.Errorf("Expected StatusCode %d, got %d", 200, resp.StatusCode)
	}
	body_bytes, _ := io.ReadAll(resp.Body)
	body := string(body_bytes)
	expected := `["slice1"]`
	if body != expected {
		t.Errorf("Expected %v, got %v", expected, body)
	}
}

func TestGetNetworkSlicesManySlices(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	dbadapter.CommonDBClient = &MockMongoClientManyNetworkSlices{}
	GetNetworkSlices(c)
	resp := w.Result()

	if resp.StatusCode != 200 {
		t.Errorf("Expected StatusCode %d, got %d", 200, resp.StatusCode)
	}
	body_bytes, _ := io.ReadAll(resp.Body)
	body := string(body_bytes)
	expected := `["slice1","slice2","slice3"]`
	if body != expected {
		t.Errorf("Expected %v, got %v", expected, body)
	}
}

func TestGetNetworkSliceByNameDoesNotExist(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	dbadapter.CommonDBClient = &MockMongoClientNotFoundNetworkSlice{}
	c.Params = append(c.Params, gin.Param{Key: "slice-name", Value: "slice1"})
	GetNetworkSliceByName(c)
	resp := w.Result()

	if resp.StatusCode != 404 {
		t.Errorf("Expected StatusCode %d, got %d", 404, resp.StatusCode)
	}
	body_bytes, _ := io.ReadAll(resp.Body)
	body := string(body_bytes)
	if body != "null" {
		t.Errorf("Expected %v, got %v", "null", body)
	}
}

func TestGetNetworkSliceByNameDoesExists(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	dbadapter.CommonDBClient = &MockMongoClientFoundNetworkSlice{}
	c.Params = append(c.Params, gin.Param{Key: "slice-name", Value: "slice1"})
	GetNetworkSliceByName(c)
	resp := w.Result()

	if resp.StatusCode != 200 {
		t.Errorf("Expected StatusCode %d, got %d", 200, resp.StatusCode)
	}
	body_bytes, _ := io.ReadAll(resp.Body)
	body := string(body_bytes)
	expected := `{"SliceName":"slice1","slice-id":{"sst":"1","sd":"010203"},"site-device-group":["group1","group2"],"site-info":{"site-name":"demo","plmn":{"mcc":"208","mnc":"93"},"gNodeBs":[{"name":"demo-gnb1","tac":1}],"upf":{"upf-name":"upf","upf-port":"8805"}}}`
	if body != expected {
		t.Errorf("Expected %v, got %v", expected, body)
	}
}
