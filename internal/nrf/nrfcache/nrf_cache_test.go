// SPDX-FileCopyrightText: 2022 Open Networking Foundation <info@opennetworking.org>
//
// SPDX-License-Identifier: Apache-2.0
package nrf_cache

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/antihax/optional"
	"github.com/omec-project/openapi/Nnrf_NFDiscovery"
	"github.com/omec-project/openapi/models"
	"github.com/yeastengine/canard/internal/nrf/logger"
)

var nfProfilesDb map[string]string
var validityPeriod int32
var evictionInterval int32
var nrfDbCallbackCallCount int32

func init() {

	validityPeriod = 60
	evictionInterval = 120

	nrfDbCallbackCallCount = 0

	nfProfilesDb = make(map[string]string)

	nfProfilesDb["SMF-010203-internet"] = `{
		  "ipv4Addresses": [
			"smf"
		  ],
		  "allowedPlmns": [
			{
			  "mcc": "208",
			  "mnc": "93"
			}
		  ],
		  "smfInfo": {
			"sNssaiSmfInfoList": [
			  {
				"sNssai": {
				  "sst": 1,
				  "sd": "010203"
				},
				"dnnSmfInfoList": [
				  {
					"dnn": "internet"
				  }
				]
			  }
			]
		  },
		  "nfServices": [
			{
			  "apiPrefix": "http://smf:29502",
			  "allowedPlmns": [
				{
				  "mcc": "208",
				  "mnc": "93"
				}
			  ],
			  "serviceInstanceId": "b926f193-1083-49a8-adb3-5fcf57a1f0bfnsmf-pdusession",
			  "serviceName": "nsmf-pdusession",
			  "versions": [
				{
				  "apiVersionInUri": "v1",
				  "apiFullVersion": "https://smf:29502/nsmf-pdusession/v1",
				  "expiry": "2022-08-17T05:31:40.997097141Z"
				}
			  ],
			  "scheme": "https",
			  "nfServiceStatus": "REGISTERED"
			},
			{
			  "scheme": "https",
			  "nfServiceStatus": "REGISTERED",
			  "apiPrefix": "http://smf:29502",
			  "allowedPlmns": [
				{
				  "mcc": "208",
				  "mnc": "93"
				}
			  ],
			  "serviceInstanceId": "b926f193-1083-49a8-adb3-5fcf57a1f0bfnsmf-event-exposure",
			  "serviceName": "nsmf-event-exposure",
			  "versions": [
				{
				  "apiVersionInUri": "v1",
				  "apiFullVersion": "https://smf:29502/nsmf-pdusession/v1",
				  "expiry": "2022-08-17T05:31:40.997097141Z"
				}
			  ]
			}
		  ],
		  "nfInstanceId": "b926f193-1083-49a8-adb3-5fcf57a1f0bf",
		  "plmnList": [
			{
			  "mnc": "93",
			  "mcc": "208"
			}
		  ],
		  "sNssais": [
			{
			  "sd": "010203",
			  "sst": 1
			}
		  ],
		  "nfType": "SMF",
		  "nfStatus": "REGISTERED"
		}`
	nfProfilesDb["SMF-010203-ims"] = `{
		  "ipv4Addresses": [
			"smf"
		  ],
		  "allowedPlmns": [
			{
			  "mcc": "208",
			  "mnc": "93"
			}
		  ],
		  "smfInfo": {
			"sNssaiSmfInfoList": [
			  {
				"sNssai": {
				  "sst": 1,
				  "sd": "010203"
				},
				"dnnSmfInfoList": [
				  {
					"dnn": "ims"
				  }
				]
			  }
			]
		  },
		  "nfServices": [
			{
			  "apiPrefix": "http://smf:29502",
			  "allowedPlmns": [
				{
				  "mcc": "208",
				  "mnc": "93"
				}
			  ],
			  "serviceInstanceId": "c926f193-1083-49a8-adb3-5fcf57a1f0bfnsmf-pdusession",
			  "serviceName": "nsmf-pdusession",
			  "versions": [
				{
				  "apiVersionInUri": "v1",
				  "apiFullVersion": "https://smf:29502/nsmf-pdusession/v1",
				  "expiry": "2022-08-17T05:31:40.997097141Z"
				}
			  ],
			  "scheme": "https",
			  "nfServiceStatus": "REGISTERED"
			},
			{
			  "scheme": "https",
			  "nfServiceStatus": "REGISTERED",
			  "apiPrefix": "http://smf:29502",
			  "allowedPlmns": [
				{
				  "mcc": "208",
				  "mnc": "93"
				}
			  ],
			  "serviceInstanceId": "c926f193-1083-49a8-adb3-5fcf57a1f0bfnsmf-event-exposure",
			  "serviceName": "nsmf-event-exposure",
			  "versions": [
				{
				  "apiVersionInUri": "v1",
				  "apiFullVersion": "https://smf:29502/nsmf-pdusession/v1",
				  "expiry": "2022-08-17T05:31:40.997097141Z"
				}
			  ]
			}
		  ],
		  "nfInstanceId": "c926f193-1083-49a8-adb3-5fcf57a1f0bf",
		  "plmnList": [
			{
			  "mnc": "93",
			  "mcc": "208"
			}
		  ],
		  "sNssais": [
			{
			  "sd": "010203",
			  "sst": 1
			}
		  ],
		  "nfType": "SMF",
		  "nfStatus": "REGISTERED"
		}
`
	nfProfilesDb["SMF-0a0b0c-internet"] = `{
		  "ipv4Addresses": [
			"smf"
		  ],
		  "allowedPlmns": [
			{
			  "mcc": "208",
			  "mnc": "93"
			}
		  ],
		  "smfInfo": {
			"sNssaiSmfInfoList": [
			  {
				"sNssai": {
				  "sst": 1,
				  "sd": "0a0b0c"
				},
				"dnnSmfInfoList": [
				  {
					"dnn": "internet"
				  }
				]
			  }
			]
		  },
		  "nfServices": [
			{
			  "apiPrefix": "http://smf:29502",
			  "allowedPlmns": [
				{
				  "mcc": "208",
				  "mnc": "93"
				}
			  ],
			  "serviceInstanceId": "d926f193-1083-49a8-adb3-5fcf57a1f0bfnsmf-pdusession",
			  "serviceName": "nsmf-pdusession",
			  "versions": [
				{
				  "apiVersionInUri": "v1",
				  "apiFullVersion": "https://smf:29502/nsmf-pdusession/v1",
				  "expiry": "2022-08-17T05:31:40.997097141Z"
				}
			  ],
			  "scheme": "https",
			  "nfServiceStatus": "REGISTERED"
			},
			{
			  "scheme": "https",
			  "nfServiceStatus": "REGISTERED",
			  "apiPrefix": "http://smf:29502",
			  "allowedPlmns": [
				{
				  "mcc": "208",
				  "mnc": "93"
				}
			  ],
			  "serviceInstanceId": "d926f193-1083-49a8-adb3-5fcf57a1f0bfnsmf-event-exposure",
			  "serviceName": "nsmf-event-exposure",
			  "versions": [
				{
				  "apiVersionInUri": "v1",
				  "apiFullVersion": "https://smf:29502/nsmf-pdusession/v1",
				  "expiry": "2022-08-17T05:31:40.997097141Z"
				}
			  ]
			}
		  ],
		  "nfInstanceId": "d926f193-1083-49a8-adb3-5fcf57a1f0bf",
		  "plmnList": [
			{
			  "mnc": "93",
			  "mcc": "208"
			}
		  ],
		  "sNssais": [
			{
			  "sd": "0a0b0c",
			  "sst": 1
			}
		  ],
		  "nfType": "SMF",
		  "nfStatus": "REGISTERED"
		}`
	nfProfilesDb["AUSF-1"] = `{ "nfServices": [
			{
			  "serviceName": "nausf-auth",
			  "versions": [
				{
				  "apiVersionInUri": "v1",
				  "apiFullVersion": "1.0.0"
				}
			  ],
			  "scheme": "http",
			  "nfServiceStatus": "REGISTERED",
			  "ipEndPoints": [
				{
				  "ipv4Address": "ausf",
				  "port": 29509
				}
			  ],
			  "serviceInstanceId": "57d0a167-5283-4170-bdd8-881076049a81"
			}
		  ],
		  "ausfInfo": {
			"supiRanges": [
			  { "start": "123456789040000", "end": "123456789049999" }
			]
		  },
		  "nfInstanceId": "57d0a167-5283-4170-bdd8-881076049a81",
		  "nfType": "AUSF",
		  "nfStatus": "REGISTERED",
		  "plmnList": [
			{
			  "mcc": "208",
			  "mnc": "93"
			}
		  ],
		  "ipv4Addresses": [
			"ausf"
		  ],
		  "ausfInfo": {
			"groupId": "ausfGroup001"
		  }
		}`
	nfProfilesDb["AUSF-2"] = `{ "nfServices": [
			{
			  "serviceName": "nausf-auth",
			  "versions": [
				{
				  "apiVersionInUri": "v1",
				  "apiFullVersion": "1.0.0"
				}
			  ],
			  "scheme": "http",
			  "nfServiceStatus": "REGISTERED",
			  "ipEndPoints": [
				{
				  "ipv4Address": "ausf",
				  "port": 29509
				}
			  ],
			  "serviceInstanceId": "67d0a167-5283-4170-bdd8-881076049a81"
			}
		  ],
		  "ausfInfo": {
			"supiRanges": [
			  { "pattern": "^imsi-22345678904[0-9]{4}$" }
			]
		  },
		  "nfInstanceId": "67d0a167-5283-4170-bdd8-881076049a81",
		  "nfType": "AUSF",
		  "nfStatus": "REGISTERED",
		  "plmnList": [
			{
			  "mcc": "208",
			  "mnc": "93"
			}
		  ],
		  "ipv4Addresses": [
			"ausf"
		  ],
		  "ausfInfo": {
			"groupId": "ausfGroup001"
		  }
		}`

	nfProfilesDb["AMF-01"] = `
	{
	  "nfServices": [
		{
		  "serviceInstanceId": "0",
		  "serviceName": "namf-comm",
		  "versions": [
			{
			  "apiVersionInUri": "v1",
			  "apiFullVersion": "1.0.0"
			}
		  ],
		  "scheme": "http",
		  "nfServiceStatus": "REGISTERED",
		  "ipEndPoints": [
			{
			  "ipv4Address": "amf",
			  "transport": "TCP",
			  "port": 29518
			}
		  ],
		  "apiPrefix": "http://amf:29518"
		}
	  ],
	  "nfInstanceId": "9f7d5a3f-88ab-4525-b31e-334da7faedab",
	  "nfType": "AMF",
	  "nfStatus": "REGISTERED",
	  "plmnList": [
		{
		  "mcc": "208",
		  "mnc": "93"
		}
	  ],
	  "sNssais": [
		{
		  "sst": 1,
		  "sd": "010203"
		}
	  ],
	  "ipv4Addresses": [
		"amf"
	  ],
	  "amfInfo": {
		"amfSetId": "3f8",
		"amfRegionId": "ca",
		"guamiList": [
		  {
			"plmnId": {
			  "mcc": "208",
			  "mnc": "93"
			},
			"amfId": "cafe00"
		  }
		],
		"taiList": [
		  {
			"plmnId": {
			  "mcc": "208",
			  "mnc": "93"
			},
			"tac": "1"
		  }
		]
	  }
	} `

}

func MarshToJsonString(v interface{}) (result []string) {
	types := reflect.TypeOf(v)
	val := reflect.ValueOf(v)
	if types.Kind() == reflect.Slice {
		for i := 0; i < val.Len(); i++ {
			tmp, err := json.Marshal(val.Index(i).Interface())
			if err != nil {
				logger.UtilLog.Errorf("Marshal error: %+v", err)
			}

			result = append(result, string(tmp))
		}
	} else {
		tmp, err := json.Marshal(v)
		if err != nil {
			logger.UtilLog.Errorf("Marshal error: %+v", err)
		}

		result = append(result, string(tmp))
	}
	return
}

func getNfProfile(key string) (models.NfProfile, error) {
	var err error
	var profile models.NfProfile

	nfProfileStr, exists := nfProfilesDb[key]

	if exists {
		err = json.Unmarshal([]byte(nfProfileStr), &profile)
	} else {
		err = fmt.Errorf("failed to find nf profile for %s", key)
	}

	return profile, err
}

func getNfProfiles(targetNfType models.NfType) ([]models.NfProfile, error) {
	var nfProfiles []models.NfProfile

	for key, elem := range nfProfilesDb {
		if strings.Contains(key, string(targetNfType)) {

			var profile models.NfProfile
			err := json.Unmarshal([]byte(elem), &profile)
			if err != nil {
				return nil, err
			}

			nfProfiles = append(nfProfiles, profile)
		}
	}

	return nfProfiles, nil
}

func nrfDbCallback(nrfUri string, targetNfType, requestNfType models.NfType,
	param *Nnrf_NFDiscovery.SearchNFInstancesParamOpts) (models.SearchResult, error) {
	fmt.Println("nrfDbCallback Entry")

	nrfDbCallbackCallCount++

	var searchResult models.SearchResult
	var nfProfile models.NfProfile
	var err error
	var key string

	searchResult.ValidityPeriod = validityPeriod

	if targetNfType == models.NfType_SMF {
		key = "SMF"

		if param != nil {
			if param.Snssais.IsSet() {
				snssais := param.Snssais.Value().([]string)

				var snssai models.Snssai
				err = json.Unmarshal([]byte(snssais[0]), &snssai)
				if err != nil {
					err = fmt.Errorf("snssai invalid %s", snssais[0])
					return searchResult, err
				}

				key += "-" + snssai.Sd
			}
			if param.Dnn.IsSet() == true {
				key += "-" + param.Dnn.Value()
			}

			nfProfile, err = getNfProfile(key)
			if err != nil {
				return searchResult, err
			}

			searchResult.NfInstances = append(searchResult.NfInstances, nfProfile)

		} else {
			searchResult.NfInstances, err = getNfProfiles(targetNfType)
		}
	} else if targetNfType == models.NfType_AUSF {
		searchResult.NfInstances, err = getNfProfiles(targetNfType)
	} else if targetNfType == models.NfType_AMF {
		searchResult.NfInstances, err = getNfProfiles(targetNfType)
	}

	return searchResult, err
}

func TestCacheMissAndHits(t *testing.T) {

	var result models.SearchResult
	var err error

	expectedCallCount := nrfDbCallbackCallCount

	evictionTimerVal := time.Duration(evictionInterval)
	InitNrfCaching(evictionTimerVal*time.Second, nrfDbCallback)

	// Cache Miss for dnn - 'internet'
	param := Nnrf_NFDiscovery.SearchNFInstancesParamOpts{
		ServiceNames: optional.NewInterface([]models.ServiceName{models.ServiceName_NSMF_PDUSESSION}),
		Dnn:          optional.NewString("internet"),
		Snssais:      optional.NewInterface(MarshToJsonString([]models.Snssai{{Sst: 1, Sd: "010203"}})),
	}

	result, err = SearchNFInstances("testNrf", models.NfType_SMF, models.NfType_AMF, &param)
	expectedCallCount++

	if err != nil {
		t.Errorf("test failed, %s", err.Error())
	}
	if len(result.NfInstances) == 0 {
		t.Error("nrf search did not return any records")
	}
	if expectedCallCount != nrfDbCallbackCallCount {
		t.Error("Unexpected nrfDbCallbackCallCount")
	}

	// Cache hit scenario
	result, err = SearchNFInstances("testNrf", models.NfType_SMF, models.NfType_AMF, &param)
	if err != nil {
		t.Errorf("test failed, %s", err.Error())
	}
	if len(result.NfInstances) == 0 {
		t.Error("nrf search did not return any records")
	}
	if expectedCallCount != nrfDbCallbackCallCount {
		t.Error("Unexpected nrfDbCallbackCallCount")
	}

	// Cache Miss for dnn 'ims'
	param = Nnrf_NFDiscovery.SearchNFInstancesParamOpts{
		ServiceNames: optional.NewInterface([]models.ServiceName{models.ServiceName_NSMF_PDUSESSION}),
		Dnn:          optional.NewString("ims"),
		Snssais:      optional.NewInterface(MarshToJsonString([]models.Snssai{{Sst: 1, Sd: "010203"}})),
	}

	result, err = SearchNFInstances("testNrf", models.NfType_SMF, models.NfType_AMF, &param)
	expectedCallCount++

	if err != nil {
		t.Errorf("test failed, %s", err.Error())
	}
	if len(result.NfInstances) == 0 {
		t.Error("nrf search did not return any records")
	}
	if expectedCallCount != nrfDbCallbackCallCount {
		t.Error("Unexpected nrfDbCallbackCallCount")
	}

	// Cache Miss for dnn 'internet' sd '0a0b0c'
	param = Nnrf_NFDiscovery.SearchNFInstancesParamOpts{
		ServiceNames: optional.NewInterface([]models.ServiceName{models.ServiceName_NSMF_PDUSESSION}),
		Dnn:          optional.NewString("internet"),
		Snssais:      optional.NewInterface(MarshToJsonString([]models.Snssai{{Sst: 1, Sd: "0a0b0c"}})),
	}

	result, err = SearchNFInstances("testNrf", models.NfType_SMF, models.NfType_AMF, &param)
	expectedCallCount++

	if err != nil {
		t.Errorf("test failed, %s", err.Error())
	}
	if len(result.NfInstances) == 0 {
		t.Error("nrf search did not return any records")
	}
	if expectedCallCount != nrfDbCallbackCallCount {
		t.Error("Unexpected nrfDbCallbackCallCount")
	}

	disableNrfCaching()
}

func TestCacheMissOnTTlExpiry(t *testing.T) {
	var result models.SearchResult
	var err error

	expectedCallCount := nrfDbCallbackCallCount

	evictionTimerVal := time.Duration(evictionInterval)
	InitNrfCaching(evictionTimerVal*time.Second, nrfDbCallback)

	result, err = SearchNFInstances("testNrf", models.NfType_SMF, models.NfType_AMF, nil)
	expectedCallCount++

	if err != nil {
		t.Errorf("test failed, %s", err.Error())
	}
	if len(result.NfInstances) == 0 {
		t.Error("nrf search did not return any records")
	}
	if expectedCallCount != nrfDbCallbackCallCount {
		t.Errorf("nrfDbCallbackCallCount: expected = %d, current = %d",
			expectedCallCount, nrfDbCallbackCallCount)
	}

	t.Log("wait for profile validity timeout")
	time.Sleep(65 * time.Second)

	// Cache Miss for dnn 'internet' sd '0a0b0c' as ttl expired..
	param := Nnrf_NFDiscovery.SearchNFInstancesParamOpts{
		ServiceNames: optional.NewInterface([]models.ServiceName{models.ServiceName_NSMF_PDUSESSION}),
		Dnn:          optional.NewString("internet"),
		Snssais:      optional.NewInterface(MarshToJsonString([]models.Snssai{{Sst: 1, Sd: "0a0b0c"}})),
	}

	result, err = SearchNFInstances("testNrf", models.NfType_SMF, models.NfType_AMF, &param)
	expectedCallCount++

	if err != nil {
		t.Errorf("test failed, %s", err.Error())
	}
	if len(result.NfInstances) == 0 {
		t.Error("nrf search did not return any records")
	}
	if expectedCallCount != nrfDbCallbackCallCount {
		t.Errorf("nrfDbCallbackCallCount: expected = %d, current = %d",
			expectedCallCount, nrfDbCallbackCallCount)
	}

	result, err = SearchNFInstances("testNrf", models.NfType_SMF, models.NfType_AMF, &param)

	if err != nil {
		t.Errorf("test failed, %s", err.Error())
	}
	if len(result.NfInstances) == 0 {
		t.Error("nrf search did not return any records")
	}
	if expectedCallCount != nrfDbCallbackCallCount {
		t.Errorf("nrfDbCallbackCallCount: expected = %d, current = %d",
			expectedCallCount, nrfDbCallbackCallCount)
	}

	disableNrfCaching()
}

func TestCacheEviction(t *testing.T) {
	var result models.SearchResult
	var err error

	evictionTimerVal := time.Duration(evictionInterval)
	InitNrfCaching(evictionTimerVal*time.Second, nrfDbCallback)

	// Cache Miss for dnn 'internet' sd '0a0b0c' as ttl expired..
	param := Nnrf_NFDiscovery.SearchNFInstancesParamOpts{
		ServiceNames: optional.NewInterface([]models.ServiceName{models.ServiceName_NSMF_PDUSESSION}),
		Dnn:          optional.NewString("internet"),
		Snssais:      optional.NewInterface(MarshToJsonString([]models.Snssai{{Sst: 1, Sd: "010203"}})),
	}

	result, err = SearchNFInstances("testNrf", models.NfType_SMF, models.NfType_AMF, &param)
	if err != nil {
		t.Errorf("test failed, %s", err.Error())
	}

	validityPeriod = 30
	param = Nnrf_NFDiscovery.SearchNFInstancesParamOpts{
		ServiceNames: optional.NewInterface([]models.ServiceName{models.ServiceName_NSMF_PDUSESSION}),
		Dnn:          optional.NewString("ims"),
		Snssais:      optional.NewInterface(MarshToJsonString([]models.Snssai{{Sst: 1, Sd: "010203"}})),
	}

	result, err = SearchNFInstances("testNrf", models.NfType_SMF, models.NfType_AMF, &param)

	if err != nil {
		t.Errorf("test failed, %s", err.Error())
	}

	validityPeriod = 90
	param = Nnrf_NFDiscovery.SearchNFInstancesParamOpts{
		ServiceNames: optional.NewInterface([]models.ServiceName{models.ServiceName_NSMF_PDUSESSION}),
		Dnn:          optional.NewString("internet"),
		Snssais:      optional.NewInterface(MarshToJsonString([]models.Snssai{{Sst: 1, Sd: "0a0b0c"}})),
	}

	result, err = SearchNFInstances("testNrf", models.NfType_SMF, models.NfType_AMF, &param)

	if err != nil {
		t.Errorf("test failed, %s", err.Error())
	}

	if len(result.NfInstances) == 0 {
		t.Errorf("nf instances len 0")
	}

	t.Log("wait for eviction timeout")

	time.Sleep(125 * time.Second)

	disableNrfCaching()
}

func TestCacheConcurrency(t *testing.T) {

	evictionTimerVal := time.Duration(evictionInterval)
	InitNrfCaching(evictionTimerVal*time.Second, nrfDbCallback)

	n := 100
	wg := sync.WaitGroup{}
	wg.Add(n)

	param := Nnrf_NFDiscovery.SearchNFInstancesParamOpts{
		ServiceNames: optional.NewInterface([]models.ServiceName{models.ServiceName_NSMF_PDUSESSION}),
		Dnn:          optional.NewString("internet"),
		Snssais:      optional.NewInterface(MarshToJsonString([]models.Snssai{{Sst: 1, Sd: "010203"}})),
	}

	expectedCallCount := nrfDbCallbackCallCount + 1

	errCh := make(chan error)

	for i := 0; i < n; i++ {
		go func() {
			_, err := SearchNFInstances("testNrf", models.NfType_SMF, models.NfType_AMF, &param)

			if err != nil {
				errCh <- err
			}
			wg.Done()
		}()
	}

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Errorf("test timed out")
	case e := <-errCh:
		t.Errorf("error %s", e.Error())

	}

	if expectedCallCount != nrfDbCallbackCallCount {
		t.Errorf("nrfDbCallbackCallCount: expected = %d, current = %d",
			expectedCallCount, nrfDbCallbackCallCount)
	}

	disableNrfCaching()
}

func TestAusfMatchFilters(t *testing.T) {
	evictionTimerVal := time.Duration(evictionInterval)
	InitNrfCaching(evictionTimerVal*time.Second, nrfDbCallback)

	// Cache Miss for dnn - 'internet'
	param := Nnrf_NFDiscovery.SearchNFInstancesParamOpts{
		Supi: optional.NewString("123456789040001"),
	}

	expectedCallCount := nrfDbCallbackCallCount
	expectedCallCount++

	result, err := SearchNFInstances("testNrf", models.NfType_AUSF, models.NfType_AMF, &param)

	if err != nil {
		t.Errorf("test failed, %s", err.Error())
	}
	if len(result.NfInstances) == 0 {
		t.Error("nrf search did not return any records")
	}
	if expectedCallCount != nrfDbCallbackCallCount {
		t.Error("Unexpected nrfDbCallbackCallCount")
	}

	result, err = SearchNFInstances("testNrf", models.NfType_AUSF, models.NfType_AMF, &param)

	if err != nil {
		t.Errorf("test failed, %s", err.Error())
	}
	if len(result.NfInstances) == 0 {
		t.Error("nrf search did not return any records")
	}
	if expectedCallCount != nrfDbCallbackCallCount {
		t.Error("Unexpected nrfDbCallbackCallCount")
	}

	param = Nnrf_NFDiscovery.SearchNFInstancesParamOpts{
		Supi: optional.NewString("imsi-223456789041111"),
	}

	result, err = SearchNFInstances("testNrf", models.NfType_AUSF, models.NfType_AMF, &param)

	if err != nil {
		t.Errorf("test failed, %s", err.Error())
	}
	if len(result.NfInstances) == 0 {
		t.Error("nrf search did not return any records")
	}
	if expectedCallCount != nrfDbCallbackCallCount {
		t.Error("Unexpected nrfDbCallbackCallCount")
	}
	disableNrfCaching()
}

func TestAmfMatchFilters(t *testing.T) {
	evictionTimerVal := time.Duration(evictionInterval)
	InitNrfCaching(evictionTimerVal*time.Second, nrfDbCallback)

	param := Nnrf_NFDiscovery.SearchNFInstancesParamOpts{
		TargetPlmnList: optional.NewInterface(MarshToJsonString(
			[]models.PlmnId{{Mcc: "208", Mnc: "93"}, {Mcc: "209", Mnc: "94"}})),
	}

	expectedCallCount := nrfDbCallbackCallCount
	expectedCallCount++

	result, err := SearchNFInstances("testNrf", models.NfType_AMF, models.NfType_AMF, &param)

	if err != nil {
		t.Errorf("test failed, %s", err.Error())
	}
	if len(result.NfInstances) == 0 {
		t.Error("nrf search did not return any records")
	}
	if expectedCallCount != nrfDbCallbackCallCount {
		t.Error("Unexpected nrfDbCallbackCallCount")
	}

	param = Nnrf_NFDiscovery.SearchNFInstancesParamOpts{
		TargetPlmnList: optional.NewInterface(MarshToJsonString([]models.PlmnId{{Mcc: "208", Mnc: "93"}})),
		AmfRegionId:    optional.NewString("ca"),
		AmfSetId:       optional.NewString("3f8"),
	}

	result, err = SearchNFInstances("testNrf", models.NfType_AMF, models.NfType_AMF, &param)

	if err != nil {
		t.Errorf("test failed, %s", err.Error())
	}
	if len(result.NfInstances) == 0 {
		t.Error("nrf search did not return any records")
	}
	if expectedCallCount != nrfDbCallbackCallCount {
		t.Error("Unexpected nrfDbCallbackCallCount")
	}

	disableNrfCaching()
}
