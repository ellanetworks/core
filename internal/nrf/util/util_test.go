//go:build !debug
// +build !debug

package util

import (
	"log"
	"testing"
	"time"

	"github.com/omec-project/openapi/models"
)

func TestDecode(t *testing.T) {
	// Set time
	date := time.Now()
	dateFormat, _ := time.Parse(time.RFC3339, date.Format(time.RFC3339))

	testData1 := map[string]interface{}{
		"NfInstanceId":   "0",
		"NfType":         models.NfType_NRF,
		"NfStatus":       models.NfStatus_REGISTERED,
		"HeartBeatTimer": 10,
		"PlmnList": &[]models.PlmnId{ // Pattern: '^[0-9]{3}[0-9]{2,3}$'
			{
				Mcc: "111",
				Mnc: "111",
			},
		},
		"SNssais": &[]models.Snssai{ // range 0-255
			{
				Sst: 222,
				Sd:  "SNssais",
			},
		},
		"NsiList": []string{
			"nsi0",
		},
		"Fqdn":          "fqdn",
		"InterPlmnFqdn": "InterPlmnFqdn",
		"Ipv4Addresses": []string{
			"140.113.1.1",
		},
		"Ipv6Addresses": []string{
			"fc00::",
		},
		"AllowedPlmns": &[]models.PlmnId{
			{
				Mcc: "111",
				Mnc: "111",
			},
		},
		"AllowedNfTypes": []models.NfType{
			models.NfType_NRF,
		},
		"AllowedNfDomains": []string{
			"nfdomain1",
		},
		"AllowedNssais": &[]models.Snssai{
			{
				Sst: 333,
				Sd:  "AllowedNssais",
			},
		},
		"Priority":             1,
		"Capacity":             1,
		"Load":                 1,
		"Locality":             "NCTU",
		"UdrInfo":              &models.UdrInfo{},
		"UdmInfo":              &models.UdmInfo{},
		"AusfInfo":             &models.AusfInfo{},
		"AmfInfo":              &models.AmfInfo{},
		"SmfInfo":              &models.SmfInfo{},
		"UpfInfo":              &models.UpfInfo{},
		"PcfInfo":              &models.PcfInfo{},
		"BsfInfo":              &models.BsfInfo{},
		"ChfInfo":              &models.ChfInfo{},
		"NrfInfo":              &models.NrfInfo{},
		"CustomInfo":           &map[string]interface{}{},
		"RecoveryTime":         &dateFormat,
		"NfServicePersistence": true,
		"NfServices": &[]models.NfService{
			{
				ServiceName:     models.ServiceName_NNRF_DISC,
				NfServiceStatus: models.NfServiceStatus_REGISTERED,
				AllowedNfDomains: []string{
					"nfdomain3",
					"nfdomain4",
				},
			},
		},
	}

	source := []map[string]interface{}{
		testData1,
	}

	target, err := Decode(source, time.RFC3339)
	if err != nil {
		t.Log(err)
	}

	log.Printf("%+v", target)
}
