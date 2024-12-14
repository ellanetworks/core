package db

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"os"
	"strings"
	"time"

	jsonpatch "github.com/evanphx/json-patch"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type MongoClient struct {
	Client *mongo.Client
	dbName string
	url    string
	pools  map[string]map[string]int32
}

func NewMongoClient(url string, dbName string) (*MongoClient, error) {
	c := MongoClient{url: url, dbName: dbName}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	client, err := mongo.Connect(ctx, options.Client().ApplyURI(c.url))
	if err != nil {
		return nil, fmt.Errorf("MongoClient Creation err: %+v", err)
	}
	c.Client = client
	return &c, nil
}

func findOneAndDecode(collection *mongo.Collection, filter bson.M) (map[string]interface{}, error) {
	var result map[string]interface{}
	if err := collection.FindOne(context.TODO(), filter).Decode(&result); err != nil {
		// ErrNoDocuments means that the filter did not match any documents in
		// the collection.
		if err == mongo.ErrNoDocuments {
			return nil, nil
		}
		return nil, err
	}
	return result, nil
}

func getOrigData(collection *mongo.Collection, filter bson.M) (map[string]interface{}, error) {
	result, err := findOneAndDecode(collection, filter)
	if err != nil {
		return nil, err
	}
	if result != nil {
		// Delete "_id" entry which is auto-inserted by MongoDB
		delete(result, "_id")
	}
	return result, nil
}

func checkDataExisted(collection *mongo.Collection, filter bson.M) (bool, error) {
	result, err := findOneAndDecode(collection, filter)
	if err != nil {
		return false, err
	}
	if result == nil {
		return false, nil
	}
	return true, nil
}

func (c *MongoClient) GetCollection(collName string) *mongo.Collection {
	collection := c.Client.Database(c.dbName).Collection(collName)
	return collection
}

func (c *MongoClient) RestfulAPIGetOne(collName string, filter bson.M) (map[string]interface{}, error) {
	collection := c.Client.Database(c.dbName).Collection(collName)
	result, err := getOrigData(collection, filter)
	if err != nil {
		return nil, fmt.Errorf("RestfulAPIGetOne err: %+v", err)
	}
	return result, nil
}

func (c *MongoClient) RestfulAPIGetMany(collName string, filter bson.M) ([]map[string]interface{}, error) {
	collection := c.Client.Database(c.dbName).Collection(collName)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cur, err := collection.Find(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("RestfulAPIGetMany err: %+v", err)
	}
	defer func(ctx context.Context) {
		if err := cur.Close(ctx); err != nil {
			return
		}
	}(ctx)

	var resultArray []map[string]interface{}
	for cur.Next(ctx) {
		var result map[string]interface{}
		if err := cur.Decode(&result); err != nil {
			return nil, fmt.Errorf("RestfulAPIGetMany err: %+v", err)
		}

		// Delete "_id" entry which is auto-inserted by MongoDB
		delete(result, "_id")
		resultArray = append(resultArray, result)
	}
	if err := cur.Err(); err != nil {
		return nil, fmt.Errorf("RestfulAPIGetMany err: %+v", err)
	}

	return resultArray, nil
}

// if no error happened, return true means data existed and false means data not existed
func (c *MongoClient) RestfulAPIPutOne(collName string, filter bson.M, putData map[string]interface{}) (bool, error) {
	collection := c.Client.Database(c.dbName).Collection(collName)
	existed, err := checkDataExisted(collection, filter)
	if err != nil {
		return false, fmt.Errorf("RestfulAPIPutOne err: %+v", err)
	}

	if existed {
		if _, err := collection.UpdateOne(context.TODO(), filter, bson.M{"$set": putData}); err != nil {
			return false, fmt.Errorf("RestfulAPIPutOne UpdateOne err: %+v", err)
		}
		return true, nil
	}

	if _, err := collection.InsertOne(context.TODO(), putData); err != nil {
		return false, fmt.Errorf("RestfulAPIPutOne InsertOne err: %+v", err)
	}
	return false, nil
}

func (c *MongoClient) RestfulAPIPullOne(collName string, filter bson.M, putData map[string]interface{}) error {
	collection := c.Client.Database(c.dbName).Collection(collName)
	if _, err := collection.UpdateOne(context.TODO(), filter, bson.M{"$pull": putData}); err != nil {
		return fmt.Errorf("RestfulAPIPullOne err: %+v", err)
	}
	return nil
}

// if no error happened, return true means data existed (not updated) and false means data not existed
func (c *MongoClient) RestfulAPIPutOneNotUpdate(collName string, filter bson.M, putData map[string]interface{}) (bool, error) {
	collection := c.Client.Database(c.dbName).Collection(collName)
	existed, err := checkDataExisted(collection, filter)
	if err != nil {
		return false, fmt.Errorf("RestfulAPIPutOneNotUpdate err: %+v", err)
	}

	if existed {
		return true, nil
	}

	if _, err := collection.InsertOne(context.TODO(), putData); err != nil {
		return false, fmt.Errorf("RestfulAPIPutOneNotUpdate InsertOne err: %+v", err)
	}
	return false, nil
}

func (c *MongoClient) RestfulAPIPutMany(collName string, filterArray []bson.M, putDataArray []map[string]interface{}) error {
	collection := c.Client.Database(c.dbName).Collection(collName)

	for i, putData := range putDataArray {
		filter := filterArray[i]
		existed, err := checkDataExisted(collection, filter)
		if err != nil {
			return fmt.Errorf("RestfulAPIPutMany err: %+v", err)
		}

		if existed {
			if _, err := collection.UpdateOne(context.TODO(), filter, bson.M{"$set": putData}); err != nil {
				return fmt.Errorf("RestfulAPIPutMany UpdateOne err: %+v", err)
			}
		} else {
			if _, err := collection.InsertOne(context.TODO(), putData); err != nil {
				return fmt.Errorf("RestfulAPIPutMany InsertOne err: %+v", err)
			}
		}
	}
	return nil
}

func (c *MongoClient) RestfulAPIDeleteOne(collName string, filter bson.M) error {
	collection := c.Client.Database(c.dbName).Collection(collName)

	if _, err := collection.DeleteOne(context.TODO(), filter); err != nil {
		return fmt.Errorf("RestfulAPIDeleteOne err: %+v", err)
	}
	return nil
}

func (c *MongoClient) RestfulAPIDeleteMany(collName string, filter bson.M) error {
	collection := c.Client.Database(c.dbName).Collection(collName)

	if _, err := collection.DeleteMany(context.TODO(), filter); err != nil {
		return fmt.Errorf("RestfulAPIDeleteMany err: %+v", err)
	}
	return nil
}

func (c *MongoClient) RestfulAPIMergePatch(collName string, filter bson.M, patchData map[string]interface{}) error {
	collection := c.Client.Database(c.dbName).Collection(collName)

	originalData, err := getOrigData(collection, filter)
	if err != nil {
		return fmt.Errorf("RestfulAPIMergePatch getOrigData err: %+v", err)
	}

	original, err := json.Marshal(originalData)
	if err != nil {
		return fmt.Errorf("RestfulAPIMergePatch Marshal err: %+v", err)
	}

	patchDataByte, err := json.Marshal(patchData)
	if err != nil {
		return fmt.Errorf("RestfulAPIMergePatch Marshal err: %+v", err)
	}

	modifiedAlternative, err := jsonpatch.MergePatch(original, patchDataByte)
	if err != nil {
		return fmt.Errorf("RestfulAPIMergePatch MergePatch err: %+v", err)
	}

	var modifiedData map[string]interface{}
	if err := json.Unmarshal(modifiedAlternative, &modifiedData); err != nil {
		return fmt.Errorf("RestfulAPIMergePatch Unmarshal err: %+v", err)
	}
	if _, err := collection.UpdateOne(context.TODO(), filter, bson.M{"$set": modifiedData}); err != nil {
		return fmt.Errorf("RestfulAPIMergePatch UpdateOne err: %+v", err)
	}
	return nil
}

func (c *MongoClient) RestfulAPIJSONPatch(collName string, filter bson.M, patchJSON []byte) error {
	collection := c.Client.Database(c.dbName).Collection(collName)

	originalData, err := getOrigData(collection, filter)
	if err != nil {
		return fmt.Errorf("RestfulAPIJSONPatch getOrigData err: %+v", err)
	}

	original, err := json.Marshal(originalData)
	if err != nil {
		return fmt.Errorf("RestfulAPIJSONPatch Marshal err: %+v", err)
	}

	patch, err := jsonpatch.DecodePatch(patchJSON)
	if err != nil {
		return fmt.Errorf("RestfulAPIJSONPatch DecodePatch err: %+v", err)
	}

	modified, err := patch.Apply(original)
	if err != nil {
		return fmt.Errorf("RestfulAPIJSONPatch Apply err: %+v", err)
	}

	var modifiedData map[string]interface{}
	if err := json.Unmarshal(modified, &modifiedData); err != nil {
		return fmt.Errorf("RestfulAPIJSONPatch Unmarshal err: %+v", err)
	}
	if _, err := collection.UpdateOne(context.TODO(), filter, bson.M{"$set": modifiedData}); err != nil {
		return fmt.Errorf("RestfulAPIJSONPatch UpdateOne err: %+v", err)
	}
	return nil
}

func (c *MongoClient) RestfulAPIJSONPatchExtend(collName string, filter bson.M, patchJSON []byte, dataName string) error {
	collection := c.Client.Database(c.dbName).Collection(collName)

	originalDataCover, err := getOrigData(collection, filter)
	if err != nil {
		return fmt.Errorf("RestfulAPIJSONPatchExtend getOrigData err: %+v", err)
	}

	originalData := originalDataCover[dataName]
	original, err := json.Marshal(originalData)
	if err != nil {
		return fmt.Errorf("RestfulAPIJSONPatchExtend Marshal err: %+v", err)
	}

	patch, err := jsonpatch.DecodePatch(patchJSON)
	if err != nil {
		return fmt.Errorf("RestfulAPIJSONPatchExtend DecodePatch err: %+v", err)
	}

	modified, err := patch.Apply(original)
	if err != nil {
		return fmt.Errorf("RestfulAPIJSONPatchExtend Apply err: %+v", err)
	}

	var modifiedData map[string]interface{}
	if err := json.Unmarshal(modified, &modifiedData); err != nil {
		return fmt.Errorf("RestfulAPIJSONPatchExtend Unmarshal err: %+v", err)
	}
	if _, err := collection.UpdateOne(context.TODO(), filter, bson.M{"$set": bson.M{dataName: modifiedData}}); err != nil {
		return fmt.Errorf("RestfulAPIJSONPatchExtend UpdateOne err: %+v", err)
	}
	return nil
}

func (c *MongoClient) RestfulAPIPost(collName string, filter bson.M, postData map[string]interface{}) (bool, error) {
	return c.RestfulAPIPutOne(collName, filter, postData)
}

func (c *MongoClient) RestfulAPIPostMany(collName string, filter bson.M, postDataArray []interface{}) error {
	collection := c.Client.Database(c.dbName).Collection(collName)

	if _, err := collection.InsertMany(context.TODO(), postDataArray); err != nil {
		return fmt.Errorf("RestfulAPIPostMany err: %+v", err)
	}
	return nil
}

func (c *MongoClient) RestfulAPICount(collName string, filter bson.M) (int64, error) {
	collection := c.Client.Database(c.dbName).Collection(collName)
	result, err := collection.CountDocuments(context.TODO(), filter)
	if err != nil {
		return 0, fmt.Errorf("RestfulAPICount err: %+v", err)
	}
	return result, nil
}

func (c *MongoClient) Drop(collName string) error {
	collection := c.Client.Database(c.dbName).Collection(collName)
	return collection.Drop(context.TODO())
}

/* Get unique identity from counter collection. */
func (c *MongoClient) GetUniqueIdentity(idName string) int32 {
	counterCollection := c.Client.Database(c.dbName).Collection("counter")

	counterFilter := bson.M{}
	counterFilter["_id"] = idName

	for {
		count := counterCollection.FindOneAndUpdate(context.TODO(), counterFilter, bson.M{"$inc": bson.M{"count": 1}})

		if count.Err() != nil {
			// logger.MongoDBLog.Println("FindOneAndUpdate error. Create entry for field  ")
			counterData := bson.M{}
			counterData["count"] = 1
			counterData["_id"] = idName
			counterCollection.InsertOne(context.TODO(), counterData)

			continue
		} else {
			// logger.MongoDBLog.Println("found entry. inc and return")
			data := bson.M{}
			count.Decode(&data)
			decodedCount := data["count"].(int32)
			return decodedCount
		}
	}
}

/* Get a unique id within a given range. */
func (c *MongoClient) GetUniqueIdentityWithinRange(pool string, minimum int32, maximum int32) int32 {
	rangeCollection := c.Client.Database(c.dbName).Collection("range")

	rangeFilter := bson.M{}
	rangeFilter["_id"] = pool

	for {
		count := rangeCollection.FindOneAndUpdate(context.TODO(), rangeFilter, bson.M{"$inc": bson.M{"count": 1}})

		if count.Err() != nil {
			counterData := bson.M{}
			counterData["count"] = minimum
			counterData["_id"] = pool
			rangeCollection.InsertOne(context.TODO(), counterData)

			continue
		} else {
			data := bson.M{}
			count.Decode(&data)
			decodedCount := data["count"].(int32)

			if decodedCount >= maximum || decodedCount <= minimum {
				// err := errors.New("Unique identity is out of range.")
				// logger.MongoDBLog.Println(err)
				return -1
			}
			return decodedCount
		}
	}
}

/* Initialize pool of ids with maximum and minimum values and chunk size and amount of retries to get a chunk. */
func (c *MongoClient) InitializeChunkPool(poolName string, minimum int32, maximum int32, retries int32, chunkSize int32) {
	// logger.MongoDBLog.Println("ENTERING InitializeChunkPool")
	poolData := map[string]int32{}
	poolData["min"] = minimum
	poolData["max"] = maximum
	poolData["retries"] = retries
	poolData["chunkSize"] = chunkSize

	c.pools[poolName] = poolData
	// logger.MongoDBLog.Println("Pools: ", pools)
}

/* Get id by inserting into collection. If insert succeeds, that id is available. Else, it isn't available so retry. */
func (c *MongoClient) GetChunkFromPool(poolName string) (int32, int32, int32, error) {
	// logger.MongoDBLog.Println("ENTERING GetChunkFromPool")

	pool := c.pools[poolName]

	if pool == nil {
		err := errors.New("this pool has not been initialized yet. Initialize by calling InitializeChunkPool")
		return -1, -1, -1, err
	}

	minimum := pool["min"]
	maximum := pool["max"]
	retries := pool["retries"]
	chunkSize := pool["chunkSize"]
	totalChunks := (maximum - minimum) / chunkSize

	var i int32 = 0
	for i < retries {
		random := rand.Int31n(totalChunks)
		lower := minimum + (random * chunkSize)
		upper := lower + chunkSize
		poolCollection := c.Client.Database(c.dbName).Collection(poolName)

		// Create an instance of an options and set the desired options
		upsert := true
		opt := options.FindOneAndUpdateOptions{
			Upsert: &upsert,
		}
		data := bson.M{}
		data["_id"] = random
		data["lower"] = lower
		data["upper"] = upper
		data["owner"] = os.Getenv("HOSTNAME")
		result := poolCollection.FindOneAndUpdate(context.TODO(), bson.M{"_id": random}, bson.M{"$setOnInsert": data}, &opt)

		if result.Err() != nil {
			// means that there was no document with that id, so the upsert should have been successful
			if result.Err() == mongo.ErrNoDocuments {
				// logger.MongoDBLog.Println("Assigned chunk # ", random, " with range ", lower, " - ", upper)
				return random, lower, upper, nil
			}

			return -1, -1, -1, result.Err()
		}
		// means there was a document before the update and result contains that document.
		// logger.MongoDBLog.Println("Chunk", random, " has already been assigned. ", retries-i-1, " retries left.")
		i++
	}

	err := errors.New("no id found after retries")
	return -1, -1, -1, err
}

/* Release the provided id to the provided pool. */
func (c *MongoClient) ReleaseChunkToPool(poolName string, id int32) {
	// logger.MongoDBLog.Println("ENTERING ReleaseChunkToPool")
	poolCollection := c.Client.Database(c.dbName).Collection(poolName)

	// only want to delete if the currentApp is the owner of this id.
	currentApp := os.Getenv("HOSTNAME")
	// logger.MongoDBLog.Println(currentApp)

	_, err := poolCollection.DeleteOne(context.TODO(), bson.M{"_id": id, "owner": currentApp})
	if err != nil {
		// logger.MongoDBLog.Println("Release Chunk(", id, ") to Pool(", poolName, ") failed : ", err)
	}
}

/* Initialize pool of ids with maximum and minimum values. */
func (c *MongoClient) InitializeInsertPool(poolName string, minimum int32, maximum int32, retries int32) {
	// logger.MongoDBLog.Println("ENTERING InitializeInsertPool")
	poolData := map[string]int32{}
	poolData["min"] = minimum
	poolData["max"] = maximum
	poolData["retries"] = retries

	c.pools[poolName] = poolData
	// logger.MongoDBLog.Println("Pools: ", pools)
}

/* Get id by inserting into collection. If insert succeeds, that id is available. Else, it isn't available so retry. */
func (c *MongoClient) GetIDFromInsertPool(poolName string) (int32, error) {
	// logger.MongoDBLog.Println("ENTERING GetIDFromInsertPool")

	pool := c.pools[poolName]

	if pool == nil {
		err := errors.New("this pool has not been initialized yet. Initialize by calling InitializeInsertPool")
		return -1, err
	}

	minimum := pool["min"]
	maximum := pool["max"]
	retries := pool["retries"]
	var i int32 = 0
	for i < retries {
		random := rand.Int31n(maximum-minimum) + minimum // returns random int in [0, maximum-minimum-1] + minimum
		poolCollection := c.Client.Database(c.dbName).Collection(poolName)

		// Create an instance of an options and set the desired options
		upsert := true
		opt := options.FindOneAndUpdateOptions{
			Upsert: &upsert,
		}
		result := poolCollection.FindOneAndUpdate(context.TODO(), bson.M{"_id": random}, bson.M{"$set": bson.M{"_id": random}}, &opt)

		if result.Err() != nil {
			// means that there was no document with that id, so the upsert should have been successful
			if result.Err().Error() == "mongo: no documents in result" {
				// logger.MongoDBLog.Println("Assigned id: ", random)
				return random, nil
			}

			return -1, result.Err()
		}
		// means there was a document before the update and result contains that document.
		// logger.MongoDBLog.Println("This id has already been assigned. ")
		doc := bson.M{}
		result.Decode(&doc)
		// logger.MongoDBLog.Println(doc)

		i++
	}

	err := errors.New("no id found after retries")
	return -1, err
}

/* Release the provided id to the provided pool. */
func (c *MongoClient) ReleaseIDToInsertPool(poolName string, id int32) {
	// logger.MongoDBLog.Println("ENTERING ReleaseIDToInsertPool")
	poolCollection := c.Client.Database(c.dbName).Collection(poolName)

	_, err := poolCollection.DeleteOne(context.TODO(), bson.M{"_id": id})
	if err != nil {
		// logger.MongoDBLog.Println("Release Id(", id, ") to Pool(", poolName, ") failed : ", err)
	}
}

/* Initialize pool of ids with maximum and minimum values. */
func (c *MongoClient) InitializePool(poolName string, minimum int32, maximum int32) {
	// logger.MongoDBLog.Println("ENTERING InitializePool")
	poolCollection := c.Client.Database(c.dbName).Collection(poolName)
	names, err := c.Client.Database(c.dbName).ListCollectionNames(context.TODO(), bson.M{})
	if err != nil {
		// logger.MongoDBLog.Println(err)
		return
	}

	// logger.MongoDBLog.Println(names)

	exists := false
	for _, name := range names {
		if name == poolName {
			// logger.MongoDBLog.Println("The collection exists!")
			exists = true
			break
		}
	}
	if !exists {
		// logger.MongoDBLog.Println("Creating collection")

		array := []int32{}
		for i := minimum; i < maximum; i++ {
			array = append(array, i)
		}
		poolData := bson.M{}
		poolData["ids"] = array
		poolData["_id"] = poolName

		// collection is created when inserting document.
		// "If a collection does not exist, MongoDB creates the collection when you first store data for that collection."
		poolCollection.InsertOne(context.TODO(), poolData)
	}
}

/* For example IP addresses need to be assigned and then returned to be used again. */
func (c *MongoClient) GetIDFromPool(poolName string) (int32, error) {
	// logger.MongoDBLog.Println("ENTERING GetIDFromPool")
	poolCollection := c.Client.Database(c.dbName).Collection(poolName)

	result := bson.M{}
	poolCollection.FindOneAndUpdate(context.TODO(), bson.M{"_id": poolName}, bson.M{"$pop": bson.M{"ids": 1}}).Decode(&result)

	var array []int32
	interfaces := []interface{}(result["ids"].(primitive.A))
	for _, s := range interfaces {
		id := s.(int32)
		array = append(array, id)
	}

	// logger.MongoDBLog.Println("Array of ids: ", array)
	if len(array) > 0 {
		res := array[len(array)-1]
		return res, nil
	} else {
		err := errors.New("there are no available ids")
		// logger.MongoDBLog.Println(err)
		return -1, err
	}
}

/* Release the provided id to the provided pool. */
func (c *MongoClient) ReleaseIDToPool(poolName string, id int32) {
	// logger.MongoDBLog.Println("ENTERING ReleaseIDToPool")
	poolCollection := c.Client.Database(c.dbName).Collection(poolName)

	poolCollection.UpdateOne(context.TODO(), bson.M{"_id": poolName}, bson.M{"$push": bson.M{"ids": id}})
}

func (c *MongoClient) GetOneCustomDataStructure(collName string, filter bson.M) (bson.M, error) {
	collection := c.Client.Database(c.dbName).Collection(collName)

	val := collection.FindOne(context.TODO(), filter)

	if val.Err() != nil {
		// logger.MongoDBLog.Println("Error getting student from db: " + val.Err().Error())
		return bson.M{}, val.Err()
	}

	var result bson.M
	err := val.Decode(&result)
	return result, err
}

func (c *MongoClient) PutOneCustomDataStructure(collName string, filter bson.M, putData interface{}) (bool, error) {
	collection := c.Client.Database(c.dbName).Collection(collName)

	var checkItem map[string]interface{}
	collection.FindOne(context.TODO(), filter).Decode(&checkItem)

	if checkItem == nil {
		_, err := collection.InsertOne(context.TODO(), putData)
		if err != nil {
			// logger.MongoDBLog.Println("insert failed : ", err)
			return false, err
		}
		return true, nil
	} else {
		collection.UpdateOne(context.TODO(), filter, bson.M{"$set": putData})
		return true, nil
	}
}

func (c *MongoClient) CreateIndex(collName string, keyField string) (bool, error) {
	collection := c.Client.Database(c.dbName).Collection(collName)

	index := mongo.IndexModel{
		Keys:    bson.D{{Key: keyField, Value: 1}},
		Options: options.Index().SetUnique(true),
	}

	_, err := collection.Indexes().CreateOne(context.Background(), index)
	if err != nil {
		// logger.MongoDBLog.Error("Create Index failed : ", keyField, err)
		return false, err
	}

	// logger.MongoDBLog.Println("Created index : ", idx, " on keyField : ", keyField, " for Collection : ", collName)

	return true, nil
}

// To create Index with common timeout for all documents, set timeout to desired value
// To create Index with custom timeout per document, set timeout to 0.
// To create Index with common timeout use timefield name like : updatedAt
// To create Index with custom timeout use timefield name like : expireAt
func (c *MongoClient) RestfulAPICreateTTLIndex(collName string, timeout int32, timeField string) bool {
	collection := c.Client.Database(c.dbName).Collection(collName)
	index := mongo.IndexModel{
		Keys:    bson.D{{Key: timeField, Value: 1}},
		Options: options.Index().SetExpireAfterSeconds(timeout).SetName(timeField),
	}

	_, err := collection.Indexes().CreateOne(context.Background(), index)
	return err == nil
}

// Use this API to drop TTL Index.
func (c *MongoClient) RestfulAPIDropTTLIndex(collName string, timeField string) bool {
	collection := c.Client.Database(c.dbName).Collection(collName)
	_, err := collection.Indexes().DropOne(context.Background(), timeField)
	return err == nil
}

// Use this API to update timeout value for TTL Index.
func (c *MongoClient) RestfulAPIPatchTTLIndex(collName string, timeout int32, timeField string) bool {
	collection := c.Client.Database(c.dbName).Collection(collName)
	_, err := collection.Indexes().DropOne(context.Background(), timeField)
	if err != nil {
		// logger.MongoDBLog.Println("Drop Index on field (", timeField, ") for collection (", collName, ") failed : ", err)
	}

	// create new index with new timeout
	index := mongo.IndexModel{
		Keys:    bson.D{{Key: timeField, Value: 1}},
		Options: options.Index().SetExpireAfterSeconds(timeout).SetName(timeField),
	}

	_, err = collection.Indexes().CreateOne(context.Background(), index)
	if err != nil {
		// logger.MongoDBLog.Println("Index on field (", timeField, ") for collection (", collName, ") already exists : ", err)
	}

	return true
}

// This API adds document to collection with name : "collName"
// This API should be used when we wish to update the timeout value for the TTL index
// It checks if an Index with name "indexName" exists on the collection.
// If such an Index is "indexName" is found, we drop the index and then
// add new Index with new timeout value.
func (c *MongoClient) RestfulAPIPatchOneTimeout(collName string, filter bson.M, putData map[string]interface{}, timeout int32, timeField string) bool {
	collection := c.Client.Database(c.dbName).Collection(collName)
	var checkItem map[string]interface{}

	// fetch all Indexes on collection
	cursor, err := collection.Indexes().List(context.TODO())
	if err != nil {
		// logger.MongoDBLog.Println("RestfulAPIPatchOneTimeout : List Index failed for collection (", collName, ") : ", err)
		return false
	}

	var result []bson.M
	// convert to map
	if err = cursor.All(context.TODO(), &result); err != nil {
		// logger.MongoDBLog.Println("RestfulAPIPatchOneTimeout : Cursor decode failed for collection (", collName, ") : ", err)
	}

	// loop through the map and check for entry with key as name
	// for every entry with key as name,check if the value string contains the timeField string.
	// the Indexes are generally named such as follows :
	// field name : createdAt, index name : createdAt_1
	// drop the index if found.
	drop := false
	for _, v := range result {
		for k1, v1 := range v {
			valStr := fmt.Sprint(v1)
			if (k1 == "name") && strings.Contains(valStr, timeField) {
				_, err = collection.Indexes().DropOne(context.Background(), valStr)
				if err != nil {
					// logger.MongoDBLog.Println("Drop Index on field (", timeField, ") for collection (", collName, ") failed : ", err)
					break
				}
			}
		}
		if drop {
			break
		}
	}

	// create new index with new timeout
	index := mongo.IndexModel{
		Keys:    bson.D{{Key: timeField, Value: 1}},
		Options: options.Index().SetExpireAfterSeconds(timeout),
	}

	_, err = collection.Indexes().CreateOne(context.Background(), index)
	if err != nil {
		// logger.MongoDBLog.Println("Index on field (", timeField, ") for collection (", collName, ") already exists : ", err)
	}

	collection.FindOne(context.TODO(), filter).Decode(&checkItem)

	if checkItem == nil {
		collection.InsertOne(context.TODO(), putData)
		return false
	} else {
		collection.UpdateOne(context.TODO(), filter, bson.M{"$set": putData})
		return true
	}
}

// This API adds document to collection with name : "collName"
// It also creates an Index with Time to live (TTL) on the collection.
// All Documents in the collection will have the the same TTL. The timestamps
// each document can be different and can be updated as per procedure.
// If the Index with same timeout value is present already then it
// does not create a new one.
// If the Index exists on the same "timeField" with a different timeout,
// then API will return error saying Index already exists.
func (c *MongoClient) RestfulAPIPutOneTimeout(collName string, filter bson.M, putData map[string]interface{}, timeout int32, timeField string) bool {
	collection := c.Client.Database(c.dbName).Collection(collName)
	var checkItem map[string]interface{}

	collection.FindOne(context.TODO(), filter).Decode(&checkItem)

	if checkItem == nil {
		collection.InsertOne(context.TODO(), putData)
		return false
	} else {
		collection.UpdateOne(context.TODO(), filter, bson.M{"$set": putData})
		return true
	}
}

func (c *MongoClient) RestfulAPIPostOnly(collName string, filter bson.M, postData map[string]interface{}) bool {
	collection := c.Client.Database(c.dbName).Collection(collName)

	_, err := collection.InsertOne(context.TODO(), postData)
	return err == nil
}

func (c *MongoClient) RestfulAPIPutOnly(collName string, filter bson.M, putData map[string]interface{}) error {
	collection := c.Client.Database(c.dbName).Collection(collName)

	result, err := collection.UpdateOne(context.TODO(), filter, bson.M{"$set": putData})
	if result.MatchedCount != 0 {
		// logger.MongoDBLog.Println("matched and replaced an existing document")
		return nil
	}
	err = fmt.Errorf("failed to update document: %s", err)
	return err
}
