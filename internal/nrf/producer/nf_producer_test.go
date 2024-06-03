package producer_test

import (
	"fmt"
	"testing"

	"github.com/google/uuid"
	"github.com/omec-project/openapi/models"
	"github.com/yeastengine/canard/internal/nrf/dbadapter"
	"github.com/yeastengine/canard/internal/nrf/factory"
	"github.com/yeastengine/canard/internal/nrf/producer"
	"go.mongodb.org/mongo-driver/bson"
)

type MockMongoDBClient struct {
	dbadapter.DBInterface
}

func init() {
	factory.InitConfigFactory("../nrfTest/nrfcfg.yaml")
}

func (db *MockMongoDBClient) RestfulAPIGetOne(collName string, filter bson.M) (map[string]interface{}, error) {
	fmt.Println("Called Mock RestfulAPIGetOne")
	return nil, nil
}

func (db *MockMongoDBClient) RestfulAPIGetMany(collName string, filter bson.M) ([]map[string]interface{}, error) {
	fmt.Println("Called Mock RestfulAPIGetMany")
	return nil, nil
}
func (db *MockMongoDBClient) PutOneWithTimeout(collName string, filter bson.M, putData map[string]interface{}, timeout int32, timeField string) bool {
	fmt.Println("Called Mock PutOneWithTimeout")
	return true
}
func (db *MockMongoDBClient) RestfulAPIPutOne(collName string, filter bson.M, putData map[string]interface{}) (bool, error) {
	fmt.Println("Called Mock RestfulAPIPutOne")
	return true, nil
}
func (db *MockMongoDBClient) RestfulAPIPutOneNotUpdate(collName string, filter bson.M, putData map[string]interface{}) (bool, error) {
	fmt.Println("Called Mock RestfulAPIPutOneNotUpdate")
	return true, nil
}
func (db *MockMongoDBClient) RestfulAPIPutMany(collName string, filterArray []bson.M, putDataArray []map[string]interface{}) error {
	fmt.Println("Called Mock RestfulAPIPutMany")
	return nil
}
func (db *MockMongoDBClient) RestfulAPIDeleteOne(collName string, filter bson.M) error {
	fmt.Println("Called Mock RestfulAPIDeleteOne")
	return nil
}
func (db *MockMongoDBClient) RestfulAPIDeleteMany(collName string, filter bson.M) error {
	fmt.Println("Called Mock RestfulAPIDeleteMany")
	return nil
}
func (db *MockMongoDBClient) RestfulAPIMergePatch(collName string, filter bson.M, patchData map[string]interface{}) error {
	fmt.Println("Called Mock RestfulAPIMergePatch")
	return nil
}
func (db *MockMongoDBClient) RestfulAPIJSONPatch(collName string, filter bson.M, patchJSON []byte) error {
	return nil
}
func (db *MockMongoDBClient) RestfulAPIJSONPatchExtend(collName string, filter bson.M, patchJSON []byte, dataName string) error {
	fmt.Println("Called Mock RestfulAPIJSONPatchExtend")
	return nil
}
func (db *MockMongoDBClient) RestfulAPIPost(collName string, filter bson.M, postData map[string]interface{}) (bool, error) {
	fmt.Println("Called Mock RestfulAPIPost")
	return true, nil
}
func (db *MockMongoDBClient) RestfulAPIPostMany(collName string, filter bson.M, postDataArray []interface{}) bool {
	fmt.Println("Called Mock RestfulAPIPost")
	return true
}

func TestNFRegisterProcedure(t *testing.T) {
	dbadapter.DBClient = &MockMongoDBClient{}
	var nf models.NfProfile
	nf.NfType = models.NfType_PCF
	nf.NfInstanceId = uuid.New().String()
	nf.NfStatus = models.NfStatus_REGISTERED
	_, _, err := producer.NFRegisterProcedure(nf)
	if err != nil {
		t.Errorf("testcase failed: %v", err)
	}
}
