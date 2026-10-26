// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package sh

import "github.com/ellanetworks/core/diameter/tgpp"

type DataReference uint32

const (
	DataRepositoryData                    DataReference = 0
	DataIMSPublicIdentity                 DataReference = 10
	DataIMSUserState                      DataReference = 11
	DataSCSCFName                         DataReference = 12
	DataInitialFilterCriteria             DataReference = 13
	DataLocationInformation               DataReference = 14
	DataUserState                         DataReference = 15
	DataChargingInformation               DataReference = 16
	DataMSISDN                            DataReference = 17
	DataPSIActivation                     DataReference = 18
	DataDSAI                              DataReference = 19
	DataServiceLevelTraceInfo             DataReference = 21
	DataIPAddressSecureBindingInformation DataReference = 22
	DataServicePriorityLevel              DataReference = 23
	DataSMSRegistrationInfo               DataReference = 24
	DataUEReachabilityForIP               DataReference = 25
	DataTADSInformation                   DataReference = 26
	DataSTNSR                             DataReference = 27
	DataUESRVCCCapability                 DataReference = 28
	DataExtendedPriority                  DataReference = 29
	DataCSRN                              DataReference = 30
	DataReferenceLocationInformation      DataReference = 31
	DataIMSI                              DataReference = 32
	DataIMSPrivateUserIdentity            DataReference = 33
	DataIMEISV                            DataReference = 34
	DataUE5GSRVCCCapability               DataReference = 35
	DataASRegistrationInfo                DataReference = 36
)

var dataReferenceNames = tgpp.EnumNames{
	DataRepositoryData:                    "RepositoryData",
	DataIMSPublicIdentity:                 "IMSPublicIdentity",
	DataIMSUserState:                      "IMSUserState",
	DataSCSCFName:                         "S-CSCFName",
	DataInitialFilterCriteria:             "InitialFilterCriteria",
	DataLocationInformation:               "LocationInformation",
	DataUserState:                         "UserState",
	DataChargingInformation:               "ChargingInformation",
	DataMSISDN:                            "MSISDN",
	DataPSIActivation:                     "PSIActivation",
	DataDSAI:                              "DSAI",
	DataServiceLevelTraceInfo:             "ServiceLevelTraceInfo",
	DataIPAddressSecureBindingInformation: "IPAddressSecureBindingInformation",
	DataServicePriorityLevel:              "ServicePriorityLevel",
	DataSMSRegistrationInfo:               "SMSRegistrationInfo",
	DataUEReachabilityForIP:               "UEReachabilityForIP",
	DataTADSInformation:                   "TADSinformation",
	DataSTNSR:                             "STN-SR",
	DataUESRVCCCapability:                 "UE-SRVCC-Capability",
	DataExtendedPriority:                  "ExtendedPriority",
	DataCSRN:                              "CSRN",
	DataReferenceLocationInformation:      "ReferenceLocationInformation",
	DataIMSI:                              "IMSI",
	DataIMSPrivateUserIdentity:            "IMSPrivateUserIdentity",
	DataIMEISV:                            "IMEISV",
	DataUE5GSRVCCCapability:               "UE-5G-SRVCC-Capability",
	DataASRegistrationInfo:                "ASRegistrationInfo",
}

func (r DataReference) Valid() bool { return dataReferenceNames.Has(uint32(r)) }

func (r DataReference) String() string { return dataReferenceNames.Name("DataReference", uint32(r)) }

type Features uint32

const (
	FeatureNotifEff         Features = 1 << 0
	FeatureUpdateEff        Features = 1 << 1
	FeatureUpdateEffEnhance Features = 1 << 2
	FeatureAdditionalMSISDN Features = 1 << 3
)

func (f Features) String() string {
	return tgpp.BitNames(uint32(f), "Notif-Eff", "Update-Eff", "Update-Eff-Enhance", "Additional-MSISDN")
}
