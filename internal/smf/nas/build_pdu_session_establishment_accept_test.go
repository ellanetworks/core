// Copyright 2025 Ella Networks

package nas_test

import (
	"net"
	"testing"

	"github.com/ellanetworks/core/internal/models"
	smfNas "github.com/ellanetworks/core/internal/smf/nas"
	"github.com/free5gc/nas"
	"github.com/free5gc/nas/nasMessage"
)

func TestBuildGSMPDUSessionEstablishmentAccept_WithSD(t *testing.T) {
	ambr := &models.Ambr{
		Uplink:   "1 Gbps",
		Downlink: "1 Gbps",
	}
	qosData := &models.QosData{
		QFI:    1,
		Var5qi: 9,
	}

	pduSessionID := uint8(10)

	pti := uint8(5)

	snssai := &models.Snssai{
		Sst: 1,
		Sd:  "010203",
	}

	dnn := "internet"

	pco := &smfNas.ProtocolConfigurationOptions{}

	msg, err := smfNas.BuildGSMPDUSessionEstablishmentAccept(ambr, qosData, pduSessionID, pti, snssai, dnn, pco, nil, 0, 0, nil)
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
	ambr := &models.Ambr{
		Uplink:   "1 Gbps",
		Downlink: "1 Gbps",
	}
	qosData := &models.QosData{
		QFI:    1,
		Var5qi: 9,
	}

	pduSessionID := uint8(10)

	pti := uint8(5)

	snssai := &models.Snssai{
		Sst: 1,
		Sd:  "",
	}

	dnn := "internet"

	pco := &smfNas.ProtocolConfigurationOptions{}

	msg, err := smfNas.BuildGSMPDUSessionEstablishmentAccept(ambr, qosData, pduSessionID, pti, snssai, dnn, pco, nil, 0, 0, nil)
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

func TestBuildGSMPDUSessionEstablishmentAccept_IPv4Address(t *testing.T) {
	ambr := &models.Ambr{Uplink: "1 Gbps", Downlink: "1 Gbps"}
	qosData := &models.QosData{QFI: 1, Var5qi: 9}
	snssai := &models.Snssai{Sst: 1}
	pco := &smfNas.ProtocolConfigurationOptions{}

	addrs := &smfNas.PDUSessionAddresses{
		PDUSessionType: nasMessage.PDUSessionTypeIPv4,
		IPv4Address:    net.IP{10, 45, 0, 1},
	}

	msg, err := smfNas.BuildGSMPDUSessionEstablishmentAccept(ambr, qosData, 5, 1, snssai, "internet", pco, nil, 0, 0, addrs)
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	nasMsg := new(nas.Message)
	if err := nasMsg.PlainNasDecode(&msg); err != nil {
		t.Fatalf("Decode failed: %v", err)
	}

	if nasMsg.PDUSessionEstablishmentAccept.GetPDUSessionTypeValue() != nasMessage.PDUSessionTypeIPv4 {
		t.Errorf("expected PDU session type IPv4, got %d", nasMsg.PDUSessionEstablishmentAccept.GetPDUSessionTypeValue())
	}

	if nasMsg.PDUAddress == nil {
		t.Fatal("PDUAddress IE is nil")
	}

	// PDU address length should be 5 (1 type + 4 addr).
	if nasMsg.PDUAddress.GetLen() != 5 {
		t.Errorf("expected PDUAddress len 5, got %d", nasMsg.PDUAddress.GetLen())
	}

	addrInfo := nasMsg.GetPDUAddressInformation()
	if addrInfo[0] != 10 || addrInfo[1] != 45 || addrInfo[2] != 0 || addrInfo[3] != 1 {
		t.Errorf("expected IPv4 10.45.0.1, got %v", addrInfo[:4])
	}
}

func TestBuildGSMPDUSessionEstablishmentAccept_IPv6IID(t *testing.T) {
	ambr := &models.Ambr{Uplink: "1 Gbps", Downlink: "1 Gbps"}
	qosData := &models.QosData{QFI: 1, Var5qi: 9}
	snssai := &models.Snssai{Sst: 1}
	pco := &smfNas.ProtocolConfigurationOptions{}

	iid := [8]byte{0xDE, 0xAD, 0xBE, 0xEF, 0xCA, 0xFE, 0xBA, 0xBE}
	addrs := &smfNas.PDUSessionAddresses{
		PDUSessionType: nasMessage.PDUSessionTypeIPv6,
		IPv6IID:        iid,
	}

	msg, err := smfNas.BuildGSMPDUSessionEstablishmentAccept(ambr, qosData, 5, 1, snssai, "internet", pco, nil, 0, 0, addrs)
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	nasMsg := new(nas.Message)
	if err := nasMsg.PlainNasDecode(&msg); err != nil {
		t.Fatalf("Decode failed: %v", err)
	}

	if nasMsg.PDUSessionEstablishmentAccept.GetPDUSessionTypeValue() != nasMessage.PDUSessionTypeIPv6 {
		t.Errorf("expected PDU session type IPv6, got %d", nasMsg.PDUSessionEstablishmentAccept.GetPDUSessionTypeValue())
	}

	if nasMsg.PDUAddress == nil {
		t.Fatal("PDUAddress IE is nil")
	}

	// PDU address length should be 9 (1 type + 8 IID).
	if nasMsg.PDUAddress.GetLen() != 9 {
		t.Errorf("expected PDUAddress len 9, got %d", nasMsg.PDUAddress.GetLen())
	}

	addrInfo := nasMsg.GetPDUAddressInformation()
	for i := 0; i < 8; i++ {
		if addrInfo[i] != iid[i] {
			t.Errorf("IID byte %d: expected 0x%02X, got 0x%02X", i, iid[i], addrInfo[i])
		}
	}
}

func TestBuildGSMPDUSessionEstablishmentAccept_IPv4v6(t *testing.T) {
	ambr := &models.Ambr{Uplink: "1 Gbps", Downlink: "1 Gbps"}
	qosData := &models.QosData{QFI: 1, Var5qi: 9}
	snssai := &models.Snssai{Sst: 1}
	pco := &smfNas.ProtocolConfigurationOptions{}

	iid := [8]byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08}
	addrs := &smfNas.PDUSessionAddresses{
		PDUSessionType: nasMessage.PDUSessionTypeIPv4IPv6,
		IPv4Address:    net.IP{192, 168, 1, 10},
		IPv6IID:        iid,
	}

	msg, err := smfNas.BuildGSMPDUSessionEstablishmentAccept(ambr, qosData, 5, 1, snssai, "internet", pco, nil, 0, 0, addrs)
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	nasMsg := new(nas.Message)
	if err := nasMsg.PlainNasDecode(&msg); err != nil {
		t.Fatalf("Decode failed: %v", err)
	}

	if nasMsg.PDUSessionEstablishmentAccept.GetPDUSessionTypeValue() != nasMessage.PDUSessionTypeIPv4IPv6 {
		t.Errorf("expected PDU session type IPv4v6, got %d", nasMsg.PDUSessionEstablishmentAccept.GetPDUSessionTypeValue())
	}

	if nasMsg.PDUAddress == nil {
		t.Fatal("PDUAddress IE is nil")
	}

	// PDU address length should be 13 (1 type + 8 IID + 4 IPv4).
	if nasMsg.PDUAddress.GetLen() != 13 {
		t.Errorf("expected PDUAddress len 13, got %d", nasMsg.PDUAddress.GetLen())
	}

	addrInfo := nasMsg.GetPDUAddressInformation()
	// First 8 bytes: IID
	for i := 0; i < 8; i++ {
		if addrInfo[i] != iid[i] {
			t.Errorf("IID byte %d: expected 0x%02X, got 0x%02X", i, iid[i], addrInfo[i])
		}
	}

	// Last 4 bytes: IPv4
	if addrInfo[8] != 192 || addrInfo[9] != 168 || addrInfo[10] != 1 || addrInfo[11] != 10 {
		t.Errorf("expected IPv4 192.168.1.10, got %v", addrInfo[8:12])
	}
}

func TestBuildGSMPDUSessionEstablishmentAccept_IPv4DNS(t *testing.T) {
	pco := &smfNas.ProtocolConfigurationOptions{
		DNSIPv4Request: true,
	}

	msg, err := smfNas.BuildGSMPDUSessionEstablishmentAccept(
		&models.Ambr{Uplink: "1 Gbps", Downlink: "1 Gbps"},
		&models.QosData{QFI: 1, Var5qi: 9},
		5, 1, &models.Snssai{Sst: 1}, "internet",
		pco, net.ParseIP("8.8.8.8"), 0, 0, nil,
	)
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	nasMsg := new(nas.Message)
	if err := nasMsg.PlainNasDecode(&msg); err != nil {
		t.Fatalf("Decode failed: %v", err)
	}

	if nasMsg.PDUSessionEstablishmentAccept.ExtendedProtocolConfigurationOptions == nil {
		t.Fatal("expected EPCO IE to be present for IPv4 DNS")
	}
}

func TestBuildGSMPDUSessionEstablishmentAccept_IPv6DNS(t *testing.T) {
	pco := &smfNas.ProtocolConfigurationOptions{
		DNSIPv6Request: true,
	}

	msg, err := smfNas.BuildGSMPDUSessionEstablishmentAccept(
		&models.Ambr{Uplink: "1 Gbps", Downlink: "1 Gbps"},
		&models.QosData{QFI: 1, Var5qi: 9},
		5, 1, &models.Snssai{Sst: 1}, "internet",
		pco, net.ParseIP("2001:4860:4860::8888"), 0, 0, nil,
	)
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	nasMsg := new(nas.Message)
	if err := nasMsg.PlainNasDecode(&msg); err != nil {
		t.Fatalf("Decode failed: %v", err)
	}

	if nasMsg.PDUSessionEstablishmentAccept.ExtendedProtocolConfigurationOptions == nil {
		t.Fatal("expected EPCO IE to be present for IPv6 DNS")
	}
}

func TestBuildGSMPDUSessionEstablishmentAccept_Cause(t *testing.T) {
	ambr := &models.Ambr{Uplink: "1 Gbps", Downlink: "1 Gbps"}
	qosData := &models.QosData{QFI: 1, Var5qi: 9}
	snssai := &models.Snssai{Sst: 1}
	pco := &smfNas.ProtocolConfigurationOptions{}

	addrs := &smfNas.PDUSessionAddresses{
		PDUSessionType: nasMessage.PDUSessionTypeIPv4,
		IPv4Address:    net.IP{10, 0, 0, 1},
	}

	// Test with no cause (normal case)
	msg, err := smfNas.BuildGSMPDUSessionEstablishmentAccept(ambr, qosData, 1, 1, snssai, "internet", pco, nil, 0, 0, addrs)
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	nasMsg := new(nas.Message)
	if err := nasMsg.PlainNasDecode(&msg); err != nil {
		t.Fatalf("Decode failed: %v", err)
	}

	if nasMsg.PDUSessionEstablishmentAccept.Cause5GSM != nil {
		t.Errorf("expected no cause IE, got %v", nasMsg.PDUSessionEstablishmentAccept.Cause5GSM)
	}

	// Test with IPv4-only cause (#50 = 0x32)
	msg, err = smfNas.BuildGSMPDUSessionEstablishmentAccept(ambr, qosData, 1, 1, snssai, "internet", pco, nil, 0, nasMessage.Cause5GSMPDUSessionTypeIPv4OnlyAllowed, addrs)
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	nasMsg = new(nas.Message)
	if err := nasMsg.PlainNasDecode(&msg); err != nil {
		t.Fatalf("Decode failed: %v", err)
	}

	if nasMsg.PDUSessionEstablishmentAccept.Cause5GSM == nil {
		t.Fatal("expected cause IE to be present")
	}

	if nasMsg.PDUSessionEstablishmentAccept.GetCauseValue() != nasMessage.Cause5GSMPDUSessionTypeIPv4OnlyAllowed {
		t.Errorf("expected cause %d, got %d", nasMessage.Cause5GSMPDUSessionTypeIPv4OnlyAllowed, nasMsg.PDUSessionEstablishmentAccept.GetCauseValue())
	}

	// Test with IPv6-only cause (#51 = 0x33)
	addrs.PDUSessionType = nasMessage.PDUSessionTypeIPv6
	addrs.IPv4Address = nil
	iid := [8]byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08}
	addrs.IPv6IID = iid

	msg, err = smfNas.BuildGSMPDUSessionEstablishmentAccept(ambr, qosData, 1, 1, snssai, "internet", pco, nil, 0, nasMessage.Cause5GSMPDUSessionTypeIPv6OnlyAllowed, addrs)
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	nasMsg = new(nas.Message)
	if err := nasMsg.PlainNasDecode(&msg); err != nil {
		t.Fatalf("Decode failed: %v", err)
	}

	if nasMsg.PDUSessionEstablishmentAccept.Cause5GSM == nil {
		t.Fatal("expected cause IE to be present")
	}

	if nasMsg.PDUSessionEstablishmentAccept.GetCauseValue() != nasMessage.Cause5GSMPDUSessionTypeIPv6OnlyAllowed {
		t.Errorf("expected cause %d, got %d", nasMessage.Cause5GSMPDUSessionTypeIPv6OnlyAllowed, nasMsg.PDUSessionEstablishmentAccept.GetCauseValue())
	}
}
