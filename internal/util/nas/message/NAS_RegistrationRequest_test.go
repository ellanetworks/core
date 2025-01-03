// Copyright 2019 free5GC.org
//
// SPDX-License-Identifier: Apache-2.0
//

package nasMessage_test

import (
	"bytes"
	"reflect"
	"testing"

	"github.com/ellanetworks/core/internal/util/nas"
	nasMessage "github.com/ellanetworks/core/internal/util/nas/message"
	nasType "github.com/ellanetworks/core/internal/util/nas/type"
	"github.com/stretchr/testify/assert"
)

type nasMessageRegistrationRequestData struct {
	inExtendedProtocolDiscriminator       uint8
	inSecurityHeader                      uint8
	inSpareHalfOctet                      uint8
	inRegistrationRequestMessageIdentity  uint8
	inNgksi                               uint8
	inRegistrationType5GS                 uint8
	inMobileIdentity5GS                   nasType.MobileIdentity5GS
	inNoncurrentNativeNASKeySetIdentifier nasType.NoncurrentNativeNASKeySetIdentifier
	inCapability5GMM                      nasType.Capability5GMM
	inUESecurityCapability                nasType.UESecurityCapability
	inRequestedNSSAI                      nasType.RequestedNSSAI
	inLastVisitedRegisteredTAI            nasType.LastVisitedRegisteredTAI
	inS1UENetworkCapability               nasType.S1UENetworkCapability
	inUplinkDataStatus                    nasType.UplinkDataStatus
	inPDUSessionStatus                    nasType.PDUSessionStatus
	inMICOIndication                      nasType.MICOIndication
	inUEStatus                            nasType.UEStatus
	inAdditionalGUTI                      nasType.AdditionalGUTI
	inAllowedPDUSessionStatus             nasType.AllowedPDUSessionStatus
	inUesUsageSetting                     nasType.UesUsageSetting
	inRequestedDRXParameters              nasType.RequestedDRXParameters
	inEPSNASMessageContainer              nasType.EPSNASMessageContainer
	inLADNIndication                      nasType.LADNIndication
	inPayloadContainer                    nasType.PayloadContainer
	inNetworkSlicingIndication            nasType.NetworkSlicingIndication
	inUpdateType5GS                       nasType.UpdateType5GS
	inNASMessageContainer                 nasType.NASMessageContainer
}

var nasMessageRegistrationRequestTable = []nasMessageRegistrationRequestData{
	{
		inExtendedProtocolDiscriminator:      nasMessage.Epd5GSMobilityManagementMessage,
		inSecurityHeader:                     0x01,
		inSpareHalfOctet:                     0x01,
		inRegistrationRequestMessageIdentity: nas.MsgTypeRegistrationRequest,
		inNgksi:                              0x01,
		inRegistrationType5GS:                0x01,
		inMobileIdentity5GS: nasType.MobileIdentity5GS{
			Len:    2,
			Buffer: []uint8{0x01, 0x01},
		},
		inNoncurrentNativeNASKeySetIdentifier: nasType.NoncurrentNativeNASKeySetIdentifier{
			Octet: 0xC0,
		},
		inCapability5GMM: nasType.Capability5GMM{
			Iei:   nasMessage.RegistrationRequestCapability5GMMType,
			Len:   13,
			Octet: [13]uint8{0x01},
		},
		inUESecurityCapability: nasType.UESecurityCapability{
			Iei:    nasMessage.RegistrationRequestUESecurityCapabilityType,
			Len:    8,
			Buffer: []uint8{0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01},
		},
		inRequestedNSSAI: nasType.RequestedNSSAI{
			Iei:    nasMessage.RegistrationRequestRequestedNSSAIType,
			Len:    2,
			Buffer: []uint8{0x01, 0x01},
		},
		inLastVisitedRegisteredTAI: nasType.LastVisitedRegisteredTAI{
			Iei:   nasMessage.RegistrationRequestLastVisitedRegisteredTAIType,
			Octet: [6]uint8{0x01, 0x01},
		},
		inS1UENetworkCapability: nasType.S1UENetworkCapability{
			Iei:    nasMessage.RegistrationRequestS1UENetworkCapabilityType,
			Len:    2,
			Buffer: []uint8{0x01, 0x01},
		},
		inUplinkDataStatus: nasType.UplinkDataStatus{
			Iei:    nasMessage.RegistrationRequestUplinkDataStatusType,
			Len:    2,
			Buffer: []uint8{0x01, 0x01},
		},
		inPDUSessionStatus: nasType.PDUSessionStatus{
			Iei:    nasMessage.RegistrationRequestPDUSessionStatusType,
			Len:    2,
			Buffer: []uint8{0x01, 0x01},
		},
		inMICOIndication: nasType.MICOIndication{
			Octet: 0xB0,
		},
		inUEStatus: nasType.UEStatus{
			Iei:   nasMessage.RegistrationRequestUEStatusType,
			Len:   2,
			Octet: 0x01,
		},
		inAdditionalGUTI: nasType.AdditionalGUTI{
			Iei:   nasMessage.RegistrationRequestAdditionalGUTIType,
			Len:   11,
			Octet: [11]uint8{0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01, 0x01},
		},
		inAllowedPDUSessionStatus: nasType.AllowedPDUSessionStatus{
			Iei:    nasMessage.RegistrationRequestAllowedPDUSessionStatusType,
			Len:    2,
			Buffer: []uint8{0x01, 0x01},
		},
		inUesUsageSetting: nasType.UesUsageSetting{
			Iei:   nasMessage.RegistrationRequestUesUsageSettingType,
			Len:   2,
			Octet: 0x01,
		},
		inRequestedDRXParameters: nasType.RequestedDRXParameters{
			Iei:   nasMessage.RegistrationRequestRequestedDRXParametersType,
			Len:   2,
			Octet: 0x01,
		},
		inEPSNASMessageContainer: nasType.EPSNASMessageContainer{
			Iei:    nasMessage.RegistrationRequestEPSNASMessageContainerType,
			Len:    2,
			Buffer: []uint8{0x01, 0x01},
		},
		inLADNIndication: nasType.LADNIndication{
			Iei:    nasMessage.RegistrationRequestLADNIndicationType,
			Len:    2,
			Buffer: []uint8{0x01, 0x01},
		},
		inPayloadContainer: nasType.PayloadContainer{
			Iei:    nasMessage.RegistrationRequestPayloadContainerType,
			Len:    2,
			Buffer: []uint8{0x01, 0x01},
		},
		inNetworkSlicingIndication: nasType.NetworkSlicingIndication{
			Octet: 0x90,
		},
		inUpdateType5GS: nasType.UpdateType5GS{
			Iei:   nasMessage.RegistrationRequestUpdateType5GSType,
			Len:   2,
			Octet: 0x01,
		},
		inNASMessageContainer: nasType.NASMessageContainer{
			Iei:    nasMessage.RegistrationRequestNASMessageContainerType,
			Len:    2,
			Buffer: []uint8{0x01, 0x01},
		},
	},
}

func TestNasTypeNewRegistrationRequest(t *testing.T) {
	a := nasMessage.NewRegistrationRequest(0)
	assert.NotNil(t, a)
}

func TestNasTypeNewRegistrationRequestMessage(t *testing.T) {
	for i, table := range nasMessageRegistrationRequestTable {
		t.Logf("Test Cnt:%d", i)
		a := nasMessage.NewRegistrationRequest(0)
		b := nasMessage.NewRegistrationRequest(0)
		assert.NotNil(t, a)
		assert.NotNil(t, b)

		a.ExtendedProtocolDiscriminator.SetExtendedProtocolDiscriminator(table.inExtendedProtocolDiscriminator)
		a.SpareHalfOctetAndSecurityHeaderType.SetSecurityHeaderType(table.inSecurityHeader)
		a.SpareHalfOctetAndSecurityHeaderType.SetSpareHalfOctet(table.inSpareHalfOctet)
		a.RegistrationRequestMessageIdentity.SetMessageType(table.inRegistrationRequestMessageIdentity)
		a.NgksiAndRegistrationType5GS.SetNasKeySetIdentifiler(table.inNgksi)
		a.NgksiAndRegistrationType5GS.SetRegistrationType5GS(table.inRegistrationType5GS)
		a.MobileIdentity5GS = table.inMobileIdentity5GS

		a.NoncurrentNativeNASKeySetIdentifier = nasType.NewNoncurrentNativeNASKeySetIdentifier(nasMessage.RegistrationRequestNoncurrentNativeNASKeySetIdentifierType)
		a.NoncurrentNativeNASKeySetIdentifier = &table.inNoncurrentNativeNASKeySetIdentifier

		a.Capability5GMM = nasType.NewCapability5GMM(nasMessage.RegistrationRequestCapability5GMMType)
		a.Capability5GMM = &table.inCapability5GMM

		a.UESecurityCapability = nasType.NewUESecurityCapability(nasMessage.RegistrationRequestUESecurityCapabilityType)
		a.UESecurityCapability = &table.inUESecurityCapability

		a.RequestedNSSAI = nasType.NewRequestedNSSAI(nasMessage.RegistrationRequestRequestedNSSAIType)
		a.RequestedNSSAI = &table.inRequestedNSSAI

		a.LastVisitedRegisteredTAI = nasType.NewLastVisitedRegisteredTAI(nasMessage.RegistrationRequestLastVisitedRegisteredTAIType)
		a.LastVisitedRegisteredTAI = &table.inLastVisitedRegisteredTAI

		a.S1UENetworkCapability = nasType.NewS1UENetworkCapability(nasMessage.RegistrationRequestS1UENetworkCapabilityType)
		a.S1UENetworkCapability = &table.inS1UENetworkCapability

		a.UplinkDataStatus = nasType.NewUplinkDataStatus(nasMessage.RegistrationRequestUplinkDataStatusType)
		a.UplinkDataStatus = &table.inUplinkDataStatus

		a.PDUSessionStatus = nasType.NewPDUSessionStatus(nasMessage.RegistrationRequestPDUSessionStatusType)
		a.PDUSessionStatus = &table.inPDUSessionStatus

		a.MICOIndication = nasType.NewMICOIndication(nasMessage.RegistrationRequestMICOIndicationType)
		a.MICOIndication = &table.inMICOIndication

		a.UEStatus = nasType.NewUEStatus(nasMessage.RegistrationRequestUEStatusType)
		a.UEStatus = &table.inUEStatus

		a.AdditionalGUTI = nasType.NewAdditionalGUTI(nasMessage.RegistrationRequestAdditionalGUTIType)
		a.AdditionalGUTI = &table.inAdditionalGUTI

		a.AllowedPDUSessionStatus = nasType.NewAllowedPDUSessionStatus(nasMessage.RegistrationRequestAllowedPDUSessionStatusType)
		a.AllowedPDUSessionStatus = &table.inAllowedPDUSessionStatus

		a.UesUsageSetting = nasType.NewUesUsageSetting(nasMessage.RegistrationRequestUesUsageSettingType)
		a.UesUsageSetting = &table.inUesUsageSetting

		a.RequestedDRXParameters = nasType.NewRequestedDRXParameters(nasMessage.RegistrationRequestRequestedDRXParametersType)
		a.RequestedDRXParameters = &table.inRequestedDRXParameters

		a.EPSNASMessageContainer = nasType.NewEPSNASMessageContainer(nasMessage.RegistrationRequestEPSNASMessageContainerType)
		a.EPSNASMessageContainer = &table.inEPSNASMessageContainer

		a.LADNIndication = nasType.NewLADNIndication(nasMessage.RegistrationRequestLADNIndicationType)
		a.LADNIndication = &table.inLADNIndication

		a.PayloadContainer = nasType.NewPayloadContainer(nasMessage.RegistrationRequestPayloadContainerType)
		a.PayloadContainer = &table.inPayloadContainer

		a.NetworkSlicingIndication = nasType.NewNetworkSlicingIndication(nasMessage.RegistrationRequestNetworkSlicingIndicationType)
		a.NetworkSlicingIndication = &table.inNetworkSlicingIndication

		a.UpdateType5GS = nasType.NewUpdateType5GS(nasMessage.RegistrationRequestUpdateType5GSType)
		a.UpdateType5GS = &table.inUpdateType5GS

		a.NASMessageContainer = nasType.NewNASMessageContainer(nasMessage.RegistrationRequestNASMessageContainerType)
		a.NASMessageContainer = &table.inNASMessageContainer

		buff := new(bytes.Buffer)
		a.EncodeRegistrationRequest(buff)

		data := make([]byte, buff.Len())
		buff.Read(data)
		b.DecodeRegistrationRequest(&data)

		if reflect.DeepEqual(a, b) != true {
			t.Errorf("Not correct")
		}
	}
}
