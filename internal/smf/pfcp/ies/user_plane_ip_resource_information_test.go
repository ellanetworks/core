package ies_test

import (
	"testing"

	"github.com/wmnsk/go-pfcp/ie"
	"github.com/yeastengine/ella/internal/smf/pfcp/ies"
)

type Flag uint8

// setBit sets the bit at the given position to the specified value (true or false)
// Positions go from 1 to 8
func (f *Flag) setBit(position uint8) {
	if position < 1 || position > 8 {
		return
	}
	*f |= 1 << (position - 1)
}

func CreateFlags(v4 bool, v6 bool, ni bool, si bool) uint8 {
	flags := new(Flag)
	if v4 {
		flags.setBit(1)
	}
	if v6 {
		flags.setBit(2)
	}

	if ni {
		flags.setBit(6)
	}

	if si {
		flags.setBit(7)
	}

	return uint8(*flags)
}

func TestUnmarshalUEIPInformationBinaryOnlySourceInterface(t *testing.T) {
	const ipv4Address = "1.2.3.4"
	flags := CreateFlags(true, false, false, true)

	userplaneIE := ie.NewUserPlaneIPResourceInformation(flags, 0, ipv4Address, "", "", ie.SrcInterfaceAccess)

	ueIpInfo, err := ies.UnmarshalUEIPInformationBinary(userplaneIE.Payload)
	if err != nil {
		t.Errorf("error unmarshalling UE IP Information: %v", err)
	}

	if ueIpInfo.TeidRange != 0 {
		t.Errorf("expected TEIDRange 0, got %d", ueIpInfo.TeidRange)
	}

	if ueIpInfo.Ipv4Address.String() != ipv4Address {
		t.Errorf("expected IP address %v got %s", ipv4Address, ueIpInfo.Ipv4Address.String())
	}

	if ueIpInfo.Ipv6Address != nil {
		t.Errorf("expected nil IPv6 address, got %s", ueIpInfo.Ipv6Address.String())
	}

	if ueIpInfo.SourceInterface != ie.SrcInterfaceAccess {
		t.Errorf("expected Source Interface Access, got %d", ueIpInfo.SourceInterface)
	}

	if string(ueIpInfo.NetworkInstance) != "" {
		t.Errorf("expected empty network instance, got %s", ueIpInfo.NetworkInstance)
	}
}

func TestUnmarshalUEIPInformationBinaryOnlyNetworkInstance(t *testing.T) {
	flags := CreateFlags(true, false, true, false)
	userplaneIE := ie.NewUserPlaneIPResourceInformation(flags, 0, "1.2.3.4", "", string(ie.NewNetworkInstanceFQDN("internet").Payload), 0)

	ueIpInfo, err := ies.UnmarshalUEIPInformationBinary(userplaneIE.Payload)
	if err != nil {
		t.Errorf("error unmarshalling UE IP Information: %v", err)
	}

	if ueIpInfo.TeidRange != 0 {
		t.Errorf("expected TEIDRange 0, got %d", ueIpInfo.TeidRange)
	}

	if ueIpInfo.Ipv4Address.String() != "1.2.3.4" {
		t.Errorf("expected IP address 1.2.3.4 got %s", ueIpInfo.Ipv4Address.String())
	}

	if ueIpInfo.Ipv6Address != nil {
		t.Errorf("expected nil IPv6 address, got %s", ueIpInfo.Ipv6Address.String())
	}

	if ueIpInfo.SourceInterface != 0 {
		t.Errorf("expected Source Interface 0, got %d", ueIpInfo.SourceInterface)
	}

	if string(ueIpInfo.NetworkInstance) != "internet" {
		t.Errorf("expected network instance internet, got %s", ueIpInfo.NetworkInstance)
	}
}

func TestUnmarshalUEIPInformationBinary(t *testing.T) {
	flags := CreateFlags(true, false, true, true)
	userplaneIE := ie.NewUserPlaneIPResourceInformation(flags, 0, "1.2.3.4", "", string(ie.NewNetworkInstanceFQDN("internet").Payload), ie.SrcInterfaceAccess)

	ueIpInfo, err := ies.UnmarshalUEIPInformationBinary(userplaneIE.Payload)
	if err != nil {
		t.Errorf("error unmarshalling UE IP Information: %v", err)
	}

	if ueIpInfo.TeidRange != 0 {
		t.Errorf("expected TEIDRange 0, got %d", ueIpInfo.TeidRange)
	}

	if ueIpInfo.Ipv4Address.String() != "1.2.3.4" {
		t.Errorf("expected IP address 1.2.3.4 got %s", ueIpInfo.Ipv4Address.String())
	}

	if ueIpInfo.Ipv6Address != nil {
		t.Errorf("expected nil IPv6 address, got %s", ueIpInfo.Ipv6Address.String())
	}

	if ueIpInfo.SourceInterface != ie.SrcInterfaceAccess {
		t.Errorf("expected Source Interface Access, got %d", ueIpInfo.SourceInterface)
	}

	if string(ueIpInfo.NetworkInstance) != "internet" {
		t.Errorf("expected network instance internet, got %s", ueIpInfo.NetworkInstance)
	}
}
