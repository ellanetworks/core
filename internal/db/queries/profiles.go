package queries

import (
	"encoding/json"

	"github.com/yeastengine/ella/internal/db"
	"github.com/yeastengine/ella/internal/db/models"
	"github.com/yeastengine/ella/internal/logger"
	"go.mongodb.org/mongo-driver/bson"
)

func ListProfiles() ([]string, error) {
	var profiles []string = make([]string, 0)
	rawProfiles, errGetMany := db.CommonDBClient.RestfulAPIGetMany(db.ProfilesColl, bson.M{})
	if errGetMany != nil {
		return nil, errGetMany
	}
	for _, rawProfile := range rawProfiles {
		groupName, err := rawProfile["name"].(string)
		if !err {
			logger.DBLog.Warnf("Could not get profile name from %v", rawProfile)
			continue
		}
		profiles = append(profiles, groupName)
	}
	return profiles, nil
}

func GetProfile(name string) *models.Profile {
	var profile *models.Profile
	filter := bson.M{"name": name}
	rawDeviceGroup, err := db.CommonDBClient.RestfulAPIGetOne(db.ProfilesColl, filter)
	if err != nil {
		logger.DBLog.Warnln(err)
		return nil
	}
	json.Unmarshal(mapToByte(rawDeviceGroup), &profile)
	return profile
}

func DeleteProfile(name string) error {
	filter := bson.M{"name": name}
	err := db.CommonDBClient.RestfulAPIDeleteOne(db.ProfilesColl, filter)
	if err != nil {
		return err
	}
	return nil
}

func CreateProfile(profile *models.Profile) error {
	filter := bson.M{"name": profile.Name}
	profileData := toBsonM(&profile)
	_, err := db.CommonDBClient.RestfulAPIPost(db.ProfilesColl, filter, profileData)
	if err != nil {
		return err
	}
	logger.DBLog.Infof("Created Profile: %v", profile.Name)
	return nil
}
