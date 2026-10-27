// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package rx

import (
	"bytes"
	"encoding/binary"
	"net/netip"
	"time"

	"github.com/ellanetworks/core/diameter"
	"github.com/ellanetworks/core/diameter/tgpp"
)

const (
	addressFamilyIPv4 uint16 = 1
	addressFamilyIPv6 uint16 = 2
)

type enum interface {
	~uint32
	Known() bool
}

var (
	maxRequestedUL = tgpp.MaxRequestedBandwidthUL
	maxRequestedDL = tgpp.MaxRequestedBandwidthDL
	minRequestedUL = tgpp.BandwidthAVPs{BPS: AVPMinRequestedBandwidthUL, KBPS: AVPExtendedMinRequestedBWUL}
	minRequestedDL = tgpp.BandwidthAVPs{BPS: AVPMinRequestedBandwidthDL, KBPS: AVPExtendedMinRequestedBWDL}
	maxSupportedUL = tgpp.BandwidthAVPs{BPS: AVPMaxSupportedBandwidthUL, KBPS: AVPExtendedMaxSupportedBWUL}
	maxSupportedDL = tgpp.BandwidthAVPs{BPS: AVPMaxSupportedBandwidthDL, KBPS: AVPExtendedMaxSupportedBWDL}
	minDesiredUL   = tgpp.BandwidthAVPs{BPS: AVPMinDesiredBandwidthUL, KBPS: AVPExtendedMinDesiredBWUL}
	minDesiredDL   = tgpp.BandwidthAVPs{BPS: AVPMinDesiredBandwidthDL, KBPS: AVPExtendedMinDesiredBWDL}
)

func vendorKey(code uint32) diameter.AVPKey {
	return diameter.AVPKey{Code: code, VendorID: tgpp.VendorID}
}

func vendorUnsigned(code, v uint32) diameter.AVP {
	return diameter.Unsigned32(code, diameter.AVPFlagMandatory, tgpp.VendorID, v)
}

func reservationPriorityAVP(p ReservationPriority) diameter.AVP {
	return diameter.Unsigned32(avpReservationPriority, 0, etsiVendorID, uint32(p))
}

func vendorOctets(code uint32, v []byte) diameter.AVP {
	return diameter.OctetString(code, diameter.AVPFlagMandatory, tgpp.VendorID, v)
}

func vendorGrouped(code uint32, avps ...diameter.AVP) diameter.AVP {
	return diameter.Grouped(code, diameter.AVPFlagMandatory, tgpp.VendorID, avps...)
}

func appendOptional(avps []diameter.AVP, code uint32, flags uint8, v *uint32) []diameter.AVP {
	if v != nil {
		avps = append(avps, diameter.Unsigned32(code, flags, tgpp.VendorID, *v))
	}

	return avps
}

func appendOctets(avps []diameter.AVP, code uint32, v []byte) []diameter.AVP {
	if len(v) > 0 {
		avps = append(avps, diameter.OctetString(code, 0, tgpp.VendorID, v))
	}

	return avps
}

func appendBandwidth(avps []diameter.AVP, codes tgpp.BandwidthAVPs, b *tgpp.Bandwidth) ([]diameter.AVP, error) {
	if b == nil {
		return avps, nil
	}

	out, err := codes.Append(avps, *b)
	if err != nil {
		return nil, invalidf("%w", err)
	}

	return out, nil
}

func parseBandwidth(avps []diameter.AVP, codes tgpp.BandwidthAVPs) (*tgpp.Bandwidth, error) {
	return codes.Parse(avps)
}

func requiredUint32(avps []diameter.AVP, code, vendorID uint32) (uint32, error) {
	a, _ := diameter.Find(avps, code, vendorID)
	return tgpp.Unsigned32(a)
}

func optionalUint32(avps []diameter.AVP, code, vendorID uint32) (*uint32, error) {
	a, ok := diameter.Find(avps, code, vendorID)
	if !ok {
		return nil, nil
	}

	v, err := tgpp.Unsigned32(a)
	if err != nil {
		return nil, err
	}

	return &v, nil
}

func uint64Value(a diameter.AVP) (uint64, error) {
	v, err := a.Unsigned64()
	if err != nil {
		return 0, tgpp.InvalidLength(a)
	}

	return v, nil
}

func enumValue[T enum](a diameter.AVP) (T, error) {
	v, err := tgpp.Unsigned32(a)
	if err != nil {
		return 0, err
	}

	if !T(v).Known() {
		return 0, tgpp.InvalidAVP(a)
	}

	return T(v), nil
}

func optionalEnum[T enum](avps []diameter.AVP, code, vendorID uint32) (*T, error) {
	a, ok := diameter.Find(avps, code, vendorID)
	if !ok {
		return nil, nil
	}

	v, err := enumValue[T](a)
	if err != nil {
		return nil, err
	}

	return &v, nil
}

func defaultEnum[T enum](avps []diameter.AVP, code, vendorID uint32) (T, error) {
	v, err := optionalEnum[T](avps, code, vendorID)
	if err != nil || v == nil {
		return 0, err
	}

	return *v, nil
}

func enumList[T enum](avps []diameter.AVP, code, vendorID uint32) ([]T, error) {
	var out []T

	for _, a := range diameter.FindAll(avps, code, vendorID) {
		v, err := enumValue[T](a)
		if err != nil {
			return nil, err
		}

		out = append(out, v)
	}

	return out, nil
}

func enumListAVPs[T enum](code uint32, flags uint8, values []T) ([]diameter.AVP, error) {
	var avps []diameter.AVP

	for _, v := range values {
		if !v.Known() {
			return nil, invalidf("AVP %d value %d", code, uint32(v))
		}

		avps = append(avps, diameter.Unsigned32(code, flags, tgpp.VendorID, uint32(v)))
	}

	return avps, nil
}

func validEnum[T enum](v *T) bool {
	return v == nil || (*v).Known()
}

func optionalOctets(avps []diameter.AVP, code, vendorID uint32) ([]byte, error) {
	a, ok := diameter.Find(avps, code, vendorID)
	if !ok {
		return nil, nil
	}

	if len(a.Data) == 0 {
		return nil, tgpp.InvalidAVP(a)
	}

	return bytes.Clone(a.Data), nil
}

func optionalString(avps []diameter.AVP, code, vendorID uint32) (string, error) {
	v, err := optionalOctets(avps, code, vendorID)
	return string(v), err
}

func optionalTime(avps []diameter.AVP, code uint32) (time.Time, error) {
	a, ok := diameter.Find(avps, code, tgpp.VendorID)
	if !ok {
		return time.Time{}, nil
	}

	t, err := a.Time()
	if err != nil {
		return time.Time{}, tgpp.InvalidLength(a)
	}

	return t, nil
}

func validAddress(a netip.Addr) bool {
	return a.IsValid() && a.Zone() == ""
}

func addressAVP(code uint32, flags uint8, a netip.Addr) (diameter.AVP, error) {
	if !validAddress(a) {
		return diameter.AVP{}, invalidf("address %s", a)
	}

	return diameter.Address(code, flags, tgpp.VendorID, a), nil
}

func addressValue(a diameter.AVP) (netip.Addr, error) {
	if len(a.Data) < 2 {
		return netip.Addr{}, tgpp.InvalidLength(a)
	}

	switch binary.BigEndian.Uint16(a.Data) {
	case addressFamilyIPv4:
		if len(a.Data) != 2+4 {
			return netip.Addr{}, tgpp.InvalidLength(a)
		}
	case addressFamilyIPv6:
		if len(a.Data) != 2+16 {
			return netip.Addr{}, tgpp.InvalidLength(a)
		}
	default:
		return netip.Addr{}, tgpp.InvalidAVP(a)
	}

	addr, _ := a.Address()

	return addr, nil
}

func optionalAddress(avps []diameter.AVP, code uint32) (netip.Addr, error) {
	a, ok := diameter.Find(avps, code, tgpp.VendorID)
	if !ok {
		return netip.Addr{}, nil
	}

	return addressValue(a)
}

func specificActionAVPs(actions []SpecificAction) ([]diameter.AVP, error) {
	return enumListAVPs(AVPSpecificAction, diameter.AVPFlagMandatory, actions)
}

func specificActions(avps []diameter.AVP, ignoreVoid bool) ([]SpecificAction, error) {
	var actions []SpecificAction

	for _, a := range diameter.FindAll(avps, AVPSpecificAction, tgpp.VendorID) {
		v, err := tgpp.Unsigned32(a)
		if err != nil {
			return nil, err
		}

		action := SpecificAction(v)

		switch {
		case ignoreVoid && action.void():
			continue
		case !action.Known():
			return nil, tgpp.InvalidAVP(a)
		}

		actions = append(actions, action)
	}

	return actions, nil
}

func subscriptionIDAVPs(ids []SubscriptionID) ([]diameter.AVP, error) {
	var avps []diameter.AVP

	for _, s := range ids {
		a, err := subscriptionIDAVP(s)
		if err != nil {
			return nil, err
		}

		avps = append(avps, a)
	}

	return avps, nil
}

func subscriptionIDs(avps []diameter.AVP) ([]SubscriptionID, error) {
	var ids []SubscriptionID

	for _, a := range diameter.FindAll(avps, diameter.AVPSubscriptionID, 0) {
		s, err := parseSubscriptionID(a)
		if err != nil {
			return nil, err
		}

		ids = append(ids, s)
	}

	return ids, nil
}

func chargingIdentifierAVPs(ids []AccessNetworkChargingIdentifier) ([]diameter.AVP, error) {
	var avps []diameter.AVP

	for _, c := range ids {
		a, err := chargingIdentifierAVP(c)
		if err != nil {
			return nil, err
		}

		avps = append(avps, a)
	}

	return avps, nil
}

func chargingIdentifiers(avps []diameter.AVP) ([]AccessNetworkChargingIdentifier, error) {
	var ids []AccessNetworkChargingIdentifier

	for _, a := range diameter.FindAll(avps, AVPAccessNetworkChargingIdentifier, tgpp.VendorID) {
		c, err := parseChargingIdentifier(a)
		if err != nil {
			return nil, err
		}

		ids = append(ids, c)
	}

	return ids, nil
}

func flowsAVPs(flows []Flows) ([]diameter.AVP, error) {
	var avps []diameter.AVP

	for _, f := range flows {
		a, err := flowsAVP(f)
		if err != nil {
			return nil, err
		}

		avps = append(avps, a)
	}

	return avps, nil
}

func flowsList(avps []diameter.AVP) ([]Flows, error) {
	var flows []Flows

	for _, a := range diameter.FindAll(avps, AVPFlows, tgpp.VendorID) {
		f, err := parseFlows(a)
		if err != nil {
			return nil, err
		}

		flows = append(flows, f)
	}

	return flows, nil
}

func classAVPs(values [][]byte) []diameter.AVP {
	avps := make([]diameter.AVP, 0, len(values))

	for _, v := range values {
		avps = append(avps, diameter.OctetString(diameter.AVPClass, diameter.AVPFlagMandatory, 0, v))
	}

	return avps
}

func classes(avps []diameter.AVP) [][]byte {
	var out [][]byte

	for _, a := range diameter.FindAll(avps, diameter.AVPClass, 0) {
		out = append(out, bytes.Clone(a.Data))
	}

	return out
}
