// Copyright 2025 Ella Networks

package nas_test

import (
	"testing"

	"github.com/ellanetworks/core/internal/models"
	smfNas "github.com/ellanetworks/core/internal/smf/nas"
	"github.com/free5gc/nas"
)

func TestBuildGSMPDUSessionEstablishmentAccept_WithSD(t *testing.T) {
	smPolicyData := &models.SmPolicyData{
		Ambr: &models.Ambr{
			Uplink:   "1 Gbps",
			Downlink: "1 Gbps",
		},
		QosData: &models.QosData{
			QFI:    1,
			Var5qi: 9,
		},
	}

	pduSessionID := uint8(10)

	pti := uint8(5)

	snssai := &models.Snssai{
		Sst: 1,
		Sd:  "010203",
	}

	dnn := "internet"

	pco := &smfNas.ProtocolConfigurationOptions{}

	msg, err := smfNas.BuildGSMPDUSessionEstablishmentAccept(smPolicyData, pduSessionID, pti, snssai, dnn, pco, nil, nil)
	if err != nil {
		t.Fatalf("failed to build GSM PDU Session Establishment Accept: %v", err)
	}

	nasMsg := new(nas.Message)

	err = nasMsg.PlainNasDecode(&msg)
	if err != nil {
		t.Fatalf("failed to decode NAS message: %v", err)
	}

	// check that the SD IE is not present
	if nasMsg.PDUSessionEstablishmentAccept.SNSSAI == nil {
		t.Errorf("SNSSAI IE is missing")
	}

	if nasMsg.PDUSessionEstablishmentAccept.SNSSAI.GetLen() != 4 {
		t.Errorf("expected SNSSAI length 1, got %d", nasMsg.PDUSessionEstablishmentAccept.SNSSAI.GetLen())
	}

	if nasMsg.PDUSessionEstablishmentAccept.GetSST() != 1 {
		t.Errorf("expected SST 1, got %d", nasMsg.PDUSessionEstablishmentAccept.GetSST())
	}

	if nasMsg.PDUSessionEstablishmentAccept.GetSD() != [3]uint8{1, 2, 3} {
		t.Errorf("expected SD [1,2,3], got %v", nasMsg.PDUSessionEstablishmentAccept.GetSD())
	}
}

func TestBuildGSMPDUSessionEstablishmentAccept_WithoutSD(t *testing.T) {
	smPolicyData := &models.SmPolicyData{
		Ambr: &models.Ambr{
			Uplink:   "1 Gbps",
			Downlink: "1 Gbps",
		},
		QosData: &models.QosData{
			QFI:    1,
			Var5qi: 9,
		},
	}

	pduSessionID := uint8(10)

	pti := uint8(5)

	snssai := &models.Snssai{
		Sst: 1,
		Sd:  "",
	}

	dnn := "internet"

	pco := &smfNas.ProtocolConfigurationOptions{}

	msg, err := smfNas.BuildGSMPDUSessionEstablishmentAccept(smPolicyData, pduSessionID, pti, snssai, dnn, pco, nil, nil)
	if err != nil {
		t.Fatalf("failed to build GSM PDU Session Establishment Accept: %v", err)
	}

	nasMsg := new(nas.Message)

	err = nasMsg.PlainNasDecode(&msg)
	if err != nil {
		t.Fatalf("failed to decode NAS message: %v", err)
	}

	// check that the SD IE is not present
	if nasMsg.PDUSessionEstablishmentAccept.SNSSAI == nil {
		t.Errorf("SNSSAI IE is missing")
	}

	if nasMsg.PDUSessionEstablishmentAccept.SNSSAI.GetLen() != 1 {
		t.Errorf("expected SNSSAI length 1, got %d", nasMsg.PDUSessionEstablishmentAccept.SNSSAI.GetLen())
	}

	if nasMsg.PDUSessionEstablishmentAccept.GetSST() != 1 {
		t.Errorf("expected SST 1, got %d", nasMsg.PDUSessionEstablishmentAccept.GetSST())
	}

	if nasMsg.PDUSessionEstablishmentAccept.GetSD() != [3]uint8{0, 0, 0} {
		t.Errorf("expected SD [0,0,0], got %v", nasMsg.PDUSessionEstablishmentAccept.GetSD())
	}
}
