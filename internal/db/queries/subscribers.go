package queries

import (
	"encoding/json"

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

func DeleteSubscriber(ueId string) error {
	filter := bson.M{"ueId": ueId}
	err := db.CommonDBClient.RestfulAPIDeleteOne(db.SubscribersColl, filter)
	if err != nil {
		return err
	}
	logger.DBLog.Infof("Deleted Subscriber: %v", ueId)
	return nil
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

// All the methods below should be removed in favor of the more generic
// Get, List, Create, Delete methods above

// func PatchAuthenticationSubscription(ueId string, patchItem []models.PatchItem) error {
// 	subscriber, err := GetSubscriber(ueId)
// 	if err != nil {
// 		return fmt.Errorf("couldn't get subscriber %s: %v", ueId, err)
// 	}
// 	if subscriber == nil {
// 		return fmt.Errorf("subscriber %s not found", ueId)
// 	}
// 	patchJSON, err := json.Marshal(patchItem)
// 	if err != nil {
// 		return err
// 	}
// 	filter := bson.M{"ueId": ueId}
// 	err = db.CommonDBClient.RestfulAPIJSONPatch(db.SubscribersColl, filter, patchJSON)
// 	if err != nil {
// 		return fmt.Errorf("couldn't patch authentication subscription for subscriber %s: %v", ueId, err)
// 	}
// 	return nil
// }
