package queries

import (
	"encoding/json"
	"fmt"

	"github.com/yeastengine/ella/internal/db"
	"github.com/yeastengine/ella/internal/db/models"
	"github.com/yeastengine/ella/internal/logger"
	"go.mongodb.org/mongo-driver/bson"
)

func CreateSubscriber(subscriber *models.Subscriber) error {
	filter := bson.M{"ueId": subscriber.UeId}
	subscriberDataBson := toBsonM(&subscriber)
	_, err := db.CommonDBClient.RestfulAPIPost(db.SubscribersColl, filter, subscriberDataBson)
	if err != nil {
		return err
	}
	logger.DBLog.Infof("Created Subscriber: %v", subscriber.UeId)
	return nil
}

func GetSubscriber(ueId string) (*models.Subscriber, error) {
	filter := bson.M{"ueId": ueId}
	subscriberDataInterface, err := db.CommonDBClient.RestfulAPIGetOne(db.SubscribersColl, filter)
	if err != nil {
		return nil, err
	}
	var subscriberData *models.Subscriber
	json.Unmarshal(mapToByte(subscriberDataInterface), &subscriberData)
	return subscriberData, nil
}

func ListSubscribers() ([]*models.Subscriber, error) {
	subscribers := make([]*models.Subscriber, 0)
	subscribersData, err := db.CommonDBClient.RestfulAPIGetMany(db.SubscribersColl, bson.M{})
	if err != nil {
		return nil, err
	}
	for _, subscriberData := range subscribersData {
		subscriber := &models.Subscriber{}
		json.Unmarshal(mapToByte(subscriberData), &subscriber)
		subscribers = append(subscribers, subscriber)
	}
	return subscribers, nil
}

func CreateSubscriberAmPolicyData(ueId string) error {
	subscriber, err := GetSubscriber(ueId)
	if err != nil {
		return fmt.Errorf("couldn't get subscriber %s: %v", ueId, err)
	}
	if subscriber == nil {
		return fmt.Errorf("subscriber %s not found", ueId)
	}
	var amPolicy models.AmPolicyData
	amPolicy.SubscCats = append(amPolicy.SubscCats, "free5gc")
	subscriber.AmPolicyData = amPolicy
	subscriberBson := toBsonM(subscriber)
	filter := bson.M{"ueId": ueId}
	_, err = db.CommonDBClient.RestfulAPIPost(db.SubscribersColl, filter, subscriberBson)
	if err != nil {
		return fmt.Errorf("couldn't create am policy data for subscriber %s: %v", ueId, err)
	}
	logger.DBLog.Infof("Created AM policy data for subscriber %s", ueId)
	return nil
}

func CreateSubscriberAmData(ueId string, amData *models.AccessAndMobilitySubscriptionData) error {
	subscriber, err := GetSubscriber(ueId)
	if err != nil {
		return fmt.Errorf("couldn't get subscriber %s: %v", ueId, err)
	}
	if subscriber == nil {
		return fmt.Errorf("subscriber %s not found", ueId)
	}
	subscriber.AccessAndMobilitySubscriptionData = *amData
	subscriberBson := toBsonM(subscriber)
	filter := bson.M{"ueId": ueId}
	_, err = db.CommonDBClient.RestfulAPIPost(db.SubscribersColl, filter, subscriberBson)
	if err != nil {
		return fmt.Errorf("couldn't create am data for subscriber %s: %v", ueId, err)
	}
	logger.DBLog.Infof("Created AM data for subscriber %s", ueId)
	return nil
}

func CreateSubscriberSmPolicyData(ueId string, smPolicyData *models.SmPolicyData) error {
	subscriber, err := GetSubscriber(ueId)
	if err != nil {
		return fmt.Errorf("couldn't get subscriber %s: %v", ueId, err)
	}
	if subscriber == nil {
		return fmt.Errorf("subscriber %s not found", ueId)
	}
	subscriber.SmPolicyData = *smPolicyData
	subscriberBson := toBsonM(subscriber)
	filter := bson.M{"ueId": ueId}
	_, err = db.CommonDBClient.RestfulAPIPost(db.SubscribersColl, filter, subscriberBson)
	if err != nil {
		return err
	}
	logger.DBLog.Infof("Created Subscriber SmPolicyData for ueId %s", ueId)
	return nil
}

func CreateSubscriberAuthenticationSubscription(ueId string, authSubsData *models.AuthenticationSubscription) error {
	subscriber, err := GetSubscriber(ueId)
	if err != nil {
		return fmt.Errorf("couldn't get subscriber %s: %v", ueId, err)
	}
	if subscriber == nil {
		return fmt.Errorf("subscriber %s not found", ueId)
	}
	subscriber.AuthenticationSubscription = *authSubsData
	subscriberBson := toBsonM(subscriber)
	filter := bson.M{"ueId": ueId}
	_, err = db.CommonDBClient.RestfulAPIPost(db.SubscribersColl, filter, subscriberBson)
	if err != nil {
		return err
	}
	return nil
}

func CreateSubscriberSmData(ueId string, smData *models.SessionManagementSubscriptionData) error {
	subscriber, err := GetSubscriber(ueId)
	if err != nil {
		return fmt.Errorf("couldn't get subscriber %s: %v", ueId, err)
	}
	if subscriber == nil {
		return fmt.Errorf("subscriber %s not found", ueId)
	}
	subscriber.SessionManagementSubscriptionData = append(subscriber.SessionManagementSubscriptionData, smData)
	subscriberBson := toBsonM(subscriber)
	filter := bson.M{"ueId": ueId}
	_, err = db.CommonDBClient.RestfulAPIPost(db.SubscribersColl, filter, subscriberBson)
	if err != nil {
		return err
	}
	return nil
}

func CreateSubscriberSmfSelectionData(ueId string, smfSelData *models.SmfSelectionSubscriptionData) error {
	subscriber, err := GetSubscriber(ueId)
	if err != nil {
		return fmt.Errorf("couldn't get subscriber %s: %v", ueId, err)
	}
	if subscriber == nil {
		return fmt.Errorf("subscriber %s not found", ueId)
	}
	subscriber.SmfSelectionSubscriptionData = *smfSelData
	subscriberBson := toBsonM(subscriber)
	filter := bson.M{"ueId": ueId}
	_, err = db.CommonDBClient.RestfulAPIPost(db.SubscribersColl, filter, subscriberBson)
	if err != nil {
		return err
	}
	return nil
}

func GetSubscriberAmData(ueId string) (*models.AccessAndMobilitySubscriptionData, error) {
	subscriber, err := GetSubscriber(ueId)
	if err != nil {
		return nil, err
	}
	if subscriber == nil {
		return nil, fmt.Errorf("subscriber %s not found", ueId)
	}
	return &subscriber.AccessAndMobilitySubscriptionData, nil
}

func DeleteSubscriberAmData(imsi string) error {
	subscriber, err := GetSubscriber("imsi-" + imsi)
	if err != nil {
		return fmt.Errorf("couldn't get subscriber %s: %v", imsi, err)
	}
	if subscriber == nil {
		return fmt.Errorf("subscriber %s not found", imsi)
	}
	subscriber.AccessAndMobilitySubscriptionData = models.AccessAndMobilitySubscriptionData{}
	subscriberBson := toBsonM(subscriber)
	filter := bson.M{"ueId": "imsi-" + imsi}
	_, err = db.CommonDBClient.RestfulAPIPost(db.SubscribersColl, filter, subscriberBson)
	if err != nil {
		return fmt.Errorf("couldn't delete am data for subscriber %s: %v", imsi, err)
	}
	logger.DBLog.Infof("Deleted AM data for subscriber %s", imsi)
	return nil
}

func ListSubscribersAmData() ([]*models.AccessAndMobilitySubscriptionData, error) {
	subscribers, err := ListSubscribers()
	if err != nil {
		return nil, err
	}
	amDataList := make([]*models.AccessAndMobilitySubscriptionData, 0)
	for _, subscriber := range subscribers {
		amDataList = append(amDataList, &subscriber.AccessAndMobilitySubscriptionData)
	}
	return amDataList, nil
}

func GetSubscriberAmPolicyData(ueId string) (*models.AmPolicyData, error) {
	subscriber, err := GetSubscriber(ueId)
	if err != nil {
		return nil, err
	}
	if subscriber == nil {
		return nil, fmt.Errorf("subscriber %s not found", ueId)
	}
	return &subscriber.AmPolicyData, nil
}

func DeleteAmPolicy(imsi string) error {
	subscriber, err := GetSubscriber("imsi-" + imsi)
	if err != nil {
		return fmt.Errorf("couldn't get subscriber %s: %v", imsi, err)
	}
	if subscriber == nil {
		return fmt.Errorf("subscriber %s not found", imsi)
	}
	subscriber.AmPolicyData = models.AmPolicyData{}
	subscriberBson := toBsonM(subscriber)
	filter := bson.M{"ueId": "imsi-" + imsi}
	_, err = db.CommonDBClient.RestfulAPIPost(db.SubscribersColl, filter, subscriberBson)
	if err != nil {
		return fmt.Errorf("couldn't delete am policy data for subscriber %s: %v", imsi, err)
	}
	logger.DBLog.Infof("Deleted AM policy data for subscriber %s", imsi)
	return nil
}

func ListSmData(ueId string) ([]*models.SessionManagementSubscriptionData, error) {
	subscriber, err := GetSubscriber(ueId)
	if err != nil {
		return nil, err
	}
	if subscriber == nil {
		return nil, fmt.Errorf("subscriber %s not found", ueId)
	}
	return subscriber.SessionManagementSubscriptionData, nil
}

func DeleteSmData(imsi string) error {
	subscriber, err := GetSubscriber("imsi-" + imsi)
	if err != nil {
		return fmt.Errorf("couldn't get subscriber %s: %v", imsi, err)
	}
	if subscriber == nil {
		return fmt.Errorf("subscriber %s not found", imsi)
	}
	subscriber.SessionManagementSubscriptionData = nil
	subscriberBson := toBsonM(subscriber)
	filter := bson.M{"ueId": "imsi-" + imsi}
	_, err = db.CommonDBClient.RestfulAPIPost(db.SubscribersColl, filter, subscriberBson)
	if err != nil {
		return fmt.Errorf("couldn't delete sm data for subscriber %s: %v", imsi, err)
	}
	logger.DBLog.Infof("Deleted SM data for subscriber %s", imsi)
	return nil
}

func GetSmPolicyData(ueId string) (*models.SmPolicyData, error) {
	subscriber, err := GetSubscriber(ueId)
	if err != nil {
		return nil, err
	}
	if subscriber == nil {
		return nil, fmt.Errorf("subscriber %s not found", ueId)
	}
	return &subscriber.SmPolicyData, nil
}

func DeleteSmPolicy(imsi string) error {
	subscriber, err := GetSubscriber("imsi-" + imsi)
	if err != nil {
		return fmt.Errorf("couldn't get subscriber %s: %v", imsi, err)
	}
	if subscriber == nil {
		return fmt.Errorf("subscriber %s not found", imsi)
	}
	subscriber.SmPolicyData = models.SmPolicyData{}
	subscriberBson := toBsonM(subscriber)
	filter := bson.M{"ueId": "imsi-" + imsi}
	_, err = db.CommonDBClient.RestfulAPIPost(db.SubscribersColl, filter, subscriberBson)
	if err != nil {
		return fmt.Errorf("couldn't delete sm policy data for subscriber %s: %v", imsi, err)
	}
	logger.DBLog.Infof("Deleted SM policy data for subscriber %s", imsi)
	return nil
}

func GetAuthenticationSubscription(ueId string) (*models.AuthenticationSubscription, error) {
	subscriber, err := GetSubscriber(ueId)
	if err != nil {
		return nil, err
	}
	if subscriber == nil {
		return nil, fmt.Errorf("subscriber %s not found", ueId)
	}
	return &subscriber.AuthenticationSubscription, nil
}

func DeleteAuthenticationSubscription(ueId string) error {
	subscriber, err := GetSubscriber(ueId)
	if err != nil {
		return fmt.Errorf("couldn't get subscriber %s: %v", ueId, err)
	}
	if subscriber == nil {
		return fmt.Errorf("subscriber %s not found", ueId)
	}
	subscriber.AuthenticationSubscription = models.AuthenticationSubscription{}
	subscriberBson := toBsonM(subscriber)
	filter := bson.M{"ueId": ueId}
	_, err = db.CommonDBClient.RestfulAPIPost(db.SubscribersColl, filter, subscriberBson)
	if err != nil {
		return fmt.Errorf("couldn't delete authentication subscription for subscriber %s: %v", ueId, err)
	}
	logger.DBLog.Infof("Deleted Authentication Subscription for subscriber %s", ueId)
	return nil
}

func PatchAuthenticationSubscription(ueId string, patchItem []models.PatchItem) error {
	subscriber, err := GetSubscriber(ueId)
	if err != nil {
		return fmt.Errorf("couldn't get subscriber %s: %v", ueId, err)
	}
	if subscriber == nil {
		return fmt.Errorf("subscriber %s not found", ueId)
	}
	patchJSON, err := json.Marshal(patchItem)
	if err != nil {
		return err
	}
	filter := bson.M{"ueId": ueId}
	err = db.CommonDBClient.RestfulAPIJSONPatch(db.SubscribersColl, filter, patchJSON)
	if err != nil {
		return fmt.Errorf("couldn't patch authentication subscription for subscriber %s: %v", ueId, err)
	}
	return nil
}

// func PatchAuthenticationSubscription(ueId string, patchItem []models.PatchItem) error {
// 	patchJSON, err := json.Marshal(patchItem)
// 	if err != nil {
// 		return err
// 	}
// 	filter := bson.M{"ueId": ueId}
// 	err = db.CommonDBClient.RestfulAPIJSONPatch(db.AuthSubsDataColl, filter, patchJSON)
// 	return err
// }

func GetSmfSelectionSubscriptionData(ueId string) (*models.SmfSelectionSubscriptionData, error) {
	subscriber, err := GetSubscriber(ueId)
	if err != nil {
		return nil, err
	}
	if subscriber == nil {
		return nil, fmt.Errorf("subscriber %s not found", ueId)
	}
	return &subscriber.SmfSelectionSubscriptionData, nil
}

func DeleteSmfSelection(imsi string) error {
	subscriber, err := GetSubscriber("imsi-" + imsi)
	if err != nil {
		return fmt.Errorf("couldn't get subscriber %s: %v", imsi, err)
	}
	if subscriber == nil {
		return fmt.Errorf("subscriber %s not found", imsi)
	}
	subscriber.SmfSelectionSubscriptionData = models.SmfSelectionSubscriptionData{}
	subscriberBson := toBsonM(subscriber)
	filter := bson.M{"ueId": "imsi-" + imsi}
	_, err = db.CommonDBClient.RestfulAPIPost(db.SubscribersColl, filter, subscriberBson)
	if err != nil {
		return fmt.Errorf("couldn't delete smf selection data for subscriber %s: %v", imsi, err)
	}
	logger.DBLog.Infof("Deleted SMF selection data for subscriber %s", imsi)
	return nil
}
