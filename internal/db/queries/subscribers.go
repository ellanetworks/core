package queries

import (
	"encoding/json"

	"github.com/yeastengine/ella/internal/db"
	"github.com/yeastengine/ella/internal/db/models"
	"go.mongodb.org/mongo-driver/bson"
)

func CreateSubscriber(subscriber *models.Subscriber) error {
	filter := bson.M{"imsi": subscriber.IMSI}
	subscriberDataBson := toBsonM(&subscriber)
	_, err := db.CommonDBClient.RestfulAPIPost(db.SubscribersColl, filter, subscriberDataBson)
	if err != nil {
		return err
	}
	return nil
}

func DeleteSubscriber(imsi string) error {
	filter := bson.M{"imsi": imsi}
	err := db.CommonDBClient.RestfulAPIDeleteOne(db.SubscribersColl, filter)
	if err != nil {
		return err
	}
	return nil
}

func GetSubscriberByImsi(imsi string) (*models.Subscriber, error) {
	var subscriber *models.Subscriber
	filter := bson.M{"imsi": imsi}
	rawSubscriber, err := db.CommonDBClient.RestfulAPIGetOne(db.SubscribersColl, filter)
	if err != nil {
		return nil, err
	}
	json.Unmarshal(mapToByte(rawSubscriber), &subscriber)
	return subscriber, nil
}
