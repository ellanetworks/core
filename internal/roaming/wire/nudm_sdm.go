// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package wire

import (
	"bytes"
	"encoding/json"
	"regexp"
)

const (
	ServiceNudmSDM   = "nudm-sdm"
	NudmSDMAPIRoot   = "/nudm-sdm/v2"
	SDMAmData        = "am-data"
	SDMSmfSelectData = "smf-select-data"
	SDMSmData        = "sm-data"
	SDMSubscriptions = "sdm-subscriptions"
	SDMSorAck        = "am-data/sor-ack"
)

var (
	sharedDataIDPattern = regexp.MustCompile(`^[0-9]{5,6}-.+$`)
	sorMacPattern       = regexp.MustCompile(`^[A-Fa-f0-9]{32}$`)
	counterSorPattern   = regexp.MustCompile(`^[A-Fa-f0-9]{4}$`)
)

type DataSetName string

const (
	DataSetAM         DataSetName = "AM"
	DataSetSMFSel     DataSetName = "SMF_SEL"
	DataSetUECSMF     DataSetName = "UEC_SMF"
	DataSetUECSMSF    DataSetName = "UEC_SMSF"
	DataSetSMSSub     DataSetName = "SMS_SUB"
	DataSetSM         DataSetName = "SM"
	DataSetTrace      DataSetName = "TRACE"
	DataSetSMSMng     DataSetName = "SMS_MNG"
	DataSetLCSPrivacy DataSetName = "LCS_PRIVACY"
	DataSetLCSMO      DataSetName = "LCS_MO"
	DataSetLCSSub     DataSetName = "LCS_SUB"
	DataSetUECAMF     DataSetName = "UEC_AMF"
	DataSetV2X        DataSetName = "V2X"
	DataSetLCSBCA     DataSetName = "LCS_BCA"
	DataSetProSe      DataSetName = "PROSE"
	DataSetUC         DataSetName = "UC"
	DataSetMBS        DataSetName = "MBS"
	DataSetA2X        DataSetName = "A2X"
	DataSetRangingSL  DataSetName = "RANGINGSL_PRIVACY"
)

func (d DataSetName) Known() bool {
	return oneOf(d, DataSetAM, DataSetSMFSel, DataSetUECSMF, DataSetUECSMSF, DataSetSMSSub, DataSetSM, DataSetTrace,
		DataSetSMSMng, DataSetLCSPrivacy, DataSetLCSMO, DataSetLCSSub, DataSetUECAMF, DataSetV2X, DataSetLCSBCA,
		DataSetProSe, DataSetUC, DataSetMBS, DataSetA2X, DataSetRangingSL)
}

type CoreNetworkType string

const (
	CoreNetwork5GC CoreNetworkType = "5GC"
	CoreNetworkEPC CoreNetworkType = "EPC"
)

func (c CoreNetworkType) Known() bool {
	return oneOf(c, CoreNetwork5GC, CoreNetworkEPC)
}

type PduSessionType string

const (
	PduSessionIPv4         PduSessionType = "IPV4"
	PduSessionIPv6         PduSessionType = "IPV6"
	PduSessionIPv4v6       PduSessionType = "IPV4V6"
	PduSessionUnstructured PduSessionType = "UNSTRUCTURED"
	PduSessionEthernet     PduSessionType = "ETHERNET"
)

func (t PduSessionType) Known() bool {
	return oneOf(t, PduSessionIPv4, PduSessionIPv6, PduSessionIPv4v6, PduSessionUnstructured, PduSessionEthernet)
}

type SscMode string

const (
	SscMode1 SscMode = "SSC_MODE_1"
	SscMode2 SscMode = "SSC_MODE_2"
	SscMode3 SscMode = "SSC_MODE_3"
)

func (m SscMode) Known() bool {
	return oneOf(m, SscMode1, SscMode2, SscMode3)
}

type PreemptionCapability string

const (
	NotPreempt PreemptionCapability = "NOT_PREEMPT"
	MayPreempt PreemptionCapability = "MAY_PREEMPT"
)

func (c PreemptionCapability) Known() bool {
	return oneOf(c, NotPreempt, MayPreempt)
}

type PreemptionVulnerability string

const (
	NotPreemptable PreemptionVulnerability = "NOT_PREEMPTABLE"
	Preemptable    PreemptionVulnerability = "PREEMPTABLE"
)

func (v PreemptionVulnerability) Known() bool {
	return oneOf(v, NotPreemptable, Preemptable)
}

type AccessTech string

const (
	AccessTechNR                  AccessTech = "NR"
	AccessTechEUTRANWBS1AndNBS1   AccessTech = "EUTRAN_IN_WBS1_MODE_AND_NBS1_MODE"
	AccessTechEUTRANNBS1Only      AccessTech = "EUTRAN_IN_NBS1_MODE_ONLY"
	AccessTechEUTRANWBS1Only      AccessTech = "EUTRAN_IN_WBS1_MODE_ONLY"
	AccessTechUTRAN               AccessTech = "UTRAN"
	AccessTechGSMAndECGSMIoT      AccessTech = "GSM_AND_ECGSM_IoT"
	AccessTechGSMWithoutECGSMIoT  AccessTech = "GSM_WITHOUT_ECGSM_IoT"
	AccessTechECGSMIoTOnly        AccessTech = "ECGSM_IoT_ONLY"
	AccessTechCDMA1xRTT           AccessTech = "CDMA_1xRTT"
	AccessTechCDMAHRPD            AccessTech = "CDMA_HRPD"
	AccessTechGSMCompact          AccessTech = "GSM_COMPACT"
	AccessTechSatelliteNGRAN      AccessTech = "SATELLITE_NG_RAN"
	AccessTechSatelliteEUTRANNBS1 AccessTech = "SATELLITE_EUTRAN_IN_NBS1_MODE"
	AccessTechSatelliteEUTRANWBS1 AccessTech = "SATELLITE_EUTRAN_IN_WBS1_MODE"
)

func (a AccessTech) Known() bool {
	return oneOf(a, AccessTechNR, AccessTechEUTRANWBS1AndNBS1, AccessTechEUTRANNBS1Only, AccessTechEUTRANWBS1Only,
		AccessTechUTRAN, AccessTechGSMAndECGSMIoT, AccessTechGSMWithoutECGSMIoT, AccessTechECGSMIoTOnly,
		AccessTechCDMA1xRTT, AccessTechCDMAHRPD, AccessTechGSMCompact, AccessTechSatelliteNGRAN,
		AccessTechSatelliteEUTRANNBS1, AccessTechSatelliteEUTRANWBS1)
}

type Nssai struct {
	SupportedFeatures   SupportedFeatures `json:"supportedFeatures,omitempty"`
	DefaultSingleNssais []Snssai          `json:"defaultSingleNssais"`
	SingleNssais        []Snssai          `json:"singleNssais,omitempty"`
	ProvisioningTime    DateTime          `json:"provisioningTime,omitempty"`
}

func (n Nssai) Check(p Policy) error {
	if len(n.DefaultSingleNssais) == 0 {
		return missing("/defaultSingleNssais")
	}

	if err := firstError(checkAll("/defaultSingleNssais", n.DefaultSingleNssais, p), checkAll("/singleNssais", n.SingleNssais, p)); err != nil {
		return err
	}

	if p != Strict {
		return nil
	}

	return firstError(
		Nest("/supportedFeatures", n.SupportedFeatures.Check(p)),
		checkDateTime("/provisioningTime", n.ProvisioningTime, true),
	)
}

type SteeringInfo struct {
	PlmnID         PlmnID       `json:"plmnId"`
	AccessTechList []AccessTech `json:"accessTechList,omitempty"`
}

func (s SteeringInfo) Check(p Policy) error {
	if err := s.PlmnID.Check(p); err != nil {
		return Nest("/plmnId", err)
	}

	return checkEnums("/accessTechList", s.AccessTechList, p)
}

type SteeringContainer struct {
	SteeringInfo  []SteeringInfo
	SecuredPacket []byte
}

func (c SteeringContainer) MarshalJSON() ([]byte, error) {
	if c.SecuredPacket != nil {
		return json.Marshal(c.SecuredPacket)
	}

	return json.Marshal(c.SteeringInfo)
}

func (c *SteeringContainer) UnmarshalJSON(data []byte) error {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) > 0 && trimmed[0] == '[' {
		var list []SteeringInfo
		if err := json.Unmarshal(trimmed, &list); err != nil {
			return err
		}

		*c = SteeringContainer{SteeringInfo: list}

		return nil
	}

	var packet []byte
	if err := json.Unmarshal(trimmed, &packet); err != nil {
		return err
	}

	*c = SteeringContainer{SecuredPacket: packet}

	return nil
}

func (c SteeringContainer) Check(p Policy) error {
	if c.SecuredPacket == nil && len(c.SteeringInfo) == 0 {
		return incorrect("", true, "empty steering container")
	}

	return checkAll("", c.SteeringInfo, p)
}

type SorInfo struct {
	SteeringContainer       *SteeringContainer `json:"steeringContainer,omitempty"`
	AckInd                  bool               `json:"ackInd"`
	SorMacIausf             string             `json:"sorMacIausf,omitempty"`
	Countersor              string             `json:"countersor,omitempty"`
	ProvisioningTime        DateTime           `json:"provisioningTime"`
	SorTransparentContainer []byte             `json:"sorTransparentContainer,omitempty"`
	SorCmci                 []byte             `json:"sorCmci,omitempty"`
	StoreSorCmciInMe        *bool              `json:"storeSorCmciInMe,omitempty"`
	UsimSupportOfSorCmci    *bool              `json:"usimSupportOfSorCmci,omitempty"`
}

func (s *SorInfo) UnmarshalJSON(data []byte) error {
	if err := requireKeys(data, "ackInd", "provisioningTime"); err != nil {
		return err
	}

	type plain SorInfo

	return json.Unmarshal(data, (*plain)(s))
}

func (s SorInfo) Check(p Policy) error {
	if err := firstError(
		checkPattern("/sorMacIausf", s.SorMacIausf, sorMacPattern, true),
		checkPattern("/countersor", s.Countersor, counterSorPattern, true),
		checkOptional("/steeringContainer", s.SteeringContainer, p),
	); err != nil {
		return err
	}

	if p != Strict {
		return nil
	}

	return checkDateTime("/provisioningTime", s.ProvisioningTime, false)
}

type AcknowledgeInfo struct {
	SorMacIue               string   `json:"sorMacIue,omitempty"`
	UpuMacIue               string   `json:"upuMacIue,omitempty"`
	ProvisioningTime        DateTime `json:"provisioningTime"`
	SorTransparentContainer []byte   `json:"sorTransparentContainer,omitempty"`
	UeNotReachable          bool     `json:"ueNotReachable,omitempty"`
	UpuTransparentContainer []byte   `json:"upuTransparentContainer,omitempty"`
}

func (a AcknowledgeInfo) Check(Policy) error {
	return firstError(
		checkDateTime("/provisioningTime", a.ProvisioningTime, false),
		checkPattern("/sorMacIue", a.SorMacIue, sorMacPattern, true),
		checkPattern("/upuMacIue", a.UpuMacIue, sorMacPattern, true),
	)
}

type AccessAndMobilitySubscriptionData struct {
	SupportedFeatures           SupportedFeatures `json:"supportedFeatures,omitempty"`
	Gpsis                       []string          `json:"gpsis,omitempty"`
	SubscribedUeAmbr            *Ambr             `json:"subscribedUeAmbr,omitempty"`
	Nssai                       *Nssai            `json:"nssai,omitempty"`
	RatRestrictions             []RatType         `json:"ratRestrictions,omitempty"`
	CoreNetworkTypeRestrictions []CoreNetworkType `json:"coreNetworkTypeRestrictions,omitempty"`
	RfspIndex                   *int              `json:"rfspIndex,omitempty"`
	SubsRegTimer                *int              `json:"subsRegTimer,omitempty"`
	UeUsageType                 *int              `json:"ueUsageType,omitempty"`
	MicoAllowed                 bool              `json:"micoAllowed,omitempty"`
	SorInfo                     *SorInfo          `json:"sorInfo,omitempty"`
	SorInfoExpectInd            bool              `json:"sorInfoExpectInd,omitempty"`
	SharedAmDataIDs             []string          `json:"sharedAmDataIds,omitempty"`
}

func (a AccessAndMobilitySubscriptionData) Check(p Policy) error {
	if err := firstError(
		checkOptional("/subscribedUeAmbr", a.SubscribedUeAmbr, p),
		checkOptional("/nssai", a.Nssai, p),
		checkOptional("/sorInfo", a.SorInfo, p),
	); err != nil {
		return err
	}

	if p != Strict {
		return nil
	}

	for i, g := range a.Gpsis {
		if !ValidGpsi(g) {
			return NestIndex("/gpsis", i, incorrect("", true, "invalid GPSI %q", g))
		}
	}

	if a.RfspIndex != nil {
		if err := checkRange("/rfspIndex", *a.RfspIndex, 1, 256, true); err != nil {
			return err
		}
	}

	for i, id := range a.SharedAmDataIDs {
		if !sharedDataIDPattern.MatchString(id) {
			return NestIndex("/sharedAmDataIds", i, incorrect("", true, "invalid shared data ID %q", id))
		}
	}

	return firstError(
		Nest("/supportedFeatures", a.SupportedFeatures.Check(p)),
		checkEnums("/ratRestrictions", a.RatRestrictions, p),
		checkEnums("/coreNetworkTypeRestrictions", a.CoreNetworkTypeRestrictions, p),
	)
}

type DnnInfo struct {
	Dnn                 string `json:"dnn"`
	DefaultDnnIndicator bool   `json:"defaultDnnIndicator,omitempty"`
	LboRoamingAllowed   bool   `json:"lboRoamingAllowed,omitempty"`
	IwkEpsInd           bool   `json:"iwkEpsInd,omitempty"`
	DnnBarred           bool   `json:"dnnBarred,omitempty"`
	InvokeNefInd        bool   `json:"invokeNefInd,omitempty"`
	SameSmfInd          bool   `json:"sameSmfInd,omitempty"`
	HrSboAllowed        bool   `json:"hrSboAllowed,omitempty"`
}

const WildcardDnn = "*"

func (d DnnInfo) Check(Policy) error {
	if d.Dnn == "" {
		return missing("/dnn")
	}

	if d.Dnn != WildcardDnn && !ValidDnn(d.Dnn) {
		return incorrect("/dnn", false, "invalid DNN %q", d.Dnn)
	}

	return nil
}

type SnssaiInfo struct {
	DnnInfos []DnnInfo `json:"dnnInfos"`
}

func (s SnssaiInfo) Check(p Policy) error {
	if p == Strict && len(s.DnnInfos) == 0 {
		return missing("/dnnInfos")
	}

	return checkAll("/dnnInfos", s.DnnInfos, p)
}

func (s SnssaiInfo) DefaultDnn() (DnnInfo, bool) {
	for _, d := range s.DnnInfos {
		if d.DefaultDnnIndicator {
			return d, true
		}
	}

	return DnnInfo{}, false
}

func (s SnssaiInfo) Dnn(dnn string) (DnnInfo, bool) {
	for _, d := range s.DnnInfos {
		if equalDnn(d.Dnn, dnn) {
			return d, true
		}
	}

	for _, d := range s.DnnInfos {
		if d.Dnn == WildcardDnn {
			return d, true
		}
	}

	return DnnInfo{}, false
}

type SmfSelectionSubscriptionData struct {
	SupportedFeatures     SupportedFeatures     `json:"supportedFeatures,omitempty"`
	SubscribedSnssaiInfos map[string]SnssaiInfo `json:"subscribedSnssaiInfos,omitempty"`
	SharedSnssaiInfosID   string                `json:"sharedSnssaiInfosId,omitempty"`
}

func (s SmfSelectionSubscriptionData) Check(p Policy) error {
	for _, key := range slicesSorted(s.SubscribedSnssaiInfos) {
		if _, err := ParseSnssai(key); err != nil {
			return incorrect("/subscribedSnssaiInfos/"+key, true, "key is not <sst>[-<sd>]")
		}

		if err := s.SubscribedSnssaiInfos[key].Check(p); err != nil {
			return Nest("/subscribedSnssaiInfos/"+key, err)
		}
	}

	if p != Strict {
		return nil
	}

	return firstError(
		Nest("/supportedFeatures", s.SupportedFeatures.Check(p)),
		checkPattern("/sharedSnssaiInfosId", s.SharedSnssaiInfosID, sharedDataIDPattern, true),
	)
}

func (s SmfSelectionSubscriptionData) SnssaiInfo(snssai Snssai) (SnssaiInfo, bool) {
	for key, info := range s.SubscribedSnssaiInfos {
		if k, err := ParseSnssai(key); err == nil && k.Equal(snssai) {
			return info, true
		}
	}

	return SnssaiInfo{}, false
}

type Arp struct {
	PriorityLevel int                     `json:"priorityLevel"`
	PreemptCap    PreemptionCapability    `json:"preemptCap"`
	PreemptVuln   PreemptionVulnerability `json:"preemptVuln"`
}

func (a Arp) Check(p Policy) error {
	return firstError(
		checkRange("/priorityLevel", a.PriorityLevel, 1, 15, false),
		checkEnum("/preemptCap", a.PreemptCap, p, false),
		checkEnum("/preemptVuln", a.PreemptVuln, p, false),
	)
}

type SubscribedDefaultQos struct {
	FiveQi        int  `json:"5qi"`
	Arp           Arp  `json:"arp"`
	PriorityLevel *int `json:"priorityLevel,omitempty"`
}

func (q *SubscribedDefaultQos) UnmarshalJSON(data []byte) error {
	if err := requireKeys(data, "5qi", "arp"); err != nil {
		return err
	}

	type plain SubscribedDefaultQos

	return json.Unmarshal(data, (*plain)(q))
}

func (q SubscribedDefaultQos) Check(p Policy) error {
	if err := checkRange("/5qi", q.FiveQi, 0, 255, false); err != nil {
		return err
	}

	if err := q.Arp.Check(p); err != nil {
		return Nest("/arp", err)
	}

	if q.PriorityLevel != nil {
		return checkRange("/priorityLevel", *q.PriorityLevel, 1, 127, true)
	}

	return nil
}

type PduSessionTypes struct {
	DefaultSessionType  PduSessionType   `json:"defaultSessionType,omitempty"`
	AllowedSessionTypes []PduSessionType `json:"allowedSessionTypes,omitempty"`
}

func (t PduSessionTypes) Check(p Policy) error {
	return firstError(
		checkEnum("/defaultSessionType", t.DefaultSessionType, p, true),
		checkEnums("/allowedSessionTypes", t.AllowedSessionTypes, p),
	)
}

type SscModes struct {
	DefaultSscMode  SscMode   `json:"defaultSscMode"`
	AllowedSscModes []SscMode `json:"allowedSscModes,omitempty"`
}

func (m SscModes) Check(p Policy) error {
	if err := checkEnum("/defaultSscMode", m.DefaultSscMode, p, false); err != nil {
		return err
	}

	if p == Strict && len(m.AllowedSscModes) > 2 {
		return incorrect("/allowedSscModes", true, "more than 2 items")
	}

	return checkEnums("/allowedSscModes", m.AllowedSscModes, p)
}

type DnnConfiguration struct {
	PduSessionTypes  PduSessionTypes       `json:"pduSessionTypes"`
	SscModes         SscModes              `json:"sscModes"`
	IwkEpsInd        bool                  `json:"iwkEpsInd,omitempty"`
	FiveGQosProfile  *SubscribedDefaultQos `json:"5gQosProfile,omitempty"`
	SessionAmbr      *Ambr                 `json:"sessionAmbr,omitempty"`
	ChargingChars    string                `json:"3gppChargingCharacteristics,omitempty"`
	SecondaryAuth    *bool                 `json:"secondaryAuth,omitempty"`
	AtsssAllowed     bool                  `json:"atsssAllowed,omitempty"`
	RedundantSession *bool                 `json:"redundantSessionAllowed,omitempty"`
}

func (d *DnnConfiguration) UnmarshalJSON(data []byte) error {
	if err := requireKeys(data, "pduSessionTypes", "sscModes"); err != nil {
		return err
	}

	type plain DnnConfiguration

	return json.Unmarshal(data, (*plain)(d))
}

func (d DnnConfiguration) Check(p Policy) error {
	if err := d.PduSessionTypes.Check(p); err != nil {
		return Nest("/pduSessionTypes", err)
	}

	if err := d.SscModes.Check(p); err != nil {
		return Nest("/sscModes", err)
	}

	if err := checkOptional("/sessionAmbr", d.SessionAmbr, p); err != nil {
		return err
	}

	if p != Strict {
		return nil
	}

	return checkOptional("/5gQosProfile", d.FiveGQosProfile, p)
}

type SessionManagementSubscriptionData struct {
	SingleNssai               Snssai                      `json:"singleNssai"`
	DnnConfigurations         map[string]DnnConfiguration `json:"dnnConfigurations,omitempty"`
	SharedDnnConfigurationsID string                      `json:"sharedDnnConfigurationsId,omitempty"`
	SupportedFeatures         SupportedFeatures           `json:"supportedFeatures,omitempty"`
}

func (s *SessionManagementSubscriptionData) UnmarshalJSON(data []byte) error {
	if err := requireKeys(data, "singleNssai"); err != nil {
		return err
	}

	type plain SessionManagementSubscriptionData

	return json.Unmarshal(data, (*plain)(s))
}

func (s SessionManagementSubscriptionData) Check(p Policy) error {
	if err := s.SingleNssai.Check(p); err != nil {
		return Nest("/singleNssai", err)
	}

	for _, dnn := range slicesSorted(s.DnnConfigurations) {
		if dnn != WildcardDnn && !ValidDnn(dnn) {
			return incorrect("/dnnConfigurations/"+dnn, true, "key is not a DNN")
		}

		if err := s.DnnConfigurations[dnn].Check(p); err != nil {
			return Nest("/dnnConfigurations/"+dnn, err)
		}
	}

	if p != Strict {
		return nil
	}

	return firstError(
		Nest("/supportedFeatures", s.SupportedFeatures.Check(p)),
		checkPattern("/sharedDnnConfigurationsId", s.SharedDnnConfigurationsID, sharedDataIDPattern, true),
	)
}

func (s SessionManagementSubscriptionData) DnnConfiguration(dnn string) (DnnConfiguration, bool) {
	for key, cfg := range s.DnnConfigurations {
		if equalDnn(key, dnn) {
			return cfg, true
		}
	}

	cfg, ok := s.DnnConfigurations[WildcardDnn]

	return cfg, ok
}

type SmSubsData struct {
	Individual          []SessionManagementSubscriptionData
	SharedSmSubsDataIDs []string
}

type extendedSmSubsData struct {
	SharedSmSubsDataIDs  []string                            `json:"sharedSmSubsDataIds"`
	IndividualSmSubsData []SessionManagementSubscriptionData `json:"individualSmSubsData,omitempty"`
}

func (d SmSubsData) MarshalJSON() ([]byte, error) {
	if len(d.SharedSmSubsDataIDs) == 0 {
		if d.Individual == nil {
			return []byte("[]"), nil
		}

		return json.Marshal(d.Individual)
	}

	return json.Marshal(extendedSmSubsData{SharedSmSubsDataIDs: d.SharedSmSubsDataIDs, IndividualSmSubsData: d.Individual})
}

func (d *SmSubsData) UnmarshalJSON(data []byte) error {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) > 0 && trimmed[0] == '{' {
		if err := requireKeys(trimmed, "sharedSmSubsDataIds"); err != nil {
			return err
		}

		var ext extendedSmSubsData
		if err := json.Unmarshal(trimmed, &ext); err != nil {
			return err
		}

		*d = SmSubsData{Individual: ext.IndividualSmSubsData, SharedSmSubsDataIDs: ext.SharedSmSubsDataIDs}

		return nil
	}

	var list []SessionManagementSubscriptionData
	if err := json.Unmarshal(trimmed, &list); err != nil {
		return err
	}

	*d = SmSubsData{Individual: list}

	return nil
}

func (d SmSubsData) Check(p Policy) error {
	for i, id := range d.SharedSmSubsDataIDs {
		if !sharedDataIDPattern.MatchString(id) {
			return NestIndex("/sharedSmSubsDataIds", i, incorrect("", false, "invalid shared data ID %q", id))
		}
	}

	return checkAll("", d.Individual, p)
}

func (d SmSubsData) Find(snssai Snssai, dnn string) (SessionManagementSubscriptionData, DnnConfiguration, bool) {
	for _, s := range d.Individual {
		if !s.SingleNssai.Equal(snssai) {
			continue
		}

		if cfg, ok := s.DnnConfiguration(dnn); ok {
			return s, cfg, true
		}
	}

	return SessionManagementSubscriptionData{}, DnnConfiguration{}, false
}

type SubscriptionDataSets struct {
	AmData     *AccessAndMobilitySubscriptionData `json:"amData,omitempty"`
	SmfSelData *SmfSelectionSubscriptionData      `json:"smfSelData,omitempty"`
	SmData     *SmSubsData                        `json:"smData,omitempty"`
}

func (s SubscriptionDataSets) Check(p Policy) error {
	return firstError(
		checkOptional("/amData", s.AmData, p),
		checkOptional("/smfSelData", s.SmfSelData, p),
		checkOptional("/smData", s.SmData, p),
	)
}

type SdmSubscription struct {
	NfInstanceID          string            `json:"nfInstanceId"`
	ImplicitUnsubscribe   bool              `json:"implicitUnsubscribe,omitempty"`
	Expires               DateTime          `json:"expires,omitempty"`
	CallbackReference     string            `json:"callbackReference"`
	MonitoredResourceUris []string          `json:"monitoredResourceUris"`
	SingleNssai           *Snssai           `json:"singleNssai,omitempty"`
	Dnn                   string            `json:"dnn,omitempty"`
	SubscriptionID        string            `json:"subscriptionId,omitempty"`
	PlmnID                *PlmnID           `json:"plmnId,omitempty"`
	ImmediateReport       bool              `json:"immediateReport,omitempty"`
	SupportedFeatures     SupportedFeatures `json:"supportedFeatures,omitempty"`
}

func (s SdmSubscription) Check(p Policy) error {
	if err := checkDateTime("/expires", s.Expires, true); err != nil {
		return err
	}

	if p != Strict {
		return nil
	}

	if len(s.MonitoredResourceUris) == 0 {
		return missing("/monitoredResourceUris")
	}

	for i, u := range s.MonitoredResourceUris {
		if u == "" {
			return NestIndex("/monitoredResourceUris", i, missing(""))
		}
	}

	return firstError(
		checkString("/nfInstanceId", s.NfInstanceID, ValidNfInstanceID, false),
		checkAbsoluteURI("/callbackReference", s.CallbackReference, false),
		checkOptional("/singleNssai", s.SingleNssai, p),
		checkString("/dnn", s.Dnn, ValidDnn, true),
		checkOptional("/plmnId", s.PlmnID, p),
		Nest("/supportedFeatures", s.SupportedFeatures.Check(p)),
	)
}

type ChangeType string

const (
	ChangeAdd     ChangeType = "ADD"
	ChangeMove    ChangeType = "MOVE"
	ChangeRemove  ChangeType = "REMOVE"
	ChangeReplace ChangeType = "REPLACE"
)

func (c ChangeType) Known() bool {
	return oneOf(c, ChangeAdd, ChangeMove, ChangeRemove, ChangeReplace)
}

type ChangeItem struct {
	Op        ChangeType      `json:"op"`
	Path      string          `json:"path"`
	From      string          `json:"from,omitempty"`
	OrigValue json.RawMessage `json:"origValue,omitempty"`
	NewValue  json.RawMessage `json:"newValue,omitempty"`
}

func (c *ChangeItem) UnmarshalJSON(data []byte) error {
	if err := requireKeys(data, "path"); err != nil {
		return err
	}

	type plain ChangeItem

	return json.Unmarshal(data, (*plain)(c))
}

func (c ChangeItem) Check(p Policy) error {
	if err := checkEnum("/op", c.Op, p, false); err != nil {
		return err
	}

	if p == Strict && c.Op == ChangeMove && c.From == "" {
		return missing("/from")
	}

	return nil
}

type NotifyItem struct {
	ResourceID string       `json:"resourceId"`
	Changes    []ChangeItem `json:"changes"`
}

func (n NotifyItem) Check(p Policy) error {
	if n.ResourceID == "" {
		return missing("/resourceId")
	}

	if len(n.Changes) == 0 {
		return missing("/changes")
	}

	return checkAll("/changes", n.Changes, p)
}

type ModificationNotification struct {
	NotifyItems    []NotifyItem `json:"notifyItems"`
	SubscriptionID string       `json:"subscriptionId,omitempty"`
	OldGuami       *Guami       `json:"oldGuami,omitempty"`
}

func (m ModificationNotification) Check(p Policy) error {
	if len(m.NotifyItems) == 0 {
		return missing("/notifyItems")
	}

	return firstError(checkAll("/notifyItems", m.NotifyItems, p), checkOptional("/oldGuami", m.OldGuami, p))
}
