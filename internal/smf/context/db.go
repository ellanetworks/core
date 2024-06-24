package context

import (
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"strconv"
	"sync"

	"github.com/omec-project/util/idgenerator"
	"github.com/omec-project/util/mongoapi"
	"github.com/yeastengine/ella/internal/smf/factory"
	"github.com/yeastengine/ella/internal/smf/logger"
	"go.mongodb.org/mongo-driver/bson"
)

const (
	SmContextDataColl = "smf.data.smContext"
	// TransactionDataCol = "smf.data.transaction"
	SeidSmContextCol = "smf.data.seidSmContext"
	NodeInDBCol      = "smf.data.nodeInDB"
	// SMPolicyClientCol  = "smf.data.smPolicyClient"
	RefSeidCol    = "smf.data.refToSeid"
	SmfCounterCol = "smf.data.smfCount"
)

func SetupSmfCollection() {
	dbName := "smf"
	dbUrl := "mongodb://mongodb-arbiter-headless"

	if factory.SmfConfig.Configuration.Mongodb.Url != "" {
		dbUrl = factory.SmfConfig.Configuration.Mongodb.Url
	}

	if factory.SmfConfig.Configuration.SmfDbName != "" {
		dbName = factory.SmfConfig.Configuration.SmfDbName
	}

	logger.CfgLog.Infof("initialising db name [%v] url [%v] ", dbName, dbUrl)

	// UUID table
	mongoapi.ConnectMongo(dbUrl, dbName)
	_, err := mongoapi.CommonDBClient.CreateIndex(SmContextDataColl, "ref")
	if err != nil {
		logger.DataRepoLog.Errorf("Create index failed on ref field.")
	}

	// SEID Table
	_, err = mongoapi.CommonDBClient.CreateIndex(SeidSmContextCol, "seid")
	if err != nil {
		logger.DataRepoLog.Errorf("Create index failed on TxnId field.")
	}

	smfCount := mongoapi.CommonDBClient.GetUniqueIdentity("smfCount")
	logger.DataRepoLog.Infof("unique id - init smfCount %d", smfCount)

	// set os env
	setEnvErr := os.Setenv("SMF_COUNT", strconv.Itoa(int(smfCount)))
	if setEnvErr != nil {
		logger.DataRepoLog.Errorf("Setting SMF_COUNT env variable is failed.")
	}
}

// print out sm context
func (smContext *SMContext) String() string {
	return fmt.Sprintf("smContext content: Ref:[%v],\nSupi: [%v],\nPei:[%v],\nGpsi:[%v],\nPDUSessionID:[%v],\nDnn:[%v],Snssai: [%v],\nHplmnSnssai: [%v],\nServingNetwork: [%v],\nServingNfId: [%v],\nUpCnxState: [%v],\nAnType: [%v],\nRatType: [%v],\nPDUAddress: [%v],\nSelectedPDUSessionType: [%v],\nSmStatusNotifyUri: [%v],\nSelectedPCFProfile: [%v],\nSMContextState: [%v],\nTunnel: [%v],\nPFCPContext: [%v],\nIdentifier: [%v],\nDNNInfo: [%v],\nSmPolicyData: [%v],\nEstAcceptCause5gSMValue: [%v]\n", smContext.Ref, smContext.Supi, smContext.Pei, smContext.Gpsi, smContext.PDUSessionID, smContext.Dnn, smContext.Snssai, smContext.HplmnSnssai, smContext.ServingNetwork, smContext.ServingNfId, smContext.UpCnxState, smContext.AnType, smContext.RatType, smContext.PDUAddress, smContext.SelectedPDUSessionType, smContext.SmStatusNotifyUri, smContext.SelectedPCFProfile, smContext.SMContextState, smContext.Tunnel, smContext.PFCPContext, smContext.Identifier, smContext.DNNInfo, smContext.SmPolicyData, smContext.EstAcceptCause5gSMValue)
}

// MarshalJSON customized marshaller for sm context
func (smContext *SMContext) MarshalJSON() ([]byte, error) {
	type Alias SMContext

	dataPathPoolInDBVal := make(map[int64]*DataPathInDB)

	var dataPathInDBIf interface{}
	var FirstDPNodeIf interface{}

	var upTunnelVal UPTunnelInDB
	if smContext.Tunnel != nil {
		upTunnelVal.ANInformation = smContext.Tunnel.ANInformation

		if smContext.Tunnel.DataPathPool != nil {
			for key, val := range smContext.Tunnel.DataPathPool {
				dataPathInDBIf = val
				dataPath := dataPathInDBIf.(*DataPath)

				firstDPNode := dataPath.FirstDPNode
				FirstDPNodeIf = firstDPNode

				dataPathNode := FirstDPNodeIf.(*DataPathNode)

				dataPathNodeInDBVal := StoreDataPathNode(dataPathNode)
				newDataPathInDB := &DataPathInDB{
					Activated:         dataPath.Activated,
					IsDefaultPath:     dataPath.IsDefaultPath,
					Destination:       dataPath.Destination,
					HasBranchingPoint: dataPath.HasBranchingPoint,
					FirstDPNode:       dataPathNodeInDBVal,
				}

				dataPathPoolInDBVal[key] = newDataPathInDB
			}
			upTunnelVal.DataPathPool = dataPathPoolInDBVal
		}
	}

	var pfcpSessionContextInDB PFCPSessionContextInDB
	PFCPContextVal := make(PFCPContextInDB)
	// store localseid and remoteseid
	for key, pfcpCtx := range smContext.PFCPContext {
		pfcpSessionContextInDB.NodeID = pfcpCtx.NodeID
		pfcpSessionContextInDB.PDRs = pfcpCtx.PDRs
		pfcpSessionContextInDB.LocalSEID = SeidConv(pfcpCtx.LocalSEID)
		pfcpSessionContextInDB.RemoteSEID = SeidConv(pfcpCtx.RemoteSEID)
		PFCPContextVal[key] = pfcpSessionContextInDB
	}

	return json.Marshal(&struct {
		*Alias
		PFCPContext PFCPContextInDB `json:"pfcpContext"`
		Tunnel      UPTunnelInDB    `json:"tunnel"`
	}{
		Alias:       (*Alias)(smContext),
		PFCPContext: PFCPContextVal,
		Tunnel:      upTunnelVal,
	})
}

// UnmarshalJSON customized unmarshaller for sm context
func (smContext *SMContext) UnmarshalJSON(data []byte) error {
	fmt.Println("db - in UnmarshalJSON")
	type Alias SMContext
	aux := &struct {
		*Alias
		PFCPContextVal PFCPContextInDB `json:"pfcpContext"`
		Tunnel         UPTunnelInDB    `json:"tunnel"`
	}{
		Alias: (*Alias)(smContext),
	}

	if err := json.Unmarshal(data, &aux); err != nil {
		fmt.Println("Err in customized unMarshall!!")
		return err
	}

	// recover smContext.PFCPContext
	smContext.PFCPContext = make(map[string]*PFCPSessionContext)
	for key, pfcpCtxInDB := range aux.PFCPContextVal {
		smContext.PFCPContext[key] = &PFCPSessionContext{}
		smContext.PFCPContext[key].NodeID = pfcpCtxInDB.NodeID
		smContext.PFCPContext[key].PDRs = pfcpCtxInDB.PDRs
		smContext.PFCPContext[key].LocalSEID, _ = strconv.ParseUint(pfcpCtxInDB.LocalSEID, 16, 64)
		smContext.PFCPContext[key].RemoteSEID, _ = strconv.ParseUint(pfcpCtxInDB.RemoteSEID, 16, 64)
	}

	var dataPathInDBIf interface{}
	var FirstDPNodeIf interface{}
	smContext.Tunnel = &UPTunnel{}
	if !reflect.DeepEqual(aux.Tunnel, UPTunnelInDB{}) {
		smContext.Tunnel.ANInformation = aux.Tunnel.ANInformation
		smContext.Tunnel.PathIDGenerator = idgenerator.NewGenerator(1, 2147483647)
		smContext.Tunnel.DataPathPool = NewDataPathPool()
		for key, val := range aux.Tunnel.DataPathPool {
			dataPathInDBIf = val
			dataPathInDB := dataPathInDBIf.(*DataPathInDB)

			firstDPNode := dataPathInDB.FirstDPNode
			FirstDPNodeIf = firstDPNode
			dataPathNodeInDBVal := FirstDPNodeIf.(*DataPathNodeInDB)
			dataPathNodeVal := RecoverDataPathNode(dataPathNodeInDBVal)

			newDataPath := NewDataPath()

			newDataPath.Activated = dataPathInDB.Activated
			newDataPath.IsDefaultPath = dataPathInDB.IsDefaultPath
			newDataPath.Destination = dataPathInDB.Destination
			newDataPath.HasBranchingPoint = dataPathInDB.HasBranchingPoint

			newDataPath.FirstDPNode = dataPathNodeVal

			smContext.Tunnel.DataPathPool[key] = newDataPath
		}
	}
	// recover logs
	smContext.initLogTags()
	// recover SBIPFCPCommunicationChan
	smContext.SBIPFCPCommunicationChan = make(chan PFCPSessionResponseStatus, 1)

	return nil
}

func ToBsonMSeidRef(data SeidSmContextRef) (ret bson.M) {
	// Marshal data into json format
	tmp, err := json.Marshal(data)
	if err != nil {
		logger.DataRepoLog.Errorf("SMContext marshall error: %v", err)
	}

	// unmarshal data into bson format
	err = json.Unmarshal(tmp, &ret)
	if err != nil {
		logger.DataRepoLog.Errorf("SMContext unmarshall error: %v", err)
	}

	return
}

func ToBsonM(data *SMContext) (ret bson.M) {
	// Marshal data into json format
	fmt.Println("db - in ToBsonM before marshal")
	tmp, err := json.Marshal(data)
	if err != nil {
		logger.DataRepoLog.Errorf("SMContext marshall error: %v", err)
	}
	// unmarshal data into bson format
	err = json.Unmarshal(tmp, &ret)
	if err != nil {
		logger.DataRepoLog.Errorf("SMContext unmarshall error: %v", err)
	}

	return
}

// StoreSmContextInDB Store SmContext In DB
func StoreSmContextInDB(smContext *SMContext) {
	fmt.Println("db - Store SMContext In DB w ref")
	smContext.SMLock.Lock()
	defer smContext.SMLock.Unlock()
	smContextBsonA := ToBsonM(smContext)
	filter := bson.M{"ref": smContext.Ref}
	logger.DataRepoLog.Infof("filter: %+v", filter)

	_, postErr := mongoapi.CommonDBClient.RestfulAPIPost(SmContextDataColl, filter, smContextBsonA)
	if postErr != nil {
		logger.DataRepoLog.Warnln(postErr)
	}
}

type SeidSmContextRef struct {
	Ref  string `json:"ref" yaml:"ref" bson:"ref"`
	Seid string `json:"seid" yaml:"seid" bson:"seid"`
}

func SeidConv(seid uint64) (seidStr string) {
	seidStr = strconv.FormatUint(seid, 16)
	return seidStr
}

// StoreSeidContextInDB Store Context by SEID In DB
func StoreSeidContextInDB(seidUint uint64, smContext *SMContext) {
	seid := SeidConv(seidUint)
	item := SeidSmContextRef{
		Ref:  smContext.Ref,
		Seid: seid,
	}
	itemBsonA := ToBsonMSeidRef(item)
	filter := bson.M{"seid": seid}
	logger.DataRepoLog.Infof("filter: %+v", filter)

	_, postErr := mongoapi.CommonDBClient.RestfulAPIPost(SeidSmContextCol, filter, itemBsonA)
	if postErr != nil {
		logger.DataRepoLog.Warnln(postErr)
	}
}

// StoreRefToSeidInDB store mapping ref to seid in db
func StoreRefToSeidInDB(seidUint uint64, smContext *SMContext) {
	seid := SeidConv(seidUint)
	item := SeidSmContextRef{
		Ref:  smContext.Ref,
		Seid: seid,
	}
	itemBsonA := ToBsonMSeidRef(item)
	filter := bson.M{"ref": smContext.Ref}
	logger.DataRepoLog.Infof("filter: %+v", filter)

	_, postErr := mongoapi.CommonDBClient.RestfulAPIPost(RefSeidCol, filter, itemBsonA)
	if postErr != nil {
		logger.DataRepoLog.Warnln(postErr)
	}
}

func GetSeidByRefInDB(ref string) (seid uint64) {
	filter := bson.M{}
	filter["ref"] = ref

	result, getOneErr := mongoapi.CommonDBClient.RestfulAPIGetOne(RefSeidCol, filter)
	if getOneErr != nil {
		logger.DataRepoLog.Warnln(getOneErr)
	}
	seidStr := result["seid"].(string)
	seid, _ = strconv.ParseUint(seidStr, 16, 64)
	return
}

// GetSMContextByRefInDB GetSMContext By Ref from DB
func GetSMContextByRefInDB(ref string) (smContext *SMContext) {
	logger.DataRepoLog.Debugf("GetSMContextByRefInDB: Ref in DB %v", ref)
	smContext = &SMContext{}
	filter := bson.M{}
	filter["ref"] = ref

	result, getOneErr := mongoapi.CommonDBClient.RestfulAPIGetOne(SmContextDataColl, filter)
	if getOneErr != nil {
		logger.DataRepoLog.Warnln(getOneErr)
	}

	if result != nil {
		err := json.Unmarshal(mapToByte(result), smContext)
		if err != nil {
			logger.DataRepoLog.Errorf("smContext unmarshall error: %v", err)
			return nil
		}
	} else {
		logger.DataRepoLog.Warningf("SmContext doesn't exist with ref: %v", ref)
		return nil
	}

	return smContext
}

// GetSMContextBySEIDInDB GetSMContext By SEID from DB
func GetSMContextBySEIDInDB(seidUint uint64) (smContext *SMContext) {
	seid := SeidConv(seidUint)
	filter := bson.M{}
	filter["seid"] = seid

	result, getOneErr := mongoapi.CommonDBClient.RestfulAPIGetOne(SeidSmContextCol, filter)
	if getOneErr != nil {
		logger.DataRepoLog.Warnln(getOneErr)
	}
	if result != nil {
		ref := result["ref"].(string)
		logger.DataRepoLog.Debugln("StoreSeidContextInDB, result string : ", ref)
		return GetSMContext(ref)
	} else {
		logger.DataRepoLog.Warningf("SmContext doesn't exist with seid: %v", seid)
		return nil
	}
}

// DeleteSmContextInDBBySEID Delete SMContext By SEID from DB
func DeleteSmContextInDBBySEID(seidUint uint64) {
	seid := SeidConv(seidUint)
	fmt.Println("db - delete SMContext In DB by seid")
	filter := bson.M{"seid": seid}
	logger.DataRepoLog.Infof("filter: %+v", filter)

	result, getOneErr := mongoapi.CommonDBClient.RestfulAPIGetOne(SeidSmContextCol, filter)
	if getOneErr != nil {
		logger.DataRepoLog.Warnln(getOneErr)
	}
	if result != nil {
		ref := result["ref"].(string)

		delOneErr := mongoapi.CommonDBClient.RestfulAPIDeleteOne(SeidSmContextCol, filter)
		if delOneErr != nil {
			logger.DataRepoLog.Warnln(delOneErr)
		}
		DeleteSmContextInDBByRef(ref)
	} else {
		logger.DataRepoLog.Infof("DB entry doesn't exist with seid: %v\n", seid)
	}
}

// DeleteSmContextInDBByRef Delete SMContext By ref from DB
func DeleteSmContextInDBByRef(ref string) {
	fmt.Println("db - delete SMContext In DB w ref")
	filter := bson.M{"ref": ref}
	logger.DataRepoLog.Infof("filter: %+v", filter)

	delOneErr := mongoapi.CommonDBClient.RestfulAPIDeleteOne(SmContextDataColl, filter)
	if delOneErr != nil {
		logger.DataRepoLog.Warnln(delOneErr)
	}
}

// ClearSMContextInMem Delete SMContext in smContextPool and seidSMContextMap, for test
func ClearSMContextInMem(ref string) {
	smContext := GetSMContext(ref)
	smContextPool.Delete(ref)
	seid := GetSeidByRefInDB(ref)
	seidSMContextMap.Delete(seid)
	canonicalRef.Delete(canonicalName(smContext.Identifier, smContext.PDUSessionID))
}

func mapToByte(data map[string]interface{}) (ret []byte) {
	ret, _ = json.Marshal(data)
	return
}

func ShowSmContextPool() {
	smContextPool.Range(func(k, v interface{}) bool {
		fmt.Println("db - iterate:", k, v)
		return true
	})
}

func GetSmContextPool() *sync.Map {
	return &smContextPool
}

func StoreSmContextPool(smContext *SMContext) {
	smContextPool.Store(smContext.Ref, smContext)
}

// func GetLocalIP() string {
// 	addrs, err := net.InterfaceAddrs()
// 	if err != nil {
// 		return ""
// 	}
// 	for _, address := range addrs {
// 		// check the address type and if it is not a loopback the display it
// 		if ipnet, ok := address.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
// 			if ipnet.IP.To4() != nil {
// 				return ipnet.IP.String()
// 			}
// 		}
// 	}
// 	return ""
// }
