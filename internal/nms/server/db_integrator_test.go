package server

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/yeastengine/ella/internal/nms/db"
	"github.com/yeastengine/ella/internal/nms/models"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

var postData []map[string]interface{}

func deviceGroup(name string) models.DeviceGroups {
	traffic_class := models.TrafficClassInfo{
		Name: "platinum",
		Qci:  8,
		Arp:  6,
		Pdb:  300,
		Pelr: 6,
	}
	qos := models.DeviceGroupsIpDomainExpandedUeDnnQos{
		DnnMbrUplink:   10000000,
		DnnMbrDownlink: 10000000,
		BitrateUnit:    "kbps",
		TrafficClass:   &traffic_class,
	}
	ipdomain := models.DeviceGroupsIpDomainExpanded{
		Dnn:          "internet",
		UeIpPool:     "172.250.1.0/16",
		DnsPrimary:   "1.1.1.1",
		DnsSecondary: "8.8.8.8",
		Mtu:          1460,
		UeDnnQos:     &qos,
	}
	deviceGroup := models.DeviceGroups{
		DeviceGroupName:  name,
		Imsis:            []string{"1234", "5678"},
		SiteInfo:         "demo",
		IpDomainName:     "pool1",
		IpDomainExpanded: ipdomain,
	}
	return deviceGroup
}

type MockMongoPost struct {
	db.DBInterface
}

type MockMongoGetOneNil struct {
	db.DBInterface
}

type MockMongoGetManyNil struct {
	db.DBInterface
}

type MockMongoGetManyGroups struct {
	db.DBInterface
}

type MockMongoGetManySlices struct {
	db.DBInterface
}

type MockMongoDeviceGroupGetOne struct {
	db.DBInterface
	testGroup models.DeviceGroups
}

type MockMongoSliceGetOne struct {
	db.DBInterface
	testSlice models.Slice
}

func (m *MockMongoPost) RestfulAPIPost(coll string, filter primitive.M, data map[string]interface{}) (bool, error) {
	params := map[string]interface{}{
		"coll":   coll,
		"filter": filter,
		"data":   data,
	}
	postData = append(postData, params)
	return true, nil
}

func (m *MockMongoGetOneNil) RestfulAPIGetOne(collName string, filter bson.M) (map[string]interface{}, error) {
	var value map[string]interface{}
	return value, nil
}

func (m *MockMongoDeviceGroupGetOne) RestfulAPIGetOne(collName string, filter bson.M) (map[string]interface{}, error) {
	var previousGroupBson bson.M
	previousGroup, err := json.Marshal(m.testGroup)
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal(previousGroup, &previousGroupBson)
	if err != nil {
		return nil, err
	}
	return previousGroupBson, nil
}

func (m *MockMongoSliceGetOne) RestfulAPIGetOne(collName string, filter bson.M) (map[string]interface{}, error) {
	var previousSliceBson bson.M
	previousSlice, err := json.Marshal(m.testSlice)
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal(previousSlice, &previousSliceBson)
	if err != nil {
		return nil, err
	}
	return previousSliceBson, nil
}

func Test_handleDeviceGroupPost(t *testing.T) {
	deviceGroups := []models.DeviceGroups{deviceGroup("group1"), deviceGroup("group2"), deviceGroup("group_no_imsis"), deviceGroup("group_no_traf_class"), deviceGroup("group_no_qos")}
	deviceGroups[2].Imsis = []string{}
	deviceGroups[3].IpDomainExpanded.UeDnnQos.TrafficClass = nil
	deviceGroups[4].IpDomainExpanded.UeDnnQos = nil
	for _, testGroup := range deviceGroups {
		configMsg := models.ConfigMessage{
			DevGroupName: testGroup.DeviceGroupName,
			DevGroup:     &testGroup,
		}
		postData = make([]map[string]interface{}, 0)
		db.CommonDBClient = &(MockMongoPost{db.CommonDBClient})
		db.CommonDBClient = &MockMongoGetOneNil{db.CommonDBClient}
		handleDeviceGroupPost(&configMsg)
		expected_collection := "webconsoleData.snapshots.devGroupData"
		if postData[0]["coll"] != expected_collection {
			t.Errorf("Expected collection %v, got %v", expected_collection, postData[0]["coll"])
		}
		expected_filter := bson.M{"group-name": testGroup.DeviceGroupName}
		if !reflect.DeepEqual(postData[0]["filter"], expected_filter) {
			t.Errorf("Expected filter %v, got %v", expected_filter, postData[0]["filter"])
		}
		var resultGroup models.DeviceGroups
		var result map[string]interface{} = postData[0]["data"].(map[string]interface{})
		err := json.Unmarshal(mapToByte(result), &resultGroup)
		if err != nil {
			t.Errorf("Could not unmarshall result %v", result)
		}
		if !reflect.DeepEqual(resultGroup, testGroup) {
			t.Errorf("Expected group %v, got %v", testGroup, resultGroup)
		}
	}
}

func Test_handleDeviceGroupPost_alreadyExists(t *testing.T) {
	deviceGroups := []models.DeviceGroups{deviceGroup("group1"), deviceGroup("group2"), deviceGroup("group_no_imsis"), deviceGroup("group_no_traf_class"), deviceGroup("group_no_qos")}
	deviceGroups[2].Imsis = []string{}
	deviceGroups[3].IpDomainExpanded.UeDnnQos.TrafficClass = nil
	deviceGroups[4].IpDomainExpanded.UeDnnQos = nil

	for _, testGroup := range deviceGroups {
		configMsg := models.ConfigMessage{
			DevGroupName: testGroup.DeviceGroupName,
			DevGroup:     &testGroup,
		}
		postData = make([]map[string]interface{}, 0)
		db.CommonDBClient = &MockMongoPost{db.CommonDBClient}
		db.CommonDBClient = &(MockMongoDeviceGroupGetOne{db.CommonDBClient, testGroup})
		handleDeviceGroupPost(&configMsg)
		expected_collection := "webconsoleData.snapshots.devGroupData"
		if postData[0]["coll"] != expected_collection {
			t.Errorf("Expected collection %v, got %v", expected_collection, postData[0]["coll"])
		}
		expected_filter := bson.M{"group-name": testGroup.DeviceGroupName}
		if !reflect.DeepEqual(postData[0]["filter"], expected_filter) {
			t.Errorf("Expected filter %v, got %v", expected_filter, postData[0]["filter"])
		}
		var resultGroup models.DeviceGroups
		var result map[string]interface{} = postData[0]["data"].(map[string]interface{})
		err := json.Unmarshal(mapToByte(result), &resultGroup)
		if err != nil {
			t.Errorf("Could not unmarshall result %v", result)
		}
		if !reflect.DeepEqual(resultGroup, testGroup) {
			t.Errorf("Expected group %v, got %v", testGroup, resultGroup)
		}
	}
}

func networkSlice(name string) models.Slice {
	upf := make(map[string]interface{}, 0)
	upf["upf-name"] = "upf"
	upf["upf-port"] = "8805"
	plmn := models.SliceSiteInfoPlmn{
		Mcc: "208",
		Mnc: "93",
	}
	gnodeb := models.SliceSiteInfoGNodeBs{
		Name: "demo-gnb1",
		Tac:  1,
	}
	slice_id := models.SliceSliceId{
		Sst: "1",
		Sd:  "010203",
	}
	site_info := models.SliceSiteInfo{
		SiteName: "demo",
		Plmn:     plmn,
		GNodeBs:  []models.SliceSiteInfoGNodeBs{gnodeb},
		Upf:      upf,
	}
	slice := models.Slice{
		SliceName:       name,
		SliceId:         slice_id,
		SiteDeviceGroup: []string{"group1", "group2"},
		SiteInfo:        site_info,
	}
	return slice
}

func Test_handleSubscriberPost(t *testing.T) {
	ueId := "208930100007487"
	configMsg := models.ConfigMessage{
		MsgType: models.Sub_data,
		Imsi:    ueId,
	}

	postData = make([]map[string]interface{}, 0)
	db.CommonDBClient = &MockMongoPost{}
	handleSubscriberPost(&configMsg)

	expected_collection := "subscriptionData.provisionedData.amData"
	if postData[0]["coll"] != expected_collection {
		t.Errorf("Expected collection %v, got %v", expected_collection, postData[0]["coll"])
	}

	expected_filter := bson.M{"ueId": ueId}
	if !reflect.DeepEqual(postData[0]["filter"], expected_filter) {
		t.Errorf("Expected filter %v, got %v", expected_filter, postData[0]["filter"])
	}

	var result map[string]interface{} = postData[0]["data"].(map[string]interface{})
	if result["ueId"] != ueId {
		t.Errorf("Expected ueId %v, got %v", ueId, result["ueId"])
	}
}

func (m *MockMongoGetManyNil) RestfulAPIGetMany(collName string, filter bson.M) ([]map[string]interface{}, error) {
	var value []map[string]interface{}
	return value, nil
}

func (m *MockMongoGetManyGroups) RestfulAPIGetMany(collName string, filter bson.M) ([]map[string]interface{}, error) {
	testGroup := deviceGroup("testGroup")
	var previousGroupBson bson.M
	previousGroup, err := json.Marshal(testGroup)
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal(previousGroup, &previousGroupBson)
	if err != nil {
		return nil, err
	}
	var groups []map[string]interface{}
	groups = append(groups, previousGroupBson)
	return groups, nil
}

func (m *MockMongoGetManySlices) RestfulAPIGetMany(collName string, filter bson.M) ([]map[string]interface{}, error) {
	testSlice := networkSlice("testGroup")
	var previousSliceBson bson.M
	previousSlice, err := json.Marshal(testSlice)
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal(previousSlice, &previousSliceBson)
	if err != nil {
		return nil, err
	}
	var slices []map[string]interface{}
	slices = append(slices, previousSliceBson)
	return slices, nil
}

func Test_firstConfigReceived_noConfigInDB(t *testing.T) {
	db.CommonDBClient = &MockMongoGetManyNil{}
	result := firstConfigReceived()
	if result {
		t.Errorf("Expected firstConfigReceived to return false, got %v", result)
	}
}

func Test_firstConfigReceived_deviceGroupInDB(t *testing.T) {
	db.CommonDBClient = &MockMongoGetManyGroups{}
	result := firstConfigReceived()
	if !result {
		t.Errorf("Expected firstConfigReceived to return true, got %v", result)
	}
}

func Test_firstConfigReceived_sliceInDB(t *testing.T) {
	db.CommonDBClient = &MockMongoGetManySlices{}
	result := firstConfigReceived()
	if !result {
		t.Errorf("Expected firstConfigReceived to return true, got %v", result)
	}
}

func TestPostGnb(t *testing.T) {
	gnbName := "some-gnb"
	newGnb := models.Gnb{
		Name: gnbName,
		Tac:  "1233",
	}

	configMsg := models.ConfigMessage{
		MsgType:   models.Inventory,
		MsgMethod: models.Post_op,
		GnbName:   gnbName,
		Gnb:       &newGnb,
	}

	postData = make([]map[string]interface{}, 0)
	db.CommonDBClient = &MockMongoPost{}
	handleGnbPost(&configMsg)

	expected_collection := "webconsoleData.snapshots.gnbData"
	if postData[0]["coll"] != expected_collection {
		t.Errorf("Expected collection %v, got %v", expected_collection, postData[0]["coll"])
	}

	expected_filter := bson.M{"name": gnbName}
	if !reflect.DeepEqual(postData[0]["filter"], expected_filter) {
		t.Errorf("Expected filter %v, got %v", expected_filter, postData[0]["filter"])
	}

	var result map[string]interface{} = postData[0]["data"].(map[string]interface{})
	if result["tac"] != newGnb.Tac {
		t.Errorf("Expected port %v, got %v", newGnb.Tac, result["tac"])
	}
}
