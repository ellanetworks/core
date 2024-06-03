package context

import (
	"github.com/omec-project/openapi/models"
)

type SearchNFInstances struct {
	TargetNFType            models.NfType        `form:"target-nf-type" binding:"required"`
	RequesterNFType         models.NfType        `form:"requester-nf-type" binding:"required"`
	ServiceNames            []models.ServiceName `form:"service-names" `
	RequesterNfInstanceFqdn string               `form:"requester-nf-instance-fqdn" `
	TargetPlmnList          []models.PlmnId      `form:"target-plmn-list" `
	RequesterPlmnList       []models.PlmnId      `form:"requester-plmn-list" `
	TargetNfInstanceID      string               `form:"target-nf-instance-id" `
	TargetNfFqdn            string               `form:"target-nf-type" `
	HnrfURI                 models.UriScheme     `form:"hnrf-uri" `
	Snssais                 []models.Snssai      `form:"snssais" `
	PlmnSpecificSnssaiList  []models.PlmnSnssai  `form:"plmn-specific-snssai-list"`
	Dnn                     string               `form:"dnn" `
	NsiList                 []string             `form:"nsi-list" `
	SmfServingArea          string               `form:"smf-serving-area" `
	Tai                     models.Tai           `form:"tai" `
	AmfRegionID             string               `form:"amf-region-id" `
	AmfSetID                string               `form:"amf-set-id" `
	Guami                   models.Guami         `form:"guami" `
	Supi                    string               `form:"supi" `
	UeIpv4Address           string               `form:"ue-ipv4-address" `
	IPDomain                string               `form:"ip-domain" `
	UeIpv6Prefix            string               `form:"ue-ipv6-prefix" `
	Pgw                     string               `form:"pgw" `
	Gpsi                    string               `form:"gpsi" `
	ExternalGroupIdentity   string               `form:"external-group-identity" `
	RoutingIndicator        string               `form:"routing-indicator" `
	PreferredLocality       string               `form:"preferred-locality" `
	DataSet                 models.DataSetId     `form:"data-set" `
	ChfSupportedPlmn        models.PlmnId        `form:"chf-supported-plmn" `
	AccessType              models.AccessType    `form:"access-type" `
	GroupIDList             []string             `form:"group-id-list" `
	DnaiList                []string             `form:"dnai-list" `
	SupportedFeatures       []string             `form:"supported-features" `
	UpfIwkEpsInd            bool                 `form:"upf-iwk-eps-ind" `
	PgwInd                  bool                 `form:"pgw-ind" `
	// IfNoneMatch             string            `form:"target-nf-type" `
}
