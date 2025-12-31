// Copyright 2024 Ella Networks

package context_test

import (
	"testing"

	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/smf/context"
	"github.com/ellanetworks/core/internal/smf/qos"
	"github.com/free5gc/nas"
)

func TestBuildGSMPDUSessionEstablishmentAccept_WithSD(t *testing.T) {
	smPolicyUpdates := &qos.PolicyUpdate{
		SessRuleUpdate: &qos.SessRulesUpdate{
			ActiveSessRule: &models.SessionRule{
				AuthSessAmbr: &models.Ambr{
					Uplink:   "1 Gbps",
					Downlink: "1 Gbps",
				},
				AuthDefQos: &models.AuthorizedDefaultQos{
					Var5qi: 9,
				},
			},
		},
		QosFlowUpdate: &qos.QosFlowsUpdate{
			Add: &models.QosData{
				QFI:    1,
				Var5qi: 9,
			},
		},
	}

	pduSessionID := uint8(10)

	pti := uint8(5)

	snssai := &models.Snssai{
		Sst: 1,
		Sd:  "010203",
	}

	dnn := "internet"

	pco := &context.ProtocolConfigurationOptions{}

	msg, err := context.BuildGSMPDUSessionEstablishmentAccept(smPolicyUpdates, pduSessionID, pti, snssai, dnn, pco, 0, nil, nil)
	if err != nil {
		t.Fatalf("failed to build GSM PDU Session Establishment Accept: %v", err)
	}

	nasMsg := new(nas.Message)

	err = nasMsg.PlainNasDecode(&msg)
	if err != nil {
		t.Fatalf("failed to decode NAS message: %v", err)
	}

	// check that the SD IE is not present
	if nasMsg.GsmMessage.PDUSessionEstablishmentAccept.SNSSAI == nil {
		t.Errorf("SNSSAI IE is missing")
	}

	if nasMsg.PDUSessionEstablishmentAccept.SNSSAI.GetLen() != 4 {
		t.Errorf("expected SNSSAI length 1, got %d", nasMsg.PDUSessionEstablishmentAccept.SNSSAI.GetLen())
	}

	if nasMsg.GsmMessage.PDUSessionEstablishmentAccept.SNSSAI.GetSST() != 1 {
		t.Errorf("expected SST 1, got %d", nasMsg.GsmMessage.PDUSessionEstablishmentAccept.SNSSAI.GetSST())
	}

	if nasMsg.GsmMessage.PDUSessionEstablishmentAccept.SNSSAI.GetSD() != [3]uint8{1, 2, 3} {
		t.Errorf("expected SD [1,2,3], got %v", nasMsg.GsmMessage.PDUSessionEstablishmentAccept.SNSSAI.GetSD())
	}
}

func TestBuildGSMPDUSessionEstablishmentAccept_WithoutSD(t *testing.T) {
	smPolicyUpdates := &qos.PolicyUpdate{
		SessRuleUpdate: &qos.SessRulesUpdate{
			ActiveSessRule: &models.SessionRule{
				AuthSessAmbr: &models.Ambr{
					Uplink:   "1 Gbps",
					Downlink: "1 Gbps",
				},
				AuthDefQos: &models.AuthorizedDefaultQos{
					Var5qi: 9,
				},
			},
		},
		QosFlowUpdate: &qos.QosFlowsUpdate{
			Add: &models.QosData{
				QFI:    1,
				Var5qi: 9,
			},
		},
	}
	pduSessionID := uint8(10)

	pti := uint8(5)

	snssai := &models.Snssai{
		Sst: 1,
		Sd:  "",
	}

	dnn := "internet"

	pco := &context.ProtocolConfigurationOptions{}

	msg, err := context.BuildGSMPDUSessionEstablishmentAccept(smPolicyUpdates, pduSessionID, pti, snssai, dnn, pco, 0, nil, nil)
	if err != nil {
		t.Fatalf("failed to build GSM PDU Session Establishment Accept: %v", err)
	}

	nasMsg := new(nas.Message)

	err = nasMsg.PlainNasDecode(&msg)
	if err != nil {
		t.Fatalf("failed to decode NAS message: %v", err)
	}

	// check that the SD IE is not present
	if nasMsg.GsmMessage.PDUSessionEstablishmentAccept.SNSSAI == nil {
		t.Errorf("SNSSAI IE is missing")
	}

	if nasMsg.GsmMessage.PDUSessionEstablishmentAccept.SNSSAI.GetLen() != 1 {
		t.Errorf("expected SNSSAI length 1, got %d", nasMsg.GsmMessage.PDUSessionEstablishmentAccept.SNSSAI.GetLen())
	}

	if nasMsg.GsmMessage.PDUSessionEstablishmentAccept.SNSSAI.GetSST() != 1 {
		t.Errorf("expected SST 1, got %d", nasMsg.GsmMessage.PDUSessionEstablishmentAccept.SNSSAI.GetSST())
	}

	if nasMsg.GsmMessage.PDUSessionEstablishmentAccept.SNSSAI.GetSD() != [3]uint8{0, 0, 0} {
		t.Errorf("expected SD [0,0,0], got %v", nasMsg.GsmMessage.PDUSessionEstablishmentAccept.SNSSAI.GetSD())
	}
}
