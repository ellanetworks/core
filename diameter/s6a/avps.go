// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s6a

import (
	"bytes"
	"fmt"
	"math"
	"net/netip"

	"github.com/ellanetworks/core/diameter"
	"github.com/ellanetworks/core/diameter/tgpp"
)

const (
	randLen       = 16
	autnLen       = 16
	autsLen       = 14
	kasmeLen      = 32
	keyLen        = 16
	sresLen       = 4
	kcLen         = 8
	minXRESLen    = 4
	maxXRESLen    = 16
	minIMEIDigits = 14
	maxIMEIDigits = 15
	svnDigits     = 2
	maxPriority   = 15
	maxUEUsage    = 255
	maxRFSP       = 256
	maxZoneCodes  = 10
	maxServedIPs  = 2
	maxAgentAddrs = 2
	kbps          = 1000
)

type enum interface {
	~uint32
	valid() bool
}

func optionalUnsigned(avps []diameter.AVP, code uint32) (uint32, bool, error) {
	a, ok := diameter.Find(avps, code, tgpp.VendorID)
	if !ok {
		return 0, false, nil
	}

	v, err := tgpp.Unsigned32(a)
	if err != nil {
		return 0, false, err
	}

	return v, true, nil
}

func requiredUnsigned(avps []diameter.AVP, code uint32) (uint32, error) {
	v, ok, err := optionalUnsigned(avps, code)
	if err == nil && !ok {
		err = tgpp.MissingAVP(code, tgpp.VendorID, 4)
	}

	return v, err
}

func optionalEnum[T enum](avps []diameter.AVP, code uint32) (*T, error) {
	a, ok := diameter.Find(avps, code, tgpp.VendorID)
	if !ok {
		return nil, nil
	}

	v, err := tgpp.Unsigned32(a)
	if err != nil {
		return nil, err
	}

	if !T(v).valid() {
		return nil, tgpp.InvalidAVP(a)
	}

	e := T(v)

	return &e, nil
}

func requiredEnum[T enum](avps []diameter.AVP, code uint32) (T, error) {
	v, err := optionalEnum[T](avps, code)
	if err == nil && v == nil {
		err = tgpp.MissingAVP(code, tgpp.VendorID, 4)
	}

	if err != nil {
		return 0, err
	}

	return *v, nil
}

func optionalFlags[T ~uint32](avps []diameter.AVP, code uint32, defined T) (T, error) {
	v, _, err := optionalUnsigned(avps, code)

	return T(v) & defined, err
}

func optionalBool(avps []diameter.AVP, code uint32) (bool, error) {
	a, ok := diameter.Find(avps, code, tgpp.VendorID)
	if !ok {
		return false, nil
	}

	v, err := tgpp.Unsigned32(a)
	if err != nil {
		return false, err
	}

	switch v {
	case enumFalse:
		return false, nil
	case enumTrue:
		return true, nil
	default:
		return false, tgpp.InvalidAVP(a)
	}
}

func optionalPresence(avps []diameter.AVP, code, value uint32) (bool, error) {
	a, ok := diameter.Find(avps, code, tgpp.VendorID)
	if !ok {
		return false, nil
	}

	v, err := tgpp.Unsigned32(a)
	if err != nil {
		return false, err
	}

	if v != value {
		return false, tgpp.InvalidAVP(a)
	}

	return true, nil
}

func optionalString(avps []diameter.AVP, code, vendorID uint32) (string, error) {
	a, ok := diameter.Find(avps, code, vendorID)
	if !ok {
		return "", nil
	}

	if len(a.Data) == 0 {
		return "", tgpp.InvalidAVP(a)
	}

	return a.UTF8String(), nil
}

func optionalE164(avps []diameter.AVP, code uint32) (string, error) {
	a, ok := diameter.Find(avps, code, tgpp.VendorID)
	if !ok {
		return "", nil
	}

	number, err := tgpp.DecodeE164(a.Data)
	if err != nil {
		return "", tgpp.InvalidAVP(a)
	}

	return number, nil
}

func appendE164(avps []diameter.AVP, code uint32, name, number string) ([]diameter.AVP, error) {
	if number == "" {
		return avps, nil
	}

	b, err := tgpp.EncodeE164(number)
	if err != nil {
		return nil, invalidf("%s: %w", name, err)
	}

	return append(avps, octetsAVP(code, b)), nil
}

func appendEnum[T enum](avps []diameter.AVP, code uint32, name string, v *T) ([]diameter.AVP, error) {
	if v == nil {
		return avps, nil
	}

	if !(*v).valid() {
		return nil, invalidf("%s %d", name, uint32(*v))
	}

	return append(avps, unsignedAVP(code, uint32(*v))), nil
}

func appendFlags[T ~uint32](avps []diameter.AVP, code uint32, name string, v, defined T) ([]diameter.AVP, error) {
	if v&^defined != 0 {
		return nil, invalidf("%s with undefined bits %#x", name, uint32(v&^defined))
	}

	if v == 0 {
		return avps, nil
	}

	return append(avps, unsignedAVP(code, uint32(v))), nil
}

func appendString(avps []diameter.AVP, code uint32, v string) []diameter.AVP {
	if v == "" {
		return avps
	}

	return append(avps, stringAVP(code, v))
}

func grouped(a diameter.AVP) ([]diameter.AVP, error) {
	inner, err := a.Grouped()
	if err != nil {
		return nil, tgpp.InvalidGroupedAVP(a, err)
	}

	return inner, nil
}

func digits(s string, minLen, maxLen int) bool {
	if len(s) < minLen || len(s) > maxLen {
		return false
	}

	for i := range len(s) {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}

	return true
}

type TerminalInformation struct {
	IMEI            string
	SoftwareVersion string
	MEID            []byte
}

func (t TerminalInformation) avp() (diameter.AVP, error) {
	switch {
	case t.IMEI == "" && t.SoftwareVersion == "" && len(t.MEID) == 0:
		return diameter.AVP{}, invalidf("empty terminal information")
	case t.IMEI != "" && !digits(t.IMEI, minIMEIDigits, maxIMEIDigits):
		return diameter.AVP{}, invalidf("IMEI %q", t.IMEI)
	case t.SoftwareVersion != "" && !digits(t.SoftwareVersion, svnDigits, svnDigits):
		return diameter.AVP{}, invalidf("software version %q", t.SoftwareVersion)
	}

	var inner []diameter.AVP

	inner = appendString(inner, AVPIMEI, t.IMEI)

	if len(t.MEID) > 0 {
		inner = append(inner, octetsAVP(AVP3GPP2MEID, t.MEID))
	}

	inner = appendString(inner, AVPSoftwareVersion, t.SoftwareVersion)

	return groupedAVP(AVPTerminalInformation, inner...), nil
}

func parseTerminalInformation(a diameter.AVP) (*TerminalInformation, error) {
	inner, err := grouped(a)
	if err != nil {
		return nil, err
	}

	var t TerminalInformation

	if imei, ok := diameter.Find(inner, AVPIMEI, tgpp.VendorID); ok {
		if t.IMEI = imei.UTF8String(); !digits(t.IMEI, minIMEIDigits, maxIMEIDigits) {
			return nil, tgpp.InvalidAVP(imei)
		}
	}

	if sv, ok := diameter.Find(inner, AVPSoftwareVersion, tgpp.VendorID); ok {
		if t.SoftwareVersion = sv.UTF8String(); !digits(t.SoftwareVersion, svnDigits, svnDigits) {
			return nil, tgpp.InvalidAVP(sv)
		}
	}

	if meid, ok := diameter.Find(inner, AVP3GPP2MEID, tgpp.VendorID); ok {
		t.MEID = bytes.Clone(meid.Data)
	}

	if t.IMEI == "" && t.SoftwareVersion == "" && len(t.MEID) == 0 {
		return nil, nil
	}

	return &t, nil
}

type AMBR struct {
	UplinkBPS   uint64
	DownlinkBPS uint64
}

func bandwidthAVPs(maxCode, extendedCode uint32, bps uint64) ([]diameter.AVP, error) {
	if bps <= math.MaxUint32 {
		return []diameter.AVP{unsignedAVP(maxCode, uint32(bps))}, nil
	}

	extended := bps / kbps
	if extended <= math.MaxUint32/kbps {
		extended = math.MaxUint32/kbps + 1
	}

	if extended > math.MaxUint32 {
		return nil, invalidf("bandwidth of %d bps", bps)
	}

	return []diameter.AVP{
		unsignedAVP(maxCode, math.MaxUint32),
		unsignedAVP(extendedCode, uint32(extended)),
	}, nil
}

func (a AMBR) avp() (diameter.AVP, error) {
	ul, err := bandwidthAVPs(tgpp.AVPMaxRequestedBandwidthUL, tgpp.AVPExtendedMaxRequestedBWUL, a.UplinkBPS)
	if err != nil {
		return diameter.AVP{}, err
	}

	dl, err := bandwidthAVPs(tgpp.AVPMaxRequestedBandwidthDL, tgpp.AVPExtendedMaxRequestedBWDL, a.DownlinkBPS)
	if err != nil {
		return diameter.AVP{}, err
	}

	return groupedAVP(AVPAMBR, append([]diameter.AVP{ul[0], dl[0]}, append(ul[1:], dl[1:]...)...)...), nil
}

func parseBandwidth(avps []diameter.AVP, maxCode, extendedCode uint32) (uint64, error) {
	v, err := requiredUnsigned(avps, maxCode)
	if err != nil {
		return 0, err
	}

	extended, ok, err := optionalUnsigned(avps, extendedCode)
	if err != nil {
		return 0, err
	}

	if ok {
		return uint64(extended) * kbps, nil
	}

	return uint64(v), nil
}

func parseAMBR(a diameter.AVP) (*AMBR, error) {
	inner, err := grouped(a)
	if err != nil {
		return nil, err
	}

	var ambr AMBR

	if ambr.UplinkBPS, err = parseBandwidth(inner, tgpp.AVPMaxRequestedBandwidthUL, tgpp.AVPExtendedMaxRequestedBWUL); err != nil {
		return nil, err
	}

	if ambr.DownlinkBPS, err = parseBandwidth(inner, tgpp.AVPMaxRequestedBandwidthDL, tgpp.AVPExtendedMaxRequestedBWDL); err != nil {
		return nil, err
	}

	return &ambr, nil
}

type AllocationRetentionPriority struct {
	PriorityLevel           uint32
	PreemptionCapability    bool
	PreemptionVulnerability bool
}

type EPSSubscribedQoSProfile struct {
	QCI uint32
	ARP AllocationRetentionPriority
}

func preemption(enabled bool) uint32 {
	if enabled {
		return preemptionEnabled
	}

	return preemptionDisabled
}

func (q EPSSubscribedQoSProfile) avp() (diameter.AVP, error) {
	if q.ARP.PriorityLevel < 1 || q.ARP.PriorityLevel > maxPriority {
		return diameter.AVP{}, invalidf("ARP priority level %d", q.ARP.PriorityLevel)
	}

	return groupedAVP(AVPEPSSubscribedQoSProfile,
		unsignedAVP(tgpp.AVPQoSClassIdentifier, q.QCI),
		groupedAVP(tgpp.AVPAllocationRetentionPriority,
			unsignedAVP(tgpp.AVPPriorityLevel, q.ARP.PriorityLevel),
			unsignedAVP(tgpp.AVPPreemptionCapability, preemption(q.ARP.PreemptionCapability)),
			unsignedAVP(tgpp.AVPPreemptionVulnerability, preemption(q.ARP.PreemptionVulnerability)),
		),
	), nil
}

func parsePreemption(avps []diameter.AVP, code uint32, absent bool) (bool, error) {
	a, ok := diameter.Find(avps, code, tgpp.VendorID)
	if !ok {
		return absent, nil
	}

	v, err := tgpp.Unsigned32(a)
	if err != nil {
		return false, err
	}

	switch v {
	case preemptionEnabled:
		return true, nil
	case preemptionDisabled:
		return false, nil
	default:
		return false, tgpp.InvalidAVP(a)
	}
}

func parseEPSSubscribedQoSProfile(a diameter.AVP) (*EPSSubscribedQoSProfile, error) {
	inner, err := grouped(a)
	if err != nil {
		return nil, err
	}

	var q EPSSubscribedQoSProfile

	if q.QCI, err = requiredUnsigned(inner, tgpp.AVPQoSClassIdentifier); err != nil {
		return nil, err
	}

	arpAVP, ok := diameter.Find(inner, tgpp.AVPAllocationRetentionPriority, tgpp.VendorID)
	if !ok {
		return nil, tgpp.MissingAVP(tgpp.AVPAllocationRetentionPriority, tgpp.VendorID, 0)
	}

	arp, err := grouped(arpAVP)
	if err != nil {
		return nil, err
	}

	level, ok := diameter.Find(arp, tgpp.AVPPriorityLevel, tgpp.VendorID)
	if !ok {
		return nil, tgpp.MissingAVP(tgpp.AVPPriorityLevel, tgpp.VendorID, 4)
	}

	if q.ARP.PriorityLevel, err = tgpp.Unsigned32(level); err != nil {
		return nil, err
	}

	if q.ARP.PriorityLevel < 1 || q.ARP.PriorityLevel > maxPriority {
		return nil, tgpp.InvalidAVP(level)
	}

	if q.ARP.PreemptionCapability, err = parsePreemption(arp, tgpp.AVPPreemptionCapability, false); err != nil {
		return nil, err
	}

	if q.ARP.PreemptionVulnerability, err = parsePreemption(arp, tgpp.AVPPreemptionVulnerability, true); err != nil {
		return nil, err
	}

	return &q, nil
}

type MIP6AgentInfo struct {
	HomeAgentAddresses []netip.Addr
	HomeAgentHost      string
	HomeAgentRealm     string
}

func (m MIP6AgentInfo) avp() (diameter.AVP, error) {
	switch {
	case len(m.HomeAgentAddresses) == 0 && m.HomeAgentHost == "":
		return diameter.AVP{}, invalidf("MIP6-Agent-Info without an address or host")
	case len(m.HomeAgentAddresses) > maxAgentAddrs:
		return diameter.AVP{}, invalidf("MIP6-Agent-Info with %d addresses", len(m.HomeAgentAddresses))
	case (m.HomeAgentHost == "") != (m.HomeAgentRealm == ""):
		return diameter.AVP{}, invalidf("MIP-Home-Agent-Host needs both a host and a realm")
	}

	var inner []diameter.AVP

	for _, addr := range m.HomeAgentAddresses {
		if !addr.IsValid() {
			return diameter.AVP{}, invalidf("MIP-Home-Agent-Address %v", addr)
		}

		inner = append(inner, diameter.Address(AVPMIPHomeAgentAddress, diameter.AVPFlagMandatory, 0, addr.Unmap()))
	}

	if m.HomeAgentHost != "" {
		inner = append(inner, diameter.Grouped(AVPMIPHomeAgentHost, diameter.AVPFlagMandatory, 0,
			diameter.UTF8String(diameter.AVPDestinationRealm, diameter.AVPFlagMandatory, 0, m.HomeAgentRealm),
			diameter.UTF8String(diameter.AVPDestinationHost, diameter.AVPFlagMandatory, 0, m.HomeAgentHost),
		))
	}

	return diameter.Grouped(AVPMIP6AgentInfo, diameter.AVPFlagMandatory, 0, inner...), nil
}

func parseMIP6AgentInfo(a diameter.AVP) (*MIP6AgentInfo, error) {
	inner, err := grouped(a)
	if err != nil {
		return nil, err
	}

	var m MIP6AgentInfo

	addrs := diameter.FindAll(inner, AVPMIPHomeAgentAddress, 0)
	if len(addrs) > maxAgentAddrs {
		return nil, diameter.NewAVPError(diameter.ResultAVPOccursTooManyTimes, addrs[maxAgentAddrs])
	}

	for _, addrAVP := range addrs {
		addr, err := addrAVP.Address()
		if err != nil {
			return nil, tgpp.InvalidAVP(addrAVP)
		}

		m.HomeAgentAddresses = append(m.HomeAgentAddresses, addr)
	}

	if hostAVP, ok := diameter.Find(inner, AVPMIPHomeAgentHost, 0); ok {
		host, err := grouped(hostAVP)
		if err != nil {
			return nil, err
		}

		h, hasHost := diameter.Find(host, diameter.AVPDestinationHost, 0)
		r, hasRealm := diameter.Find(host, diameter.AVPDestinationRealm, 0)

		switch {
		case !hasHost:
			return nil, tgpp.MissingAVP(diameter.AVPDestinationHost, 0, 0)
		case !hasRealm:
			return nil, tgpp.MissingAVP(diameter.AVPDestinationRealm, 0, 0)
		case len(h.Data) == 0:
			return nil, tgpp.InvalidAVP(h)
		case len(r.Data) == 0:
			return nil, tgpp.InvalidAVP(r)
		}

		m.HomeAgentHost, m.HomeAgentRealm = h.UTF8String(), r.UTF8String()
	}

	if len(m.HomeAgentAddresses) == 0 && m.HomeAgentHost == "" {
		return nil, tgpp.InvalidAVP(a)
	}

	return &m, nil
}

type LocalTimeZone struct {
	TimeZone       string
	DaylightSaving DaylightSavingTime
}

func (l LocalTimeZone) avp() (diameter.AVP, error) {
	if l.TimeZone == "" {
		return diameter.AVP{}, invalidf("local time zone without a time zone")
	}

	if !l.DaylightSaving.valid() {
		return diameter.AVP{}, invalidf("Daylight-Saving-Time %d", l.DaylightSaving)
	}

	return groupedAVP(AVPLocalTimeZone,
		stringAVP(AVPTimeZone, l.TimeZone),
		unsignedAVP(AVPDaylightSavingTime, uint32(l.DaylightSaving)),
	), nil
}

func parseLocalTimeZone(a diameter.AVP) (*LocalTimeZone, error) {
	inner, err := grouped(a)
	if err != nil {
		return nil, err
	}

	tz, ok := diameter.Find(inner, AVPTimeZone, tgpp.VendorID)
	if !ok {
		return nil, tgpp.MissingAVP(AVPTimeZone, tgpp.VendorID, 0)
	}

	if len(tz.Data) == 0 {
		return nil, tgpp.InvalidAVP(tz)
	}

	dst, err := requiredEnum[DaylightSavingTime](inner, AVPDaylightSavingTime)
	if err != nil {
		return nil, err
	}

	return &LocalTimeZone{TimeZone: tz.UTF8String(), DaylightSaving: dst}, nil
}

type Resync struct {
	RAND []byte
	AUTS []byte
}

type RequestedAuthenticationInfo struct {
	NumberOfVectors            uint32
	ImmediateResponsePreferred bool
	Resync                     *Resync
}

func (r RequestedAuthenticationInfo) avp(code uint32) (diameter.AVP, error) {
	var inner []diameter.AVP

	if r.NumberOfVectors != 0 {
		inner = append(inner, unsignedAVP(AVPNumberOfRequestedVectors, r.NumberOfVectors))
	}

	if r.ImmediateResponsePreferred {
		inner = append(inner, unsignedAVP(AVPImmediateResponsePreferred, 1))
	}

	if r.Resync != nil {
		if len(r.Resync.RAND) != randLen || len(r.Resync.AUTS) != autsLen {
			return diameter.AVP{}, invalidf("resynchronisation with a %d-octet RAND and a %d-octet AUTS", len(r.Resync.RAND), len(r.Resync.AUTS))
		}

		inner = append(inner, octetsAVP(AVPReSynchronizationInfo, append(bytes.Clone(r.Resync.RAND), r.Resync.AUTS...)))
	}

	return groupedAVP(code, inner...), nil
}

func parseRequestedAuthenticationInfo(a diameter.AVP) (*RequestedAuthenticationInfo, error) {
	inner, err := grouped(a)
	if err != nil {
		return nil, err
	}

	var r RequestedAuthenticationInfo

	if r.NumberOfVectors, _, err = optionalUnsigned(inner, AVPNumberOfRequestedVectors); err != nil {
		return nil, err
	}

	if _, ok := diameter.Find(inner, AVPImmediateResponsePreferred, tgpp.VendorID); ok {
		r.ImmediateResponsePreferred = true
	}

	if resync, ok := diameter.Find(inner, AVPReSynchronizationInfo, tgpp.VendorID); ok {
		if len(resync.Data) != randLen+autsLen {
			return nil, tgpp.InvalidAVP(resync)
		}

		r.Resync = &Resync{RAND: bytes.Clone(resync.Data[:randLen]), AUTS: bytes.Clone(resync.Data[randLen:])}
	}

	return &r, nil
}

type EUTRANVector struct {
	ItemNumber uint32
	RAND       []byte
	XRES       []byte
	AUTN       []byte
	KASME      []byte
}

type UTRANVector struct {
	ItemNumber uint32
	RAND       []byte
	XRES       []byte
	AUTN       []byte
	CK         []byte
	IK         []byte
}

type GERANVector struct {
	ItemNumber uint32
	RAND       []byte
	SRES       []byte
	Kc         []byte
}

type vectorField struct {
	code           uint32
	name           string
	value          *[]byte
	minLen, maxLen int
}

func encodeVector(code, itemNumber uint32, fields []vectorField) (diameter.AVP, error) {
	var inner []diameter.AVP

	if itemNumber != 0 {
		inner = append(inner, unsignedAVP(AVPItemNumber, itemNumber))
	}

	for _, f := range fields {
		if len(*f.value) < f.minLen || len(*f.value) > f.maxLen {
			return diameter.AVP{}, invalidf("authentication vector with a %d-octet %s", len(*f.value), f.name)
		}

		inner = append(inner, octetsAVP(f.code, *f.value))
	}

	return groupedAVP(code, inner...), nil
}

func decodeVector(a diameter.AVP, itemNumber *uint32, fields []vectorField) error {
	inner, err := a.Grouped()
	if err != nil {
		return fmt.Errorf("vector %d: %w", a.Code, err)
	}

	if *itemNumber, _, err = optionalUnsigned(inner, AVPItemNumber); err != nil {
		return err
	}

	for _, f := range fields {
		v, ok := diameter.Find(inner, f.code, tgpp.VendorID)
		if !ok {
			return fmt.Errorf("vector without %s", f.name)
		}

		if len(v.Data) < f.minLen || len(v.Data) > f.maxLen {
			return fmt.Errorf("vector with a %d-octet %s", len(v.Data), f.name)
		}

		*f.value = bytes.Clone(v.Data)
	}

	return nil
}

func (v *EUTRANVector) fields() []vectorField {
	return []vectorField{
		{AVPRAND, "RAND", &v.RAND, randLen, randLen},
		{AVPXRES, "XRES", &v.XRES, minXRESLen, maxXRESLen},
		{AVPAUTN, "AUTN", &v.AUTN, autnLen, autnLen},
		{AVPKASME, "KASME", &v.KASME, kasmeLen, kasmeLen},
	}
}

func (v *UTRANVector) fields() []vectorField {
	return []vectorField{
		{AVPRAND, "RAND", &v.RAND, randLen, randLen},
		{AVPXRES, "XRES", &v.XRES, minXRESLen, maxXRESLen},
		{AVPAUTN, "AUTN", &v.AUTN, autnLen, autnLen},
		{tgpp.AVPConfidentialityKey, "CK", &v.CK, keyLen, keyLen},
		{tgpp.AVPIntegrityKey, "IK", &v.IK, keyLen, keyLen},
	}
}

func (v *GERANVector) fields() []vectorField {
	return []vectorField{
		{AVPRAND, "RAND", &v.RAND, randLen, randLen},
		{AVPSRES, "SRES", &v.SRES, sresLen, sresLen},
		{AVPKc, "Kc", &v.Kc, kcLen, kcLen},
	}
}

func (v EUTRANVector) avp() (diameter.AVP, error) {
	return encodeVector(AVPEUTRANVector, v.ItemNumber, v.fields())
}

func (v UTRANVector) avp() (diameter.AVP, error) {
	return encodeVector(AVPUTRANVector, v.ItemNumber, v.fields())
}

func (v GERANVector) avp() (diameter.AVP, error) {
	return encodeVector(AVPGERANVector, v.ItemNumber, v.fields())
}

func parseEUTRANVector(a diameter.AVP) (EUTRANVector, error) {
	var v EUTRANVector

	err := decodeVector(a, &v.ItemNumber, v.fields())

	return v, err
}

func parseUTRANVector(a diameter.AVP) (UTRANVector, error) {
	var v UTRANVector

	err := decodeVector(a, &v.ItemNumber, v.fields())

	return v, err
}

func parseGERANVector(a diameter.AVP) (GERANVector, error) {
	var v GERANVector

	err := decodeVector(a, &v.ItemNumber, v.fields())

	return v, err
}
