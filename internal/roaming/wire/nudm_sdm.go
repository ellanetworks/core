// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package wire

import (
	"bytes"
	"encoding/json"
	"errors"
	"regexp"
	"strings"
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

type OdbPacketServices string

const (
	OdbAllPacketServices   OdbPacketServices = "ALL_PACKET_SERVICES"
	OdbRoamerAccessHPLMNAP OdbPacketServices = "ROAMER_ACCESS_HPLMN_AP"
	OdbRoamerAccessVPLMNAP OdbPacketServices = "ROAMER_ACCESS_VPLMN_AP"
)

func (o OdbPacketServices) Known() bool {
	return oneOf(o, OdbAllPacketServices, OdbRoamerAccessHPLMNAP, OdbRoamerAccessVPLMNAP)
}

func (o OdbPacketServices) BarsLocalBreakout() bool {
	return o == OdbAllPacketServices || o == OdbRoamerAccessVPLMNAP
}

type RoamingRestrictions struct {
	AccessAllowed *bool `json:"accessAllowed,omitempty"`
}

func (RoamingRestrictions) Check(Policy) error {
	return nil
}

type Nssai struct {
	SupportedFeatures   SupportedFeatures `json:"supportedFeatures,omitempty"`
	DefaultSingleNssais []Snssai          `json:"defaultSingleNssais"`
	SingleNssais        []Snssai          `json:"singleNssais,omitempty"`
	ProvisioningTime    DateTime          `json:"provisioningTime,omitempty"`
}

func (n Nssai) Check(p Policy) error {
	if err := firstError(
		present("/defaultSingleNssais", len(n.DefaultSingleNssais) > 0),
		CheckAll("/defaultSingleNssais", n.DefaultSingleNssais, p),
		CheckAll("/singleNssais", n.SingleNssais, p),
	); err != nil || p != Strict {
		return err
	}

	return firstError(
		Nest("/supportedFeatures", n.SupportedFeatures.Check(p)),
		checkDateTime("/provisioningTime", n.ProvisioningTime, true),
	)
}

type SteeringInfo struct {
	PlmnID         *PlmnID      `json:"plmnId"`
	AccessTechList []AccessTech `json:"accessTechList,omitempty"`
}

func (s SteeringInfo) Check(p Policy) error {
	return firstError(
		CheckRequired("/plmnId", s.PlmnID, p),
		CheckEnums("/accessTechList", s.AccessTechList, p),
	)
}

type SteeringContainer struct {
	SteeringInfo  []SteeringInfo
	SecuredPacket []byte
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
		return Incorrect("", true, "empty steering container")
	}

	return CheckAll("", c.SteeringInfo, p)
}

type SorInfo struct {
	SteeringContainer       *SteeringContainer `json:"steeringContainer,omitempty"`
	AckInd                  *bool              `json:"ackInd"`
	SorMacIausf             string             `json:"sorMacIausf,omitempty"`
	Countersor              string             `json:"countersor,omitempty"`
	ProvisioningTime        DateTime           `json:"provisioningTime"`
	SorTransparentContainer []byte             `json:"sorTransparentContainer,omitempty"`
}

func (s SorInfo) Check(p Policy) error {
	if err := firstError(
		present("/ackInd", s.AckInd != nil),
		present("/provisioningTime", s.ProvisioningTime != ""),
		CheckOptional("/steeringContainer", s.SteeringContainer, p),
		checkPattern("/sorMacIausf", s.SorMacIausf, sorMacPattern, true),
		checkPattern("/countersor", s.Countersor, counterSorPattern, true),
	); err != nil || p != Strict {
		return err
	}

	return checkDateTime("/provisioningTime", s.ProvisioningTime, false)
}

type AcknowledgeInfo struct {
	SorMacIue               string   `json:"sorMacIue,omitempty"`
	ProvisioningTime        DateTime `json:"provisioningTime"`
	SorTransparentContainer []byte   `json:"sorTransparentContainer,omitempty"`
}

func (a AcknowledgeInfo) Check(p Policy) error {
	if err := present("/provisioningTime", a.ProvisioningTime != ""); err != nil || p != Strict {
		return err
	}

	return firstError(
		checkDateTime("/provisioningTime", a.ProvisioningTime, false),
		checkPattern("/sorMacIue", a.SorMacIue, sorMacPattern, true),
	)
}

type AccessAndMobilitySubscriptionData struct {
	SupportedFeatures           SupportedFeatures    `json:"supportedFeatures,omitempty"`
	SubscribedUeAmbr            *Ambr                `json:"subscribedUeAmbr,omitempty"`
	Nssai                       *Nssai               `json:"nssai,omitempty"`
	RatRestrictions             []RatType            `json:"ratRestrictions,omitempty"`
	CoreNetworkTypeRestrictions []CoreNetworkType    `json:"coreNetworkTypeRestrictions,omitempty"`
	SorInfo                     *SorInfo             `json:"sorInfo,omitempty"`
	SorInfoExpectInd            bool                 `json:"sorInfoExpectInd,omitempty"`
	OdbPacketServices           OdbPacketServices    `json:"odbPacketServices,omitempty"`
	RoamingRestrictions         *RoamingRestrictions `json:"roamingRestrictions,omitempty"`
}

func (a AccessAndMobilitySubscriptionData) Check(p Policy) error {
	if err := firstError(
		CheckOptional("/subscribedUeAmbr", a.SubscribedUeAmbr, p),
		CheckOptional("/nssai", a.Nssai, p),
		CheckEnums("/ratRestrictions", a.RatRestrictions, p),
		CheckEnums("/coreNetworkTypeRestrictions", a.CoreNetworkTypeRestrictions, p),
		CheckOptional("/sorInfo", a.SorInfo, p),
		CheckEnum("/odbPacketServices", a.OdbPacketServices, p, true),
	); err != nil || p != Strict {
		return err
	}

	return Nest("/supportedFeatures", a.SupportedFeatures.Check(p))
}

func (a AccessAndMobilitySubscriptionData) RATRestricted(r RatType) bool {
	return oneOf(r, a.RatRestrictions...)
}

func (a AccessAndMobilitySubscriptionData) CoreNetworkRestricted(c CoreNetworkType) bool {
	return oneOf(c, a.CoreNetworkTypeRestrictions...)
}

func (a AccessAndMobilitySubscriptionData) RoamingAccessDenied() bool {
	return a.RoamingRestrictions != nil && a.RoamingRestrictions.AccessAllowed != nil && !*a.RoamingRestrictions.AccessAllowed
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

func validDnnKey(s string) bool {
	return s == WildcardDnn || ValidDnn(s)
}

func (d DnnInfo) Check(p Policy) error {
	if err := present("/dnn", d.Dnn != ""); err != nil || p != Strict {
		return err
	}

	return checkString("/dnn", d.Dnn, validDnnKey, false)
}

type SnssaiInfo struct {
	DnnInfos []DnnInfo `json:"dnnInfos"`
}

func (s *SnssaiInfo) UnmarshalJSON(data []byte) error {
	var aux struct {
		DnnInfos []json.RawMessage `json:"dnnInfos"`
	}

	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}

	s.DnnInfos = decodeValid[DnnInfo](aux.DnnInfos)

	return nil
}

func (s SnssaiInfo) Check(p Policy) error {
	return firstError(
		present("/dnnInfos", len(s.DnnInfos) > 0),
		CheckAll("/dnnInfos", s.DnnInfos, p),
	)
}

func (s SnssaiInfo) DefaultDnn() (DnnInfo, bool) {
	for _, d := range s.DnnInfos {
		if d.DefaultDnnIndicator && d.Dnn != WildcardDnn {
			return d, true
		}
	}

	return DnnInfo{}, false
}

func (s SnssaiInfo) Dnn(dnn string) (DnnInfo, bool) {
	for _, d := range s.DnnInfos {
		if d.Dnn != WildcardDnn && equalDnn(d.Dnn, dnn) {
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

func validSnssaiKey(k string) bool {
	_, err := ParseSnssai(k)

	return err == nil
}

func (s *SmfSelectionSubscriptionData) UnmarshalJSON(data []byte) error {
	type plain SmfSelectionSubscriptionData

	aux := struct {
		*plain
		SubscribedSnssaiInfos map[string]json.RawMessage `json:"subscribedSnssaiInfos"`
	}{plain: (*plain)(s)}

	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}

	s.SubscribedSnssaiInfos = decodeValidMap[SnssaiInfo](aux.SubscribedSnssaiInfos, validSnssaiKey, nil)

	return nil
}

func (s SmfSelectionSubscriptionData) Check(p Policy) error {
	for _, key := range sortedKeys(s.SubscribedSnssaiInfos) {
		if !validSnssaiKey(key) {
			return Incorrect("/subscribedSnssaiInfos/"+key, true, "key is not <sst>[-<sd>]")
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
	for _, key := range sortedKeys(s.SubscribedSnssaiInfos) {
		if k, err := ParseSnssai(key); err == nil && k.Equal(snssai) {
			return s.SubscribedSnssaiInfos[key], true
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
		CheckEnum("/preemptCap", a.PreemptCap, p, false),
		CheckEnum("/preemptVuln", a.PreemptVuln, p, false),
	)
}

type SubscribedDefaultQos struct {
	FiveQi        *int `json:"5qi"`
	Arp           *Arp `json:"arp"`
	PriorityLevel *int `json:"priorityLevel,omitempty"`
}

func (q SubscribedDefaultQos) Check(p Policy) error {
	if err := firstError(
		present("/5qi", q.FiveQi != nil),
		present("/arp", q.Arp != nil),
	); err != nil || p != Strict {
		return err
	}

	return firstError(
		checkRequiredRange("/5qi", q.FiveQi, 0, 255),
		CheckRequired("/arp", q.Arp, p),
		checkOptionalRange("/priorityLevel", q.PriorityLevel, 1, 127),
	)
}

type PduSessionTypes struct {
	DefaultSessionType  PduSessionType   `json:"defaultSessionType,omitempty"`
	AllowedSessionTypes []PduSessionType `json:"allowedSessionTypes,omitempty"`
}

func (t PduSessionTypes) Check(p Policy) error {
	return firstError(
		CheckEnum("/defaultSessionType", t.DefaultSessionType, p, true),
		CheckEnums("/allowedSessionTypes", t.AllowedSessionTypes, p),
	)
}

type SscModes struct {
	DefaultSscMode  SscMode   `json:"defaultSscMode"`
	AllowedSscModes []SscMode `json:"allowedSscModes,omitempty"`
}

func (m SscModes) Check(p Policy) error {
	if err := CheckEnum("/defaultSscMode", m.DefaultSscMode, p, false); err != nil {
		return err
	}

	if p == Strict && len(m.AllowedSscModes) > 2 {
		return Incorrect("/allowedSscModes", true, "more than 2 items")
	}

	return CheckEnums("/allowedSscModes", m.AllowedSscModes, p)
}

type DnnConfiguration struct {
	PduSessionTypes  *PduSessionTypes      `json:"pduSessionTypes"`
	SscModes         *SscModes             `json:"sscModes"`
	IwkEpsInd        bool                  `json:"iwkEpsInd,omitempty"`
	FiveGQosProfile  *SubscribedDefaultQos `json:"5gQosProfile,omitempty"`
	SessionAmbr      *Ambr                 `json:"sessionAmbr,omitempty"`
	ChargingChars    string                `json:"3gppChargingCharacteristics,omitempty"`
	SecondaryAuth    *bool                 `json:"secondaryAuth,omitempty"`
	AtsssAllowed     bool                  `json:"atsssAllowed,omitempty"`
	RedundantSession *bool                 `json:"redundantSessionAllowed,omitempty"`
}

func (d DnnConfiguration) Check(p Policy) error {
	if err := firstError(
		CheckRequired("/pduSessionTypes", d.PduSessionTypes, p),
		CheckRequired("/sscModes", d.SscModes, p),
		CheckOptional("/sessionAmbr", d.SessionAmbr, p),
	); err != nil || p != Strict {
		return err
	}

	return CheckOptional("/5gQosProfile", d.FiveGQosProfile, p)
}

type SessionManagementSubscriptionData struct {
	SingleNssai               *Snssai                     `json:"singleNssai"`
	DnnConfigurations         map[string]DnnConfiguration `json:"dnnConfigurations,omitempty"`
	SharedDnnConfigurationsID string                      `json:"sharedDnnConfigurationsId,omitempty"`
	SupportedFeatures         SupportedFeatures           `json:"supportedFeatures,omitempty"`
}

func (s *SessionManagementSubscriptionData) UnmarshalJSON(data []byte) error {
	type plain SessionManagementSubscriptionData

	aux := struct {
		*plain
		DnnConfigurations map[string]json.RawMessage `json:"dnnConfigurations"`
	}{plain: (*plain)(s)}

	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}

	s.DnnConfigurations = decodeValidMap[DnnConfiguration](aux.DnnConfigurations, func(k string) bool { return k != "" }, nil)

	return nil
}

func (s SessionManagementSubscriptionData) Check(p Policy) error {
	if err := CheckRequired("/singleNssai", s.SingleNssai, p); err != nil {
		return err
	}

	for _, dnn := range sortedKeys(s.DnnConfigurations) {
		if dnn == "" || p == Strict && !validDnnKey(dnn) {
			return Incorrect("/dnnConfigurations/"+dnn, true, "key is not a DNN")
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
	if cfg, ok := s.DnnConfigurations[dnn]; ok && dnn != WildcardDnn {
		return cfg, true
	}

	for _, key := range sortedKeys(s.DnnConfigurations) {
		if key != WildcardDnn && equalDnn(key, dnn) {
			return s.DnnConfigurations[key], true
		}
	}

	cfg, ok := s.DnnConfigurations[WildcardDnn]

	return cfg, ok
}

type SmSubsData struct {
	Individual          []SessionManagementSubscriptionData
	SharedSmSubsDataIDs []string
	extended            bool
}

func (d *SmSubsData) UnmarshalJSON(data []byte) error {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) > 0 && trimmed[0] == '{' {
		var ext struct {
			SharedSmSubsDataIDs  []string          `json:"sharedSmSubsDataIds"`
			IndividualSmSubsData []json.RawMessage `json:"individualSmSubsData"`
		}

		if err := json.Unmarshal(trimmed, &ext); err != nil {
			return err
		}

		*d = SmSubsData{
			Individual:          decodeValid[SessionManagementSubscriptionData](ext.IndividualSmSubsData),
			SharedSmSubsDataIDs: ext.SharedSmSubsDataIDs,
			extended:            true,
		}

		return nil
	}

	var list []json.RawMessage
	if err := json.Unmarshal(trimmed, &list); err != nil {
		return err
	}

	*d = SmSubsData{Individual: decodeValid[SessionManagementSubscriptionData](list)}

	return nil
}

func (d SmSubsData) Check(p Policy) error {
	if d.extended || len(d.SharedSmSubsDataIDs) > 0 {
		if err := present("/sharedSmSubsDataIds", len(d.SharedSmSubsDataIDs) > 0); err != nil {
			return err
		}
	} else if len(d.Individual) == 0 {
		return Missing("")
	}

	for i, id := range d.SharedSmSubsDataIDs {
		if !sharedDataIDPattern.MatchString(id) {
			return NestIndex("/sharedSmSubsDataIds", i, Incorrect("", false, "invalid shared data ID %q", id))
		}
	}

	return CheckAll("", d.Individual, p)
}

var (
	ErrNotSubscribed = errors.New("wire: S-NSSAI and DNN not subscribed")
	ErrSharedData    = errors.New("wire: subscription data refers to shared data, which Ella does not negotiate")
)

func (d SmSubsData) Find(snssai Snssai, dnn string) (SessionManagementSubscriptionData, DnnConfiguration, error) {
	for _, s := range d.Individual {
		if s.SingleNssai == nil || !s.SingleNssai.Equal(snssai) {
			continue
		}

		if cfg, ok := s.DnnConfiguration(dnn); ok {
			return s, cfg, nil
		}
	}

	if len(d.SharedSmSubsDataIDs) > 0 {
		return SessionManagementSubscriptionData{}, DnnConfiguration{}, ErrSharedData
	}

	return SessionManagementSubscriptionData{}, DnnConfiguration{}, ErrNotSubscribed
}

type SubscriptionDataSets struct {
	AmData     *AccessAndMobilitySubscriptionData `json:"amData,omitempty"`
	SmfSelData *SmfSelectionSubscriptionData      `json:"smfSelData,omitempty"`
	SmData     *SmSubsData                        `json:"smData,omitempty"`
}

func (s SubscriptionDataSets) Check(p Policy) error {
	return firstError(
		CheckOptional("/amData", s.AmData, p),
		CheckOptional("/smfSelData", s.SmfSelData, p),
		CheckOptional("/smData", s.SmData, p),
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
	UniqueSubscription    *bool             `json:"uniqueSubscription,omitempty"`
}

func (s SdmSubscription) Check(p Policy) error {
	if err := firstError(
		present("/nfInstanceId", s.NfInstanceID != ""),
		present("/callbackReference", s.CallbackReference != ""),
		present("/monitoredResourceUris", len(s.MonitoredResourceUris) > 0),
		checkDateTime("/expires", s.Expires, true),
	); err != nil || p != Strict {
		return err
	}

	for i, u := range s.MonitoredResourceUris {
		if u == "" {
			return NestIndex("/monitoredResourceUris", i, Missing(""))
		}
	}

	return firstError(
		checkString("/nfInstanceId", s.NfInstanceID, ValidNfInstanceID, false),
		checkAbsoluteURI("/callbackReference", s.CallbackReference, false),
		CheckOptional("/singleNssai", s.SingleNssai, p),
		checkString("/dnn", s.Dnn, ValidDnn, true),
		CheckOptional("/plmnId", s.PlmnID, p),
		Nest("/supportedFeatures", s.SupportedFeatures.Check(p)),
	)
}

type SdmSubsModification struct {
	Expires               DateTime `json:"expires,omitempty"`
	MonitoredResourceUris []string `json:"monitoredResourceUris,omitempty"`
}

func (m SdmSubsModification) Check(p Policy) error {
	if p != Strict {
		return nil
	}

	if m.Expires == "" && m.MonitoredResourceUris == nil {
		return Incorrect("", false, "empty modification")
	}

	if m.MonitoredResourceUris != nil && len(m.MonitoredResourceUris) == 0 {
		return Incorrect("/monitoredResourceUris", true, "empty list")
	}

	for i, u := range m.MonitoredResourceUris {
		if u == "" {
			return NestIndex("/monitoredResourceUris", i, Missing(""))
		}
	}

	return checkDateTime("/expires", m.Expires, true)
}

type SdmSubsModifyResult struct {
	Subscription *SdmSubscription
	Patch        *PatchResult
}

func (r *SdmSubsModifyResult) UnmarshalJSON(data []byte) error {
	var keys map[string]json.RawMessage
	if err := json.Unmarshal(data, &keys); err != nil {
		return err
	}

	if _, ok := keys["report"]; ok {
		var pr PatchResult
		if err := json.Unmarshal(data, &pr); err != nil {
			return err
		}

		*r = SdmSubsModifyResult{Patch: &pr}

		return nil
	}

	var s SdmSubscription
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}

	*r = SdmSubsModifyResult{Subscription: &s}

	return nil
}

func (r SdmSubsModifyResult) Check(p Policy) error {
	if r.Patch != nil {
		return r.Patch.Check(p)
	}

	if r.Subscription == nil {
		return Missing("")
	}

	return r.Subscription.Check(p)
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
	Path      *string         `json:"path"`
	From      string          `json:"from,omitempty"`
	OrigValue json.RawMessage `json:"origValue,omitempty"`
	NewValue  json.RawMessage `json:"newValue,omitempty"`
}

func (c ChangeItem) Check(p Policy) error {
	if err := firstError(
		CheckEnum("/op", c.Op, p, false),
		present("/path", c.Path != nil),
	); err != nil || p != Strict {
		return err
	}

	if c.Op == ChangeMove && c.From == "" {
		return Missing("/from")
	}

	return nil
}

type NotifyItem struct {
	ResourceID string       `json:"resourceId"`
	Changes    []ChangeItem `json:"changes"`
}

func (n NotifyItem) Check(p Policy) error {
	if err := firstError(
		present("/resourceId", n.ResourceID != ""),
		present("/changes", len(n.Changes) > 0),
	); err != nil || p != Strict {
		return err
	}

	return CheckAll("/changes", n.Changes, p)
}

func (n NotifyItem) About(supi string, resource func(string) string) bool {
	return strings.HasSuffix(n.ResourceID, resource(supi))
}

type ModificationNotification struct {
	NotifyItems    []NotifyItem `json:"notifyItems"`
	SubscriptionID string       `json:"subscriptionId,omitempty"`
	OldGuami       *Guami       `json:"oldGuami,omitempty"`
}

func (m ModificationNotification) Check(p Policy) error {
	if err := firstError(
		present("/notifyItems", len(m.NotifyItems) > 0),
		CheckAll("/notifyItems", m.NotifyItems, p),
	); err != nil || p != Strict {
		return err
	}

	return CheckOptional("/oldGuami", m.OldGuami, p)
}
