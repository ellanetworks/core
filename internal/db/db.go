package db

import (
	"time"

	"github.com/omec-project/util/mongoapi"
	"go.mongodb.org/mongo-driver/bson"
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
	RestfulAPIDeleteOne(collName string, filter bson.M) error
	RestfulAPIPost(collName string, filter bson.M, postData map[string]interface{}) (bool, error)
}

var CommonDBClient DBInterface

type MongoDBClient struct {
	mongoapi.MongoClient
}

func setCommonDBClient(url string, dbname string) error {
	mClient, errConnect := mongoapi.NewMongoClient(url, dbname)
	if mClient.Client != nil {
		CommonDBClient = mClient
		CommonDBClient.(*mongoapi.MongoClient).Client.Database(dbname)
	}
	return errConnect
}

func ConnectMongo(url string, dbname string) {
	ticker := time.NewTicker(2 * time.Second)
	defer func() { ticker.Stop() }()
	timer := time.After(180 * time.Second)
ConnectMongo:
	for {
		commonDbErr := setCommonDBClient(url, dbname)
		if commonDbErr == nil {
			break ConnectMongo
		}
		select {
		case <-ticker.C:
			continue
		case <-timer:
			DbLog.Errorln("Timed out while connecting to MongoDB in 3 minutes.")
			return
		}
	}

	DbLog.Infoln("Connected to MongoDB.")
}

func (db *MongoDBClient) RestfulAPIGetOne(collName string, filter bson.M) (map[string]interface{}, error) {
	return db.MongoClient.RestfulAPIGetOne(collName, filter)
}

func (db *MongoDBClient) RestfulAPIGetMany(collName string, filter bson.M) ([]map[string]interface{}, error) {
	return db.MongoClient.RestfulAPIGetMany(collName, filter)
}

func (db *MongoDBClient) RestfulAPIDeleteOne(collName string, filter bson.M) error {
	return db.MongoClient.RestfulAPIDeleteOne(collName, filter)
}

func (db *MongoDBClient) RestfulAPIPost(collName string, filter bson.M, postData map[string]interface{}) (bool, error) {
	return db.MongoClient.RestfulAPIPost(collName, filter, postData)
}
