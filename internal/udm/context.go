package udm

import (
	"sync"

	"github.com/ellanetworks/core/internal/util/idgenerator"
	"github.com/ellanetworks/core/internal/util/suci"
	"github.com/omec-project/openapi"
	"github.com/omec-project/openapi/models"
)

var udmContext UDMContext

const (
	LocationUriAmf3GppAccessRegistration int = iota
	LocationUriAmfNon3GppAccessRegistration
	LocationUriSmfRegistration
	LocationUriSdmSubscription
	LocationUriSharedDataSubscription
)

type UDMContext struct {
	NfId                           string
	GroupId                        string
	UriScheme                      models.UriScheme
	NfService                      map[models.ServiceName]models.NfService
	UdmUePool                      sync.Map // map[supi]*UdmUeContext
	GpsiSupiList                   models.IdentityData
	SharedSubsDataMap              map[string]models.SharedData // sharedDataIds as key
	SubscriptionOfSharedDataChange sync.Map                     // subscriptionID as key
	SuciProfiles                   []suci.SuciProfile
	EeSubscriptionIDGenerator      *idgenerator.IDGenerator
}

type UdmNFContext struct {
	SubscribeToNotifChange           *models.SdmSubscription // SubscriptionID as key
	SubscribeToNotifSharedDataChange *models.SdmSubscription // SubscriptionID as key
	SubscriptionID                   string
}

func (context *UDMContext) ManageSmData(smDatafromUDR []models.SessionManagementSubscriptionData, snssaiFromReq string,
	dnnFromReq string) (mp map[string]models.SessionManagementSubscriptionData,
) {
	smDataMap := make(map[string]models.SessionManagementSubscriptionData)
	AllDnns := make([]map[string]models.DnnConfiguration, len(smDatafromUDR))

	for idx, smSubscriptionData := range smDatafromUDR {
		singleNssaiStr := openapi.MarshToJsonString(smSubscriptionData.SingleNssai)[0]
		smDataMap[singleNssaiStr] = smSubscriptionData
		AllDnns[idx] = smSubscriptionData.DnnConfigurations
	}

	return smDataMap
}

// functions related UecontextInSmfData
func (context *UDMContext) CreateUeContextInSmfDataforUe(supi string, body models.UeContextInSmfData) {
	ue, ok := context.UdmUeFindBySupi(supi)
	if !ok {
		ue = context.NewUdmUe(supi)
	}
	ue.UeCtxtInSmfData = &body
}

// functions for SmfSelectionSubscriptionData
func (context *UDMContext) CreateSmfSelectionSubsDataforUe(supi string, body models.SmfSelectionSubscriptionData) {
	ue, ok := context.UdmUeFindBySupi(supi)
	if !ok {
		ue = context.NewUdmUe(supi)
	}
	ue.SmfSelSubsData = &body
}

func (context *UDMContext) NewUdmUe(supi string) *UdmUeContext {
	ue := new(UdmUeContext)
	ue.init()
	ue.Supi = supi
	context.UdmUePool.Store(supi, ue)
	return ue
}

func (context *UDMContext) UdmUeFindBySupi(supi string) (*UdmUeContext, bool) {
	if value, ok := context.UdmUePool.Load(supi); ok {
		return value.(*UdmUeContext), ok
	} else {
		return nil, false
	}
}

func (context *UDMContext) UdmAmf3gppRegContextExists(supi string) bool {
	if ue, ok := context.UdmUeFindBySupi(supi); ok {
		return ue.Amf3GppAccessRegistration != nil
	} else {
		return false
	}
}

func (context *UDMContext) CreateAmf3gppRegContext(supi string, body models.Amf3GppAccessRegistration) {
	ue, ok := context.UdmUeFindBySupi(supi)
	if !ok {
		ue = context.NewUdmUe(supi)
	}
	ue.Amf3GppAccessRegistration = &body
}
