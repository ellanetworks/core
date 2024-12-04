package db

import (
	"context"
	"fmt"
	"time"

	"github.com/omec-project/util/mongoapi"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.uber.org/zap"
)

const (
	DevGroupDataColl = "webconsoleData.snapshots.devGroupData"
	SliceDataColl    = "webconsoleData.snapshots.sliceData"
	GnbDataColl      = "webconsoleData.snapshots.gnbData"

	AuthSubsDataColl                     = "subscriptionData.authenticationData.authenticationSubscription"
	SUBSCDATA_AUT_AUTHSTATUS             = "subscriptionData.authenticationData.authenticationStatus"
	AmDataColl                           = "subscriptionData.provisionedData.amData"
	SmDataColl                           = "subscriptionData.provisionedData.smData"
	SmfSelDataColl                       = "subscriptionData.provisionedData.smfSelectionSubscriptionData"
	SUBSCDATA_PROVISIONED_SMSMNG         = "subscriptionData.provisionedData.smsMngData"
	SUBSCDATA_PROVISIONED_SMS            = "subscriptionData.provisionedData.smsData"
	SUBSCDATA_PROVISIONED_TRACE          = "subscriptionData.provisionedData.traceData"
	SUBSCDATA_CTXDATA_AMF_3GPPACCESS     = "subscriptionData.contextData.amf3gppAccess"
	SUBSCDATA_CTXDATA_AMF_NON3GPPACCESS  = "subscriptionData.contextData.amfNon3gppAccess"
	SUBSCDATA_CTXDATA_SMF_REGISTRATION   = "subscriptionData.contextData.smfRegistrations"
	SUBSCDATA_CTXDATA_SMSF_3GPPACCESS    = "subscriptionData.contextData.smsf3gppAccess"
	SUBSCDATA_CTXDATA_SMSF_NON3GPPACCESS = "subscriptionData.contextData.smsfNon3gppAccess"
	SUBSCDATA_PPData                     = "subscriptionData.ppData"
	SUBSCDATA_IDENTITY                   = "subscriptionData.identityData"
	SUBSCDATA_EEPROFILE                  = "subscriptionData.eeProfileData"
	SUBSCDATA_OPERATORSPECIFIC           = "subscriptionData.operatorSpecificData"
	SUBSCDATA_OPERATORDETERMINEDBARRING  = "subscriptionData.operatorDeterminedBarringData"
	SUBSCDATA_SHARED                     = "subscriptionData.sharedData"
	SUBSCDATA_UEUPDATECONFIRMATION_SOR   = "subscriptionData.ueUpdateConfirmationData.sorData"

	AmPolicyDataColl                   = "policyData.ues.amData"
	SmPolicyDataColl                   = "policyData.ues.smData"
	FlowRuleDataColl                   = "policyData.ues.flowRule"
	POLICYDATA_BDTDATA                 = "policyData.bdtData"
	POLICYDATA_UES_OPSPECDATA          = "policyData.ues.operatorSpecificData"
	POLICYDATA_UES_SMDATA_USAGEMONDATA = "policyData.ues.smData.usageMonData"
	POLICYDATA_UES_UEPOLICYSET         = "policyData.ues.uePolicySet"
	POLICYDATA_PLMNs_UEPOLICYSET       = "policyData.plmns.uePolicySet"
	POLICYDATA_SPONSORS                = "policyData.sponsorConnectivityData"

	APPDATA_INFLUDATA_DB_COLLECTION_NAME       = "applicationData.influenceData"
	APPDATA_INFLUDATA_SUBSC_DB_COLLECTION_NAME = "applicationData.influenceData.subsToNotify"
	APPDATA_PFD_DB_COLLECTION_NAME             = "applicationData.pfds"
)

type DBInterface interface {
	RestfulAPIGetOne(collName string, filter bson.M) (map[string]interface{}, error)
	RestfulAPIGetMany(collName string, filter bson.M) ([]map[string]interface{}, error)
	RestfulAPIPutOne(collName string, filter bson.M, putData map[string]interface{}) (bool, error)
	RestfulAPIDeleteOne(collName string, filter bson.M) error
	RestfulAPIMergePatch(collName string, filter bson.M, patchData map[string]interface{}) error
	RestfulAPIJSONPatch(collName string, filter bson.M, patchJSON []byte) error
	RestfulAPIJSONPatchExtend(collName string, filter bson.M, patchJSON []byte, dataName string) error
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

func Initialize(url string, name string) error {
	SetLogLevel(zap.DebugLevel)
	err := TestConnection(url)
	if err != nil {
		DbLog.Fatalf("failed to connect to MongoDB: %v", err)
		return err
	}
	ConnectMongo(url, name)
	return nil
}

func TestConnection(url string) error {
	clientOptions := options.Client().ApplyURI(url)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	client, err := mongo.Connect(ctx, clientOptions)
	if err != nil {
		return fmt.Errorf("failed to connect to MongoDB: %w", err)
	}
	defer client.Disconnect(ctx)

	err = client.Ping(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to ping MongoDB: %w", err)
	}

	return nil
}

func (db *MongoDBClient) RestfulAPIGetOne(collName string, filter bson.M) (map[string]interface{}, error) {
	return db.MongoClient.RestfulAPIGetOne(collName, filter)
}

func (db *MongoDBClient) RestfulAPIGetMany(collName string, filter bson.M) ([]map[string]interface{}, error) {
	return db.MongoClient.RestfulAPIGetMany(collName, filter)
}

func (db *MongoDBClient) RestfulAPIPutOne(collName string, filter bson.M, putData map[string]interface{}) (bool, error) {
	return db.MongoClient.RestfulAPIPutOne(collName, filter, putData)
}

func (db *MongoDBClient) RestfulAPIDeleteOne(collName string, filter bson.M) error {
	return db.MongoClient.RestfulAPIDeleteOne(collName, filter)
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
