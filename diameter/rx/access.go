// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package rx

import (
	"bytes"
	"math"
	"net/netip"
	"time"

	"github.com/ellanetworks/core/diameter"
	"github.com/ellanetworks/core/diameter/tgpp"
)

const (
	maxANGWAddresses = 2
	msTimeZoneOctets = 2
)

type AccessNetwork struct {
	IPCANType     *IPCANType
	RATType       *RATType
	ANTrusted     *ANTrusted
	ANGWAddresses []netip.Addr
}

type ServingNetwork struct {
	PLMN string
	NID  []byte
}

type UserLocation struct {
	UserLocationInfo         []byte
	UserLocationInfoTime     time.Time
	MSTimeZone               []byte
	TWANIdentifier           []byte
	UELocalIPAddress         netip.Addr
	UDPSourcePort            uint16
	TCPSourcePort            uint16
	ServingSatelliteIdentity []byte
	RANNASReleaseCauses      [][]byte
}

func (n AccessNetwork) avps() ([]diameter.AVP, error) {
	switch {
	case !validEnum(n.IPCANType):
		return nil, invalidf("IP-CAN-Type %s", *n.IPCANType)
	case !validEnum(n.ANTrusted):
		return nil, invalidf("AN-Trusted %s", *n.ANTrusted)
	case len(n.ANGWAddresses) > maxANGWAddresses:
		return nil, invalidf("%d AN-GW-Address, at most %d", len(n.ANGWAddresses), maxANGWAddresses)
	}

	var avps []diameter.AVP

	for _, a := range n.ANGWAddresses {
		address, err := addressAVP(tgpp.AVPANGWAddress, 0, a)
		if err != nil {
			return nil, err
		}

		avps = append(avps, address)
	}

	avps = appendOptional(avps, tgpp.AVPANTrusted, 0, (*uint32)(n.ANTrusted))
	avps = appendOptional(avps, tgpp.AVPIPCANType, diameter.AVPFlagMandatory, (*uint32)(n.IPCANType))
	avps = appendOptional(avps, tgpp.AVPRATType, 0, (*uint32)(n.RATType))

	return avps, nil
}

func parseAccessNetwork(avps []diameter.AVP) (AccessNetwork, error) {
	var (
		n   AccessNetwork
		err error
	)

	addresses := diameter.FindAll(avps, tgpp.AVPANGWAddress, tgpp.VendorID)
	if len(addresses) > maxANGWAddresses {
		return AccessNetwork{}, diameter.NewAVPError(diameter.ResultAVPOccursTooManyTimes, addresses[maxANGWAddresses])
	}

	for _, a := range addresses {
		address, err := addressValue(a)
		if err != nil {
			return AccessNetwork{}, err
		}

		n.ANGWAddresses = append(n.ANGWAddresses, address)
	}

	if n.ANTrusted, err = optionalEnum[ANTrusted](avps, tgpp.AVPANTrusted, tgpp.VendorID); err != nil {
		return AccessNetwork{}, err
	}

	if n.IPCANType, err = optionalEnum[IPCANType](avps, tgpp.AVPIPCANType, tgpp.VendorID); err != nil {
		return AccessNetwork{}, err
	}

	rat, err := optionalUint32(avps, tgpp.AVPRATType, tgpp.VendorID)
	if err != nil {
		return AccessNetwork{}, err
	}

	n.RATType = (*RATType)(rat)

	return n, nil
}

func validPLMN(s string) bool {
	if len(s) != 5 && len(s) != 6 {
		return false
	}

	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}

	return true
}

func (n ServingNetwork) avps() ([]diameter.AVP, error) {
	if n.PLMN != "" && !validPLMN(n.PLMN) {
		return nil, invalidf("3GPP-SGSN-MCC-MNC %q", n.PLMN)
	}

	var avps []diameter.AVP

	if n.PLMN != "" {
		avps = append(avps, diameter.UTF8String(tgpp.AVP3GPPSGSNMCCMNC, 0, tgpp.VendorID, n.PLMN))
	}

	return appendOctets(avps, AVPNID, n.NID), nil
}

func parseServingNetwork(avps []diameter.AVP) (ServingNetwork, error) {
	var (
		n   ServingNetwork
		err error
	)

	if a, ok := diameter.Find(avps, tgpp.AVP3GPPSGSNMCCMNC, tgpp.VendorID); ok {
		if n.PLMN = a.UTF8String(); !validPLMN(n.PLMN) {
			return ServingNetwork{}, tgpp.InvalidAVP(a)
		}
	}

	if n.NID, err = optionalOctets(avps, AVPNID, tgpp.VendorID); err != nil {
		return ServingNetwork{}, err
	}

	return n, nil
}

func (l UserLocation) avps() ([]diameter.AVP, error) {
	switch {
	case !l.UserLocationInfoTime.IsZero() && !validTime(l.UserLocationInfoTime):
		return nil, invalidf("User-Location-Info-Time %s", l.UserLocationInfoTime)
	case l.MSTimeZone != nil && len(l.MSTimeZone) != msTimeZoneOctets:
		return nil, invalidf("3GPP-MS-TimeZone of %d octets", len(l.MSTimeZone))
	case l.UELocalIPAddress.IsValid() && !validAddress(l.UELocalIPAddress):
		return nil, invalidf("UE-Local-IP-Address %s", l.UELocalIPAddress)
	}

	for _, c := range l.RANNASReleaseCauses {
		if len(c) == 0 {
			return nil, invalidf("empty RAN-NAS-Release-Cause")
		}
	}

	avps := appendOctets(nil, tgpp.AVP3GPPUserLocationInfo, l.UserLocationInfo)

	if !l.UserLocationInfoTime.IsZero() {
		avps = append(avps, diameter.Time(tgpp.AVPUserLocationInfoTime, 0, tgpp.VendorID, l.UserLocationInfoTime))
	}

	avps = appendOctets(avps, tgpp.AVP3GPPMSTimeZone, l.MSTimeZone)
	avps = appendOctets(avps, AVPServingSatelliteIdentity, l.ServingSatelliteIdentity)

	for _, c := range l.RANNASReleaseCauses {
		avps = append(avps, diameter.OctetString(tgpp.AVPRANNASReleaseCause, 0, tgpp.VendorID, c))
	}

	avps = appendOctets(avps, tgpp.AVPTWANIdentifier, l.TWANIdentifier)
	avps = appendPort(avps, tgpp.AVPTCPSourcePort, l.TCPSourcePort)
	avps = appendPort(avps, tgpp.AVPUDPSourcePort, l.UDPSourcePort)

	if l.UELocalIPAddress.IsValid() {
		avps = append(avps, diameter.Address(tgpp.AVPUELocalIPAddress, 0, tgpp.VendorID, l.UELocalIPAddress))
	}

	return avps, nil
}

func appendPort(avps []diameter.AVP, code uint32, port uint16) []diameter.AVP {
	if port != 0 {
		avps = append(avps, diameter.Unsigned32(code, 0, tgpp.VendorID, uint32(port)))
	}

	return avps
}

func parseUserLocation(avps []diameter.AVP) (UserLocation, error) {
	var (
		l   UserLocation
		err error
	)

	for _, f := range []struct {
		code uint32
		dst  *[]byte
	}{
		{tgpp.AVP3GPPUserLocationInfo, &l.UserLocationInfo},
		{tgpp.AVP3GPPMSTimeZone, &l.MSTimeZone},
		{AVPServingSatelliteIdentity, &l.ServingSatelliteIdentity},
		{tgpp.AVPTWANIdentifier, &l.TWANIdentifier},
	} {
		if *f.dst, err = optionalOctets(avps, f.code, tgpp.VendorID); err != nil {
			return UserLocation{}, err
		}
	}

	if l.MSTimeZone != nil && len(l.MSTimeZone) != msTimeZoneOctets {
		a, _ := diameter.Find(avps, tgpp.AVP3GPPMSTimeZone, tgpp.VendorID)
		return UserLocation{}, tgpp.InvalidLength(a)
	}

	if l.UserLocationInfoTime, err = optionalTime(avps, tgpp.AVPUserLocationInfoTime); err != nil {
		return UserLocation{}, err
	}

	for _, a := range diameter.FindAll(avps, tgpp.AVPRANNASReleaseCause, tgpp.VendorID) {
		if len(a.Data) == 0 {
			return UserLocation{}, tgpp.InvalidAVP(a)
		}

		l.RANNASReleaseCauses = append(l.RANNASReleaseCauses, bytes.Clone(a.Data))
	}

	for _, f := range []struct {
		code uint32
		dst  *uint16
	}{
		{tgpp.AVPTCPSourcePort, &l.TCPSourcePort},
		{tgpp.AVPUDPSourcePort, &l.UDPSourcePort},
	} {
		if *f.dst, err = optionalPort(avps, f.code); err != nil {
			return UserLocation{}, err
		}
	}

	if l.UELocalIPAddress, err = optionalAddress(avps, tgpp.AVPUELocalIPAddress); err != nil {
		return UserLocation{}, err
	}

	return l, nil
}

func optionalPort(avps []diameter.AVP, code uint32) (uint16, error) {
	v, err := optionalUint32(avps, code, tgpp.VendorID)
	if err != nil || v == nil {
		return 0, err
	}

	if *v == 0 || *v > math.MaxUint16 {
		a, _ := diameter.Find(avps, code, tgpp.VendorID)
		return 0, tgpp.InvalidAVP(a)
	}

	return uint16(*v), nil
}

var (
	accessNetworkRules = diameter.Rules{
		vendorKey(tgpp.AVPANGWAddress): {Multiple: true},
		vendorKey(tgpp.AVPANTrusted):   {},
		vendorKey(tgpp.AVPIPCANType):   {},
		vendorKey(tgpp.AVPRATType):     {},
	}
	servingNetworkRules = diameter.Rules{
		vendorKey(tgpp.AVP3GPPSGSNMCCMNC): {},
		vendorKey(AVPNID):                 {},
	}
	userLocationRules = diameter.Rules{
		vendorKey(tgpp.AVP3GPPUserLocationInfo): {},
		vendorKey(tgpp.AVPUserLocationInfoTime): {},
		vendorKey(tgpp.AVP3GPPMSTimeZone):       {},
		vendorKey(AVPServingSatelliteIdentity):  {},
		vendorKey(tgpp.AVPRANNASReleaseCause):   {Multiple: true},
		vendorKey(avp5GSRANNASReleaseCause):     {Multiple: true},
		vendorKey(tgpp.AVPTWANIdentifier):       {},
		vendorKey(tgpp.AVPTCPSourcePort):        {},
		vendorKey(tgpp.AVPUDPSourcePort):        {},
		vendorKey(tgpp.AVPUELocalIPAddress):     {},
		vendorKey(avpWirelineUserLocationInfo):  {},
		vendorKey(tgpp.AVPNetLocAccessSupport):  {},
	}
)
