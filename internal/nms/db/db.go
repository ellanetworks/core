package db

import (
	"time"

	"github.com/omec-project/util/mongoapi"
	"github.com/yeastengine/ella/internal/nms/logger"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

const (
	DevGroupDataColl = "webconsoleData.snapshots.devGroupData"
	SliceDataColl    = "webconsoleData.snapshots.sliceData"
	GnbDataColl      = "webconsoleData.snapshots.gnbData"

	AuthSubsDataColl = "subscriptionData.authenticationData.authenticationSubscription"
	AmDataColl       = "subscriptionData.provisionedData.amData"
	SmDataColl       = "subscriptionData.provisionedData.smData"
	SmfSelDataColl   = "subscriptionData.provisionedData.smfSelectionSubscriptionData"

	AmPolicyDataColl = "policyData.ues.amData"
	SmPolicyDataColl = "policyData.ues.smData"
	FlowRuleDataColl = "policyData.ues.flowRule"
)

type DBInterface interface {
	RestfulAPIGetOne(collName string, filter bson.M) (map[string]interface{}, error)
	RestfulAPIGetMany(collName string, filter bson.M) ([]map[string]interface{}, error)
	RestfulAPIPutOneTimeout(collName string, filter bson.M, putData map[string]interface{}, timeout int32, timeField string) bool
	RestfulAPIPutOne(collName string, filter bson.M, putData map[string]interface{}) (bool, error)
	RestfulAPIPutOneNotUpdate(collName string, filter bson.M, putData map[string]interface{}) (bool, error)
	RestfulAPIPutMany(collName string, filterArray []primitive.M, putDataArray []map[string]interface{}) error
	RestfulAPIDeleteOne(collName string, filter bson.M) error
	RestfulAPIDeleteMany(collName string, filter bson.M) error
	RestfulAPIMergePatch(collName string, filter bson.M, patchData map[string]interface{}) error
	RestfulAPIJSONPatch(collName string, filter bson.M, patchJSON []byte) error
	RestfulAPIJSONPatchExtend(collName string, filter bson.M, patchJSON []byte, dataName string) error
	RestfulAPIPost(collName string, filter bson.M, postData map[string]interface{}) (bool, error)
	RestfulAPIPostMany(collName string, filter bson.M, postDataArray []interface{}) error
}

var (
	CommonDBClient DBInterface
	AuthDBClient   DBInterface
)

type MongoDBClient struct {
	mongoapi.MongoClient
}

// Set CommonDBClient
func setCommonDBClient(url string, dbname string) error {
	mClient, errConnect := mongoapi.NewMongoClient(url, dbname)
	if mClient.Client != nil {
		CommonDBClient = mClient
		CommonDBClient.(*mongoapi.MongoClient).Client.Database(dbname)
	}
	return errConnect
}

// Set AuthDBClient
func setAuthDBClient(authurl string, authkeysdbname string) error {
	mClient, errConnect := mongoapi.NewMongoClient(authurl, authkeysdbname)
	if mClient.Client != nil {
		AuthDBClient = mClient
		AuthDBClient.(*mongoapi.MongoClient).Client.Database(authkeysdbname)
	}
	return errConnect
}

func ConnectMongo(url string, dbname string, authurl string, authkeysdbname string) {
	// Connect to MongoDB
	ticker := time.NewTicker(2 * time.Second)
	defer func() { ticker.Stop() }()
	timer := time.After(180 * time.Second)
ConnectMongo:
	for {
		commonDbErr := setCommonDBClient(url, dbname)
		authDbErr := setAuthDBClient(authurl, authkeysdbname)
		if commonDbErr == nil && authDbErr == nil {
			break ConnectMongo
		}
		select {
		case <-ticker.C:
			continue
		case <-timer:
			logger.DbLog.Errorln("Timed out while connecting to MongoDB in 3 minutes.")
			return
		}
	}

	logger.DbLog.Infoln("Connected to MongoDB.")
}

func (db *MongoDBClient) RestfulAPIGetOne(collName string, filter bson.M) (map[string]interface{}, error) {
	return db.MongoClient.RestfulAPIGetOne(collName, filter)
}

func (db *MongoDBClient) RestfulAPIGetMany(collName string, filter bson.M) ([]map[string]interface{}, error) {
	return db.MongoClient.RestfulAPIGetMany(collName, filter)
}

func (db *MongoDBClient) RestfulAPIPutOneTimeout(collName string, filter bson.M, putData map[string]interface{}, timeout int32, timeField string) bool {
	return db.MongoClient.RestfulAPIPutOneTimeout(collName, filter, putData, timeout, timeField)
}

func (db *MongoDBClient) RestfulAPIPutOne(collName string, filter bson.M, putData map[string]interface{}) (bool, error) {
	return db.MongoClient.RestfulAPIPutOne(collName, filter, putData)
}

func (db *MongoDBClient) RestfulAPIPutOneNotUpdate(collName string, filter bson.M, putData map[string]interface{}) (bool, error) {
	return db.MongoClient.RestfulAPIPutOneNotUpdate(collName, filter, putData)
}

func (db *MongoDBClient) RestfulAPIPutMany(collName string, filterArray []primitive.M, putDataArray []map[string]interface{}) error {
	return db.MongoClient.RestfulAPIPutMany(collName, filterArray, putDataArray)
}

func (db *MongoDBClient) RestfulAPIDeleteOne(collName string, filter bson.M) error {
	return db.MongoClient.RestfulAPIDeleteOne(collName, filter)
}

func (db *MongoDBClient) RestfulAPIDeleteMany(collName string, filter bson.M) error {
	return db.MongoClient.RestfulAPIDeleteMany(collName, filter)
}

func (db *MongoDBClient) RestfulAPIMergePatch(collName string, filter bson.M, patchData map[string]interface{}) error {
	return db.MongoClient.RestfulAPIMergePatch(collName, filter, patchData)
}

func (db *MongoDBClient) RestfulAPIJSONPatch(collName string, filter bson.M, patchJSON []byte) error {
	return db.MongoClient.RestfulAPIJSONPatch(collName, filter, patchJSON)
}

func (db *MongoDBClient) RestfulAPIJSONPatchExtend(collName string, filter bson.M, patchJSON []byte, dataName string) error {
	return db.MongoClient.RestfulAPIJSONPatchExtend(collName, filter, patchJSON, dataName)
}

func (db *MongoDBClient) RestfulAPIPost(collName string, filter bson.M, postData map[string]interface{}) (bool, error) {
	return db.MongoClient.RestfulAPIPost(collName, filter, postData)
}

func (db *MongoDBClient) RestfulAPIPostMany(collName string, filter bson.M, postDataArray []interface{}) error {
	return db.MongoClient.RestfulAPIPostMany(collName, filter, postDataArray)
}
