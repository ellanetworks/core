package util

import (
	"encoding/json"
	"fmt"
	"reflect"

	"github.com/omec-project/openapi/models"
	"github.com/omec-project/util/path_util"
	"github.com/yeastengine/ella/internal/nssf/db"
	"github.com/yeastengine/ella/internal/nssf/factory"
	"github.com/yeastengine/ella/internal/nssf/logger"
)

// Title in Problem Details for NSSF HTTP APIs
const (
	INVALID_REQUEST       = "Invalid request message framing"
	MALFORMED_REQUEST     = "Malformed request syntax"
	UNAUTHORIZED_CONSUMER = "Unauthorized NF service consumer"
	UNSUPPORTED_RESOURCE  = "Unsupported request resources"
)

// Path of HTTP2 key and log file
var (
	NSSF_LOG_PATH = path_util.Free5gcPath("omec-project/nssfsslkey.log")
	NSSF_PEM_PATH = path_util.Free5gcPath("free5gc/support/TLS/nssf.pem")
	NSSF_KEY_PATH = path_util.Free5gcPath("free5gc/support/TLS/nssf.key")
)

// Default configuration file
var DEFAULT_CONFIG string = "free5gc/config/nssfcfg.yaml"

// Check if a slice contains an element
func Contain(target interface{}, slice interface{}) bool {
	arr := reflect.ValueOf(slice)
	if arr.Kind() == reflect.Slice {
		for i := 0; i < arr.Len(); i++ {
			if reflect.DeepEqual(arr.Index(i).Interface(), target) {
				return true
			}
		}
	}
	return false
}

// Check whether UE's Home PLMN is configured/supported
func CheckSupportedHplmn(homePlmnId models.PlmnId) bool {
	factory.ConfigLock.RLock()
	defer factory.ConfigLock.RUnlock()
	for _, mappingFromPlmn := range factory.NssfConfig.Configuration.MappingListFromPlmn {
		if *mappingFromPlmn.HomePlmnId == homePlmnId {
			return true
		}
	}
	logger.Util.Warnf("No Home PLMN %+v in NSSF configuration", homePlmnId)
	return false
}

// Check whether UE's current TA is configured/supported
func CheckSupportedTa(tai models.Tai) bool {
	factory.ConfigLock.RLock()
	defer factory.ConfigLock.RUnlock()
	for _, taConfig := range factory.NssfConfig.Configuration.TaList {
		if reflect.DeepEqual(*taConfig.Tai, tai) {
			return true
		}
	}
	e, err := json.Marshal(tai)
	if err != nil {
		logger.Util.Errorf("Marshal error in CheckSupportedTa: %+v", err)
	}
	logger.Util.Warnf("No TA %s in NSSF configuration", e)
	return false
}

// Check whether the given S-NSSAI is supported or not in PLMN
func CheckSupportedSnssaiInPlmn(snssai models.Snssai, plmnId models.PlmnId) bool {
	factory.ConfigLock.RLock()
	defer factory.ConfigLock.RUnlock()
	if CheckStandardSnssai(snssai) {
		return true
	}
	supportedNssaiInPlmn, err := db.GetSupportedNssaiInPlmnList()
	if err != nil {
		logger.Util.Errorf("couldn't get supported nssais: %+v", err)
		return false
	}
	for _, supportedNssaiInPlmn := range supportedNssaiInPlmn {
		if *supportedNssaiInPlmn.PlmnId == plmnId {
			for _, supportedSnssai := range supportedNssaiInPlmn.SupportedSnssaiList {
				if snssai == supportedSnssai {
					return true
				}
			}
			return false
		}
	}
	logger.Util.Warnf("No supported S-NSSAI list of PLMNID %+v in NSSF configuration", plmnId)
	return false
}

// Check whether S-NSSAIs in NSSAI are supported or not in PLMN
func CheckSupportedNssaiInPlmn(nssai []models.Snssai, plmnId models.PlmnId) bool {
	supportedNssaiInPlmn, err := db.GetSupportedNssaiInPlmnList()
	if err != nil {
		logger.Util.Errorf("couldn't get supported nssais: %+v", err)
		return false
	}
	for _, supportedNssaiInPlmn := range supportedNssaiInPlmn {
		if *supportedNssaiInPlmn.PlmnId == plmnId {
			for _, snssai := range nssai {
				// Standard S-NSSAIs are supposed to be supported
				// If not, disable following check and be sure to add supported standard S-NSSAI(s) in configuration
				if CheckStandardSnssai(snssai) {
					continue
				}

				hitSupportedNssai := false
				for _, supportedSnssai := range supportedNssaiInPlmn.SupportedSnssaiList {
					if snssai == supportedSnssai {
						hitSupportedNssai = true
						break
					}
				}

				if !hitSupportedNssai {
					return false
				}
			}
			return true
		}
	}
	logger.Util.Warnf("No supported S-NSSAI list of PLMNID %+v in NSSF configuration", plmnId)
	return false
}

// Check whether S-NSSAI is supported or not at UE's current TA
func CheckSupportedSnssaiInTa(snssai models.Snssai, tai models.Tai) bool {
	factory.ConfigLock.RLock()
	defer factory.ConfigLock.RUnlock()
	for _, taConfig := range factory.NssfConfig.Configuration.TaList {
		if reflect.DeepEqual(*taConfig.Tai, tai) {
			for _, supportedSnssai := range taConfig.SupportedSnssaiList {
				if supportedSnssai == snssai {
					return true
				}
			}
			return false
		}
	}
	return false

	// // Check supported S-NSSAI in AmfList instead of TaList
	// for _, amfConfig := range factory.NssfConfig.Configuration.AmfList {
	//     if checkSupportedNssaiAvailabilityData(snssai, tai, amfConfig.SupportedNssaiAvailabilityData) == true {
	//         return true
	//     }
	// }
	// return false
}

// Check whether S-NSSAI is in SupportedNssaiAvailabilityData under the given TAI
func CheckSupportedNssaiAvailabilityData(
	snssai models.Snssai, tai models.Tai, s []models.SupportedNssaiAvailabilityData,
) bool {
	for _, supportedNssaiAvailabilityData := range s {
		if reflect.DeepEqual(*supportedNssaiAvailabilityData.Tai, tai) &&
			CheckSnssaiInNssai(snssai, supportedNssaiAvailabilityData.SupportedSnssaiList) {
			return true
		}
	}
	return false
}

// Check whether S-NSSAI is supported or not by the AMF at UE's current TA
func CheckSupportedSnssaiInAmfTa(snssai models.Snssai, nfId string, tai models.Tai) bool {
	// Uncomment following lines if supported S-NSSAI lists of AMF Sets are independent of those of AMFs
	// for _, amfSetConfig := range factory.NssfConfig.Configuration.AmfSetList {
	//     if amfSetConfig.AmfList != nil && len(amfSetConfig.AmfList) != 0 && Contain(nfId, amfSetConfig.AmfList) {
	//         return checkSupportedNssaiAvailabilityData(snssai, tai, amfSetConfig.SupportedNssaiAvailabilityData)
	//     }
	// }

	for _, amfConfig := range factory.NssfConfig.Configuration.AmfList {
		if amfConfig.NfId == nfId {
			return CheckSupportedNssaiAvailabilityData(snssai, tai, amfConfig.SupportedNssaiAvailabilityData)
		}
	}

	logger.Util.Warnf("No AMF %s in NSSF configuration", nfId)
	return false
}

// Check whether all S-NSSAIs in Allowed NSSAI is supported by the AMF at UE's current TA
func CheckAllowedNssaiInAmfTa(allowedNssaiList []models.AllowedNssai, nfId string, tai models.Tai) bool {
	for _, allowedNssai := range allowedNssaiList {
		for _, allowedSnssai := range allowedNssai.AllowedSnssaiList {
			if CheckSupportedSnssaiInAmfTa(*allowedSnssai.AllowedSnssai, nfId, tai) {
				continue
			} else {
				return false
			}
		}
	}
	return true
}

// Check whether S-NSSAI is standard or non-standard value
// A standard S-NSSAI is only comprised of a standardized SST value and no SD
func CheckStandardSnssai(snssai models.Snssai) bool {
	if snssai.Sst >= 1 && snssai.Sst <= 3 && snssai.Sd == "" {
		return true
	}
	return false
}

// Check whether the NSSAI contains the specific S-NSSAI
func CheckSnssaiInNssai(targetSnssai models.Snssai, nssai []models.Snssai) bool {
	for _, snssai := range nssai {
		if snssai == targetSnssai {
			return true
		}
	}
	return false
}

// Get S-NSSAI mappings of the given Home PLMN ID from configuration
func GetMappingOfPlmnFromConfig(homePlmnId models.PlmnId) []models.MappingOfSnssai {
	factory.ConfigLock.RLock()
	defer factory.ConfigLock.RUnlock()
	for _, mappingFromPlmn := range factory.NssfConfig.Configuration.MappingListFromPlmn {
		if *mappingFromPlmn.HomePlmnId == homePlmnId {
			return mappingFromPlmn.MappingOfSnssai
		}
	}
	return nil
}

// Get NSI information list of the given S-NSSAI from configuration
func GetNsiInformationListFromConfig(snssai models.Snssai) []models.NsiInformation {
	factory.ConfigLock.RLock()
	defer factory.ConfigLock.RUnlock()
	for _, nsiConfig := range factory.NssfConfig.Configuration.NsiList {
		if *nsiConfig.Snssai == snssai {
			return nsiConfig.NsiInformationList
		}
	}
	return nil
}

// Get Access Type of the given TAI from configuraion
func GetAccessTypeFromConfig(tai models.Tai) models.AccessType {
	factory.ConfigLock.RLock()
	defer factory.ConfigLock.RUnlock()
	for _, taConfig := range factory.NssfConfig.Configuration.TaList {
		if reflect.DeepEqual(*taConfig.Tai, tai) {
			return *taConfig.AccessType
		}
	}
	e, err := json.Marshal(tai)
	if err != nil {
		logger.Util.Errorf("Marshal error in GetAccessTypeFromConfig: %+v", err)
	}
	logger.Util.Warnf("No TA %s in NSSF configuration", e)
	return models.AccessType__3_GPP_ACCESS
}

// Get restricted S-NSSAI list of the given TAI from configuration
func GetRestrictedSnssaiListFromConfig(tai models.Tai) []models.RestrictedSnssai {
	factory.ConfigLock.RLock()
	defer factory.ConfigLock.RUnlock()
	for _, taConfig := range factory.NssfConfig.Configuration.TaList {
		if reflect.DeepEqual(*taConfig.Tai, tai) {
			if taConfig.RestrictedSnssaiList != nil && len(taConfig.RestrictedSnssaiList) != 0 {
				return taConfig.RestrictedSnssaiList
			} else {
				return nil
			}
		}
	}
	e, err := json.Marshal(tai)
	if err != nil {
		logger.Util.Errorf("Marshal error in GetRestrictedSnssaiListFromConfig: %+v", err)
	}
	logger.Util.Warnf("No TA %s in NSSF configuration", e)
	return nil
}

// Get authorized NSSAI availability data of the given NF ID and TAI from configuration
func AuthorizeOfAmfTaFromConfig(nfId string, tai models.Tai) (models.AuthorizedNssaiAvailabilityData, error) {
	var authorizedNssaiAvailabilityData models.AuthorizedNssaiAvailabilityData
	authorizedNssaiAvailabilityData.Tai = new(models.Tai)
	*authorizedNssaiAvailabilityData.Tai = tai

	for _, amfConfig := range factory.NssfConfig.Configuration.AmfList {
		if amfConfig.NfId == nfId {
			for _, supportedNssaiAvailabilityData := range amfConfig.SupportedNssaiAvailabilityData {
				if reflect.DeepEqual(*supportedNssaiAvailabilityData.Tai, tai) {
					authorizedNssaiAvailabilityData.SupportedSnssaiList = supportedNssaiAvailabilityData.SupportedSnssaiList
					authorizedNssaiAvailabilityData.RestrictedSnssaiList = GetRestrictedSnssaiListFromConfig(tai)

					// TODO: Sort the returned slice
					return authorizedNssaiAvailabilityData, nil
				}
			}
			e, err1 := json.Marshal(tai)
			if err1 != nil {
				logger.Util.Errorf("Marshal error in AuthorizeOfAmfTaFromConfig: %+v", err1)
			}
			err := fmt.Errorf("No supported S-NSSAI list by AMF %s under TAI %s in NSSF configuration", nfId, e)
			return authorizedNssaiAvailabilityData, err
		}
	}
	err := fmt.Errorf("No AMF configuration of %s", nfId)
	return authorizedNssaiAvailabilityData, err
}

// Get all authorized NSSAI availability data of the given NF ID from configuration
func AuthorizeOfAmfFromConfig(nfId string) ([]models.AuthorizedNssaiAvailabilityData, error) {
	var authorizedNssaiAvailabilityDataList []models.AuthorizedNssaiAvailabilityData

	factory.ConfigLock.RLock()
	defer factory.ConfigLock.RUnlock()
	for _, amfConfig := range factory.NssfConfig.Configuration.AmfList {
		if amfConfig.NfId == nfId {
			for _, supportedNssaiAvailabilityData := range amfConfig.SupportedNssaiAvailabilityData {
				var authorizedNssaiAvailabilityData models.AuthorizedNssaiAvailabilityData
				authorizedNssaiAvailabilityData.Tai = new(models.Tai)
				*authorizedNssaiAvailabilityData.Tai = *supportedNssaiAvailabilityData.Tai
				authorizedNssaiAvailabilityData.SupportedSnssaiList = supportedNssaiAvailabilityData.SupportedSnssaiList
				authorizedNssaiAvailabilityData.RestrictedSnssaiList = GetRestrictedSnssaiListFromConfig(*authorizedNssaiAvailabilityData.Tai)

				authorizedNssaiAvailabilityDataList = append(authorizedNssaiAvailabilityDataList, authorizedNssaiAvailabilityData)
			}
			return authorizedNssaiAvailabilityDataList, nil
		}
	}
	err := fmt.Errorf("No AMF configuration of %s", nfId)
	return authorizedNssaiAvailabilityDataList, err
}

// Get authorized NSSAI availability data of the given TAI list from configuration
func AuthorizeOfTaListFromConfig(taiList []models.Tai) []models.AuthorizedNssaiAvailabilityData {
	var authorizedNssaiAvailabilityDataList []models.AuthorizedNssaiAvailabilityData

	for _, taConfig := range factory.NssfConfig.Configuration.TaList {
		for _, tai := range taiList {
			if reflect.DeepEqual(*taConfig.Tai, tai) {
				var authorizedNssaiAvailabilityData models.AuthorizedNssaiAvailabilityData
				authorizedNssaiAvailabilityData.Tai = new(models.Tai)
				*authorizedNssaiAvailabilityData.Tai = tai
				authorizedNssaiAvailabilityData.SupportedSnssaiList = taConfig.SupportedSnssaiList
				authorizedNssaiAvailabilityData.RestrictedSnssaiList = GetRestrictedSnssaiListFromConfig(tai)

				authorizedNssaiAvailabilityDataList = append(authorizedNssaiAvailabilityDataList, authorizedNssaiAvailabilityData)
			}
		}
	}
	return authorizedNssaiAvailabilityDataList
}

// Get supported S-NSSAI list of the given NF ID and TAI from configuration
func GetSupportedSnssaiListFromConfig(nfId string, tai models.Tai) []models.Snssai {
	for _, amfConfig := range factory.NssfConfig.Configuration.AmfList {
		if amfConfig.NfId == nfId {
			for _, supportedNssaiAvailabilityData := range amfConfig.SupportedNssaiAvailabilityData {
				if reflect.DeepEqual(*supportedNssaiAvailabilityData.Tai, tai) {
					return supportedNssaiAvailabilityData.SupportedSnssaiList
				}
			}
			return nil
		}
	}
	return nil
}

// Find target S-NSSAI mapping with serving S-NSSAIs from mapping of S-NSSAI(s)
func FindMappingWithServingSnssai(
	snssai models.Snssai, mappings []models.MappingOfSnssai,
) (models.MappingOfSnssai, bool) {
	for _, mapping := range mappings {
		if *mapping.ServingSnssai == snssai {
			return mapping, true
		}
	}
	return models.MappingOfSnssai{}, false
}

// Find target S-NSSAI mapping with home S-NSSAIs from mapping of S-NSSAI(s)
func FindMappingWithHomeSnssai(snssai models.Snssai, mappings []models.MappingOfSnssai) (models.MappingOfSnssai, bool) {
	for _, mapping := range mappings {
		if *mapping.HomeSnssai == snssai {
			return mapping, true
		}
	}
	return models.MappingOfSnssai{}, false
}

// Add Allowed S-NSSAI to Authorized Network Slice Info
func AddAllowedSnssai(allowedSnssai models.AllowedSnssai, accessType models.AccessType,
	authorizedNetworkSliceInfo *models.AuthorizedNetworkSliceInfo,
) {
	hitAllowedNssai := false
	allowedNssaiNum := 8
	for i := range authorizedNetworkSliceInfo.AllowedNssaiList {
		if authorizedNetworkSliceInfo.AllowedNssaiList[i].AccessType == accessType {
			hitAllowedNssai = true
			if len(authorizedNetworkSliceInfo.AllowedNssaiList[i].AllowedSnssaiList) == allowedNssaiNum {
				logger.Util.Infof("Unable to add a new Allowed S-NSSAI since already eight S-NSSAIs in Allowed NSSAI")
			} else {
				authorizedNetworkSliceInfo.AllowedNssaiList[i].AllowedSnssaiList = append(authorizedNetworkSliceInfo.AllowedNssaiList[i].AllowedSnssaiList, allowedSnssai)
			}
			break
		}
	}

	if !hitAllowedNssai {
		var allowedNssaiElement models.AllowedNssai
		allowedNssaiElement.AllowedSnssaiList = append(allowedNssaiElement.AllowedSnssaiList, allowedSnssai)
		allowedNssaiElement.AccessType = accessType

		authorizedNetworkSliceInfo.AllowedNssaiList = append(authorizedNetworkSliceInfo.AllowedNssaiList, allowedNssaiElement)
	}
}

// Add AMF information to Authorized Network Slice Info
func AddAmfInformation(tai models.Tai, authorizedNetworkSliceInfo *models.AuthorizedNetworkSliceInfo) {
	factory.ConfigLock.RLock()
	defer factory.ConfigLock.RUnlock()
	if authorizedNetworkSliceInfo.AllowedNssaiList == nil || len(authorizedNetworkSliceInfo.AllowedNssaiList) == 0 {
		return
	}

	// Check if any AMF can serve the UE
	// That is, whether NSSAI of all Allowed S-NSSAIs is a subset of NSSAI supported by AMF

	// Find AMF Set that could serve UE from AMF Set list in configuration
	// Simply use the first applicable AMF set
	// TODO: Policies of AMF selection (e.g. load balance between AMF instances)
	for _, amfSetConfig := range factory.NssfConfig.Configuration.AmfSetList {
		hitAllowedNssai := true
		for _, allowedNssai := range authorizedNetworkSliceInfo.AllowedNssaiList {
			for _, allowedSnssai := range allowedNssai.AllowedSnssaiList {
				if CheckSupportedNssaiAvailabilityData(*allowedSnssai.AllowedSnssai,
					tai, amfSetConfig.SupportedNssaiAvailabilityData) {
					continue
				} else {
					hitAllowedNssai = false
					break
				}
			}
			if !hitAllowedNssai {
				break
			}
		}

		if !hitAllowedNssai {
			continue
		} else {
			// Add AMF Set to Authorized Network Slice Info
			if amfSetConfig.AmfList != nil && len(amfSetConfig.AmfList) != 0 {
				// List of candidate AMF(s) provided in configuration
				authorizedNetworkSliceInfo.CandidateAmfList = append(authorizedNetworkSliceInfo.CandidateAmfList, amfSetConfig.AmfList...)
			} else {
				// TODO: Possibly querying the NRF
				authorizedNetworkSliceInfo.TargetAmfSet = amfSetConfig.AmfSetId
			}
			return
		}
	}

	// No AMF Set in configuration can serve the UE
	// Find all candidate AMFs that could serve UE from AMF list in configuration
	hitAmf := false
	for _, amfConfig := range factory.NssfConfig.Configuration.AmfList {
		hitAllowedNssai := true
		for _, allowedNssai := range authorizedNetworkSliceInfo.AllowedNssaiList {
			for _, allowedSnssai := range allowedNssai.AllowedSnssaiList {
				if CheckSupportedNssaiAvailabilityData(*allowedSnssai.AllowedSnssai,
					tai, amfConfig.SupportedNssaiAvailabilityData) {
					continue
				} else {
					hitAllowedNssai = false
					break
				}
			}
			if !hitAllowedNssai {
				break
			}
		}

		if !hitAllowedNssai {
			continue
		} else {
			// Add AMF Set to Authorized Network Slice Info
			authorizedNetworkSliceInfo.CandidateAmfList = append(authorizedNetworkSliceInfo.CandidateAmfList, amfConfig.NfId)
			hitAmf = true
		}
	}

	if !hitAmf {
		logger.Util.Warnf("No candidate AMF or AMF Set can serve the UE")
	}
}
