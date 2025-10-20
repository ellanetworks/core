package nas

import (
	"encoding/hex"
	"fmt"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/omec-project/nas"
	"github.com/omec-project/nas/nasMessage"
	"github.com/omec-project/nas/nasType"
	"go.uber.org/zap"
)

type PLMNID struct {
	Mcc string `json:"mcc"`
	Mnc string `json:"mnc"`
}

type TAI struct {
	PLMNID PLMNID `json:"plmn_id"`
	TAC    string `json:"tac"`
}

type NetworkFeatureSupport5GS struct {
	Emc          uint8 `json:"emc"`
	EmcN3        uint8 `json:"emc_n3"`
	Emf          uint8 `json:"emf"`
	ImsVoPS      uint8 `json:"ims_vops"`
	IwkN26       uint8 `json:"iwk_n26"`
	Mcsi         uint8 `json:"mcsi"`
	Mpsi         uint8 `json:"mpsi"`
	IMSVoPS3GPP  uint8 `json:"ims_vops_3gpp"`
	IMSVoPSN3GPP uint8 `json:"ims_vops_n3gpp"`
}

type SNSSAI struct {
	SST int32   `json:"sst"`
	SD  *string `json:"sd,omitempty"`
}

type RegistrationAccept struct {
	ExtendedProtocolDiscriminator       uint8                     `json:"extended_protocol_discriminator"`
	SpareHalfOctetAndSecurityHeaderType uint8                     `json:"spare_half_octet_and_security_header_type"`
	RegistrationAcceptMessageIdentity   string                    `json:"registration_accept_message_identity"`
	RegistrationResult5GS               string                    `json:"registration_result_5gs"`
	GUTI5G                              *string                   `json:"guti_5g,omitempty"`
	EquivalentPLMNs                     []PLMNID                  `json:"equivalent_plmns,omitempty"`
	TAIList                             []TAI                     `json:"tai_list,omitempty"`
	AllowedNSSAI                        []SNSSAI                  `json:"allowed_nssai,omitempty"`
	NetworkFeatureSupport5GS            *NetworkFeatureSupport5GS `json:"network_feature_support_5gs,omitempty"`
}

func buildRegistrationResult5GS(msg nasType.RegistrationResult5GS) string {
	value := msg.GetRegistrationResultValue5GS()
	switch {
	case value&(nasMessage.AccessType3GPP|nasMessage.AccessTypeNon3GPP) == (nasMessage.AccessType3GPP | nasMessage.AccessTypeNon3GPP):
		return "3GPP and Non-3GPP"
	case value&nasMessage.AccessType3GPP != 0:
		return "3GPP only"
	case value&nasMessage.AccessTypeNon3GPP != 0:
		return "Non-3GPP only"
	default:
		return fmt.Sprintf("Unknown(%d)", value)
	}
}

func buildRegistrationAccept(msg *nasMessage.RegistrationAccept) *RegistrationAccept {
	if msg == nil {
		return nil
	}

	registrationAccept := &RegistrationAccept{
		ExtendedProtocolDiscriminator:       msg.ExtendedProtocolDiscriminator.Octet,
		SpareHalfOctetAndSecurityHeaderType: msg.SpareHalfOctetAndSecurityHeaderType.Octet,
		RegistrationAcceptMessageIdentity:   nas.MessageName(msg.RegistrationAcceptMessageIdentity.Octet),
		RegistrationResult5GS:               buildRegistrationResult5GS(msg.RegistrationResult5GS),
	}

	if msg.GUTI5G != nil {
		guti := buildGUTI5G(*msg.GUTI5G)
		registrationAccept.GUTI5G = &guti
	}

	if msg.EquivalentPlmns != nil {
		registrationAccept.EquivalentPLMNs = equivalentPlmnsToList(*msg.EquivalentPlmns)
	}

	if msg.TAIList != nil {
		taiList := nasToTaiList(msg.TAIList)
		registrationAccept.TAIList = taiList
	}

	if msg.AllowedNSSAI != nil {
		allowedNssai := buildAllowedSNSSAI(*msg.AllowedNSSAI)
		registrationAccept.AllowedNSSAI = allowedNssai
	}

	if msg.RejectedNSSAI != nil {
		logger.EllaLog.Warn("RejectedNSSAI not yet implemented")
	}

	if msg.ConfiguredNSSAI != nil {
		logger.EllaLog.Warn("ConfiguredNSSAI not yet implemented")
	}

	if msg.NetworkFeatureSupport5GS != nil {
		networkfeatureSupport5Gs := buildNetworkFeatureSupport5GS(*msg.NetworkFeatureSupport5GS)
		registrationAccept.NetworkFeatureSupport5GS = &networkfeatureSupport5Gs
	}

	if msg.PDUSessionStatus != nil {
		logger.EllaLog.Warn("PDUSessionStatus not yet implemented")
	}

	if msg.PDUSessionReactivationResult != nil {
		logger.EllaLog.Warn("PDUSessionReactivationResult not yet implemented")
	}

	if msg.PDUSessionReactivationResultErrorCause != nil {
		logger.EllaLog.Warn("PDUSessionReactivationResultErrorCause not yet implemented")
	}

	if msg.LADNInformation != nil {
		logger.EllaLog.Warn("LADNInformation not yet implemented")
	}

	if msg.MICOIndication != nil {
		logger.EllaLog.Warn("MICOIndication not yet implemented")
	}

	if msg.NetworkSlicingIndication != nil {
		logger.EllaLog.Warn("NetworkSlicingIndication not yet implemented")
	}

	if msg.ServiceAreaList != nil {
		logger.EllaLog.Warn("ServiceAreaList not yet implemented")
	}

	if msg.ServiceAreaList != nil {
		logger.EllaLog.Warn("ServiceAreaList not yet implemented")
	}

	if msg.T3512Value != nil {
		logger.EllaLog.Warn("T3512Value not yet implemented")
	}

	if msg.Non3GppDeregistrationTimerValue != nil {
		logger.EllaLog.Warn("Non3GppDeregistrationTimerValue not yet implemented")
	}
	if msg.T3502Value != nil {
		logger.EllaLog.Warn("T3502Value not yet implemented")
	}
	if msg.EmergencyNumberList != nil {
		logger.EllaLog.Warn("EmergencyNumberList not yet implemented")
	}
	if msg.ExtendedEmergencyNumberList != nil {
		logger.EllaLog.Warn("ExtendedEmergencyNumberList not yet implemented")
	}
	if msg.SORTransparentContainer != nil {
		logger.EllaLog.Warn("SORTransparentContainer not yet implemented")
	}

	if msg.EAPMessage != nil {
		logger.EllaLog.Warn("EAPMessage not yet implemented")
	}

	if msg.NSSAIInclusionMode != nil {
		logger.EllaLog.Warn("NSSAIInclusionMode not yet implemented")
	}

	if msg.OperatordefinedAccessCategoryDefinitions != nil {
		logger.EllaLog.Warn("OperatordefinedAccessCategoryDefinitions not yet implemented")
	}

	if msg.NegotiatedDRXParameters != nil {
		logger.EllaLog.Warn("NegotiatedDRXParameters not yet implemented")
	}

	return registrationAccept
}

func buildGUTI5G(gutiNas nasType.GUTI5G) string {
	mcc1 := gutiNas.GetMCCDigit1()
	mcc2 := gutiNas.GetMCCDigit2()
	mcc3 := gutiNas.GetMCCDigit3()
	mnc1 := gutiNas.GetMNCDigit1()
	mnc2 := gutiNas.GetMNCDigit2()
	mnc3 := gutiNas.GetMNCDigit3()

	amfRegionID := gutiNas.GetAMFRegionID()
	amfSetID := gutiNas.GetAMFSetID()
	amfPointer := gutiNas.GetAMFPointer()
	amfID := nasToAmfId(amfRegionID, amfSetID, amfPointer)

	tmsi := hex.EncodeToString(gutiNas.Octet[7:11])

	if mnc3 == 0x0F {
		return fmt.Sprintf("%d%d%d%d%d%s%s", mcc1, mcc2, mcc3, mnc1, mnc2, amfID, tmsi)
	}

	return fmt.Sprintf("%d%d%d%d%d%d%s%s", mcc1, mcc2, mcc3, mnc1, mnc2, mnc3, amfID, tmsi)
}

func nasToAmfId(regionID uint8, setID uint16, pointer uint8) string {
	setID &= 0x03FF // 10 bits
	pointer &= 0x3F // 6 bits

	b0 := regionID
	b1 := uint8(setID >> 2)
	b2 := uint8((setID&0x3)<<6) | (pointer & 0x3F)

	return fmt.Sprintf("%02x%02x%02x", b0, b1, b2)
}

// nasToTaiList decodes the NAS-encoded TAI list produced by TaiListToNas.
func nasToTaiList(nas *nasType.TAIList) []TAI {
	if nas == nil {
		return nil
	}

	data := nas.GetPartialTrackingAreaIdentityList()

	if len(data) < 1 {
		logger.EllaLog.Warn("TAIList too short")
		return nil
	}

	header := data[0]
	typeOfList := int((header >> 5) & 0x07) // top 3 bits
	n := int(header&0x1F) + 1               // number of TAIs

	switch typeOfList {
	case 0x00:
		// Structure: [HDR][PLMN(3)][TAC(3) x N]
		minLen := 1 + 3 + 3*n
		if len(data) < minLen {
			return nil
		}
		idx := 1
		plmn, err := plmnFromNas3(data[idx], data[idx+1], data[idx+2])
		if err != nil {
			return nil
		}
		idx += 3

		out := make([]TAI, 0, n)
		for range n {
			tacBytes := data[idx : idx+3]
			idx += 3
			out = append(out, TAI{
				PLMNID: plmn,                         // same PLMN for all
				TAC:    hex.EncodeToString(tacBytes), // 6 hex chars
			})
		}

		if idx != len(data) {
			logger.EllaLog.Warn("TAIList has trailing bytes")
		}
		return out

	case 0x02:
		// Structure: [HDR][PLMN(3)+TAC(3)] x N
		minLen := 1 + n*6
		if len(data) < minLen {
			return nil
		}
		idx := 1
		out := make([]TAI, 0, n)
		for range n {
			plmn, err := plmnFromNas3(data[idx], data[idx+1], data[idx+2])
			if err != nil {
				logger.EllaLog.Warn("TAIList invalid PLMN", zap.Error(err))
				return nil
			}
			idx += 3
			tacBytes := data[idx : idx+3]
			idx += 3
			out = append(out, TAI{
				PLMNID: plmn,
				TAC:    hex.EncodeToString(tacBytes),
			})
		}
		if idx != len(data) {
			logger.EllaLog.Warn("TAIList has trailing bytes")
		}
		return out

	default:
		return nil
	}
}

func plmnFromNas3(b0, b1, b2 uint8) (PLMNID, error) {
	mcc1 := int(b0 & 0x0F)
	mcc2 := int((b0 >> 4) & 0x0F)
	mcc3 := int(b1 & 0x0F)
	mnc3 := int((b1 >> 4) & 0x0F)
	mnc1 := int(b2 & 0x0F)
	mnc2 := int((b2 >> 4) & 0x0F)

	// basic digit checks
	if mcc1 > 9 || mcc2 > 9 || mcc3 > 9 || mnc1 > 9 || mnc2 > 9 || (mnc3 != 0xF && mnc3 > 9) {
		return PLMNID{}, fmt.Errorf("invalid BCD digits in PLMN: %02x %02x %02x", b0, b1, b2)
	}

	plmn := PLMNID{
		Mcc: fmt.Sprintf("%d%d%d", mcc1, mcc2, mcc3),
	}
	if mnc3 == 0xF {
		plmn.Mnc = fmt.Sprintf("%d%d", mnc1, mnc2) // 2-digit MNC
	} else {
		plmn.Mnc = fmt.Sprintf("%d%d%d", mnc1, mnc2, mnc3) // 3-digit MNC
	}
	return plmn, nil
}

// Full inverse for the NAS Equivalent PLMNs IE.
// EquivalentPlmns.Len is the number of bytes in Octet actually used (multiple of 3).
func equivalentPlmnsToList(eq nasType.EquivalentPlmns) []PLMNID {
	if eq.Len == 0 {
		logger.EllaLog.Warn("EquivalentPlmns length is zero")
		return nil
	}

	if eq.Len%3 != 0 {
		logger.EllaLog.Warn("EquivalentPlmns length not multiple of 3")
		return nil
	}

	if int(eq.Len) > len(eq.Octet) {
		logger.EllaLog.Warn("EquivalentPlmns has trailing bytes")
		return nil
	}

	n := int(eq.Len) / 3
	out := make([]PLMNID, 0, n)

	for i := range n {
		base := i * 3
		plmn, err := nasPlmn3ToID(eq.Octet[base], eq.Octet[base+1], eq.Octet[base+2])
		if err != nil {
			logger.EllaLog.Warn("EquivalentPlmns invalid PLMN", zap.Error(err))
			return nil
		}
		out = append(out, plmn)
	}

	return out
}

func nasPlmn3ToID(b0, b1, b2 uint8) (PLMNID, error) {
	mcc1 := int(b0 & 0x0F)
	mcc2 := int((b0 >> 4) & 0x0F)
	mcc3 := int(b1 & 0x0F)
	mnc3 := int((b1 >> 4) & 0x0F)
	mnc1 := int(b2 & 0x0F)
	mnc2 := int((b2 >> 4) & 0x0F)

	// Basic digit validation (0..9 or 0xF for mnc3)
	if mcc1 > 9 || mcc2 > 9 || mcc3 > 9 || mnc1 > 9 || mnc2 > 9 || (mnc3 != 0x0F && mnc3 > 9) {
		return PLMNID{}, fmt.Errorf("invalid BCD digits in PLMN bytes: %02x %02x %02x", b0, b1, b2)
	}

	mcc := fmt.Sprintf("%d%d%d", mcc1, mcc2, mcc3)
	var mnc string
	if mnc3 == 0x0F {
		// 2-digit MNC
		mnc = fmt.Sprintf("%d%d", mnc1, mnc2)
	} else {
		// 3-digit MNC
		mnc = fmt.Sprintf("%d%d%d", mnc1, mnc2, mnc3)
	}

	return PLMNID{Mcc: mcc, Mnc: mnc}, nil
}

func buildAllowedSNSSAI(msg nasType.AllowedNSSAI) []SNSSAI {
	value := msg.GetSNSSAIValue()
	out := make([]SNSSAI, 0, 4)

	for i := 0; i < len(value); {
		if i >= len(value) {
			logger.EllaLog.Warn("AllowedNSSAI: unexpected end of buffer")
			break
		}
		l := int(value[i])
		i++

		if l != 1 && l != 4 {
			logger.EllaLog.Warn("AllowedNSSAI: unsupported or malformed element length", zap.Int("length", l))
			break
		}
		if i+l > len(value) {
			logger.EllaLog.Warn("AllowedNSSAI: element length exceeds buffer", zap.Int("length", l), zap.Int("remaining", len(value)-i))
			break
		}

		sst := int32(value[i])
		if l == 1 {
			out = append(out, SNSSAI{
				SST: sst,
				SD:  nil,
			})
			i += 1
			continue
		}

		// l == 4 → SST + 3-byte SD
		sdBytes := value[i+1 : i+4]
		sdStr := hex.EncodeToString(sdBytes)
		out = append(out, SNSSAI{
			SST: sst,
			SD:  &sdStr,
		})
		i += 4
	}

	return out
}

func buildNetworkFeatureSupport5GS(msg nasType.NetworkFeatureSupport5GS) NetworkFeatureSupport5GS {
	return NetworkFeatureSupport5GS{
		Emc:          msg.GetEMC(),
		EmcN3:        msg.GetEMCN(),
		Emf:          msg.GetEMF(),
		IwkN26:       msg.GetIWKN26(),
		Mpsi:         msg.GetMPSI(),
		Mcsi:         msg.GetMCSI(),
		IMSVoPS3GPP:  msg.GetIMSVoPS3GPP(),
		IMSVoPSN3GPP: msg.GetIMSVoPSN3GPP(),
	}
}
