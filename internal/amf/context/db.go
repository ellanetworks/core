package context

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"

	"github.com/omec-project/openapi/models"
	"github.com/omec-project/util/idgenerator"
	"github.com/omec-project/util/mongoapi"
	"github.com/yeastengine/ella/internal/amf/factory"
	"github.com/yeastengine/ella/internal/amf/logger"
	"go.mongodb.org/mongo-driver/bson"
)

var dbMutex sync.Mutex

type CustomFieldsAmfUe struct {
	State       map[models.AccessType]string `json:"state"`
	SmCtxList   map[string]SmContext         `json:"smCtxList"`
	N1N2Message N1N2Message                  `json:"n1n2Msg"`
	ULCount     uint32                       `json:"ulCount"`
	DLCount     uint32                       `json:"dlCount"`
	RanUeNgapId int64                        `json:"ranUeNgapId"`
	AmfUeNgapId int64                        `json:"amfUeNgapId"`
	RanId       string                       `json:"ranId"`
}

var (
	Namespace     = os.Getenv("POD_NAMESPACE")
	AmfUeDataColl = "amf.data.amfState"
)

func AllocateUniqueID(generator **idgenerator.IDGenerator, idName string) (int64, error) {
	// Use MongoDB increment field to generate new offset.
	// generate ids between offset to 8192 above offset.
	dbMutex.Lock()
	defer dbMutex.Unlock()
	if *generator == nil {
		logger.DataRepoLog.Infof("generator null. fetch offset from db")
		val := mongoapi.CommonDBClient.GetUniqueIdentity(idName)
		// Mongodb returns value starting from 1.
		// Limiting users to 8192(2^13) per instance.
		// TODO : Make this value configurable.
		//        Later this value can be used to trigger
		//        creation of new instance
		minVal := int64((val-1)*8192 + 1)
		maxVal := minVal + 8192
		*generator = idgenerator.NewGenerator(minVal, maxVal)
	}

	val, err := (*generator).Allocate()
	if err != nil {
		logger.DataRepoLog.Warnf("Max IDs generated for Instance")
		return -1, err
	}

	return val, nil
}

func SetupAmfCollection() {
	var mongoDbUrl string = "mongodb://mongodb:27017"
	if factory.AmfConfig.Configuration.AmfDBName == "" {
		factory.AmfConfig.Configuration.AmfDBName = "amf"
	}

	if (factory.AmfConfig.Configuration.Mongodb != nil) &&
		(factory.AmfConfig.Configuration.Mongodb.Url != "") {
		mongoDbUrl = factory.AmfConfig.Configuration.Mongodb.Url
	}

	logger.DataRepoLog.Infof("MondbName: %v, Url: %v", factory.AmfConfig.Configuration.AmfDBName, mongoDbUrl)

	if Namespace != "" {
		AmfUeDataColl = Namespace + "." + AmfUeDataColl
	}
	for {
		mongoapi.ConnectMongo(mongoDbUrl, factory.AmfConfig.Configuration.AmfDBName)
		if mongoapi.CommonDBClient.(*mongoapi.MongoClient).Client == nil {
			logger.DataRepoLog.Errorf("MongoDb Connection failed")
		} else {
			logger.DataRepoLog.Infof("Successfully connected to Mongodb")
			break
		}
	}
	_, err := mongoapi.CommonDBClient.CreateIndex(AmfUeDataColl, "supi")
	if err != nil {
		logger.DataRepoLog.Errorf("Create index failed on Supi field.")
	}

	_, err = mongoapi.CommonDBClient.CreateIndex(AmfUeDataColl, "guti")
	if err != nil {
		logger.DataRepoLog.Errorf("Create index failed on Guti field.")
	}

	_, err = mongoapi.CommonDBClient.CreateIndex(AmfUeDataColl, "tmsi")
	if err != nil {
		logger.DataRepoLog.Errorf("Create index failed on Tmsi field.")
	}

	/*_, err = CommonDBClient.CreateIndex(AmfUeDataColl, "customFieldsAmfUe.amfUeNgapId")
	if err != nil {
		logger.DataRepoLog.Errorf("Create index failed on AmfUeNgapID field.")
	}*/

	// Indexing for ranUeNgapId would fail if we have multiple gnbs.
	// TODO: We should create index with multiple fields (ranUeNgapId & ranIpAddr)
	/*_, err = CommonDBClient.CreateIndex(AmfUeDataColl, "customFieldsAmfUe.ranUeNgapId")
	if err != nil {
		logger.DataRepoLog.Errorf("Create index failed on RanUeNgapID field.")
	}*/
}

func ToBsonM(data *AmfUe) (ret bson.M) {
	tmp, err := json.Marshal(data)
	if err != nil {
		logger.DataRepoLog.Errorf("amfue marshall error: %v", err)
	}
	err = json.Unmarshal(tmp, &ret)
	if err != nil {
		logger.DataRepoLog.Errorf("amfue unmarshall error: %v", err)
	}

	return
}

func StoreContextInDB(ue *AmfUe) {
	self := AMF_Self()
	if self.EnableDbStore {
		amfUeBsonA := ToBsonM(ue)
		filter := bson.M{"supi": ue.Supi}

		_, postErr := mongoapi.CommonDBClient.RestfulAPIPost(AmfUeDataColl, filter, amfUeBsonA)
		if postErr != nil {
			logger.DataRepoLog.Warnln(postErr)
		}
	}
}

func DeleteContextFromDB(ue *AmfUe) {
	self := AMF_Self()
	if self.EnableDbStore {
		filter := bson.M{"supi": ue.Supi}

		delErr := mongoapi.CommonDBClient.RestfulAPIDeleteOne(AmfUeDataColl, filter)
		if delErr != nil {
			logger.DataRepoLog.Warnln(delErr)
		}
	}
}

func DbFetch(collName string, filter bson.M) *AmfUe {
	ue := &AmfUe{}
	ue.init()
	result, getOneErr := mongoapi.CommonDBClient.RestfulAPIGetOne(collName, filter)
	if getOneErr != nil {
		logger.DataRepoLog.Warnln(getOneErr)
	}

	if len(result) == 0 {
		return nil
	}
	err := json.Unmarshal(mapToByte(result), ue)
	if err != nil {
		logger.DataRepoLog.Errorf("amfue unmarshall error: %v", err)
		return nil
	}

	dbMutex.Lock()
	defer dbMutex.Unlock()

	ue.RanUe[models.AccessType__3_GPP_ACCESS].AmfUe = ue
	AMF_Self().RanUePool.Store(ue.RanUe[models.AccessType__3_GPP_ACCESS].AmfUeNgapId, ue.RanUe[models.AccessType__3_GPP_ACCESS])
	AMF_Self().UePool.Store(ue.Supi, ue)
	ue.EventChannel = nil
	ue.NASLog = logger.NasLog.WithField(logger.FieldAmfUeNgapID, fmt.Sprintf("AMF_UE_NGAP_ID:%d", ue.RanUe[models.AccessType__3_GPP_ACCESS].AmfUeNgapId))
	ue.GmmLog = logger.GmmLog.WithField(logger.FieldAmfUeNgapID, fmt.Sprintf("AMF_UE_NGAP_ID:%d", ue.RanUe[models.AccessType__3_GPP_ACCESS].AmfUeNgapId))
	ue.TxLog = logger.GmmLog.WithField(logger.FieldAmfUeNgapID, fmt.Sprintf("AMF_UE_NGAP_ID:%d", ue.RanUe[models.AccessType__3_GPP_ACCESS].AmfUeNgapId))
	ue.ProducerLog = logger.ProducerLog.WithField(logger.FieldSupi, fmt.Sprintf("SUPI:%s", ue.Supi))
	ue.AmfInstanceName = os.Getenv("HOSTNAME")
	ue.AmfInstanceIp = os.Getenv("POD_IP")
	ue.TxLog.Errorln("amfue fetched")
	return ue
}

func DbFetchRanUeByRanUeNgapID(ranUeNgapID int64, ran *AmfRan) *RanUe {
	filter := bson.M{}
	filter["customFieldsAmfUe.ranUeNgapId"] = ranUeNgapID
	filter["customFieldsAmfUe.ranId"] = ran.GnbId

	ue := DbFetch(AmfUeDataColl, filter)
	if ue == nil {
		logger.DataRepoLog.Errorln("DbFetchRanUeByRanUeNgapID: no document found for ranUeNgapID ", ranUeNgapID)
		return nil
	}

	// Check if some parallel procedure has already
	// fetched AmfUe and stored the RanUE in context.
	// If so, then return the stored RanUE
	// else return RanUE from newly fetched AmfUe
	// and store in context
	ranUe := ran.RanUeFindByRanUeNgapIDLocal(ranUeNgapID)
	if ranUe != nil {
		return ranUe
	}
	return ue.RanUe[models.AccessType__3_GPP_ACCESS]
}

func DbFetchRanUeByAmfUeNgapID(amfUeNgapID int64) *RanUe {
	self := AMF_Self()
	filter := bson.M{}
	filter["customFieldsAmfUe.amfUeNgapId"] = amfUeNgapID
	ue := DbFetch(AmfUeDataColl, filter)
	if ue == nil {
		logger.DataRepoLog.Errorln("DbFetchRanUeByAmfUeNgapID : no document found for amfUeNgapID ", amfUeNgapID)
		return nil
	}

	// Check if some parallel procedure has already
	// fetched AmfUe and stored the RanUE in context.
	// If so, then return the stored RanUE
	// else return RanUE from newly fetched AmfUe
	// and store in context
	ranUe := self.RanUeFindByAmfUeNgapIDLocal(amfUeNgapID)
	if ranUe != nil {
		return ranUe
	}
	return ue.RanUe[models.AccessType__3_GPP_ACCESS]
}

func DbFetchUeByGuti(guti string) (ue *AmfUe, ok bool) {
	self := AMF_Self()
	filter := bson.M{}
	filter["guti"] = guti

	ue = DbFetch(AmfUeDataColl, filter)
	if ue == nil {
		logger.DataRepoLog.Warnln("FindByGuti : no document found for guti ", guti)
		return nil, false
	} else {
		ok = true
	}

	// Check if some parallel procedure has already
	// fetched AmfUe. If so, then return the same.
	// else return newly fetched AmfUe and store in context
	if amfUe, ret := self.AmfUeFindByGutiLocal(guti); ret {
		logger.DataRepoLog.Infoln("FindByGuti : found by local", guti)
		ue = amfUe
		ok = ret
	}

	return ue, ok
}

func DbFetchUeBySupi(supi string) (ue *AmfUe, ok bool) {
	self := AMF_Self()
	filter := bson.M{}
	filter["supi"] = supi

	ue = DbFetch(AmfUeDataColl, filter)
	if ue == nil {
		logger.DataRepoLog.Warnln("FindBySupi : no document found for supi ", supi)
		return nil, false
	} else {
		ok = true
	}
	// Check if some parallel procedure has already
	// fetched AmfUe. If so, then return the same.
	// else return newly fetched AmfUe and store in context
	if amfUe, ret := self.AmfUeFindBySupiLocal(supi); ret {
		logger.DataRepoLog.Infoln("FindBySupi : found by local", supi)
		ue = amfUe
		ok = ret
	}

	return ue, ok
}

func DbFetchAllEntries() (ueList []*AmfUe) {
	ue := &AmfUe{}
	filter := bson.M{}
	results, getManyErr := mongoapi.CommonDBClient.RestfulAPIGetMany(AmfUeDataColl, filter)
	if getManyErr != nil {
		logger.DataRepoLog.Warnln(getManyErr)
	}

	for _, val := range results {
		ue = &AmfUe{}
		ue.init()
		err := json.Unmarshal(mapToByte(val), ue)
		if err != nil {
			logger.DataRepoLog.Errorf("amfue unmarshall error: %v", err)
			return nil
		}
		ueList = append(ueList, ue)
	}

	return ueList
}
